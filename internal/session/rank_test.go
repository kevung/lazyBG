package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/engine"
	"lazybg/internal/perceive"
)

func obsFromBoard(b bg.Board) perceive.ObservedBoard {
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

// Without an observation the ranking must be exactly the equity ranking —
// no regression from the equity-only skeleton (issue #15 acceptance).
func TestRank_NoObservation_EquityOrder(t *testing.T) {
	s := New()
	cands, err := s.EnterDice(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if cands[0].Notation != "8/5 6/5" {
		t.Fatalf("top = %q, want the equity-best 8/5 6/5", cands[0].Notation)
	}
	for i := 1; i < len(cands); i++ {
		if cands[i].Equity > cands[i-1].Equity {
			t.Fatalf("not equity-ordered at %d without an observation", i)
		}
	}
}

// With a confident post-move observation, the board-diff cue re-weights the
// list (architecture §4: w_boarddiff 0.5 > w_engine 0.2): the move whose
// resulting board matches what the camera saw must outrank the higher-equity
// alternative the pixels contradict.
func TestRank_ObservationReweights(t *testing.T) {
	// Find a non-top candidate to "observe" as what was actually played.
	pos := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	moves, err := engine.LegalMoves(pos)
	if err != nil {
		t.Fatal(err)
	}
	if len(moves) < 4 {
		t.Fatal("unexpectedly few legal 3-1 moves")
	}
	played := moves[3] // clearly not the equity-top move

	s := New()
	obs := obsFromBoard(played.Result)
	s.SetObservation(&obs)
	cands, err := s.EnterDice(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if cands[0].Notation != played.Notation {
		t.Fatalf("top with matching observation = %q, want %q", cands[0].Notation, played.Notation)
	}
	if cands[0].Score <= cands[1].Score {
		t.Fatal("fused score must strictly separate the observed move")
	}
}

// The observation applies to the pending turn only: confirming clears it.
func TestRank_ObservationClearedOnConfirm(t *testing.T) {
	pos := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	moves, _ := engine.LegalMoves(pos)
	played := moves[3]

	s := New()
	obs := obsFromBoard(played.Result)
	s.SetObservation(&obs)
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 0); err != nil {
		t.Fatal(err)
	}
	// Next turn: no observation → pure equity order again.
	cands, err := s.EnterDice(6, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(cands); i++ {
		if cands[i].Equity > cands[i-1].Equity {
			t.Fatalf("observation leaked into the next turn (not equity-ordered at %d)", i)
		}
	}
}

// Candidate traceability: the .lbg records which cues contributed
// (session-format-spec §3) — board-diff only when an observation was used.
func TestRank_CuesRecorded(t *testing.T) {
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

	// Turn 1: no observation.
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 100); err != nil {
		t.Fatal(err)
	}
	// Turn 2: with observation.
	pos := bg.Position{Board: s.Board(), Dice: bg.Dice{6, 2}, PlayerOnRoll: bg.P2}
	moves, err := engine.LegalMoves(pos)
	if err != nil || len(moves) == 0 {
		t.Fatalf("legal moves: %v", err)
	}
	obs := obsFromBoard(moves[0].Result)
	s.SetObservation(&obs)
	if _, err := s.EnterDice(6, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 200); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(lbgPath)
	var doc LBG
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if got := doc.Turns[0].Cues; len(got) != 1 || got[0] != "engine-equity" {
		t.Fatalf("turn 1 cues = %v, want [engine-equity]", got)
	}
	c2 := doc.Turns[1].Cues
	if len(c2) != 2 || c2[0] != "engine-equity" || c2[1] != "board-diff" {
		t.Fatalf("turn 2 cues = %v, want [engine-equity board-diff]", c2)
	}
}
