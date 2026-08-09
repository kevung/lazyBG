package boarddiff

import (
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/engine"
	"lazybg/internal/perceive"
)

func exactObs(b bg.Board) perceive.ObservedBoard {
	var ob perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		c := b.Pts[p]
		if c.N == 0 {
			ob.Points[p] = perceive.PointObs{Confidence: 1}
			continue
		}
		side := perceive.A
		if c.Owner == bg.P2 {
			side = perceive.B
		}
		ob.Points[p] = perceive.PointObs{Count: c.N, Side: side, Confidence: 1}
	}
	return ob
}

// The contract WholeBoardMatch's own doc states — "NOT a per-move
// discriminator" — measured against the one that is. With a perfect reading of
// the board a candidate produces, DeltaMatch must separate that candidate from
// its rivals far more sharply than WholeBoardMatch does, because it only scores
// the points the move actually changed.
func TestDeltaMatch_DiscriminatesWhereWholeBoardDoesNot(t *testing.T) {
	pre := bg.StandardStart()
	moves, err := engine.LegalMoves(bg.Position{Board: pre, Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1})
	if err != nil {
		t.Fatal(err)
	}
	prev := exactObs(pre)
	for _, target := range moves {
		cur := exactObs(target.Result)

		var wholeSpread, deltaSpread float64
		wholeTarget := WholeBoardMatch(target.Result, cur)
		deltaTarget := DeltaMatch(pre, target.Result, prev, cur)
		for _, rival := range moves {
			if rival.Notation == target.Notation {
				continue
			}
			if w := WholeBoardMatch(rival.Result, cur); wholeTarget-w > wholeSpread {
				wholeSpread = wholeTarget - w
			}
			if d := DeltaMatch(pre, rival.Result, prev, cur); deltaTarget-d > deltaSpread {
				deltaSpread = deltaTarget - d
			}
		}
		if deltaTarget != 1 {
			t.Errorf("%s: DeltaMatch on its own board = %.4f, want 1", target.Notation, deltaTarget)
		}
		if deltaSpread <= wholeSpread {
			t.Errorf("%s: delta spread %.4f is not wider than whole-board spread %.4f",
				target.Notation, deltaSpread, wholeSpread)
		}
	}
}

// A rival must never score a perfect delta against another move's board:
// distinct resulting boards imply distinct deltas from a shared pre-board.
func TestDeltaMatch_RivalsNeverScorePerfect(t *testing.T) {
	pre := bg.StandardStart()
	moves, err := engine.LegalMoves(bg.Position{Board: pre, Dice: bg.Dice{6, 5}, PlayerOnRoll: bg.P1})
	if err != nil {
		t.Fatal(err)
	}
	prev := exactObs(pre)
	cur := exactObs(moves[0].Result)
	for _, rival := range moves[1:] {
		if boardSig(rival.Result) == boardSig(moves[0].Result) {
			continue // same board reached another way
		}
		if d := DeltaMatch(pre, rival.Result, prev, cur); d == 1 {
			t.Errorf("%s scores a perfect delta against %s's board", rival.Notation, moves[0].Notation)
		}
	}
}
