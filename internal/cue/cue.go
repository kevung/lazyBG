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
	Dice       bg.Dice // for DiceValue
	Notation   string  // for BoardDiff: the candidate move implied by the diff
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
