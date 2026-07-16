package dienet

import (
	"image"
	"math"
)

// Classify reads one die-box crop: bilinear-resize to the network input,
// forward, softmax. Returns the top-face value (1..6) with its softmax
// confidence, or (0, conf) when the crop is junk — not a single readable die
// (the class the hand-labeled trainer rejects with ~99% recall).
func Classify(net *Net, crop image.Image) (int, float64) {
	b := crop.Bounds()
	x := make([]float32, 3*In*In)
	sx := float64(b.Dx()) / In
	sy := float64(b.Dy()) / In
	for y := 0; y < In; y++ {
		for xx := 0; xx < In; xx++ {
			fx := (float64(xx)+0.5)*sx - 0.5
			fy := (float64(y)+0.5)*sy - 0.5
			r, g, bl := bilinearRGB(crop, fx, fy)
			i := y*In + xx
			x[0*In*In+i] = r
			x[1*In*In+i] = g
			x[2*In*In+i] = bl
		}
	}
	logits := net.Forward(x)
	best, sum := 0, 0.0
	var mx float32
	for i, v := range logits {
		if v > logits[best] {
			best = i
		}
		if v > mx {
			mx = v
		}
	}
	for _, v := range logits {
		sum += math.Exp(float64(v - mx))
	}
	conf := math.Exp(float64(logits[best]-mx)) / sum
	return best, conf
}

func bilinearRGB(img image.Image, fx, fy float64) (float32, float32, float32) {
	b := img.Bounds()
	clamp := func(v, lo, hi int) int {
		if v < lo {
			return lo
		}
		if v > hi {
			return hi
		}
		return v
	}
	x0 := clamp(int(math.Floor(fx)), 0, b.Dx()-1)
	y0 := clamp(int(math.Floor(fy)), 0, b.Dy()-1)
	x1 := clamp(x0+1, 0, b.Dx()-1)
	y1 := clamp(y0+1, 0, b.Dy()-1)
	ax := fx - math.Floor(fx)
	ay := fy - math.Floor(fy)
	if fx < 0 {
		ax = 0
	}
	if fy < 0 {
		ay = 0
	}
	at := func(x, y int) (float64, float64, float64) {
		r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return float64(r >> 8), float64(g >> 8), float64(bl >> 8)
	}
	r00, g00, b00 := at(x0, y0)
	r10, g10, b10 := at(x1, y0)
	r01, g01, b01 := at(x0, y1)
	r11, g11, b11 := at(x1, y1)
	lerp2 := func(v00, v10, v01, v11 float64) float32 {
		top := v00 + (v10-v00)*ax
		bot := v01 + (v11-v01)*ax
		return float32((top + (bot-top)*ay) / 255)
	}
	return lerp2(r00, r10, r01, r11), lerp2(g00, g10, g01, g11), lerp2(b00, b10, b01, b11)
}
