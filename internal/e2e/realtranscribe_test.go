package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/eval"
	"lazybg/internal/matimport"
	"lazybg/internal/transcribe"
)

// TestRealCorpus_TranscribeOpeningMinutes runs the ENTIRE pipeline — stream,
// stable windows, full-res board reads, unknown-dice inference, conducting —
// on the first three minutes of the real pilot video, then scores the result
// against the ground-truth .mat.
//
// Current honest baseline: the classical reader's ~85% per-point accuracy on
// real frames is not enough for blind dice+move inference, so plies land in
// the review queue (top-K candidates), not in auto-fill — coverage 0, error
// 0. The assertions guard the pipeline MECHANICS (turn segmentation finds the
// turns, everything is gated honestly); the accuracy floor arrives with the
// learned reader (experiment-plan §8 step 8). The log carries the numbers.
func TestRealCorpus_TranscribeOpeningMinutes(t *testing.T) {
	if testing.Short() {
		t.Skip("long: decodes minutes of real video")
	}
	m, _ := loadPilot(t)

	o := transcribe.DefaultRunOptions()
	o.LimitMs = 180000
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
	t.Logf("3 minutes: %d plies produced, %d matched truth, autofilled %d (errors %d), reviewed %d, skipped %d, unexplained %d",
		got, s.Matched, s.AutoFilled, s.AutoFillErrors(), s.Reviewed, out.Skipped, out.Unexplained)
	for gi, g := range out.Match.Games {
		for pi, p := range g.Plies {
			t.Logf("  g%d p%d @%dms: %v %v %q conf=%.2f", gi+1, pi, p.Tick, p.Player, p.Dice, p.Notation, p.Confidence)
		}
	}

	if got < 10 || got > 45 {
		t.Errorf("%d plies produced in 3 minutes of play, want 10..45 (≈ turns played)", got)
	}
	// The guarded metric: nothing wrong may pass the gate silently.
	if s.AutoFillErrors() > 0 {
		t.Errorf("%d confidently wrong auto-fills — the gate must hold these back", s.AutoFillErrors())
	}
}
