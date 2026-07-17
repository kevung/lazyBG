# lazyBG

**Offline, lightweight, cross-platform desktop tool that accelerates and semi-automates
transcribing a backgammon match from its video capture.**

Feed it a match video. For each turn, lazyBG proposes the dice rolled and the checker move
played — each with a **confidence score** and the **video timecode** at which it happened. Moves
it is confident about are filled in automatically; moves it is unsure about are queued for you to
confirm or correct. The result is exported as a standard **`.mat` (Jellyfish)** match file,
readable by gnubg, XG, and BGBlitz.

> **lazyBG is a standalone project, not a fork.** It reuses one thing from the backgammon
> ecosystem — a pure-Go port of the GNU Backgammon engine — and rebuilds everything else. See
> [Credits](#credits).

## Status

**Fresh rebuild, in progress.** The full **video → `.mat`** path exists end-to-end and runs
against real competition footage:

- **Capture & segmentation** — ffmpeg-backed frame capture (`internal/capture`), stable-window
  turn segmentation (`internal/perceive/stableframe`), homography calibration with optional lens
  undistortion (`internal/calibrate`, `internal/geom`).
- **Perception** — a shape-first classical board reader (`perceive/boardstate`,
  `perceive/checker`), a classical dice reader (`perceive/dice`), and a **learned per-point CNN
  reader** (`perceive/pointnet`) that runs in pure Go (no cgo, torch-parity-tested) and already
  beats the classical baseline on blind transcription.
- **Inference & fusion** — unknown-dice move inference in reading-delta space
  (`boarddiff.DecideAnyDice`), the match conductor (`internal/transcribe`: alternation, inferred
  dances, game boundaries), and multi-cue fusion + a confidence gate (`internal/fusion`,
  `internal/gate`) that decides auto-fill vs. review.
- **I/O & eval** — `.mat` import/export (`internal/matimport`, `internal/matexport`) and an
  effort-saved evaluation harness (`internal/eval`, exercised in `internal/e2e` against the real
  pilot video).

**Honest baseline on real footage:** turn segmentation and alignment work; the classical
reader's ~85% per-point accuracy on real frames is the measured blocker for confident blind
inference, so plies currently land in the review queue (coverage 0, auto-fill errors 0 — the
gate holds rather than guessing). The learned reader (89% per-crop on held-out games) is the path
past this; it needs **more corpus manifests** for diversity before auto-fill coverage rises.

**The labeling machine is live** (`lazybg align`): truth-forced monotonic alignment anchors a
`.mat` to its video and extracts labeled per-point training crops — "labels for free" that feed
model training under `ml/` (dev-time Python → ONNX + a flat weight file run in Go).

**Next:** the review UI, first scoped as a **standalone manual-transcription tool** (produce a
`.mat` from a video entirely by hand) built on the same data model the automatic pipeline targets
— so as detector confidence rises, the same screen becomes the automatic pipeline's review UI for
free. A walking skeleton exists (`internal/session`). Toolkit is Wails v2 + Svelte
(see [ADR-0002](docs/adr/0002-wails-v2-svelte-ui-toolkit.md)).

The previous manual-transcription app is archived on branch `legacy_v0`.

## Build & test

```bash
go build ./...            # build everything
go test ./gnubg/...       # the engine reuse contract — must stay green
go test ./...             # full suite
```

The engine data is embedded (`//go:embed all:data`), so `go build ./cmd/lazybg` produces a
self-contained offline binary. Runs are CPU-only and fully offline.

## Usage

```bash
# Play a few turns through the real gnubg engine and print the .mat (no video needed):
go run ./cmd/lazybg

# Transcribe a captured match into a .mat, driven by a Recording manifest:
lazybg transcribe -manifest corpus/manifest/<id>.json -out match.mat \
                  [-model <point-reader.weights>] [-dice-model <dice.weights>]

# Score a transcription against its ground-truth .mat (effort-saved metrics):
lazybg eval -manifest corpus/manifest/<id>.json [-model <point-reader.weights>]

# Anchor a .mat to its video and extract labeled training crops:
lazybg align -manifest corpus/manifest/<id>.json -write-manifest -crops corpus/crops/<id>/
```

A **Recording manifest** (`corpus/manifest/*.json`) declares a capture's calibration, session
priors, spans, and aligned per-turn ticks. Manifests are committed; the videos and crops they
reference are machine-local and gitignored.

## Documentation

- [`CLAUDE.md`](CLAUDE.md) — project overview, locked decisions, and working conventions.
- [`docs/domain-model.md`](docs/domain-model.md) — ubiquitous language.
- [`docs/architecture.md`](docs/architecture.md) — pipeline design.
- [`docs/research/video-analysis-survey.md`](docs/research/video-analysis-survey.md) — state of
  the art and the stack decision.
- [`docs/experiment-plan.md`](docs/experiment-plan.md) — corpus, labeling & evaluation plan.
- [`docs/functional-spec.md`](docs/functional-spec.md),
  [`docs/ux-spec.md`](docs/ux-spec.md),
  [`docs/session-format-spec.md`](docs/session-format-spec.md) — the manual/automatic
  transcription tool: what it does, its UI/flow, and the `.lbg` session file.
- [`docs/adr/`](docs/adr/) — architecture decision records.

## Credits

lazyBG stands on work by others, taken deliberately and acknowledged here. It is **not** a fork of
any of these projects and does not track their development.

- **[GNU Backgammon](https://www.gnu.org/software/gnubg/)** — the origin of the engine lazyBG
  depends on for move legality and ranking, and of the neural-network weights, bearoff databases,
  and match equity tables embedded from `data/`.
- **[bgweb-api](https://github.com/foochu/bgweb-api)** by **Rami Keränen** (`foochu`) — the Go port
  of that engine, published under the MIT License, which is what `gnubg/` actually is. This
  repository began its life from bgweb-api's history and has since been detached into a standalone
  project; Rami's commits remain in the log, and his copyright notice is preserved in
  [`gnubg/LICENSE`](gnubg/LICENSE).
- **[BackgammonNews](https://www.youtube.com/@BackgammonNews)** — the source of the real-world match
  footage the perception work is developed and evaluated against.

lazyBG's own source is MIT-licensed, Copyright (c) 2025-2026 Kévin Unger — see
[`LICENSE`](LICENSE). Full provenance and the terms of every bundled third-party component are in
[`NOTICE.md`](NOTICE.md).
