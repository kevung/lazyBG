package diceevent

import (
	"image"
	"image/color"
	"testing"

	"lazybg/internal/capture"
)

const w, h = 160, 120

var felt = color.RGBA{168, 166, 162, 255}

func feltFrame() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, felt)
		}
	}
	return img
}

func withRect(base *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(base.Bounds())
	copy(img.Pix, base.Pix)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func feed(d *Detector, imgs []*image.RGBA) []Event {
	var out []Event
	for i, img := range imgs {
		out = append(out, d.Feed(capture.Frame{Tick: i * 333, Img: img})...)
	}
	return out
}

func newDetector() *Detector {
	return New(Options{Felt: felt})
}

// A die-sized bright blob that lands and stays must produce one Appeared
// event, timestamped at its first stable frame.
func TestFeed_DieAppears(t *testing.T) {
	base := feltFrame()
	die := withRect(base, 70, 50, 78, 58, color.RGBA{250, 250, 250, 255})
	var seq []*image.RGBA
	for i := 0; i < 10; i++ {
		seq = append(seq, base)
	}
	for i := 0; i < 12; i++ {
		seq = append(seq, die)
	}
	events := feed(newDetector(), seq)
	if len(events) != 1 || events[0].Kind != Appeared {
		t.Fatalf("events = %+v, want one Appeared", events)
	}
	// tick of the first die frame is 10*333; the fast background needs a few
	// samples to absorb the die, so the event may lag by ~2 samples.
	if events[0].Tick < 10*333 || events[0].Tick > 16*333 {
		t.Errorf("tick %d, want close to %d", events[0].Tick, 10*333)
	}
	if !events[0].Box.Overlaps(image.Rect(70, 50, 78, 58)) {
		t.Errorf("box %v does not cover the die", events[0].Box)
	}
}

// Picking the die back up produces a Removed event.
func TestFeed_DieRemoved(t *testing.T) {
	base := feltFrame()
	die := withRect(base, 70, 50, 78, 58, color.RGBA{250, 250, 250, 255})
	var seq []*image.RGBA
	for i := 0; i < 8; i++ {
		seq = append(seq, base)
	}
	for i := 0; i < 30; i++ { // long enough for the slow background to absorb it
		seq = append(seq, die)
	}
	for i := 0; i < 12; i++ {
		seq = append(seq, base)
	}
	events := feed(newDetector(), seq)
	if len(events) < 2 {
		t.Fatalf("events = %+v, want Appeared then Removed", events)
	}
	last := events[len(events)-1]
	if last.Kind != Removed {
		t.Errorf("last event %+v, want Removed", last)
	}
}

// A hand sweeping across (large fast blob) must not fire events.
func TestFeed_HandIgnored(t *testing.T) {
	base := feltFrame()
	var seq []*image.RGBA
	for i := 0; i < 10; i++ {
		seq = append(seq, base)
	}
	for i := 0; i < 6; i++ { // moving large dark blob
		seq = append(seq, withRect(base, 10+i*20, 30, 10+i*20+50, 100, color.RGBA{90, 60, 40, 255}))
	}
	for i := 0; i < 10; i++ {
		seq = append(seq, base)
	}
	if events := feed(newDetector(), seq); len(events) != 0 {
		t.Errorf("events = %+v, want none for a passing hand", events)
	}
}

// A die placed while a hand also crosses later is still reported once.
func TestFeed_NoDuplicateEvents(t *testing.T) {
	base := feltFrame()
	die := withRect(base, 40, 40, 48, 48, color.RGBA{15, 15, 15, 255})
	var seq []*image.RGBA
	for i := 0; i < 8; i++ {
		seq = append(seq, base)
	}
	for i := 0; i < 25; i++ {
		seq = append(seq, die)
	}
	appeared := 0
	for _, e := range feed(newDetector(), seq) {
		if e.Kind == Appeared {
			appeared++
		}
	}
	if appeared != 1 {
		t.Errorf("appeared %d times, want exactly 1", appeared)
	}
}
