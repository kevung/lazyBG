// Package boardstate reads an ObservedBoard from a rectified board image using
// classical color-segmentation + stack counting (the calibrated-classical-first
// MVP approach — docs/architecture.md §3, CLAUDE.md §3). It needs no training
// data: with the declared checker colors and the canonical grid, it counts
// checkers per point by measuring the colored run along each point's stack axis.
package boardstate

import (
	"image"
	"image/color"
	"math"

	"lazybg/internal/calibrate"
	"lazybg/internal/perceive"
	"lazybg/internal/profile"
)

// DefaultColorDist is the maximum RGB Euclidean distance for a pixel to be
// classified as a given checker color.
const DefaultColorDist = 80.0

// Reader counts checkers on a rectified board.
type Reader struct {
	Profile   profile.CaptureProfile
	ColorDist float64 // 0 -> DefaultColorDist
}

// Read produces an ObservedBoard for points 1..24 from the rectified image.
func (r Reader) Read(img image.Image, cb calibrate.CanonicalBoard) perceive.ObservedBoard {
	dist := r.ColorDist
	if dist == 0 {
		dist = DefaultColorDist
	}
	var ob perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		region, dir := cb.PointRegion(p)
		ob.Points[p] = r.readPoint(img, region, dir, cb.PointW, dist)
	}
	return ob
}

// readPoint scans the vertical centerline of a point's region, tallies checker
// pixels by color, and infers the count as the colored run length divided by the
// checker diameter (PointW).
func (r Reader) readPoint(img image.Image, region image.Rectangle, dir calibrate.StackDir, diameter int, dist float64) perceive.PointObs {
	cx := (region.Min.X + region.Max.X) / 2

	ys := make([]int, 0, region.Dy())
	if dir == calibrate.StackDown {
		for y := region.Min.Y; y < region.Max.Y; y++ {
			ys = append(ys, y)
		}
	} else {
		for y := region.Max.Y - 1; y >= region.Min.Y; y-- {
			ys = append(ys, y)
		}
	}

	var aCount, bCount int
	for _, y := range ys {
		switch r.classify(img.At(cx, y), dist) {
		case perceive.A:
			aCount++
		case perceive.B:
			bCount++
		}
	}
	colored := aCount + bCount

	if colored == 0 {
		return perceive.PointObs{Count: 0, Side: perceive.None, Confidence: 1}
	}

	side := perceive.A
	dom := aCount
	if bCount > aCount {
		side, dom = perceive.B, bCount
	}

	fraction := float64(colored) / float64(diameter)
	count := int(fraction + 0.5)
	if count == 0 { // a sliver of color that doesn't make a whole checker
		return perceive.PointObs{Count: 0, Side: perceive.None, Confidence: clamp01(1 - fraction)}
	}

	// Confidence blends color purity with how cleanly the run divides into whole
	// checkers. Calibration of these signals is deferred (architecture §4).
	purity := float64(dom) / float64(colored)
	integ := 1 - 2*math.Abs(fraction-float64(count))
	conf := clamp01(purity * clamp01(integ))
	return perceive.PointObs{Count: count, Side: side, Confidence: conf}
}

// classify returns the nearest checker color within dist, or None.
func (r Reader) classify(c color.Color, dist float64) perceive.Side {
	dA := colorDist(c, r.Profile.CheckerA)
	dB := colorDist(c, r.Profile.CheckerB)
	if dA <= dB && dA <= dist {
		return perceive.A
	}
	if dB < dA && dB <= dist {
		return perceive.B
	}
	return perceive.None
}

func colorDist(c color.Color, ref color.RGBA) float64 {
	r, g, b, _ := c.RGBA()
	dr := float64(uint8(r>>8)) - float64(ref.R)
	dg := float64(uint8(g>>8)) - float64(ref.G)
	db := float64(uint8(b>>8)) - float64(ref.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
