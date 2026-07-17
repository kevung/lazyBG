// Board observation for manual entry (issue #23): the perception seam that
// activates the fused candidate ranking (issue #15) in real use. When a session
// is calibrated the GUI wires a real frame→rectify→read observer here; entering
// dice reads the board near the current video tick and feeds the board-diff cue
// into rankMoves. With no reader, no calibration, or an unreadable frame the
// observation is simply absent and ranking degrades to equity-only — exactly the
// skeleton's behaviour (criterion 2).
//
// The observation logic lives in the session core, not the Wails binding
// (ADR-0003): gui/app.go only injects the reader and enables capture-backed
// frame grabbing.
package session

import (
	"image"

	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/geom"
	"lazybg/internal/perceive"
)

// BoardReader reads a rectified board image into a per-point ObservedBoard —
// the same seam the automatic pipeline uses (transcribe.boardReader): the
// learned pointnet.Reader or the classical boardstate.CircleReader.
type BoardReader interface {
	Read(img image.Image, cb calibrate.CanonicalBoard) perceive.ObservedBoard
}

// frameGrabber decodes a single frame at a video tick (ms). The real grabber is
// capture.FrameAt over the session's video; tests inject a fake.
type frameGrabber func(tickMs int) (image.Image, error)

// EnableVideoObservation wires a real board observer into the session: reader
// classifies each rectified frame, and frames are grabbed from the session's
// video with the bundled ffmpeg. Passing a nil reader disables observation
// (ranking reverts to equity-only). The GUI calls this whenever it opens or
// resumes a session (gui/app.go). It is idempotent and safe to re-call.
func (s *Service) EnableVideoObservation(reader BoardReader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reader = reader
	if reader == nil {
		s.grab = nil
		return
	}
	s.grab = func(tickMs int) (image.Image, error) {
		return capture.FrameAt(s.videoFileLocked(), tickMs)
	}
}

// videoFileLocked is the session's (first Part's) local video path. Caller
// holds s.mu.
func (s *Service) videoFileLocked() string {
	if s.doc == nil || len(s.doc.Parts) == 0 {
		return ""
	}
	return s.doc.Parts[0].File
}

// observeLocked reads the board near tickMs and returns the observation, or nil
// when no usable reading is possible (no reader/grabber wired, uncalibrated
// session, degenerate calibration, or an undecodable frame). Caller holds s.mu.
func (s *Service) observeLocked(tickMs int) *perceive.ObservedBoard {
	if s.reader == nil || s.grab == nil {
		return nil
	}
	if s.doc == nil || len(s.doc.Parts) == 0 {
		return nil
	}
	cal, cb, ok := buildCalibration(s.doc.Parts[0].Calibration)
	if !ok {
		return nil
	}
	frame, err := s.grab(tickMs)
	if err != nil || frame == nil {
		return nil
	}
	ob := s.reader.Read(cal.RectifyMasked(frame), cb)
	return &ob
}

// buildCalibration turns a stored corpus.Calibration into the geometry the
// board reader needs — the learned reader needs geometry only, so this is the
// color-free half of transcribe.PartSetup. ok is false when the session is not
// calibrated (needs 4 corners) or the homography is degenerate.
func buildCalibration(c corpus.Calibration) (calibrate.BoardCalibration, calibrate.CanonicalBoard, bool) {
	if len(c.Corners) != 4 {
		return calibrate.BoardCalibration{}, calibrate.CanonicalBoard{}, false
	}
	var corners [4]geom.Pt
	for i, p := range c.Corners {
		corners[i] = geom.P(p[0], p[1])
	}
	cb := calibrate.DefaultCanonical()
	if cn := c.Canonical; cn != nil {
		cb = calibrate.CanonicalBoard{MarginX: cn.MarginX, MarginY: cn.MarginY,
			PointW: cn.PointW, QuadH: cn.QuadH, BarGap: cn.BarGap, OffW: cn.OffW}
	}
	lens := calibrate.Lens{}
	if l := c.Lens; l != nil {
		lens = calibrate.Lens{K1: l.K1, CenterX: l.CenterX, CenterY: l.CenterY, Norm: l.Norm}
	}
	cal, ok := calibrate.NewWithLens(corners, cb, lens)
	if !ok {
		return calibrate.BoardCalibration{}, cb, false
	}
	for _, z := range c.Masks {
		cal.Masks = append(cal.Masks, image.Rect(z[0], z[1], z[2], z[3]))
	}
	return cal, cb, true
}
