package palette

import (
	"image"
	"image/color"
	"testing"

	"lazybg/internal/calibrate"
)

var (
	felt   = color.RGBA{200, 198, 190, 255} // light grey playing surface
	triA   = color.RGBA{20, 110, 106, 255}  // teal
	triB   = color.RGBA{212, 175, 55, 255}  // yellow
	chkA   = color.RGBA{231, 224, 213, 255} // ivory
	chkB   = color.RGBA{49, 34, 28, 255}    // dark brown
	nearly = func(a, b color.RGBA, tol int) bool {
		d := func(x, y uint8) int {
			if x > y {
				return int(x) - int(y)
			}
			return int(y) - int(x)
		}
		return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
	}
)

// renderBoard draws a rectified board the way the real thing looks: felt
// everywhere, alternating triangles narrowing towards the middle, and a few
// checker stacks sitting at the outer end of their point.
func renderBoard(cb calibrate.CanonicalBoard, stacks map[int]color.RGBA) *image.RGBA {
	w, h := cb.Size()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, felt)
		}
	}
	for p := 1; p <= 24; p++ {
		region, dir := cb.PointRegion(p)
		c := triA
		if p%2 == 0 {
			c = triB
		}
		n := region.Dy()
		for i := 0; i < n; i++ {
			// Width shrinks to a point at the inner end.
			half := (region.Dx() / 2) * (n - i) / n
			y := region.Min.Y + i
			if dir == calibrate.StackUp {
				y = region.Max.Y - 1 - i
			}
			mid := (region.Min.X + region.Max.X) / 2
			for x := mid - half; x < mid+half; x++ {
				if x >= 0 && x < w && y >= 0 && y < h {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
	for p, c := range stacks {
		region, dir := cb.PointRegion(p)
		d := cb.PointW
		for k := 0; k < 3; k++ { // three checkers deep
			var r image.Rectangle
			if dir == calibrate.StackDown {
				r = image.Rect(region.Min.X, region.Min.Y+k*d, region.Max.X, region.Min.Y+(k+1)*d)
			} else {
				r = image.Rect(region.Min.X, region.Max.Y-(k+1)*d, region.Max.X, region.Max.Y-k*d)
			}
			r = r.Intersect(img.Bounds())
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
	return img
}

func testBoard() (calibrate.CanonicalBoard, *image.RGBA) {
	cb := calibrate.DefaultCanonical()
	stacks := map[int]color.RGBA{
		24: chkA, 13: chkA, 8: chkA, 6: chkA,
		1: chkB, 12: chkB, 17: chkB, 19: chkB,
	}
	return cb, renderBoard(cb, stacks)
}

func TestSample_RecoversTheBoardPalette(t *testing.T) {
	cb, img := testBoard()
	got, ok := Sample(img, cb, chkA, chkB)
	if !ok {
		t.Fatal("Sample reported failure on a clean synthetic board")
	}
	if !nearly(got.Felt, felt, 12) {
		t.Errorf("felt = %v, want ~%v", got.Felt, felt)
	}
	// The two triangle colours may come back in either order — the board has
	// no canonical "first" triangle colour.
	if !(nearly(got.PointA, triA, 12) && nearly(got.PointB, triB, 12)) &&
		!(nearly(got.PointA, triB, 12) && nearly(got.PointB, triA, 12)) {
		t.Errorf("point colours = %v/%v, want ~%v/%v", got.PointA, got.PointB, triA, triB)
	}
}

// The checker colours must come back on the SAME side as they were declared:
// sampling refines the declared pair, it never re-decides which player is
// which (that is the "swap the two players" gesture, ADR-0009).
func TestSample_KeepsTheDeclaredCheckerAssignment(t *testing.T) {
	cb, img := testBoard()

	got, ok := Sample(img, cb, chkA, chkB)
	if !ok {
		t.Fatal("Sample reported failure")
	}
	if !nearly(got.CheckerA, chkA, 12) || !nearly(got.CheckerB, chkB, 12) {
		t.Fatalf("checkers = %v/%v, want ~%v/%v", got.CheckerA, got.CheckerB, chkA, chkB)
	}

	// Declared the other way round: the same two measured colours, swapped.
	rev, ok := Sample(img, cb, chkB, chkA)
	if !ok {
		t.Fatal("Sample reported failure on the reversed declaration")
	}
	if !nearly(rev.CheckerA, chkB, 12) || !nearly(rev.CheckerB, chkA, 12) {
		t.Errorf("reversed checkers = %v/%v, want ~%v/%v", rev.CheckerA, rev.CheckerB, chkB, chkA)
	}
}

// Rough declared values (the colour-picker guesses this replaces) must still
// land on the right side: that is the whole point of sampling.
func TestSample_SnapsRoughDeclarations(t *testing.T) {
	cb, img := testBoard()
	got, ok := Sample(img, cb, color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255})
	if !ok {
		t.Fatal("Sample reported failure")
	}
	if !nearly(got.CheckerA, chkA, 12) || !nearly(got.CheckerB, chkB, 12) {
		t.Errorf("checkers = %v/%v, want ~%v/%v from white/black guesses",
			got.CheckerA, got.CheckerB, chkA, chkB)
	}
}

// An empty board has no checkers to measure. The board palette is still
// usable, so the caller is told what was found rather than getting silence.
func TestSample_EmptyBoardYieldsNoCheckers(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	img := renderBoard(cb, nil)
	got, ok := Sample(img, cb, chkA, chkB)
	if !ok {
		t.Fatal("Sample reported failure on an empty board")
	}
	if got.HasCheckers {
		t.Errorf("HasCheckers = true on an empty board (got %v/%v)", got.CheckerA, got.CheckerB)
	}
	if !nearly(got.Felt, felt, 12) {
		t.Errorf("felt = %v, want ~%v even with no checkers", got.Felt, felt)
	}
}
