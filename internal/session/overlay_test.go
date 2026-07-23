package session

import (
	"image"
	"image/png"
	"os"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/corpus"
)

// realOpeningService is a session calibrated on the pilot's settled opening
// frame (the same one-time Board Calibration as
// boardstate.TestCircleReader_RealOpeningFrame), grabbing that committed
// fixture at every tick.
func realOpeningService(t *testing.T) *Service {
	t.Helper()
	f, err := os.Open("../../testdata/perception/hsbtMars2025-r1-opening.png")
	if err != nil {
		t.Skipf("fixture not present: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	s := New()
	s.doc = &LBG{
		SchemaVersion: LBGSchemaVersion,
		Parts: []LBGPart{{
			File: "video.mp4",
			Calibration: corpus.Calibration{
				Corners:   [][2]float64{{203, 54}, {825, 46}, {818, 614}, {200, 628}},
				Canonical: &corpus.Canonical{MarginX: 16, MarginY: 18, PointW: 58, QuadH: 300, BarGap: 60, OffW: 24},
			},
		}},
	}
	s.grab = func(int) (image.Image, error) { return img, nil }
	return s
}

// The disc layer must find most of the 30 checkers on a real opening frame, and
// every disc it reports must sit inside a point's stack region.
//
// The board-wide pass this replaces could not: checker.DetectWith thresholds the
// accumulator at PeakFrac × the image-wide maximum, so one high-contrast disc
// silences every lower-contrast one — 21 discs for 30 checkers, 4 of them
// nowhere near a point. Detecting per point region makes the threshold local to
// each point, which is the contrast-relative behaviour the detector was designed
// for (and what CircleReader already does).
func TestOverlay_DiscsPerPointRegion(t *testing.T) {
	s := realOpeningService(t)
	ov := s.Overlay(0)
	if !ov.OK {
		t.Fatal("overlay not OK on a calibrated session with a decodable frame")
	}

	_, cb, ok := buildCalibration(s.doc.Parts[0].Calibration)
	if !ok {
		t.Fatal("degenerate calibration")
	}
	stray := 0
	for _, c := range ov.Circles {
		in := false
		for p := 1; p <= 24; p++ {
			r, _ := cb.PointRegion(p)
			if image.Pt(c.X, c.Y).In(r) {
				in = true
				break
			}
		}
		if !in {
			stray++
		}
	}
	t.Logf("discs: %d for 30 checkers, %d outside any point region", len(ov.Circles), stray)
	if len(ov.Circles) < 24 {
		t.Errorf("discs = %d, want >= 24 of 30 checkers", len(ov.Circles))
	}
	if stray != 0 {
		t.Errorf("%d discs outside any point region, want 0", stray)
	}
}

// The disc layer is evidence ABOUT points, so it must not invent checkers on an
// empty one: a disc on a bare point would send the user hunting a calibration
// bug that isn't there. Under-counting a tall stack is the classical detector's
// documented ~90% ceiling and is not asserted here — the layer is a debugging
// aid, not the reader.
func TestOverlay_DiscsOnlyOnOccupiedPoints(t *testing.T) {
	s := realOpeningService(t)
	ov := s.Overlay(0)
	if !ov.OK {
		t.Fatal("overlay not OK")
	}
	_, cb, _ := buildCalibration(s.doc.Parts[0].Calibration)

	perPoint := map[int]int{}
	for _, c := range ov.Circles {
		for p := 1; p <= 24; p++ {
			r, _ := cb.PointRegion(p)
			if image.Pt(c.X, c.Y).In(r) {
				perPoint[p]++
				break
			}
		}
	}
	truth := bg.StandardStart()
	occupied := 0
	for p := 1; p <= 24; p++ {
		n, want := perPoint[p], truth.Pts[p].N
		if want > 0 {
			if n > 0 {
				occupied++
			}
			if n > want {
				t.Errorf("point %d: %d discs over %d checkers", p, n, want)
			}
			continue
		}
		if n > 0 {
			t.Errorf("point %d is empty but carries %d discs", p, n)
		}
	}
	t.Logf("discs cover %d of the opening's 8 occupied points", occupied)
	if occupied < 7 {
		t.Errorf("discs cover %d occupied points, want >= 7 of 8", occupied)
	}
}
