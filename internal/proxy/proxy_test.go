package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tmpFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFingerprintStableAndContentSensitive(t *testing.T) {
	a := tmpFile(t, "a.mkv", strings.Repeat("x", 4096))
	b := tmpFile(t, "b.mkv", strings.Repeat("x", 4096)) // same content, different path
	c := tmpFile(t, "c.mkv", strings.Repeat("y", 4096)) // different content

	fa, err := Fingerprint(a)
	if err != nil {
		t.Fatal(err)
	}
	fb, _ := Fingerprint(b)
	fc, _ := Fingerprint(c)

	if fa != fb {
		t.Errorf("same content should fingerprint equal: %s vs %s", fa, fb)
	}
	if fa == fc {
		t.Errorf("different content should fingerprint differently")
	}
	if fa == "" {
		t.Errorf("empty fingerprint")
	}
}

func TestPlanRemuxesH264(t *testing.T) {
	args := planArgs("in.mkv", "out.mp4", "h264")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-c copy") {
		t.Errorf("h264 source must be stream-copied (exact timeline), got: %s", joined)
	}
	if strings.Contains(joined, "libx264") {
		t.Errorf("h264 source must NOT be re-encoded, got: %s", joined)
	}
	// Never seek before input / never force fps — that would shift the timeline.
	if strings.Contains(joined, "-r ") || strings.Contains(joined, "-ss ") {
		t.Errorf("plan must not resample fps or pre-seek, got: %s", joined)
	}
	assertOrderedInputThenOutput(t, args, "in.mkv", "out.mp4")
}

func TestPlanTranscodesNonH264(t *testing.T) {
	for _, codec := range []string{"vp9", "mpeg4", "hevc", ""} {
		args := planArgs("in.webm", "out.mp4", codec)
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "libx264") {
			t.Errorf("codec %q must transcode to H.264, got: %s", codec, joined)
		}
		if !strings.Contains(joined, "aac") {
			t.Errorf("codec %q must produce AAC audio, got: %s", codec, joined)
		}
		if strings.Contains(joined, "-r ") {
			t.Errorf("codec %q: must not force fps (timeline), got: %s", codec, joined)
		}
	}
}

// The output path must come after the input so ffmpeg treats it as the target.
func assertOrderedInputThenOutput(t *testing.T, args []string, in, out string) {
	t.Helper()
	iIn, iOut := -1, -1
	for i, a := range args {
		if a == in {
			iIn = i
		}
		if a == out {
			iOut = i
		}
	}
	if iIn < 0 || iOut < 0 || iOut < iIn {
		t.Errorf("args must list input %q before output %q: %v", in, out, args)
	}
}
