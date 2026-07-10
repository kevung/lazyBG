// Package autocal finds a capture's Board Calibration automatically — the
// step that turns "add a corpus video" from a manual corner-clicking session
// into a command. It is a hypothesize-and-verify loop (architecture §3's
// Session Priors made executable):
//
//  1. a per-pixel temporal MEDIAN over early frames erases hands and dice,
//  2. a color mask of the declared point colors bounds the playing surface,
//     whose extreme projections give an initial corner quad,
//  3. the quad is REFINED by the strongest oracle we own: the opening
//     minutes must contain the standard start position, so the corners are
//     locally optimized until the board reader scores best against it.
//
// The same oracle also yields the Active Span's begin tick for free.
package autocal

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/eval"
	"lazybg/internal/geom"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/perceive/boardstate"
	"lazybg/internal/perceive/checker"
	"lazybg/internal/profile"
)

// Colors are the declared board-scheme colors (Session Priors): the two
// alternating point-triangle colors and the felt they sit on.
type Colors struct {
	PointA, PointB color.RGBA
	Felt           color.RGBA
}

// Options tunes the search.
type Options struct {
	Colors    Colors
	Profile   profile.CaptureProfile // checker colors, for the verification reads
	Canonical calibrate.CanonicalBoard

	ScanEndMs  int     // how deep to look for the opening (default 420s — recordings often start minutes before play; the early-exit keeps the cost nil when the opening is early)
	ColorTol   float64 // per-channel-ish euclidean tolerance for the mask
	MedianN    int     // frames in the temporal median
	DetectW    int     // detection resolution (median+mask); corners scale up
	MinOpening int     // per-point score (of 24) the opening frame must reach
	PeakFrac   float64 // circle-reader tuning for verification reads
}

// DefaultOptions returns the corpus-validated starting tuning.
func DefaultOptions() Options {
	return Options{
		Canonical:  calibrate.CanonicalBoard{MarginX: 16, MarginY: 18, PointW: 58, QuadH: 300, BarGap: 60, OffW: 24},
		ScanEndMs:  420000,
		ColorTol:   45,
		MedianN:    24,
		DetectW:    640,
		MinOpening: 19,
		PeakFrac:   0.38,
	}
}

// Result is a validated automatic calibration.
type Result struct {
	Corners      [4]geom.Pt
	SpanBeginMs  int
	OpeningScore int // per-point score (of 24) at SpanBeginMs
}

// Calibrate runs the full loop on a video.
func Calibrate(video string, o Options) (Result, error) {
	probe, err := capture.FrameAt(video, 0)
	if err != nil {
		return Result{}, fmt.Errorf("autocal probe: %w", err)
	}
	srcW := probe.Bounds().Dx()
	srcH := probe.Bounds().Dy()
	detW := o.DetectW
	detH := srcH * detW / srcW

	// 1. Temporal median at detection resolution over the scan window.
	med, err := MedianFrame(video, 0, o.ScanEndMs, o.MedianN, detW, detH)
	if err != nil {
		return Result{}, err
	}

	// 2. Colors: declared priors, or derived from the median frame itself.
	if o.Colors == (Colors{}) {
		var ok bool
		o.Colors, ok = AutoColors(med)
		if !ok {
			return Result{}, fmt.Errorf("autocal: could not derive board colors from %s", video)
		}
	}

	// 3. Initial quad from the point-color mask, outlier components dropped.
	mask := ColorMask(med, []color.RGBA{o.Colors.PointA, o.Colors.PointB}, o.ColorTol)
	mask = TriangleComponents(mask, med, o.Colors.Felt, o.ColorTol, detW, detH)

	// 4. Candidate initial quads — RowQuad (rotation-aware) and the extreme
	// projections — each refined against the opening oracle; the best final
	// read wins. Hypothesize-and-verify all the way: a candidate whose PCA
	// went wrong (few/odd components can produce a wild diagonal axis) just
	// loses the contest instead of sinking the calibration.
	var cands, outOfBounds [][4]geom.Pt
	if q, ok := RowQuad(mask, detW, detH); ok {
		if quadInBounds(q, detW, detH) {
			cands = append(cands, q)
		} else {
			outOfBounds = append(outOfBounds, clampQuad(q, detW, detH))
		}
	}
	if q, ok := QuadFromMask(mask, detW, detH); ok {
		if quadInBounds(q, detW, detH) {
			cands = append(cands, q)
		} else {
			outOfBounds = append(outOfBounds, clampQuad(q, detW, detH))
		}
	}
	// Bounds order preference; a runaway fit still gets its (clamped) shot
	// rather than aborting the calibration outright.
	cands = append(cands, outOfBounds...)
	if len(cands) == 0 {
		return Result{}, fmt.Errorf("autocal: point-color mask found no plausible board in %s", video)
	}

	sx := float64(srcW) / float64(detW)
	best := Result{OpeningScore: -1}
	for _, quad := range cands {
		var corners [4]geom.Pt
		for i, p := range quad {
			corners[i] = geom.P(p.X*sx, p.Y*sx)
		}
		corners = expandQuad(corners, 0.045, 0.04)

		tick, _, err := FindOpening(video, corners, o, 0)
		if err != nil {
			continue
		}
		frame, err := capture.FrameAt(video, tick)
		if err != nil {
			continue
		}
		corners = RefineCorners(frame, corners, o)
		tick, score, err := FindOpening(video, corners, o, 0)
		if err != nil {
			continue
		}
		if score > best.OpeningScore {
			best = Result{Corners: corners, SpanBeginMs: tick, OpeningScore: score}
		}
		if best.OpeningScore >= o.MinOpening+2 {
			break // already a confident calibration; skip the other candidate
		}
	}
	if best.OpeningScore < 0 {
		return Result{}, fmt.Errorf("autocal: no candidate quad produced a readable opening in %s", video)
	}
	if best.OpeningScore < o.MinOpening {
		return best, fmt.Errorf("autocal: best opening read %d/24 below %d — calibration not trusted", best.OpeningScore, o.MinOpening)
	}
	return best, nil
}

