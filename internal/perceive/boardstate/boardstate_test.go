package boardstate

import (
	"testing"

	"lazybg/internal/calibrate"
	"lazybg/internal/geom"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boardsynth"
	"lazybg/internal/profile"
)

var (
	colors = boardsynth.DefaultColors()
	prof   = profile.CaptureProfile{CheckerA: colors.A, CheckerB: colors.B}
)

func st(s perceive.Side, n int) boardsynth.Stack { return boardsynth.Stack{Side: s, N: n} }

// standard backgammon opening position, A = CheckerA (white), B = CheckerB.
func openingLayout() map[int]boardsynth.Stack {
	return map[int]boardsynth.Stack{
		24: st(perceive.A, 2), 13: st(perceive.A, 5), 8: st(perceive.A, 3), 6: st(perceive.A, 5),
		1: st(perceive.B, 2), 12: st(perceive.B, 5), 17: st(perceive.B, 3), 19: st(perceive.B, 5),
	}
}

func assertLayout(t *testing.T, ob perceive.ObservedBoard, layout map[int]boardsynth.Stack, minConf float64) {
	t.Helper()
	for p := 1; p <= 24; p++ {
		got := ob.Points[p]
		want, occupied := layout[p]
		if !occupied {
			if got.Count != 0 || got.Side != perceive.None {
				t.Errorf("point %d: got %+v, want empty", p, got)
			}
			continue
		}
		if got.Count != want.N || got.Side != want.Side {
			t.Errorf("point %d: got count=%d side=%v, want count=%d side=%v", p, got.Count, got.Side, want.N, want.Side)
		}
		if got.Confidence < minConf {
			t.Errorf("point %d: confidence %.3f < %.3f", p, got.Confidence, minConf)
		}
	}
}

func TestRead_CanonicalOpeningPosition(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	img := boardsynth.Render(cb, openingLayout(), colors)
	ob := Reader{Profile: prof}.Read(img, cb)
	assertLayout(t, ob, openingLayout(), 0.9)
}

func TestRead_EmptyBoard(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	img := boardsynth.Render(cb, nil, colors)
	ob := Reader{Profile: prof}.Read(img, cb)
	for p := 1; p <= 24; p++ {
		if ob.Points[p].Count != 0 {
			t.Errorf("empty board: point %d read as %+v", p, ob.Points[p])
		}
	}
}

// End-to-end through calibration: warp a canonical board into a perspective
// "camera" frame, rectify it back, and confirm the reader still recovers the
// counts (the calibrated-classical path the MVP relies on).
func TestRead_ThroughPerspectiveRectify(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	layout := map[int]boardsynth.Stack{24: st(perceive.A, 2), 6: st(perceive.A, 5), 1: st(perceive.B, 3), 13: st(perceive.B, 5)}
	canon := boardsynth.Render(cb, layout, colors)

	srcCorners := [4]geom.Pt{geom.P(80, 60), geom.P(900, 40), geom.P(950, 760), geom.P(30, 700)}
	src := boardsynth.WarpToSource(canon, cb, srcCorners, 1000, 820, colors.Background)

	cal, ok := calibrate.New(srcCorners, cb)
	if !ok {
		t.Fatal("calibration failed")
	}
	rect := cal.Rectify(src)
	ob := Reader{Profile: prof}.Read(rect, cb)
	assertLayout(t, ob, layout, 0.6) // some edge blur from resampling → lower conf bar
}
