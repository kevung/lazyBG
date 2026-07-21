package autocal

import (
	"math"
	"testing"

	"lazybg/internal/geom"
)

// drawTriangle rasterizes a filled triangle with a horizontal base at baseY
// and its apex at (apexX, apexY), into mask (w×h). Width shrinks linearly
// from baseW at the base to 0 at the apex.
func drawTriangle(mask []bool, w, h int, apexX, apexY, baseY, baseW float64) {
	dir := 1.0
	if apexY < baseY {
		dir = -1.0
	}
	total := math.Abs(apexY - baseY)
	for y := 0; y <= int(total); y++ {
		yy := int(baseY) + int(dir)*y
		if yy < 0 || yy >= h {
			continue
		}
		frac := 1 - float64(y)/total
		half := baseW * frac / 2
		for x := int(apexX - half); x <= int(apexX+half); x++ {
			if x >= 0 && x < w {
				mask[yy*w+x] = true
			}
		}
	}
}

func TestApexComponents_RecoversSyntheticApexes(t *testing.T) {
	w, h := 640, 360
	mask := make([]bool, w*h)
	// Two rows like a real board: top row points down, bottom row points up.
	wantDown := []geom.Pt{geom.P(60, 150), geom.P(120, 150), geom.P(180, 150)}
	wantUp := []geom.Pt{geom.P(60, 210), geom.P(120, 210), geom.P(180, 210)}
	for _, a := range wantDown {
		drawTriangle(mask, w, h, a.X, a.Y, 10, 40)
	}
	for _, a := range wantUp {
		drawTriangle(mask, w, h, a.X, a.Y, 350, 40)
	}

	apexes := ApexComponents(mask, w, h)
	if len(apexes) != 6 {
		t.Fatalf("got %d apexes, want 6", len(apexes))
	}
	for _, want := range append(append([]geom.Pt{}, wantDown...), wantUp...) {
		if d := nearestDist(apexes, want); d > 2.0 {
			t.Errorf("apex near %v missed by %.2fpx", want, d)
		}
	}
	// Orientation must be recovered too: top-row triangles (apex in the
	// upper half here) point down, bottom-row ones point up.
	for _, a := range apexes {
		wantDownDir := a.Pt.Y < 180
		if a.Down != wantDownDir {
			t.Errorf("apex %v: Down=%v, want %v", a.Pt, a.Down, wantDownDir)
		}
	}
}

func TestApexComponents_RobustToBaseTruncation(t *testing.T) {
	w, h := 640, 360
	mask := make([]bool, w*h)
	drawTriangle(mask, w, h, 100, 160, 10, 44)
	// A stack of checkers hides the base: erase the first 60 rows.
	for y := 0; y < 70; y++ {
		for x := 0; x < w; x++ {
			mask[y*w+x] = false
		}
	}
	apexes := ApexComponents(mask, w, h)
	if len(apexes) != 1 {
		t.Fatalf("got %d apexes, want 1", len(apexes))
	}
	if d := math.Hypot(apexes[0].Pt.X-100, apexes[0].Pt.Y-160); d > 2.5 {
		t.Errorf("truncated triangle apex off by %.2fpx", d)
	}
}

func TestApexComponents_SlantedTriangle(t *testing.T) {
	// Perspective shears triangles: lateral edges stay straight but the apex
	// is not above the base centre. Shear the whole triangle by shifting each
	// row proportionally.
	w, h := 640, 360
	mask := make([]bool, w*h)
	base := make([]bool, w*h)
	drawTriangle(base, w, h, 100, 160, 10, 44)
	shear := 0.25 // px of x-shift per px of y
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if base[y*w+x] {
				nx := x + int(float64(y)*shear)
				if nx < w {
					mask[y*w+nx] = true
				}
			}
		}
	}
	apexes := ApexComponents(mask, w, h)
	if len(apexes) != 1 {
		t.Fatalf("got %d apexes, want 1", len(apexes))
	}
	wantX := 100 + 160*shear
	if d := math.Hypot(apexes[0].Pt.X-wantX, apexes[0].Pt.Y-160); d > 2.5 {
		t.Errorf("slanted apex off by %.2fpx (got %v)", d, apexes[0].Pt)
	}
}

func TestApexComponents_RejectsNonConvergingBlob(t *testing.T) {
	w, h := 640, 360
	mask := make([]bool, w*h)
	for y := 40; y < 200; y++ { // a rectangle: edges never converge
		for x := 300; x < 360; x++ {
			mask[y*w+x] = true
		}
	}
	if apexes := ApexComponents(mask, w, h); len(apexes) != 0 {
		t.Fatalf("rectangle must yield no apex, got %v", apexes)
	}
}

func TestMatchApexes_MutualNearestWithinRadius(t *testing.T) {
	predicted := []geom.Pt{geom.P(10, 10), geom.P(50, 10), geom.P(90, 10), geom.P(130, 10)}
	detected := []geom.Pt{
		geom.P(11, 12),   // slot 0, slightly off
		geom.P(52, 9),    // slot 1
		geom.P(300, 300), // spurious, far from everything
		// slot 2 missing (occluded)
		geom.P(131, 11), // slot 3
	}
	m := MatchApexes(detected, predicted, 8)
	want := map[int]int{0: 0, 1: 1, 3: 3}
	if len(m) != len(want) {
		t.Fatalf("got %v, want %v", m, want)
	}
	for slot, det := range want {
		if m[slot] != det {
			t.Errorf("slot %d matched to %d, want %d (%v)", slot, m[slot], det, m)
		}
	}
}

func TestMatchApexes_AmbiguousDetectionUnmatched(t *testing.T) {
	// Two detections compete for one slot: mutual-nearest keeps the closer,
	// never both, and the loser must not steal a farther slot beyond maxDist.
	predicted := []geom.Pt{geom.P(10, 10)}
	detected := []geom.Pt{geom.P(11, 10), geom.P(13, 10)}
	m := MatchApexes(detected, predicted, 8)
	if len(m) != 1 || m[0] != 0 {
		t.Fatalf("got %v, want {0:0}", m)
	}
}

func nearestDist(apexes []Apex, p geom.Pt) float64 {
	best := math.Inf(1)
	for _, a := range apexes {
		if d := math.Hypot(a.Pt.X-p.X, a.Pt.Y-p.Y); d < best {
			best = d
		}
	}
	return best
}