// clampQuad projects a quad's corners into the frame.
func clampQuad(q [4]geom.Pt, w, h int) [4]geom.Pt {
	for i, p := range q {
		q[i] = geom.P(math.Min(math.Max(p.X, 0), float64(w-1)), math.Min(math.Max(p.Y, 0), float64(h-1)))
	}
	return q
}

// quadInBounds flags quads that stray far outside the frame — the sign of
// a runaway fit, not a board.
func quadInBounds(q [4]geom.Pt, w, h int) bool {
	mx, my := 0.15*float64(w), 0.15*float64(h)
	for _, p := range q {
		if p.X < -mx || p.X > float64(w)+mx || p.Y < -my || p.Y > float64(h)+my {
			return false
		}
	}
	return true
}

// MedianFrame decodes n frames evenly across [beginMs,endMs] at (w,h) and
// returns their per-pixel, per-channel median — a hands-free view of the
// static scene.
func MedianFrame(video string, beginMs, endMs, n, w, h int) (*image.RGBA, error) {
	if n < 3 {
		n = 3
	}
	src, err := capture.Stream(video, capture.StreamOpts{
		BeginMs: beginMs, EndMs: endMs,
		FPS: float64(n) * 1000 / float64(endMs-beginMs),
		W:   w, H: h,
	})
	if err != nil {
		return nil, err
	}
	defer src.Close()
	var frames []*image.RGBA
	for {
		f, ok := src.Next()
		if !ok {
			break
		}
		frames = append(frames, f.Img.(*image.RGBA))
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("median: no frames decoded from %s", video)
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	vals := make([]uint8, len(frames))
	for i := 0; i < w*h*4; i++ {
		if i%4 == 3 {
			out.Pix[i] = 0xff
			continue
		}
		for k, f := range frames {
			vals[k] = f.Pix[i]
		}
		out.Pix[i] = median8(vals)
	}
	return out, nil
}

func median8(v []uint8) uint8 {
	// counting sort over 256 bins — v is small but this is branch-simple
	var bins [256]int
	for _, x := range v {
		bins[x]++
	}
	mid := len(v) / 2
	acc := 0
	for b, c := range bins {
		acc += c
		if acc > mid {
			return uint8(b)
		}
	}
	return 0
}

// ColorMask marks pixels within tol (euclidean RGB distance) of any target.
func ColorMask(img *image.RGBA, targets []color.RGBA, tol float64) []bool {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	mask := make([]bool, w*h)
	tol2 := tol * tol
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			r, g, bl := float64(img.Pix[i]), float64(img.Pix[i+1]), float64(img.Pix[i+2])
			for _, t := range targets {
				dr, dg, db := r-float64(t.R), g-float64(t.G), bl-float64(t.B)
				if dr*dr+dg*dg+db*db <= tol2 {
					mask[y*w+x] = true
					break
				}
			}
		}
	}
	return mask
}

