// Package stableframe finds the moments when the board is still — the instants
// worth reading (docs/architecture.md §3, §5; domain-model §3 "Stable Frame").
// It is the cheap classical gate that keeps CPU cost bounded: the board is read
// only on stable frames, never every frame. It also underpins the "last stable
// board before a Commit Event" rule that defeats player experimentation.
package stableframe

import (
	"image"
	"image/color"

	"lazybg/internal/capture"
)

// Motion is the mean absolute luma difference (0..255) between two frames within
// an ROI. 0 means identical; larger means more changed (a hand, a moving
// checker). An empty ROI means the whole (intersected) image.
func Motion(a, b image.Image, roi image.Rectangle) float64 {
	r := a.Bounds().Intersect(b.Bounds())
	if !roi.Empty() {
		r = r.Intersect(roi)
	}
	if r.Empty() {
		return 0
	}
	var sum int64
	var n int64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			d := int64(luma(a.At(x, y))) - int64(luma(b.At(x, y)))
			if d < 0 {
				d = -d
			}
			sum += d
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

// luma is the 8-bit Rec.601 luminance of a color.
func luma(c color.Color) int {
	r, g, b, _ := c.RGBA()
	// RGBA() returns 16-bit; shift to 8-bit and weight.
	return (299*int(r>>8) + 587*int(g>>8) + 114*int(b>>8)) / 1000
}

// Window is a maximal run of consecutive low-motion frames — a stable instant of
// the board. Rep is the settled (last) frame of the run, the one to read.
type Window struct {
	StartTick int
	EndTick   int
	Frames    int
	Rep       capture.Frame
}

// Detector finds stable windows in a frame stream.
type Detector struct {
	ROI       image.Rectangle // region to watch; empty = whole frame
	MaxMotion float64         // motion ≤ this between neighbours is "still"
	MinFrames int             // a window needs at least this many still frames
}

// Windows splits the stream into maximal still runs and returns those with at
// least MinFrames frames. The representative frame of each is its last (settled)
// frame — a strong player's committed position is the last stable one.
func (d Detector) Windows(src capture.Source) []Window {
	frames := capture.Drain(src)
	if len(frames) == 0 {
		return nil
	}
	minF := d.MinFrames
	if minF < 1 {
		minF = 1
	}

	var windows []Window
	segStart := 0
	flush := func(end int) {
		n := end - segStart + 1
		if n >= minF {
			windows = append(windows, Window{
				StartTick: frames[segStart].Tick,
				EndTick:   frames[end].Tick,
				Frames:    n,
				Rep:       frames[end],
			})
		}
	}
	for i := 1; i < len(frames); i++ {
		if Motion(frames[i-1].Img, frames[i].Img, d.ROI) > d.MaxMotion {
			flush(i - 1)
			segStart = i
		}
	}
	flush(len(frames) - 1)
	return windows
}
