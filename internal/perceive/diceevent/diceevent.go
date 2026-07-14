// Package diceevent detects dice landing on and leaving the felt — the
// appearance/removal Commit signal of architecture §3, built on the
// abandoned-object literature's dual-background recipe (see
// docs/research/dice-reading-survey.md): a fast and a slow running-average
// background; a settled new object is foreground against the SLOW model but
// already absorbed by the FAST one, hands are foreground against both (and
// huge), and a removal leaves a slow-foreground hole whose pixels match the
// felt again.
package diceevent

import (
	"image"
	"image/color"

	"lazybg/internal/capture"
)

// Kind tells what happened at a spot on the felt.
type Kind int

const (
	Appeared Kind = iota // a die (small stable object) landed
	Removed              // it was picked back up
)

func (k Kind) String() string {
	if k == Removed {
		return "removed"
	}
	return "appeared"
}

// Event is one detected dice-zone change.
type Event struct {
	Tick int // first frame of the stable run
	Kind Kind
	Box  image.Rectangle
}

// Options tunes the detector. Zero values take the defaults below.
type Options struct {
	Felt color.RGBA // declared felt color (removal test)

	AlphaFast float64 // fast background learning rate (default 0.45 — the
	// mask needs |luma-fast| to fall under Thresh within ~3 samples of a
	// die landing; the event tick therefore lags the landing by ~1-2
	// samples, fine for a commit cue)
	AlphaSlow float64 // slow background learning rate (default 0.04)
	Thresh    float64 // per-pixel |Δluma| foreground threshold (default 18)
	MinArea   int     // smallest blob kept, px (default 12)
	MaxArea   int     // largest blob kept, px — hands are bigger (default 900)
	MinFrames int     // stable frames before an event fires (default 3)
	// MotionSkip: fraction of ROI pixels in fast-foreground above which the
	// frame is "a hand is here" — backgrounds freeze, nothing fires.
	MotionSkip float64 // default 0.08
}

func defaults(o Options) Options {
	if o.AlphaFast == 0 {
		o.AlphaFast = 0.45
	}
	if o.AlphaSlow == 0 {
		o.AlphaSlow = 0.04
	}
	if o.Thresh == 0 {
		o.Thresh = 18
	}
	if o.MinArea == 0 {
		o.MinArea = 12
	}
	if o.MaxArea == 0 {
		o.MaxArea = 900
	}
	if o.MinFrames == 0 {
		o.MinFrames = 3
	}
	if o.MotionSkip == 0 {
		o.MotionSkip = 0.08
	}
	return o
}

// Detector is a streaming dual-background detector. Feed frames in order.
type Detector struct {
	o          Options
	w, h       int
	fast, slow []float64 // per-pixel luma backgrounds
	inited     bool

	pending map[image.Point]*track // candidate blobs by rounded center
	latched []image.Rectangle      // appeared dice not yet removed
}

type track struct {
	box       image.Rectangle
	kind      Kind
	frames    int
	firstTick int
}

// New builds a Detector.
func New(o Options) *Detector {
	return &Detector{o: defaults(o), pending: map[image.Point]*track{}}
}

// Feed consumes the next frame and returns the events that became stable.
func (d *Detector) Feed(f capture.Frame) []Event {
	img := f.Img
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if !d.inited {
		d.w, d.h = w, h
		d.fast = make([]float64, w*h)
		d.slow = make([]float64, w*h)
		i := 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				l := lumaAt(img, x, y)
				d.fast[i], d.slow[i] = l, l
				i++
			}
		}
		d.inited = true
		return nil
	}
	if w != d.w || h != d.h {
		return nil // geometry changed; ignore rather than corrupt state
	}

	// Classify pixels against both backgrounds.
	luma := make([]float64, w*h)
	fastFG := 0
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			l := lumaAt(img, x, y)
			luma[i] = l
			if abs64(l-d.fast[i]) > d.o.Thresh {
				fastFG++
			}
			i++
		}
	}
	// Hand gate: too much fast-foreground = motion. The SLOW model freezes
	// (a hand must not leak into the scene memory), but the FAST one keeps
	// tracking — otherwise a board change during the gated stretch leaves
	// the fast model stale, every later frame re-triggers the gate, and the
	// detector freezes for good.
	if float64(fastFG) > d.o.MotionSkip*float64(w*h) {
		for i := range luma {
			d.fast[i] += d.o.AlphaFast * (luma[i] - d.fast[i])
		}
		return nil
	}

	// Stationary-change mask: differs from slow, agrees with fast.
	mask := make([]bool, w*h)
	for i := range mask {
		mask[i] = abs64(luma[i]-d.slow[i]) > d.o.Thresh && abs64(luma[i]-d.fast[i]) <= d.o.Thresh
	}

	events := d.trackBlobs(img, b, mask, f.Tick)

	// Update backgrounds (slow only where not masked, so a latched die does
	// not silently melt into the model before its removal can be seen —
	// removal is detected as a NEW stationary change at a latched box).
	for i := range luma {
		d.fast[i] += d.o.AlphaFast * (luma[i] - d.fast[i])
		if !mask[i] {
			d.slow[i] += d.o.AlphaSlow * (luma[i] - d.slow[i])
		} else {
			d.slow[i] += d.o.AlphaSlow / 4 * (luma[i] - d.slow[i])
		}
	}
	return events
}

