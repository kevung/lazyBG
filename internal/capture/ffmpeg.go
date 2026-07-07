package capture

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png" // decode PNG frames from ffmpeg
	"os/exec"
)

// FFmpegBin is the ffmpeg executable used for decoding. The shipped app points
// this at the bundled binary; tests use whatever is on PATH.
var FFmpegBin = "ffmpeg"

// FrameAt decodes a single frame from a video at tickMs (milliseconds). It uses
// input seeking (-ss before -i), which is fast on long files but lands on the
// nearest preceding keyframe — good enough for reading a settled commit window;
// exact-seek (go-astiav) is a later upgrade (experiment-plan §8).
func FrameAt(path string, tickMs int) (image.Image, error) {
	if tickMs < 0 {
		tickMs = 0
	}
	cmd := exec.Command(FFmpegBin,
		"-nostdin", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", float64(tickMs)/1000.0),
		"-i", path,
		"-frames:v", "1",
		"-f", "image2pipe", "-c:v", "png", "pipe:1",
	)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg decode %s @%dms: %w: %s", path, tickMs, err, errb.String())
	}
	img, _, err := image.Decode(&out)
	if err != nil {
		return nil, fmt.Errorf("decode frame %s @%dms: %w", path, tickMs, err)
	}
	return img, nil
}

// FFmpegSource is a Source that decodes one real frame per requested tick, in
// order. It backs running the pipeline over a corpus clip at labeled commit
// ticks (the manifest's per-turn ticks feed straight in).
type FFmpegSource struct {
	Path  string
	Ticks []int
	i     int
}

// NewFFmpegSource builds a Source that yields a frame for each tick in order.
func NewFFmpegSource(path string, ticks []int) *FFmpegSource {
	return &FFmpegSource{Path: path, Ticks: ticks}
}

// Next decodes and yields the next tick's frame, or false when exhausted. A tick
// that fails to decode is skipped (it does not abort the whole source).
func (s *FFmpegSource) Next() (Frame, bool) {
	for s.i < len(s.Ticks) {
		tick := s.Ticks[s.i]
		s.i++
		img, err := FrameAt(s.Path, tick)
		if err != nil {
			continue
		}
		return Frame{Tick: tick, Img: img}, true
	}
	return Frame{}, false
}
