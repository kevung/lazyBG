package geom

import (
	"math"
	"testing"
)

func TestHomographyFit_MatchesExactFourPoints(t *testing.T) {
	src := [4]Pt{P(0, 0), P(100, 0), P(100, 80), P(0, 80)}
	dst := [4]Pt{P(12, 7), P(140, 22), P(133, 110), P(3, 95)}
	exact, ok := Homography(src, dst)
	if !ok {
		t.Fatal("exact homography failed")
	}
	fit, ok := HomographyFit(src[:], dst[:])
	if !ok {
		t.Fatal("fit failed")
	}
	for _, p := range []Pt{P(50, 40), P(10, 70), P(90, 5)} {
		a, b := exact.Apply(p), fit.Apply(p)
		if math.Hypot(a.X-b.X, a.Y-b.Y) > 1e-6 {
			t.Errorf("fit differs from exact at %v: %v vs %v", p, b, a)
		}
	}
}

func TestHomographyFit_OverdeterminedNoisy(t *testing.T) {
	truth, _ := Homography(
		[4]Pt{P(0, 0), P(100, 0), P(100, 80), P(0, 80)},
		[4]Pt{P(20, 10), P(150, 30), P(140, 120), P(10, 100)},
	)
	var src, dst []Pt
	noise := []float64{0.3, -0.2, 0.1, -0.3, 0.2, 0.0, -0.1, 0.3, -0.2, 0.1, 0.25, -0.15}
	i := 0
	for y := 0.0; y <= 80; y += 40 {
		for x := 0.0; x <= 100; x += 25 {
			q := truth.Apply(P(x, y))
			src = append(src, P(x, y))
			dst = append(dst, P(q.X+noise[i%len(noise)], q.Y+noise[(i+5)%len(noise)]))
			i++
		}
	}
	fit, ok := HomographyFit(src, dst)
	if !ok {
		t.Fatal("fit failed")
	}
	// The fit must reproject well within the injected noise amplitude.
	var worst float64
	for j, p := range src {
		q := fit.Apply(p)
		if d := math.Hypot(q.X-dst[j].X, q.Y-dst[j].Y); d > worst {
			worst = d
		}
	}
	if worst > 1.0 {
		t.Errorf("worst reprojection %.3fpx, want < 1.0", worst)
	}
}

func TestHomographyFit_RejectsDegenerate(t *testing.T) {
	// All points collinear: no homography.
	var src, dst []Pt
	for x := 0.0; x < 6; x++ {
		src = append(src, P(x*10, 0))
		dst = append(dst, P(x*10+5, 3))
	}
	if _, ok := HomographyFit(src, dst); ok {
		t.Error("collinear fit must fail")
	}
	if _, ok := HomographyFit(src[:3], dst[:3]); ok {
		t.Error("under-determined fit must fail")
	}
}

func TestHomographyFitLines_BreaksTwoRowDegeneracy(t *testing.T) {
	// Correspondences confined to two horizontal lines (a board's two apex
	// rows) leave a one-parameter family: the point-only fit may reproject
	// the rows perfectly yet be wildly wrong elsewhere. Line constraints
	// (edge lines through off-row points) must recover the true map.
	truth, _ := Homography(
		[4]Pt{P(0, 0), P(100, 0), P(100, 80), P(0, 80)},
		[4]Pt{P(20, 10), P(150, 30), P(140, 120), P(10, 100)},
	)
	var src, dst []Pt
	for x := 5.0; x <= 95; x += 15 { // two rows only: y=25 and y=55
		for _, y := range []float64{25, 55} {
			src = append(src, P(x, y))
			dst = append(dst, truth.Apply(P(x, y)))
		}
	}
	// Edge-like constraints: for a few known source points OFF the rows,
	// build the image line through their true images (any line through the
	// image point works; use one with a distinct slope per constraint).
	var cons []LineConstraint
	for i, sp := range []Pt{P(10, 0), P(60, 0), P(30, 80), P(90, 80)} {
		ip := truth.Apply(sp)
		slope := 0.3 + 0.4*float64(i)
		// line through ip with direction (1, slope): -slope·x + y + c = 0
		c := slope*ip.X - ip.Y
		cons = append(cons, LineConstraint{Src: sp, L: [3]float64{-slope, 1, c}})
	}
	fit, ok := HomographyFitLines(src, dst, cons)
	if !ok {
		t.Fatal("constrained fit failed")
	}
	for _, sp := range []Pt{P(0, 0), P(100, 0), P(50, 80), P(0, 80), P(100, 80)} {
		a, b := truth.Apply(sp), fit.Apply(sp)
		if d := math.Hypot(a.X-b.X, a.Y-b.Y); d > 0.5 {
			t.Errorf("constrained fit off by %.2fpx at %v (%v vs %v)", d, sp, b, a)
		}
	}
}
