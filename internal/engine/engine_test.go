package engine

import (
	"strings"
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

// The unscored fast path must enumerate exactly the same legal moves as the
// scored one — only the equity evaluation is skipped.
func TestLegalMovesUnscored_SameMoveSet(t *testing.T) {
	pos := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	scored, err := LegalMoves(pos)
	if err != nil {
		t.Fatal(err)
	}
	fast, err := LegalMovesUnscored(pos)
	if err != nil {
		t.Fatal(err)
	}
	if len(fast) != len(scored) {
		t.Fatalf("unscored %d moves, scored %d", len(fast), len(scored))
	}
	want := map[string]bool{}
	for _, m := range scored {
		want[m.Notation] = true
	}
	for _, m := range fast {
		if !want[m.Notation] {
			t.Errorf("unscored produced %q, absent from scored set", m.Notation)
		}
		if m.Equity != 0 {
			t.Errorf("unscored move %q has equity %v, want 0", m.Notation, m.Equity)
		}
	}
}

// Player-mapping regression: an asymmetric position (P1 on the bar) forces
// EVERY P1 move to enter from the bar, while P2 moves freely. The original
// wrapper passed the on-roll player straight through to gnubg's FindMoves,
// which actually moves the OTHER slot — invisible on the mirror-symmetric
// standard start every earlier test used.
func TestLegalMoves_PlayerMappingAsymmetric(t *testing.T) {
	b := bg.StandardStart()
	b.Pts[24] = bg.Point{N: 1, Owner: bg.P1} // one of P1's two back checkers…
	b.Bar[bg.P1] = 1                         // …is on the bar instead

	p1Moves, err := LegalMoves(bg.Position{Board: b, Dice: bg.Dice{5, 2}, PlayerOnRoll: bg.P1})
	if err != nil {
		t.Fatal(err)
	}
	if len(p1Moves) == 0 {
		t.Fatal("P1 has legal entries from the bar")
	}
	for _, m := range p1Moves {
		if !strings.HasPrefix(m.Notation, "bar/") {
			t.Fatalf("P1 is on the bar but got move %q", m.Notation)
		}
	}

	p2Moves, err := LegalMoves(bg.Position{Board: b, Dice: bg.Dice{5, 2}, PlayerOnRoll: bg.P2})
	if err != nil {
		t.Fatal(err)
	}
	forced := 0
	for _, m := range p2Moves {
		if strings.HasPrefix(m.Notation, "bar/") {
			forced++
		}
	}
	if forced != 0 {
		t.Errorf("P2 is not on the bar but %d/%d moves enter from it", forced, len(p2Moves))
	}
}

// Route fidelity: a combined hop must be represented via its LEGAL
// intermediate point so the notation replays outside the engine. (24/17* here
// must route via the open 20-point, not through the opponent-held 21-point.)
func TestLegalMoves_ChainedHopUsesLegalIntermediate(t *testing.T) {
	b := bg.StandardStart()
	// P2 holds abs 21 (two checkers) and leaves a blot on abs 17.
	b.Pts[21] = bg.Point{N: 2, Owner: bg.P2}
	b.Pts[17] = bg.Point{N: 1, Owner: bg.P2}
	b.Pts[19] = bg.Point{N: 2, Owner: bg.P2} // keep checker count plausible
	b.Pts[12] = bg.Point{N: 2, Owner: bg.P2}
	b.Pts[1] = bg.Point{N: 1, Owner: bg.P2}

	moves, err := LegalMoves(bg.Position{Board: b, Dice: bg.Dice{4, 3}, PlayerOnRoll: bg.P1})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range moves {
		if strings.Contains(m.Notation, "24/21") {
			t.Errorf("move %q routes through the blocked 21-point", m.Notation)
		}
	}
}
