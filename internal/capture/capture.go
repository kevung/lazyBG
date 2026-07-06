// Package capture is the video front-end boundary (docs/architecture.md §3,
// "capture"). It defines the Frame and Source types the perception detectors
// consume. The ffmpeg-backed Source that decodes a real video lands once a
// corpus clip exists; until then, SliceSource feeds synthetic frames.
package capture

import "image"

// Frame is a decoded frame at a video Tick (milliseconds into the capture).
type Frame struct {
	Tick int
	Img  image.Image
}

// Source yields frames in Tick order. Next returns false when exhausted.
type Source interface {
	Next() (Frame, bool)
}

// SliceSource is an in-memory Source over a fixed slice of frames — the test and
// synthetic-input driver.
type SliceSource struct {
	Frames []Frame
	i      int
}

// NewSliceSource wraps frames in a Source.
func NewSliceSource(frames []Frame) *SliceSource { return &SliceSource{Frames: frames} }

// Next yields the next frame, or false when the slice is exhausted.
func (s *SliceSource) Next() (Frame, bool) {
	if s.i >= len(s.Frames) {
		return Frame{}, false
	}
	f := s.Frames[s.i]
	s.i++
	return f, true
}

// Drain pulls every remaining frame from a Source into a slice.
func Drain(src Source) []Frame {
	var out []Frame
	for {
		f, ok := src.Next()
		if !ok {
			return out
		}
		out = append(out, f)
	}
}
