package e2e

import (
	"image"
	"image/color"
	"os"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/engine"
	"lazybg/internal/geom"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/perceive/boardstate"
	"lazybg/internal/perceive/boardsynth"
	"lazybg/internal/perceive/stableframe"
	"lazybg/internal/profile"
)

func TestMain(m *testing.M) {
	if err := engine.Init(os.DirFS("../..")); err != nil {
		panic("engine init: " + err.Error())
	}
	os.Exit(m.Run())
}

// TestPerceptionToMove wires the entire video front-half. A strong player plays
// the engine's best 3-1 (8/5 6/5); we render the resulting board, warp it into a
// slanted camera frame, hold it still with a hand-motion blip in the middle,
// then run: stableframe (find the settled window) → calibrate (rectify the
// settled frame) → boardstate (read checker counts) → boarddiff + engine
// (recover the move). The pipeline must recover exactly 8/5 6/5.
func TestPerceptionToMove(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	colors := boardsynth.DefaultColors()
	prof := profile.CaptureProfile{CheckerA: colors.A, CheckerB: colors.B}

	pre := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	moves, err := engine.LegalMoves(pre)
	if err != nil || len(moves) == 0 {
		t.Fatalf("engine: %v", err)
	}
	played := moves[0] // 8/5 6/5

	postCanon := boardsynth.Render(cb, boardsynth.LayoutFromBoard(played.Result), colors)
	srcCorners := [4]geom.Pt{geom.P(80, 60), geom.P(900, 40), geom.P(950, 760), geom.P(30, 700)}
	const camW, camH = 1000, 820
	cam := boardsynth.WarpToSource(postCanon, cb, srcCorners, camW, camH, colors.Background)

	frames := stillMotionStill(cam)

	roi := boundingBox(srcCorners)
	windows := stableframe.Detector{ROI: roi, MaxMotion: 3, MinFrames: 3}.
		Windows(capture.NewSliceSource(frames))
	if len(windows) < 1 {
		t.Fatal("no stable window found")
	}
	rep := windows[len(windows)-1].Rep // settled post-move frame

	cal, ok := calibrate.New(srcCorners, cb)
	if !ok {
		t.Fatal("calibration failed")
	}
	obs := boardstate.Reader{Profile: prof}.Read(cal.Rectify(rep.Img), cb)

	scored, err := boarddiff.Detect(pre, obs)
	if err != nil {
		t.Fatal(err)
	}
	if scored[0].Move.Notation != "8/5 6/5" {
		t.Fatalf("perception pipeline recovered %q, want %q", scored[0].Move.Notation, "8/5 6/5")
	}
	if scored[0].Match < 0.9 {
		t.Errorf("match %.3f too low for a clean settled frame", scored[0].Match)
	}
}

// stillMotionStill returns 5 still camera frames, 2 frames with a hand crossing
// the board, then 5 more still frames (ticks 100ms apart).
func stillMotionStill(cam *image.RGBA) []capture.Frame {
	var frames []capture.Frame
	tick := 0
	add := func(img image.Image) {
		frames = append(frames, capture.Frame{Tick: tick, Img: img})
		tick += 100
	}
	for i := 0; i < 5; i++ {
		add(cam)
	}
	add(withHand(cam, 300))
	add(withHand(cam, 420))
	for i := 0; i < 5; i++ {
		add(cam)
	}
	return frames
}

// withHand copies cam and paints a dark rectangle over the board (a hand).
func withHand(cam *image.RGBA, x int) *image.RGBA {
	c := image.NewRGBA(cam.Bounds())
	copy(c.Pix, cam.Pix)
	boardsynth.Fill(c, image.Rect(x, 200, x+140, 520), color.RGBA{30, 20, 10, 255})
	return c
}

func boundingBox(corners [4]geom.Pt) image.Rectangle {
	minX, minY := corners[0].X, corners[0].Y
	maxX, maxY := minX, minY
	for _, p := range corners {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return image.Rect(int(minX), int(minY), int(maxX), int(maxY))
}
