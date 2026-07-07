package boardstate

import (
	"image/color"
	"image/png"
	"os"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/geom"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/checker"
	"lazybg/internal/profile"
)

// Real-pixel validation of the shape-first reader on a settled competition
// opening frame (hsbtMars2025 r1) against the known StandardStart. The
// calibration below (corners / board geometry / checker colours / radius) is
// this capture's one-time Board Calibration + Session Priors — not magic
// constants but per-capture inputs.
//
// This is the payoff of the perception spike + detector survey
// (docs/research/perception-detector-survey.md): detect checkers by circular
// SHAPE, use colour only for owner. The colour-centreline reader managed ~42%
// per-point on this exact frame (grey felt ≈ white checkers, marbled swirl ≈
// felt); the shape reader clears ~88%. The residual — the two off-tray-corner
// back-checker pairs and one tall-stack off-by-one — is the survey's known
// ~90% per-point ceiling, caught downstream by fusion + engine legality + the
// review queue. We assert a conservative floor so the test is a stable
// regression guard, not a brittle exact-match.
func TestCircleReader_RealOpeningFrame(t *testing.T) {
	f, err := os.Open("../../../testdata/perception/hsbtMars2025-r1-opening.png")
	if err != nil {
		t.Skipf("fixture not present: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	// Board Calibration for this capture (fixed camera).
	corners := [4]geom.Pt{geom.P(203, 54), geom.P(825, 46), geom.P(818, 614), geom.P(200, 628)}
	cb := calibrate.CanonicalBoard{MarginX: 16, MarginY: 18, PointW: 58, QuadH: 300, BarGap: 60, OffW: 24}
	cal, ok := calibrate.New(corners, cb)
	if !ok {
		t.Fatal("degenerate calibration")
	}
	rect := cal.Rectify(img)

	// Session Priors: sampled checker colours + checker radius.
	prof := profile.CaptureProfile{
		CheckerA: color.RGBA{225, 222, 210, 255}, // white
		CheckerB: color.RGBA{70, 72, 80, 255},    // dark
	}
	reader := CircleReader{Profile: prof, Radius: 29, Params: checker.Params{PeakFrac: 0.38}}
	obs := reader.Read(rect, cb)

	truth := bg.StandardStart()
	exact := 0
	for p := 1; p <= 24; p++ {
		got := obs.Points[p]
		tp := truth.Pts[p]
		ts := perceive.None
		if tp.N > 0 {
			ts = perceive.A
			if tp.Owner == bg.P2 {
				ts = perceive.B
			}
		}
		if got.Count == tp.N && got.Side == ts {
			exact++
		}
	}
	t.Logf("real-frame per-point accuracy: %d/24", exact)
	if exact < 20 { // achieved 21/24; floor with margin
		t.Errorf("real-frame per-point accuracy %d/24, want >= 20 — shape-first reader regressed", exact)
	}
}
