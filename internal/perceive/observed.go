// Package perceive holds the shared perception types produced by the board
// detectors (docs/domain-model.md §3). An ObservedBoard is a noisy reading, not
// a validated Position — it records what was seen at a stable instant and how
// sure the reader was, per point.
package perceive

// Side identifies which player's checkers occupy a point (or None if empty).
// It maps to the profile's CheckerA / CheckerB.
type Side int

const (
	None Side = iota
	A
	B
)

func (s Side) String() string {
	switch s {
	case A:
		return "A"
	case B:
		return "B"
	}
	return "none"
}

// PointObs is the reading at a single point.
type PointObs struct {
	Count      int     // number of checkers, 0 if empty
	Side       Side    // occupying color, None if empty
	Confidence float64 // [0,1]
}

// ObservedBoard is the per-point reading of the whole board. Index 1..24 hold
// the points; index 0 is unused so point numbers read naturally.
type ObservedBoard struct {
	Points [25]PointObs
}
