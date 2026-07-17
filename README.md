# lazyBG

**Offline, lightweight, cross-platform tool to accelerate and semi-automate transcribing a
backgammon match from its video capture.**

Feed it a match video; for each turn it proposes the dice and the checker move played — each
with a confidence score and the video timecode — auto-filling the confident ones and queuing the
uncertain ones for you to confirm. Exports a standard **`.mat` (Jellyfish)** match file.

> **Status: fresh rebuild in progress.** `main` contains the salvaged, self-contained **gnubg
> engine** (a pure-Go port used offline for legality checking and move ranking), the design docs,
> and a working **inference + perception core**: `geom`/`calibrate` rectify a board via a
> homography; `boardstate` reads checker counts per point by classical color-segmentation; the
> `engine` seam wraps the gnubg port (legal moves + equity + resulting boards); `boarddiff` matches
> an observed board against the engine's legal moves; and `fusion`/`gate` turn the evidence into an
> auto-filled or needs-review `.mat` move. The engine data is embedded (`//go:embed all:data`), so
> `go build ./cmd/lazybg` produces a **self-contained offline binary**; `go run ./cmd/lazybg` plays
> a few turns through the real gnubg engine and prints the `.mat`. On the video side, `capture`
> defines the frame stream and `stableframe` finds the still moments worth reading (motion
> gating) — the ffmpeg-backed frame source and the remaining detectors (clock/commit, dice, cube)
> land once a corpus clip exists. For the labeled corpus, `matimport` reads a `.mat` (round-trips
> with the writer) and `derive` replays it to reconstruct the board + dice at every turn — the
> "labels for free" step (see `docs/experiment-plan.md`). All pure Go (no OpenCV yet). The UI is
> next. The previous manual-transcription app is archived on branch `legacy_v0`.

## Documentation

- [`CLAUDE.md`](CLAUDE.md) — project overview, decisions, and working conventions.
- [`docs/research/video-analysis-survey.md`](docs/research/video-analysis-survey.md) — state of
  the art (pending).
- [`docs/domain-model.md`](docs/domain-model.md) — ubiquitous language.
- [`docs/architecture.md`](docs/architecture.md) — design.

## Build & test

```bash
go build ./...
go test ./gnubg/...   # engine reuse contract
```

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
