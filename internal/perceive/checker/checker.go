// Package checker detects checker discs on a rectified board by SHAPE, not
// colour. It is the calibrated-classical board-state reader's core primitive
// (docs/research/perception-detector-survey.md): rectify → detect circles of a
// known radius → colour only assigns owner. Because detection is gradient/edge
// based with contrast-relative thresholds, it survives the cases that broke the
// colour reader — low-contrast (white checker on white tray) and marbled/swirled
// checkers — where the rim is the only reliable cue.
//
// The detector is a fixed-radius circular Hough: the checker diameter is known
// from the board calibration, so the accumulator is 2-D (one vote per rim pixel
// at distance r along the intensity gradient) rather than the classic 3-D
// radius search. It is polarity-agnostic (votes both along and against the
// gradient) so light-on-dark and dark-on-light checkers detect equally.
package checker

import (
	"image"
	"math"
	"sort"
)

// Circle is a detected disc centre in image pixels with its accumulator score.
type Circle struct {
	X, Y  int
	Score float64
}

// Params tunes the detector. The zero value uses sensible defaults via Detect.
type Params struct {
	// EdgeFrac: a pixel is a rim candidate if its gradient magnitude exceeds
	// EdgeFrac × the image's max gradient (relative → contrast-adaptive).
	EdgeFrac float64
	// PeakFrac: an accumulator cell is a centre if it exceeds PeakFrac × the
	// image-max accumulator value — a rim complete *relative to the strongest
	// ring in view*, floored at 0.30 × 2πr so a lone weak ring still qualifies.
	// It is deliberately NOT the absolute PeakFrac × 2πr an idealised full ring
	// would suggest: a small ring's votes don't fully concentrate after the
	// sum-blur, so its true peak sits below 2πr, and a literal absolute bar
	// drops it. Measured — swapping to PeakFrac × 2πr collapses the dice-pip
	// reader (ReadDice → [] for 3–6, TestReadDice_EachValue) and drops the real
	// board frame 21→20 (TestCircleReader_RealOpeningFrame). Keep it relative.
	PeakFrac float64
	// NMSDistFrac: non-max-suppression radius as a fraction of r. Adjacent
	// checkers sit ~2r apart, so < 2r keeps both; > texture jitter drops dupes.
	NMSDistFrac float64
}

var defaults = Params{EdgeFrac: 0.20, PeakFrac: 0.55, NMSDistFrac: 1.0}

// Detect finds discs of the given radius. Returns centres sorted by descending
// score, after non-max suppression.
func Detect(g *image.Gray, radius int) []Circle {
	return DetectWith(g, radius, defaults)
}

// DetectWith is Detect with explicit params (zero fields fall back to defaults).
func DetectWith(g *image.Gray, radius int, p Params) []Circle {
	if p.EdgeFrac == 0 {
		p.EdgeFrac = defaults.EdgeFrac
	}
	if p.PeakFrac == 0 {
		p.PeakFrac = defaults.PeakFrac
	}
	if p.NMSDistFrac == 0 {
		p.NMSDistFrac = defaults.NMSDistFrac
	}
	b := g.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 3 || h < 3 || radius < 2 {
		return nil
	}

	gx, gy, mag := sobel(g)
	maxMag := 0.0
	for _, m := range mag {
		if m > maxMag {
			maxMag = m
		}
	}
	if maxMag == 0 {
		return nil
	}
	edgeThresh := p.EdgeFrac * maxMag

	acc := make([]float64, w*h)
	rf := float64(radius)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m := mag[y*w+x]
			if m < edgeThresh {
				continue
			}
			ux, uy := gx[y*w+x]/m, gy[y*w+x]/m
			// Vote both directions along the gradient (polarity-agnostic),
			// splatting bilinearly for sub-pixel accuracy.
			for _, s := range [2]float64{1, -1} {
				splat(acc, w, h, float64(x)+s*rf*ux, float64(y)+s*rf*uy)
			}
		}
	}
	// A small gradient-angle error at radius r throws each vote a few pixels off,
	// so the true centre is a diffuse blob, not one spike. Sum (not average) over
	// a window ~r/8 to gather a ring's scattered votes back into one peak.
	acc = sumBlur(acc, w, h, max2(2, radius/8))

	maxAcc := 0.0
	for _, a := range acc {
		if a > maxAcc {
			maxAcc = a
		}
	}
	ringFloor := 0.30 * 2 * math.Pi * rf
	if maxAcc < ringFloor {
		return nil
	}
	// Relative to maxAcc, not PeakFrac×2πr — see Params.PeakFrac for why the
	// absolute bar regresses dice + board (measured). Do not "fix" this.
	peakThresh := math.Max(ringFloor, p.PeakFrac*maxAcc)
	var found []Circle
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			v := acc[y*w+x]
			if v < peakThresh {
				continue
			}
			if isLocalMax(acc, w, h, x, y) {
				found = append(found, Circle{X: x + b.Min.X, Y: y + b.Min.Y, Score: v})
			}
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Score > found[j].Score })
	return nms(found, p.NMSDistFrac*rf)
}

