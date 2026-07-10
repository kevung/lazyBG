# CLAUDE.md — lazyBG

Guidance for Claude Code (and humans) working in this repository. Read this first.

---

## 1. What lazyBG is

lazyBG is an **offline, lightweight, cross-platform desktop tool that accelerates and
semi-automates the transcription of a backgammon match from its video capture.**

You give it a video of a match (e.g. a competition recording). It watches the game, and for
each turn it proposes the dice rolled and the checker move played, each with a **confidence
score** and the **video timecode** at which it happened. Moves it is confident about are filled
in automatically; moves it is unsure about are queued for you to confirm or correct. The final
result is exported as a standard **`.mat` (Jellyfish) match file**, readable by gnubg, XG, and
BGBlitz.

This is a **fresh rebuild.** The previous incarnation (a manual transcription editor) is
archived on the `legacy_v0` branch. We kept exactly one thing from it — the backgammon engine —
and are rebuilding everything else on clean foundations.

---

## 2. Current state

- **The full video→`.mat` path exists** (`lazybg transcribe/eval/align/demo`): ffmpeg-backed
  capture (`internal/capture`, incl. one-process low-res streaming), stable-window turn
  segmentation (`stableframe`), homography calibration (+ optional lens undistortion), the
  shape-first classical board reader (`perceive/boardstate`, `perceive/checker`), classical dice
  reader (`perceive/dice`), unknown-dice move inference (`boarddiff.DecideAnyDice`, reading-delta
  space), the match conductor (`internal/transcribe`: alternation, inferred dances, game
  boundaries), fusion/gate, `.mat` import/export, and the effort-saved eval harness
  (`eval.ScoreMatch`; runs in `internal/e2e` against the real pilot video).
- **Honest baseline on real footage:** turn segmentation and alignment work; the classical
  reader's ~85% per-point real-frame accuracy is the measured blocker for confident blind
  inference, so plies currently land in the review queue (coverage 0, auto-fill errors 0 —
  the gate holds). See `internal/e2e/realtranscribe_test.go`.
- **The labeling machine is live** (`lazybg align`): truth-forced monotonic alignment anchors a
  `.mat` to its video (per-turn ticks written into `corpus/manifest/*.json`) and extracts
  labeled per-point training crops (`corpus/crops/<id>/`). `ml/` holds the dev-time Python
  training for the learned point reader (ONNX) that should lift the blocker.
- The review UI and the learned-reader Go integration are **yet to be built.**

Do not reintroduce legacy code wholesale. Reference `legacy_v0` for ideas, port deliberately.

---

## 3. Locked decisions (from the design grilling)

These are settled. Revisit them explicitly with the user, not silently.

1. **Reuse scope = gnubg engine only.** The Go engine is the one irreplaceable, offline-capable
   asset. Everything else is rebuilt.
2. **Export format = `.mat` (Jellyfish) / `.txt`** as the canonical output.
3. **Pipeline paradigm = multi-cue probabilistic fusion.** Several *independent* detectors each
   emit a hypothesis + confidence; a fusion step correlates them into the most-probable
   `(dice, move)` per turn with a joint confidence. Agreement ⇒ auto-fill; conflict/weak ⇒
   human-review queue. Each detector is a small, independently testable unit.
4. **Fusion formalism = interpretable-first.** Hard legality filter (engine) + a transparent
   weighted/Bayesian combination of soft cues with hand-set initial weights. No training data
   required to start. A *learned* fusion model is a later upgrade; earlier manual transcriptions
   become its training set.
5. **MVP first vertical = competition footage with a clock.** Fixed camera, chess clock present,
   turn-end signalled by a clock hit, dice re-thrown each turn — the most structured "commit"
   signal. Generalize to casual/handheld footage afterward.
6. **Runtime = CPU-only, fully offline, modest PC.** No discrete GPU assumed at inference.
   Training/dataset work may be heavier and is dev-time-only. Every technique must be judged on
   its CPU/offline runtime cost.
7. **Robustness across heterogeneous captures is first-class** — resolution, aspect ratio,
   codec, board colors, angle, and lighting all vary.
8. **User-declared Capture Profile (Session Priors).** At setup the user can declare known
   constants (clock present? board orientation / which player on which side? board color scheme,
   player names/colors, match length, camera fixed?). Each prior constrains the CV and raises
   baseline confidence. First-class domain concept.
9. **Process = TDD + git worktrees + ~500-line docs** (see §7).

