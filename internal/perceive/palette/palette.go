// Package palette measures a capture's board colours from a rectified board
// image — the two point-triangle colours, the felt, and the two checker
// colours (issue #64, ADR-0009 grilling).
//
// Why measure what the user already declares: the declared checker colours are
// not decoration. boardstate classifies every checker by colour distance to
// them, and they currently come from a human guessing a hex on a monitor.
// Measuring them improves perception, and painting the reconstructed board with
// the same palette as the video means the only thing left differing between the
// two panels is the reading error the user is hunting.
//
// Geometry does the work, not frame-wide heuristics: on a rectified board every
// pixel's role is known. The apex end of a point region is triangle, the middle
// band is felt, and whatever sits at the outer end and matches none of those is
// a checker.
package palette

import (
	"image"
	"image/color"
	"sort"

	"lazybg/internal/calibrate"
)

// Palette is a capture's measured board colours.
type Palette struct {
	PointA, PointB color.RGBA // the two alternating triangle colours
	Felt           color.RGBA // the playing surface
	CheckerA       color.RGBA // the declared Player 1 colour, refined
	CheckerB       color.RGBA // the declared Player 2 colour, refined
	// HasCheckers reports that two checker clusters were actually found. An
	// empty board (or one frame where the reader sees none) still yields a
	// usable board palette, and the caller must not silently keep the
	// declared colours as if they had been measured.
	HasCheckers bool
}

// Sample measures the palette of a rectified board. declA/declB are the
// currently declared checker colours: the two measured clusters are assigned to
// whichever of them they are closest to, so sampling REFINES the declaration
// and never re-decides which player is which — that is the swap gesture's job
// (ADR-0009). ok is false only when the image is unusable.
func Sample(rect *image.RGBA, cb calibrate.CanonicalBoard, declA, declB color.RGBA) (Palette, bool) {
	if rect == nil || rect.Bounds().Empty() {
		return Palette{}, false
	}
	w, h := cb.Size()
	b := rect.Bounds()
	if b.Dx() < w || b.Dy() < h {
		return Palette{}, false
	}
	var out Palette
	out.Felt = feltColor(rect, cb)

	// Triangles alternate by point number, so the odd and even points give the
	// two colours directly. Sampling the apex third keeps checkers out of it:
	// stacks grow from the outer edge, and 5 checkers deep still leaves the
	// inner third clear.
	odd := newCluster()
	even := newCluster()
	for p := 1; p <= 24; p++ {
		region, dir := cb.PointRegion(p)
		apex := apexThird(region, dir)
		c := odd
		if p%2 == 0 {
			c = even
		}
		forEachPixel(rect, apex, func(px color.RGBA) {
			// The triangle narrows to a tip, so the apex box holds felt too.
			if closeTo(px, out.Felt, 26) {
				return
			}
			c.add(px)
		})
	}
	out.PointA = odd.dominant(out.Felt)
	out.PointB = even.dominant(out.Felt)

	a, bb, found := checkerColors(rect, cb, out)
	if !found {
		out.CheckerA, out.CheckerB = declA, declB
		return out, true
	}
	// Assign the two measured clusters to the two declared colours, whichever
	// pairing is closer overall.
	if dist2(a, declA)+dist2(bb, declB) <= dist2(a, declB)+dist2(bb, declA) {
		out.CheckerA, out.CheckerB = a, bb
	} else {
		out.CheckerA, out.CheckerB = bb, a
	}
	out.HasCheckers = true
	return out, true
}

// feltColor is the dominant colour of the horizontal band between the two rows:
// a stack of checkers never reaches it, and no triangle tip crosses it, so it
// is bare surface whatever the position on the board.
func feltColor(rect *image.RGBA, cb calibrate.CanonicalBoard) color.RGBA {
	w, h := cb.Size()
	band := image.Rect(cb.MarginX, cb.MarginY+cb.QuadH, w-cb.MarginX-cb.OffW, h-cb.MarginY-cb.QuadH)
	c := newCluster()
	forEachPixel(rect, band, func(px color.RGBA) { c.add(px) })
	return c.dominant(color.RGBA{})
}

// apexThird is the inner third of a point region — the tip of the triangle,
// the part a stack of checkers never reaches.
func apexThird(region image.Rectangle, dir calibrate.StackDir) image.Rectangle {
	third := region.Dy() / 3
	if dir == calibrate.StackDown {
		return image.Rect(region.Min.X, region.Max.Y-third, region.Max.X, region.Max.Y)
	}
	return image.Rect(region.Min.X, region.Min.Y, region.Max.X, region.Min.Y+third)
}

