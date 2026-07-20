package bg

// Orientation is the board's on-screen orientation relative to the canonical
// numbering — a Capture Profile prior (docs/domain-model.md §Capture Profile,
// ADR-0006). It names the video quadrant that holds Player 1's home (inner)
// board. The core (Board, engine, .mat) always uses the canonical numbering
// (P1 home = points 1..6, bottom-right); Orientation is applied ONLY at the two
// boundaries — perception-in (observed region -> canonical point) and
// display-out (canonical point -> on-screen position). It never enters the core.
//
// The four values are the dihedral configurations preserving the "bar in the
// middle, two rows" structure. Each is an involution (its own inverse), so the
// same TransformPoint maps position->point and point->position.
type Orientation int

const (
	// P1HomeBottomRight is the canonical reference (identity): Player 1's home
	// board is in the bottom-right quadrant, matching bg.Board's numbering.
	P1HomeBottomRight Orientation = iota
	// P1HomeBottomLeft is the horizontal mirror.
	P1HomeBottomLeft
	// P1HomeTopRight is the vertical mirror.
	P1HomeTopRight
	// P1HomeTopLeft is the 180° rotation.
	P1HomeTopLeft
)

// The two bits: bit 0 = horizontal mirror, bit 1 = vertical mirror.
func (o Orientation) flipH() bool { return o&1 != 0 }
func (o Orientation) flipV() bool { return o&2 != 0 }

// AllOrientations returns the four orientations in canonical order.
func AllOrientations() []Orientation {
	return []Orientation{P1HomeBottomRight, P1HomeBottomLeft, P1HomeTopRight, P1HomeTopLeft}
}

// String renders the stable machine form used for persistence.
func (o Orientation) String() string {
	switch o {
	case P1HomeBottomRight:
		return "p1-home-bottom-right"
	case P1HomeBottomLeft:
		return "p1-home-bottom-left"
	case P1HomeTopRight:
		return "p1-home-top-right"
	case P1HomeTopLeft:
		return "p1-home-top-left"
	}
	return "p1-home-bottom-right"
}

// ParseOrientation maps a stored string onto the enum. It accepts the canonical
// String() forms and migrates the three legacy vocabularies (ADR-0006):
// "p1-right"/"p1-bottom" -> P1HomeBottomRight, "p1-left" -> P1HomeBottomLeft.
// The legacy forms only ever encoded the bearing side (P1 implicitly bottom).
func ParseOrientation(s string) (Orientation, bool) {
	switch s {
	case "p1-home-bottom-right", "p1-right", "p1-bottom", "":
		return P1HomeBottomRight, true
	case "p1-home-bottom-left", "p1-left":
		return P1HomeBottomLeft, true
	case "p1-home-top-right":
		return P1HomeTopRight, true
	case "p1-home-top-left":
		return P1HomeTopLeft, true
	}
	return P1HomeBottomRight, false
}

// FlipHorizontal returns the orientation mirrored left↔right (for the WYSIWYG
// mirror control, issue #37).
func (o Orientation) FlipHorizontal() Orientation { return o ^ 1 }

// FlipVertical returns the orientation mirrored top↔bottom.
func (o Orientation) FlipVertical() Orientation { return o ^ 2 }

// pointToCell maps a canonical point (1..24) to its grid cell under the identity
// orientation, matching the calibrate canonical board: top row 13..24 fill
// columns 0..11 left→right; bottom row 12..1 fill columns 0..11 left→right.
func pointToCell(p int) (col int, top bool) {
	if p >= 13 && p <= 24 {
		return p - 13, true
	}
	return 12 - p, false // 1..12
}

func cellToPoint(col int, top bool) int {
	if top {
		return 13 + col
	}
	return 12 - col
}

// TransformPoint maps between a canonical point number and the point occupying
// the same on-screen/region position once the board is placed in orientation o.
// Because every orientation is an involution the mapping is symmetric, so this
// one function serves both boundaries:
//   - display-out: the on-screen position of point p shows checkers of point
//     TransformPoint(p);
//   - perception-in: the reading at canonical region p belongs to point
//     TransformPoint(p).
//
// Points outside 1..24 (bar/off sentinels) pass through unchanged — orientation
// never renumbers the bar or the bearoff tray.
func (o Orientation) TransformPoint(p int) int {
	if p < 1 || p > 24 {
		return p
	}
	col, top := pointToCell(p)
	if o.flipH() {
		col = 11 - col
	}
	if o.flipV() {
		top = !top
	}
	return cellToPoint(col, top)
}
