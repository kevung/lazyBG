package session

import (
	"fmt"
	"image"

	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/corpus"
	"lazybg/internal/engine"
	"lazybg/internal/perceive"
)

// fakeReader returns a fixed reading regardless of the pixels — the perception
// seam under test is the frame→rectify→read plumbing, not the reader itself.
type fakeReader struct{ ob perceive.ObservedBoard }

func (f fakeReader) Read(image.Image, calibrate.CanonicalBoard) perceive.ObservedBoard {
	return f.ob
}

// calibratedService is an in-memory session with a 4-corner calibration — the
// minimum for the observation path to fire.
func calibratedService() *Service {
	s := New()
	s.doc = &LBG{
		SchemaVersion: LBGSchemaVersion,
		Parts: []LBGPart{{
			File:        "video.mp4",
			Calibration: corpus.Calibration{Corners: [][2]float64{{0, 0}, {63, 0}, {63, 63}, {0, 63}}},
		}},
	}
	return s
}

func fakeGrab(image.Image) frameGrabber {
	return func(int) (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 64, 64)), nil }
}

// With a calibrated session and a reader that sees a specific non-top move on
// the board, entering dice at a tick must fetch the observation and re-weight
// the candidate list — the board-diff cue surfacing the observed move.
func TestObserve_EnterDiceAtReweights(t *testing.T) {
	pos := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	moves, err := engine.LegalMoves(pos)
	if err != nil {
		t.Fatal(err)
	}
	played := moves[3] // not the equity-top move

	s := calibratedService()
	s.reader = fakeReader{ob: obsFromBoard(played.Result)}
	s.grab = fakeGrab(nil)

	res, err := s.EnterDiceAt(3, 1, 4200)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) == 0 || res.Candidates[0].Notation != played.Notation {
		t.Fatalf("top = %+v, want observed %q on top", res.Candidates, played.Notation)
	}
	if got := s.pending.cues; len(got) != 2 || got[1] != "board-diff" {
		t.Fatalf("cues = %v, want board-diff contributed", got)
	}
}

// No calibration → the observation path must not fire and ranking stays pure
// equity (criterion 2: degrade exactly as today).
func TestObserve_UncalibratedDegradesToEquity(t *testing.T) {
	s := New() // no doc, no corners
	s.reader = fakeReader{ob: perceive.ObservedBoard{}}
	s.grab = fakeGrab(nil)

	res, err := s.EnterDiceAt(3, 1, 4200)
	if err != nil {
		t.Fatal(err)
	}
	if res.Candidates[0].Notation != "8/5 6/5" {
		t.Fatalf("top = %q, want equity-best without a usable observation", res.Candidates[0].Notation)
	}
	if got := s.pending.cues; len(got) != 1 || got[0] != "engine-equity" {
		t.Fatalf("cues = %v, want [engine-equity] only", got)
	}
}

// An unreadable frame (decode error) must also degrade cleanly to equity.
func TestObserve_UnreadableFrameDegradesToEquity(t *testing.T) {
	s := calibratedService()
	s.reader = fakeReader{ob: perceive.ObservedBoard{}}
	s.grab = func(int) (image.Image, error) { return nil, fmt.Errorf("bad decode") }

	res, err := s.EnterDiceAt(3, 1, 4200)
	if err != nil {
		t.Fatal(err)
	}
	if res.Candidates[0].Notation != "8/5 6/5" {
		t.Fatalf("top = %q, want equity-best on an unreadable frame", res.Candidates[0].Notation)
	}
	if got := s.pending.cues; len(got) != 1 {
		t.Fatalf("cues = %v, want equity-only on an unreadable frame", got)
	}
}
