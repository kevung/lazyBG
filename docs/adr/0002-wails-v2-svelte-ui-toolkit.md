---
status: accepted
---

# UI toolkit: Wails v2 + a fresh Svelte frontend

`docs/architecture.md` §9 listed the UI toolkit as an explicit open risk, deliberately deferred to
the review-UI build milestone "on real ergonomics, not up front." Speccing the manual
transcription tool (`docs/functional-spec.md`, `docs/ux-spec.md`) made the UI shell the very next
piece of work, and the user has already shipped a Wails v2 + Svelte + Go backend app across
Windows/Linux/macOS and was satisfied with the result — real ergonomics evidence, not a guess. We
lock **Wails v2** (Go + the OS's native webview — no bundled Chromium, HTML5 `<video>` makes
scrubbing trivial, stays lightweight/offline) with a **fresh Svelte frontend** — new code, not a
reuse of `legacy_v0`'s Svelte app, which was a different tool (manual-editor-only, no video, no
Go engine binding front-and-center).

## Considered options

- Native-Go GUI (Gio/Fyne) — ruled out per `architecture.md` §3: weaker video playback, the core
  interaction surface of this tool.
- Toolkit-agnostic UX spec, decide later — rejected: the UX spec (candidate lists, keyboard flow,
  video scrubber) needs a concrete component vocabulary to be useful now, and there's already
  first-hand ergonomic evidence for this exact stack.

## Consequences

- `docs/ux-spec.md` is written in terms of Wails-bindable Go methods + Svelte components/routes,
  not toolkit-neutral abstractions.
- `docs/architecture.md` §3, §6, §8, §9 and `CLAUDE.md` §3 updated to reflect the lock.
- See ADR-0003 for how the Go side stays decoupled from Wails specifically.
