package capture

import "testing"

func TestStream_Real(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH")
	}
	src, err := Stream(makeTestVideo(t), StreamOpts{FPS: 5, W: 64, H: 48})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	frames := Drain(src)
	// 2s at 5 fps ≈ 10 frames; allow encoder edge slack.
	if len(frames) < 9 || len(frames) > 11 {
		t.Fatalf("got %d frames, want ~10", len(frames))
	}
	if b := frames[0].Img.Bounds(); b.Dx() != 64 || b.Dy() != 48 {
		t.Errorf("frame size %dx%d, want 64x48", b.Dx(), b.Dy())
	}
	if frames[0].Tick != 0 {
		t.Errorf("first tick %d, want 0", frames[0].Tick)
	}
	if got := frames[1].Tick - frames[0].Tick; got != 200 {
		t.Errorf("tick step %d, want 200", got)
	}
}

func TestStream_Span(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH")
	}
	src, err := Stream(makeTestVideo(t), StreamOpts{BeginMs: 500, EndMs: 1500, FPS: 5, W: 64, H: 48})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	frames := Drain(src)
	if len(frames) < 4 || len(frames) > 6 {
		t.Fatalf("got %d frames, want ~5", len(frames))
	}
	if frames[0].Tick != 500 {
		t.Errorf("first tick %d, want 500", frames[0].Tick)
	}
	last := frames[len(frames)-1].Tick
	if last > 1500 {
		t.Errorf("last tick %d beyond span end 1500", last)
	}
}

func TestStream_CloseEarly(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH")
	}
	src, err := Stream(makeTestVideo(t), StreamOpts{FPS: 25, W: 64, H: 48})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := src.Next(); !ok {
		t.Fatal("expected at least one frame")
	}
	if err := src.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// After Close the source must simply report exhaustion, not hang.
	if _, ok := src.Next(); ok {
		t.Error("Next after Close should report exhaustion")
	}
}
