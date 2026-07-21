package autocal

import (
	"math"
	"testing"

	"lazybg/internal/calibrate"
	"lazybg/internal/geom"
)

// truthHandles is a plausible detection-space board: perspective (right side
// smaller), slight tilt, bar off centre.
func truthHandles() (corners, barEdges [4]geom.Pt) {
	corners = [4]geom.Pt{geom.P(80, 40), geom.P(560, 60), geom.P(548, 330), geom.P(70, 320)}
	barEdges = [4]geom.Pt{geom.P(300, 49), geom.P(330, 50), geom.P(324, 325), geom.P(295, 324)}
	return
}

// renderApexMask rasterizes the 24 point triangles of the ground-truth
// calibration into a detection-space mask, skipping the points listed in
// omit (simulating fully occluded triangles).
func renderApexMask(t *testing.T, w, h int, cal calibrate.BoardCalibration, cb calibrate.CanonicalBoard, omit map[int]bool) []bool {
	t.Helper()
	mask := make([]bool, w*h)
	_, ch := cb.Size()
	for p := 1; p <= 24; p++ {
		if omit[p] {
			continue
		}
		r, dir := cb.PointRegion(p)
		outerY := float64(r.Min.Y)
		if dir == calibrate.StackUp {
			outerY = float64(r.Max.Y)
		}
		_ = ch
		apex := cal.ToSource(cb.PointApex(p))
		b1 := cal.ToSource(geom.P(float64(r.Min.X)+2, outerY))
		b2 := cal.ToSource(geom.P(float64(r.Max.X)-2, outerY))
		fillTriangle(mask, w, h, apex, b1, b2)
	}
	return mask
}

func fillTriangle(mask []bool, w, h int, a, b, c geom.Pt) {
	minX := int(math.Min(a.X, math.Min(b.X, c.X)))
	maxX := int(math.Max(a.X, math.Max(b.X, c.X))) + 1
	minY := int(math.Min(a.Y, math.Min(b.Y, c.Y)))
	maxY := int(math.Max(a.Y, math.Max(b.Y, c.Y))) + 1
	den := (b.Y-c.Y)*(a.X-c.X) + (c.X-b.X)*(a.Y-c.Y)
	if den == 0 {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if x < 0 || x >= w || y < 0 || y >= h {
				continue
			}
			px, py := float64(x)+0.5, float64(y)+0.5
			l1 := ((b.Y-c.Y)*(px-c.X) + (c.X-b.X)*(py-c.Y)) / den
			l2 := ((c.Y-a.Y)*(px-c.X) + (a.X-c.X)*(py-c.Y)) / den
			l3 := 1 - l1 - l2
			if l1 >= 0 && l2 >= 0 && l3 >= 0 {
				mask[y*w+x] = true
			}
		}
	}
}

func perturb(pts [4]geom.Pt, d [4][2]float64) [4]geom.Pt {
	for i := range pts {
		pts[i] = geom.P(pts[i].X+d[i][0], pts[i].Y+d[i][1])
	}
	return pts
}

func TestFitHandles_RecoversTruthFromPerturbedSeed(t *testing.T) {
	w, h := 640, 360
	cb := calibrate.DefaultCanonical()
	tc, tb := truthHandles()
	cal, ok := calibrate.NewSplit([8]geom.Pt{tc[0], tc[1], tc[2], tc[3], tb[0], tb[1], tb[2], tb[3]}, cb)
	if !ok {
		t.Fatal("truth calibration failed")
	}
	mask := renderApexMask(t, w, h, cal, cb, nil)

	seedC := perturb(tc, [4][2]float64{{9, -7}, {-11, 6}, {8, 9}, {-6, -10}})
	seedB := perturb(tb, [4][2]float64{{12, 2}, {12, 1}, {13, -2}, {12, 0}})
	res, ok := FitHandles(mask, w, h, seedC, seedB, cb)
	if !ok {
		t.Fatalf("fit failed (matches=%d resid=%.2f)", res.Matches, res.Resid)
	}
	// 2px: the renderer insets triangle bases by 2 canonical px (real boards
	// have a small gap too) and the mask quantizes to pixel centres, so exact
	// truth recovery is impossible by construction; 2px in detection space is
	// still ~an order sharper than the mask-extremes seed.
	for i := range tc {
		if d := math.Hypot(res.Corners[i].X-tc[i].X, res.Corners[i].Y-tc[i].Y); d > 2.0 {
			t.Errorf("corner %d off by %.2fpx (%v vs %v)", i, d, res.Corners[i], tc[i])
		}
	}
	for i := range tb {
		if d := math.Hypot(res.BarEdges[i].X-tb[i].X, res.BarEdges[i].Y-tb[i].Y); d > 2.5 {
			t.Errorf("bar edge %d off by %.2fpx (%v vs %v)", i, d, res.BarEdges[i], tb[i])
		}
	}
	if res.Matches < 20 {
		t.Errorf("only %d matches, want >= 20", res.Matches)
	}
}