**Resolved by the deep-research survey** (`docs/research/video-analysis-survey.md`, now written):

10. **Shipped stack = one Go app.** gnubg engine in-process; small ONNX detectors via
    `onnxruntime_go` for CPU inference; classical CV in Go (`gocv`/hand-rolled); **video decode via
    a bundled `ffmpeg`**. **Python is dev-time-only** (model training + synthetic rendering).
    Honest caveat: not a literal single static file — a Go binary + ONNX Runtime shared lib +
    `ffmpeg`, bundled per platform (a lightweight *offline bundle*, not one file).
11. **Calibrated-classical-first perception (MVP).** Lean on Session Priors + a one-time
    user-assisted **Board Calibration** (fixed camera) + classical color-segmentation checker
    counting. Learned models are a later upgrade — the survey showed fully-automatic readers
    collapse (~99%→65%) on heterogeneous real footage, so single learned readers are not shipped.

Still open (decided at their milestone, not up front): the **UI toolkit** (leading candidate
Wails v2 + a fresh minimal frontend; see `docs/architecture.md` §3), and the concrete per-detector
techniques for the **evidence-gap** areas — dice/cube reading, clock-hit (incl. audio) detection,
occlusion handling, confidence calibration — which are **prototype-first** (survey §12–§13).

---

## 4. Domain vocabulary (ubiquitous language)

Full definitions live in `docs/domain-model.md`. The essentials:

- **Capture** — the source video; everything is keyed by **video timecode / tick**.
- **Capture Profile / Session Priors** — user-declared known constants that seed the pipeline.
- **Commit Event** — clock hit / dice removed / long board-stability. Anchors turn-end and
  defeats the "players try variations before deciding" problem: only the *last stable board
  before the commit* counts.
- **Stable Board State** — board occupancy + colors at a stable instant, with per-point
  confidence.
- **Cue / Detector output** — one piece of evidence (turn-end, dice value, board state,
  board-diff move, engine prior) with a confidence.
- **Move Hypothesis** — a candidate `(dice, move)` with confidence.
- **Move Decision** — the fused chosen move + joint confidence + video tick; **auto-filled** or
  **needs-review**.
- **Review Item** — a queued low-confidence decision with top-K pre-ranked candidates.
- **Transcription** — the whole match being transcribed: Games → Moves, plus metadata.
- **Backgammon core** — Match, Game, Move, Position, Board, Cube (videau / doubling cube), Dice,
  Score, Crawford, Jacoby, Beaver.
- **Engine** — the gnubg port: **legality is a hard constraint; move-ranking is a soft prior.**

---

## 5. Architecture direction

Full design lives in `docs/architecture.md`. The shape:

```
Capture (video)  ──►  Turn segmentation (Commit Event detectors)
                          │  clock-hit / dice-removed / board-stable
                          ▼
                 per turn: two Stable Board States (pre-roll, post-move)
                          │
        ┌─────────────────┼──────────────────┬───────────────────┐
        ▼                 ▼                  ▼                   ▼
  Dice detector    Board-state reader   Board-diff move     Engine prior
  (pips / none)    (per-point conf.)    (candidate moves)   (legal + ranked)
        └─────────────────┴──────────────────┴───────────────────┘
                          ▼
                     FUSION  (hard legality filter + weighted soft cues)
                          ▼
                  Move Decision  (dice, move, joint confidence, tick)
                          ▼
          confidence ≥ threshold ? ── auto-fill ──► Transcription
                          └──── else ──► Review queue ──► human ──► Transcription
                                                                      ▼
                                                              Export .mat
```

The cues **rescue each other**: if the dice were never visible, infer the dice set that makes
the observed board-diff legal (often unique); if a board-diff is ambiguous from occlusion, the
dice + engine prior disambiguate. Independent agreement raises confidence.

---

## 6. Repository layout

