package gate

import (
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
)

func decision(conf float64, notation string) cue.MoveDecision {
	return cue.MoveDecision{
		Player:     bg.P1,
		Confidence: conf,
		Top:        cue.MoveHypothesis{Notation: notation},
	}
}

func TestClassify(t *testing.T) {
	p := Default() // threshold 0.8
	cases := []struct {
		name string
		d    cue.MoveDecision
		want Outcome
	}{
		{"above threshold auto-fills", decision(0.85, "8/5 6/5"), AutoFill},
		{"exactly at threshold auto-fills", decision(0.80, "8/5 6/5"), AutoFill},
		{"below threshold reviews", decision(0.79, "8/5 6/5"), NeedsReview},
		{"no candidate reviews regardless of confidence", decision(0.99, ""), NeedsReview},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, reason := p.Classify(c.d)
			if got != c.want {
				t.Errorf("Classify() = %v (%s), want %v", got, reason, c.want)
			}
		})
	}
}

func TestPolicy_ThresholdTunable(t *testing.T) {
	lenient := Policy{Threshold: 0.5}
	if got, _ := lenient.Classify(decision(0.6, "8/5")); got != AutoFill {
		t.Errorf("with a lenient threshold 0.6 should auto-fill, got %v", got)
	}
}
