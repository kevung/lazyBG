package e2e

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/align"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/transcribe"
)

// TestRealCorpus_AlignOpeningMinutes anchors the ground-truth transcript to
// the real pilot video over its first three minutes: the aligner must place
// most of the truth turns played in that span on monotonically increasing
// ticks with strong board agreement, and the labeled crop extraction must
// produce one labeled crop per point per aligned turn. This is the labeling
// machine the learned reader trains on.
func TestRealCorpus_AlignOpeningMinutes(t *testing.T) {
	if testing.Short() {
		t.Skip("long: decodes minutes of real video")
	}
	m, _ := loadPilot(t)
	matBytes, err := os.ReadFile(filepath.Join(repoRoot, m.Transcript))
	if err != nil {
		t.Skipf("truth transcript not present: %v", err)
	}
	truth, err := matimport.Parse(string(matBytes))
	if err != nil {
		t.Fatal(err)
	}

	o := transcribe.DefaultRunOptions()
	o.LimitMs = 180000
	events, err := transcribe.ReadEvents(repoRoot, m, o)
	if err != nil {
		t.Fatal(err)
	}
	turns := align.TruthTurns(truth)
	assign := align.Align(turns, events)

	aligned, lastTick := 0, -1
	var meanMatch float64
	for k, turn := range turns {
		if assign[k] < 0 {
			continue
		}
		ev := events[assign[k]]
		score := boarddiff.WholeBoardMatch(turn.Board, ev.Obs)
		t.Logf("turn %3d g%d %v %-22q @%6dms match=%.2f", turn.Index, turn.Game, turn.Dice, turn.Notation, ev.Tick, score)
		if ev.Tick <= lastTick {
			t.Errorf("turn %d tick %d not after previous %d", turn.Index, ev.Tick, lastTick)
		}
		lastTick = ev.Tick
		aligned++
		meanMatch += score
	}
	t.Logf("aligned %d/%d truth turns over %d events", aligned, len(turns), len(events))
	// ~12 turns are played in the first 3 minutes of the pilot.
	if aligned < 8 {
		t.Errorf("aligned %d turns, want >= 8 in the first 3 minutes", aligned)
	}
	if aligned > 0 {
		meanMatch /= float64(aligned)
		if meanMatch < 0.80 {
			t.Errorf("mean board agreement %.3f, want >= 0.80", meanMatch)
		}
	}

	// Crops: one labeled crop per point per aligned turn.
	dir := t.TempDir()
	res, err := align.ExtractCrops(repoRoot, m, turns, assign, events, dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Turns != aligned || res.Crops != aligned*24 {
		t.Errorf("crops: %d turns / %d crops, want %d / %d", res.Turns, res.Crops, aligned, aligned*24)
	}
	f, err := os.Open(filepath.Join(dir, "labels.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != res.Crops+1 { // header + one per crop
		t.Errorf("labels.csv rows = %d, want %d", len(rows), res.Crops+1)
	}
}
