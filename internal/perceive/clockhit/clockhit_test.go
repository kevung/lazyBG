package clockhit

import (
	"image"
	"image/color"
	"testing"

	"lazybg/internal/capture"
)

const w, h = 120, 80

var roi = image.Rect(80, 20, 115, 60)

func base() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{120, 60, 60, 255})
		}
	}
	return img
}

// withHand paints a hand over the clock ROI.
func withHand(img *image.RGBA) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	for y := roi.Min.Y; y < roi.Max.Y; y++ {
		for x := roi.Min.X; x < roi.Max.X; x++ {
			out.SetRGBA(x, y, color.RGBA{210, 170, 140, 255})
		}
	}
	return out
}

func feed(d *Detector, imgs []*image.RGBA) []Hit {
	var out []Hit
	for i, img := range imgs {
		out = append(out, d.Feed(capture.Frame{Tick: i * 333, Img: img})...)
	}
	return out
}

// quiet → 2-frame press → quiet = one hit at the press start.
func TestFeed_PressDetected(t *testing.T) {
	b, hand := base(), withHand(base())
	var seq []*image.RGBA
	for i := 0; i < 6; i++ {
		seq = append(seq, b)
	}
	seq = append(seq, hand, hand)
	for i := 0; i < 6; i++ {
		seq = append(seq, b)
	}
	hits := feed(New(Options{ROI: roi}), seq)
	if len(hits) != 1 {
		t.Fatalf("hits = %+v, want exactly 1", hits)
	}
	if hits[0].Tick != 6*333 {
		t.Errorf("tick %d, want %d (press start)", hits[0].Tick, 6*333)
	}
}

// A parked arm produces enter/leave motion frames but the long still stretch
// between them must not multiply hits.
func TestFeed_ParkedArmAtMostEnterLeave(t *testing.T) {
	b, hand := base(), withHand(base())
	var seq []*image.RGBA
	for i := 0; i < 5; i++ {
		seq = append(seq, b)
	}
	for i := 0; i < 12; i++ {
		seq = append(seq, hand)
	}
	for i := 0; i < 5; i++ {
		seq = append(seq, b)
	}
	hits := feed(New(Options{ROI: roi}), seq)
	if len(hits) > 2 {
		t.Errorf("hits = %+v, want at most enter+leave (2)", hits)
	}
}

// Two separate presses = two hits.
func TestFeed_TwoPresses(t *testing.T) {
	b, hand := base(), withHand(base())
	var seq []*image.RGBA
	add := func(n int, img *image.RGBA) {
		for i := 0; i < n; i++ {
			seq = append(seq, img)
		}
	}
	add(5, b)
	add(2, hand)
	add(6, b)
	add(2, hand)
	add(5, b)
	hits := feed(New(Options{ROI: roi}), seq)
	if len(hits) != 2 {
		t.Errorf("hits = %+v, want 2", hits)
	}
}

// Motion outside the ROI is invisible.
func TestFeed_OutsideROIIgnored(t *testing.T) {
	b := base()
	moved := image.NewRGBA(b.Bounds())
	copy(moved.Pix, b.Pix)
	for y := 10; y < 70; y++ {
		for x := 5; x < 60; x++ { // left of the ROI
			moved.SetRGBA(x, y, color.RGBA{20, 20, 20, 255})
		}
	}
	var seq []*image.RGBA
	for i := 0; i < 4; i++ {
		seq = append(seq, b)
	}
	seq = append(seq, moved, moved)
	for i := 0; i < 4; i++ {
		seq = append(seq, b)
	}
	if hits := feed(New(Options{ROI: roi}), seq); len(hits) != 0 {
		t.Errorf("hits = %+v, want none for off-ROI motion", hits)
	}
}
