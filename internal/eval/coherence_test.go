package eval

import (
	"math"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
	"lazybg/internal/perceive/boarddiff"
)

// decisionWithScores builds a decision the way DecideAnyDice does: the top's
// Confidence has already paid the runner-up penalty, the alternatives carry
// their raw score.
func decisionWithScores(player bg.Player, scores []float64, notations []string) cue.MoveDecision {
	d := cue.MoveDecision{Player: player}
	top := scores[0]
	if len(scores) > 1 {
		top -= boarddiff.RunnerUpPenalty * scores[1]
	}
	d.Top = cue.MoveHypothesis{Dice: bg.Dice{3, 1}, Notation: notations[0], Confidence: top}
	d.Confidence = top
	for i := 1; i < len(scores); i++ {
		d.Alternatives = append(d.Alternatives, cue.MoveHypothesis{
			Dice: bg.Dice{3, 1}, Notation: notations[i], Confidence: scores[i]})
	}
	return d
}

func TestRawScores_RecoversTheTopsRawScore(t *testing.T) {
	want := []float64{0.80, 0.50, 0.20}
	d := decisionWithScores(bg.P1, want, []string{"a/b", "c/d", "e/f"})
	got := RawScores(d)
	if len(got) != len(want) {
		t.Fatalf("got %d scores, want %d", len(got), len(want))
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("score %d = %.6f, want %.6f", i, got[i], want[i])
		}
	}
}

// A decision with no alternative pays no penalty, so its confidence IS its raw
// score.
func TestRawScores_SingleCandidate(t *testing.T) {
	d := decisionWithScores(bg.P1, []float64{0.7}, []string{"a/b"})
	got := RawScores(d)
	if len(got) != 1 || math.Abs(got[0]-0.7) > 1e-9 {
		t.Errorf("got %v, want [0.7]", got)
	}
}

// The guard that matters: with nothing to say, the coherence pass must not
// reorder anything.
func TestRescore_ZeroLookaheadIsANoOp(t *testing.T) {
	notations := []string{"a/b", "c/d", "e/f"}
	d := decisionWithScores(bg.P1, []float64{0.80, 0.50, 0.20}, notations)
	got := Rescore(d, []float64{0, 0, 0}, 1.0)
	gotOrder := []string{got.Top.Notation}
	for _, a := range got.Alternatives {
		gotOrder = append(gotOrder, a.Notation)
	}
	for i := range notations {
		if gotOrder[i] != notations[i] {
			t.Errorf("rank %d = %q, want %q", i+1, gotOrder[i], notations[i])
		}
	}
}

// Lookahead evidence strong enough must be able to promote a runner-up.
func TestRescore_LookaheadPromotesARunnerUp(t *testing.T) {
	d := decisionWithScores(bg.P1, []float64{0.80, 0.50, 0.20}, []string{"a/b", "c/d", "e/f"})
	got := Rescore(d, []float64{0.0, 0.9, 0.0}, 1.0)
	if got.Top.Notation != "c/d" {
		t.Errorf("top = %q, want c/d promoted by its lookahead", got.Top.Notation)
	}
}

// …but weak evidence must not. A coherence pass that reorders on noise is the
// failure mode experiment-plan §6 documents three times over.
func TestRescore_WeakLookaheadDoesNotReorder(t *testing.T) {
	d := decisionWithScores(bg.P1, []float64{0.80, 0.50, 0.20}, []string{"a/b", "c/d", "e/f"})
	got := Rescore(d, []float64{0.0, 0.1, 0.0}, 1.0)
	if got.Top.Notation != "a/b" {
		t.Errorf("top = %q, want a/b — a 0.1 lookahead cannot close a 0.30 gap", got.Top.Notation)
	}
}

// Re-ranking must never change WHICH candidates are on the list: coherence may
// reorder evidence, never invent or drop it.
func TestRescore_PreservesTheCandidateSet(t *testing.T) {
	notations := []string{"a/b", "c/d", "e/f", "g/h"}
	d := decisionWithScores(bg.P1, []float64{0.8, 0.6, 0.4, 0.2}, notations)
	got := Rescore(d, []float64{0.1, 0.9, 0.5, 0.3}, 1.0)
	seen := map[string]bool{got.Top.Notation: true}
	for _, a := range got.Alternatives {
		seen[a.Notation] = true
	}
	if len(seen) != len(notations) {
		t.Fatalf("candidate set changed: %v", seen)
	}
	for _, n := range notations {
		if !seen[n] {
			t.Errorf("candidate %q disappeared", n)
		}
	}
}

// A mismatched lookahead length is a caller bug; the decision must come back
// untouched rather than half-rescored.
func TestRescore_MismatchedLookaheadIsIgnored(t *testing.T) {
	d := decisionWithScores(bg.P1, []float64{0.8, 0.6}, []string{"a/b", "c/d"})
	got := Rescore(d, []float64{0.9}, 1.0)
	if got.Top.Notation != "a/b" || len(got.Alternatives) != 1 {
		t.Errorf("decision was modified despite a mismatched lookahead")
	}
}
