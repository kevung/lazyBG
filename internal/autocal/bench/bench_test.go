package bench

import (
	"strings"
	"testing"

	"lazybg/internal/corpus"
)

func res(id string, scoreAuto int, err string) CaptureResult {
	return CaptureResult{ID: id, ScoreAuto: scoreAuto, Err: err}
}

func TestCompare_NoRegression(t *testing.T) {
	base := Report{Results: []CaptureResult{res("a", 20, ""), res("b", 15, "")}}
	cur := Report{Results: []CaptureResult{res("a", 20, ""), res("b", 14, "")}}
	if v := Compare(base, cur, 1); len(v) != 0 {
		t.Fatalf("within tolerance should pass, got %v", v)
	}
}

func TestCompare_ScoreRegressionFlagged(t *testing.T) {
	base := Report{Results: []CaptureResult{res("a", 20, "")}}
	cur := Report{Results: []CaptureResult{res("a", 18, "")}}
	v := Compare(base, cur, 1)
	if len(v) != 1 || !strings.Contains(v[0], "a") {
		t.Fatalf("2-point drop must be flagged, got %v", v)
	}
}

func TestCompare_NewFailureFlagged(t *testing.T) {
	base := Report{Results: []CaptureResult{res("a", 20, "")}}
	cur := Report{Results: []CaptureResult{res("a", 0, "detect failed")}}
	if v := Compare(base, cur, 1); len(v) != 1 {
		t.Fatalf("a capture that worked and now errors must be flagged, got %v", v)
	}
}

func TestCompare_BaselineFailureMayStayFailed(t *testing.T) {
	base := Report{Results: []CaptureResult{res("a", 0, "detect failed")}}
	cur := Report{Results: []CaptureResult{res("a", 0, "detect failed")}}
	if v := Compare(base, cur, 1); len(v) != 0 {
		t.Fatalf("an already-failing capture is not a regression, got %v", v)
	}
}

func TestCompare_AbsentAndNewCapturesIgnored(t *testing.T) {
	// Baseline capture not re-run (video absent locally) and a brand-new
	// capture (not yet in the baseline) are both informational, not gates.
	base := Report{Results: []CaptureResult{res("gone", 22, "")}}
	cur := Report{Results: []CaptureResult{res("new", 3, "")}}
	if v := Compare(base, cur, 1); len(v) != 0 {
		t.Fatalf("absent/new captures must not gate, got %v", v)
	}
}

func TestMeanScore(t *testing.T) {
	r := Report{Results: []CaptureResult{res("a", 20, ""), res("b", 10, ""), res("c", 0, "failed")}}
	// Errored captures count as 0 — a detector that starts failing must not
	// raise the mean by dropping hard captures from the denominator.
	if got := r.MeanScore(); got != 10 {
		t.Fatalf("mean = %v, want 10 (errors count as zero over all 3)", got)
	}
}

func TestOpeningWindow_AnchoredOnFirstTurn(t *testing.T) {
	m := corpus.Manifest{
		Parts: []corpus.Part{{Span: corpus.Span{BeginMs: 0, EndMs: 100000}}},
		Turns: []corpus.Turn{{Index: 2, Part: 0, TickMs: 90000}, {Index: 1, Part: 0, TickMs: 60000}},
	}
	begin, end := OpeningWindow(m, 0)
	if begin != 30000 || end != 63000 {
		t.Fatalf("window = [%d,%d], want [30000,63000] (t0-30s .. t0+3s)", begin, end)
	}
}

func TestOpeningWindow_ClampedToSpanBegin(t *testing.T) {
	m := corpus.Manifest{
		Parts: []corpus.Part{{Span: corpus.Span{BeginMs: 5000}}},
		Turns: []corpus.Turn{{Index: 1, Part: 0, TickMs: 12000}},
	}
	begin, end := OpeningWindow(m, 0)
	if begin != 5000 || end != 15000 {
		t.Fatalf("window = [%d,%d], want [5000,15000]", begin, end)
	}
}

func TestOpeningWindow_NoTurnsFallsBackToScanDefault(t *testing.T) {
	m := corpus.Manifest{Parts: []corpus.Part{{Span: corpus.Span{BeginMs: 2000}}}}
	begin, end := OpeningWindow(m, 0)
	if begin != 2000 || end != 2000+DefaultScanMs {
		t.Fatalf("window = [%d,%d], want [2000,%d]", begin, end, 2000+DefaultScanMs)
	}
}
