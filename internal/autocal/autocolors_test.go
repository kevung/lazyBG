package autocal

import (
	"image"
	"image/color"
	"testing"
)

// marseilleLike reproduces the corpus failure mode of the 2026-05 captures:
// the table (light gray) dominates the frame centre, so the felt pick lands
// on the table and the "point colors" become whatever saturated junk sits
// next to the table — while the real board (darker felt, teal/yellow
// points) is smaller and loses the vote.
func marseilleLike() *image.RGBA {
	w, h := 640, 360
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
	table := color.RGBA{204, 204, 204, 255}
	felt := color.RGBA{168, 166, 162, 255}
	teal := color.RGBA{30, 100, 100, 255}
	yellow := color.RGBA{170, 135, 10, 255}
	blue := color.RGBA{32, 62, 132, 255} // a mat/banner on the table

	fill(0, 0, w, h, table)
	// A big saturated distractor adjacent to the TABLE, inside the centre crop.
	fill(140, 80, 500, 100, blue)
	// The (small) board: felt with two rows of alternating point stripes.
	fill(220, 130, 460, 250, felt)
	for i := 0; i < 10; i++ {
		c := teal
		if i%2 == 1 {
			c = yellow
		}
		fill(228+i*23, 136, 228+i*23+12, 180, c) // top row, felt all around
		fill(228+i*23, 200, 228+i*23+12, 244, c) // bottom row
	}
	return img
}

func near(a, b color.RGBA, tol int) bool {
	d := func(x, y uint8) int {
		v := int(x) - int(y)
		if v < 0 {
			v = -v
		}
		return v
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

func TestAutoColorCandidates_IncludesTruthDespiteTableDominance(t *testing.T) {
	med := marseilleLike()
	cands := AutoColorCandidates(med, 6)
	if len(cands) < 2 {
		t.Fatalf("want several candidates, got %d", len(cands))
	}
	teal := color.RGBA{30, 100, 100, 255}
	yellow := color.RGBA{170, 135, 10, 255}
	felt := color.RGBA{168, 166, 162, 255}
	found := false
	for _, c := range cands {
		pointsOK := (near(c.PointA, teal, 30) && near(c.PointB, yellow, 30)) ||
			(near(c.PointA, yellow, 30) && near(c.PointB, teal, 30))
		if pointsOK && near(c.Felt, felt, 30) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no candidate matches the real board colors; got %+v", cands)
	}
}

func TestAutoColorCandidates_FirstIsLegacyAutoColors(t *testing.T) {
	med := marseilleLike()
	legacy, ok := AutoColors(med)
	if !ok {
		t.Fatal("legacy AutoColors failed")
	}
	cands := AutoColorCandidates(med, 6)
	if len(cands) == 0 || cands[0] != legacy {
		t.Fatalf("candidate #0 must be the legacy answer for compatibility:\n got %+v\nwant %+v", cands[0], legacy)
	}
}

func TestAutoColorCandidates_Deduplicates(t *testing.T) {
	med := marseilleLike()
	cands := AutoColorCandidates(med, 6)
	seen := map[Colors]bool{}
	for _, c := range cands {
		if seen[c] {
			t.Fatalf("duplicate candidate %+v", c)
		}
		seen[c] = true
	}
}

// blueFeltBoard reproduces the 2026-05 Marseille blind spot: a SATURATED
// blue felt with light-blue and cream triangles, on a white plastic frame
// over a red table. No unsaturated surface is the felt, so the legacy
// felt model cannot express the right hypothesis at all.
func blueFeltBoard() *image.RGBA {
	w, h := 640, 360
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
	red := color.RGBA{140, 40, 50, 255}     // table cloth
	frame := color.RGBA{235, 235, 232, 255} // white plastic frame (unsaturated)
	felt := color.RGBA{35, 70, 160, 255}    // saturated blue felt
	lightBlue := color.RGBA{90, 150, 220, 255}
	cream := color.RGBA{225, 222, 195, 255}

	fill(0, 0, w, h, red)
	fill(140, 60, 500, 300, frame)
	fill(170, 80, 470, 280, felt)
	for i := 0; i < 10; i++ {
		c := lightBlue
		if i%2 == 1 {
			c = cream
		}
		fill(180+i*29, 86, 180+i*29+14, 150, c) // top row
		fill(180+i*29, 210, 180+i*29+14, 274, c) // bottom row
	}
	return img
}

func TestAutoColorCandidates_SaturatedFeltBoards(t *testing.T) {
	med := blueFeltBoard()
	cands := AutoColorCandidates(med, 8)
	felt := color.RGBA{35, 70, 160, 255}
	lightBlue := color.RGBA{90, 150, 220, 255}
	cream := color.RGBA{225, 222, 195, 255}
	found := false
	for _, c := range cands {
		pointsOK := (near(c.PointA, lightBlue, 35) && near(c.PointB, cream, 35)) ||
			(near(c.PointA, cream, 35) && near(c.PointB, lightBlue, 35))
		if pointsOK && near(c.Felt, felt, 35) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no candidate matches the blue-felt board; got %+v", cands)
	}
}

func TestAutoColorCandidates_SaturatedFeltsRankAfterUnsaturated(t *testing.T) {
	// Compat: on a classic unsaturated-felt board the first candidates (and
	// #0 = legacy) must stay what they were before saturated felts existed.
	med := marseilleLike()
	legacy, _ := AutoColors(med)
	if cands := AutoColorCandidates(med, 8); cands[0] != legacy {
		t.Fatalf("candidate #0 changed: %+v vs %+v", cands[0], legacy)
	}
}
