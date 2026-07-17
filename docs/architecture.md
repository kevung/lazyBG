# Architecture — lazyBG

How the pipeline is built. Companion docs: `domain-model.md` (the vocabulary used throughout),
`research/video-analysis-survey.md` (the evidence behind every technique choice). Target ~500
lines (`CLAUDE.md` §7).

**Locked decisions this design assumes** (from grilling + the survey):
- **Shipped stack = one Go app**, gnubg engine in-process; small ONNX detectors via
  `onnxruntime_go`; classical CV in Go (`gocv`/hand-rolled); **video decode via a bundled
  `ffmpeg`**. **Python is dev-time-only** (training, synthetic rendering).
- **Calibrated-classical-first MVP** — one-time user-assisted **Board Calibration** + Session
  Priors + classical color-segmentation; learned models are a later upgrade.
- **Multi-cue probabilistic fusion**, interpretable-first, over a **hard engine legality filter**.
- **MVP vertical = competition footage with a clock**; **export = `.mat` (Jellyfish)**.

---

## 1. System overview

```
 Capture(video) ─► [capture] decode ─► Frames@Tick
                                         │
                       [profile] Session Priors ──┐
                       [calibrate] Homography ────┤ (one-time, fixed camera)
                                         ▼         ▼
        ┌──────────────────── PERCEPTION (independent Detectors → Cues) ───────────────────┐
        │ [commit] clock-hit / dice-removed / stability → Commit Cue                        │
        │ [stableframe] motion-free window → Stable Frame selection                         │
        │ [boardstate] per-point count+color+conf → Observed Board                          │
        │ [boarddiff] pre/post Observed Board → candidate move(s)                           │
        │ [dice] pip read (or absent) → Dice Cue      [cube] value/side → Cube Cue          │
        └───────────────────────────────────────┬──────────────────────────────────────────┘
                                                 ▼
                       [engine] gnubg: legal moves (hard) + equity ranking (prior)
                                                 ▼
                       [fusion] Cues + legality + prior → ranked Move Hypotheses
                                                 ▼
                       [gate] joint Confidence ≥ threshold?
                            ├─ yes ─► auto-fill ─┐
                            └─ no ─► Review Item ─┤► [transcription] aggregate
                                     (human)      │
                                                  ▼
                                      [matexport] .mat (Jellyfish)
```

Every arrow carries a **Tick**; nothing crosses a context boundary without one (`domain-model.md`
§9). Perception is blind to legality/scoring; Inference is blind to pixels.

---

## 2. Package layout (Go module `lazybg`)

```
gnubg/                     existing pure-Go engine (unchanged; the reuse contract)
data/                      engine weights/bearoff/MET (embedded via //go:embed at ship time)
internal/
  cue/                     Cue types + Confidence (shared perception↔inference vocabulary)
  capture/                 video open, seek, frame decode (bundled ffmpeg), Tick math
  profile/                 CaptureProfile (Session Priors) load/edit/validate
  calibrate/               BoardCalibration: corner input → homography → canonical grid
  perceive/                Detectors, each emitting a Cue:
    stableframe/             motion-window stable-frame selection
    boardstate/              Observed Board reader (classical seg; ONNX later)
    boarddiff/               two Observed Boards → candidate move(s)
    dice/                    pip reader (optional; degrades gracefully)
    commit/                  clock-hit (visual ROI + audio onset) / dice-removed / stability
    cube/                    doubling-cube value/side + double events
  engine/                  thin adapter over gnubg (legality filter + ranking prior)
  fusion/                  Cues → ranked MoveHypotheses → MoveDecision (pure)
  gate/                    Confidence→{auto-fill,review} threshold policy (pure)
  transcription/           aggregate root: Games→Turns→MoveDecisions + Review queue
  matexport/               .mat (Jellyfish) writer + importer
cmd/
  lazybg/                  desktop app: UI shell wiring the pipeline + review workflow
ml/                        DEV-TIME ONLY (Python): dataset rendering + model training → ONNX
testdata/                  committed golden frames (hand-labeled)
corpus/                    gitignored raw videos + manifest (URL + timecode)
```

