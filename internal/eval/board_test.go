package eval

import (
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/perceive"
)

// perfectObs builds the exact ObservedBoard a flawless reader would produce.
func perfectObs(b bg.Board) perceive.ObservedBoard {
	var ob perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		tp := b.Pts[p]
		if tp.N == 0 {
			continue
		}
		s := perceive.A
		if tp.Owner == bg.P2 {
			s = perceive.B
		}
		ob.Points[p] = perceive.PointObs{Count: tp.N, Side: s, Confidence: 1}
	}
	return ob
}

func TestScoreBoard_Perfect(t *testing.T) {
	r := ScoreBoard(perfectObs(bg.StandardStart()), bg.StandardStart())
	if r.Correct != 24 || !r.Exact() || r.PerPoint() != 1.0 {
		t.Errorf("perfect reading: %+v perPoint=%.2f exact=%v", r, r.PerPoint(), r.Exact())
	}
}

func TestScoreBoard_OneWrong(t *testing.T) {
	obs := perfectObs(bg.StandardStart())
	obs.Points[6].Count-- // misread the 6-point stack (5 -> 4)
	r := ScoreBoard(obs, bg.StandardStart())
	if r.Correct != 23 || r.Exact() {
		t.Errorf("one wrong: correct=%d exact=%v, want 23 / false", r.Correct, r.Exact())
	}
}

func TestScoreBoard_EmptyReading(t *testing.T) {
	// A blank reading matches only the empty points. StandardStart occupies 8
	// points, so 16 empty points are correct.
	var blank perceive.ObservedBoard
	r := ScoreBoard(blank, bg.StandardStart())
	if r.Correct != 16 {
		t.Errorf("empty reading vs StandardStart: correct=%d, want 16", r.Correct)
	}
}

func TestScoreBoard_WrongSide(t *testing.T) {
	obs := perfectObs(bg.StandardStart())
	obs.Points[13].Side = perceive.B // right count, wrong owner
	r := ScoreBoard(obs, bg.StandardStart())
	if r.Correct != 23 {
		t.Errorf("wrong side: correct=%d, want 23", r.Correct)
	}
}
