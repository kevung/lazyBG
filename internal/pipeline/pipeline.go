// Package pipeline wires the inference spine for a turn: cues + legality → a
// fused MoveDecision → a gate outcome → either an auto-filled bg.Ply or a queued
// review item. It is the seam the video front-end (capture + detectors) will
// feed once those milestones land; for now it is driven by synthetic Cues in
// tests and the demo command.
package pipeline

import (
	"lazybg/internal/bg"
	"lazybg/internal/cue"
	"lazybg/internal/fusion"
	"lazybg/internal/gate"
)

// Turn is one segmented turn's worth of evidence handed to the pipeline: the
// player on roll, the video tick, the cues observed, the engine's legal
// candidates, and the engine's ranking prior over them.
type Turn struct {
	Player bg.Player
	Tick   int
	Cues   []cue.Cue
	Legal  []fusion.Candidate
	Prior  func(fusion.Candidate) float64
}

// ReviewItem is a queued low-confidence decision awaiting a human.
type ReviewItem struct {
	Decision cue.MoveDecision
	Reason   string
}

// Result is the outcome of running a batch of turns.
type Result struct {
	Plies  []bg.Ply     // auto-filled moves, in order
	Review []ReviewItem // turns that need a human
}

// Run fuses and gates each turn, auto-filling confident decisions into Plies and
// queuing the rest for review.
func Run(turns []Turn, w fusion.Weights, policy gate.Policy) Result {
	var r Result
	for _, t := range turns {
		d := fusion.Fuse(t.Player, t.Tick, t.Cues, t.Legal, t.Prior, w)
		outcome, reason := policy.Classify(d)
		switch outcome {
		case gate.AutoFill:
			r.Plies = append(r.Plies, bg.Ply{
				Player:     d.Player,
				Dice:       d.Top.Dice,
				Notation:   d.Top.Notation,
				Tick:       d.Tick,
				Confidence: d.Confidence,
			})
		default:
			r.Review = append(r.Review, ReviewItem{Decision: d, Reason: reason})
		}
	}
	return r
}