// checkerColors clusters the pixels at the outer end of every point — where
// stacks sit — after dropping everything that matches the board itself. The two
// largest distinct clusters are the two checker colours.
func checkerColors(rect *image.RGBA, cb calibrate.CanonicalBoard, p Palette) (a, b color.RGBA, ok bool) {
	c := newCluster()
	for pt := 1; pt <= 24; pt++ {
		region, dir := cb.PointRegion(pt)
		depth := 5 * cb.PointW
		if depth > region.Dy() {
			depth = region.Dy()
		}
		outer := image.Rect(region.Min.X, region.Min.Y, region.Max.X, region.Min.Y+depth)
		if dir == calibrate.StackUp {
			outer = image.Rect(region.Min.X, region.Max.Y-depth, region.Max.X, region.Max.Y)
		}
		forEachPixel(rect, outer, func(px color.RGBA) {
			if closeTo(px, p.Felt, 30) || closeTo(px, p.PointA, 30) || closeTo(px, p.PointB, 30) {
				return
			}
			c.add(px)
		})
	}
	tops := c.top(2)
	if len(tops) < 2 {
		return a, b, false
	}
	return tops[0], tops[1], true
}

// --- colour clustering -------------------------------------------------
//
// Quantized-bin voting, the same shape autocal uses for its felt/point
// hypotheses: coarse enough to survive JPEG noise and lighting gradients,
// fine enough to keep two checker colours apart.

const binSize = 24

type binKey struct{ r, g, b int }

type cluster struct {
	count map[binKey]int
	sum   map[binKey][3]int
}

func newCluster() *cluster {
	return &cluster{count: map[binKey]int{}, sum: map[binKey][3]int{}}
}

func (c *cluster) add(px color.RGBA) {
	k := binKey{int(px.R) / binSize, int(px.G) / binSize, int(px.B) / binSize}
	c.count[k]++
	s := c.sum[k]
	s[0] += int(px.R)
	s[1] += int(px.G)
	s[2] += int(px.B)
	c.sum[k] = s
}

func (c *cluster) mean(k binKey) color.RGBA {
	n := c.count[k]
	if n == 0 {
		return color.RGBA{}
	}
	s := c.sum[k]
	return color.RGBA{uint8(s[0] / n), uint8(s[1] / n), uint8(s[2] / n), 255}
}

// keysByCount lists the bins from most to least populated, ties broken
// lexicographically so map iteration order never decides a colour.
func (c *cluster) keysByCount() []binKey {
	keys := make([]binKey, 0, len(c.count))
	for k := range c.count {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if c.count[a] != c.count[b] {
			return c.count[a] > c.count[b]
		}
		if a.r != b.r {
			return a.r < b.r
		}
		if a.g != b.g {
			return a.g < b.g
		}
		return a.b < b.b
	})
	return keys
}

// dominant returns the most populated bin's mean colour, skipping bins that
// are a quantization neighbour of avoid (pass the zero colour to skip nothing).
func (c *cluster) dominant(avoid color.RGBA) color.RGBA {
	av := binKey{int(avoid.R) / binSize, int(avoid.G) / binSize, int(avoid.B) / binSize}
	skip := avoid.A != 0
	for _, k := range c.keysByCount() {
		if skip && neighbors(k, av) {
			continue
		}
		return c.mean(k)
	}
	return color.RGBA{}
}

// top returns up to n dominant colours that are not quantization neighbours of
// one another — two shades of the same physical surface must not both win.
func (c *cluster) top(n int) []color.RGBA {
	var keys []binKey
	for _, k := range c.keysByCount() {
		if c.count[k] < 32 { // a handful of edge pixels is not a checker
			continue
		}
		dup := false
		for _, o := range keys {
			if neighbors(k, o) {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		keys = append(keys, k)
		if len(keys) == n {
			break
		}
	}
	out := make([]color.RGBA, len(keys))
	for i, k := range keys {
		out[i] = c.mean(k)
	}
	return out
}

func neighbors(a, b binKey) bool {
	return abs(a.r-b.r) <= 1 && abs(a.g-b.g) <= 1 && abs(a.b-b.b) <= 1
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func dist2(a, b color.RGBA) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	return dr*dr + dg*dg + db*db
}

func closeTo(a, b color.RGBA, tol int) bool { return dist2(a, b) <= tol*tol }

func forEachPixel(img *image.RGBA, r image.Rectangle, fn func(color.RGBA)) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := img.PixOffset(x, y)
			fn(color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 255})
		}
	}
}
