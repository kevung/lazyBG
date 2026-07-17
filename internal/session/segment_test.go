package session

import (
	"image"
	"image/color"
	"reflect"
	"testing"

	"lazybg/internal/capture"
)

func fillFrame(tick int, c color.RGBA) capture.Frame {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 0xff
	}
	return capture.Frame{Tick: tick, Img: img}
}

// segmentTicks returns the mid-tick of each stable window and drops the
// one-frame transition runs (below MinFrames) — the segmentation-proposed
// candidate commit instants for Tab/Shift+Tab (issue #23, criterion 3).
func TestSegmentTicks_MidTicksOfStableWindows(t *testing.T) {
	gray := color.RGBA{128, 128, 128, 255}
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}
	frames := []capture.Frame{
		fillFrame(0, gray), fillFrame(100, gray), fillFrame(200, gray), fillFrame(300, gray),
		fillFrame(400, white), // motion spike → 1-frame run, filtered out
		fillFrame(500, black), fillFrame(600, black), fillFrame(700, black),
	}
	got := segmentTicks(capture.NewSliceSource(frames), image.Rectangle{}, 1.5, 3)
	want := []int{150, 600}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("segmentTicks = %v, want %v", got, want)
	}
}

// CandidateTicks is nil until a scan stores ticks, then returns a copy.
func TestCandidateTicks_Accessor(t *testing.T) {
	s := New()
	if got := s.CandidateTicks(); got != nil {
		t.Fatalf("fresh session candidate ticks = %v, want nil", got)
	}
	s.candidateTicks = []int{150, 600}
	got := s.CandidateTicks()
	if !reflect.DeepEqual(got, []int{150, 600}) {
		t.Fatalf("CandidateTicks = %v, want [150 600]", got)
	}
	got[0] = 0 // mutating the copy must not corrupt the session's slice
	if s.candidateTicks[0] != 150 {
		t.Fatal("CandidateTicks returned an aliased slice")
	}
}
