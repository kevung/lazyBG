// Automatic calibration seed (issue #47, ADR-0007): a best-effort detector that
// PRE-FILLS all eight calibration handles so the user adjusts rather than places
// from scratch. It is never the final word — the manual draggable handles remain
// the reliable path, and a failed detection leaves the current handles untouched
// (the GUI shows the reason and says "drag manually").
package session

import (
	"fmt"
	"log"

	"lazybg/internal/autocal"
	"lazybg/internal/corpus"
)

// DetectedHandles is a best-effort auto-calibration seed: the four playing-surface
// corners (TL,TR,BR,BL) and the four bar-edge points (barTL,barTR,barBR,barBL),
// all in source-frame pixels.
type DetectedHandles struct {
	Corners  [][2]float64
	BarEdges [][2]float64
	// Lens is the fit's admitted radial distortion (nil = pinhole), in the
	// manifest's schema so the GUI/session can persist it as-is.
	Lens *corpus.Lens
}

// DetectCorners detects the eight calibration handles on the frame at tickMs and
// returns them to seed the GUI. It works on the SINGLE current frame (a short
// median), so it returns quickly and does not require the opening position —
// unlike a full auto-calibration. On failure it returns the reason (surfaced by
// the GUI) and leaves the handles untouched; the user always has manual drag.
func (s *Service) DetectCorners(tickMs int) (DetectedHandles, error) {
	s.mu.Lock()
	video := s.videoFileLocked()
	s.mu.Unlock()
	if video == "" {
		return DetectedHandles{}, fmt.Errorf("no video to detect from")
	}
	corners, barEdges, lens, err := autocal.DetectHandles(video, tickMs, autocal.DefaultOptions())
	if err != nil {
		log.Printf("DetectCorners @%dms: %v", tickMs, err)
		return DetectedHandles{}, err
	}
	out := DetectedHandles{Corners: make([][2]float64, 4), BarEdges: make([][2]float64, 4)}
	if lens.K1 != 0 || lens.K2 != 0 {
		out.Lens = &corpus.Lens{K1: lens.K1, K2: lens.K2, CenterX: lens.CenterX, CenterY: lens.CenterY, Norm: lens.Norm}
	}
	for i := 0; i < 4; i++ {
		out.Corners[i] = [2]float64{corners[i].X, corners[i].Y}
		out.BarEdges[i] = [2]float64{barEdges[i].X, barEdges[i].Y}
	}
	return out, nil
}
