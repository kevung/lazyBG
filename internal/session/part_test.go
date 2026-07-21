package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func addSecondVideo(t *testing.T, lbgPath string) string {
	t.Helper()
	v2 := filepath.Join(filepath.Dir(lbgPath), "v2.mp4")
	if err := os.WriteFile(v2, []byte("second-video"), 0o644); err != nil {
		t.Fatal(err)
	}
	return v2
}

// A session can append Part 2+, keep recording turns against it, and restore
// the active Part on resume (issue #26, functional-spec §6).
func TestMultiPart_AddRecordSwitchResume(t *testing.T) {
	s, lbgPath := newFileSession(t)
	mustEnter(t, s, 3, 1, 100) // turn 0 in Part 0

	v2 := addSecondVideo(t, lbgPath)
	idx, err := s.AddPart(v2, "https://example/v2")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 1 || s.ActivePart() != 1 {
		t.Fatalf("added part idx=%d active=%d, want 1/1", idx, s.ActivePart())
	}
	if !strings.HasSuffix(s.VideoPath(), "v2.mp4") {
		t.Fatalf("VideoPath after add = %q, want the new Part's file", s.VideoPath())
	}
	mustEnter(t, s, 6, 2, 200) // turn 1 in Part 1

	// The .lbg records the inheritable new Part and the per-turn Part index.
	raw, _ := os.ReadFile(lbgPath)
	var doc LBG
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Parts) != 2 {
		t.Fatalf("doc parts = %d, want 2", len(doc.Parts))
	}
	if !doc.Parts[1].Priors.Inherit || !doc.Parts[1].Calibration.Inherit {
		t.Fatalf("new Part not inheritable: %+v", doc.Parts[1])
	}
	if doc.Turns[0].Part != 0 || doc.Turns[1].Part != 1 {
		t.Fatalf("turn Parts = %d,%d, want 0,1", doc.Turns[0].Part, doc.Turns[1].Part)
	}
	if doc.LastPart != 1 {
		t.Fatalf("doc.LastPart = %d, want 1", doc.LastPart)
	}

	// Resume restores the active Part.
	s2, _, err := Open(lbgPath)
	if err != nil {
		t.Fatal(err)
	}
	if s2.ActivePart() != 1 {
		t.Fatalf("reopened active part = %d, want 1", s2.ActivePart())
	}
	// Switching back to Part 0 is allowed.
	if err := s2.SetActivePart(0); err != nil {
		t.Fatal(err)
	}
	if s2.ActivePart() != 0 || !strings.HasSuffix(s2.VideoPath(), "v.mp4") {
		t.Fatalf("after SetActivePart(0): active=%d video=%q", s2.ActivePart(), s2.VideoPath())
	}
}

// The manifest projection carries every Part and the per-turn Part index
// (issue #26).
func TestMultiPart_ManifestProjectsAllParts(t *testing.T) {
	s, lbgPath := newFileSession(t)
	mustEnter(t, s, 3, 1, 100)
	v2 := addSecondVideo(t, lbgPath)
	if _, err := s.AddPart(v2, ""); err != nil {
		t.Fatal(err)
	}
	mustEnter(t, s, 6, 2, 200)

	manPath := filepath.Join(filepath.Dir(lbgPath), "m.json")
	if err := s.ExportManifest(manPath, "m.mat"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(manPath)
	if !strings.Contains(string(raw), "v2.mp4") {
		t.Fatal("manifest missing the second Part's file")
	}
	// Two parts, and a turn tagged Part 1.
	if strings.Count(string(raw), "\"file\"") < 2 {
		t.Fatal("manifest should carry two Parts")
	}
}

// SetActivePart rejects an out-of-range index.
func TestMultiPart_SetActivePartRange(t *testing.T) {
	s, _ := newFileSession(t)
	if err := s.SetActivePart(5); err == nil {
		t.Fatal("out-of-range SetActivePart should fail")
	}
}
