package bg

import "testing"

func TestOrientationIdentity(t *testing.T) {
	for p := 1; p <= 24; p++ {
		if got := P1HomeBottomRight.TransformPoint(p); got != p {
			t.Errorf("P1HomeBottomRight.TransformPoint(%d) = %d, want %d (identity)", p, got, p)
		}
	}
}

func TestOrientationAnchors(t *testing.T) {
	// Hand-worked anchors from the calibrate canonical convention:
	// top row 13..24 at columns 0..11, bottom row 12..1 at columns 0..11,
	// P1 home (1..6) in the bottom-right half under the identity.
	cases := []struct {
		o       Orientation
		p, want int
	}{
		// Horizontal mirror: P1 home moves to the bottom-left half.
		{P1HomeBottomLeft, 1, 12},
		{P1HomeBottomLeft, 6, 7},
		{P1HomeBottomLeft, 24, 13},
		{P1HomeBottomLeft, 13, 24},
		// Vertical mirror: P1 home moves to the top-right half.
		{P1HomeTopRight, 1, 24},
		{P1HomeTopRight, 24, 1},
		{P1HomeTopRight, 6, 19},
		// 180°: P1 home moves to the top-left half.
		{P1HomeTopLeft, 1, 13},
		{P1HomeTopLeft, 13, 1},
	}
	for _, c := range cases {
		if got := c.o.TransformPoint(c.p); got != c.want {
			t.Errorf("%v.TransformPoint(%d) = %d, want %d", c.o, c.p, got, c.want)
		}
	}
}

// Every orientation is a dihedral involution: applying it twice is the identity.
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
// never renumbers the bar or the bearoff tray (ADR-0006).
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
	// Two horizontal flips (or two vertical) return to the start; H then V is
	// the 180° rotation.
	for _, o := range AllOrientations() {
		if o.FlipHorizontal().FlipHorizontal() != o {
			t.Errorf("%v: double horizontal flip not identity", o)
		}
		if o.FlipVertical().FlipVertical() != o {
			t.Errorf("%v: double vertical flip not identity", o)
		}
	}
	if P1HomeBottomRight.FlipHorizontal() != P1HomeBottomLeft {
		t.Error("bottom-right flipped horizontally should be bottom-left")
	}
	if P1HomeBottomRight.FlipVertical() != P1HomeTopRight {
		t.Error("bottom-right flipped vertically should be top-right")
	}
	if P1HomeBottomRight.FlipHorizontal().FlipVertical() != P1HomeTopLeft {
		t.Error("bottom-right flipped both ways should be top-left")
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
	// Legacy strings migrate onto the enum (ADR-0006).
	for legacy, want := range map[string]Orientation{
		"p1-right":  P1HomeBottomRight,
		"p1-left":   P1HomeBottomLeft,
		"p1-bottom": P1HomeBottomRight,
	} {
		if got, ok := ParseOrientation(legacy); !ok || got != want {
			t.Errorf("ParseOrientation(%q) = %v,%v; want %v,true", legacy, got, ok, want)
		}
	}
}
