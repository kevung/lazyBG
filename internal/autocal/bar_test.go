package autocal

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// barFractions must pin the bar to the actual felt gap — a wide, off-centre one
// here — not to the centred fallback, using the half-maximum density crossings.
func TestBarFractions_FindsOffCentreBar(t *testing.T) {
	cb := DefaultOptions().Canonical
	w, h := cb.Size()
	red := color.RGBA{220, 40, 40, 255}   // point A
	blue := color.RGBA{40, 40, 220, 255}  // point B
	green := color.RGBA{40, 200, 40, 255} // felt / bar

	barL, barR := int(0.40*float64(w)), int(0.56*float64(w))
	rect := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		col := red
		if x >= barL && x < barR {
			col = green // the bar gap: no point triangles
		} else if (x/cb.PointW)%2 == 1 {
			col = blue
		}
		for y := 0; y < h; y++ {
			rect.SetRGBA(x, y, col)
		}
	}

	lf, rf := barFractions(rect, cb, Colors{PointA: red, PointB: blue, Felt: green}, 40)
	wantL, wantR := float64(barL)/float64(w), float64(barR)/float64(w)
	if math.Abs(lf-wantL) > 0.02 || math.Abs(rf-wantR) > 0.02 {
		t.Errorf("bar fractions = %.3f..%.3f, want ~%.3f..%.3f", lf, rf, wantL, wantR)
	}
	// And clearly not the centred fallback.
	if math.Abs(lf-0.47) < 0.01 && math.Abs(rf-0.53) < 0.01 {
		t.Error("fell back to the centred default instead of detecting the bar")
	}
}
