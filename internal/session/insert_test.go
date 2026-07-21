package session

import (
	"testing"

	"lazybg/internal/bg"
)

// Inserting a skipped turn before an existing one shifts the downstream turns
// and re-derives the board chain (issue #25, functional-spec §3).
func TestInsertTurn_InsertsShiftsAndReDerives(t *testing.T) {
	s := New()
	mustEnter(t, s, 3, 1, 0)    // T0: P1 opening 8/5 6/5 (best)
	mustEnter(t, s, 6, 2, 1000) // T1: P2
	before := s.Moves()[1].Notation

	// Insert a P1 dance (cannot-move) before T1 — a board-neutral turn, so the
	// downstream turn stays legal and simply shifts to seq 2.
	if err := s.InsertTurn(1, int(bg.P1), 6, 6, "", 500); err != nil {
		t.Fatal(err)
	}
	moves := s.Moves()
	if len(moves) != 3 {
		t.Fatalf("moves after insert = %d, want 3", len(moves))
	}
	if !moves[1].CannotMove || moves[1].Player != int(bg.P1) {
		t.Fatalf("inserted turn = %+v, want a P1 cannot-move", moves[1])
	}
	if moves[2].Notation != before {
		t.Fatalf("downstream turn = %q, want it shifted intact (%q)", moves[2].Notation, before)
	}
	// A board-neutral insert breaks nothing downstream.
	for _, it := range s.ReviewItems() {
		if it.Reason == ReasonCascade {
			t.Fatalf("board-neutral insert raised a cascade flag: %+v", it)
		}
	}
}

// An inserted move that is not physically applicable on the board before it is
// rejected (ADR-0001: legality is a prior, but the move must at least fit).
func TestInsertTurn_RejectsInapplicable(t *testing.T) {
	s := New()
	mustEnter(t, s, 3, 1, 0)
	// P1 has no checker that can play "1/2" — an impossible notation.
	if err := s.InsertTurn(1, int(bg.P1), 2, 1, "1/2", 0); err == nil {
		t.Fatal("inserting an inapplicable move should fail")
	}
}

// Deleting a cube ply re-derives the live cube state (issue #25).
func TestDeleteTurn_CubePlyReDerivesCube(t *testing.T) {
	s := New()
	if _, err := s.EnterCube("double", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterCube("take", 200); err != nil {
		t.Fatal(err)
	}
	if s.CubeValue() != 2 {
		t.Fatalf("cube after take = %d, want 2", s.CubeValue())
	}
	// Delete the take (seq 1): the double is left pending again, value back to 1.
	if err := s.DeleteTurn(1); err != nil {
		t.Fatal(err)
	}
	if s.CubeValue() != 1 {
		t.Fatalf("cube after deleting the take = %d, want 1 (centered)", s.CubeValue())
	}
	if got := s.CubeActions(); len(got) != 2 || got[0] != "take" {
		t.Fatalf("actions after deleting the take = %v, want [take drop] (double pending again)", got)
	}
}

// Inserting a cube ply re-derives the cube state and follows in the chain.
func TestInsertCube_ReDerivesCube(t *testing.T) {
	s := New()
	mustEnter(t, s, 3, 1, 0) // T0: a checker turn
	// Insert a double before T1 (i.e. at the end here): cube becomes pending.
	if err := s.InsertCube(1, "double", 500); err != nil {
		t.Fatal(err)
	}
	moves := s.Moves()
	if len(moves) != 2 || moves[1].Cube != "double" {
		t.Fatalf("moves after cube insert = %+v, want a trailing double", moves)
	}
	if got := s.CubeActions(); len(got) != 2 || got[0] != "take" {
		t.Fatalf("actions after inserting a double = %v, want [take drop]", got)
	}
}

// mustEnter records one confirmed best-candidate turn.
func mustEnter(t *testing.T, s *Service, d1, d2, tick int) {
	t.Helper()
	if _, err := s.EnterDice(d1, d2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, tick); err != nil {
		t.Fatal(err)
	}
}
