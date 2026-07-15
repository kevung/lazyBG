package transcribe

import (
	"fmt"
	"image"
	"io"
	"path/filepath"

	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/geom"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boardstate"
	"lazybg/internal/perceive/checker"
	"lazybg/internal/perceive/clockhit"
	"lazybg/internal/perceive/pointnet"
	"lazybg/internal/perceive/stableframe"
	"lazybg/internal/profile"
)

// RunOptions tunes the video half of a Recording run. The perception
// parameters are the values validated on the pilot capture; per-capture
// overrides belong in the manifest when a capture needs them.
type RunOptions struct {
	Conduct Options

	FPS              float64 // segmentation sampling rate
	StreamW, StreamH int     // segmentation stream size
	MaxMotion        float64 // stableframe threshold on the low-res stream
	MinFrames        int     // minimum still frames per stable window
	PeakFrac         float64 // circle-detector peak threshold

	// ModelPath, when set, reads points with the learned classifier
	// (internal/perceive/pointnet) instead of the classical shape-first
	// reader.
	ModelPath string

	// LimitMs stops each Part this long after its span begins (0 = full
	// span) — the knob that keeps integration tests short.
	LimitMs int

	// Log, when non-nil, receives one line per stage/event for long runs.
	Log io.Writer
}

// DefaultRunOptions is the pilot-validated starting tuning.
func DefaultRunOptions() RunOptions {
	return RunOptions{
		Conduct:   DefaultOptions(),
		FPS:       3,
		StreamW:   320,
		StreamH:   180,
		MaxMotion: 1.5,
		MinFrames: 4,
		PeakFrac:  0.38,
	}
}

// PartSetup converts a manifest Part into its perception inputs: the
// homography, the canonical board geometry, and the declared checker colors.
func PartSetup(part corpus.Part) (calibrate.BoardCalibration, calibrate.CanonicalBoard, profile.CaptureProfile, error) {
	var corners [4]geom.Pt
	if len(part.Calibration.Corners) != 4 {
		return calibrate.BoardCalibration{}, calibrate.CanonicalBoard{}, profile.CaptureProfile{},
			fmt.Errorf("part %q: needs 4 corners", part.File)
	}
	for i, c := range part.Calibration.Corners {
		corners[i] = geom.P(c[0], c[1])
	}
	cb := calibrate.DefaultCanonical()
	if c := part.Calibration.Canonical; c != nil {
		cb = calibrate.CanonicalBoard{MarginX: c.MarginX, MarginY: c.MarginY,
			PointW: c.PointW, QuadH: c.QuadH, BarGap: c.BarGap, OffW: c.OffW}
	}
	lens := calibrate.Lens{}
	if l := part.Calibration.Lens; l != nil {
		lens = calibrate.Lens{K1: l.K1, CenterX: l.CenterX, CenterY: l.CenterY, Norm: l.Norm}
	}
	cal, ok := calibrate.NewWithLens(corners, cb, lens)
	if !ok {
		return calibrate.BoardCalibration{}, cb, profile.CaptureProfile{},
			fmt.Errorf("part %q: degenerate calibration", part.File)
	}
	ca, err := profile.ParseHex(part.Priors.CheckerA)
	if err != nil {
		return cal, cb, profile.CaptureProfile{}, fmt.Errorf("part %q: checkerA: %w", part.File, err)
	}
	cbc, err := profile.ParseHex(part.Priors.CheckerB)
	if err != nil {
		return cal, cb, profile.CaptureProfile{}, fmt.Errorf("part %q: checkerB: %w", part.File, err)
	}
	return cal, cb, profile.CaptureProfile{CheckerA: ca, CheckerB: cbc}, nil
}

