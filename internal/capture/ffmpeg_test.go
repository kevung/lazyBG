package capture

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func ffmpegAvailable() bool {
	_, err := exec.LookPath(FFmpegBin)
	return err == nil
}

// makeTestVideo renders a 2s 320x240 test pattern to a temp mp4.
func makeTestVideo(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.mp4")
	cmd := exec.Command(FFmpegBin, "-nostdin", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=2:size=320x240:rate=25", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("make test video: %v", err)
	}
	return path
}

func TestFrameAt_Real(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH")
	}
	img, err := FrameAt(makeTestVideo(t), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 320 || b.Dy() != 240 {
		t.Errorf("frame size %dx%d, want 320x240", b.Dx(), b.Dy())
	}
}

func TestFFmpegSource_Real(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH")
	}
	src := NewFFmpegSource(makeTestVideo(t), []int{200, 800, 1500})
	frames := Drain(src)
	if len(frames) != 3 {
		t.Fatalf("got %d frames, want 3", len(frames))
	}
	for i, f := range frames {
		if f.Img == nil {
			t.Errorf("frame %d has nil image", i)
		}
	}
	if frames[0].Tick != 200 || frames[2].Tick != 1500 {
		t.Errorf("ticks not preserved in order: %d..%d", frames[0].Tick, frames[2].Tick)
	}
}
