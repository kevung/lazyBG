package transcribe

import (
	"testing"

	"lazybg/internal/corpus"
	"lazybg/internal/geom"
)

// A declared lens must change the source mapping: with barrel distortion the
// same canonical point samples a different source pixel than the pinhole
// homography would.
func TestPartSetupUsesLens(t *testing.T) {
	part := corpus.Part{
		File: "x",
		Calibration: corpus.Calibration{
			Corners: [][2]float64{{100, 100}, {1180, 100}, {1180, 620}, {100, 620}},
		},
		Priors: corpus.Priors{CheckerA: "#ffffff", CheckerB: "#000000"},
	}
	calNo, cb, _, err := PartSetup(part)
	if err != nil {
		t.Fatal(err)
	}
	part.Calibration.Lens = &corpus.Lens{K1: -0.2, CenterX: 640, CenterY: 360, Norm: 640}
	calYes, _, _, err := PartSetup(part)
	if err != nil {
		t.Fatal(err)
	}
	// probe an off-centre point (near a corner, where barrel bends most)
	p := geom.P(150, 150)
	a, b := calNo.ToCanonical(p), calYes.ToCanonical(p)
	if a == b {
		t.Fatalf("lens had no effect: %v == %v", a, b)
	}
	w, h := cb.Size()
	if b.X < -50 || b.Y < -50 || b.X > float64(w)+50 || b.Y > float64(h)+50 {
		t.Fatalf("lensed mapping unreasonable: %v (canvas %dx%d)", b, w, h)
	}
}