// QuadFromMask returns the mask's extreme-projection corners (TL,TR,BR,BL) —
// the argmax points of ±x±y — provided the mask plausibly covers a board
// (enough pixels, enough spread).
func QuadFromMask(mask []bool, w, h int) ([4]geom.Pt, bool) {
	var tl, tr, br, bl geom.Pt
	stl, str, sbr, sbl := 1e18, -1e18, -1e18, 1e18
	count := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y*w+x] {
				continue
			}
			count++
			fx, fy := float64(x), float64(y)
			if fx+fy < stl {
				stl, tl = fx+fy, geom.P(fx, fy)
			}
			if fx-fy > str {
				str, tr = fx-fy, geom.P(fx, fy)
			}
			if fx+fy > sbr {
				sbr, br = fx+fy, geom.P(fx, fy)
			}
			if fx-fy < sbl {
				sbl, bl = fx-fy, geom.P(fx, fy)
			}
		}
	}
	if count < w*h/100 { // under 1% coverage is not a board
		return [4]geom.Pt{}, false
	}
	return [4]geom.Pt{tl, tr, br, bl}, true
}

// openingScore reads a frame with the candidate corners and scores it
// against the standard start: the integer per-point score plus the
// continuous whole-board agreement as a fractional tie-break, so the
// refinement keeps a gradient even where the integer score saturates.
func openingScore(frame image.Image, corners [4]geom.Pt, o Options) float64 {
	cal, ok := calibrate.New(corners, o.Canonical)
	if !ok {
		return -1
	}
	reader := boardstate.CircleReader{Profile: o.Profile, Params: checker.Params{PeakFrac: o.PeakFrac}}
	obs := reader.Read(cal.Rectify(frame), o.Canonical)
	start := bg.StandardStart()
	return float64(eval.ScoreBoard(obs, start).Correct) + boarddiff.WholeBoardMatch(start, obs)
}

// FindOpening scans [scanBeginMs, o.ScanEndMs] at 1 fps for the earliest
// frame whose read matches the standard start well — the settled opening,
// which doubles as the Active Span's begin.
func FindOpening(video string, corners [4]geom.Pt, o Options, scanBeginMs int) (int, int, error) {
	bestTick, bestScore := -1, -1.0
	for tick := scanBeginMs; tick < o.ScanEndMs; tick += 1000 {
		frame, err := capture.FrameAt(video, tick)
		if err != nil {
			continue
		}
		s := openingScore(frame, corners, o)
		if s > bestScore {
			bestTick, bestScore = tick, s
		}
		// The earliest confident opening wins: no need to scan past a great
		// match (play will begin and scores only degrade).
		if bestScore >= float64(o.MinOpening)+2 {
			break
		}
	}
	if bestTick < 0 {
		return 0, 0, fmt.Errorf("autocal: no readable frame in the scan window of %s", video)
	}
	return bestTick, int(bestScore), nil
}

// RefineCorners hill-climbs to maximize the opening read — the
// oracle-driven refinement that absorbs mask margins, slight rotations and
// lens residue. Besides single corners it moves whole EDGES and the whole
// quad: when one side of the board is collectively offset (the usual mask
// error), no single-corner move improves the read and pure per-corner
// descent stalls in a local optimum.
func RefineCorners(frame image.Image, corners [4]geom.Pt, o Options) [4]geom.Pt {
	// move groups: each is the set of corner indices displaced together
	groups := [][]int{
		{0}, {1}, {2}, {3}, // single corners
		{0, 3}, {1, 2}, // left edge, right edge
		{0, 1}, {2, 3}, // top edge, bottom edge
		{0, 1, 2, 3}, // whole quad
	}
	best := openingScore(frame, corners, o)
	for _, step := range []float64{24, 12, 6, 3} {
		improved := true
		for improved {
			improved = false
			for _, g := range groups {
				for _, d := range [][2]float64{{step, 0}, {-step, 0}, {0, step}, {0, -step}} {
					cand := corners
					for _, c := range g {
						cand[c] = geom.P(corners[c].X+d[0], corners[c].Y+d[1])
					}
					if s := openingScore(frame, cand, o); s > best {
						best, corners = s, cand
						improved = true
					}
				}
			}
			// Rotation moves: tilted captures (extreme-projection initial
			// quads are axis-biased) need the whole quad to turn — a
			// translation-only walk stalls on them.
			for _, deg := range []float64{step / 4, -step / 4} {
				cand := rotateQuad(corners, deg)
				if s := openingScore(frame, cand, o); s > best {
					best, corners = s, cand
					improved = true
				}
			}
		}
	}
	return corners
}

