package eval

import (
	"lazybg/internal/bg"
	"lazybg/internal/cue"
	"lazybg/internal/derive"
)

// TurnRank records where the truth sat in one turn's ranked candidate list —
// the list the review UI puts in front of the human (ux-spec §2). Coverage and
// error rate only see the top candidate; this sees the whole list, which is
// what "effort saved" actually depends on when a human confirms with one key.
type TurnRank struct {
	Index    int // truth turn index, for cross-referencing the manifest
	Rank     int // 1-based rank of the truth move; 0 when absent from the list
	Listed   int // candidates the list held
	DiceRank int // 1-based rank of the first candidate proposing the truth ROLL; 0 when absent
}

// Found reports whether the truth move appeared in the list at all.
func (r TurnRank) Found() bool { return r.Rank > 0 }

// RankTruth locates truth in the decision's ranked candidate list (Top first,
// then Alternatives in order). Move identity is the same as ScoreMatch's:
// unordered dice plus the canonical hop multiset, so notation spelling and die
// order do not matter. A decision about the other player matches nothing.
//
// DiceRank answers the diagnostic question separately: even when the move is
// wrong, was the ROLL among the proposals? A missing roll is a dice-inference
// failure; a present roll with the wrong move is a board-diff or prior failure.
func RankTruth(d cue.MoveDecision, truth bg.Ply) TurnRank {
	r := TurnRank{Index: 0}
	cands := candidates(d)
	r.Listed = len(cands)
	if d.Player != truth.Player {
		return r
	}
	wantDice := normalizeDice(truth.Dice)
	wantCanon, ok := canonical(truth)
	for i, c := range cands {
		if normalizeDice(c.Dice) != wantDice {
			continue
		}
		if r.DiceRank == 0 {
			r.DiceRank = i + 1
		}
		if ok && r.Rank == 0 {
			if got, gotOK := canonicalNotation(c.Notation); gotOK && got == wantCanon {
				r.Rank = i + 1
			}
		}
	}
	return r
}

// candidates flattens a decision into its ranked list. A decision whose Top
// carries no move (nothing survived) contributes no candidates.
func candidates(d cue.MoveDecision) []cue.MoveHypothesis {
	if d.Top.Notation == "" && len(d.Alternatives) == 0 {
		return nil
	}
	out := make([]cue.MoveHypothesis, 0, 1+len(d.Alternatives))
	if d.Top.Notation != "" {
		out = append(out, d.Top)
	}
	return append(out, d.Alternatives...)
}

func normalizeDice(d bg.Dice) bg.Dice {
	if d[0] < d[1] {
		d[0], d[1] = d[1], d[0]
	}
	return d
}

func canonical(p bg.Ply) (string, bool) {
	if p.CannotMove || p.Notation == "" || p.Notation == "????" {
		return "", false
	}
	return canonicalNotation(p.Notation)
}

func canonicalNotation(n string) (string, bool) {
	c, err := derive.CanonicalPlays(n)
	if err != nil {
		return "", false
	}
	return c, true
}

// RankHistogram buckets a set of TurnRanks into the shape the decision needs:
// how often the human's first key is already the right one, how often a couple
// of arrow presses get there, and how often the list does not contain the
// answer at all.
type RankHistogram struct {
	N                 int
	Top1              int // truth was the pre-highlighted candidate
	Rank2to3          int // one or two arrow presses away
	Rank4Plus         int // in the list, but past the comfortable reach
	Absent            int // not in the list — the human must type it
	AbsentRollMissing int // …of which the truth ROLL was never proposed either

	byRank map[int]int // exact rank -> count, so WithinTop stays exact
}

// Histogram buckets ranks.
func Histogram(rs []TurnRank) RankHistogram {
	h := RankHistogram{N: len(rs), byRank: map[int]int{}}
	for _, r := range rs {
		h.byRank[r.Rank]++
		switch {
		case r.Rank == 1:
			h.Top1++
		case r.Rank == 2 || r.Rank == 3:
			h.Rank2to3++
		case r.Rank >= 4:
			h.Rank4Plus++
		default:
			h.Absent++
			if r.DiceRank == 0 {
				h.AbsentRollMissing++
			}
		}
	}
	return h
}

// WithinTop is the fraction of turns whose truth sat at rank k or better —
// the quantity a "confirm from a short list" interaction actually rides on.
func (h RankHistogram) WithinTop(k int) float64 {
	if h.N == 0 {
		return 0
	}
	n := 0
	for rank, c := range h.byRank {
		if rank >= 1 && rank <= k {
			n += c
		}
	}
	return float64(n) / float64(h.N)
}
