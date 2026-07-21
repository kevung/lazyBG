---
status: accepted
---

# Auto-calibration is a triangle↔canonical correspondence fit, benchmarked on the corpus

## Context

A grilling session (2026-07-21) on issue #11 (auto-estimate board geometry + lens distortion
from image features), with the goal of making the auto-detected corners **precise and
generalizable** across heterogeneous captures.

State at the time: `internal/autocal` finds an initial quad from the point-color mask and
refines it either against the standard-start oracle (`Calibrate`) or not at all
(`DetectHandles`, the interactive 8-handle seed for the GUI). An attempt to sharpen corners by
raising the detection resolution (#47) regressed the bar detection and was reverted — the
symptom of tuning against a single capture with no multi-capture measurement. `calibrate.Lens`
models k1 only, and a hand-swept k1 gave nothing on the pilot (board near frame centre), while
the concern is real for oblique/wide-angle rigs.

## Decisions

1. **Success metric = the existing corpus, both yardsticks.** The ~22 committed manifests carry
   hand-placed calibrations; the benchmark runs auto-detection on every locally-present capture
   (42 videos at decision time) and reports (a) pixel distance to the manual handles — corners
   *and* bar — as a diagnostic (manual handles are only ±10 px truth), and (b) the opening read
   score with auto vs manual calibration as the functional judge. The bench derives board colors
   via `AutoColors` (manifests do not declare them), exercising the same no-priors path the GUI
   uses.

2. **Baseline first, then ratchet.** The bench is landed *before* any detection change, and the
   per-capture score table is committed as the reference. Every subsequent change must not
   degrade any capture beyond noise (−1 read point) and must raise the mean. Absolute exit
   thresholds for #11 are frozen only after the baseline exists. This is the direct antidote to
   the #47 bump/revert cycle.

3. **One oracle-free core for both consumers.** Corner precision is implemented once, without
   the standard-start oracle, so `DetectHandles` (any frame, interactive) uses it directly and
   `Calibrate` chains it with the opening oracle as final validation.

4. **Estimator = explicit correspondences + least squares; score hill-climb as polish and
   fallback.** Triangle components are indexed (2 rows × 12 columns via the principal axis
   already computed by `RowQuad`); each triangle contributes its **apex, recovered as the
   intersection of its two fitted lateral edges** — sub-pixel, and insensitive to base
   truncation by stacked checkers (whose centroid bias would otherwise leak into the fit and
   into k1). The correspondences drive a least-squares homography fit per half (the ADR-0007
   two-homography model); per-column residuals expose true pitch and bar width, radial residuals
   and lateral-edge curvature expose lens distortion. When indexing fails (occlusion, merged
   components), the existing mask-score hill-climb remains as fallback; it also serves as a
   final polish pass.

5. **Lens = k1 + k2, centre fixed at the image centre, nested admission.** `calibrate.Lens`
   gains a K2 term (r⁴; Newton inversion adapted). Estimating the distortion centre from
   board-only features is ill-conditioned (centre↔H compensate) and stays out until a real
   wide-angle cell demands it. Guard against overfit: fit 0 → k1 → k1+k2 and admit each extra
   coefficient only if it cuts the correspondence RMS by a significant margin (threshold set at
   the bench), else the coefficient stays exactly 0 — preserving Lens's "zero = identity"
   contract. The pilot must come out k1=k2=0 (bench test).

6. **k1/k2 validation = synthetic in CI, real cell as the final gate.** Development and CI
   validate by applying a *known* barrel distortion to existing captures and requiring recovery
   of the coefficients and of the undistorted read score. A labeled real wide-angle capture
   remains the exit gate for the lens half of #11, to be found or shot for the corpus.

7. **Frames: single short median in the GUI, multi-instant aggregation in the pipeline.**
   `DetectHandles` keeps its short median (latency; the result is a correctable seed).
   `Calibrate` aggregates apexes from 2–3 spaced instants so moving checkers uncover different
   triangles — that path persists the fit, so it buys the robustness.

8. **Persistence = the existing v2 vocabulary + `Lens.K2` only.** The fit projects its result
   onto the 8 handles + optional `Canonical` + `Lens` (now with `k2`, omitempty — no schema
   version bump; old manifests read unchanged). No per-column pitch field unless the bench
   proves a residual the 8 handles cannot express.

9. **GUI: honest overlay, no manual coefficients.** The calibration grid overlay is drawn
   through `Lens.distort()` (lines curve as reality does); a small indicator shows the estimated
   k1/k2 with a single "reset to 0" action. No sliders; dragging handles never mutates the lens.

## Delivery (one worktree per stage, each gated by the bench)

1. `feat/autocal-bench` — the multi-capture bench + committed baseline (no detection change).
2. `feat/triangle-apex` — apex extraction by lateral-edge intersection + 2×12 indexing.
3. `feat/corners-fit` — correspondence fit → 8 handles; wired into `DetectHandles`, hill-climb
   fallback.
4. `feat/lens-k1k2` — `Lens.K2` + nested admission, validated on synthetic distortion.
5. `feat/calibrate-multiframe` — multi-instant aggregation in `Calibrate`; opening oracle kept
   as final validation.
6. `feat/gui-lens-overlay` — curved overlay + k1/k2 indicator + reset.

## Consequences

- The bench makes calibration work measurable across events; single-capture tuning regressions
  (the #47 cycle) are caught at PR time.
- The correspondence fit turns #11's "geometry estimation" into observable quantities (per-column
  residuals, edge curvature) instead of a blind high-dimensional search; its failure modes are
  detectable (indexed-triangle count, final residual), so the fallback is clean.
- The schema stays stable; every downstream consumer (GUI, `.lbg`, pipeline) keeps speaking
  8 handles + canonical + lens.
- The lens half of #11 is only *provisionally* validated until a real wide-angle capture joins
  the corpus — tracked as a corpus task, not a code stage.

## Implementation notes (2026-07-22, stages 1–6 delivered)

What the bench forced us to learn, recorded so the next reader does not
re-derive it:

- **Apexes alone are degenerate.** The 24 tips lie on two parallel rows; a
  point-only homography fit leaves a one-parameter vertical-stretch family
  (±70px corner error). Fixed with **line↔line correspondences**
  (`geom.HomographyFitFeatures`, `l_canon ∝ Hᵀ·l_img`): the two outer edges
  pin the transverse extent.
- **Never trust triangle width.** Constraining canonical base corners (or
  lateral-edge endpoints at full column width) drags the scale — real points
  are narrower than their column (~17% shrink measured on the pilot). The
  admitted primitives are width-free: apex points + outer edge lines.
- **Outer edges need envelope logic, not LSQ.** Checker stacks truncate
  bases strictly centreward; colour bleed leaks outward. The edge is found
  as the outermost well-supported line through the base points (dense pair
  RANSAC); both plain LSQ and one-sided trimming fail on real masks.
- **One geometry parameter is estimated per capture:** the apex line's
  canonical y (effective triangle length), re-estimated each round with
  top/bottom symmetry imposed. With DefaultCanonical's fixed proportions the
  apexes fight the edge lines (14px residual on the pilot).
- **Seed-free bootstrap.** When the mask quad goes wild (RowQuad's principal
  axis), apexes are indexed without a seed: rows from apex orientation,
  absolute columns by scoring all (leftmost, rightmost)-slot alignment
  hypotheses — the bar gap anchors them — cross-checked between rows.
- **Lens admission is residual-driven** (not curvature-solving as §5
  sketched): golden-section on k1 then k1+k2, refitting on undistorted
  features per candidate; admission needs −12% relative AND −0.15px absolute
  gain, so pinhole captures come out exactly 0/0.
- **Calibrate scores the fit with the full model** (8 handles + lens) and
  polishes with `RefineHandles` — the oracle hill-climb over all eight
  handles (bar x/width included), which observes exactly the outer-x and bar
  proportions the image structure cannot pin. The corners-only oracle path
  remains as a competitor; the better opening read wins.
- **Measured:** bench mean auto opening score 5.86 → 7.32/24 (22 captures,
  none regressed; four captures reach near-manual corners at 30–61px);
  pilot `Calibrate` 16/24 → 20/24 (the e2e test passes again). Known
  remaining failure modes: `AutoColors` picks wrong clusters on the three
  2026-05 Marseille captures (empty triangle mask), and heavily stacked
  mid-game frames on some cdf/vbc cells.
