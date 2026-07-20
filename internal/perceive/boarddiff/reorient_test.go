package boarddiff

import (
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/perceive"
)

func TestReorientIdentity(t *testing.T) {
	var ob perceive.ObservedBoard
	ob.Points[1] = perceive.PointObs{Count: 2, Side: perceive.A, Confidence: 0.9}
	ob.Points[24] = perceive.PointObs{Count: 5, Side: perceive.B, Confidence: 0.7}
	got := Reorient(ob, bg.P1HomeBottomRight)
	if got != ob {
		t.Errorf("identity Reorient changed the reading: %+v != %+v", got, ob)
	}
}

func TestReorientMovesPointsByTransform(t *testing.T) {
	var ob perceive.ObservedBoard
	// A reading sitting at the canonical region of point 1 (bottom-right cell).
	ob.Points[1] = perceive.PointObs{Count: 3, Side: perceive.A, Confidence: 0.8}
	// Under P1HomeBottomLeft the physical board is mirrored, so that cell's
	// checkers belong to canonical point 12 (TransformPoint(1) == 12).
	got := Reorient(ob, bg.P1HomeBottomLeft)
	if got.Points[12] != (perceive.PointObs{Count: 3, Side: perceive.A, Confidence: 0.8}) {
		t.Errorf("point-1 reading did not move to canonical point 12: %+v", got.Points[12])
	}
	if got.Points[1] != (perceive.PointObs{}) {
		t.Errorf("point-1 region should be empty after reorient, got %+v", got.Points[1])
	}
}

// Reorient preserves the multiset of readings (it only permutes indices) and is
// an involution for every orientation, matching bg.Orientation.
func TestReorientInvolution(t *testing.T) {
	var ob perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		ob.Points[p] = perceive.PointObs{Count: p % 4, Side: perceive.Side(p % 3), Confidence: float64(p) / 24}
	}
	for _, o := range bg.AllOrientations() {
		if got := Reorient(Reorient(ob, o), o); got != ob {
			t.Errorf("%v: Reorient not an involution", o)
		}
	}
}
