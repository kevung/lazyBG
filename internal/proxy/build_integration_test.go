package proxy

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lazybg/internal/capture"
)

func ffmpegAvailable() bool {
	if _, err := exec.LookPath(capture.FFmpegBin); err != nil {
		return false
	}
	_, err := exec.LookPath(FFprobeBin)
	return err == nil
}

// genClip renders a 1s synthetic clip with the given video codec/container.
func genClip(t *testing.T, dir, name, vcodec string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	cmd := exec.Command(capture.FFmpegBin, "-nostdin", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=25",
		"-c:v", vcodec, out)
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot generate %s clip (%s missing?): %v", vcodec, vcodec, err)
	}
	return out
}

func TestBuildRemuxesH264AndCaches(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg/ffprobe not on PATH")
	}
	dir := t.TempDir()
	src := genClip(t, dir, "src.mkv", "libx264")
	cache := filepath.Join(dir, "cache")

	out, warn, err := Build(src, cache)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out, ".mp4") {
		t.Errorf("proxy should be .mp4, got %s", out)
	}
	if c := videoCodec(out); c != "h264" {
		t.Errorf("proxy video codec = %q, want h264", c)
	}
	if warn != "" {
		t.Errorf("remux should preserve duration, got warning: %s", warn)
	}
	// Second call is a cache hit → same path, no rebuild error.
	out2, _, err := Build(src, cache)
	if err != nil || out2 != out {
		t.Errorf("cache hit failed: out2=%s err=%v", out2, err)
	}
}

func TestBuildTranscodesNonH264(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg/ffprobe not on PATH")
	}
	dir := t.TempDir()
	src := genClip(t, dir, "src.webm", "libvpx") // VP8
	cache := filepath.Join(dir, "cache")

	out, warn, err := Build(src, cache)
	if err != nil {
		t.Fatal(err)
	}
	if c := videoCodec(out); c != "h264" {
		t.Errorf("transcoded proxy video codec = %q, want h264", c)
	}
	if warn != "" {
		t.Errorf("transcode should preserve duration, got warning: %s", warn)
	}
}
