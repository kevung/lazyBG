// Measuring the capture's board colours (issue #64): the two triangle colours,
// the felt, and the two checker colours, read off the current frame through the
// handles the user has in place. An explicit gesture, not magic — the setup
// panel's "sample colours from this frame" button — so the manual calibration
// path (the one taken precisely when detection fails) gets measured colours too.
//
// The colours stay DECLARED priors: sampling is an input method, not a
// competing authority. One value on disk, no precedence rule, and the user sees
// what the machine read and can correct it.
package session

import (
	"fmt"
	"image"
	"image/color"

	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/perceive/palette"
	"lazybg/internal/profile"
)

// PaletteSample is a measured board palette, in the priors' hex vocabulary.
type PaletteSample struct {
	PointA   string `json:"pointA"`
	PointB   string `json:"pointB"`
	Felt     string `json:"felt"`
	CheckerA string `json:"checkerA"`
	CheckerB string `json:"checkerB"`
	// HasCheckers is false when no checker cluster stood out — an empty board,
	// or a frame the sampler could not separate. The board colours are still
	// measured; the checker ones are the declared values echoed back, and the
	// GUI must say so rather than present a guess as a measurement.
	HasCheckers bool `json:"hasCheckers"`
}

// SamplePalette measures the palette at tickMs through the given calibration —
// the handles as they stand in the form, not as they were last saved, so the
// button works before the first save. declA/declB are the currently declared
// checker colours: the measured clusters are assigned to whichever they are
// closest to, so sampling refines the declaration without re-deciding which
// player is which (that is the swap gesture, ADR-0009).
func (s *Service) SamplePalette(tickMs int, cal corpus.Calibration, declA, declB string) (PaletteSample, error) {
	s.mu.Lock()
	video := s.videoFileLocked()
	grab := s.grab
	s.mu.Unlock()

	bcal, cb, ok := buildCalibration(cal)
	if !ok {
		return PaletteSample{}, fmt.Errorf("place the 4 corner handles before sampling the colours")
	}
	if grab == nil {
		if video == "" {
			return PaletteSample{}, fmt.Errorf("no video to sample from")
		}
		grab = func(t int) (image.Image, error) { return capture.FrameAt(video, t) }
	}
	frame, err := grab(tickMs)
	if err != nil || frame == nil {
		return PaletteSample{}, fmt.Errorf("could not read the frame at %dms: %w", tickMs, err)
	}
	ca, errA := profile.ParseHex(declA)
	cb2, errB := profile.ParseHex(declB)
	if errA != nil {
		ca = color.RGBA{231, 224, 213, 255}
	}
	if errB != nil {
		cb2 = color.RGBA{49, 34, 28, 255}
	}
	p, ok := palette.Sample(bcal.RectifyMasked(frame), cb, ca, cb2)
	if !ok {
		return PaletteSample{}, fmt.Errorf("the frame at %dms could not be rectified into a board", tickMs)
	}
	return PaletteSample{
		PointA:      hexOf(p.PointA),
		PointB:      hexOf(p.PointB),
		Felt:        hexOf(p.Felt),
		CheckerA:    hexOf(p.CheckerA),
		CheckerB:    hexOf(p.CheckerB),
		HasCheckers: p.HasCheckers,
	}, nil
}
