package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/geom"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boardstate"
	"lazybg/internal/perceive/checker"
	"lazybg/internal/profile"
)

// repoRoot is where the corpus/ tree and manifests live, relative to this
// package's directory.
const repoRoot = "../.."

// loadPilot loads the pilot recording's manifest, skipping the test when the
// manifest or the (gitignored, machine-local) video is absent so CI without
// the corpus stays green.
func loadPilot(t *testing.T) (corpus.Manifest, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, "corpus/manifest/hsbtMars2025-main-r1.json"))
	if err != nil {
		t.Skipf("pilot manifest not present: %v", err)
	}
	m, err := corpus.Load(data)
	if err != nil {
		t.Fatalf("pilot manifest invalid: %v", err)
	}
	video := filepath.Join(repoRoot, m.Parts[0].File)
	if _, err := os.Stat(video); err != nil {
		t.Skipf("corpus video not present: %v", err)
	}
	return m, video
}

// TestRealCorpus_OpeningFrame exercises the real front half on actual corpus
// footage: manifest → ffmpeg decode at the span's begin tick (a settled
// standard-start frame by labeling convention) → homography rectification →
// shape-first board reading, checked against the known standard start. This is
// the first test where every stage runs on real video, no synthetic input.
func TestRealCorpus_OpeningFrame(t *testing.T) {
	m, video := loadPilot(t)
	part := m.Parts[0]

	img, err := capture.FrameAt(video, part.Span.BeginMs)
	if err != nil {
		t.Fatalf("decode opening frame: %v", err)
	}

	var corners [4]geom.Pt
	for i, c := range part.Calibration.Corners {
		corners[i] = geom.P(c[0], c[1])
	}
	cb := calibrate.DefaultCanonical()
	if c := part.Calibration.Canonical; c != nil {
		cb = calibrate.CanonicalBoard{MarginX: c.MarginX, MarginY: c.MarginY,
			PointW: c.PointW, QuadH: c.QuadH, BarGap: c.BarGap, OffW: c.OffW}
	}
	cal, ok := calibrate.New(corners, cb)
	if !ok {
		t.Fatal("degenerate calibration")
	}
	rect := cal.Rectify(img)

	ca, err := profile.ParseHex(part.Priors.CheckerA)
	if err != nil {
		t.Fatal(err)
	}
	cbcol, err := profile.ParseHex(part.Priors.CheckerB)
	if err != nil {
		t.Fatal(err)
	}
	reader := boardstate.CircleReader{
		Profile: profile.CaptureProfile{CheckerA: ca, CheckerB: cbcol},
		Params:  checker.Params{PeakFrac: 0.38},
	}
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
	t.Logf("real-video opening per-point accuracy: %d/24", exact)
	if exact < 20 {
		t.Errorf("real-video opening accuracy %d/24, want >= 20", exact)
	}
}
