package engine

import (
	"os"
	"testing"

	"lazybg/internal/bg"
)

func TestMain(m *testing.M) {
	// gnubg.Init expects an FS that contains a data/ subdir, so pass the repo root.
	if err := Init(os.DirFS("../..")); err != nil {
		panic("engine init: " + err.Error())
	}
	os.Exit(m.Run())
}

func findMove(moves []LegalMove, notation string) (LegalMove, bool) {
	for _, mv := range moves {
		if mv.Notation == notation {
			return mv, true
		}
	}
	return LegalMove{}, false
}

// The best 3-1 opening play is the 5-point ("8/5 6/5") — the most-agreed opening
// move in backgammon theory and gnubg's top choice. This validates the whole
// board→engine→notation→result seam for the player-on-roll = P1.
func TestLegalMoves_Opening31_P1(t *testing.T) {
	pos := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	moves, err := LegalMoves(pos)
	if err != nil {
		t.Fatal(err)
	}
	if len(moves) == 0 {
		t.Fatal("no legal moves for the opening 3-1")
	}
	if moves[0].Notation != "8/5 6/5" {
		t.Errorf("best 3-1 = %q, want %q", moves[0].Notation, "8/5 6/5")
	}
	// Resulting board: P1 makes the 5-point.
	r := moves[0].Result
	for p, want := range map[int]int{5: 2, 6: 4, 8: 2} {
		if r.Pts[p].N != want || r.Pts[p].Owner != bg.P1 {
			t.Errorf("after 8/5 6/5: point %d = %+v, want N=%d P1", p, r.Pts[p], want)
		}
	}
	if r.Checkers(bg.P1) != 15 || r.Checkers(bg.P2) != 15 {
		t.Errorf("checker conservation broken: P1=%d P2=%d", r.Checkers(bg.P1), r.Checkers(bg.P2))
	}
}

// P2 on roll exercises the mirrored (White) coordinate mapping. Player-relative
// notation is identical ("8/5 6/5"), but the resulting board touches the
// mirrored absolute points.
func TestLegalMoves_Opening31_P2(t *testing.T) {
	pos := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P2}
	moves, err := LegalMoves(pos)
	if err != nil {
		t.Fatal(err)
	}
	if moves[0].Notation != "8/5 6/5" {
		t.Fatalf("best 3-1 (P2) = %q, want %q", moves[0].Notation, "8/5 6/5")
	}
	// P2's 5-point is absolute point 20; its 6→ abs 19, 8→ abs 17.
	r := moves[0].Result
	for p, want := range map[int]int{20: 2, 19: 4, 17: 2} {
		if r.Pts[p].N != want || r.Pts[p].Owner != bg.P2 {
			t.Errorf("after P2 8/5 6/5: point %d = %+v, want N=%d P2", p, r.Pts[p], want)
		}
	}
}

func TestLegalMoves_ContainsAllReasonableOptions(t *testing.T) {
	pos := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	moves, _ := LegalMoves(pos)
	// Other legal 3-1 plays gnubg enumerates (play order as the engine renders it).
	for _, want := range []string{"13/10 24/23", "24/21 6/5"} {
		if _, ok := findMove(moves, want); !ok {
			t.Errorf("legal move %q not found among %d moves", want, len(moves))
		}
	}
	// Equity must be monotonically non-increasing (ranked best-first).
	for i := 1; i < len(moves); i++ {
		if moves[i].Equity > moves[i-1].Equity+1e-9 {
			t.Errorf("moves not ranked by equity at %d: %.4f > %.4f", i, moves[i].Equity, moves[i-1].Equity)
		}
	}
}

func TestFrameRoundTrip(t *testing.T) {
	start := bg.StandardStart()
	for _, who := range []bg.Player{bg.P1, bg.P2} {
		got := fromOnRollFrame(onRollFrame(start, who), who)
		for i := 1; i <= 24; i++ {
			if got.Pts[i] != start.Pts[i] {
				t.Errorf("onRoll=%v round-trip point %d = %+v, want %+v", who, i, got.Pts[i], start.Pts[i])
			}
		}
	}
}