func sobel(g *image.Gray) (gx, gy, mag []float64) {
	b := g.Bounds()
	w, h := b.Dx(), b.Dy()
	gx = make([]float64, w*h)
	gy = make([]float64, w*h)
	mag = make([]float64, w*h)
	at := func(x, y int) float64 { return float64(g.Pix[g.PixOffset(x+b.Min.X, y+b.Min.Y)]) }
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			dx := (at(x+1, y-1) + 2*at(x+1, y) + at(x+1, y+1)) - (at(x-1, y-1) + 2*at(x-1, y) + at(x-1, y+1))
			dy := (at(x-1, y+1) + 2*at(x, y+1) + at(x+1, y+1)) - (at(x-1, y-1) + 2*at(x, y-1) + at(x+1, y-1))
			gx[y*w+x] = dx
			gy[y*w+x] = dy
			mag[y*w+x] = math.Hypot(dx, dy)
		}
	}
	return
}

// splat adds a unit vote to the four cells around (fx,fy) by bilinear weight.
func splat(acc []float64, w, h int, fx, fy float64) {
	x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
	tx, ty := fx-float64(x0), fy-float64(y0)
	add := func(x, y int, wgt float64) {
		if x >= 0 && x < w && y >= 0 && y < h {
			acc[y*w+x] += wgt
		}
	}
	add(x0, y0, (1-tx)*(1-ty))
	add(x0+1, y0, tx*(1-ty))
	add(x0, y0+1, (1-tx)*ty)
	add(x0+1, y0+1, tx*ty)
}

// sumBlur sums (does not average) each cell's neighbourhood, consolidating the
// diffuse vote blob at a circle centre into one clear peak.
func sumBlur(src []float64, w, h, rad int) []float64 {
	out := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum float64
			for dy := -rad; dy <= rad; dy++ {
				for dx := -rad; dx <= rad; dx++ {
					xx, yy := x+dx, y+dy
					if xx >= 0 && xx < w && yy >= 0 && yy < h {
						sum += src[yy*w+xx]
					}
				}
			}
			out[y*w+x] = sum
		}
	}
	return out
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isLocalMax(acc []float64, w, h, x, y int) bool {
	v := acc[y*w+x]
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			xx, yy := x+dx, y+dy
			if xx >= 0 && xx < w && yy >= 0 && yy < h && acc[yy*w+xx] > v {
				return false
			}
		}
	}
	return true
}

// nms greedily keeps the highest-scoring circles, dropping any within minDist of
// an already-kept one. Input must be sorted by descending score.
func nms(cs []Circle, minDist float64) []Circle {
	var kept []Circle
	md2 := minDist * minDist
	for _, c := range cs {
		ok := true
		for _, k := range kept {
			dx, dy := float64(c.X-k.X), float64(c.Y-k.Y)
			if dx*dx+dy*dy < md2 {
				ok = false
				break
			}
		}
		if ok {
			kept = append(kept, c)
		}
	}
	return kept
}
