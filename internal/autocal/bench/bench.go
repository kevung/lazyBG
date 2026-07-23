// Package bench measures auto-calibration against the corpus manifests — the
// multi-capture benchmark of ADR-0008. Every locally-present capture gets the
// detector run on its opening frame; the manual (hand-placed) calibration is
// the reference: pixel distances to the manual handles are a diagnostic, the
// opening read score with auto vs manual corners is the functional judge.
// A committed baseline report turns the bench into a ratchet (Compare): no
// capture may regress beyond noise, so single-capture tuning can no longer
// silently break the rest of the corpus.
package bench

import (
	"fmt"
	"math"
	"path/filepath"

	"lazybg/internal/autocal"
	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/geom"
	"lazybg/internal/profile"
)

// DefaultScanMs bounds the opening search when a manifest has no aligned
// turns to anchor the window on (mirrors autocal's default scan depth).
const DefaultScanMs = 420000

// CaptureResult is one capture's bench measurement.
type CaptureResult struct {
	ID  string `json:"id"`
	Err string `json:"err,omitempty"`
	// OpeningTickMs is the settled-opening frame found with the MANUAL
	// calibration — the anchor every score below is read at.
	OpeningTickMs int `json:"openingTickMs,omitempty"`
	ScoreManual   int `json:"scoreManual,omitempty"`
	ScoreAuto     int `json:"scoreAuto,omitempty"`
	// CornerDistPx / BarDistPx are the auto-vs-manual handle distances
	// (TL,TR,BR,BL order); BarDistPx only when the manifest declares bar
	// edges (calibration v2).
	CornerDistPx []float64 `json:"cornerDistPx,omitempty"`
	BarDistPx    []float64 `json:"barDistPx,omitempty"`
	// K1/K2 are the fit's admitted lens coefficients (0 = pinhole) — the
	// diagnostic that shows whether real wide-angle captures trigger the
	// lens estimator.
	K1 float64 `json:"k1,omitempty"`
	K2 float64 `json:"k2,omitempty"`
}

// Report is the whole bench run; the committed baseline is one of these.
type Report struct {
	Results []CaptureResult `json:"results"`
}

// MeanScore averages ScoreAuto over ALL results — errored captures count as
// zero so a detector cannot raise the mean by failing on hard captures.
func (r Report) MeanScore() float64 {
	if len(r.Results) == 0 {
		return 0
	}
	var s float64
	for _, c := range r.Results {
		s += float64(c.ScoreAuto)
	}
	return s / float64(len(r.Results))
}

// Compare ratchets current against baseline: a capture present in both may
// not lose more than tol read points, and a capture that used to succeed may
// not start erroring. Captures missing from either side are informational
// (video absent locally / newly added) and never gate. Returns human-readable
// violations, empty when the ratchet holds.
func Compare(baseline, current Report, tol int) []string {
	base := map[string]CaptureResult{}
	for _, c := range baseline.Results {
		base[c.ID] = c
	}
	var violations []string
	for _, cur := range current.Results {
		b, ok := base[cur.ID]
		if !ok || b.Err != "" {
			continue
		}
		if cur.Err != "" {
			violations = append(violations, fmt.Sprintf("%s: succeeded in baseline (score %d) but now errors: %s", cur.ID, b.ScoreAuto, cur.Err))
			continue
		}
		if cur.ScoreAuto < b.ScoreAuto-tol {
			violations = append(violations, fmt.Sprintf("%s: auto score %d dropped below baseline %d (tolerance %d)", cur.ID, cur.ScoreAuto, b.ScoreAuto, tol))
		}
	}
	return violations
}

// OpeningWindow returns the [begin,end) ms window in which the settled
// opening of the given part must lie. With aligned turns, the standard start
// position necessarily persists up to the part's first commit tick, so a
// short window ending just after it suffices; without turns, fall back to a
// deep scan from the span begin.
func OpeningWindow(m corpus.Manifest, part int) (beginMs, endMs int) {
	span := m.Parts[part].Span
	first := -1
	for _, t := range m.Turns {
		if t.Part == part && (first < 0 || t.TickMs < first) {
			first = t.TickMs
		}
	}
	if first < 0 {
		return span.BeginMs, span.BeginMs + DefaultScanMs
	}
	begin := first - 30000
	if begin < span.BeginMs {
		begin = span.BeginMs
	}
	return begin, first + 3000
}

// RunCapture benches part 0 of one manifest: find the opening with the
// manual calibration, run the automatic handle detection on that frame, and
// score both calibrations against the standard start.
func RunCapture(root string, m corpus.Manifest) CaptureResult {
	out := CaptureResult{ID: m.ID}
	part := m.Parts[0]
	video := filepath.Join(root, part.File)

	if len(part.Calibration.Corners) != 4 {
		out.Err = "manifest has no 4-corner manual calibration"
		return out
	}
	var manual [4]geom.Pt
	for i, c := range part.Calibration.Corners {
		manual[i] = geom.P(c[0], c[1])
	}

	o := autocal.DefaultOptions()
	if c := part.Calibration.Canonical; c != nil {
		o.Canonical = calibrate.CanonicalBoard{
			MarginX: c.MarginX, MarginY: c.MarginY, PointW: c.PointW,
			QuadH: c.QuadH, BarGap: c.BarGap, OffW: c.OffW,
		}
	}
	ca, errA := profile.ParseHex(part.Priors.CheckerA)
	cb, errB := profile.ParseHex(part.Priors.CheckerB)
	if errA != nil || errB != nil {
		out.Err = fmt.Sprintf("checker color priors unusable: %v %v", errA, errB)
		return out
	}
	o.Profile = profile.CaptureProfile{CheckerA: ca, CheckerB: cb}
	// Colors stay zero: the bench exercises the same no-priors AutoColors
	// path the GUI's Detect button uses (ADR-0008 §1).

	beginMs, endMs := OpeningWindow(m, 0)
	oFind := o
	oFind.ScanEndMs = endMs
	tick, scoreManual, err := autocal.FindOpening(video, manual, oFind, beginMs)
	if err != nil {
		out.Err = fmt.Sprintf("no opening with manual calibration: %v", err)
		return out
	}
	out.OpeningTickMs = tick
	out.ScoreManual = scoreManual

	corners, barEdges, detLens, _, err := autocal.DetectHandles(video, tick, o)
	if err != nil {
		out.Err = fmt.Sprintf("detect: %v", err)
		return out
	}

	out.K1, out.K2 = detLens.K1, detLens.K2

	frame, err := capture.FrameAt(video, tick)
	if err != nil {
		out.Err = fmt.Sprintf("decode opening frame: %v", err)
		return out
	}
	out.ScoreAuto = int(autocal.OpeningScore(frame, corners, o))

	out.CornerDistPx = make([]float64, 4)
	for i := range manual {
		out.CornerDistPx[i] = math.Hypot(corners[i].X-manual[i].X, corners[i].Y-manual[i].Y)
	}
	if len(part.Calibration.BarEdges) == 4 {
		out.BarDistPx = make([]float64, 4)
		for i, b := range part.Calibration.BarEdges {
			out.BarDistPx[i] = math.Hypot(barEdges[i].X-b[0], barEdges[i].Y-b[1])
		}
	}
	return out
}
