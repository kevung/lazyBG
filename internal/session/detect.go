// Automatic calibration seed (issue #47, ADR-0007): a best-effort detector that
// PRE-FILLS all eight calibration handles so the user adjusts rather than places
// from scratch. It is never the final word — the manual draggable handles remain
// the reliable path, and a failed detection leaves the current handles untouched
// (the GUI shows the reason and says "drag manually").
package session

import (
	"fmt"
	"image/color"
	"log"
	"math"

	"lazybg/internal/autocal"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/geom"
)

// workspaceMargin is how far outside the frame a calibration handle may sit, as
// a fraction of the frame's width/height. It mirrors autocal.quadInBounds' 0.15
// on purpose: everything auto-calibration still considers a plausible board is
// representable — and draggable — in the GUI, and nothing beyond it is. The
// margin is not slack: four committed corpus manifests carry a corner a few
// pixels above the frame (the playing surface's top edge is cropped) and read
// correctly, so clamping onto the frame itself would rewrite working
// calibrations.
//
// The clamp lives HERE and not in internal/autocal on purpose: the ratcheted
// multi-capture bench calls autocal.DetectHandles directly and its committed
// baseline is bit-deterministic. The bench must keep measuring what detection
// really finds, overshoot included.
const workspaceMargin = 0.15

// clampHandles confines the eight detected handles to the workspace, per axis,
// and reports whether anything moved. A zero frame size leaves them untouched:
// better an unclamped handle than one teleported by a failed probe.
func clampHandles(corners, barEdges [4]geom.Pt, w, h int) ([4]geom.Pt, [4]geom.Pt, bool) {
	if w <= 0 || h <= 0 {
		return corners, barEdges, false
	}
	mx, my := workspaceMargin*float64(w), workspaceMargin*float64(h)
	clamped := false
	clamp := func(p geom.Pt) geom.Pt {
		q := geom.P(
			math.Min(math.Max(p.X, -mx), float64(w)+mx),
			math.Min(math.Max(p.Y, -my), float64(h)+my),
		)
		if q != p {
			clamped = true
		}
		return q
	}
	for i := range corners {
		corners[i] = clamp(corners[i])
	}
	for i := range barEdges {
		barEdges[i] = clamp(barEdges[i])
	}
	return corners, barEdges, clamped
}

// frameSize probes the video for its pixel dimensions — the same one-frame
// probe segmentation uses. Zeroes on failure, so clampHandles leaves the
// handles alone rather than guessing.
func frameSize(video string, tickMs int) (int, int) {
	img, err := capture.FrameAt(video, tickMs)
	if err != nil {
		log.Printf("frameSize %s @%dms: %v", video, tickMs, err)
		return 0, 0
	}
	return img.Bounds().Dx(), img.Bounds().Dy()
}

// DetectedHandles is a best-effort auto-calibration seed: the four playing-surface
// corners (TL,TR,BR,BL) and the four bar-edge points (barTL,barTR,barBR,barBL),
// all in source-frame pixels.
type DetectedHandles struct {
	Corners  [][2]float64
	BarEdges [][2]float64
	// Lens is the fit's admitted radial distortion (nil = pinhole), in the
	// manifest's schema so the GUI/session can persist it as-is.
	Lens *corpus.Lens
	// Clamped reports that the fit put at least one handle beyond the
	// workspace and it was pulled back (#61). The GUI turns this into an
	// explicit message: either the detection ran off, or the video does not
	// show the whole board — it cannot tell which, but it must not stay silent.
	Clamped bool
	// PointA/PointB/Felt are the board's own colours, as hex — the hypothesis
	// the winning fit was found under, so they are measured and already
	// adjudicated rather than guessed a second time (#64). Empty when the
	// detection fell back to a quad with no colour hypothesis behind it.
	PointA string
	PointB string
	Felt   string
}

// hexOf renders a measured colour as "#rrggbb", or "" for the zero colour.
func hexOf(c color.RGBA) string {
	if c == (color.RGBA{}) {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
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
	corners, barEdges, lens, colors, err := autocal.DetectHandles(video, tickMs, autocal.DefaultOptions())
	if err != nil {
		log.Printf("DetectCorners @%dms: %v", tickMs, err)
		return DetectedHandles{}, err
	}
	w, h := frameSize(video, tickMs)
	corners, barEdges, clamped := clampHandles(corners, barEdges, w, h)
	out := DetectedHandles{
		Corners:  make([][2]float64, 4),
		BarEdges: make([][2]float64, 4),
		Clamped:  clamped,
		PointA:   hexOf(colors.PointA),
		PointB:   hexOf(colors.PointB),
		Felt:     hexOf(colors.Felt),
	}
	if lens.K1 != 0 || lens.K2 != 0 {
		out.Lens = &corpus.Lens{K1: lens.K1, K2: lens.K2, CenterX: lens.CenterX, CenterY: lens.CenterY, Norm: lens.Norm}
	}
	for i := 0; i < 4; i++ {
		out.Corners[i] = [2]float64{corners[i].X, corners[i].Y}
		out.BarEdges[i] = [2]float64{barEdges[i].X, barEdges[i].Y}
	}
	return out, nil
}
