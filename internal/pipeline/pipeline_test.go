package pipeline

import (
	"strings"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
	"lazybg/internal/fusion"
	"lazybg/internal/gate"
	"lazybg/internal/matexport"
)

func prior(m map[string]float64) func(fusion.Candidate) float64 {
	return func(c fusion.Candidate) float64 { return m[c.Notation] }
}

// TestSpine_EndToEnd proves the whole walking skeleton: synthetic cues → fusion
// → gate → auto-filled plies → a .mat export, plus a low-confidence turn routed
// to the review queue.
func TestSpine_EndToEnd(t *testing.T) {
	turns := []Turn{
		{ // P2 opening: unanimous cues → auto-fill
			Player: bg.P2, Tick: 1000,
			Cues: []cue.Cue{
				{Kind: cue.BoardDiff, Confidence: 0.95, Notation: "24/23 13/11"},
				{Kind: cue.DiceValue, Confidence: 0.95, Dice: bg.Dice{2, 1}},
			},
			Legal: []fusion.Candidate{
				{Dice: bg.Dice{2, 1}, Notation: "24/23 13/11"},
				{Dice: bg.Dice{2, 1}, Notation: "13/11 6/5"},
			},
			Prior: prior(map[string]float64{"24/23 13/11": 1.0, "13/11 6/5": 0.3}),
		},
		{ // P1 reply: board-diff + dice agree, sole strong candidate → auto-fill
			Player: bg.P1, Tick: 2000,
			Cues: []cue.Cue{
				{Kind: cue.BoardDiff, Confidence: 0.9, Notation: "8/5 6/5"},
				{Kind: cue.DiceValue, Confidence: 0.9, Dice: bg.Dice{3, 1}},
			},
			Legal: []fusion.Candidate{{Dice: bg.Dice{3, 1}, Notation: "8/5 6/5"}},
			Prior: prior(map[string]float64{"8/5 6/5": 1.0}),
		},
		{ // P2: ambiguous, no direct evidence → needs review
			Player: bg.P2, Tick: 3000,
			Legal: []fusion.Candidate{
				{Dice: bg.Dice{5, 2}, Notation: "24/22 13/8"},
				{Dice: bg.Dice{5, 2}, Notation: "13/8 13/11"},
			},
			Prior: prior(map[string]float64{"24/22 13/8": 0.52, "13/8 13/11": 0.5}),
		},
	}

	res := Run(turns, fusion.DefaultWeights(), gate.Default())

	if len(res.Plies) != 2 {
		t.Fatalf("expected 2 auto-filled plies, got %d (%+v)", len(res.Plies), res.Plies)
	}
	if len(res.Review) != 1 {
		t.Fatalf("expected 1 review item, got %d", len(res.Review))
	}
	if res.Plies[0].Notation != "24/23 13/11" || res.Plies[1].Notation != "8/5 6/5" {
		t.Errorf("auto-filled the wrong moves: %+v", res.Plies)
	}
	if res.Plies[0].Tick != 1000 {
		t.Errorf("ply lost its video tick: %+v", res.Plies[0])
	}

	// Assemble the auto-filled plies into a match and export .mat.
	m := bg.Match{
		Length:  3,
		Players: [2]string{"Alice", "Bob"},
		Meta:    []bg.MetaField{{Key: "Site", Value: "Skeleton"}},
		Games:   []bg.Game{{Number: 1, Plies: res.Plies}},
	}
	out := matexport.Write(m)

	if !strings.Contains(out, "3 point match") {
		t.Errorf(".mat missing match length:\n%s", out)
	}
	// P2's opening sits alone on row 1 (right column at col 39).
	if !strings.Contains(out, "  1) "+strings.Repeat(" ", 34)+"21: 24/23 13/11") {
		t.Errorf(".mat missing P2 opening row:\n%s", out)
	}
	// P1's reply is the left cell of row 2.
	if !strings.Contains(out, "  2) 31: 8/5 6/5") {
		t.Errorf(".mat missing P1 reply row:\n%s", out)
	}
}
