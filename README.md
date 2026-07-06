# lazyBG

**Offline, lightweight, cross-platform tool to accelerate and semi-automate transcribing a
backgammon match from its video capture.**

Feed it a match video; for each turn it proposes the dice and the checker move played — each
with a confidence score and the video timecode — auto-filling the confident ones and queuing the
uncertain ones for you to confirm. Exports a standard **`.mat` (Jellyfish)** match file.

> **Status: fresh rebuild in progress.** This branch (`main`) currently contains only the
> salvaged, self-contained **gnubg engine** (a pure-Go port used offline for legality checking
> and move ranking) plus the design docs. The video pipeline, computer vision, and UI are being
> built on top. The previous manual-transcription app is archived on branch `legacy_v0`.

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

The engine is a port of [GNU Backgammon](https://www.gnu.org/software/gnubg/); the Go checker
evaluation port is credited to Rami Keränen (foochu) via the bgweb-api project. lazyBG is
licensed under the terms in [`LICENSE`](LICENSE).
