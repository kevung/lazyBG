package session

import (
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/bg"
)

// The cube menu is the small fixed action set filtered by cube state
// (functional-spec §4, ux-spec §9): center cube → the player on roll may
// double; pending double → the opponent may take or drop; owned by the
// opponent → nothing.
func TestCubeActions_FiltersByState(t *testing.T) {
	s := New()
	if got := s.CubeActions(); len(got) != 1 || got[0] != "double" {
		t.Fatalf("center-cube actions = %v, want [double]", got)
	}
	if _, err := s.EnterCube("double", 1000); err != nil {
		t.Fatal(err)
	}
	// Now the opponent must respond.
	if got := s.CubeActions(); len(got) != 2 || got[0] != "take" || got[1] != "drop" {
		t.Fatalf("pending-double actions = %v, want [take drop]", got)
	}
	if _, err := s.EnterCube("take", 2000); err != nil {
		t.Fatal(err)
	}
	// Cube now owned by the taker (P2); the doubler (P1, back on roll)
	// cannot redouble.
	if got := s.CubeActions(); len(got) != 0 {
		t.Fatalf("doubler-on-roll actions after take = %v, want none (cube owned by opponent)", got)
	}
	if s.CubeValue() != 2 {
		t.Fatalf("cube value after take = %d, want 2", s.CubeValue())
	}
}

// Cube() exposes value + owner + centered to the frontend board (issue #28):
// the board draws the cube on a player's side (or centered) with its value.
func TestCubeView_ReflectsOwnershipAndValue(t *testing.T) {
	s := New()
	if c := s.Cube(); c.Value != 1 || !c.Centered {
		t.Fatalf("start cube = %+v, want value 1, centered", c)
	}
	if _, err := s.EnterCube("double", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterCube("take", 200); err != nil {
		t.Fatal(err)
	}
	c := s.Cube()
	if c.Centered {
		t.Errorf("cube should be owned after a take, got centered")
	}
	if c.Value != 2 {
		t.Errorf("cube value = %d, want 2", c.Value)
	}
	if c.Owner != 1 {
		t.Errorf("cube owner = %d, want 1 (the taker)", c.Owner)
	}
}

func TestCube_DoubleRecordsPlyAndPassesTurn(t *testing.T) {
	s := New()
	ply, err := s.EnterCube("double", 500)
	if err != nil {
		t.Fatal(err)
	}
	if ply.Cube != "double" || ply.CubeValue != 2 || ply.Player != 0 {
		t.Fatalf("double ply = %+v", ply)
	}
	if s.OnRoll() != 1 {
		t.Fatal("double must pass the decision to the opponent")
	}
	// Dice entry is blocked while the cube decision is pending.
	if _, err := s.EnterDice(3, 1); err == nil {
		t.Fatal("dice accepted while a cube decision is pending")
	}
}

func TestCube_TakeReturnsRollToDoubler(t *testing.T) {
	s := New()
	if _, err := s.EnterCube("double", 0); err != nil {
		t.Fatal(err)
	}
	ply, err := s.EnterCube("take", 100)
	if err != nil {
		t.Fatal(err)
	}
	if ply.Cube != "take" || ply.Player != 1 {
		t.Fatalf("take ply = %+v", ply)
	}
	if s.OnRoll() != 0 {
		t.Fatal("after a take the doubler rolls")
	}
	// Dice entry works again.
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
}

func TestCube_InvalidActionsRejected(t *testing.T) {
	s := New()
	if _, err := s.EnterCube("take", 0); err == nil {
		t.Fatal("take accepted with no pending double")
	}
	if _, err := s.EnterCube("double", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterCube("double", 0); err == nil {
		t.Fatal("re-double accepted while pending")
	}
	if _, err := s.EnterCube("nonsense", 0); err == nil {
		t.Fatal("unknown action accepted")
	}
}

// Cube state survives the .lbg round trip.
func TestCube_PersistsAndReplays(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "v.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lbgPath := filepath.Join(dir, "v.lbg")
	s, _, err := Create(lbgPath, video, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterCube("double", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterCube("take", 200); err != nil {
		t.Fatal(err)
	}

	s2, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if s2.CubeValue() != 2 {
		t.Fatalf("reopened cube value = %d, want 2", s2.CubeValue())
	}
	if s2.OnRoll() != 0 {
		t.Fatal("reopened onRoll must be the doubler")
	}
	if got := s2.CubeActions(); len(got) != 0 {
		t.Fatalf("reopened doubler actions = %v, want none", got)
	}
	moves := s2.Moves()
	if len(moves) != 2 || moves[0].Cube != "double" || moves[1].Cube != "take" {
		t.Fatalf("reopened cube moves = %+v", moves)
	}
}

// Cube-in-play off (a Session Prior) removes the cube entirely: no actions
// ever, even from a centered cube (issue #24, functional-spec §3).
func TestCubeActions_AbsentWhenCubeNotInPlay(t *testing.T) {
	s := New()
	no := false
	s.priors.CubeInPlay = &no
	if got := s.CubeActions(); got != nil {
		t.Fatalf("cube-off actions = %v, want none", got)
	}
}

// In the Crawford game no doubling is allowed; the game after it (post-
// Crawford) restores the cube (issue #24). The Crawford game is the first
// game entered with a player one point from the match.
func TestCubeActions_NoDoublingInCrawfordGame(t *testing.T) {
	s := New()
	s.match.Length = 5
	// Game 2 is entered at 4-2: the leader just reached length-1 → Crawford.
	s.match.Games = []bg.Game{
		{Number: 1, StartScore: [2]int{0, 0}},
		{Number: 2, StartScore: [2]int{4, 2}},
	}
	if got := s.CubeActions(); got != nil {
		t.Fatalf("Crawford-game actions = %v, want none (no doubling)", got)
	}
	// Game 3 at 4-2 is post-Crawford: doubling is back.
	s.match.Games = append(s.match.Games, bg.Game{Number: 3, StartScore: [2]int{4, 2}})
	if got := s.CubeActions(); len(got) != 1 || got[0] != "double" {
		t.Fatalf("post-Crawford actions = %v, want [double]", got)
	}
	// Crawford disabled as a prior → doubling allowed even at match point.
	no := false
	s.priors.Crawford = &no
	s.match.Games = s.match.Games[:2] // back to the would-be Crawford game
	if got := s.CubeActions(); len(got) != 1 || got[0] != "double" {
		t.Fatalf("Crawford-off actions = %v, want [double]", got)
	}
}