// rotateQuad turns the quad around its centroid by deg degrees.
func rotateQuad(q [4]geom.Pt, deg float64) [4]geom.Pt {
	var cx, cy float64
	for _, p := range q {
		cx += p.X / 4
		cy += p.Y / 4
	}
	rad := deg * math.Pi / 180
	sin, cos := math.Sin(rad), math.Cos(rad)
	for i, p := range q {
		dx, dy := p.X-cx, p.Y-cy
		q[i] = geom.P(cx+dx*cos-dy*sin, cy+dx*sin+dy*cos)
	}
	return q
}

// TriangleComponents keeps only the mask's connected components that look
// like point triangles: tall (viewing angles compress them, but they stay
// taller than wide), big enough to matter, not huge blobs, AND sitting on
// felt — the ring of pixels around a real triangle is mostly the declared
// felt color, which a clothing patch, the doubling cube or a marbled-checker
// swirl never satisfies.
func TriangleComponents(mask []bool, img *image.RGBA, felt color.RGBA, tol float64, w, h int) []bool {
	minArea := w * h / 3500 // ≈ a slim triangle at detection resolution
	maxArea := w * h / 40   // anything bigger is not one triangle
	out := make([]bool, len(mask))
	seen := make([]bool, len(mask))
	var stack []int
	for start := range mask {
		if !mask[start] || seen[start] {
			continue
		}
		stack = append(stack[:0], start)
		seen[start] = true
		var comp []int
		minX, minY, maxX, maxY := w, h, 0, 0
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			comp = append(comp, i)
			x, y := i%w, i/w
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
			for _, j := range [4]int{i - 1, i + 1, i - w, i + w} {
				if j < 0 || j >= len(mask) || seen[j] || !mask[j] {
					continue
				}
				if (j == i-1 && x == 0) || (j == i+1 && x == w-1) {
					continue
				}
				seen[j] = true
				stack = append(stack, j)
			}
		}
		if len(comp) < minArea || len(comp) > maxArea {
			continue
		}
		if float64(maxY-minY+1) < 1.4*float64(maxX-minX+1) {
			continue // triangles read taller than wide
		}
		// Ring test: sample the bbox border (expanded by 3px); a triangle
		// sits on felt, an off-board look-alike does not.
		feltHits, ringN := 0, 0
		tol2 := tol * tol
		x0, y0, x1, y1 := minX-3, minY-3, maxX+3, maxY+3
		for x := x0; x <= x1; x++ {
			for _, y := range [2]int{y0, y1} {
				if x < 0 || x >= w || y < 0 || y >= h {
					continue
				}
				ringN++
				if colorNear(img, x, y, felt, tol2) {
					feltHits++
				}
			}
		}
		for y := y0 + 1; y < y1; y++ {
			for _, x := range [2]int{x0, x1} {
				if x < 0 || x >= w || y < 0 || y >= h {
					continue
				}
				ringN++
				if colorNear(img, x, y, felt, tol2) {
					feltHits++
				}
			}
		}
		if ringN == 0 || float64(feltHits) < 0.35*float64(ringN) {
			continue
		}
		for _, i := range comp {
			out[i] = true
		}
	}
	return out
}

func colorNear(img *image.RGBA, x, y int, c color.RGBA, tol2 float64) bool {
	i := img.PixOffset(img.Bounds().Min.X+x, img.Bounds().Min.Y+y)
	dr := float64(img.Pix[i]) - float64(c.R)
	dg := float64(img.Pix[i+1]) - float64(c.G)
	db := float64(img.Pix[i+2]) - float64(c.B)
	return dr*dr+dg*dg+db*db <= tol2
}

