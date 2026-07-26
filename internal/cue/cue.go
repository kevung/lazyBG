// Package cue is the shared vocabulary between perception and inference
// (docs/domain-model.md §3–§4). Detectors emit Cues; Fusion consumes them and
// produces a MoveDecision. Nothing here touches pixels.
package cue

import "lazybg/internal/bg"

// Kind enumerates the independent evidence types.
type Kind int

const (
	Commit      Kind = iota // a turn boundary occurred
	BoardDiff               // the pre/post board diff implies a candidate move
	DiceValue               // the dice were read visually
	CubeState               // the doubling cube's value/side, or a double event
	EnginePrior             // legality verdict + equity ranking from the engine
)

func (k Kind) String() string {
	switch k {
	case Commit:
		return "commit"
	case BoardDiff:
		return "board-diff"
	case DiceValue:
		return "dice"
	case CubeState:
		return "cube"
	case EnginePrior:
		return "engine-prior"
	}
	return "unknown"
}

// Cue is one piece of confidence-bearing evidence at a Tick. Payload fields are
// interpreted per Kind; unused fields stay zero. (A skeleton-level flat struct;
// this becomes a sum type as detectors grow.)
type Cue struct {
	Kind       Kind
	Tick       int
	Confidence float64 // [0,1]
	Dice       bg.Dice // for DiceValue: hard pair (legacy binary agreement)
	Notation   string  // for BoardDiff: the candidate move implied by the diff

	// DicePMF, for DiceValue, is a probability distribution over the 21
	// distinct rolls (keys high-die-first, values summing to ~1). When set
	// it replaces the hard pair in fusion: agreement becomes proportional
	// to each roll's mass, so a SPREAD distribution cannot steer a decision
	// the way a wrong hard pair does (measured 2026-07-24: hard scan pairs
	// were wrong ~83-100% on real footage and pushed decisions off truth),
	// while a PEAKED one keeps the hard pair's disambiguation power.
	DicePMF map[bg.Dice]float64
}

// MoveHypothesis is a candidate (dice, move) with a confidence and the kinds of
// evidence that supported it.
type MoveHypothesis struct {
	Dice       bg.Dice
	Notation   string
	Confidence float64
	Support    []Kind
}

// MoveDecision is the fused outcome for one turn: the top hypothesis, the ranked
// alternatives, the joint confidence, and the Tick to jump to in the video.
type MoveDecision struct {
	Player       bg.Player
	Tick         int
	Top          MoveHypothesis
	Alternatives []MoveHypothesis
	Confidence   float64
}
