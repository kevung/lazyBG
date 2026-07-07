package boardstate

import (
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boardsynth"
	"lazybg/internal/profile"
)

// The shape-first reader recovers a full StandardStart rendered as rimmed discs:
// every occupied point's count and owner must match, and empty points stay empty.
func TestCircleReader_StandardStart(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	col := boardsynth.DefaultColors()
	layout := boardsynth.LayoutFromBoard(bg.StandardStart())
	img := boardsynth.RenderDiscs(cb, layout, col)

	prof := profile.CaptureProfile{CheckerA: col.A, CheckerB: col.B}
	obs := CircleReader{Profile: prof}.Read(img, cb)

	truth := bg.StandardStart()
	for p := 1; p <= 24; p++ {
		tp := truth.Pts[p]
		got := obs.Points[p]
		wantSide := perceive.None
		if tp.N > 0 {
			wantSide = perceive.A
			if tp.Owner == bg.P2 {
				wantSide = perceive.B
			}
		}
		if got.Count != tp.N || got.Side != wantSide {
			t.Errorf("point %d: got %d×%v, want %d×%v", p, got.Count, got.Side, tp.N, wantSide)
		}
	}
}
