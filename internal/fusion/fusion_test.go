package fusion

import (
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
)

// rankPrior builds an engine-prior function from a notation→prior map.
func rankPrior(m map[string]float64) func(Candidate) float64 {
	return func(c Candidate) float64 { return m[c.Notation] }
}

func TestFuse_StrongAgreement_HighConfidence(t *testing.T) {
	legal := []Candidate{
		{Dice: bg.Dice{3, 1}, Notation: "8/5 6/5"},
		{Dice: bg.Dice{3, 1}, Notation: "24/21 13/12"},
	}
	cues := []cue.Cue{
		{Kind: cue.BoardDiff, Confidence: 0.9, Notation: "8/5 6/5"},
		{Kind: cue.DiceValue, Confidence: 0.9, Dice: bg.Dice{3, 1}},
	}
	prior := rankPrior(map[string]float64{"8/5 6/5": 1.0, "24/21 13/12": 0.2})

	d := Fuse(bg.P1, 1234, cues, legal, prior, DefaultWeights())

	if d.Top.Notation != "8/5 6/5" {
		t.Fatalf("top = %q, want %q", d.Top.Notation, "8/5 6/5")
	}
	// The dice (3-1) match both candidates, so only the board-diff discriminates;
	// confidence is high-ish but not maxed (honest: dice didn't disambiguate).
	if d.Confidence < 0.65 {
		t.Errorf("confidence = %.3f, want ≥ 0.65 when board-diff clearly favors the top", d.Confidence)
	}
	if d.Tick != 1234 || d.Player != bg.P1 {
		t.Errorf("decision lost player/tick: %+v", d)
	}
	if len(d.Alternatives) != 1 || d.Alternatives[0].Notation != "24/21 13/12" {
		t.Errorf("alternatives = %+v, want the runner-up", d.Alternatives)
	}
}

func TestFuse_Ambiguous_LowConfidence(t *testing.T) {
	legal := []Candidate{
		{Dice: bg.Dice{5, 2}, Notation: "24/22 13/8"},
		{Dice: bg.Dice{5, 2}, Notation: "24/19 6/4"},
	}
	// No board-diff, no dice cue; the engine barely prefers one.
	prior := rankPrior(map[string]float64{"24/22 13/8": 0.55, "24/19 6/4": 0.5})

	d := Fuse(bg.P2, 10, nil, legal, prior, DefaultWeights())

	if d.Top.Notation != "24/22 13/8" {
		t.Fatalf("top = %q, want the slightly-preferred move", d.Top.Notation)
	}
	if d.Confidence >= 0.5 {
		t.Errorf("confidence = %.3f, want < 0.5 for a near-tie with no direct evidence", d.Confidence)
	}
}

// When the dice were never seen but only one candidate is legal, the engine's
// legality filter rescues the turn: a single legal candidate the board-diff
// confirms should yield a usable confidence.
func TestFuse_DiceAbsent_SingleLegal_Rescued(t *testing.T) {
	legal := []Candidate{{Dice: bg.Dice{6, 4}, Notation: "24/18 13/9"}}
	cues := []cue.Cue{{Kind: cue.BoardDiff, Confidence: 0.85, Notation: "24/18 13/9"}}
	prior := rankPrior(map[string]float64{"24/18 13/9": 0.9})

	d := Fuse(bg.P1, 7, cues, legal, prior, DefaultWeights())

	if d.Top.Notation != "24/18 13/9" || len(d.Alternatives) != 0 {
		t.Fatalf("expected the sole legal candidate as top with no alternatives, got %+v", d)
	}
	if d.Confidence < 0.7 {
		t.Errorf("confidence = %.3f, want ≥ 0.7 (single legal candidate, board-diff agrees)", d.Confidence)
	}
	if !supports(d.Top, cue.BoardDiff) || !supports(d.Top, cue.EnginePrior) {
		t.Errorf("top hypothesis should credit board-diff + engine: %+v", d.Top.Support)
	}
}

func TestFuse_NoLegalMoves_ZeroDecision(t *testing.T) {
	d := Fuse(bg.P1, 0, nil, nil, rankPrior(nil), DefaultWeights())
	if d.Confidence != 0 || d.Top.Notation != "" || len(d.Alternatives) != 0 {
		t.Errorf("empty legal set should yield a zero decision, got %+v", d)
	}
}

func supports(h cue.MoveHypothesis, k cue.Kind) bool {
	for _, s := range h.Support {
		if s == k {
			return true
		}
	}
	return false
}
