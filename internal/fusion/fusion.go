// Package fusion correlates independent Cues into a ranked MoveDecision under a
// hard legality filter (docs/architecture.md §4). It is a pure function of its
// inputs — no pixels, no I/O — which is exactly the synthetic-cue test surface.
//
// Skeleton scope: the interpretable weighted combination described in the
// architecture doc. The Dempster–Shafer / learned upgrades slot in behind the
// same Fuse signature later.
package fusion

import (
	"sort"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
)

// Candidate is a legal (dice, move) produced by the engine's legality filter.
type Candidate struct {
	Dice     bg.Dice
	Notation string
}

// Weights are the hand-set reliability weights for each soft cue. A cue kind
// that produced no evidence for a turn is simply dropped from the combination
// (its weight is not counted), so absence never penalizes.
type Weights struct {
	BoardDiff float64
	Dice      float64
	Engine    float64
}

// DefaultWeights favors the most direct evidence (the board diff), then the
// observed dice, then the engine's ranking prior.
func DefaultWeights() Weights { return Weights{BoardDiff: 0.5, Dice: 0.3, Engine: 0.2} }

// Fuse ranks the legal candidates against the cues and returns a MoveDecision.
// prior maps a candidate to the engine's ranking prior in [0,1] (top moves →
// higher). legal must already be legality-filtered by the engine.
func Fuse(player bg.Player, tick int, cues []cue.Cue, legal []Candidate, prior func(Candidate) float64, w Weights) cue.MoveDecision {
	d := cue.MoveDecision{Player: player, Tick: tick}
	if len(legal) == 0 {
		return d
	}

	hasBoardDiff := anyKind(cues, cue.BoardDiff)
	hasDice := anyKind(cues, cue.DiceValue)

	type scored struct {
		h     cue.MoveHypothesis
		score float64
	}
	items := make([]scored, 0, len(legal))
	for _, c := range legal {
		var wSum, num float64
		var support []cue.Kind

		if hasBoardDiff {
			a := boardDiffAgree(cues, c)
			num += w.BoardDiff * a
			wSum += w.BoardDiff
			if a > 0 {
				support = append(support, cue.BoardDiff)
			}
		}
		if hasDice {
			a := diceAgree(cues, c)
			num += w.Dice * a
			wSum += w.Dice
			if a > 0 {
				support = append(support, cue.DiceValue)
			}
		}
		// The engine prior is always present.
		p := prior(c)
		num += w.Engine * p
		wSum += w.Engine
		support = append(support, cue.EnginePrior)

		score := 0.0
		if wSum > 0 {
			score = num / wSum
		}
		items = append(items, scored{
			h: cue.MoveHypothesis{Dice: c.Dice, Notation: c.Notation, Confidence: score, Support: support},
		})
		items[len(items)-1].score = score
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })

	top := items[0]
	second := 0.0
	if len(items) > 1 {
		second = items[1].score
	}
	// Joint confidence = strength of the winning evidence, discounted by how
	// strongly the best alternative competes. A lone or clearly-separated winner
	// stays near its raw agreement; a close runner-up pulls it toward review.
	// (The 0.3 runner-up penalty is a hand-set starting point; confidence
	// calibration is deferred until labeled data exists — see architecture §4.)
	const runnerUpPenalty = 0.3
	d.Confidence = clamp01(top.score - runnerUpPenalty*second)
	d.Top = top.h
	for _, it := range items[1:] {
		d.Alternatives = append(d.Alternatives, it.h)
	}
	return d
}

func anyKind(cues []cue.Cue, k cue.Kind) bool {
	for _, c := range cues {
		if c.Kind == k {
			return true
		}
	}
	return false
}

// boardDiffAgree is the confidence of the best board-diff cue that names this
// candidate's move, or 0 if none match.
func boardDiffAgree(cues []cue.Cue, c Candidate) float64 {
	best := 0.0
	for _, cu := range cues {
		if cu.Kind == cue.BoardDiff && cu.Notation == c.Notation && cu.Confidence > best {
			best = cu.Confidence
		}
	}
	return best
}

// diceAgree is the confidence of the best dice cue matching this candidate's
// dice, order-insensitive, or 0 if none match.
func diceAgree(cues []cue.Cue, c Candidate) float64 {
	best := 0.0
	for _, cu := range cues {
		if cu.Kind == cue.DiceValue && sameDice(cu.Dice, c.Dice) && cu.Confidence > best {
			best = cu.Confidence
		}
	}
	return best
}

func sameDice(a, b bg.Dice) bool {
	return (a[0] == b[0] && a[1] == b[1]) || (a[0] == b[1] && a[1] == b[0])
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
