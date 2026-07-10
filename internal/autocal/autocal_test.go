package autocal

import (
	"image"
	"image/color"
	"math"
	"testing"

	"lazybg/internal/geom"
)

// paintQuad fills a convex quadrilateral region of an image with a color by
// scanline containment (test helper).
func paintQuad(img *image.RGBA, quad [4]geom.Pt, c color.RGBA) {
	inside := func(x, y float64) bool {
		sign := 0.0
		for i := 0; i < 4; i++ {
			a, b := quad[i], quad[(i+1)%4]
			cross := (b.X-a.X)*(y-a.Y) - (b.Y-a.Y)*(x-a.X)
			if cross != 0 {
				if sign == 0 {
					sign = cross
				} else if (cross > 0) != (sign > 0) {
					return false
				}
			}
		}
		return true
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if inside(float64(x)+0.5, float64(y)+0.5) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func TestColorMaskAndQuad_RecoverPaintedQuad(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	bgc := color.RGBA{60, 20, 20, 255}
	teal := color.RGBA{30, 110, 110, 255}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			img.SetRGBA(x, y, bgc)
		}
	}
	want := [4]geom.Pt{geom.P(50, 30), geom.P(270, 40), geom.P(260, 170), geom.P(40, 160)}
	paintQuad(img, want, teal)

	mask := ColorMask(img, []color.RGBA{teal}, 40)
	got, ok := QuadFromMask(mask, 320, 200)
	if !ok {
		t.Fatal("no quad found")
	}
	for i := range got {
		dx, dy := got[i].X-want[i].X, got[i].Y-want[i].Y
		if math.Hypot(dx, dy) > 4 {
			t.Errorf("corner %d = %v, want ≈%v", i, got[i], want[i])
		}
	}
}

func TestColorMask_ToleranceAndSpeckles(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	teal := color.RGBA{30, 110, 110, 255}
	near := color.RGBA{45, 100, 120, 255} // within tolerance
	far := color.RGBA{200, 110, 110, 255} // way off in red
	img.SetRGBA(5, 5, teal)
	img.SetRGBA(5, 6, near)
	img.SetRGBA(10, 10, far)

	mask := ColorMask(img, []color.RGBA{teal}, 40)
	if !mask[5*20+5] || !mask[6*20+5] {
		t.Error("in-tolerance pixels not masked")
	}
	if mask[10*20+10] {
		t.Error("out-of-tolerance pixel masked")
	}
}

func TestQuadFromMask_RejectsTinyMask(t *testing.T) {
	mask := make([]bool, 100*100)
	mask[50*100+50] = true // a single speck
	if _, ok := QuadFromMask(mask, 100, 100); ok {
		t.Error("a speck must not produce a quad")
	}
}

// TriangleComponents must keep the tall point triangles and drop blobs
// (clothing), specks (marbled-checker swirls) and small squares (the cube).
func TestTriangleComponents_FiltersOutliers(t *testing.T) {
	w, h := 640, 360
	felt := color.RGBA{168, 166, 162, 255}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{60, 20, 20, 255}) // dark table
		}
	}
	// felt zone hosting the triangles
	for y := 30; y < 160; y++ {
		for x := 80; x < 380; x++ {
			img.SetRGBA(x, y, felt)
		}
	}
	mask := make([]bool, w*h)
	rect := func(x0, y0, x1, y1 int) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				mask[y*w+x] = true
			}
		}
	}
	// 6 tall "triangles" (as thin rectangles) in a row on the felt
	for i := 0; i < 6; i++ {
		rect(100+i*40, 40, 100+i*40+22, 140)
	}
	rect(60, 0, 200, 26)     // wide clothing blob at the top edge (off felt)
	rect(500, 200, 516, 216) // the cube: small square on the dark table
	mask[300*w+30] = true    // speck

	kept := TriangleComponents(mask, img, felt, 45, w, h)
	// The union of kept pixels must span exactly the triangle row.
	quad, ok := QuadFromMask(kept, w, h)
	if !ok {
		t.Fatal("no quad")
	}
	if quad[0].Y < 35 || quad[0].X < 95 {
		t.Errorf("top-left %v pulled by the clothing blob", quad[0])
	}
	if quad[2].X > 330 || quad[2].Y > 145 {
		t.Errorf("bottom-right %v pulled by cube/speck", quad[2])
	}
}

// RowQuad must recover a ROTATED board's quad with the right corner order —
// the case extreme projections get wrong (they return a diamond of extreme
// pixels whose identity as TL/TR/BR/BL no longer holds under rotation).
func TestRowQuad_RecoversRotatedRows(t *testing.T) {
	w, h := 640, 360
	mask := make([]bool, w*h)
	deg := 12.0
	rad := deg * math.Pi / 180
	cx, cy := 320.0, 180.0
	rot := func(x, y float64) (int, int) {
		dx, dy := x-cx, y-cy
		return int(cx + dx*math.Cos(rad) - dy*math.Sin(rad)),
			int(cy + dx*math.Sin(rad) + dy*math.Cos(rad))
	}
	// two rows of 8 slim "triangles" (as in a board), then rotate every pixel
	for i := 0; i < 8; i++ {
		for _, rowY := range []float64{60, 230} {
			for y := 0.0; y < 70; y++ {
				for x := 0.0; x < 20; x++ {
					px, py := rot(140+float64(i)*45+x, rowY+y)
					if px >= 0 && px < w && py >= 0 && py < h {
						mask[py*w+px] = true
					}
				}
			}
		}
	}
	quad, ok := RowQuad(mask, w, h)
	if !ok {
		t.Fatal("no quad")
	}
	// Expected corners: the unrotated triangle-mass rect, rotated.
	wantRect := [][2]float64{{140, 60}, {475, 60}, {475, 300}, {140, 300}}
	for i, wc := range wantRect {
		ex, ey := rot(wc[0], wc[1])
		d := math.Hypot(quad[i].X-float64(ex), quad[i].Y-float64(ey))
		if d > 8 {
			t.Errorf("corner %d = %v, want ≈(%d,%d) (off %.1fpx)", i, quad[i], ex, ey, d)
		}
	}
}

// Too few components = no quad (fall back to the projection method).
func TestRowQuad_RejectsSparseMask(t *testing.T) {
	w, h := 640, 360
	mask := make([]bool, w*h)
	for y := 100; y < 160; y++ {
		for x := 100; x < 118; x++ {
			mask[y*w+x] = true
		}
	}
	if _, ok := RowQuad(mask, w, h); ok {
		t.Error("a single component must not produce a quad")
	}
}

func TestQuadInBounds(t *testing.T) {
	good := [4]geom.Pt{geom.P(10, 10), geom.P(600, 12), geom.P(590, 350), geom.P(15, 340)}
	if !quadInBounds(good, 640, 360) {
		t.Error("in-frame quad rejected")
	}
	wild := [4]geom.Pt{geom.P(32, 246), geom.P(668, -238), geom.P(1161, 395), geom.P(525, 881)}
	if quadInBounds(wild, 640, 360) {
		t.Error("runaway quad accepted")
	}
}
