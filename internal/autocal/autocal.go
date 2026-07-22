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
	Corners [4]geom.Pt
	// BarEdges and Lens come from the multi-instant correspondence fit
	// (ADR-0008 §7); BarEdges is nil when the fit did not run and the
	// mask-quad fallback produced the corners.
	BarEdges     []geom.Pt
	Lens         calibrate.Lens
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

	// 2. Colors: declared priors, or ranked hypotheses derived from the
	// median frame — each verified by the whole calibrate-and-read loop; the
	// first hypothesis reaching a trusted opening wins (#56).
	colorCands := []Colors{o.Colors}
	if o.Colors == (Colors{}) {
		colorCands = AutoColorCandidates(med, 4)
		if len(colorCands) == 0 {
			return Result{}, fmt.Errorf("autocal: could not derive board colors from %s", video)
		}
	}
	best := Result{OpeningScore: -1}
	for _, colors := range colorCands {
		oc := o
		oc.Colors = colors
		res, err := calibrateColors(video, oc, med, srcW, srcH, detW, detH)
		if err != nil {
			continue
		}
		if res.OpeningScore >= o.MinOpening {
			return res, nil
		}
		if res.OpeningScore > best.OpeningScore {
			best = res
		}
	}
	if best.OpeningScore < 0 {
		return Result{}, fmt.Errorf("autocal: no color hypothesis produced a readable opening in %s (%d tried)", video, len(colorCands))
	}
	return best, fmt.Errorf("autocal: best opening read %d/24 below %d — calibration not trusted", best.OpeningScore, o.MinOpening)
}

// calibrateColors is Calibrate's quad-hypothesis loop for ONE color scheme
// (o.Colors is set).
func calibrateColors(video string, o Options, med *image.RGBA, srcW, srcH, detW, detH int) (Result, error) {

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
		// Multi-instant correspondence fit (ADR-0008 §7): apexes aggregated
		// over spaced instants — the checkers move between them, so the
		// union covers triangles any single instant loses. The fit is
		// scored with its FULL calibration (8 handles + lens); the legacy
		// oracle path (RefineCorners on the mask quad, corners-only model)
		// runs as well, and the better opening read wins — the oracle stays
		// the final judge either way.
		cand := Result{OpeningScore: -1}
		if fit, ok := fitAtInstants(video, []int{tick, tick + 90000, tick + 180000}, o, corners, srcW, srcH); ok {
			if cal, okCal := calibrate.NewFromHandles(fit.Corners, fit.BarEdges[:], o.Canonical, fit.Lens); okCal {
				if fTick, _, err := findOpeningCal(video, cal, o, 0); err == nil {
					fc, fb := fit.Corners, fit.BarEdges
					if frame, err := capture.FrameAt(video, fTick); err == nil {
						fc, fb = RefineHandles(frame, fc, fb, fit.Lens, o)
					}
					if cal2, ok2 := calibrate.NewFromHandles(fc, fb[:], o.Canonical, fit.Lens); ok2 {
						if fTick2, fScore, err := findOpeningCal(video, cal2, o, 0); err == nil {
							cand = Result{Corners: fc, BarEdges: fb[:], Lens: fit.Lens, SpanBeginMs: fTick2, OpeningScore: fScore}
						}
					}
				}
			}
		}
		frame, err := capture.FrameAt(video, tick)
		if err == nil {
			legacy := RefineCorners(frame, corners, o)
			if lTick, lScore, err := FindOpening(video, legacy, o, 0); err == nil && lScore > cand.OpeningScore {
				cand = Result{Corners: legacy, SpanBeginMs: lTick, OpeningScore: lScore}
			}
		}
		if cand.OpeningScore > best.OpeningScore {
			best = cand
		}
		if best.OpeningScore >= o.MinOpening+2 {
			break // already a confident calibration; skip the other candidate
		}
	}
	if best.OpeningScore < 0 {
		return Result{}, fmt.Errorf("autocal: no candidate quad produced a readable opening in %s", video)
	}
	return best, nil
}

