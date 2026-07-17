package session

import (
	"testing"
)

// Resolving by edit: correcting a flagged turn through the normal entry flow
// (ReplaceTurn) closes its open Review Items — one resolution mechanism for
// the whole app (ux-spec §7).
func TestReview_ResolvedByEdit(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmFlag(0, 100); err != nil {
		t.Fatal(err)
	}
	if len(s.ReviewItems()) != 1 {
		t.Fatal("expected one human-flagged item")
	}
	cands, err := s.CandidatesFor(0, 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceTurn(0, 3, 1, cands[0].Notation); err != nil {
		t.Fatal(err)
	}
	if items := s.ReviewItems(); len(items) != 0 {
		t.Fatalf("edit did not resolve the item: %+v", items)
	}
}

// Resolving as-is: the human looked again and the entry was right.
func TestReview_MarkReviewed(t *testing.T) {
	s := New()
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmFlag(0, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkReviewed(0); err != nil {
		t.Fatal(err)
	}
	if items := s.ReviewItems(); len(items) != 0 {
		t.Fatalf("mark-reviewed did not resolve: %+v", items)
	}
	// Resolving an unflagged turn is a no-op error.
	if err := s.MarkReviewed(0); err == nil {
		t.Fatal("resolving an already-resolved turn should error")
	}
}

// The resolved flag persists (resolution history is part of the .lbg's
// training payoff — a resolved item is a labeled correction).
func TestReview_ResolutionPersists(t *testing.T) {
	s, lbgPath := newFileSession(t)
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmFlag(0, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkReviewed(0); err != nil {
		t.Fatal(err)
	}
	s2, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if items := s2.ReviewItems(); len(items) != 0 {
		t.Fatalf("resolved item reopened after reload: %+v", items)
	}
}
