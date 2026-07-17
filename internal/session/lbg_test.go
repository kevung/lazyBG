package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func tempVideo(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "match.mp4")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Create → confirm turns → every confirm autosaves; Open resumes the exact
// state: moves, board chain, alternation, last video position
// (session-format-spec §1: resume/edit an in-progress transcription).
func TestLBG_CreateConfirmReopen(t *testing.T) {
	video := tempVideo(t, "fake-video-bytes-0123456789")
	lbgPath := filepath.Join(filepath.Dir(video), "match.lbg")

	s, warn, err := Create(lbgPath, video, "https://youtube.com/watch?v=x")
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("unexpected warning on create: %q", warn)
	}
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 5000); err != nil {
		t.Fatal(err)
	}

	// Autosave: the file already holds the first turn before any explicit save.
	raw, err := os.ReadFile(lbgPath)
	if err != nil {
		t.Fatalf("no autosaved .lbg after first confirm: %v", err)
	}
	var doc LBG
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Turns) != 1 {
		t.Fatalf("autosaved turns = %d, want 1", len(doc.Turns))
	}
	if doc.SchemaVersion != LBGSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", doc.SchemaVersion, LBGSchemaVersion)
	}

	if _, err := s.EnterDice(6, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(1, 9000); err != nil {
		t.Fatal(err)
	}
	s.SetVideoPos(12500)

	// Reopen: full state restored.
	s2, warn, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("unexpected warning on clean reopen: %q", warn)
	}
	moves := s2.Moves()
	if len(moves) != 2 {
		t.Fatalf("reopened moves = %d, want 2", len(moves))
	}
	if moves[0].Dice != "31" || moves[0].TickMs != 5000 || moves[1].Dice != "62" {
		t.Fatalf("reopened moves wrong: %+v", moves)
	}
	if s2.OnRoll() != 0 {
		t.Fatalf("reopened onRoll = %d, want 0 (two plies played)", s2.OnRoll())
	}
	if s2.LastVideoPos() != 12500 {
		t.Fatalf("reopened video pos = %d, want 12500", s2.LastVideoPos())
	}
	if s2.Board() != s.Board() {
		t.Fatal("reopened board differs from live board")
	}
}

// Candidate traceability (session-format-spec §3): the .lbg records the
// candidate list as shown, the chosen index, and the contributing cues.
func TestLBG_CandidateTraceability(t *testing.T) {
	video := tempVideo(t, "vid")
	lbgPath := filepath.Join(filepath.Dir(video), "m.lbg")
	s, _, err := Create(lbgPath, video, "")
	if err != nil {
		t.Fatal(err)
	}
	cands, err := s.EnterDice(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(2, 100); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(lbgPath)
	var doc LBG
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	turn := doc.Turns[0]
	if len(turn.Candidates) != len(cands) {
		t.Fatalf("persisted candidates = %d, want %d (as shown)", len(turn.Candidates), len(cands))
	}
	if turn.ChosenIndex != 2 {
		t.Fatalf("chosenIndex = %d, want 2", turn.ChosenIndex)
	}
	if turn.Candidates[0].Notation != cands[0].Notation {
		t.Fatal("persisted candidate list differs from the list shown")
	}
	if len(turn.Cues) == 0 {
		t.Fatal("no contributing cues recorded")
	}
}

// A substituted/corrupted video is detected on reopen (session-format-spec §1:
// fingerprint check surfaces a warning, never silently proceeds).
func TestLBG_FingerprintMismatchWarns(t *testing.T) {
	video := tempVideo(t, "original-content")
	lbgPath := filepath.Join(filepath.Dir(video), "m.lbg")
	if _, _, err := Create(lbgPath, video, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(video, []byte("REPLACED-content-of-different-length"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, warn, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if warn == "" {
		t.Fatal("no warning for a substituted video file")
	}
}

// A missing video file also warns (moved captures are common), but the
// session still opens — the transcription data is intact.
func TestLBG_MissingVideoWarns(t *testing.T) {
	video := tempVideo(t, "content")
	lbgPath := filepath.Join(filepath.Dir(video), "m.lbg")
	if _, _, err := Create(lbgPath, video, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(video); err != nil {
		t.Fatal(err)
	}
	s, warn, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if warn == "" {
		t.Fatal("no warning for a missing video file")
	}
	if s == nil {
		t.Fatal("session must still open — the transcription data is intact")
	}
}

// The video's canonical URL is carried in the file (portability for shared
// sessions — session-format-spec §1).
func TestLBG_CarriesVideoURL(t *testing.T) {
	video := tempVideo(t, "v")
	lbgPath := filepath.Join(filepath.Dir(video), "m.lbg")
	if _, _, err := Create(lbgPath, video, "https://youtube.com/watch?v=abc"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(lbgPath)
	var doc LBG
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Parts) != 1 || doc.Parts[0].URL != "https://youtube.com/watch?v=abc" {
		t.Fatalf("video URL not carried: %+v", doc.Parts)
	}
}
