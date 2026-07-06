// Command lazybg is, for now, a walking-skeleton demo of the inference spine:
// it drives the pipeline with synthetic Cues (standing in for the not-yet-built
// video front-end), fuses and gates each turn, prints the resulting .mat, and
// lists any turns queued for human review.
//
// This is scaffolding: the real app (video scrubber ↔ move list ↔ review queue)
// arrives at the UI milestone (docs/architecture.md §3, §8).
package main

import (
	"fmt"
	"os"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
	"lazybg/internal/fusion"
	"lazybg/internal/gate"
	"lazybg/internal/matexport"
	"lazybg/internal/pipeline"
)

func main() {
	prior := func(m map[string]float64) func(fusion.Candidate) float64 {
		return func(c fusion.Candidate) float64 { return m[c.Notation] }
	}

	turns := []pipeline.Turn{
		{
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
		{
			Player: bg.P1, Tick: 2000,
			Cues: []cue.Cue{
				{Kind: cue.BoardDiff, Confidence: 0.9, Notation: "8/5 6/5"},
				{Kind: cue.DiceValue, Confidence: 0.9, Dice: bg.Dice{3, 1}},
			},
			Legal: []fusion.Candidate{{Dice: bg.Dice{3, 1}, Notation: "8/5 6/5"}},
			Prior: prior(map[string]float64{"8/5 6/5": 1.0}),
		},
		{
			Player: bg.P2, Tick: 3000,
			Legal: []fusion.Candidate{
				{Dice: bg.Dice{5, 2}, Notation: "24/22 13/8"},
				{Dice: bg.Dice{5, 2}, Notation: "13/8 13/11"},
			},
			Prior: prior(map[string]float64{"24/22 13/8": 0.52, "13/8 13/11": 0.5}),
		},
	}

	res := pipeline.Run(turns, fusion.DefaultWeights(), gate.Default())

	m := bg.Match{
		Length:  3,
		Players: [2]string{"Alice", "Bob"},
		Meta: []bg.MetaField{
			{Key: "Site", Value: "lazyBG skeleton demo"},
			{Key: "Player 1", Value: "Alice"},
			{Key: "Player 2", Value: "Bob"},
		},
		Games: []bg.Game{{Number: 1, Plies: res.Plies}},
	}

	fmt.Fprint(os.Stdout, matexport.Write(m))
	fmt.Fprintf(os.Stderr, "\n— auto-filled %d move(s); %d queued for review —\n",
		len(res.Plies), len(res.Review))
	for _, ri := range res.Review {
		fmt.Fprintf(os.Stderr, "  review @tick %d: top=%q conf=%.2f (%s)\n",
			ri.Decision.Tick, ri.Decision.Top.Notation, ri.Decision.Confidence, ri.Reason)
	}
}
