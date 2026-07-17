// Candidate ranking: the same fused score the automatic pipeline uses
// (architecture §4, fusion.Weights), applied to manual entry. The engine's
// equity prior is always present; a post-move board observation — when any
// perception is available and confident enough to have an opinion — re-weights
// the list. With no observation the ranking degrades to pure equity order
// (issue #15; functional-spec §4).
package session

import (
	"sort"

	"lazybg/internal/engine"
	"lazybg/internal/fusion"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boarddiff"
)

// rankedMove is one candidate with its fused score and the cues that
// contributed to it.
type rankedMove struct {
	mv    engine.LegalMove
	score float64
}

// rankMoves fuses the equity prior with the (optional) post-move observation
// and returns the candidates best-first. All legal moves are scored before
// capping, so a low-equity move the pixels support can surface.
func rankMoves(moves []engine.LegalMove, obs *perceive.ObservedBoard) ([]rankedMove, []string) {
	if len(moves) == 0 {
		return nil, nil
	}
	w := fusion.DefaultWeights()

	// Equity → prior in [0,1], min-max over the legal set (the same
	// normalization boarddiff.Decide uses).
	best, worst := moves[0].Equity, moves[0].Equity
	for _, m := range moves {
		if m.Equity > best {
			best = m.Equity
		}
		if m.Equity < worst {
			worst = m.Equity
		}
	}
	prior := func(e float64) float64 {
		if best > worst {
			return (e - worst) / (best - worst)
		}
		return 1
	}

	cues := []string{"engine-equity"}
	ranked := make([]rankedMove, len(moves))
	for i, m := range moves {
		score := prior(m.Equity)
		if obs != nil {
			agree := boarddiff.WholeBoardMatch(m.Result, *obs)
			score = (w.BoardDiff*agree + w.Engine*prior(m.Equity)) / (w.BoardDiff + w.Engine)
		}
		ranked[i] = rankedMove{mv: m, score: score}
	}
	if obs != nil {
		cues = append(cues, "board-diff")
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	return ranked, cues
}