Rationale: `internal/` keeps the pipeline encapsulated; `cue/` is the *lingua franca* so
perception and inference share types without a dependency cycle; `engine/` isolates the gnubg
coordinate-translation seam (the legacy `analysis.go` pain point) in one place.

---

## 3. Module responsibilities & interfaces

Interfaces below are indicative Go signatures — the shape, not the final API. Each is designed as
a **deep module**: a small surface over real work, and an obvious unit-test seam.

### capture — video → Frames
Opens a Capture, decodes frames, seeks by Tick. Wraps a **bundled ffmpeg** (CLI via `ffmpeg-go`
for the MVP; `go-astiav` CGO later if frame-accurate seeking demands it — survey §9).
```go
type Frame struct { Tick Tick; Img image.Image }
type Capture interface {
    Info() CaptureInfo
    FrameAt(Tick) (Frame, error)          // nearest-keyframe seek + decode
    Frames(from, to Tick, step) iter.Seq[Frame]
    Audio() (AudioTrack, bool)            // for the commit audio-onset cue
}
```

### profile — Session Priors
Loads/edits/validates a `CaptureProfile`. Pure data + validation; no I/O beyond its own file.
Feeds every Detector and the Gate.

### calibrate — Board Calibration
Turns user-clicked corners (MVP) into a homography and the canonical grid; exposes pixel↔point
mapping. Pure geometry once corners are supplied.
```go
type BoardCalibration interface {
    Rectify(Frame) RectifiedFrame            // apply homography
    PointRegion(p PointID) image.Rectangle   // where point p lives on the rectified board
    Confidence() float64
}
```

### perceive — Detectors (the Cue producers)
Each Detector is independent and emits one Cue kind. Common contract:
```go
type Detector[C Cue] interface {
    Detect(in DetectInput) (C, error)   // in: Frame(s) + BoardCalibration + relevant priors
}
```
- **stableframe**: inter-frame diff over the board ROI → stable windows (gates everything; cheap).
- **boardstate**: per-point classical color-segmentation + stack counting on the rectified board
  → `ObservedBoard` with per-point counts, colors, confidences. Self-calibrates stack
  pixel-height→count from the known opening layout (survey §2). ONNX ordinal head later.
- **boarddiff**: diff two `ObservedBoard`s → minimal checker relocation(s) = candidate move(s).
- **dice**: pip read when visible; returns *absent* otherwise (fusion infers dice from legality).
- **commit**: prioritized cascade — clock-hit (visual ROI-diff in the declared clock region +
  audio-onset) → dice-removed/re-thrown → long stability. Emits a `CommitCue` with confidence.
- **cube**: value/side + rare double events; review-heavy by default.

### engine — the gnubg seam
The one place that translates lazyBG `Board`/`Position` ↔ gnubg `TanBoard`, and the only caller of
`gnubg.FindMoves`. Exposes exactly the two roles from `domain-model.md` §6:
```go
type Engine interface {
    LegalMoves(pos Position, dice Dice) []Move            // hard constraint
    Rank(pos Position, dice Dice) []RankedMove            // soft prior (equity)
}
```
Port the legacy `positionToGnubgBoard` / `gnubgMoveToLazyBG` logic here deliberately, with tests.

### fusion — Cues → Move Decision (pure, the heart)
```go
type Fusion interface {
    Fuse(seg TurnSegment, cues []Cue, eng Engine) MoveDecision
}
```
Algorithm (interpretable-first; §4). Pure function of its inputs → the **synthetic-cue** test
surface. No pixels, no I/O.

### gate — auto-fill vs review (pure)
`Gate(decision) → {AutoFill | NeedsReview, reason}`. Single inspectable threshold policy,
parameterizable by Session Priors (e.g. stricter with a live cube). Pure; exhaustively testable.

