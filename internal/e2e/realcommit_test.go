package e2e

import (
	"image"
	"image/color"
	"testing"

	"lazybg/internal/capture"
	"lazybg/internal/perceive/clockhit"
	"lazybg/internal/perceive/diceevent"
)

// TestRealCorpus_CommitCues validates the two commit-signal detectors on ten
// real minutes of the pilot, against the truth-aligned turn ticks: most
// turns should have a dice-appearance event and a clock press in their
// neighbourhood, without an implausible flood of false events. One low-res
// stream feeds both detectors.
func TestRealCorpus_CommitCues(t *testing.T) {
	if testing.Short() {
		t.Skip("long: streams real video")
	}
	m, video := loadPilot(t)
	part := m.Parts[0]
	const begin, end = 6000, 606000 // ten minutes of play
	const sw, sh = 320, 180

	src, err := capture.Stream(video, capture.StreamOpts{BeginMs: begin, EndMs: end, FPS: 3, W: sw, H: sh})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	// Board ROI (scaled) for dice events; clock ROI (scaled) for presses.
	minX, minY, maxX, maxY := 1e9, 1e9, 0.0, 0.0
	for _, c := range part.Calibration.Corners {
		minX, maxX = min(minX, c[0]), max(maxX, c[0])
		minY, maxY = min(minY, c[1]), max(maxY, c[1])
	}
	sx, sy := float64(sw)/1280, float64(sh)/720
	clockROI := image.Rect(
		int(float64(part.Priors.ClockROI[0])*sx), int(float64(part.Priors.ClockROI[1])*sy),
		int(float64(part.Priors.ClockROI[2])*sx), int(float64(part.Priors.ClockROI[3])*sy))

	dd := diceevent.New(diceevent.Options{Felt: color.RGBA{168, 166, 162, 255}})
	cd := clockhit.New(clockhit.Options{ROI: clockROI})

	var diceTicks, pressTicks []int
	for {
		f, ok := src.Next()
		if !ok {
			break
		}
		// dice detector sees only the central felt band between the two
		// point rows — where dice land; checkers moving on the points must
		// not fire appearance events
		bandTop := minY + 0.40*(maxY-minY)
		bandBot := minY + 0.60*(maxY-minY)
		sub := f.Img.(*image.RGBA).SubImage(image.Rect(
			int(minX*sx), int(bandTop*sy), int(maxX*sx), int(bandBot*sy))).(*image.RGBA)
		for _, ev := range dd.Feed(capture.Frame{Tick: f.Tick, Img: sub}) {
			if ev.Kind == diceevent.Appeared {
				diceTicks = append(diceTicks, ev.Tick)
			}
		}
		for _, hit := range cd.Feed(f) {
			pressTicks = append(pressTicks, hit.Tick)
		}
	}

	// turns inside the window
	var turns []int
	for _, tn := range m.Turns {
		if tn.TickMs >= begin && tn.TickMs <= end {
			turns = append(turns, tn.TickMs)
		}
	}
	near := func(ticks []int, t0, before, after int) bool {
		for _, tk := range ticks {
			if tk >= t0-before && tk <= t0+after {
				return true
			}
		}
		return false
	}
	diceCovered, pressCovered := 0, 0
	for _, tt := range turns {
		if near(diceTicks, tt, 12000, 12000) { // dice bracket the settled board: this
			// player's roll lands before the tick, the opponent's right after
			diceCovered++
		}
		if near(pressTicks, tt, 4000, 8000) { // press just after the move
			pressCovered++
		}
	}
	t.Logf("10 min: %d truth turns | dice appearances %d (turn coverage %d/%d) | clock presses %d (coverage %d/%d)",
		len(turns), len(diceTicks), diceCovered, len(turns), len(pressTicks), pressCovered, len(turns))
	if len(turns) < 5 {
		t.Skip("not enough aligned turns in the window")
	}
	if diceCovered*2 < len(turns) {
		t.Errorf("dice appearances cover only %d/%d turns", diceCovered, len(turns))
	}
	if pressCovered*2 < len(turns) {
		t.Errorf("clock presses cover only %d/%d turns", pressCovered, len(turns))
	}
	// flood guard: an event every second would be noise, not signal
	if len(diceTicks) > len(turns)*6 || len(pressTicks) > len(turns)*6 {
		t.Errorf("event flood: %d dice / %d presses for %d turns", len(diceTicks), len(pressTicks), len(turns))
	}
}