// DetectHandles detects ALL eight calibration handles — the four playing-surface
// corners (TL,TR,BR,BL) AND the four bar-edge points (barTL,barTR,barBR,barBL) —
// from a SHORT temporal median around tickMs. It is a fast, single-shot
// alternative to Calibrate for interactive use (issue #47): no video-wide opening
// scan and no opening verification, so it returns in a moment and works on any
// frame that shows the board. The result is a best-effort SEED the user refines
// by dragging, not a trusted calibration. All points are source-frame pixels.
// The returned lens (zero = pinhole) is the fit's admitted radial
// distortion, already scaled to source pixels.
func DetectHandles(video string, tickMs int, o Options) (corners, barEdges [4]geom.Pt, lens calibrate.Lens, err error) {
	probe, err := capture.FrameAt(video, tickMs)
	if err != nil {
		return corners, barEdges, lens, fmt.Errorf("decode at %dms: %w", tickMs, err)
	}
	srcW, srcH := probe.Bounds().Dx(), probe.Bounds().Dy()
	// Detect at the tuned default resolution: the mask/component thresholds and
	// the bar-valley detection are calibrated for it — raising it perturbed the
	// quad and regressed the bar. Corner precision is tuned separately.
	detW := o.DetectW
	detH := srcH * detW / srcW
	// A short median suppresses transient hands/dice without scanning the video.
	med, err := MedianFrame(video, tickMs, tickMs+1500, 5, detW, detH)
	if err != nil {
		return corners, barEdges, lens, fmt.Errorf("sample near %dms: %w", tickMs, err)
	}
	// Color hypotheses: declared priors, or the ranked candidates derived
	// from the frame — the correspondence fit is the verifier that decides
	// which hypothesis is actually a board (#56). The first hypothesis whose
	// fit locks wins; if none fits, the first hypothesis that at least
	// produced a plausible quad keeps the legacy single-answer behaviour.
	cands := []Colors{o.Colors}
	if o.Colors == (Colors{}) {
		cands = AutoColorCandidates(med, 5)
		if len(cands) == 0 {
			return corners, barEdges, lens, fmt.Errorf("could not derive board colours from the frame — position it on the board, or place the handles by hand")
		}
	}
	var seedC, seedB [4]geom.Pt
	found := false
	for _, colors := range cands {
		c, b, l, fitOK, ok := detectWithColors(med, colors, o, detW, detH)
		if !ok {
			continue
		}
		if fitOK {
			seedC, seedB, lens = c, b, l
			found = true
			break
		}
		if !found {
			// remember the first plausible-but-unverified quad; keep
			// scanning — a later hypothesis whose fit locks beats it
			seedC, seedB, lens = c, b, l
			found = true
		}
	}
	if !found {
		return corners, barEdges, lens, fmt.Errorf("no plausible board found in the frame (%d colour hypotheses tried)", len(cands))
	}

	sx := float64(srcW) / float64(detW)
	for i := range seedC {
		corners[i] = geom.P(seedC[i].X*sx, seedC[i].Y*sx)
	}
	for i := range seedB {
		barEdges[i] = geom.P(seedB[i].X*sx, seedB[i].Y*sx)
	}
	// The lens was estimated in detection space; its coefficients are
	// dimensionless, only centre and norm scale to source pixels.
	if lens.Norm > 0 {
		lens.CenterX *= sx
		lens.CenterY *= sx
		lens.Norm *= sx
	}
	return corners, barEdges, lens, nil
}

