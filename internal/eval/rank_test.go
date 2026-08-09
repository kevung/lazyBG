package eval

import (
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
)

// hyp is a candidate shorthand for the tests.
func hyp(d1, d2 int, notation string) cue.MoveHypothesis {
	return cue.MoveHypothesis{Dice: bg.Dice{d1, d2}, Notation: notation}
}

func decision(player bg.Player, top cue.MoveHypothesis, alts ...cue.MoveHypothesis) cue.MoveDecision {
	return cue.MoveDecision{Player: player, Top: top, Alternatives: alts}
}

func truthPly(player bg.Player, d1, d2 int, notation string) bg.Ply {
	return bg.Ply{Player: player, Dice: bg.Dice{d1, d2}, Notation: notation}
}

func TestRankTruth_TopCandidateIsRankOne(t *testing.T) {
	d := decision(bg.P1, hyp(3, 1, "8/5 6/5"), hyp(3, 1, "13/10 6/5"))
	got := RankTruth(d, truthPly(bg.P1, 3, 1, "8/5 6/5"))
	if got.Rank != 1 {
		t.Errorf("Rank = %d, want 1", got.Rank)
	}
	if got.DiceRank != 1 {
		t.Errorf("DiceRank = %d, want 1", got.DiceRank)
	}
	if got.Listed != 2 {
		t.Errorf("Listed = %d, want 2", got.Listed)
	}
	if !got.Found() {
		t.Error("Found = false, want true")
	}
}

func TestRankTruth_AlternativesAreRankedAfterTop(t *testing.T) {
	d := decision(bg.P1,
		hyp(3, 1, "13/10 6/5"),
		hyp(3, 1, "24/21 6/5"),
		hyp(3, 1, "8/5 6/5"),
	)
	got := RankTruth(d, truthPly(bg.P1, 3, 1, "8/5 6/5"))
	if got.Rank != 3 {
		t.Errorf("Rank = %d, want 3 (top is 1, alternatives follow)", got.Rank)
	}
	if got.Listed != 3 {
		t.Errorf("Listed = %d, want 3", got.Listed)
	}
}

// Notation spelling must not matter: the same hops in another order are the
// same move, exactly as ScoreMatch already treats them.
func TestRankTruth_MatchesOnCanonicalHopsNotSpelling(t *testing.T) {
	d := decision(bg.P1, hyp(3, 1, "6/5 8/5"))
	got := RankTruth(d, truthPly(bg.P1, 3, 1, "8/5 6/5"))
	if got.Rank != 1 {
		t.Errorf("Rank = %d, want 1 (same hops, different spelling)", got.Rank)
	}
}

// Dice are unordered: a 1-3 truth matches a 3-1 candidate.
func TestRankTruth_DiceAreUnordered(t *testing.T) {
	d := decision(bg.P1, hyp(3, 1, "8/5 6/5"))
	got := RankTruth(d, truthPly(bg.P1, 1, 3, "8/5 6/5"))
	if got.Rank != 1 {
		t.Errorf("Rank = %d, want 1 (dice are unordered)", got.Rank)
	}
}

// The diagnostic split the ticket asks for: the truth move is absent, but was
// its ROLL even proposed? That separates a dice-inference failure from a
// board-diff/prior failure.
func TestRankTruth_AbsentMoveButRollProposed(t *testing.T) {
	d := decision(bg.P1,
		hyp(6, 6, "24/18(2) 13/7(2)"),
		hyp(3, 1, "13/10 6/5"),
	)
	got := RankTruth(d, truthPly(bg.P1, 3, 1, "8/5 6/5"))
	if got.Rank != 0 {
		t.Errorf("Rank = %d, want 0 (truth move absent)", got.Rank)
	}
	if got.DiceRank != 2 {
		t.Errorf("DiceRank = %d, want 2 (the roll appears at rank 2)", got.DiceRank)
	}
}

func TestRankTruth_AbsentMoveAndRollNeverProposed(t *testing.T) {
	d := decision(bg.P1,
		hyp(6, 6, "24/18(2) 13/7(2)"),
		hyp(5, 2, "13/8 13/11"),
	)
	got := RankTruth(d, truthPly(bg.P1, 3, 1, "8/5 6/5"))
	if got.Rank != 0 {
		t.Errorf("Rank = %d, want 0", got.Rank)
	}
	if got.DiceRank != 0 {
		t.Errorf("DiceRank = %d, want 0 (the roll was never proposed)", got.DiceRank)
	}
}

// A decision about the other player explains nothing about this turn.
func TestRankTruth_WrongPlayerNeverMatches(t *testing.T) {
	d := decision(bg.P2, hyp(3, 1, "8/5 6/5"))
	got := RankTruth(d, truthPly(bg.P1, 3, 1, "8/5 6/5"))
	if got.Rank != 0 || got.DiceRank != 0 {
		t.Errorf("Rank/DiceRank = %d/%d, want 0/0 (decision is about the other player)", got.Rank, got.DiceRank)
	}
}

// An empty decision (no candidate survived) is a clean miss, not a panic.
func TestRankTruth_EmptyDecision(t *testing.T) {
	got := RankTruth(cue.MoveDecision{Player: bg.P1}, truthPly(bg.P1, 3, 1, "8/5 6/5"))
	if got.Rank != 0 || got.Listed != 0 {
		t.Errorf("Rank/Listed = %d/%d, want 0/0", got.Rank, got.Listed)
	}
}

func TestRankHistogram_Buckets(t *testing.T) {
	h := Histogram([]TurnRank{
		{Rank: 1, DiceRank: 1},
		{Rank: 1, DiceRank: 1},
		{Rank: 2, DiceRank: 1},
		{Rank: 3, DiceRank: 2},
		{Rank: 5, DiceRank: 1},
		{Rank: 0, DiceRank: 3}, // move missed, roll proposed
		{Rank: 0, DiceRank: 0}, // roll never proposed
	})
	if h.N != 7 {
		t.Errorf("N = %d, want 7", h.N)
	}
	if h.Top1 != 2 {
		t.Errorf("Top1 = %d, want 2", h.Top1)
	}
	if h.Rank2to3 != 2 {
		t.Errorf("Rank2to3 = %d, want 2", h.Rank2to3)
	}
	if h.Rank4Plus != 1 {
		t.Errorf("Rank4Plus = %d, want 1", h.Rank4Plus)
	}
	if h.Absent != 2 {
		t.Errorf("Absent = %d, want 2", h.Absent)
	}
	if h.AbsentRollMissing != 1 {
		t.Errorf("AbsentRollMissing = %d, want 1", h.AbsentRollMissing)
	}
	if got, want := h.WithinTop(3), 4.0/7.0; got != want {
		t.Errorf("WithinTop(3) = %v, want %v", got, want)
	}
}