// trackBlobs extracts die-sized stationary blobs, advances their persistence
// counters, and fires events when they stabilize.
func (d *Detector) trackBlobs(img image.Image, b image.Rectangle, mask []bool, tick int) []Event {
	w, h := d.w, d.h
	seen := make([]bool, len(mask))
	current := map[image.Point]image.Rectangle{}
	var stack []int
	for start := range mask {
		if !mask[start] || seen[start] {
			continue
		}
		stack = append(stack[:0], start)
		seen[start] = true
		area := 0
		minX, minY, maxX, maxY := w, h, 0, 0
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			area++
			x, y := i%w, i/w
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
			for _, j := range [4]int{i - 1, i + 1, i - w, i + w} {
				if j < 0 || j >= len(mask) || seen[j] || !mask[j] {
					continue
				}
				if (j == i-1 && x == 0) || (j == i+1 && x == w-1) {
					continue
				}
				seen[j] = true
				stack = append(stack, j)
			}
		}
		if area < d.o.MinArea || area > d.o.MaxArea {
			continue
		}
		box := image.Rect(minX, minY, maxX+1, maxY+1)
		key := image.Pt((minX+maxX)/8, (minY+maxY)/8) // coarse center bucket
		current[key] = box
	}

	var events []Event
	// advance or create tracks
	for key, box := range current {
		tr := d.pending[key]
		if tr == nil {
			kind := Appeared
			// A stationary change over a latched die whose pixels read as
			// felt again is the die leaving, not a new object.
			if d.overLatched(box) && d.mostlyFelt(img, b, box) {
				kind = Removed
			}
			d.pending[key] = &track{box: box, kind: kind, frames: 1, firstTick: tick}
			continue
		}
		tr.frames++
		tr.box = tr.box.Union(box)
		if tr.frames == d.o.MinFrames {
			events = append(events, Event{Tick: tr.firstTick, Kind: tr.kind, Box: tr.box})
			if tr.kind == Appeared {
				d.latched = append(d.latched, tr.box)
			} else {
				d.unlatch(tr.box)
			}
		}
	}
	// drop tracks whose blob vanished — including already-fired ones, so a
	// later change at the same spot (the removal) starts a fresh track.
	for key := range d.pending {
		if _, ok := current[key]; !ok {
			delete(d.pending, key)
		}
	}
	return events
}

func (d *Detector) overLatched(box image.Rectangle) bool {
	for _, l := range d.latched {
		if l.Overlaps(box) {
			return true
		}
	}
	return false
}

func (d *Detector) unlatch(box image.Rectangle) {
	kept := d.latched[:0]
	for _, l := range d.latched {
		if !l.Overlaps(box) {
			kept = append(kept, l)
		}
	}
	d.latched = kept
}

// mostlyFelt reports whether the box's pixels are back to the declared felt.
func (d *Detector) mostlyFelt(img image.Image, b image.Rectangle, box image.Rectangle) bool {
	hits, n := 0, 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			dr := float64(r>>8) - float64(d.o.Felt.R)
			dg := float64(g>>8) - float64(d.o.Felt.G)
			db := float64(bl>>8) - float64(d.o.Felt.B)
			n++
			if dr*dr+dg*dg+db*db < 45*45 {
				hits++
			}
		}
	}
	return n > 0 && float64(hits) >= 0.6*float64(n)
}

func lumaAt(img image.Image, x, y int) float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return (299*float64(r>>8) + 587*float64(g>>8) + 114*float64(b>>8)) / 1000
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
