package session

import (
	"testing"

	"lazybg/internal/bg"
)

// BoardAt replays the chain up to a turn — the board panel's data source
// when a past turn is selected (ux-spec §6), and the cascade re-validation's
// foundation (issue #20).
func TestBoardAt_ReplaysPrefix(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	afterFirst := s.Board()
	if _, err := s.EnterDice(6, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}

	start, err := s.BoardAt(-1)
	if err != nil {
		t.Fatal(err)
	}
	if start != bg.StandardStart() {
		t.Fatal("BoardAt(-1) must be the starting position")
	}
	b0, err := s.BoardAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if b0 != afterFirst {
		t.Fatal("BoardAt(0) must equal the board after the first ply")
	}
	b1, err := s.BoardAt(1)
	if err != nil {
		t.Fatal(err)
	}
	if b1 != s.Board() {
		t.Fatal("BoardAt(last) must equal the live board")
	}
	if _, err := s.BoardAt(5); err == nil {
		t.Fatal("out-of-range seq accepted")
	}
}

// Across a game boundary the replay restarts from a fresh board.
func TestBoardAt_CrossesGameBoundary(t *testing.T) {
	s := New()
	if _, err := s.EnterCube("double", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterCube("drop", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmGameEnd(0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	// Seq 1 is the drop (game 1, board untouched → standard start).
	b, err := s.BoardAt(1)
	if err != nil {
		t.Fatal(err)
	}
	if b != bg.StandardStart() {
		t.Fatal("board after the drop must be the untouched start")
	}
	// Seq 2 is game 2's first checker play on a fresh board.
	b2, err := s.BoardAt(2)
	if err != nil {
		t.Fatal(err)
	}
	if b2 == bg.StandardStart() {
		t.Fatal("game-2 ply must advance a fresh board")
	}
	if b2 != s.Board() {
		t.Fatal("BoardAt(last) must equal the live board")
	}
}
