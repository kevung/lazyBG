package calibrate

import (
	"image"
	"image/color"
	"math"
	"testing"

	"lazybg/internal/geom"
)

func TestCanonical_PointRegions(t *testing.T) {
	cb := DefaultCanonical()
	w, h := cb.Size()

	tests := []struct {
		p    int
		want image.Rectangle
		dir  StackDir
	}{
		{13, image.Rect(20, 20, 80, 380), StackDown},   // top-left
		{24, image.Rect(720, 20, 780, 380), StackDown}, // top-right
		{12, image.Rect(20, 420, 80, 780), StackUp},    // bottom-left
		{1, image.Rect(720, 420, 780, 780), StackUp},   // bottom-right
	}
	for _, tc := range tests {
		got, dir := cb.PointRegion(tc.p)
		if got != tc.want || dir != tc.dir {
			t.Errorf("PointRegion(%d) = %v,%v; want %v,%v", tc.p, got, dir, tc.want, tc.dir)
		}
	}

	// The bar gutter sits exactly between point 18 and point 19 (columns 5|6).
	p18, _ := cb.PointRegion(18)
	p19, _ := cb.PointRegion(19)
	bar := cb.BarRegion()
	if p18.Max.X != bar.Min.X || p19.Min.X != bar.Max.X {
		t.Errorf("bar %v should sit between p18 %v and p19 %v", bar, p18, p19)
	}

	// Everything stays inside the canonical image.
	for p := 1; p <= 24; p++ {
		r, _ := cb.PointRegion(p)
		if r.Min.X < 0 || r.Min.Y < 0 || r.Max.X > w || r.Max.Y > h {
			t.Errorf("PointRegion(%d)=%v escapes board %dx%d", p, r, w, h)
		}
	}
}

func TestNew_ToCanonical_MapsCornersBack(t *testing.T) {
	cb := DefaultCanonical()
	w, h := cb.Size()
	src := [4]geom.Pt{geom.P(50, 40), geom.P(800, 20), geom.P(830, 760), geom.P(20, 700)} // an arbitrary camera quad
	c, ok := New(src, cb)
	if !ok {
		t.Fatal("New failed")
	}
	canon := [4]geom.Pt{geom.P(0, 0), geom.P(float64(w), 0), geom.P(float64(w), float64(h)), geom.P(0, float64(h))}
	for i := range src {
		got := c.ToCanonical(src[i])
		if math.Abs(got.X-canon[i].X) > 1e-3 || math.Abs(got.Y-canon[i].Y) > 1e-3 {
			t.Errorf("corner %d ToCanonical = %v, want %v", i, got, canon[i])
		}
	}
}

// With identity corners, Rectify must reproduce the source image pixel-for-pixel.
func TestRectify_IdentityReproducesSource(t *testing.T) {
	cb := DefaultCanonical()
	w, h := cb.Size()
	srcImg := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcImg.SetRGBA(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 100, 255})
		}
	}
	corners := [4]geom.Pt{geom.P(0, 0), geom.P(float64(w), 0), geom.P(float64(w), float64(h)), geom.P(0, float64(h))}
	c, ok := New(corners, cb)
	if !ok {
		t.Fatal("New failed")
	}
	out := c.Rectify(srcImg)

	for _, p := range []image.Point{{10, 10}, {400, 300}, {w - 2, h - 2}, {123, 456}} {
		if got, want := out.RGBAAt(p.X, p.Y), srcImg.RGBAAt(p.X, p.Y); got != want {
			t.Errorf("Rectify identity at %v = %v, want %v", p, got, want)
		}
	}
}