// Recording runs the full video front half over every Part of a manifest —
// stream → stable windows → full-res board reading — and conducts the
// resulting events into a transcription. root is the directory the
// manifest's relative paths hang from (the repository root for the corpus).
func Recording(root string, m corpus.Manifest, o RunOptions) (Outcome, error) {
	events, commits, err := ReadEventsAndCommits(root, m, o)
	if err != nil {
		return Outcome{}, err
	}
	if m.Parts[0].Priors.MatchLength > 0 {
		o.Conduct.MatchLength = m.Parts[0].Priors.MatchLength
	}
	// Per-recording gate experiment, MEASURED and shelved (2026-07-15
	// mini-sweep): gate 0.72 on well-calibrated captures tripled coverage
	// but at 71% precision (4 confident errors / 14 auto-fills) vs 100% at
	// 0.80 — and a confident wrong ply costs more than a review
	// (experiment-plan §6). The mechanism (openingScore in the manifest)
	// stays for the day the dice-value cue provides the missing
	// discriminator; until then the gate holds at the default.
	return RunEventsWithCommits(events, commits, o.Conduct), nil
}

// boardReader is the per-frame reading seam: classical or learned.
type boardReader interface {
	Read(img image.Image, cb calibrate.CanonicalBoard) perceive.ObservedBoard
}

// ReadEvents extracts the observed stable-board events of every Part.
func ReadEvents(root string, m corpus.Manifest, o RunOptions) ([]Event, error) {
	events, _, err := ReadEventsAndCommits(root, m, o)
	return events, err
}