// barFractions finds the bar's left/right position as fractions of the board
// width from a rectified board image. It locates the bar centre as the lowest
// point-triangle-density column in the central band, then walks outward to the
// half-maximum crossings — the onset of the flanking point columns — which pins
// the bar edges far more precisely than a flat threshold. Returns a centred
// default when no clear valley exists.
func barFractions(rect *image.RGBA, cb calibrate.CanonicalBoard, colors Colors, tol float64) (leftFrac, rightFrac float64) {
	w, h := cb.Size()
	mask := ColorMask(rect, []color.RGBA{colors.PointA, colors.PointB}, tol)
	y0, y1 := cb.MarginY, h-cb.MarginY
	dens := make([]float64, w)
	for x := 0; x < w; x++ {
		c := 0
		for y := y0; y < y1; y++ {
			if mask[y*w+x] {
				c++
			}
		}
		dens[x] = float64(c)
	}
	// Smooth (moving average) to steady the edge crossings.
	const r = 3
	sm := make([]float64, w)
	for x := 0; x < w; x++ {
		var s float64
		var n int
		for k := -r; k <= r; k++ {
			if x+k >= 0 && x+k < w {
				s += dens[x+k]
				n++
			}
		}
		sm[x] = s / float64(n)
	}
	maxD := 0.0
	for _, d := range sm {
		if d > maxD {
			maxD = d
		}
	}
	if maxD == 0 {
		return 0.47, 0.53
	}
	// Bar centre: the lowest-density column in the central band.
	lo, hi := int(float64(w)*0.42), int(float64(w)*0.58)
	mid, minD := lo, sm[lo]
	for x := lo; x <= hi; x++ {
		if sm[x] < minD {
			minD, mid = sm[x], x
		}
	}
	// Walk out to where density reaches half the peak — the flanking columns.
	thr := 0.5 * maxD
	left, right := mid, mid
	for left > 0 && sm[left] < thr {
		left--
	}
	for right < w-1 && sm[right] < thr {
		right++
	}
	bw := right - left
	if bw < int(0.01*float64(w)) || bw > int(0.22*float64(w)) {
		return 0.47, 0.53 // implausible width → centred default
	}
	return float64(left) / float64(w), float64(right) / float64(w)
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

// openingScoreCal is openingScore on a FULL calibration (8 handles + lens):
// the fit's bar edges and lens are part of what it estimated, and scoring
// them through the corners-only v1 model would throw them away (the manual
// reference corners were hand-tuned UNDER that v1 model, so the comparison
// would punish the fit for being right).
func openingScoreCal(frame image.Image, cal calibrate.BoardCalibration, o Options) float64 {
	reader := boardstate.CircleReader{Profile: o.Profile, Params: checker.Params{PeakFrac: o.PeakFrac}}
	obs := reader.Read(cal.Rectify(frame), o.Canonical)
	start := bg.StandardStart()
	return float64(eval.ScoreBoard(obs, start).Correct) + boarddiff.WholeBoardMatch(start, obs)
}

// findOpeningCal is FindOpening with a full calibration.
func findOpeningCal(video string, cal calibrate.BoardCalibration, o Options, scanBeginMs int) (int, int, error) {
	bestTick, bestScore := -1, -1.0
	for tick := scanBeginMs; tick < o.ScanEndMs; tick += 1000 {
		frame, err := capture.FrameAt(video, tick)
		if err != nil {
			continue
		}
		s := openingScoreCal(frame, cal, o)
		if s > bestScore {
			bestTick, bestScore = tick, s
		}
		if bestScore >= float64(o.MinOpening)+2 {
			break
		}
	}
	if bestTick < 0 {
		return 0, 0, fmt.Errorf("autocal: no readable frame in the scan window of %s", video)
	}
	return bestTick, int(bestScore), nil
}

// RefineHandles hill-climbs the FULL eight handles — corners AND bar edges —
// to maximize the full-model opening read (openingScoreCal). It is the
// oracle polish for the correspondence fit: the fit pins columns and outer
// lines from image structure, but the x extrapolation to the outer corners
// and the bar width rest on canonical proportions that vary per board; the
// opening oracle observes exactly those. RefineCorners cannot help here — it
// moves corners only, under the v1 corners-only model.
func RefineHandles(frame image.Image, corners, barEdges [4]geom.Pt, lens calibrate.Lens, o Options) ([4]geom.Pt, [4]geom.Pt) {
	score := func(c, b [4]geom.Pt) float64 {
		cal, ok := calibrate.NewFromHandles(c, b[:], o.Canonical, lens)
		if !ok {
			return -1
		}
		return openingScoreCal(frame, cal, o)
	}
	type move struct {
		corners []int // corner indices displaced together
		bars    []int // bar-edge indices displaced together
		xOnly   bool
	}
	moves := []move{
		{corners: []int{0}}, {corners: []int{1}}, {corners: []int{2}}, {corners: []int{3}},
		{corners: []int{0, 3}}, {corners: []int{1, 2}}, // left / right edge
		{corners: []int{0, 1}}, {corners: []int{2, 3}}, // top / bottom edge
		{corners: []int{0, 1, 2, 3}, bars: []int{0, 1, 2, 3}}, // whole board
		{bars: []int{0, 3}, xOnly: true},                      // bar left edge
		{bars: []int{1, 2}, xOnly: true},                      // bar right edge
		{bars: []int{0, 1, 2, 3}, xOnly: true},                // whole bar
	}
	best := score(corners, barEdges)
	for _, step := range []float64{16, 8, 4, 2} {
		improved := true
		for improved {
			improved = false
			for _, m := range moves {
				dirs := [][2]float64{{step, 0}, {-step, 0}, {0, step}, {0, -step}}
				if m.xOnly {
					dirs = dirs[:2]
				}
				for _, d := range dirs {
					c, b := corners, barEdges
					for _, i := range m.corners {
						c[i] = geom.P(c[i].X+d[0], c[i].Y+d[1])
					}
					for _, i := range m.bars {
						b[i] = geom.P(b[i].X+d[0], b[i].Y+d[1])
					}
					if s := score(c, b); s > best {
						best, corners, barEdges = s, c, b
						improved = true
					}
				}
			}
		}
	}
	return corners, barEdges
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
// which are saturated but never surrounded by felt. It is exactly the first
// candidate of AutoColorCandidates; callers that can VERIFY a hypothesis
// (the correspondence fit) should iterate the candidates instead (#56).
func AutoColors(med *image.RGBA) (Colors, bool) {
	cands := AutoColorCandidates(med, 1)
	if len(cands) == 0 {
		return Colors{}, false
	}
	return cands[0], true
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

// OpeningScore scores a frame's read against the standard start with the
// given corners: the integer per-point score plus a fractional whole-board
// tie-break (see openingScore). Exported for the corpus bench, which scores
// automatic and manual calibrations at the same anchor frame.
func OpeningScore(frame image.Image, corners [4]geom.Pt, o Options) float64 {
	return openingScore(frame, corners, o)
}

// fitAtInstants runs the correspondence fit on apexes aggregated from
// several instants of the capture (ADR-0008 §7). Instants that cannot be
// decoded or yield too few apexes are simply skipped. seedCorners are
// source-space; the result is scaled back to source pixels.
func fitAtInstants(video string, ticks []int, o Options, seedCorners [4]geom.Pt, srcW, srcH int) (FitResult, bool) {
	detW := o.DetectW
	detH := srcH * detW / srcW
	sx := float64(srcW) / float64(detW)
	colors := o.Colors
	var sets [][]Apex
	var firstMed *image.RGBA
	for _, t := range ticks {
		med, err := MedianFrame(video, t, t+1500, 5, detW, detH)
		if err != nil {
			continue
		}
		if colors == (Colors{}) {
			c, ok := AutoColors(med)
			if !ok {
				continue
			}
			colors = c
		}
		mask := ColorMask(med, []color.RGBA{colors.PointA, colors.PointB}, o.ColorTol)
		mask = TriangleComponents(mask, med, colors.Felt, o.ColorTol, detW, detH)
		aps := ApexComponents(mask, detW, detH)
		if len(aps) >= 2*fitMinMatchesHalf {
			sets = append(sets, aps)
			if firstMed == nil {
				firstMed = med
			}
		}
	}
	if len(sets) == 0 {
		return FitResult{}, false
	}

	var det [4]geom.Pt
	for i := range seedCorners {
		det[i] = geom.P(seedCorners[i].X/sx, seedCorners[i].Y/sx)
	}
	pitch := math.Hypot(det[1].X-det[0].X, det[1].Y-det[0].Y) / 13
	merged := MergeApexes(sets, 0.3*pitch)

	leftFrac, rightFrac := 0.47, 0.53
	if cal, ok := calibrate.New(det, o.Canonical); ok && firstMed != nil {
		leftFrac, rightFrac = barFractions(cal.Rectify(firstMed), o.Canonical, colors, o.ColorTol)
	}
	tl, tr, br, bl := det[0], det[1], det[2], det[3]
	lerp := func(a, b geom.Pt, t float64) geom.Pt {
		return geom.P(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
	}
	seedB := [4]geom.Pt{lerp(tl, tr, leftFrac), lerp(tl, tr, rightFrac), lerp(bl, br, rightFrac), lerp(bl, br, leftFrac)}

	// Cascade: fit the FIRST instant alone (the settled opening — the
	// cleanest mask, where the seed-free bootstrap can index), then use its
	// result as the seed for an ICP pass over the merged union. Mid-game
	// instants carry spurious components (dice, cube) that break the
	// bootstrap's row alignment, but the seeded matching filters them by
	// radius and orientation. If the union fit fails, the single-instant
	// fit stands.
	res0, ok0 := FitApexes(sets[0], detW, detH, det, seedB, o.Canonical)
	if ok0 {
		det, seedB = res0.Corners, res0.BarEdges
	}
	res, ok := FitApexes(merged, detW, detH, det, seedB, o.Canonical)
	if !ok {
		res, ok = res0, ok0
	}
	if !ok {
		return FitResult{}, false
	}
	for i := range res.Corners {
		res.Corners[i] = geom.P(res.Corners[i].X*sx, res.Corners[i].Y*sx)
		res.BarEdges[i] = geom.P(res.BarEdges[i].X*sx, res.BarEdges[i].Y*sx)
	}
	if res.Lens.Norm > 0 {
		res.Lens.CenterX *= sx
		res.Lens.CenterY *= sx
		res.Lens.Norm *= sx
	}
	return res, true
}

// detectWithColors runs the mask → quad → bar → correspondence-fit chain for
// ONE color hypothesis, in detection space. ok is false when no plausible
// quad exists under these colors; fitOK says whether the correspondence fit
// verified the hypothesis (the caller prefers verified hypotheses).
func detectWithColors(med *image.RGBA, colors Colors, o Options, detW, detH int) (seedC, seedB [4]geom.Pt, lens calibrate.Lens, fitOK, ok bool) {
	mask := ColorMask(med, []color.RGBA{colors.PointA, colors.PointB}, o.ColorTol)
	mask = TriangleComponents(mask, med, colors.Felt, o.ColorTol, detW, detH)
	var quad [4]geom.Pt
	got := false
	if q, k := RowQuad(mask, detW, detH); k {
		quad, got = q, true
	} else if q, k := QuadFromMask(mask, detW, detH); k {
		quad, got = q, true
	}
	if !got {
		return seedC, seedB, lens, false, false
	}
	if !quadInBounds(quad, detW, detH) {
		quad = clampQuad(quad, detW, detH)
	}

	// Locate the bar as the central low-triangle-density valley in the quad's
	// own rectified frame; fall back to a centred default when it isn't clear.
	leftFrac, rightFrac := 0.47, 0.53
	if cal, k := calibrate.New(quad, o.Canonical); k {
		leftFrac, rightFrac = barFractions(cal.Rectify(med), o.Canonical, colors, o.ColorTol)
	}

	// Seed handles: the mask quad nudged outward toward the playing-surface
	// corners (the triangle mask sits inside by the margins), bar edges
	// riding the top/bottom edges at the detected fractions.
	seedC = expandQuad(quad, 0.045, 0.04)
	tl, tr, br, bl := seedC[0], seedC[1], seedC[2], seedC[3]
	lerp := func(a, b geom.Pt, t float64) geom.Pt {
		return geom.P(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
	}
	seedB = [4]geom.Pt{lerp(tl, tr, leftFrac), lerp(tl, tr, rightFrac), lerp(bl, br, rightFrac), lerp(bl, br, leftFrac)}

	// Correspondence fit (ADR-0008): sharpen the seed against the detected
	// triangle apexes and lateral edges — and, for auto-derived colors,
	// verify the hypothesis at all.
	if fit, k := FitHandles(mask, detW, detH, seedC, seedB, o.Canonical); k {
		return fit.Corners, fit.BarEdges, fit.Lens, true, true
	}
	return seedC, seedB, calibrate.Lens{}, false, true
}
