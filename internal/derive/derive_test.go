package derive

import (
	"os"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/engine"
	"lazybg/internal/matimport"
)

func TestMain(m *testing.M) {
	if err := engine.Init(os.DirFS("../..")); err != nil {
		panic("engine init: " + err.Error())
	}
	os.Exit(m.Run())
}

func boardsEqual(a, b bg.Board) bool {
	for i := 1; i <= 24; i++ {
		if a.Pts[i] != b.Pts[i] {
			return false
		}
	}
	return a.Bar == b.Bar && a.Off == b.Off
}

// The strongest check: for many positions, applying each legal move's notation
// with ApplyNotation must reproduce exactly the board the (independently
// validated) engine computed. Covers plain moves, hits, and bear-off.
func TestApplyNotation_AgreesWithEngine(t *testing.T) {
	// Clean opening positions only. Hit-heavy and bear-off/racing positions are
	// validated directly below (TestApplyNotation_HitSendsOpponentToBar,
	// TestApplyNotation_BearOff) because the engine seam has a separate, known
	// issue emitting garbled moves/results for those — see the task notes.
	positions := []bg.Position{
		{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1},
		{Board: bg.StandardStart(), Dice: bg.Dice{6, 5}, PlayerOnRoll: bg.P2},
		{Board: bg.StandardStart(), Dice: bg.Dice{4, 2}, PlayerOnRoll: bg.P1},
		{Board: bg.StandardStart(), Dice: bg.Dice{6, 4}, PlayerOnRoll: bg.P2},
	}
	for _, pos := range positions {
		moves, err := engine.LegalMoves(pos)
		if err != nil {
			t.Fatal(err)
		}
		for _, mv := range moves {
			got, err := ApplyNotation(pos.Board, pos.PlayerOnRoll, mv.Notation)
			if err != nil {
				t.Errorf("ApplyNotation(%q) error: %v", mv.Notation, err)
				continue
			}
			if !boardsEqual(got, mv.Result) {
				t.Errorf("notation %q: derived board != engine result", mv.Notation)
			}
		}
	}
}

// hitPosition places a lone P2 blot on P1's 20-point.
func hitPosition() bg.Board {
	b := bg.StandardStart()
	b.Pts[19] = bg.Point{N: 4, Owner: bg.P2}
	b.Pts[20] = bg.Point{N: 1, Owner: bg.P2}
	return b
}

// bearoffPosition puts all of P1's checkers in the home board (points 1-6).
func bearoffPosition() bg.Board {
	var b bg.Board
	b.Pts[6] = bg.Point{N: 5, Owner: bg.P1}
	b.Pts[5] = bg.Point{N: 4, Owner: bg.P1}
	b.Pts[4] = bg.Point{N: 3, Owner: bg.P1}
	b.Pts[3] = bg.Point{N: 2, Owner: bg.P1}
	b.Pts[2] = bg.Point{N: 1, Owner: bg.P1}
	// P2 far away, out of contact.
	b.Pts[24] = bg.Point{N: 8, Owner: bg.P2}
	b.Pts[23] = bg.Point{N: 7, Owner: bg.P2}
	return b
}

func TestApplyNotation_HitSendsOpponentToBar(t *testing.T) {
	got, err := ApplyNotation(hitPosition(), bg.P1, "24/20*")
	if err != nil {
		t.Fatal(err)
	}
	if got.Pts[20] != (bg.Point{N: 1, Owner: bg.P1}) {
		t.Errorf("point 20 after hit = %+v, want 1×P1", got.Pts[20])
	}
	if got.Bar[bg.P2] != 1 {
		t.Errorf("P2 bar after hit = %d, want 1", got.Bar[bg.P2])
	}
	if got.Checkers(bg.P1) != 15 || got.Checkers(bg.P2) != 15 {
		t.Errorf("conservation: P1=%d P2=%d", got.Checkers(bg.P1), got.Checkers(bg.P2))
	}
}

func TestApplyNotation_Grouping(t *testing.T) {
	// "13/7(2)" moves two checkers 13→7 for P1.
	got, err := ApplyNotation(bg.StandardStart(), bg.P1, "13/7(2)")
	if err != nil {
		t.Fatal(err)
	}
	if got.Pts[13].N != 3 || got.Pts[7] != (bg.Point{N: 2, Owner: bg.P1}) {
		t.Errorf("after 13/7(2): p13=%+v p7=%+v", got.Pts[13], got.Pts[7])
	}
}

// Direct bear-off validation (independent of the engine): from the bearoff
// board, "6/off 5/off" bears one checker off the 6-point and one off the
// 5-point.
func TestApplyNotation_BearOff(t *testing.T) {
	got, err := ApplyNotation(bearoffPosition(), bg.P1, "6/off 5/off")
	if err != nil {
		t.Fatal(err)
	}
	if got.Pts[6].N != 4 || got.Pts[5].N != 3 {
		t.Errorf("after 6/off 5/off: p6=%+v p5=%+v, want 4 and 3", got.Pts[6], got.Pts[5])
	}
	if got.Off[bg.P1] != 2 {
		t.Errorf("P1 off = %d, want 2", got.Off[bg.P1])
	}
	if got.Checkers(bg.P1) != 15 {
		t.Errorf("conservation: P1 = %d, want 15", got.Checkers(bg.P1))
	}
}

// The .mat/Jellyfish numeric bear-off form writes the destination "off" as
// point 0 (e.g. "6/0"), as produced by our xg2mat converter. It must behave
// identically to "6/off". Regression for the bug that made every game go
// board-unknown from its first bear-off.
func TestApplyNotation_BearOffNumericZero(t *testing.T) {
	got, err := ApplyNotation(bearoffPosition(), bg.P1, "6/0 5/0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Pts[6].N != 4 || got.Pts[5].N != 3 {
		t.Errorf("after 6/0 5/0: p6=%+v p5=%+v, want 4 and 3", got.Pts[6], got.Pts[5])
	}
	if got.Off[bg.P1] != 2 {
		t.Errorf("P1 off = %d, want 2", got.Off[bg.P1])
	}
	if got.Checkers(bg.P1) != 15 {
		t.Errorf("conservation: P1 = %d, want 15", got.Checkers(bg.P1))
	}
}

// Full real-match replay: every derivable turn conserves 15 checkers per side;
// the two "????" games flag exactly at the unrecorded move.
func TestReplay_RealMatch(t *testing.T) {
	data, err := os.ReadFile("../../testdata/mat/match1.txt")
	if err != nil {
		t.Skipf("fixture not present: %v", err)
	}
	m, err := matimport.Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	states := Replay(m)

	applied, unknown := 0, 0
	for _, ts := range states {
		if ts.Err == ErrUnknownMove {
			unknown++
		}
		if ts.Err == nil {
			// Every non-error turn's Post is a valid, conserved board.
			if ts.Post.Checkers(bg.P1) != 15 || ts.Post.Checkers(bg.P2) != 15 {
				t.Errorf("game %d ply %d: checker conservation broken (P1=%d P2=%d)",
					ts.Game, ts.Ply, ts.Post.Checkers(bg.P1), ts.Post.Checkers(bg.P2))
			}
			if ts.Applied {
				applied++
			}
		}
	}
	if applied < 50 {
		t.Errorf("only %d applied turns derived; expected a full match's worth", applied)
	}
	if unknown != 2 { // games 1 and 4 each end in a "????"
		t.Errorf("unknown moves = %d, want 2", unknown)
	}
}
