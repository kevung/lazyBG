package e2e

import (
	"image"
	"testing"

	"lazybg/internal/capture"
	"lazybg/internal/perceive/stableframe"
)

// Segmentation parameters validated on the pilot capture (see the motion
// distribution in the feature's test log: quiet board ≈0.3–1.8 mean |Δluma|,
// hands crossing ≈6–21). They are starting priors for competition footage, not
// universal constants; per-capture overrides can join the manifest if needed.
const (
	segFPS       = 3
	segW, segH   = 320, 180
	segMaxMotion = 1.5
	segMinFrames = 4 // ≥ ~1.3s of stillness at 3 fps
)

// boardROI is the calibration corners' bounding box scaled from source
// resolution (sw×sh) to the segmentation stream size.
func boardROI(corners [][2]float64, sw, sh int) image.Rectangle {
	minX, minY := corners[0][0], corners[0][1]
	maxX, maxY := minX, minY
	for _, c := range corners[1:] {
		minX, maxX = min(minX, c[0]), max(maxX, c[0])
		minY, maxY = min(minY, c[1]), max(maxY, c[1])
	}
	sx, sy := float64(segW)/float64(sw), float64(segH)/float64(sh)
	return image.Rect(int(minX*sx), int(minY*sy), int(maxX*sx), int(maxY*sy))
}

// TestRealCorpus_TurnSegmentation streams the pilot's first two minutes at low
// rate/resolution and splits them into stable board windows — the O(1)-memory
// front half of turn segmentation on real footage. The window count brackets
// the plies actually played in that span (a ply can yield an extra window when
// the player adjusts checkers; the board-change eventizer collapses those).
func TestRealCorpus_TurnSegmentation(t *testing.T) {
	m, video := loadPilot(t)
	part := m.Parts[0]

	src, err := capture.Stream(video, capture.StreamOpts{
		BeginMs: part.Span.BeginMs, EndMs: part.Span.BeginMs + 120000,
		FPS: segFPS, W: segW, H: segH,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	d := stableframe.Detector{
		ROI:       boardROI(part.Calibration.Corners, 1280, 720),
		MaxMotion: segMaxMotion,
		MinFrames: segMinFrames,
	}
	var ws []stableframe.Window
	d.EachWindow(src, func(w stableframe.Window) bool {
		ws = append(ws, w)
		return true
	})

	t.Logf("stable windows in first 2 min: %d", len(ws))
	if len(ws) < 10 || len(ws) > 25 {
		t.Errorf("got %d windows, want 10..25 (≈ plies played + adjustments)", len(ws))
	}
	prevEnd := -1
	for i, w := range ws {
		if w.StartTick < part.Span.BeginMs || w.EndTick > part.Span.BeginMs+120000 {
			t.Errorf("window %d [%d..%d] outside scanned span", i, w.StartTick, w.EndTick)
		}
		if w.StartTick <= prevEnd {
			t.Errorf("window %d starts at %d, not after previous end %d", i, w.StartTick, prevEnd)
		}
		if w.Rep.Img == nil {
			t.Errorf("window %d has no representative frame", i)
		}
		prevEnd = w.EndTick
	}
}
