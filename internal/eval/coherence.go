package eval

import (
	"sort"

	"lazybg/internal/cue"
	"lazybg/internal/perceive/boarddiff"
)

// RawScores recovers the comparable per-candidate scores behind a
// MoveDecision. It exists because Top.Confidence is NOT on the same scale as
// the Alternatives' Confidence: the top has already paid the runner-up penalty
// (boarddiff.RunnerUpPenalty), the alternatives carry their raw score. Any
// re-ranking that mixed the two would silently favour the alternatives.
//
// The returned slice is parallel to Candidates(d).
func RawScores(d cue.MoveDecision) []float64 {
	cands := Candidates(d)
	if len(cands) == 0 {
		return nil
	}
	out := make([]float64, len(cands))
	out[0] = d.Top.Confidence
	if len(cands) > 1 {
		out[0] += boarddiff.RunnerUpPenalty * d.Alternatives[0].Confidence
	}
	for i, a := range d.Alternatives {
		out[i+1] = a.Confidence
	}
	return out
}

// Candidates flattens a decision into its ranked list, Top first. A decision
// whose Top carries no move contributes nothing.
func Candidates(d cue.MoveDecision) []cue.MoveHypothesis {
	return candidates(d)
}

// Rescore re-orders a decision's candidates by score + weight*lookahead[i],
// where lookahead[i] is independent evidence about candidate i gathered from
// somewhere other than this turn — in practice, how well the NEXT turn's
// observed transition can be explained if candidate i were true.
//
// The returned decision keeps the same candidate set: re-ranking may move the
// truth up or down, never in or out. That is deliberate — the failure mode this
// guards against is a coherence pass that manufactures agreement rather than
// finding it (docs/experiment-plan.md §6: three cues made more talkative, three
// end-to-end losses).
func Rescore(d cue.MoveDecision, lookahead []float64, weight float64) cue.MoveDecision {
	cands := Candidates(d)
	if len(cands) == 0 || len(lookahead) != len(cands) {
		return d
	}
	raw := RawScores(d)
	type item struct {
		h     cue.MoveHypothesis
		score float64
		orig  int
	}
	items := make([]item, len(cands))
	for i, c := range cands {
		items[i] = item{c, raw[i] + weight*lookahead[i], i}
	}
	// Stable on the original order so an all-zero lookahead is a no-op.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].orig < items[j].orig
	})
	out := cue.MoveDecision{Player: d.Player, Tick: d.Tick}
	out.Top = items[0].h
	out.Top.Confidence = items[0].score
	if len(items) > 1 {
		out.Top.Confidence -= boarddiff.RunnerUpPenalty * items[1].score
	}
	out.Confidence = out.Top.Confidence
	for _, it := range items[1:] {
		h := it.h
		h.Confidence = it.score
		out.Alternatives = append(out.Alternatives, h)
	}
	return out
}
