// Package clockhit detects chess-clock presses — the strongest Commit Event
// of the MVP vertical (architecture §3): a player ends their turn by hitting
// the clock, so a brief motion spike inside the declared clock ROI anchors a
// turn boundary. The detector is the cheap classical shape: a quiet→burst→
// quiet pattern in mean |Δluma|, with bursts too long to be a press (an arm
// parked over the clock, a re-arrangement) rejected.
package clockhit

import (
	"image"

	"lazybg/internal/capture"
	"lazybg/internal/perceive/stableframe"
)

// Hit is one detected clock press.
type Hit struct {
	Tick int // first frame of the motion burst
}

// Options tunes the detector. Zero values take the defaults.
type Options struct {
	ROI      image.Rectangle // the clock, in frame coordinates (Session Prior)
	Thresh   float64         // motion above this = a hand over the clock (default 6)
	MaxBurst int             // bursts longer than this many frames are not presses (default 6)
	MinQuiet int             // quiet frames required before a burst counts (default 2)
}

func defaults(o Options) Options {
	if o.Thresh == 0 {
		o.Thresh = 6
	}
	if o.MaxBurst == 0 {
		o.MaxBurst = 6
	}
	if o.MinQuiet == 0 {
		o.MinQuiet = 2
	}
	return o
}

// Detector is a streaming press detector. Feed frames in order.
type Detector struct {
	o Options

	prev      capture.Frame
	hasPrev   bool
	quiet     int // consecutive quiet frames before the current burst
	burst     int // consecutive moving frames
	burstTick int
}

// New builds a Detector.
func New(o Options) *Detector { return &Detector{o: defaults(o)} }

// Feed consumes the next sampled frame and returns a Hit when a completed
// press (quiet → short burst → quiet again) is recognized.
func (d *Detector) Feed(f capture.Frame) []Hit {
	if !d.hasPrev {
		d.prev, d.hasPrev = f, true
		d.quiet = d.o.MinQuiet // stream start counts as quiet
		return nil
	}
	m := stableframe.Motion(d.prev.Img, f.Img, d.o.ROI)
	d.prev = f

	if m > d.o.Thresh {
		if d.burst == 0 {
			d.burstTick = f.Tick
		}
		d.burst++
		return nil
	}
	// quiet frame: close any burst
	var hits []Hit
	if d.burst > 0 {
		if d.burst <= d.o.MaxBurst && d.quiet >= d.o.MinQuiet {
			hits = append(hits, Hit{Tick: d.burstTick})
		}
		d.burst = 0
		d.quiet = 0
	}
	d.quiet++
	return hits
}
