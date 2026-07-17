# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the
codebase.

## Before exploring, read these

- **`docs/domain-model.md`** at the repo root — lazyBG's ubiquitous language. This repo predates
  the `CONTEXT.md` convention and uses this file as its equivalent; treat it as `CONTEXT.md` for
  every skill that expects one.
- **`docs/architecture.md`** — how the pieces interact; read alongside `domain-model.md`.
- **`docs/functional-spec.md`**, **`docs/session-format-spec.md`**, **`docs/ux-spec.md`** — living
  specs for the manual/automatic transcription tool (produced via `/grill-with-docs`). Read
  whichever is relevant to the area you're touching.
- **`docs/adr/`** — read ADRs that touch the area you're about to work in.

Single-context repo — no `CONTEXT-MAP.md` / per-context split.

If any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest
creating them upfront. `/domain-modeling` (reached via `/grill-with-docs`) creates/extends them
lazily when terms or decisions actually get resolved.

## File structure

```
/
├── CLAUDE.md
├── docs/
│   ├── domain-model.md        ← glossary (this repo's CONTEXT.md equivalent)
│   ├── architecture.md
│   ├── functional-spec.md
│   ├── session-format-spec.md
│   ├── ux-spec.md
│   ├── experiment-plan.md
│   ├── research/
│   └── adr/
│       ├── 0001-legality-is-a-prior-not-a-wall-for-human-entry.md
│       ├── 0002-wails-v2-svelte-ui-toolkit.md
│       └── 0003-session-service-decoupled-from-wails.md
├── internal/
├── cmd/lazybg/
└── gnubg/
```

## Use the glossary's vocabulary

When your output names a domain concept (an issue title, a hypothesis, a test name), use the term
as defined in `docs/domain-model.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal — either you're inventing
language the project doesn't use (reconsider) or there's a real gap (note it for
`/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently
overriding:

> _Contradicts ADR-0002 (Wails v2 + Svelte toolkit) — but worth reopening because…_
