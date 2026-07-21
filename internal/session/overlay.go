// Perception Overlay backing (issue #36, domain-model §3 "Perception Overlay"):
// for a stabilised video tick, run the board readers on the rectified frame and
// return their detections in CANONICAL coordinates. The GUI de-projects them
// onto the ROI-cropped video frame with the same homography it uses for the
// calibration grid (gui/frontend/src/lib/calibration.js), so the drawing math
// lives in one place. This is read-only: it never touches the transcription.
//
// Unlike observeLocked (perception-in, which reorients onto canonical bg
// numbering for fusion), the overlay returns the RAW reader output: Points[i]
// is the reading at canonical region i, i.e. the physical board location the
// user is looking at. Orientation is irrelevant to what's drawn on the frame.
package session

import (
	"image"
	"image/color"

	"lazybg/internal/perceive"
	"lazybg/internal/perceive/checker"
	"lazybg/internal/perceive/dice"
)

// OverlayDot is a detected point in canonical pixel coordinates.
type OverlayDot struct {
	X, Y  int
	Score float64
}

// OverlayView is the perception evidence for one tick, in canonical coordinates.
// OK is false when no reading was possible (uncalibrated, no reader/grabber, or
// an undecodable frame) — the GUI then shows the calibration grid only.
type OverlayView struct {
	OK             bool
	Points         [25]perceive.PointObs // raw per-region reading (index 1..24)
	Circles        []OverlayDot          // detected checker-disc centres
	Pips           []OverlayDot          // detected dice-pip centres
	CanonW, CanonH int                   // canonical board size
	Corners        [][2]float64          // calibrated source corners (TL,TR,BR,BL)
	BarEdges       [][2]float64          // bar-edge handles (barTL,barTR,barBR,barBL), if any
}

// Overlay reads the board at tickMs and returns its detections for the GUI
// overlay. Caller-facing (Wails-bound via gui/app.go).
func (s *Service) Overlay(tickMs int) OverlayView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := OverlayView{}
	if s.doc == nil || len(s.doc.Parts) == 0 {
		return out
	}
	out.Corners = s.doc.Parts[s.activePartIdx()].Calibration.Corners
	out.BarEdges = s.doc.Parts[s.activePartIdx()].Calibration.BarEdges
	cal, cb, ok := buildCalibration(s.doc.Parts[s.activePartIdx()].Calibration)
	out.CanonW, out.CanonH = cb.Size()
	if !ok || s.grab == nil {
		return out
	}
	frame, err := s.grab(tickMs)
	if err != nil || frame == nil {
		return out
	}
	rect := cal.RectifyMasked(frame)
	if s.reader != nil {
		out.Points = s.reader.Read(rect, cb).Points
	}
	// Layer 3a: checker discs (radius ≈ half a point column) on the rectified
	// grayscale.
	g := grayOf(rect)
	radius := cb.PointW / 2
	if radius < 4 {
		radius = 4
	}
	for _, c := range checker.Detect(g, radius) {
		out.Circles = append(out.Circles, OverlayDot{X: c.X, Y: c.Y, Score: c.Score})
	}
	// Layer 3b: dice pips in the central felt band (no declared dice ROI in the
	// MVP, so scan the middle 40–60% — the same band the runner's dice scan uses).
	band := image.Rect(0, out.CanonH*40/100, out.CanonW, out.CanonH*60/100)
	pipR := cb.PointW / 8
	if pipR < 3 {
		pipR = 3
	}
	for _, p := range dice.ReadPips(rect, band, pipR) {
		out.Pips = append(out.Pips, OverlayDot{X: p.X, Y: p.Y})
	}
	out.OK = true
	return out
}

// grayOf converts an image to grayscale for the shape detectors.
func grayOf(img image.Image) *image.Gray {
	b := img.Bounds()
	g := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, gg, bb, _ := img.At(x, y).RGBA()
			lum := (299*(r>>8) + 587*(gg>>8) + 114*(bb>>8)) / 1000
			g.SetGray(x, y, color.Gray{Y: uint8(lum)})
		}
	}
	return g
}
