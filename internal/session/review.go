// Review-queue resolution (issue #21; ux-spec §7): one mechanism for the
// whole app — a Review Item resolves either by editing its turn through the
// normal entry flow, or by marking it reviewed-as-is.
package session

import "fmt"

// MarkReviewed resolves every open Review Item on the turn at seq without
// changing the entry — the human looked again and it was right.
func (s *Service) MarkReviewed(seq int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for i := range s.reviews {
		if s.reviews[i].TurnSeq == seq && !s.reviews[i].Resolved {
			s.reviews[i].Resolved = true
			n++
		}
	}
	if n == 0 {
		return fmt.Errorf("no open review item on turn %d", seq)
	}
	if err := s.save(); err != nil {
		return fmt.Errorf("autosave: %w", err)
	}
	return nil
}

// resolveReviewsLocked marks open items on seq resolved (the edit path).
func (s *Service) resolveReviewsLocked(seq int) {
	for i := range s.reviews {
		if s.reviews[i].TurnSeq == seq && !s.reviews[i].Resolved {
			s.reviews[i].Resolved = true
		}
	}
}
