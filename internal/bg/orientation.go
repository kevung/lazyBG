package bg

// Orientation is the board's on-screen orientation relative to the canonical
// numbering — a Capture Profile prior (docs/domain-model.md §Capture Profile,
// ADR-0006, ADR-0009). The core (Board, engine, .mat) always uses the canonical
// numbering (P1 home = points 1..6, bottom-right); Orientation is applied ONLY
// at the two boundaries — perception-in (observed region -> canonical point)
// and display-out (canonical point -> on-screen position). It never enters the
// core.
//
// **Player 1 is the player at the bottom of the video, by definition**
// (ADR-0009). Who is called "Player 1" is a naming convention, not a fact read
// off the capture: when the near player is the one entered second, the fix is
// to exchange the two players, not to turn the board over. Only the left/right
// mirror is therefore left — the direction of play, i.e. which half holds the
// two home boards.
type Orientation int

const (
	// P1HomeRight is the canonical reference (identity): the home boards are
	// in the right half, Player 1's in the bottom-right quadrant, matching
	// bg.Board's numbering.
	P1HomeRight Orientation = iota
	// P1HomeLeft is the horizontal mirror: the home boards are in the left
	// half, Player 1's in the bottom-left quadrant.
	P1HomeLeft
)

// flipH reports whether o mirrors left<->right.
func (o Orientation) flipH() bool { return o&1 != 0 }

// AllOrientations returns the orientations in canonical order.
func AllOrientations() []Orientation {
	return []Orientation{P1HomeRight, P1HomeLeft}
}

// String renders the stable machine form used for persistence.
func (o Orientation) String() string {
	if o == P1HomeLeft {
		return "p1-home-left"
	}
	return "p1-home-right"
}

// ParseOrientation maps a stored string onto the enum, migrating every
// vocabulary this repo has written: "p1-right"/"p1-bottom"/"" and the
// four-value "p1-home-bottom-right" -> P1HomeRight, "p1-left" and
// "p1-home-bottom-left" -> P1HomeLeft.
//
// The two "p1-home-top-*" forms also land on the side that keeps the home
// boards where they were — but that is only half of reading such a document,
// because its Player 1 is the top player. Callers holding a whole session must
// additionally exchange the players; LegacyTopOrientation reports exactly that
// case.
func ParseOrientation(s string) (Orientation, bool) {
	switch s {
	case "p1-home-right", "p1-home-bottom-right", "p1-home-top-right", "p1-right", "p1-bottom", "":
		return P1HomeRight, true
	case "p1-home-left", "p1-home-bottom-left", "p1-home-top-left", "p1-left":
		return P1HomeLeft, true
	}
	return P1HomeRight, false
}

// LegacyTopOrientation reports that s was written under the pre-ADR-0009 model
// with Player 1 on the top row. Such a document's players must be exchanged to
// be read under the current rule (see session.SwapPlayers).
func LegacyTopOrientation(s string) bool {
	return s == "p1-home-top-right" || s == "p1-home-top-left"
}

// FlipHorizontal returns the orientation mirrored left<->right — the single
// WYSIWYG mirror control (issue #37, ADR-0009).
func (o Orientation) FlipHorizontal() Orientation { return o ^ 1 }

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
// never renumbers the bar or the bearoff tray. Rows are never exchanged, which
// is what keeps Player 1 at the bottom.
func (o Orientation) TransformPoint(p int) int {
	if p < 1 || p > 24 {
		return p
	}
	col, top := pointToCell(p)
	if o.flipH() {
		col = 11 - col
	}
	return cellToPoint(col, top)
}