// expandQuad pushes each corner outward from the quad centroid — the point
// triangles sit inside the playing surface by the board's margins and off
// column, so the mask quad underestimates the surface by a few percent.
func expandQuad(q [4]geom.Pt, fx, fy float64) [4]geom.Pt {
	var cx, cy float64
	for _, p := range q {
		cx += p.X / 4
		cy += p.Y / 4
	}
	for i, p := range q {
		q[i] = geom.P(cx+(p.X-cx)*(1+fx*2), cy+(p.Y-cy)*(1+fy*2))
	}
	return q
}

// AutoColors derives the board's color priors from the median frame itself:
// the felt is the dominant low-saturation tone of the frame's center, and
// the point colors are the two dominant saturated clusters ADJACENT to felt
// pixels — adjacency is what excludes the table wood, carpet and clothing,
// which are saturated but never surrounded by felt.
func AutoColors(med *image.RGBA) (Colors, bool) {
	b := med.Bounds()
	w, h := b.Dx(), b.Dy()
	x0, x1 := w/5, w*4/5
	y0, y1 := h/5, h*4/5

	q := func(v uint8) int { return int(v) / 24 }
	type bin struct{ r, g, b int }
	at := func(x, y int) (uint8, uint8, uint8) {
		i := med.PixOffset(b.Min.X+x, b.Min.Y+y)
		return med.Pix[i], med.Pix[i+1], med.Pix[i+2]
	}

	// 1. felt: dominant unsaturated mid-brightness bin in the center.
	feltBins := map[bin]int{}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, bl := at(x, y)
			mx, mn := max(r, max(g, bl)), min(r, min(g, bl))
			if int(mx)-int(mn) < 30 && mx > 60 && mx < 235 {
				feltBins[bin{q(r), q(g), q(bl)}]++
			}
		}
	}
	var feltBin bin
	bestN := 0
	for k, n := range feltBins {
		if n > bestN {
			feltBin, bestN = k, n
		}
	}
	if bestN == 0 {
		return Colors{}, false
	}
	felt := color.RGBA{uint8(feltBin.r*24 + 12), uint8(feltBin.g*24 + 12), uint8(feltBin.b*24 + 12), 255}

	// felt mask for the adjacency test
	isFelt := func(x, y int) bool {
		if x < 0 || x >= w || y < 0 || y >= h {
			return false
		}
		r, g, bl := at(x, y)
		return q(r) == feltBin.r && q(g) == feltBin.g && q(bl) == feltBin.b
	}

	// 2. point-color candidates: pixels clearly DIFFERENT from the felt yet
	// adjacent to it. A hard saturation gate misses dim point colors under
	// warm light (teal reads (24,72,72): spread 48); distance-from-felt with
	// only a weak spread floor keeps them while the felt-distance excludes
	// warm wood and the spread floor excludes white/dark checkers.
	satBins := map[bin]int{}
	sums := map[bin][4]int{} // r,g,b,count for cluster averaging
	const reach = 8
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, bl := at(x, y)
			mx, mn := max(r, max(g, bl)), min(r, min(g, bl))
			if int(mx)-int(mn) < 18 {
				continue
			}
			dr := float64(r) - float64(felt.R)
			dg := float64(g) - float64(felt.G)
			db := float64(bl) - float64(felt.B)
			if dr*dr+dg*dg+db*db < 45*45 {
				continue
			}
			adj := 0
			for _, d := range [4][2]int{{reach, 0}, {-reach, 0}, {0, reach}, {0, -reach}} {
				if isFelt(x+d[0], y+d[1]) {
					adj++
				}
			}
			if adj < 1 {
				continue
			}
			k := bin{q(r), q(g), q(bl)}
			satBins[k]++
			s := sums[k]
			s[0] += int(r)
			s[1] += int(g)
			s[2] += int(bl)
			s[3]++
			sums[k] = s
		}
	}
	// top-2 distinct bins (distinct = not neighbours in quantized space)
	var k1, k2 bin
	n1, n2 := 0, 0
	for k, n := range satBins {
		if n > n1 {
			k2, n2 = k1, n1
			k1, n1 = k, n
		} else if n > n2 {
			near := abs(k.r-k1.r) <= 1 && abs(k.g-k1.g) <= 1 && abs(k.b-k1.b) <= 1
			if !near {
				k2, n2 = k, n
			}
		}
	}
	if n1 == 0 {
		return Colors{}, false
	}
	avg := func(k bin) color.RGBA {
		s := sums[k]
		if s[3] == 0 {
			return color.RGBA{}
		}
		return color.RGBA{uint8(s[0] / s[3]), uint8(s[1] / s[3]), uint8(s[2] / s[3]), 255}
	}
	c := Colors{PointA: avg(k1), PointB: avg(k1), Felt: felt}
	if n2 > 0 {
		c.PointB = avg(k2)
	}
	return c, true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// RowQuad fits the initial quad as a ROTATED bounding rectangle of the
