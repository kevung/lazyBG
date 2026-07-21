// Automatic calibration seed (issue #47, ADR-0007): a best-effort corner
// detector that PRE-FILLS the calibration handles so the user adjusts rather
// than places from scratch. It is never the final word — the manual draggable
// handles remain the reliable path, and a failed or low-confidence detection
// simply leaves the current handles untouched (the GUI says "drag manually").
package session

import (
	"fmt"
	"log"

	"lazybg/internal/autocal"
	"lazybg/internal/geom"
)

// DetectedCorners is a best-effort auto-calibration seed: four playing-surface
// corners (TL,TR,BR,BL) in source-frame pixels, plus the opening-read score the
// detector achieved (of 24) as a rough confidence.
type DetectedCorners struct {
	Corners [][2]float64
	Score   int
}

// DetectCorners scans the session's video with the automatic calibrator and
// returns the four detected corners to seed the handles. It returns an error
// (and no corners) only on a hard failure — no video, or the detector found no
// plausible board. A low-confidence result is still returned: it is a seed, and
// the user refines it by dragging (issue #47).
func (s *Service) DetectCorners() (DetectedCorners, error) {
	s.mu.Lock()
	video := s.videoFileLocked()
	s.mu.Unlock()
	if video == "" {
		return DetectedCorners{}, fmt.Errorf("no video to detect from")
	}
	// autocal.Calibrate returns its best quad even when it distrusts the read
	// (a "below MinOpening" error still carries usable corners); only a truly
	// empty result means no board was found.
	res, err := autocal.Calibrate(video, autocal.DefaultOptions())
	if res.Corners == ([4]geom.Pt{}) { // zero quad ⇒ hard failure
		if err != nil {
			// Surface autocal's real reason (e.g. "could not derive board
			// colors", "no candidate quad produced a readable opening") so the
			// GUI can show it instead of a generic "failed".
			log.Printf("DetectCorners: %v", err)
			return DetectedCorners{}, fmt.Errorf("auto-calibration: %w", err)
		}
		return DetectedCorners{}, fmt.Errorf("auto-calibration found no board in the video")
	}
	out := make([][2]float64, 4)
	for i, p := range res.Corners {
		out[i] = [2]float64{p.X, p.Y}
	}
	return DetectedCorners{Corners: out, Score: res.OpeningScore}, nil
}
