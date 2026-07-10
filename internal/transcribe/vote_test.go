package transcribe

import (
	"testing"

	"lazybg/internal/perceive"
)

func TestVoteObservations(t *testing.T) {
	mk := func(p6 int, conf float64) perceive.ObservedBoard {
		var ob perceive.ObservedBoard
		for p := 1; p <= 24; p++ {
			ob.Points[p] = perceive.PointObs{Confidence: 1}
		}
		ob.Points[6] = perceive.PointObs{Count: p6, Side: perceive.A, Confidence: conf}
		ob.Points[8] = perceive.PointObs{Count: 3, Side: perceive.B, Confidence: 0.9}
		return ob
	}
	// Two reads say 5 checkers on point 6, one (sporadic occlusion) says 3.
	voted := VoteObservations([]perceive.ObservedBoard{mk(5, 0.9), mk(3, 0.8), mk(5, 0.7)})

	if got := voted.Points[6]; got.Count != 5 || got.Side != perceive.A {
		t.Errorf("point 6 voted %d/%v, want 5/A", got.Count, got.Side)
	}
	// Confidence reflects the 2/3 agreement — strictly below a unanimous point.
	if voted.Points[6].Confidence >= voted.Points[8].Confidence {
		t.Errorf("split vote conf %.2f should be below unanimous conf %.2f",
			voted.Points[6].Confidence, voted.Points[8].Confidence)
	}

	// Single reading passes through.
	one := VoteObservations([]perceive.ObservedBoard{mk(4, 0.6)})
	if one.Points[6].Count != 4 {
		t.Errorf("single-read vote altered the reading")
	}
}
