package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/corpus"
)

func newFileSession(t *testing.T) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	video := filepath.Join(dir, "v.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lbgPath := filepath.Join(dir, "v.lbg")
	s, _, err := Create(lbgPath, video, "")
	if err != nil {
		t.Fatal(err)
	}
	return s, lbgPath
}

// Setup is the blocking first step (functional-spec §3): a session is not
// ready for turn entry until Session Priors + the 4 board corners exist.
func TestSetup_BlockingUntilDone(t *testing.T) {
	s, _ := newFileSession(t)
	if s.SetupDone() {
		t.Fatal("fresh session cannot be set up")
	}
	setup := Setup{
		Players: [2]string{"Alice", "Bob"},
		Priors: corpus.Priors{
			Clock:       true,
			MatchLength: 7,
			Orientation: "p1-right",
		},
		Corners: [][2]float64{{100, 50}, {900, 50}, {900, 600}, {100, 600}},
	}
	if err := s.SaveSetup(setup); err != nil {
		t.Fatal(err)
	}
	if !s.SetupDone() {
		t.Fatal("setup not marked done")
	}
	if got := s.GetSetup(); got.Players != setup.Players || got.Priors.MatchLength != 7 || len(got.Corners) != 4 {
		t.Fatalf("GetSetup = %+v", got)
	}
}

func TestSetup_CornerCountValidated(t *testing.T) {
	s, _ := newFileSession(t)
	err := s.SaveSetup(Setup{Corners: [][2]float64{{1, 2}, {3, 4}}})
	if err == nil {
		t.Fatal("2-corner calibration accepted")
	}
}

// Setup persists into the .lbg Part (corpus vocabulary) and survives reopen;
// the match length flows into the match shell.
func TestSetup_PersistsInLBGPart(t *testing.T) {
	s, lbgPath := newFileSession(t)
	setup := Setup{
		Players: [2]string{"Alice", "Bob"},
		Priors:  corpus.Priors{MatchLength: 5, CheckerA: "#ffffff", CheckerB: "#222222"},
		Corners: [][2]float64{{1, 2}, {3, 4}, {5, 6}, {7, 8}},
	}
	if err := s.SaveSetup(setup); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(lbgPath)
	var doc LBG
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Parts[0].Priors.MatchLength != 5 || len(doc.Parts[0].Calibration.Corners) != 4 {
		t.Fatalf("part setup not persisted: %+v", doc.Parts[0])
	}
	if doc.Players != [2]string{"Alice", "Bob"} {
		t.Fatalf("players not persisted: %v", doc.Players)
	}

	s2, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !s2.SetupDone() {
		t.Fatal("reopened session lost its setup")
	}
	if s2.GetSetup().Priors.CheckerA != "#ffffff" {
		t.Fatal("reopened priors wrong")
	}
}

// The cube-rule priors round-trip through the .lbg and drive the live cube
// menu after reopening (issue #24).
func TestSetup_CubePriorsPersistAndGateCube(t *testing.T) {
	s, lbgPath := newFileSession(t)
	no := false
	if err := s.SaveSetup(Setup{
		Priors:  corpus.Priors{MatchLength: 7, CubeInPlay: &no},
		Corners: [][2]float64{{1, 2}, {3, 4}, {5, 6}, {7, 8}},
	}); err != nil {
		t.Fatal(err)
	}
	// Cube off in the live session: no actions.
	if got := s.CubeActions(); got != nil {
		t.Fatalf("cube-off actions = %v, want none", got)
	}
	// Reopen: the prior survives and still gates the cube.
	s2, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if s2.GetSetup().Priors.CubeOn() {
		t.Fatal("reopened session lost cube-off prior")
	}
	if got := s2.CubeActions(); got != nil {
		t.Fatalf("reopened cube-off actions = %v, want none", got)
	}
}

// Correcting setup mid-session never touches recorded turns
// (functional-spec §3: plies carry no geometry).
func TestSetup_MidSessionCorrectionKeepsTurns(t *testing.T) {
	s, _ := newFileSession(t)
	if err := s.SaveSetup(Setup{
		Players: [2]string{"A", "B"},
		Priors:  corpus.Priors{MatchLength: 7},
		Corners: [][2]float64{{1, 1}, {2, 2}, {3, 3}, {4, 4}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 100); err != nil {
		t.Fatal(err)
	}
	before := s.Moves()

	if err := s.SaveSetup(Setup{
		Players: [2]string{"A", "B"},
		Priors:  corpus.Priors{MatchLength: 7},
		Corners: [][2]float64{{10, 10}, {20, 20}, {30, 30}, {40, 40}},
	}); err != nil {
		t.Fatal(err)
	}
	after := s.Moves()
	if len(after) != len(before) || after[0] != before[0] {
		t.Fatal("mid-session setup correction altered recorded turns")
	}
	if s.GetSetup().Corners[0] != [2]float64{10, 10} {
		t.Fatal("corrected corners not applied")
	}
}

// The detected lens round-trips through SaveSetup / persistence / GetSetup,
// and a nil lens (the GUI's "Reset to 0") clears it (ADR-0008 §9).
func TestSetup_LensRoundTripAndReset(t *testing.T) {
	s, lbgPath := newFileSession(t)
	setup := Setup{
		Priors:  corpus.Priors{MatchLength: 5, CheckerA: "#ffffff", CheckerB: "#222222"},
		Corners: [][2]float64{{1, 2}, {3, 4}, {5, 6}, {7, 8}},
		Lens:    &corpus.Lens{K1: -0.12, K2: 0.03, CenterX: 640, CenterY: 360, Norm: 640},
	}
	if err := s.SaveSetup(setup); err != nil {
		t.Fatal(err)
	}
	s2, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	got := s2.GetSetup()
	if got.Lens == nil || got.Lens.K1 != -0.12 || got.Lens.K2 != 0.03 {
		t.Fatalf("lens did not round-trip: %+v", got.Lens)
	}

	setup.Lens = nil // the reset button
	if err := s2.SaveSetup(setup); err != nil {
		t.Fatal(err)
	}
	if l := s2.GetSetup().Lens; l != nil {
		t.Fatalf("reset lens must persist as nil, got %+v", l)
	}
}
