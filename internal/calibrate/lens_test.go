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
