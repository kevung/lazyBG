package capture

import (
	"image"
	"testing"
)

func TestSliceSource_DrainInOrder(t *testing.T) {
	frames := []Frame{
		{Tick: 0, Img: image.NewRGBA(image.Rect(0, 0, 1, 1))},
		{Tick: 100, Img: image.NewRGBA(image.Rect(0, 0, 1, 1))},
		{Tick: 200, Img: image.NewRGBA(image.Rect(0, 0, 1, 1))},
	}
	got := Drain(NewSliceSource(frames))
	if len(got) != 3 {
		t.Fatalf("drained %d frames, want 3", len(got))
	}
	for i, want := range []int{0, 100, 200} {
		if got[i].Tick != want {
			t.Errorf("frame %d tick = %d, want %d", i, got[i].Tick, want)
		}
	}
}

func TestSliceSource_ExhaustsOnce(t *testing.T) {
	s := NewSliceSource([]Frame{{Tick: 0}})
	if _, ok := s.Next(); !ok {
		t.Fatal("first Next should succeed")
	}
	if _, ok := s.Next(); ok {
		t.Error("second Next should report exhaustion")
	}
}