```
gnubg/            The salvaged pure-Go gnubg engine (eval, neural nets, bearoff, MET,
                  position keys). Self-contained: stdlib + its own subpackages only.
  sigmoid/  math32/  met/
data/             Engine data, embedded at runtime: gnubg.weights, gnubg_os0.bd (one-sided
                  bearoff), gnubg_ts0.bd (two-sided bearoff), met/*.xml (match-equity tables).
docs/             Design docs (see §7 for the size rule).
  research/       The deep-research survey (unbounded length).
  domain-model.md   Ubiquitous language.
  architecture.md   Pipeline design.
  experiment-plan.md  Corpus, labeling & evaluation plan.
cmd/lazybg/       CLI: transcribe / eval / align / demo.
internal/         The pipeline (bg, engine, capture, calibrate, perceive/*, boarddiff,
                  fusion, gate, pipeline, transcribe, align, corpus, eval, mat import/export).
corpus/manifest/  Committed Recording manifests (calibration, priors, spans, aligned ticks).
                  Everything else under corpus/ (videos, crops) is gitignored, machine-local.
ml/               Dev-time Python model training (→ ONNX); .venv/ and out/ are gitignored.
testdata/         Committed golden fixtures (hand-checked frames, .mat samples).
tools/xg2mat/     Standalone .xg → .mat converter (own module, vendored deps).
CLAUDE.md         This file.
go.mod / go.sum   Module `lazybg`. Currently zero external dependencies.
```

New code (video ingestion, cues, fusion, review UI, `.mat` export) will be added per the build
order once the survey confirms the stack.

---

## 7. Conventions

### TDD (mandatory)
Red → green → refactor. Write the failing test first, then the minimum code to pass, then clean
up. For this project specifically:
- **Per-cue unit tests** run against **golden-frame fixtures** (hand-labeled still frames):
  dice-value, board-state, clock-event, board-diff each tested in isolation.
- **Fusion tests** use *synthetic* cue inputs (no images) → assert the chosen move, confidence,
  and gate decision. Pure logic, fast, exhaustive.
- **Engine tests** (`go test ./gnubg/...`) must stay green — they are the reuse contract.
- **End-to-end tests** run on short labeled clips → assert the produced `.mat` matches the
  ground-truth transcription within tolerance and flags the right moves for review.

### Git worktrees (mandatory)
Every feature / unit of work happens in its **own git worktree on its own branch**, and is
**merged to `main` when complete and green**. Never develop directly on `main`.

```bash
git worktree add -b feature/<name> /home/unger/src/lazyBG.worktrees/<name> main
# ... work, test, commit in the worktree ...
git -C /home/unger/src/lazyBG checkout main
git -C /home/unger/src/lazyBG merge --no-ff feature/<name>
git worktree remove /home/unger/src/lazyBG.worktrees/<name>
git branch -d feature/<name>
```
Worktrees live under `/home/unger/src/lazyBG.worktrees/` (gitignored sibling directory).

### Documentation size
Design docs (`domain-model.md`, `architecture.md`, feature specs, `CLAUDE.md`) target
**~500 lines** — long enough to be complete, short enough to scan. **Exception:** the
deep-research survey is **unbounded** — as broad and detailed as the subject requires.

### Corpus & fixtures policy
- **Test corpus:** the [BackgammonNews YouTube channel](https://www.youtube.com/@BackgammonNews)
  — diverse real-world captures.
- **Large raw videos are NOT committed.** They live under `corpus/` (gitignored) and are
  referenced by URL + timestamp in a manifest.
- **Small golden-frame fixtures ARE committed** (hand-labeled still frames under `testdata/`),
  kept minimal so the repo stays light.

### Commits
Small, focused, green. Conventional-ish subjects (`feat:`, `fix:`, `test:`, `docs:`, `chore:`).
Do not commit or push unless the user asks; when committing, branch first (never on `main`
directly except merges).

---

## 8. Build & test

```bash
go build ./...            # build everything
go test ./gnubg/...       # the engine reuse contract — must stay green
go test ./...             # full suite (as new packages land)
go vet ./...              # static checks
```

The engine loads its data from an `fs.FS`:
```go
import "lazybg/gnubg"
gnubg.Init(os.DirFS("data"))                       // or an embed.FS via //go:embed all:data
ml, err := gnubg.FindMoves(board, dice, player, /*scoreMoves*/ true, /*cubeful*/ false)
```
At runtime the shipped app will embed `data/` (`//go:embed all:data`) so it stays a single
offline binary. Engine tests reference the on-disk `data/` via a relative path (`../data`).

---

## 9. Where to look next

- **Approved plan & build order:** the plan file referenced in the working session.
- **State of the art / stack decision:** `docs/research/video-analysis-survey.md`.
- **Concepts:** `docs/domain-model.md`.
- **Design:** `docs/architecture.md`.
- **Corpus, labeling & evaluation:** `docs/experiment-plan.md`.
- **Legacy reference (ideas only):** branch `legacy_v0`.
