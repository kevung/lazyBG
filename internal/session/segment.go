// Segmentation-proposed candidate ticks (issue #23, criterion 3): the stable
// instants of the match, offered to Tab/Shift+Tab so the transcriber jumps
// between committed positions instead of stepping a fixed 5s. This is the same
// stable-window segmentation the automatic pipeline uses (transcribe/runner),
// run here purely for navigation — it reads no board, only motion. The scan
// streams the whole span with the bundled ffmpeg, so the GUI runs it off the
// UI thread; until it finishes CandidateTicks returns nil and navigation falls
// back to recorded-turn ticks.
package session

import (
	"fmt"
	"image"

	"lazybg/internal/capture"
	"lazybg/internal/perceive/stableframe"
)

// Segmentation tuning — the pilot-validated low-res scan values (mirrors
// transcribe.DefaultRunOptions). Kept local so the session core need not import
// the transcribe runner.
const (
	segFPS       = 3.0
	segStreamW   = 320
	segStreamH   = 180
	segMaxMotion = 1.5
	segMinFrames = 4
)

// segmentTicks returns the mid-tick of every stable window a detector finds in
// src. Pure and O(1)-memory (stableframe.EachWindow holds one frame pair) —
// unit-tested with a fake source, no ffmpeg.
func segmentTicks(src capture.Source, roi image.Rectangle, maxMotion float64, minFrames int) []int {
	d := stableframe.Detector{ROI: roi, MaxMotion: maxMotion, MinFrames: minFrames}
	var ticks []int
	d.EachWindow(src, func(w stableframe.Window) bool {
		ticks = append(ticks, w.StartTick+(w.EndTick-w.StartTick)/2)
		return true
	})
	return ticks
}

// CandidateTicks returns the segmentation-proposed candidate ticks for this
// session's video (ascending), or nil if none have been computed yet.
func (s *Service) CandidateTicks() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.candidateTicks == nil {
		return nil
	}
	return append([]int(nil), s.candidateTicks...)
}

// ComputeCandidateTicks scans the session's video for stable windows and stores
// their mid-ticks as navigation candidates. It streams the whole active span
// with the bundled ffmpeg — slow; call it off the UI thread. It is a no-op
// (returns 0) when the session is uncalibrated (no ROI to watch) or a scan is
// already running; safe to call more than once.
func (s *Service) ComputeCandidateTicks() (int, error) {
	s.mu.Lock()
	if s.segmenting || s.doc == nil || len(s.doc.Parts) == 0 {
		s.mu.Unlock()
		return 0, nil
	}
	part := s.doc.Parts[s.activePartIdx()]
	if len(part.Calibration.Corners) != 4 || part.File == "" {
		s.mu.Unlock()
		return 0, nil
	}
	s.segmenting = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.segmenting = false
		s.mu.Unlock()
	}()

	video := part.File
	beginMs, endMs := part.Span.BeginMs, part.Span.EndMs
	first, err := capture.FrameAt(video, beginMs)
	if err != nil {
		return 0, fmt.Errorf("probe %s: %w", video, err)
	}
	srcW, srcH := first.Bounds().Dx(), first.Bounds().Dy()
	if endMs <= beginMs {
		dur, err := capture.DurationMs(video)
		if err != nil {
			return 0, fmt.Errorf("duration %s: %w", video, err)
		}
		endMs = dur
	}
	src, err := capture.Stream(video, capture.StreamOpts{
		BeginMs: beginMs, EndMs: endMs, FPS: segFPS, W: segStreamW, H: segStreamH,
	})
	if err != nil {
		return 0, err
	}
	roi := scaledBBox(part.Calibration.Corners, srcW, srcH, segStreamW, segStreamH)
	ticks := segmentTicks(src, roi, segMaxMotion, segMinFrames)
	src.Close()

	s.mu.Lock()
	s.candidateTicks = ticks
	s.mu.Unlock()
	return len(ticks), nil
}

// scaledBBox is the corners' bounding box scaled from source to stream size —
// the motion-watch ROI (same construction as transcribe.scaledBBox).
func scaledBBox(corners [][2]float64, srcW, srcH, dstW, dstH int) image.Rectangle {
	if len(corners) == 0 || srcW == 0 || srcH == 0 {
		return image.Rectangle{}
	}
	minX, minY := corners[0][0], corners[0][1]
	maxX, maxY := minX, minY
	for _, c := range corners[1:] {
		if c[0] < minX {
			minX = c[0]
		}
		if c[0] > maxX {
			maxX = c[0]
		}
		if c[1] < minY {
			minY = c[1]
		}
		if c[1] > maxY {
			maxY = c[1]
		}
	}
	sx, sy := float64(dstW)/float64(srcW), float64(dstH)/float64(srcH)
	return image.Rect(int(minX*sx), int(minY*sy), int(maxX*sx), int(maxY*sy))
}
