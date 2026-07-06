package stableframe

import (
	"image"
	"image/color"
	"testing"

	"lazybg/internal/capture"
)

var (
	grayC  = color.RGBA{128, 128, 128, 255}
	blackC = color.RGBA{0, 0, 0, 255}
)

const (
	frameW = 200
	frameH = 160
)

func filled(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, frameW, frameH))
	for y := 0; y < frameH; y++ {
		for x := 0; x < frameW; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// withSquare returns the gray base with a 40×40 black square whose left edge is
// at x (simulating a hand crossing the board).
func withSquare(x int) *image.RGBA {
	img := filled(grayC)
	for yy := 60; yy < 100; yy++ {
		for xx := x; xx < x+40 && xx < frameW; xx++ {
			img.SetRGBA(xx, yy, blackC)
		}
	}
	return img
}

// sequence: 5 still frames, 3 frames with a hand crossing, then 4 still frames.
// Ticks are 100ms apart (0..1100).
func sequence() []capture.Frame {
	base := filled(grayC)
	var frames []capture.Frame
	add := func(tick int, img image.Image) { frames = append(frames, capture.Frame{Tick: tick, Img: img}) }
	for i := 0; i < 5; i++ {
		add(i*100, base)
	}
	for i := 5; i < 8; i++ {
		add(i*100, withSquare((i-5)*40))
	}
	for i := 8; i < 12; i++ {
		add(i*100, base)
	}
	return frames
}

func TestMotion(t *testing.T) {
	base := filled(grayC)
	if m := Motion(base, base, image.Rectangle{}); m != 0 {
		t.Errorf("identical frames: motion = %v, want 0", m)
	}
	// A 40×40 black square over a 200×160 gray frame: mean |Δluma| ≈
	// 128 * 1600 / 32000 = 6.4.
	m := Motion(base, withSquare(0), image.Rectangle{})
	if m < 6.0 || m > 6.8 {
		t.Errorf("square motion = %.2f, want ≈6.4", m)
	}
}

func TestWindows_StillMotionStill(t *testing.T) {
	d := Detector{MaxMotion: 2, MinFrames: 3}
	ws := d.Windows(capture.NewSliceSource(sequence()))

	if len(ws) != 2 {
		t.Fatalf("got %d windows, want 2: %+v", len(ws), ws)
	}
	w0, w1 := ws[0], ws[1]
	if w0.StartTick != 0 || w0.EndTick != 400 || w0.Frames != 5 || w0.Rep.Tick != 400 {
		t.Errorf("window 0 = %+v, want ticks 0..400, 5 frames, rep@400", w0)
	}
	if w1.StartTick != 800 || w1.EndTick != 1100 || w1.Frames != 4 || w1.Rep.Tick != 1100 {
		t.Errorf("window 1 = %+v, want ticks 800..1100, 4 frames, rep@1100", w1)
	}
}

func TestWindows_MinFramesFilters(t *testing.T) {
	// Requiring 6 still frames drops both runs (5 and 4 frames).
	d := Detector{MaxMotion: 2, MinFrames: 6}
	if ws := d.Windows(capture.NewSliceSource(sequence())); len(ws) != 0 {
		t.Errorf("MinFrames=6 should yield no windows, got %d", len(ws))
	}
}

// Motion outside the ROI is ignored: watching a quiet corner, the whole sequence
// is one stable window.
func TestWindows_ROIIgnoresOutsideMotion(t *testing.T) {
	roi := image.Rect(0, 0, 30, 30) // the hand crosses y=60..100, never here
	d := Detector{ROI: roi, MaxMotion: 2, MinFrames: 3}
	ws := d.Windows(capture.NewSliceSource(sequence()))
	if len(ws) != 1 || ws[0].Frames != 12 || ws[0].Rep.Tick != 1100 {
		t.Errorf("ROI-quiet sequence should be one 12-frame window, got %+v", ws)
	}
}

func TestWindows_Empty(t *testing.T) {
	d := Detector{MaxMotion: 2, MinFrames: 1}
	if ws := d.Windows(capture.NewSliceSource(nil)); ws != nil {
		t.Errorf("empty source should yield nil, got %+v", ws)
	}
}
