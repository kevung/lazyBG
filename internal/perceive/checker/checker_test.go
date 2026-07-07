package checker

import (
	"image"
	"math"
	"testing"
)

// disc renders a filled circle of value fg on a background of value bg into g.
func disc(g *image.Gray, cx, cy, r int, fg uint8) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if !(image.Pt(x, y).In(g.Bounds())) {
				continue
			}
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				g.Pix[g.PixOffset(x, y)] = fg
			}
		}
	}
}

func filled(w, h int, bg uint8) *image.Gray {
	g := image.NewGray(image.Rect(0, 0, w, h))
	for i := range g.Pix {
		g.Pix[i] = bg
	}
	return g
}

func near(got []Circle, cx, cy, tol int) bool {
	for _, c := range got {
		if math.Abs(float64(c.X-cx)) <= float64(tol) && math.Abs(float64(c.Y-cy)) <= float64(tol) {
			return true
		}
	}
	return false
}

// Two discs spread along an axis (as checkers sit on a point) must both be found.
func TestDetect_TwoDiscsAlongAxis(t *testing.T) {
	g := filled(200, 200, 100)
	r := 20
	disc(g, 60, 60, r, 180)
	disc(g, 60, 100, r, 180) // 2r apart, just touching
	got := Detect(g, r)
	if len(got) != 2 {
		t.Fatalf("found %d circles, want 2: %+v", len(got), got)
	}
	if !near(got, 60, 60, 4) || !near(got, 60, 100, 4) {
		t.Errorf("centers off: %+v", got)
	}
}

// The whole point of shape-over-colour: a disc only ~15 grey levels above the
// background (low contrast, like a white checker on a white tray) is still
// detected by its rim, where a colour-distance classifier would fail.
func TestDetect_LowContrast(t *testing.T) {
	g := filled(160, 160, 150)
	r := 22
	disc(g, 80, 80, r, 165) // contrast 15
	got := Detect(g, r)
	if !near(got, 80, 80, 5) {
		t.Fatalf("low-contrast disc missed: %+v", got)
	}
}

// Dark-on-light must work as well as light-on-dark (polarity-agnostic).
func TestDetect_DarkOnLight(t *testing.T) {
	g := filled(160, 160, 200)
	r := 20
	disc(g, 80, 80, r, 60)
	got := Detect(g, r)
	if !near(got, 80, 80, 5) {
		t.Fatalf("dark-on-light disc missed: %+v", got)
	}
}

// Empty field yields no circles.
func TestDetect_Empty(t *testing.T) {
	g := filled(120, 120, 128)
	if got := Detect(g, 20); len(got) != 0 {
		t.Errorf("empty field found %d circles: %+v", len(got), got)
	}
}
