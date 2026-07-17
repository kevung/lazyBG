package session

import (
	"os"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/engine"
)

func TestMain(m *testing.M) {
	// gnubg.Init expects an FS that contains a data/ subdir, so pass the repo root.
	if err := engine.Init(os.DirFS("../..")); err != nil {
		panic("engine init: " + err.Error())
	}
	os.Exit(m.Run())
}

// The ranked-candidate entry path (functional-spec §4): entering the dice
// yields a short equity-ranked list of legal moves, best first. 3-1 for P1 on
// the standard start must rank the 5-point play ("8/5 6/5") first — gnubg's
// canonical top choice.
func TestEnterDice_RanksCandidatesByEquity(t *testing.T) {
	s := New()
	cands, err := s.EnterDice(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) == 0 {
		t.Fatal("no candidates for an opening 3-1")
	}
	if len(cands) > MaxCandidates {
		t.Fatalf("candidate list too long: %d > %d", len(cands), MaxCandidates)
	}
	if cands[0].Notation != "8/5 6/5" {
		t.Fatalf("top candidate = %q, want the 5-point play 8/5 6/5", cands[0].Notation)
	}
	for i := 1; i < len(cands); i++ {
		if cands[i].Equity > cands[i-1].Equity {
			t.Fatalf("candidates not sorted by equity desc at %d", i)
		}
	}
}

// Confirming a candidate applies the ply (tick + confidence 0 = human entry,
// bg.Ply contract) and alternates the player on roll.
func TestConfirm_AppliesPlyAndAlternates(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	ply, err := s.Confirm(0, 12345)
	if err != nil {
		t.Fatal(err)
	}
	if ply.Player != 0 || ply.Dice != "31" || ply.Notation != "8/5 6/5" || ply.TickMs != 12345 {
		t.Fatalf("unexpected ply view: %+v", ply)
	}
	moves := s.Moves()
	if len(moves) != 1 || moves[0] != ply {
		t.Fatalf("move list = %+v, want the confirmed ply", moves)
	}
	// Next turn belongs to the other player.
	if _, err := s.EnterDice(6, 6); err != nil {
		t.Fatal(err)
	}
	ply2, err := s.Confirm(0, 20000)
	if err != nil {
		t.Fatal(err)
	}
	if ply2.Player != 1 {
		t.Fatalf("second ply player = %d, want 1 (alternation)", ply2.Player)
	}
}

// The transcriber watches who actually plays first: the pending turn's player
// is settable before dice are confirmed (first turn of a game, mainly).
func TestSetTurnPlayer(t *testing.T) {
	s := New()
	if err := s.SetTurnPlayer(1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterDice(5, 2); err != nil {
		t.Fatal(err)
	}
	ply, err := s.Confirm(0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if ply.Player != 1 {
		t.Fatalf("ply player = %d, want 1", ply.Player)
	}
	// Alternation resumes from the chosen player.
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	ply2, err := s.Confirm(0, 200)
	if err != nil {
		t.Fatal(err)
	}
	if ply2.Player != 0 {
		t.Fatalf("next ply player = %d, want 0", ply2.Player)
	}
}

// Confirming a candidate must leave the board in the engine's resulting
// position — the chain the next turn's candidates are generated from.
func TestConfirm_AdvancesBoardChain(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	b := s.Board()
	// 8/5 6/5 for P1: point 5 now holds two P1 checkers.
	if b.Pts[5].N != 2 || b.Pts[5].Owner != bg.P1 {
		t.Fatalf("point 5 after 8/5 6/5 = %+v, want 2 P1 checkers", b.Pts[5])
	}
	if b.Pts[8].N != 2 || b.Pts[6].N != 4 {
		t.Fatalf("source points wrong: pt8=%+v pt6=%+v", b.Pts[8], b.Pts[6])
	}
}

func TestEnterDice_Validation(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(0, 3); err == nil {
		t.Fatal("die 0 accepted")
	}
	if _, err := s.EnterDice(1, 7); err == nil {
		t.Fatal("die 7 accepted")
	}
}

func TestConfirm_Errors(t *testing.T) {
	s := New()
	if _, err := s.Confirm(0, 0); err == nil {
		t.Fatal("confirm with no pending dice accepted")
	}
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(999, 0); err == nil {
		t.Fatal("out-of-range candidate index accepted")
	}
	if _, err := s.Confirm(-1, 0); err == nil {
		t.Fatal("negative candidate index accepted")
	}
}

// Re-entering dice before confirming replaces the pending turn (typo fix, the
// cheap-error-recovery principle of functional-spec §4).
func TestEnterDice_ReplacesPending(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	cands, err := s.EnterDice(6, 5)
	if err != nil {
		t.Fatal(err)
	}
	if cands[0].Notation != "24/18 18/13" && cands[0].Notation != "24/13" {
		t.Logf("top 6-5 candidate: %q", cands[0].Notation)
	}
	ply, err := s.Confirm(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ply.Dice != "65" {
		t.Fatalf("confirmed dice = %q, want 65 (the re-entered roll)", ply.Dice)
	}
}