### transcription — aggregate root
Holds the growing match (Games→Turns→MoveDecisions), the Review queue, the Capture Profile and
Board Calibration, and metadata. Applies resolved Review Items. Serializes to lazyBG's own working
format — the `.lbg` session file, single source of truth; `.mat` and the corpus manifest are
projections generated from it on demand (`docs/session-format-spec.md`).

### matexport — .mat (Jellyfish)
Projects the Transcription to the Jellyfish `.mat`/`.txt` layout (metadata block, `N point
match`, two-column move tables, cube actions, bear-off/resign). Round-trip-stable with the
importer. Reference the legacy `matchExporter.js`/`matchParser.js` on `legacy_v0` for the exact
column layout, but reimplement in Go with tests.

### cmd/lazybg — the UI shell
Wires the pipeline and drives the human-in-the-loop workflow: **video scrubber ↔ move list ↔
board render ↔ review queue**, plus the one-time calibration click and the Session-Priors setup
form. **UI toolkit locked: Wails v2** (Go + the OS's native webview; HTML5 `<video>` makes
scrubbing trivial and it stays lightweight — no bundled Chromium) **+ a fresh Svelte frontend**
(new code, not a reuse of the `legacy_v0` Svelte app) — ADR-0002. The Wails layer is a thin
binding over an independent Go **session service** package (turn entry, candidate ranking,
export) with no Wails-specific types in its API — ADR-0003 — so the same service can later be
driven headlessly over a REST API without rearchitecting.

---

## 4. Confidence & fusion model

**Inputs per Turn Segment:** the pre/post `ObservedBoard`s, an optional `DiceCue`, an optional
`CubeCue`, the `CommitCue`, and the Engine.

**Step 1 — candidate generation under hard legality.**
- If dice observed: `cands = Engine.LegalMoves(preBoard, dice)`.
- If dice absent: for each plausible dice pair `d`, take `Engine.LegalMoves(preBoard, d)`; keep
  those whose resulting board matches the post `ObservedBoard` within tolerance. The dice set that
  makes the board-diff legal is frequently unique (survey §3) — this is how the engine *rescues*
  missing dice.

**Step 2 — soft scoring (interpretable weighted combination).**
For each legal candidate move `m`, combine independent cue agreements:
```
score(m) = Σ_i  w_i · agree_i(m)
  agree_boarddiff(m)  = overlap(apply(m,preBoard), postObservedBoard)   ∈ [0,1]  (per-point conf-weighted)
  agree_dice(m)       = diceMatch(m, DiceCue)                            (1 if consistent / prior if absent)
  agree_enginePrior(m)= rankToPrior(Engine.Rank(...), m)                 (top moves → higher prior)
  agree_cube(m)       = cubeConsistency(m, CubeCue)
```
`w_i` are hand-set reliability weights (per `CaptureProfile` — e.g. down-weight `dice` when the
profile says transparent dice). Normalize `score` over candidates → a distribution.

**Step 3 — decision + joint confidence.**
`top = argmax; margin = score(top) − score(2nd)`. **Joint Confidence** is a monotonic function of
both the winner's normalized mass **and** the margin (a clear winner with agreeing cues → high;
a near-tie or cue conflict → low). Attach top-K for review.

**Upgrade path.** Swap the weighted sum for **Dempster–Shafer / Dynamic Belief Fusion** (explicit
*uncertain* mass → maps onto the Gate; survey §7) behind the same `Fusion` interface, and add a
**learned** fusion once transcriptions accumulate — without touching detectors or the Gate.

**Calibration.** Confidence starts *uncalibrated*; thresholds are conservative (over-refer to
review). Add temperature scaling / reliability diagrams once labeled data exists.

---

## 5. The per-turn control loop (in `cmd/lazybg`)

