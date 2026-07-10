package eval

import (
	"testing"

	"lazybg/internal/bg"
)

func ply(who bg.Player, d bg.Dice, notation string, conf float64) bg.Ply {
	return bg.Ply{Player: who, Dice: d, Notation: notation, Confidence: conf}
}

// A four-ply game both sides agree on.
func truthGame() bg.Game {
	return bg.Game{Number: 1, Plies: []bg.Ply{
		ply(bg.P1, bg.Dice{3, 1}, "8/5 6/5", 0),
		ply(bg.P2, bg.Dice{4, 2}, "8/4 6/4", 0),
		ply(bg.P1, bg.Dice{6, 1}, "13/7 8/7", 0),
		ply(bg.P2, bg.Dice{5, 3}, "8/3 6/3", 0),
	}}
}

func matchOf(g ...bg.Game) bg.Match {
	return bg.Match{Length: 7, Players: [2]string{"A", "B"}, Games: g}
}

func TestScoreMatch_Identical(t *testing.T) {
	truth := matchOf(truthGame())
	got := matchOf(truthGame())
	// Mark all output plies as confidently auto-filled.
	for i := range got.Games[0].Plies {
		got.Games[0].Plies[i].Confidence = 0.9
	}
	s := ScoreMatch(got, truth, 0.8)
	if s.TruthCheckerPlies != 4 || s.Matched != 4 {
		t.Errorf("matched %d/%d, want 4/4", s.Matched, s.TruthCheckerPlies)
	}
	if s.AutoFilled != 4 || s.AutoFilledCorrect != 4 {
		t.Errorf("autofilled %d correct %d, want 4/4", s.AutoFilled, s.AutoFilledCorrect)
	}
	if s.AutoFillErrors() != 0 {
		t.Errorf("autofill errors = %d, want 0", s.AutoFillErrors())
	}
}

// A wrong auto-filled ply must count as an auto-fill ERROR — the guarded
// metric (experiment-plan §6).
func TestScoreMatch_WrongPlyIsAutoFillError(t *testing.T) {
	truth := matchOf(truthGame())
	got := matchOf(truthGame())
	for i := range got.Games[0].Plies {
		got.Games[0].Plies[i].Confidence = 0.9
	}
	got.Games[0].Plies[2] = ply(bg.P1, bg.Dice{6, 1}, "24/18 18/17", 0.9) // wrong move

	s := ScoreMatch(got, truth, 0.8)
	if s.Matched != 3 {
		t.Errorf("matched = %d, want 3", s.Matched)
	}
	if s.AutoFillErrors() != 1 {
		t.Errorf("autofill errors = %d, want 1", s.AutoFillErrors())
	}
}

// A ply the pipeline sent to review (low confidence) must not count as an
// auto-fill error even when wrong.
func TestScoreMatch_ReviewedWrongPlyIsNotAnError(t *testing.T) {
	truth := matchOf(truthGame())
	got := matchOf(truthGame())
	for i := range got.Games[0].Plies {
		got.Games[0].Plies[i].Confidence = 0.9
	}
	got.Games[0].Plies[2] = ply(bg.P1, bg.Dice{6, 1}, "24/18 18/17", 0.3) // wrong but reviewed

	s := ScoreMatch(got, truth, 0.8)
	if s.AutoFillErrors() != 0 {
		t.Errorf("autofill errors = %d, want 0 (the wrong ply was under threshold)", s.AutoFillErrors())
	}
	if s.Reviewed != 1 {
		t.Errorf("reviewed = %d, want 1", s.Reviewed)
	}
}

// A missed ply (e.g. an undetected turn) must not desynchronize the rest of
// the game: alignment is a subsequence match, not index-by-index.
func TestScoreMatch_MissingPlyStillAlignsRest(t *testing.T) {
	truth := matchOf(truthGame())
	g := truthGame()
	g.Plies = append(g.Plies[:1], g.Plies[2:]...) // drop ply 1
	got := matchOf(g)

	s := ScoreMatch(got, truth, 0.8)
	if s.Matched != 3 {
		t.Errorf("matched = %d, want 3 (one truth ply missed)", s.Matched)
	}
}

// Cube actions in the truth are counted separately: v1 perception cannot see
// them, and the metric must say so rather than hide it.
func TestScoreMatch_CountsTruthCubeActions(t *testing.T) {
	g := truthGame()
	cube := bg.Ply{Player: bg.P1, Cube: bg.Double, CubeValue: 2}
	take := bg.Ply{Player: bg.P2, Cube: bg.Take}
	g.Plies = append(g.Plies[:2:2], append([]bg.Ply{cube, take}, g.Plies[2:]...)...)
	truth := matchOf(g)
	got := matchOf(truthGame())

	s := ScoreMatch(got, truth, 0.8)
	if s.TruthCubeActions != 2 {
		t.Errorf("truth cube actions = %d, want 2", s.TruthCubeActions)
	}
	if s.Matched != 4 {
		t.Errorf("matched = %d, want 4 (checker plies align around cube actions)", s.Matched)
	}
}

// Dance plies align on player + dice with unchanged boards.
func TestScoreMatch_DanceAligns(t *testing.T) {
	g := truthGame()
	g.Plies = append(g.Plies, bg.Ply{Player: bg.P1, Dice: bg.Dice{6, 6}, CannotMove: true})
	truth := matchOf(g)
	got := matchOf(g)

	s := ScoreMatch(got, truth, 0.8)
	if s.TruthCheckerPlies != 5 || s.Matched != 5 {
		t.Errorf("matched %d/%d, want 5/5", s.Matched, s.TruthCheckerPlies)
	}
}
