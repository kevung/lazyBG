---
status: accepted
---

# GUI board: port blunderDB's two.js renderer; migrate the frontend to Svelte 5

lazyBG's board panel (`gui/frontend/src/Board.svelte`) is a ~75-line read-only SVG that draws a
`bg.Board` (`Pts`/`Bar`/`Off`) beside the raw video frame ("what the app believes", ux-spec §6). It
must also show the **cube (value + owner)** and the **match score** — the information a transcriber
needs to keep the running state straight. blunderDB (`~/src/blunderDB`, same author, MIT) already
has a mature two.js board renderer with exactly the visual language and these capabilities.

The blunderDB component cannot be copied as-is: it uses **Svelte 5 runes** (lazyBG is on Svelte 4),
depends on **two.js** (lazyBG's frontend has zero runtime deps), and is wired to ~12 blunderDB
stores (`positionStore`, `analysisStore`, `boardColorsStore`, search/exclude stores, i18n, logger).
Much of its capability (position editing, search-exclude structures, analysis-move arrows) is
blunderDB-specific and irrelevant here.

## Decision

- **Port the two.js rendering engine** (board / checkers / dice / cube / score), **read-only**,
  **driven by props** (`bg.Board` + `cube{value, owner, centered}` + `score`), dropping the
  blunderDB store coupling and the search/analysis/editing capabilities. **No pip count** (declared
  out of scope by the user).
- **Migrate the lazyBG frontend to Svelte 5.** It is largely backward-compatible (Svelte 4 syntax
  runs under 5), so the port keeps the component's runes with minimal rewrite and opens the door to
  a genuinely shared board component between the two apps later.
- **Add `two.js` as the frontend's first runtime dependency.**
- **Prerequisite (small binding/model change):** expose the cube **owner / centered** state to the
  frontend. `bg.Board` carries only `Pts`/`Bar`/`Off`; the cube lives in the session
  (`internal/session/cube.go`) and only `CubeValue()` is currently bound.

## Considered options

- **Restyle the existing native SVG board** (no two.js, stay Svelte 4) — the lightweight option, but
  the user chose the exact blunderDB look and its richer rendering (dice/cube already built, useful
  for the roadmap).
- **Stay on Svelte 4, rewrite the runes to `$:`** — viable (the kept drawing code is
  framework-agnostic), but the user chose Svelte 5 to ease future code-sharing with blunderDB.
- **Full interactive port (drag to correct checkers)** — deferred; the board stays read-only. If
  manual board correction is wanted later, the two.js base makes it reachable.
- **Extract one shared board package across both apps now** — attractive long-term, too large a
  refactor (versioning, monorepo) for this milestone.

## Consequences

- The frontend gains a runtime dependency (`two.js`); record its provenance in `NOTICE.md`. The
  "only external dependency is Wails" note in CLAUDE.md §6 is about **Go** modules and stays true;
  clarify it covers the Go side.
- The Svelte 5 migration touches `App.svelte` / `SetupPanel.svelte` (low risk).
- A small cube-owner binding lands before the board port.
- Open sub-decision (deferred, low-stakes): when the board follows a past position via
  `BoardAt(seq)`, the cube/score shown are the **current** state, not that turn's — acceptable for a
  read-only "what the app believes" panel; revisit if it misleads.