// filtered triangle mask: the rotation angle comes from the principal axis
// of the triangle-component centroids (the two point rows define the board's
// true orientation), so tilted captures get a correctly-ordered quad where
// extreme ±x±y projections would return a diamond with scrambled corner
// identities. Returns false on degenerate masks (fall back to QuadFromMask).
func RowQuad(mask []bool, w, h int) ([4]geom.Pt, bool) {
	// component centroids of the (already filtered) mask
	seen := make([]bool, len(mask))
	var cents []geom.Pt
	var stack []int
	for start := range mask {
		if !mask[start] || seen[start] {
			continue
		}
		stack = append(stack[:0], start)
		seen[start] = true
		var sx, sy, n float64
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := i%w, i/w
			sx += float64(x)
			sy += float64(y)
			n++
			for _, j := range [4]int{i - 1, i + 1, i - w, i + w} {
				if j < 0 || j >= len(mask) || seen[j] || !mask[j] {
					continue
				}
				if (j == i-1 && x == 0) || (j == i+1 && x == w-1) {
					continue
				}
				seen[j] = true
				stack = append(stack, j)
			}
		}
		if n >= 30 {
			cents = append(cents, geom.P(sx/n, sy/n))
		}
	}
	if len(cents) < 8 {
		return [4]geom.Pt{}, false
	}

	// principal axis of the centroids (2x2 covariance eigenvector)
	var mx, my float64
	for _, c := range cents {
		mx += c.X / float64(len(cents))
		my += c.Y / float64(len(cents))
	}
	var sxx, sxy, syy float64
	for _, c := range cents {
		dx, dy := c.X-mx, c.Y-my
		sxx += dx * dx
		sxy += dx * dy
		syy += dy * dy
	}
	theta := 0.5 * math.Atan2(2*sxy, sxx-syy)
	// clamp to ±45°: beyond that the corner-order convention breaks anyway
	if theta > math.Pi/4 {
		theta -= math.Pi / 2
	}
	if theta < -math.Pi/4 {
		theta += math.Pi / 2
	}

	// rotated bounding rect of ALL mask pixels
	sin, cos := math.Sin(-theta), math.Cos(-theta)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i, on := range mask {
		if !on {
			continue
		}
		x, y := float64(i%w)-mx, float64(i/w)-my
		rx := x*cos - y*sin
		ry := x*sin + y*cos
		minX, maxX = math.Min(minX, rx), math.Max(maxX, rx)
		minY, maxY = math.Min(minY, ry), math.Max(maxY, ry)
	}
	sin, cos = math.Sin(theta), math.Cos(theta)
	back := func(x, y float64) geom.Pt {
		return geom.P(mx+x*cos-y*sin, my+x*sin+y*cos)
	}
	return [4]geom.Pt{
		back(minX, minY), back(maxX, minY), back(maxX, maxY), back(minX, maxY),
	}, true
}

// CalibrateAssisted runs the oracle half of the loop on HUMAN-provided
// initial corners: find the settled opening, refine the corners against it,
// re-settle the opening tick. This is the "4 clicks + polish" mode the
// board-detection survey recommends as the universal fallback — the human
// (or an operator reading one frame) supplies the quad the mask stage could
// not, and the standard-start oracle does the rest.
func CalibrateAssisted(video string, initial [4]geom.Pt, o Options) (Result, error) {
	tick, _, err := FindOpening(video, initial, o, 0)
	if err != nil {
		return Result{}, err
	}
	frame, err := capture.FrameAt(video, tick)
	if err != nil {
		return Result{}, err
	}
	corners := RefineCorners(frame, initial, o)
	tick, score, err := FindOpening(video, corners, o, 0)
	if err != nil {
		return Result{}, err
	}
	res := Result{Corners: corners, SpanBeginMs: tick, OpeningScore: score}
	if score < o.MinOpening {
		return res, fmt.Errorf("autocal assisted: best opening read %d/24 below %d", score, o.MinOpening)
	}
	return res, nil
}