```
for each Commit Event (from commit detector, in Tick order):
    seg      := pair(lastStableBefore(prevCommit), lastStableBefore(thisCommit))
    obsPre   := boardstate.Detect(seg.preFrame,  calib, profile)
    obsPost  := boardstate.Detect(seg.postFrame, calib, profile)
    cues     := collect(boarddiff(obsPre,obsPost), diceCueNear(seg), cubeCueNear(seg), thisCommit)
    decision := fusion.Fuse(seg, cues, engine)      // pure
    switch gate.Classify(decision, profile) {
        case AutoFill:    transcription.Append(decision)
        case NeedsReview: transcription.Queue(reviewItemFrom(decision))
    }
render(transcription); user resolves review queue; matexport.Write(transcription)
```
Only stable/commit frames are read — never every frame — which is what keeps CPU cost bounded
(survey §0.3, §12.7).

---

## 6. Tech stack & packaging

| Concern            | Choice (MVP)                              | Notes / upgrade |
|--------------------|-------------------------------------------|-----------------|
| Language (app)     | **Go** (module `lazybg`)                  | engine in-process |
| Engine             | **gnubg/** (existing)                     | legality + prior |
| Classical CV       | **gocv** or hand-rolled Go                | avoid full OpenCV dep where cheap |
| Learned inference  | **ONNX via `onnxruntime_go`**             | ships an ORT shared lib |
| Video decode       | **bundled `ffmpeg`** (`ffmpeg-go` CLI)    | `go-astiav` CGO later for exact seek |
| UI shell           | **Wails v2** (native webview) — locked, ADR-0002 | fresh Svelte frontend |
| Model training     | **Python (dev-time only)** → ONNX         | + synthetic 3D rendering |

**Packaging reality (survey §8–§9):** the deliverable is **not** one static file — it is a Go
binary **+** the ONNX Runtime shared lib **+** an `ffmpeg` binary, bundled per platform. Target: a
*lightweight, fully-offline, self-contained bundle*. `data/` is embedded via `//go:embed all:data`.

---

## 7. Testing architecture (maps to `CLAUDE.md` §7 TDD)

- **Detector unit tests** → committed **golden frames** in `testdata/` (dice-value, board-state,
  clock-event, board-diff), each Detector in isolation.
- **Fusion tests** → **synthetic Cue** inputs (no images): assert chosen move, joint confidence
  band, and Gate outcome. Fast, exhaustive, deterministic.
- **Engine tests** → keep `go test ./gnubg/...` green (the reuse contract) + tests on the
  `engine/` translation seam.
- **matexport tests** → round-trip `.mat` fixtures (import → export → structurally stable).
- **End-to-end tests** → short labeled corpus clips: assert the produced `.mat` matches ground
  truth within tolerance and flags the right turns for review.

Dependencies point inward and toward pure functions, so most of the system tests without a video
at all — only Detectors need frames.

---

## 8. Build order (each a worktree → merge to `main`)

1. **Walking skeleton** (proves the spine): `capture` opens a clip → a **single** `commit`
   detector (or a hand-stubbed one) segments turns → a **stubbed** `MoveDecision` with a fixed
   confidence → `matexport` writes a `.mat`. Golden-frame + synthetic-cue tests. A throwaway UI or
   CLI is fine here.
2. **calibrate** + **boardstate** (classical) → real `ObservedBoard`s on a calibrated fixed-camera
   clip.
3. **boarddiff** + **engine** seam + **fusion** (weighted) + **gate** → real Move Decisions with
   legality-constrained candidates and dice-inference.
4. **commit** detector for real (visual ROI + audio onset) + **dice**/**cube** cues.
5. **review UI** (Wails v2 + Svelte, locked — ADR-0002) + Session-Priors setup + one-time
   calibration flow.
6. Robustness pass; then generalize beyond the clocked-competition vertical.

---

## 9. Open architectural risks (tracked in survey §12)

- **OOD board-reading collapse** → mitigated by calibration + priors + classical MVP + fusion +
  review; do not ship a lone learned reader.
- **Dice / clock / occlusion have no prior art** → prototype-first; engine-legality makes dice
  optional; audio-onset for commit is unproven and must be measured.
- ~~UI toolkit not locked~~ **resolved** → Wails v2 + Svelte, ADR-0002.
- **"Single binary" is a myth** → ship a documented lightweight bundle instead.
