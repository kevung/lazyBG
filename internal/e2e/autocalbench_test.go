package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"lazybg/internal/autocal/bench"
	"lazybg/internal/corpus"
)

const autocalBaselinePath = "testdata/autocal_baseline.json"

// TestRealCorpus_AutocalBench is the ADR-0008 multi-capture ratchet: it runs
// the automatic handle detection on every manifest whose video is present
// locally and compares against the committed baseline. Regenerate the
// baseline (after reviewing WHY every number moved) with:
//
//	LAZYBG_AUTOCAL_BASELINE=write go test ./internal/e2e -run AutocalBench
func TestRealCorpus_AutocalBench(t *testing.T) {
	if testing.Short() {
		t.Skip("long: decodes real corpus videos")
	}
	// Worktrees carry the committed manifests but not the machine-local
	// videos; LAZYBG_CORPUS_ROOT points the bench at the checkout that has
	// them (e.g. the main working tree).
	root := repoRoot
	if v := os.Getenv("LAZYBG_CORPUS_ROOT"); v != "" {
		root = v
	}
	manifests, err := filepath.Glob(filepath.Join(root, "corpus/manifest/*.json"))
	if err != nil || len(manifests) == 0 {
		t.Skipf("no corpus manifests: %v", err)
	}
	sort.Strings(manifests)

	var report bench.Report
	for _, path := range manifests {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		m, err := corpus.Load(data)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		if len(m.Parts) == 0 {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, m.Parts[0].File)); err != nil {
			t.Logf("skip %s: video absent locally", m.ID)
			continue
		}
		res := bench.RunCapture(root, m)
		report.Results = append(report.Results, res)
		t.Logf("%s", formatResult(res))
	}
	if len(report.Results) == 0 {
		t.Skip("no corpus videos present locally")
	}
	t.Logf("mean auto score: %.2f/24 over %d captures", report.MeanScore(), len(report.Results))

	if os.Getenv("LAZYBG_AUTOCAL_BASELINE") == "write" {
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(autocalBaselinePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(autocalBaselinePath, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("baseline written to %s — review and commit it", autocalBaselinePath)
		return
	}

	data, err := os.ReadFile(autocalBaselinePath)
	if err != nil {
		t.Fatalf("no committed baseline (%v) — generate one with LAZYBG_AUTOCAL_BASELINE=write", err)
	}
	var baseline bench.Report
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}
	for _, v := range bench.Compare(baseline, report, 1) {
		t.Errorf("ratchet: %s", v)
	}
	t.Logf("baseline mean was %.2f/24 over %d captures", baseline.MeanScore(), len(baseline.Results))
}

func formatResult(r bench.CaptureResult) string {
	if r.Err != "" {
		return fmt.Sprintf("%-45s ERR %s", r.ID, r.Err)
	}
	s := fmt.Sprintf("%-45s tick %7dms  manual %2d/24  auto %2d/24  corners %.0fpx",
		r.ID, r.OpeningTickMs, r.ScoreManual, r.ScoreAuto, maxf(r.CornerDistPx))
	if r.BarDistPx != nil {
		s += fmt.Sprintf("  bar %.0fpx", maxf(r.BarDistPx))
	}
	if r.K1 != 0 || r.K2 != 0 {
		s += fmt.Sprintf("  lens k1=%.3f k2=%.3f", r.K1, r.K2)
	}
	return s
}

func maxf(v []float64) float64 {
	m := 0.0
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}
