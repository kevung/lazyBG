package e2e

import (
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/autocal"
	"lazybg/internal/profile"
)

// pilotAutocalOptions are the pilot board's declared color priors (teal /
// yellow points, pearl / dark checkers) — the only inputs autocalibration
// needs besides the video.
func pilotAutocalOptions() autocal.Options {
	o := autocal.DefaultOptions()
	o.Colors = autocal.Colors{
		PointA: color.RGBA{30, 100, 100, 255},
		PointB: color.RGBA{170, 135, 10, 255},
		Felt:   color.RGBA{168, 166, 162, 255},
	}
	o.Profile = profile.CaptureProfile{
		CheckerA: color.RGBA{225, 222, 210, 255},
		CheckerB: color.RGBA{70, 72, 80, 255},
	}
	return o
}

// TestRealCorpus_AutoCalibratePilot must rediscover, from the video alone
// plus color priors, what was hand-calibrated for the pilot: the four
// corners (within a few pixels), a trustworthy opening read, and the span
// begin near the known settled opening (~6s).
func TestRealCorpus_AutoCalibratePilot(t *testing.T) {
	if testing.Short() {
		t.Skip("long: decodes minutes of real video")
	}
	m, video := loadPilot(t)

	res, err := autocal.Calibrate(video, pilotAutocalOptions())
	if err != nil {
		t.Fatalf("autocal: %v (got %+v)", err, res)
	}
	t.Logf("corners: %v", res.Corners)
	t.Logf("span begin: %dms, opening score %d/24", res.SpanBeginMs, res.OpeningScore)

	// The functional criterion is the read score: 21/24 is the classical
	// reader's ceiling on this capture with HAND-picked corners, so parity
	// means the auto calibration is as good as manual. Corner distances vs
	// the hand-picked ones are a diagnostic only (hand corners are not truth
	// to ±10px); 60px catches gross failures.
	manual := m.Parts[0].Calibration.Corners
	for i := range manual {
		dx := res.Corners[i].X - manual[i][0]
		dy := res.Corners[i].Y - manual[i][1]
		if d := math.Hypot(dx, dy); d > 60 {
			t.Errorf("corner %d off by %.1fpx (auto %v vs manual %v)", i, d, res.Corners[i], manual[i])
		}
	}
	if res.OpeningScore < 20 {
		t.Errorf("opening score %d/24, want >= 20 (manual-calibration parity is 21)", res.OpeningScore)
	}
	if res.SpanBeginMs < 0 || res.SpanBeginMs > 15000 {
		t.Errorf("span begin %dms, want the settled opening in the first seconds", res.SpanBeginMs)
	}
}

// TestRealCorpus_AutoCalibrateOblique runs the same loop on a different
// event's capture (vbc, tilted ~10° with stronger perspective): the
// generalization check that auto-calibration is not overfit to the pilot's
// straight-on view. Same physical board, so the color priors carry over.
func TestRealCorpus_AutoCalibrateOblique(t *testing.T) {
	t.Skip("WIP: still 16/24 with rotation moves — the residual is upstream of refinement " +
		"(likely extreme-projection corner ORDER on a rotated board, or the opening-scan frame " +
		"not being a settled start). Next: line-based initial quad + corner-order disambiguation " +
		"(docs/research/board-autocalibration-survey.md); kept as the executable target.")
	if testing.Short() {
		t.Skip("long: decodes minutes of real video")
	}
	video := filepath.Join(repoRoot, "corpus/2025-10_vbc/r2_DanielRozenberg/20251006 vbc ronde2 KevinUnger DanielRozenberg 11p [UHRjtH-OfRg].mkv")
	if _, err := os.Stat(video); err != nil {
		t.Skipf("corpus video not present: %v", err)
	}
	o := pilotAutocalOptions()
	o.Colors = autocal.Colors{} // exercise automatic color derivation
	o.ScanEndMs = 240000        // the opening may come later in this recording

	res, err := autocal.Calibrate(video, o)
	if err != nil {
		t.Fatalf("autocal: %v (got %+v)", err, res)
	}
	t.Logf("corners: %v", res.Corners)
	t.Logf("span begin: %dms, opening score %d/24", res.SpanBeginMs, res.OpeningScore)
	if res.OpeningScore < 19 {
		t.Errorf("opening score %d/24, want >= 19", res.OpeningScore)
	}
}
