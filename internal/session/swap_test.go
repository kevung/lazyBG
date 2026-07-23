package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/corpus"
)

// writeDoc lays down a .lbg next to a fake video and returns both paths.
func writeDoc(t *testing.T, doc LBG) (lbgPath string, video string) {
	t.Helper()
	video = tempVideo(t, "fake-video-bytes-0123456789")
	fp, err := Fingerprint(video)
	if err != nil {
		t.Fatal(err)
	}
	for i := range doc.Parts {
		doc.Parts[i].File = video
		doc.Parts[i].Fingerprint = fp
	}
	lbgPath = filepath.Join(filepath.Dir(video), "match.lbg")
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lbgPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return lbgPath, video
}

func legacyTopDoc() LBG {
	return LBG{
		SchemaVersion: LBGSchemaVersion,
		Length:        7,
		Players:       [2]string{"Top Player", "Bottom Player"},
		Parts: []LBGPart{{Priors: corpus.Priors{
			MatchLength: 7,
			CheckerA:    "#1a1614", // the top player's checkers
			CheckerB:    "#e7e0d5",
			Orientation: "p1-home-top-left",
		}}},
		Turns: []LBGTurn{
			{Game: 1, Player: 0, Dice: [2]int{3, 1}, Notation: "8/5 6/5", TickMs: 5000, ChosenIndex: 0},
			{Game: 1, Player: 1, Dice: [2]int{6, 2}, Notation: "24/18 13/11", TickMs: 9000, ChosenIndex: 0},
		},
	}
}

// A document written under the pre-ADR-0009 model (Player 1 on the top row)
// must open with the two players exchanged: under the current rule Player 1 is
// the bottom player, so the person who was "Player 1" there is Player 2 here.
func TestOpen_MigratesLegacyTopOrientation(t *testing.T) {
	lbgPath, _ := writeDoc(t, legacyTopDoc())
	s, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.doc.Players; got != [2]string{"Bottom Player", "Top Player"} {
		t.Errorf("players = %v, want the bottom player first", got)
	}
	p := s.doc.Parts[0].Priors
	if p.CheckerA != "#e7e0d5" || p.CheckerB != "#1a1614" {
		t.Errorf("checker colours = %s/%s, want them exchanged", p.CheckerA, p.CheckerB)
	}
	// The home boards do not move: top-LEFT keeps the homes in the left half.
	if p.Orientation != "p1-home-left" {
		t.Errorf("orientation = %q, want p1-home-left", p.Orientation)
	}
	if o, _ := bg.ParseOrientation(p.Orientation); o != bg.P1HomeLeft {
		t.Errorf("orientation does not parse back to P1HomeLeft")
	}
	if s.doc.Turns[0].Player != 1 || s.doc.Turns[1].Player != 0 {
		t.Errorf("turn players = %d,%d, want 1,0", s.doc.Turns[0].Player, s.doc.Turns[1].Player)
	}
	// The replayed match must agree with the rewritten document.
	if got := s.match.Games[0].Plies[0].Player; got != bg.P2 {
		t.Errorf("replayed ply 1 player = %v, want P2", got)
	}
}

// A document already under the current rule is left strictly alone.
func TestOpen_LeavesCurrentDocumentsAlone(t *testing.T) {
	doc := legacyTopDoc()
	doc.Parts[0].Priors.Orientation = "p1-bottom" // the 23 corpus manifests
	lbgPath, _ := writeDoc(t, doc)
	s, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.doc.Players; got != [2]string{"Top Player", "Bottom Player"} {
		t.Errorf("players = %v, want them untouched", got)
	}
	if s.doc.Turns[0].Player != 0 {
		t.Error("turn player was flipped on a document that needed no migration")
	}
	if s.doc.Parts[0].Priors.Orientation != "p1-bottom" {
		t.Error("orientation string rewritten on a document that needed no migration")
	}
}

// Swapping twice returns the session to its exact starting state — the property
// that makes the setup panel's button safe to press at any time.
func TestSwapPlayers_IsAnInvolution(t *testing.T) {
	doc := legacyTopDoc()
	doc.Parts[0].Priors.Orientation = "p1-home-right"
	doc.Results = []LBGResult{{Game: 1, Winner: 1, Points: 2}}
	lbgPath, _ := writeDoc(t, doc)
	s, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(s.doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SwapPlayers(); err != nil {
		t.Fatal(err)
	}
	if s.doc.Results[0].Winner != 0 {
		t.Errorf("winner = %d, want 0 — the score follows the players", s.doc.Results[0].Winner)
	}
	if s.doc.Parts[0].Priors.Orientation != "p1-home-right" {
		t.Error("swapping the players moved the board; it must only rename")
	}
	if err := s.SwapPlayers(); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(s.doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("swap is not an involution:\n before %s\n after  %s", before, after)
	}
}

// The setup form exchanges the names and the colours it holds, then asks
// SaveSetup to move the recorded play with them. Without SwapPlies the plies
// would stay with the wrong player — the silent half of the bug.
func TestSaveSetup_SwapPliesMovesTheRecordedPlay(t *testing.T) {
	doc := legacyTopDoc()
	doc.Parts[0].Priors.Orientation = "p1-home-right"
	doc.Results = []LBGResult{{Game: 1, Winner: 0, Points: 1}}
	lbgPath, _ := writeDoc(t, doc)
	s, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	setup := s.GetSetup()
	// What the form does when the button is pressed.
	setup.Players[0], setup.Players[1] = setup.Players[1], setup.Players[0]
	setup.Priors.CheckerA, setup.Priors.CheckerB = setup.Priors.CheckerB, setup.Priors.CheckerA
	setup.SwapPlies = true
	setup.Corners = [][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	if err := s.SaveSetup(setup); err != nil {
		t.Fatal(err)
	}
	if s.doc.Turns[0].Player != 1 || s.doc.Results[0].Winner != 1 {
		t.Errorf("plies did not follow the players: turn0=%d winner=%d",
			s.doc.Turns[0].Player, s.doc.Results[0].Winner)
	}
	if got := s.doc.Players; got != [2]string{"Bottom Player", "Top Player"} {
		t.Errorf("players = %v, want the form's exchange persisted", got)
	}
	// Saving again WITHOUT the flag must leave the play alone.
	setup2 := s.GetSetup()
	setup2.Corners = setup.Corners
	if err := s.SaveSetup(setup2); err != nil {
		t.Fatal(err)
	}
	if s.doc.Turns[0].Player != 1 {
		t.Error("a plain save moved the recorded play")
	}
}

// The swap must survive a round-trip through the file, since it is applied to a
// session the user goes on transcribing.
func TestSwapPlayers_Persists(t *testing.T) {
	doc := legacyTopDoc()
	doc.Parts[0].Priors.Orientation = "p1-home-right"
	lbgPath, _ := writeDoc(t, doc)
	s, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SwapPlayers(); err != nil {
		t.Fatal(err)
	}
	s2, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.doc.Players; got != [2]string{"Bottom Player", "Top Player"} {
		t.Errorf("reopened players = %v, want the swap persisted", got)
	}
	if s2.doc.Turns[0].Player != 1 {
		t.Error("reopened turn player lost the swap")
	}
}
