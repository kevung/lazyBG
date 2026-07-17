package session

import (
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/bg"
)

// nearlyDone puts P1 one checker from winning; loserOff controls whether the
// loss is plain (loser bore off) or a gammon candidate.
func nearlyDone(s *Service, loserOff int, loserInWinnerHome bool) {
	var b bg.Board
	b.Off[bg.P1] = 14
	b.Pts[1] = bg.Point{N: 1, Owner: bg.P1}
	b.Off[bg.P2] = loserOff
	rest := 15 - loserOff
	if loserInWinnerHome {
		b.Pts[3] = bg.Point{N: 1, Owner: bg.P2}
		rest--
	}
	b.Pts[19] = bg.Point{N: rest, Owner: bg.P2}
	s.board = b
	s.onRoll = bg.P1
}

// Bearing off the last checker is detected as a game end with a pre-filled
// result (functional-spec §5b): winner + points from the board, nothing
// typed from scratch.
func TestGameEnd_BearoffDetected(t *testing.T) {
	s := New()
	nearlyDone(s, 2, false)
	if s.PendingGameEnd() != nil {
		t.Fatal("game end detected before it happened")
	}
	if _, err := s.EnterDice(1, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 1000); err != nil {
		t.Fatal(err)
	}
	prop := s.PendingGameEnd()
	if prop == nil {
		t.Fatal("bear-off completion not detected")
	}
	if prop.Winner != 0 || prop.Points != 1 || prop.Gammon {
		t.Fatalf("proposal = %+v, want plain 1-point win for player 0", prop)
	}
	// Further entry is held until the boundary is confirmed.
	if _, err := s.EnterDice(3, 1); err == nil {
		t.Fatal("dice accepted with an unconfirmed game end")
	}
}

func TestGameEnd_GammonAndBackgammon(t *testing.T) {
	s := New()
	nearlyDone(s, 0, false) // loser bore off nothing → gammon
	if _, err := s.EnterDice(1, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	prop := s.PendingGameEnd()
	if prop == nil || !prop.Gammon || prop.Backgammon || prop.Points != 2 {
		t.Fatalf("gammon proposal = %+v", prop)
	}

	s2 := New()
	nearlyDone(s2, 0, true) // loser also trapped in the winner's home → backgammon
	if _, err := s2.EnterDice(1, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	prop2 := s2.PendingGameEnd()
	if prop2 == nil || !prop2.Backgammon || prop2.Points != 3 {
		t.Fatalf("backgammon proposal = %+v", prop2)
	}
}

// A drop ends the game for the doubler at the pre-double stake.
func TestGameEnd_DropDetected(t *testing.T) {
	s := New()
	if _, err := s.EnterCube("double", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterCube("drop", 100); err != nil {
		t.Fatal(err)
	}
	prop := s.PendingGameEnd()
	if prop == nil {
		t.Fatal("drop not detected as a game end")
	}
	if prop.Winner != 0 || prop.Points != 1 {
		t.Fatalf("drop proposal = %+v, want doubler wins 1 (pre-double stake)", prop)
	}
}

// Confirming the result closes the game, banks the score, and opens the next
// game on a fresh board; the human can correct the pre-filled values.
func TestConfirmGameEnd_OpensNextGame(t *testing.T) {
	s := New()
	nearlyDone(s, 2, false)
	if _, err := s.EnterDice(1, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	res, err := s.ConfirmGameEnd(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchOver {
		t.Fatal("unlimited session cannot be over")
	}
	if res.Score != [2]int{1, 0} {
		t.Fatalf("score = %v, want [1 0]", res.Score)
	}
	if s.Board() != bg.StandardStart() {
		t.Fatal("next game must start on a fresh board")
	}
	if s.PendingGameEnd() != nil {
		t.Fatal("boundary still pending after confirm")
	}
	if s.CubeValue() != 1 {
		t.Fatal("cube must recentre between games")
	}
	// Entry works again.
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
}

// Reaching the match length flags match end (functional-spec §5b: same
// principle, one mental model).
func TestConfirmGameEnd_MatchOver(t *testing.T) {
	s := New()
	s.match.Length = 1
	nearlyDone(s, 2, false)
	if _, err := s.EnterDice(1, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	res, err := s.ConfirmGameEnd(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !res.MatchOver {
		t.Fatal("match end not flagged at match length")
	}
}

// Multi-game state survives the .lbg round trip.
func TestGameEnd_PersistsAcrossReopen(t *testing.T) {
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
	// Game 1 ends by drop (replayable from the standard start, unlike a
	// hand-built bear-off board).
	if _, err := s.EnterCube("double", 100); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterCube("drop", 200); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmGameEnd(0, 1); err != nil {
		t.Fatal(err)
	}
	// One ply into game 2.
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 900); err != nil {
		t.Fatal(err)
	}

	s2, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Score(); got != [2]int{1, 0} {
		t.Fatalf("reopened score = %v, want [1 0]", got)
	}
	moves := s2.Moves()
	if len(moves) != 3 {
		t.Fatalf("reopened moves = %d, want 3 (double, drop, game-2 ply)", len(moves))
	}
	if moves[2].Game != 2 {
		t.Fatalf("game-2 ply game = %d, want 2", moves[2].Game)
	}
	if s2.Board() == bg.StandardStart() {
		t.Fatal("game 2's first ply must have advanced the fresh board")
	}
}
