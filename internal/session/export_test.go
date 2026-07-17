package session

import (
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/corpus"
	"lazybg/internal/matimport"
)

// .mat and the corpus manifest are projections of the .lbg, exportable at
// ANY point — never a separate finalize transform (session-format-spec §1,
// functional-spec §2/§3).
func TestExport_ProjectionsFromLBG(t *testing.T) {
	s, lbgPath := newFileSession(t)
	dir := filepath.Dir(lbgPath)
	if err := s.SaveSetup(Setup{
		Players: [2]string{"Alice", "Bob"},
		Priors:  corpus.Priors{MatchLength: 7, Clock: true},
		Corners: [][2]float64{{1, 1}, {2, 2}, {3, 3}, {4, 4}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterDice(3, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 5000); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterDice(6, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 9000); err != nil {
		t.Fatal(err)
	}

	matPath := filepath.Join(dir, "out.mat")
	if err := s.ExportMat(matPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(matPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := matimport.Parse(string(raw))
	if err != nil {
		t.Fatalf("exported .mat does not re-import: %v", err)
	}
	if m.Length != 7 || m.Players != [2]string{"Alice", "Bob"} {
		t.Fatalf("mat header wrong: length=%d players=%v", m.Length, m.Players)
	}
	if len(m.Games) == 0 || len(m.Games[0].Plies) != 2 {
		t.Fatalf("mat plies wrong: %+v", m.Games)
	}

	manPath := filepath.Join(dir, "out.manifest.json")
	if err := s.ExportManifest(manPath, matPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(manPath)
	if err != nil {
		t.Fatal(err)
	}
	man, err := corpus.Load(data)
	if err != nil {
		t.Fatalf("exported manifest fails the corpus loader: %v", err)
	}
	if len(man.Turns) != 2 || man.Turns[0].TickMs != 5000 || man.Turns[1].TickMs != 9000 {
		t.Fatalf("manifest turns wrong: %+v", man.Turns)
	}
	if len(man.Parts) != 1 || len(man.Parts[0].Calibration.Corners) != 4 || !man.Parts[0].Priors.Clock {
		t.Fatalf("manifest part wrong: %+v", man.Parts)
	}
	if man.Transcript != matPath {
		t.Fatalf("manifest transcript = %q, want %q", man.Transcript, matPath)
	}

	// Mid-session re-export stays consistent: one more turn, both updated.
	if _, err := s.EnterDice(5, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(0, 15000); err != nil {
		t.Fatal(err)
	}
	if err := s.ExportMat(matPath); err != nil {
		t.Fatal(err)
	}
	if err := s.ExportManifest(manPath, matPath); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(matPath)
	m, err = matimport.Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Games[0].Plies) != 3 {
		t.Fatalf("re-exported mat plies = %d, want 3", len(m.Games[0].Plies))
	}
	data, _ = os.ReadFile(manPath)
	man, err = corpus.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(man.Turns) != 3 {
		t.Fatalf("re-exported manifest turns = %d, want 3", len(man.Turns))
	}
}
