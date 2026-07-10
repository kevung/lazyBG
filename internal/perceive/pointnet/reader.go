package pointnet

import (
	"image"
	"image/draw"
	"math"

	"lazybg/internal/calibrate"
	"lazybg/internal/perceive"
)

// Reader reads a rectified board with the learned per-point classifier — a
// drop-in for the classical shape-first reader. Confidence is the softmax
// probability of the winning class, which the fusion layer consumes exactly
// like any other cue confidence.
type Reader struct {
	Net *Net
}

// Read classifies every point region of a rectified board image.
func (r Reader) Read(src image.Image, cb calibrate.CanonicalBoard) perceive.ObservedBoard {
	img, ok := src.(*image.RGBA)
	if !ok {
		b := src.Bounds()
		img = image.NewRGBA(b)
		draw.Draw(img, b, src, b.Min, draw.Src)
	}
	var ob perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		region, _ := cb.PointRegion(p)
		x := cropToInput(img, region, p <= 12)
		logits := r.Net.Forward(x)
		cls, conf := softmaxTop(logits)
		ob.Points[p] = pointObsOf(cls, conf)
	}
	return ob
}

// pointObsOf decodes a class index (0 empty, 1..6 A×n, 7..12 B×n; 6/12 mean
// "6 or more") into a PointObs.
func pointObsOf(cls int, conf float64) perceive.PointObs {
	switch {
	case cls == 0:
		return perceive.PointObs{Count: 0, Side: perceive.None, Confidence: conf}
	case cls <= 6:
		return perceive.PointObs{Count: cls, Side: perceive.A, Confidence: conf}
	default:
		return perceive.PointObs{Count: cls - 6, Side: perceive.B, Confidence: conf}
	}
}

// cropToInput extracts a point region, optionally flips it vertically
// (bottom-half points stack from the bottom; the model always sees stacks
// growing from the top — ml/pointreader.json flip rule), and bilinearly
// resizes it to the network input, normalized to [0,1] CHW.
func cropToInput(img *image.RGBA, region image.Rectangle, flip bool) []float32 {
	region = region.Intersect(img.Bounds())
	rw, rh := region.Dx(), region.Dy()
	out := make([]float32, 3*InH*InW)
	if rw == 0 || rh == 0 {
		return out
	}
	sx := float64(rw) / float64(InW)
	sy := float64(rh) / float64(InH)
	for y := 0; y < InH; y++ {
		srcYf := (float64(y)+0.5)*sy - 0.5
		if flip {
			srcYf = float64(rh) - 1 - srcYf
		}
		y0 := int(math.Floor(srcYf))
		fy := srcYf - float64(y0)
		y1 := y0 + 1
		if y0 < 0 {
			y0, y1, fy = 0, 0, 0
		}
		if y1 >= rh {
			y1 = rh - 1
			if y0 > y1 {
				y0, fy = y1, 0
			}
		}
		for x := 0; x < InW; x++ {
			srcXf := (float64(x)+0.5)*sx - 0.5
			x0 := int(math.Floor(srcXf))
			fx := srcXf - float64(x0)
			x1 := x0 + 1
			if x0 < 0 {
				x0, x1, fx = 0, 0, 0
			}
			if x1 >= rw {
				x1 = rw - 1
				if x0 > x1 {
					x0, fx = x1, 0
				}
			}
			for c := 0; c < 3; c++ {
				v00 := float64(pix(img, region, x0, y0, c))
				v01 := float64(pix(img, region, x1, y0, c))
				v10 := float64(pix(img, region, x0, y1, c))
				v11 := float64(pix(img, region, x1, y1, c))
				v := v00*(1-fx)*(1-fy) + v01*fx*(1-fy) + v10*(1-fx)*fy + v11*fx*fy
				out[c*InH*InW+y*InW+x] = float32(v / 255.0)
			}
		}
	}
	return out
}

func pix(img *image.RGBA, region image.Rectangle, x, y, c int) uint8 {
	i := img.PixOffset(region.Min.X+x, region.Min.Y+y)
	return img.Pix[i+c]
}

func softmaxTop(logits []float32) (int, float64) {
	best := 0
	for i, v := range logits {
		if v > logits[best] {
			best = i
		}
	}
	var sum float64
	for _, v := range logits {
		sum += math.Exp(float64(v - logits[best]))
	}
	return best, 1 / sum
}
