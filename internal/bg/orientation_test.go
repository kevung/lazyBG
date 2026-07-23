package bg

import "testing"

func TestOrientationIdentity(t *testing.T) {
	for p := 1; p <= 24; p++ {
		if got := P1HomeRight.TransformPoint(p); got != p {
			t.Errorf("P1HomeRight.TransformPoint(%d) = %d, want %d (identity)", p, got, p)
		}
	}
}

func TestOrientationAnchors(t *testing.T) {
	// Hand-worked anchors from the calibrate canonical convention: top row
	// 13..24 at columns 0..11, bottom row 12..1 at columns 0..11, P1 home
	// (1..6) in the bottom-right half under the identity. Player 1 is the
	// bottom player by definition (ADR-0009), so only the left/right mirror
	// exists — the rows never exchange.
	cases := []struct {
		o       Orientation
		p, want int
	}{
		{P1HomeLeft, 1, 12},
		{P1HomeLeft, 6, 7},
		{P1HomeLeft, 24, 13},
		{P1HomeLeft, 13, 24},
	}
	for _, c := range cases {
		if got := c.o.TransformPoint(c.p); got != c.want {
			t.Errorf("%v.TransformPoint(%d) = %d, want %d", c.o, c.p, got, c.want)
		}
	}
}

// The mirror never moves a point across the rows: a top-row point stays on the
// top row. This is the property that makes "Player 1 sits at the bottom" an
// invariant of the renderer rather than a convention it has to remember.
func TestOrientationKeepsRows(t *testing.T) {
	for _, o := range AllOrientations() {
		for p := 1; p <= 24; p++ {
			if (p >= 13) != (o.TransformPoint(p) >= 13) {
				t.Errorf("%v.TransformPoint(%d) = %d crossed the rows", o, p, o.TransformPoint(p))
			}
		}
	}
}

// Every orientation is an involution: applying it twice is the identity.
func TestOrientationInvolution(t *testing.T) {
	for _, o := range AllOrientations() {
		for p := 1; p <= 24; p++ {
			if got := o.TransformPoint(o.TransformPoint(p)); got != p {
				t.Errorf("%v not an involution at %d: got %d", o, p, got)
			}
		}
	}
}

// TransformPoint is a bijection of 1..24 for every orientation.
func TestOrientationBijection(t *testing.T) {
	for _, o := range AllOrientations() {
		seen := map[int]bool{}
		for p := 1; p <= 24; p++ {
			q := o.TransformPoint(p)
			if q < 1 || q > 24 {
				t.Fatalf("%v.TransformPoint(%d) = %d out of range", o, p, q)
			}
			if seen[q] {
				t.Fatalf("%v.TransformPoint not injective: %d hit twice", o, q)
			}
			seen[q] = true
		}
	}
}

// Points outside 1..24 (bar/off sentinels) pass through unchanged: orientation
// never renumbers the bar or the bearoff tray (ADR-0006 decision 2, intact).
func TestOrientationPassthrough(t *testing.T) {
	for _, o := range AllOrientations() {
		for _, p := range []int{0, 25, -1} {
			if got := o.TransformPoint(p); got != p {
				t.Errorf("%v.TransformPoint(%d) = %d, want passthrough", o, p, got)
			}
		}
	}
}

func TestOrientationFlips(t *testing.T) {
	for _, o := range AllOrientations() {
		if o.FlipHorizontal().FlipHorizontal() != o {
			t.Errorf("%v: double horizontal flip not identity", o)
		}
	}
	if P1HomeRight.FlipHorizontal() != P1HomeLeft {
		t.Error("right flipped horizontally should be left")
	}
}

func TestOrientationParse(t *testing.T) {
	for _, o := range AllOrientations() {
		got, ok := ParseOrientation(o.String())
		if !ok || got != o {
			t.Errorf("ParseOrientation(%q) = %v,%v; want %v,true", o.String(), got, ok, o)
		}
	}
	if _, ok := ParseOrientation("nonsense"); ok {
		t.Error("ParseOrientation(nonsense) ok=true, want false")
	}
	// Every vocabulary this repo has ever written, migrated (ADR-0006, ADR-0009).
	// The 23 committed corpus manifests all say "p1-bottom".
	for legacy, want := range map[string]Orientation{
		"":                     P1HomeRight,
		"p1-right":             P1HomeRight,
		"p1-bottom":            P1HomeRight,
		"p1-home-bottom-right": P1HomeRight,
		"p1-home-top-right":    P1HomeRight,
		"p1-left":              P1HomeLeft,
		"p1-home-bottom-left":  P1HomeLeft,
		"p1-home-top-left":     P1HomeLeft,
	} {
		if got, ok := ParseOrientation(legacy); !ok || got != want {
			t.Errorf("ParseOrientation(%q) = %v,%v; want %v,true", legacy, got, ok, want)
		}
	}
}

// A "p1-home-top-*" document was written under the old four-value model, where
// the vertical mirror was the only way to say "the other player is the near
// one". Its Player 1 is the top player, so reading it under the new rule means
// exchanging the two players — the geometry alone is not enough.
func TestLegacyTopOrientation(t *testing.T) {
	for _, s := range []string{"p1-home-top-right", "p1-home-top-left"} {
		if !LegacyTopOrientation(s) {
			t.Errorf("LegacyTopOrientation(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "p1-bottom", "p1-right", "p1-left", "p1-home-right", "p1-home-left", "nonsense"} {
		if LegacyTopOrientation(s) {
			t.Errorf("LegacyTopOrientation(%q) = true, want false", s)
		}
	}
}
