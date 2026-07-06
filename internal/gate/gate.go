// Package gate maps a MoveDecision's confidence to auto-fill vs needs-review
// (docs/domain-model.md §4, docs/architecture.md §3). It is the single,
// inspectable place where product tuning lives — pure and exhaustively testable.
package gate

import "lazybg/internal/cue"

// Outcome is the gate's verdict for a MoveDecision.
type Outcome int

const (
	AutoFill Outcome = iota
	NeedsReview
)

func (o Outcome) String() string {
	if o == AutoFill {
		return "auto-fill"
	}
	return "needs-review"
}

// Policy is the threshold policy. Threshold is the minimum joint confidence to
// auto-fill. Conservative by design: until confidence is calibrated, keep the
// threshold high so the system over-refers to human review (domain-model §4).
type Policy struct {
	Threshold float64
}

// Default is the starting policy for the MVP: deliberately cautious.
func Default() Policy { return Policy{Threshold: 0.8} }

// Classify returns the outcome and a short human-readable reason.
func (p Policy) Classify(d cue.MoveDecision) (Outcome, string) {
	if d.Top.Notation == "" {
		return NeedsReview, "no candidate move"
	}
	if d.Confidence >= p.Threshold {
		return AutoFill, "confidence above threshold"
	}
	return NeedsReview, "confidence below threshold"
}
