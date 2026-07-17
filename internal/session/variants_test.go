package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/bg"
)

// Shift+Space semantics (functional-spec §4, ux-spec §2): the ply is applied
// immediately — the human did commit — AND a human-flagged Review Item opens
// alongside it for a later second pass.
func TestConfirmFlag_AppliesAndOpensReviewItem(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	ply, err := s.ConfirmFlag(0, 7000)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Moves()) != 1 {
		t.Fatal("flagged confirm did not apply the ply")
	}
	items := s.ReviewItems()
	if len(items) != 1 {
		t.Fatalf("review items = %d, want 1", len(items))
	}
	it := items[0]
	if it.Reason != "human-flagged" || it.TickMs != 7000 || it.Notation != ply.Notation {
		t.Fatalf("review item wrong: %+v", it)
	}
}

// The override escape hatch (ADR-0001): free entry, never blocked by engine
// legality — only physically impossible notations (no checker to move) fail.
func TestOverride_FreeEntryBeyondCandidates(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	// "24/20" uses a 4 — ILLEGAL for a 3-1 roll, but physically possible.
	// A human witnessed it; the app records it.
	ply, err := s.Override("24/20", 500)
	if err != nil {
		t.Fatalf("override rejected an illegal-but-physical move: %v", err)
	}
	if ply.Notation != "24/20" || ply.Dice != "31" {
		t.Fatalf("override ply wrong: %+v", ply)
	}
	b := s.Board()
	if b.Pts[20].N != 1 || b.Pts[20].Owner != bg.P1 {
		t.Fatalf("override not applied to the board: pt20=%+v", b.Pts[20])
	}
	if s.OnRoll() != 1 {
		t.Fatal("override must alternate the turn like any confirm")
	}
}

func TestOverride_PhysicallyImpossibleFails(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	// Point 20 is empty at the start — nothing there to move.
	if _, err := s.Override("20/16", 0); err == nil {
		t.Fatal("physically impossible move accepted")
	}
}

// Override with empty notation records a Cannot Move (the human saw a dance
// the engine disagrees with — rare, but never blocked).
func TestOverride_EmptyNotationIsCannotMove(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	ply, err := s.Override("", 300)
	if err != nil {
		t.Fatal(err)
	}
	if !ply.CannotMove {
		t.Fatalf("empty override should be Cannot Move: %+v", ply)
	}
	if s.Board() != bg.StandardStart() {
		t.Fatal("cannot-move must leave the board unchanged")
	}
}

// Automatic dance (functional-spec §4): once dice yield zero legal moves the
// turn records itself — no candidate step, nothing to choose.
func TestEnterDiceAt_DanceAutoRecords(t *testing.T) {
	s := New()
	// P1 on the bar against a closed board: P2 owns every entry point (24..19).
	var b bg.Board
	b.Bar[bg.P1] = 1
	for pt := 19; pt <= 24; pt++ {
		b.Pts[pt] = bg.Point{N: 2, Owner: bg.P2}
	}
	// Park the remaining checkers plausibly.
	b.Pts[13] = bg.Point{N: 7, Owner: bg.P1}
	b.Pts[6] = bg.Point{N: 7, Owner: bg.P1}
	b.Pts[1] = bg.Point{N: 3, Owner: bg.P2}
	s.board = b

	res, err := s.EnterDiceAt(5, 2, 4200)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Danced {
		t.Fatal("closed board not detected as a dance")
	}
	if len(res.Candidates) != 0 {
		t.Fatal("dance must not present candidates")
	}
	moves := s.Moves()
	if len(moves) != 1 || !moves[0].CannotMove || moves[0].TickMs != 4200 {
		t.Fatalf("dance not auto-recorded: %+v", moves)
	}
	if s.OnRoll() != 1 {
		t.Fatal("dance must pass the turn")
	}
}

// Review items persist in the .lbg and survive reopen.
func TestReviewItems_PersistAndReload(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "v.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _, err := Create(filepath.Join(dir, "v.lbg"), video, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmFlag(1, 900); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join(dir, "v.lbg"))
	var doc LBG
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Review) != 1 || doc.Review[0].Reason != "human-flagged" {
		t.Fatalf("persisted review = %+v", doc.Review)
	}

	s2, _, err := Open(filepath.Join(dir, "v.lbg"))
	if err != nil {
		t.Fatal(err)
	}
	if items := s2.ReviewItems(); len(items) != 1 || items[0].Reason != "human-flagged" {
		t.Fatalf("reloaded review items = %+v", items)
	}
}
