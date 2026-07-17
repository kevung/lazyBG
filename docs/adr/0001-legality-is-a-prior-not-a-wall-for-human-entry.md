---
status: accepted
---

# Legality is a hard filter only on the automatic inference path, never on human-witnessed entry

`docs/domain-model.md` §6 states "legality = hard constraint" for the Engine, meaning the
automatic Fusion path (reconciling board-diff pixels with physics) discards any candidate move
the Engine doesn't recognize as legal. Specifying manual/human transcription
(`docs/functional-spec.md` §4) surfaced a case that rule doesn't cover: a person directly
recording what they saw on video may have witnessed a move that was actually illegal under the
rules — a misplay that stood, or a disputed ruling — and real matches do contain these. We
decided the hard filter binds only automatic candidate generation. Human-witnessed entry (checker
moves or cube actions typed by someone watching the video) always keeps a free-entry escape hatch
one step away from the Engine's ranked-legal-move list, regardless of legality, so the
Transcription can match what actually happened. The ranked list stays the default, fast path for
the common case; it is a prior for the human to pick from, never a wall.

## Consequences

- Fusion/gate/board-diff code is unchanged: it keeps discarding illegal candidates.
- Manual-entry UI code must never hard-validate a human-confirmed move against
  `engine.LegalMoves` before accepting it — it may warn, it must not block.
- `docs/domain-model.md` §6 now states this distinction explicitly.