// ReadEventsAndCommits also collects clock-press commit ticks when the
// manifest declares a clock ROI (the press detector validated at ~77% turn
// coverage on the pilot). The presses ride the SAME low-res stream as the
// stable-window scan — a tee, not a second decode.
func ReadEventsAndCommits(root string, m corpus.Manifest, o RunOptions) ([]Event, []int, error) {
	var learned *pointnet.Net
	if o.ModelPath != "" {
		var err error
		learned, err = pointnet.Load(o.ModelPath)
		if err != nil {
			return nil, nil, err
		}
	}
	var events []Event
	var commits []int
	for pi, part := range m.Parts {
		cal, cb, prof, err := PartSetup(part)
		if err != nil {
			return nil, nil, err
		}
		video := filepath.Join(root, part.File)

		// Source dimensions: decode one frame at span begin.
		first, err := capture.FrameAt(video, part.Span.BeginMs)
		if err != nil {
			return nil, nil, fmt.Errorf("part %d: probe: %w", pi, err)
		}
		srcW, srcH := first.Bounds().Dx(), first.Bounds().Dy()

		endMs := part.Span.EndMs
		if o.LimitMs > 0 && part.Span.BeginMs+o.LimitMs < endMs {
			endMs = part.Span.BeginMs + o.LimitMs
		}
		src, err := capture.Stream(video, capture.StreamOpts{
			BeginMs: part.Span.BeginMs, EndMs: endMs,
			FPS: o.FPS, W: o.StreamW, H: o.StreamH,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("part %d: stream: %w", pi, err)
		}

		var clock *clockhit.Detector
		if r := part.Priors.ClockROI; r != [4]int{} {
			sx := float64(o.StreamW) / float64(srcW)
			sy := float64(o.StreamH) / float64(srcH)
			clock = clockhit.New(clockhit.Options{ROI: image.Rect(
				int(float64(r[0])*sx), int(float64(r[1])*sy),
				int(float64(r[2])*sx), int(float64(r[3])*sy))})
		}
		tee := teeSource{src: src, fn: func(f capture.Frame) {
			if clock == nil {
				return
			}
			for _, hit := range clock.Feed(f) {
				commits = append(commits, hit.Tick)
			}
		}}

		roi := scaledBBox(part.Calibration.Corners, srcW, srcH, o.StreamW, o.StreamH)
		var reader boardReader = boardstate.CircleReader{Profile: prof, Params: checker.Params{PeakFrac: o.PeakFrac}}
		if learned != nil {
			reader = pointnet.Reader{Net: learned}
		}
		d := stableframe.Detector{ROI: roi, MaxMotion: o.MaxMotion, MinFrames: o.MinFrames}

		nWin := 0
		d.EachWindow(tee, func(w stableframe.Window) bool {
			nWin++
			// Read the window's INTERIOR, away from both edges (the edges abut
			// the motion that bounded the window), and vote the reads.
			span := w.EndTick - w.StartTick
			var reads []perceive.ObservedBoard
			for _, frac := range []float64{0.3, 0.5, 0.7} {
				tick := w.StartTick + int(float64(span)*frac)
				full, err := capture.FrameAt(video, tick)
				if err != nil {
					continue // a bad decode skips the read, not the window
				}
				reads = append(reads, reader.Read(cal.Rectify(full), cb))
			}
			if len(reads) == 0 {
				return true
			}
			mid := w.StartTick + span/2
			events = append(events, Event{Tick: mid, Part: pi, Obs: VoteObservations(reads)})
			if o.Log != nil {
				fmt.Fprintf(o.Log, "part %d window %d @%dms (%d frames, %d reads)\n", pi, nWin, mid, w.Frames, len(reads))
			}
			return true
		})
		src.Close()
		if o.Log != nil {
			fmt.Fprintf(o.Log, "part %d: %d stable windows\n", pi, nWin)
		}
	}
	return events, commits, nil
}

// teeSource forwards frames while feeding a side observer.
type teeSource struct {
	src capture.Source
	fn  func(capture.Frame)
}

func (t teeSource) Next() (capture.Frame, bool) {
	f, ok := t.src.Next()
	if ok {
		t.fn(f)
	}
	return f, ok
}

// scaledBBox is the corners' bounding box scaled from source to stream size.
func scaledBBox(corners [][2]float64, srcW, srcH, dstW, dstH int) image.Rectangle {
	minX, minY := corners[0][0], corners[0][1]
	maxX, maxY := minX, minY
	for _, c := range corners[1:] {
		minX, maxX = min(minX, c[0]), max(maxX, c[0])
		minY, maxY = min(minY, c[1]), max(maxY, c[1])
	}
	sx, sy := float64(dstW)/float64(srcW), float64(dstH)/float64(srcH)
	return image.Rect(int(minX*sx), int(minY*sy), int(maxX*sx), int(maxY*sy))
}

// obsFlipSides swaps the A/B ownership of a reading — the orientation prior
// when CheckerA belongs to P2.
func obsFlipSides(ob perceive.ObservedBoard) perceive.ObservedBoard {
	for p := 1; p <= 24; p++ {
		switch ob.Points[p].Side {
		case perceive.A:
			ob.Points[p].Side = perceive.B
		case perceive.B:
			ob.Points[p].Side = perceive.A
		}
	}
	return ob
}

// VoteObservations fuses several readings of the same stable window into one:
// per point, the majority (count, side) wins; the confidence is the mean
// confidence of the agreeing reads scaled by the agreement fraction, so a
// point that flickered across reads is trusted less. Sporadic per-frame
// misreads (a hand edge, a die, motion blur) rarely survive the vote.
func VoteObservations(obs []perceive.ObservedBoard) perceive.ObservedBoard {
	if len(obs) == 1 {
		return obs[0]
	}
	var out perceive.ObservedBoard
	type key struct {
		count int
		side  perceive.Side
	}
	for p := 1; p <= 24; p++ {
		votes := map[key]int{}
		confSum := map[key]float64{}
		for _, ob := range obs {
			k := key{ob.Points[p].Count, ob.Points[p].Side}
			votes[k]++
			confSum[k] += ob.Points[p].Confidence
		}
		var bk key
		best := -1
		for k, n := range votes {
			if n > best || (n == best && confSum[k] > confSum[bk]) {
				bk, best = k, n
			}
		}
		frac := float64(best) / float64(len(obs))
		out.Points[p] = perceive.PointObs{
			Count: bk.count, Side: bk.side,
			Confidence: (confSum[bk] / float64(best)) * frac,
		}
	}
	return out
}