func TestFitHandles_ToleratesOccludedTriangles(t *testing.T) {
	w, h := 640, 360
	cb := calibrate.DefaultCanonical()
	tc, tb := truthHandles()
	cal, _ := calibrate.NewSplit([8]geom.Pt{tc[0], tc[1], tc[2], tc[3], tb[0], tb[1], tb[2], tb[3]}, cb)
	// Standard-start-ish occlusion: checkers swallow several whole triangles.
	omit := map[int]bool{6: true, 13: true, 24: true, 8: true, 19: true}
	mask := renderApexMask(t, w, h, cal, cb, omit)

	seedC := perturb(tc, [4][2]float64{{-8, 8}, {10, -6}, {-9, -8}, {7, 9}})
	seedB := perturb(tb, [4][2]float64{{-10, 0}, {-10, 2}, {-11, 1}, {-10, -1}})
	res, ok := FitHandles(mask, w, h, seedC, seedB, cb)
	if !ok {
		t.Fatalf("fit failed (matches=%d)", res.Matches)
	}
	for i := range tc {
		if d := math.Hypot(res.Corners[i].X-tc[i].X, res.Corners[i].Y-tc[i].Y); d > 2.0 {
			t.Errorf("corner %d off by %.2fpx with occlusions", i, d)
		}
	}
}

func TestFitHandles_FailsCleanlyOnEmptyMask(t *testing.T) {
	w, h := 640, 360
	cb := calibrate.DefaultCanonical()
	tc, tb := truthHandles()
	if _, ok := FitHandles(make([]bool, w*h), w, h, tc, tb, cb); ok {
		t.Fatal("empty mask must not produce a fit")
	}
}

func TestFitHandles_BootstrapsFromGarbageSeed(t *testing.T) {
	// The real corpus failure: RowQuad's principal axis goes wild and the
	// seed is a rotated diamond hundreds of px off. The apexes are still
	// good, so the seed-free bootstrap indexing must rescue the fit.
	w, h := 640, 360
	cb := calibrate.DefaultCanonical()
	tc, tb := truthHandles()
	cal, _ := calibrate.NewSplit([8]geom.Pt{tc[0], tc[1], tc[2], tc[3], tb[0], tb[1], tb[2], tb[3]}, cb)
	mask := renderApexMask(t, w, h, cal, cb, map[int]bool{6: true, 20: true})

	seedC := [4]geom.Pt{geom.P(320, -80), geom.P(600, 180), geom.P(320, 420), geom.P(40, 180)} // diamond
	seedB := [4]geom.Pt{geom.P(440, 40), geom.P(470, 60), geom.P(200, 320), geom.P(170, 300)}
	res, ok := FitHandles(mask, w, h, seedC, seedB, cb)
	if !ok {
		t.Fatalf("bootstrap fit failed (matches=%d resid=%.2f)", res.Matches, res.Resid)
	}
	for i := range tc {
		if d := math.Hypot(res.Corners[i].X-tc[i].X, res.Corners[i].Y-tc[i].Y); d > 2.5 {
			t.Errorf("corner %d off by %.2fpx from garbage seed (%v vs %v)", i, d, res.Corners[i], tc[i])
		}
	}
}

