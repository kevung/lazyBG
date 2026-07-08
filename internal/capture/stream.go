package capture

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"os/exec"
)

// StreamOpts configures a sampled, downscaled decode of a video span. The low
// rate + small size keep a whole-match scan cheap (motion analysis / turn
// segmentation); full-resolution single frames for reading the board come from
// FrameAt instead.
type StreamOpts struct {
	BeginMs int     // span start; 0 = file start
	EndMs   int     // span end; 0 = file end
	FPS     float64 // sampling rate, required > 0
	W, H    int     // output frame size, required
}

// StreamSource decodes frames with a single long-lived ffmpeg process writing
// raw RGB to a pipe — one process per scan instead of one per frame. It
// implements Source; Close releases the process early.
type StreamSource struct {
	cmd  *exec.Cmd
	out  io.ReadCloser
	errb *bytes.Buffer
	opts StreamOpts
	buf  []byte
	i    int
	done bool
}

// Stream starts a sampled decode of path over the given span.
func Stream(path string, o StreamOpts) (*StreamSource, error) {
	if o.FPS <= 0 || o.W <= 0 || o.H <= 0 {
		return nil, fmt.Errorf("stream %s: FPS, W, H are required, got %+v", path, o)
	}
	args := []string{"-nostdin", "-loglevel", "error"}
	if o.BeginMs > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", float64(o.BeginMs)/1000))
	}
	args = append(args, "-i", path)
	if o.EndMs > o.BeginMs {
		args = append(args, "-t", fmt.Sprintf("%.3f", float64(o.EndMs-o.BeginMs)/1000))
	}
	args = append(args,
		"-vf", fmt.Sprintf("fps=%g,scale=%d:%d", o.FPS, o.W, o.H),
		"-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1",
	)
	cmd := exec.Command(FFmpegBin, args...)
	errb := &bytes.Buffer{}
	cmd.Stderr = errb
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stream %s: %w", path, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("stream %s: %w", path, err)
	}
	return &StreamSource{
		cmd:  cmd,
		out:  out,
		errb: errb,
		opts: o,
		buf:  make([]byte, o.W*o.H*3),
	}, nil
}

// Next yields the next sampled frame. Ticks are derived from the sample index:
// BeginMs + i·(1000/FPS).
func (s *StreamSource) Next() (Frame, bool) {
	if s.done {
		return Frame{}, false
	}
	if _, err := io.ReadFull(s.out, s.buf); err != nil {
		s.Close()
		return Frame{}, false
	}
	img := image.NewRGBA(image.Rect(0, 0, s.opts.W, s.opts.H))
	for i, j := 0, 0; i < len(s.buf); i, j = i+3, j+4 {
		img.Pix[j] = s.buf[i]
		img.Pix[j+1] = s.buf[i+1]
		img.Pix[j+2] = s.buf[i+2]
		img.Pix[j+3] = 0xff
	}
	tick := s.opts.BeginMs + int(float64(s.i)*1000/s.opts.FPS)
	s.i++
	return Frame{Tick: tick, Img: img}, true
}

// Close stops the decode and reaps the process. Safe to call more than once
// and after natural exhaustion.
func (s *StreamSource) Close() error {
	if s.done {
		return nil
	}
	s.done = true
	s.out.Close()
	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	s.cmd.Wait()
	return nil
}
