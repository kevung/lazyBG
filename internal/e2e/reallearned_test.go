package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/eval"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive/pointnet"
	"lazybg/internal/transcribe"
)

const modelPath = "../../data/models/pointreader.bin"

// TestRealCorpus_LearnedReaderOpeningFrame reads the real settled opening
// with the LEARNED point reader and compares it to the classical reader's
// documented 21/24 on the same frame. The model was trained on crops of this
// recording's games 1,2,3,6 (games 4,5 held out at 89% per-crop) — this is a
// same-capture integration check, not a generalization claim.
func TestRealCorpus_LearnedReaderOpeningFrame(t *testing.T) {
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model not present: %v", err)
	}
	m, video := loadPilot(t)
	part := m.Parts[0]

	img, err := capture.FrameAt(video, part.Span.BeginMs)
	if err != nil {
		t.Fatal(err)
	}
	cal, cb, _, err := transcribe.PartSetup(part)
	if err != nil {
		t.Fatal(err)
	}
	_ = cal
	net, err := pointnet.Load(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	obs := pointnet.Reader{Net: net}.Read(cal.Rectify(img), cb)

	r := eval.ScoreBoard(obs, bg.StandardStart())
	// Current model (89% per-crop, single-capture data): 20/24 here — the
	// residual is tall-stack 5→4 undercounts at LOW confidence (which the
	// delta matcher weights down) plus the opening die read as a checker.
	// The classical reader scores 21/24 on the same frame with the same
	// failure profile. Floor is a regression guard; raising it is the job of
	// more corpus manifests, not of this test.
	t.Logf("learned reader on the real opening: %d/24 (classical reader: 21/24)", r.Correct)
	if r.Correct < 19 {
		t.Errorf("learned reader %d/24, want >= 19", r.Correct)
	}
}

// TestRealCorpus_LearnedTranscribe re-runs the 3-minute end-to-end
// transcription with the learned reader and reports the effort-saved bundle
// next to the classical baseline (0 matched / everything reviewed).
func TestRealCorpus_LearnedTranscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("long: decodes minutes of real video")
	}
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model not present: %v", err)
	}
	m, _ := loadPilot(t)

	o := transcribe.DefaultRunOptions()
	o.LimitMs = 180000
	o.ModelPath = modelPath
	out, err := transcribe.Recording(repoRoot, m, o)
	if err != nil {
		t.Fatal(err)
	}
	matBytes, err := os.ReadFile(filepath.Join(repoRoot, m.Transcript))
	if err != nil {
		t.Skipf("truth transcript not present: %v", err)
	}
	truth, err := matimport.Parse(string(matBytes))
	if err != nil {
		t.Fatal(err)
	}
	s := eval.ScoreMatch(out.Match, truth, o.Conduct.Policy.Threshold)
	got := 0
	for _, g := range out.Match.Games {
		got += len(g.Plies)
	}
	t.Logf("learned 3 minutes: %d plies, %d matched truth, autofilled %d (errors %d), reviewed %d, skipped %d, unexplained %d",
		got, s.Matched, s.AutoFilled, s.AutoFillErrors(), s.Reviewed, out.Skipped, out.Unexplained)
	for _, g := range out.Match.Games {
		for pi, p := range g.Plies {
			t.Logf("  g%d p%d @%dms: %v %v %q conf=%.2f", g.Number, pi, p.Tick, p.Player, p.Dice, p.Notation, p.Confidence)
		}
	}
	// The guarded metric still rules: no confidently wrong plies.
	if s.AutoFillErrors() > 0 {
		t.Errorf("%d confidently wrong auto-fills", s.AutoFillErrors())
	}
	// The learned reader must beat the classical baseline (0 matched).
	if s.Matched < 3 {
		t.Errorf("matched %d truth plies, want >= 3 with the learned reader", s.Matched)
	}
}

var _ = calibrate.CanonicalBoard{}