func TestBootstrapMatches_AbsoluteColumnsViaBarGap(t *testing.T) {
	w, h := 640, 360
	cb := calibrate.DefaultCanonical()
	tc, tb := truthHandles()
	cal, _ := calibrate.NewSplit([8]geom.Pt{tc[0], tc[1], tc[2], tc[3], tb[0], tb[1], tb[2], tb[3]}, cb)
	omit := map[int]bool{13: true, 24: true, 5: true} // outermost + one inner missing
	mask := renderApexMask(t, w, h, cal, cb, omit)
	apexes := ApexComponents(mask, w, h)

	matches, ok := bootstrapMatches(apexes, cb)
	if !ok {
		t.Fatalf("bootstrap failed with %d apexes", len(apexes))
	}
	if len(matches) < 18 {
		t.Fatalf("only %d assignments, want >= 18", len(matches))
	}
	for slot, det := range matches {
		p := slot + 1
		want := cal.ToSource(cb.PointApex(p))
		got := apexes[det].Pt
		if d := math.Hypot(got.X-want.X, got.Y-want.Y); d > 6 {
			t.Errorf("slot %d (point %d) assigned to apex %.0fpx away", slot, p, d)
		}
	}
}

// renderDistortedApexMask renders the 24 triangles through the truth
// calibration and then records them through a known lens — pixel p is on iff
// its undistorted position falls inside an ideal-space triangle.
func renderDistortedApexMask(t *testing.T, w, h int, cal calibrate.BoardCalibration, cb calibrate.CanonicalBoard, lens calibrate.Lens) []bool {
	t.Helper()
	ideal := renderApexMask(t, w, h, cal, cb, nil)
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u := lens.Undistort(geom.P(float64(x)+0.5, float64(y)+0.5))
			ux, uy := int(u.X), int(u.Y)
			if ux >= 0 && ux < w && uy >= 0 && uy < h && ideal[uy*w+ux] {
				mask[y*w+x] = true
			}
		}
	}
	return mask
}

func TestFitHandles_ZeroDistortionStaysExactlyZero(t *testing.T) {
	w, h := 640, 360
	cb := calibrate.DefaultCanonical()
	tc, tb := truthHandles()
	cal, _ := calibrate.NewSplit([8]geom.Pt{tc[0], tc[1], tc[2], tc[3], tb[0], tb[1], tb[2], tb[3]}, cb)
	mask := renderApexMask(t, w, h, cal, cb, nil)
	res, ok := FitHandles(mask, w, h, tc, tb, cb)
	if !ok {
		t.Fatal("fit failed")
	}
	if res.Lens.K1 != 0 || res.Lens.K2 != 0 {
		t.Fatalf("undistorted board must be admitted at exactly 0/0, got k1=%v k2=%v", res.Lens.K1, res.Lens.K2)
	}
}

func TestFitHandles_RecoversSyntheticBarrel(t *testing.T) {
	w, h := 640, 360
	cb := calibrate.DefaultCanonical()
	tc, tb := truthHandles()
	cal, _ := calibrate.NewSplit([8]geom.Pt{tc[0], tc[1], tc[2], tc[3], tb[0], tb[1], tb[2], tb[3]}, cb)
	truth := calibrate.Lens{K1: -0.14, CenterX: float64(w) / 2, CenterY: float64(h) / 2, Norm: float64(w) / 2}
	mask := renderDistortedApexMask(t, w, h, cal, cb, truth)

	// Seed: the distorted positions of the truth handles (what a mask quad
	// would roughly see).
	seedC, seedB := tc, tb
	for i := range seedC {
		seedC[i] = truth.Distort(seedC[i])
		seedB[i] = truth.Distort(seedB[i])
	}
	res, ok := FitHandles(mask, w, h, seedC, seedB, cb)
	if !ok {
		t.Fatalf("fit failed (matches=%d resid=%.2f)", res.Matches, res.Resid)
	}
	if res.Lens.K1 == 0 {
		t.Fatal("barrel distortion present but k1 not admitted")
	}
	if math.Abs(res.Lens.K1-truth.K1) > 0.05 {
		t.Errorf("k1 = %.3f, want %.3f ± 0.05", res.Lens.K1, truth.K1)
	}
	// Handles are RECORDED-space: compare against the distorted truth.
	for i := range tc {
		want := truth.Distort(tc[i])
		if d := math.Hypot(res.Corners[i].X-want.X, res.Corners[i].Y-want.Y); d > 3.0 {
			t.Errorf("corner %d off by %.2fpx under barrel (%v vs %v)", i, d, res.Corners[i], want)
		}
	}
}
