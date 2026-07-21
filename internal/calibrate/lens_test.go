package calibrate

import (
	"math"
	"testing"

	"lazybg/internal/geom"
)

// distort and undistort must invert each other within sub-pixel drift.
func TestLens_RoundTrip(t *testing.T) {
	// Realistic action-cam barrel: mild K1, normalised by the half-diagonal so
	// the whole frame stays within the distort map's invertible range.
	l := Lens{K1: -0.08, CenterX: 640, CenterY: 360, Norm: 734}
	for _, p := range []geom.Pt{{X: 100, Y: 80}, {X: 640, Y: 360}, {X: 1200, Y: 700}, {X: 300, Y: 600}} {
		u := l.undistort(l.distort(p))
		if math.Hypot(u.X-p.X, u.Y-p.Y) > 0.3 {
			t.Errorf("undistort(distort(%v)) = %v, drift too large", p, u)
		}
		d := l.distort(l.undistort(p))
		if math.Hypot(d.X-p.X, d.Y-p.Y) > 0.3 {
			t.Errorf("distort(undistort(%v)) = %v, drift too large", p, d)
		}
	}
}

// The zero-value lens is inactive (identity); the centre is a fixed point.
func TestLens_InactiveAndCentre(t *testing.T) {
	var off Lens
	p := geom.P(123, 456)
	if off.distort(p) != p || off.undistort(p) != p {
		t.Errorf("inactive lens is not the identity")
	}
	on := Lens{K1: -0.2, CenterX: 100, CenterY: 100, Norm: 200}
	if c := on.distort(geom.P(100, 100)); c != geom.P(100, 100) {
		t.Errorf("distortion centre moved: %v", c)
	}
}

func TestLensK2_RoundTrip(t *testing.T) {
	l := Lens{K1: -0.18, K2: 0.06, CenterX: 320, CenterY: 180, Norm: 320}
	for _, p := range []geom.Pt{{X: 10, Y: 20}, {X: 320, Y: 180}, {X: 600, Y: 40}, {X: 40, Y: 340}, {X: 630, Y: 350}} {
		d := l.distort(p)
		u := l.undistort(d)
		if math.Hypot(u.X-p.X, u.Y-p.Y) > 1e-6 {
			t.Errorf("round trip failed at %v: distort→%v→undistort→%v", p, d, u)
		}
	}
}

func TestLensK2_AloneIsActive(t *testing.T) {
	l := Lens{K2: 0.1, CenterX: 320, CenterY: 180, Norm: 320}
	p := geom.Pt{X: 600, Y: 40}
	if d := l.distort(p); d == p {
		t.Fatal("a pure-k2 lens must distort")
	}
	if !l.active() {
		t.Fatal("k2 alone must activate the lens")
	}
}

func TestLensK2_ZeroKeepsK1Behaviour(t *testing.T) {
	a := Lens{K1: -0.2, CenterX: 320, CenterY: 180, Norm: 320}
	b := Lens{K1: -0.2, K2: 0, CenterX: 320, CenterY: 180, Norm: 320}
	p := geom.Pt{X: 100, Y: 300}
	da, db := a.distort(p), b.distort(p)
	if math.Hypot(da.X-db.X, da.Y-db.Y) > 1e-12 {
		t.Fatal("k2=0 must reproduce the k1-only lens exactly")
	}
}
