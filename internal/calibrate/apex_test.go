package calibrate

import "testing"

func TestPointApex_TopAndBottomRows(t *testing.T) {
	cb := DefaultCanonical()
	_, h := cb.Size()

	// Point 13: top row leftmost — tip points down, at the inner end of QuadH.
	a := cb.PointApex(13)
	if a.X != float64(cb.MarginX)+float64(cb.PointW)/2 || a.Y != float64(cb.MarginY+cb.QuadH) {
		t.Errorf("point 13 apex = %v", a)
	}
	// Point 1: bottom row rightmost — tip points up.
	b := cb.PointApex(1)
	wantX := float64(cb.columnX(11)) + float64(cb.PointW)/2
	wantY := float64(h - cb.MarginY - cb.QuadH)
	if b.X != wantX || b.Y != wantY {
		t.Errorf("point 1 apex = %v, want (%v,%v)", b, wantX, wantY)
	}
	// Point 24: top row rightmost shares point 1's column.
	if c := cb.PointApex(24); c.X != wantX {
		t.Errorf("point 24 apex x = %v, want %v", c.X, wantX)
	}
}
