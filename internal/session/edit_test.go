package session

import (
	"strings"
	"testing"

	"lazybg/internal/derive"
)

// pickCandidate returns the index of the first candidate whose canonical form
// matches, or -1.
func pickCandidate(t *testing.T, cands []Candidate, notation string) int {
	t.Helper()
	want, err := derive.CanonicalPlays(notation)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cands {
		got, err := derive.CanonicalPlays(c.Notation)
		if err != nil {
			continue
		}
		if got == want {
			return i
		}
	}
	return -1
}

// CandidatesFor re-opens the normal entry flow at a past turn: candidates
// computed on the board BEFORE that turn (ux-spec §4).
func TestCandidatesFor_UsesBoardBefore(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterDice(6, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	// Re-open turn 0: same position as the original opening 3-1.
	cands, err := s.CandidatesFor(0, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) == 0 || cands[0].Notation != "8/5 6/5" {
		t.Fatalf("re-opened turn 0 candidates wrong: %+v", cands[:min(3, len(cands))])
	}
}

// The cascade (functional-spec §5): editing an upstream turn re-validates
// downstream turns; ones no longer legal are flagged as Review Items —
// nothing is deleted or silently overwritten.
func TestReplaceTurn_CascadeFlagsIllegalDownstream(t *testing.T) {
	s := New()
	// T0: P1 6-1, pick a move that does NOT make the bar point (abs 7) and
	// leaves a blot on abs 18: "24/18 6/5".
	cands, err := s.EnterDice(6, 1)
	if err != nil {
		t.Fatal(err)
	}
	i := pickCandidate(t, cands, "24/18 6/5")
	if i < 0 {
		t.Fatalf("24/18 6/5 not among 6-1 candidates")
	}
	if _, err := s.Confirm(i, 1000); err != nil {
		t.Fatal(err)
	}
	// T1: P2 6-6 with both back checkers running 24/18(2) (abs 1→7, needs
	// abs 7 open) and 13/7*(2) (abs 12→18, hitting the blot).
	cands, err = s.EnterDice(6, 6)
	if err != nil {
		t.Fatal(err)
	}
	j := pickCandidate(t, cands, "24/18 24/18 13/7 13/7")
	if j < 0 {
		t.Fatalf("double-run 6-6 not among candidates")
	}
	if _, err := s.Confirm(j, 2000); err != nil {
		t.Fatal(err)
	}

	// Edit T0 to the bar-point play: abs 7 now blocked, no blot on 18 —
	// T1's recorded move is no longer legal.
	if err := s.ReplaceTurn(0, 6, 1, "13/7 8/7"); err != nil {
		t.Fatal(err)
	}

	moves := s.Moves()
	if len(moves) != 2 {
		t.Fatalf("cascade deleted a turn: %d moves", len(moves))
	}
	if moves[0].Notation != "13/7 8/7" {
		t.Fatalf("edited turn = %q", moves[0].Notation)
	}
	if !strings.Contains(moves[1].Notation, "24/18") {
		t.Fatalf("downstream turn altered: %q", moves[1].Notation)
	}
	items := s.ReviewItems()
	if len(items) != 1 || items[0].Reason != "cascade" || items[0].TurnSeq != 1 {
		t.Fatalf("cascade review items = %+v", items)
	}
}

// An edit that keeps every downstream turn legal flags nothing.
func TestReplaceTurn_NoFalseFlags(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil { // 8/5 6/5
		t.Fatal(err)
	}
	cands, err := s.EnterDice(6, 5)
	if err != nil {
		t.Fatal(err)
	}
	j := pickCandidate(t, cands, "24/18 18/13") // lover's leap, untouched by P1's home
	if j < 0 {
		t.Fatal("24/13 run not among 6-5 candidates")
	}
	if _, err := s.Confirm(j, 0); err != nil {
		t.Fatal(err)
	}
	// Re-edit T0 to another 3-1 play that cannot interfere with P2's run.
	cands, err = s.CandidatesFor(0, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	i := pickCandidate(t, cands, "13/10 24/23")
	if i < 0 {
		t.Fatal("13/10 24/23 not among 3-1 candidates")
	}
	if err := s.ReplaceTurn(0, 3, 1, cands[i].Notation); err != nil {
		t.Fatal(err)
	}
	if items := s.ReviewItems(); len(items) != 0 {
		t.Fatalf("false cascade flags: %+v", items)
	}
	// The live board reflects the re-applied chain.
	if s.Board().Pts[5].N != 0 {
		t.Fatal("old edit's 5-point still on the board — chain not recomputed")
	}
}

// Editing can change the dice too (typo fix); the board chain recomputes.
func TestReplaceTurn_NewDice(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	cands, err := s.CandidatesFor(0, 6, 1)
	if err != nil {
		t.Fatal(err)
	}
	i := pickCandidate(t, cands, "13/7 8/7")
	if i < 0 {
		t.Fatal("bar point not among 6-1 candidates")
	}
	if err := s.ReplaceTurn(0, 6, 1, cands[i].Notation); err != nil {
		t.Fatal(err)
	}
	m := s.Moves()[0]
	if m.Dice != "61" || m.Notation != cands[i].Notation {
		t.Fatalf("edited ply = %+v", m)
	}
	if s.Board().Pts[7].N != 2 {
		t.Fatal("board not recomputed for the new move")
	}
}

func TestDeleteTurn_RemovesAndCascades(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterDice(6, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 200); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTurn(0); err != nil {
		t.Fatal(err)
	}
	moves := s.Moves()
	if len(moves) != 1 || moves[0].Dice != "65" {
		t.Fatalf("after delete: %+v", moves)
	}
	// The remaining turn now belongs to a chain where P1 never moved; it is
	// still legal (P2's opening 6-5 run) so no flag — but alternation is now
	// P2-then-P1, and the board must reflect only the remaining ply.
	if s.Board().Pts[5].N == 2 {
		t.Fatal("deleted ply still on the board")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
