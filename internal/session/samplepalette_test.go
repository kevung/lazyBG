package session

import (
	"image"
	"image/color"
	"strconv"
	"testing"

	"lazybg/internal/calibrate"
	"lazybg/internal/corpus"
)

// paintedBoard renders a board straight into canonical geometry: felt, two
// alternating triangle colours, and one three-checker stack of each colour.
// With the calibration corners set to the image corners the rectification is
// the identity, so the sampler sees exactly these pixels.
func paintedBoard(cb calibrate.CanonicalBoard) *image.RGBA {
	w, h := cb.Size()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	felt := color.RGBA{200, 198, 190, 255}
	triA := color.RGBA{20, 110, 106, 255}
	triB := color.RGBA{212, 175, 55, 255}
	chkA := color.RGBA{231, 224, 213, 255}
	chkB := color.RGBA{49, 34, 28, 255}
	fill := func(r image.Rectangle, c color.RGBA) {
		r = r.Intersect(img.Bounds())
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
	fill(img.Bounds(), felt)
	for p := 1; p <= 24; p++ {
		region, dir := cb.PointRegion(p)
		c := triA
		if p%2 == 0 {
			c = triB
		}
		n := region.Dy()
		for i := 0; i < n; i++ {
			half := (region.Dx() / 2) * (n - i) / n
			y := region.Min.Y + i
			if dir == calibrate.StackUp {
				y = region.Max.Y - 1 - i
			}
			mid := (region.Min.X + region.Max.X) / 2
			fill(image.Rect(mid-half, y, mid+half, y+1), c)
		}
	}
	for _, st := range []struct {
		p int
		c color.RGBA
	}{{24, chkA}, {13, chkA}, {6, chkA}, {1, chkB}, {12, chkB}, {19, chkB}} {
		region, dir := cb.PointRegion(st.p)
		d := cb.PointW
		for k := 0; k < 3; k++ {
			if dir == calibrate.StackDown {
				fill(image.Rect(region.Min.X, region.Min.Y+k*d, region.Max.X, region.Min.Y+(k+1)*d), st.c)
			} else {
				fill(image.Rect(region.Min.X, region.Max.Y-(k+1)*d, region.Max.X, region.Max.Y-k*d), st.c)
			}
		}
	}
	return img
}

func TestSamplePalette_MeasuresThroughTheSubmittedHandles(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	img := paintedBoard(cb)
	w, h := cb.Size()

	s := New()
	s.doc = &LBG{SchemaVersion: LBGSchemaVersion, Parts: []LBGPart{{File: "video.mp4"}}}
	s.grab = func(int) (image.Image, error) { return img, nil }

	// The handles as they stand in the FORM — the session has none saved.
	cal := corpus.Calibration{Corners: [][2]float64{
		{0, 0}, {float64(w - 1), 0}, {float64(w - 1), float64(h - 1)}, {0, float64(h - 1)},
	}}

	got, err := s.SamplePalette(0, cal, "#ffffff", "#000000")
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasCheckers {
		t.Fatal("HasCheckers = false on a board with six stacks on it")
	}
	// Rough white/black declarations must snap onto the real ivory/brown.
	if !nearHex(t, got.CheckerA, 231, 224, 213) {
		t.Errorf("checkerA = %s, want ~#e7e0d5", got.CheckerA)
	}
	if !nearHex(t, got.CheckerB, 49, 34, 28) {
		t.Errorf("checkerB = %s, want ~#31221c", got.CheckerB)
	}
	if !nearHex(t, got.Felt, 200, 198, 190) {
		t.Errorf("felt = %s, want ~#c8c6be", got.Felt)
	}
	if got.PointA == "" || got.PointB == "" || got.PointA == got.PointB {
		t.Errorf("point colours = %s/%s, want two distinct measured colours", got.PointA, got.PointB)
	}
}

// Without four corners there is no board to rectify: say so instead of
// returning a palette measured from whatever the frame happened to contain.
func TestSamplePalette_RefusesWithoutCalibration(t *testing.T) {
	s := New()
	s.doc = &LBG{SchemaVersion: LBGSchemaVersion, Parts: []LBGPart{{File: "video.mp4"}}}
	s.grab = func(int) (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 64, 64)), nil }
	if _, err := s.SamplePalette(0, corpus.Calibration{}, "#fff", "#000"); err == nil {
		t.Error("expected an error with no corners placed")
	}
}

// nearHex reports whether a measured "#rrggbb" is within a small distance of
// the colour that was painted — sampling averages, it does not copy pixels.
func nearHex(t *testing.T, hex string, r, g, b int) bool {
	t.Helper()
	if len(hex) != 7 || hex[0] != '#' {
		return false
	}
	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return false
	}
	d := func(a, b int) int {
		if a > b {
			return a - b
		}
		return b - a
	}
	return d(int(v>>16), r) <= 14 && d(int(v>>8&0xff), g) <= 14 && d(int(v&0xff), b) <= 14
}
