---
status: accepted
---

# Player 1 is the player at the bottom of the video

## Context

A retex on the calibration panel (2026-07-23): *"the declared player checker colours do not match
the colours drawn on the reconstructed boards or on the orientation preview. Player 1 is the
bottom player, Player 2 the top one — so the two back checkers at the top must be Player 1's."*

The report reads like a paint bug. It is not. Statically, the mapping is correct: the preview
board sets the 24-point to owner 0 and `Board.svelte` fills owner 0 with `checkerColors[0]`, i.e.
the colour beside the "Player 1" field. Under the identity orientation the top-right pair *is*
Player 1's colour.

The mismatch comes from one line: `const p1Top = flipV(orientation)`. Pressing **⇅ Mirror
top/bottom** puts `Orientation` into `P1HomeTop*`, which moves P1's home board to the top row,
P1's back checkers to the bottom, and the `P1` score label to the top. And that button was the
*only* gesture the UI offered for what the user actually meant — "no, the **other** player is the
near one". Both real `.lbg` sessions carry `"orientation": "p1-home-top-left"`; in one of them
`checkerA` had been hand-inverted to compensate.

ADR-0006 introduced the four-value enum deliberately, arguing that "competition footage puts
Player 1 on the near *or* far side, so a two-value prior (which implicitly pins P1 to the bottom
row) is insufficient". That argument is geometrically true and modelling-wise wrong: **which
physical player we call "Player 1" is a free naming convention, not a fact to be read off the
capture.** Pinning it costs nothing and removes a degree of freedom that only ever produced this
bug. The corpus agrees — all 23 committed manifest parts say `p1-bottom`, and `p1-home-top-*`
was never written by anything but the GUI's mirror button.

The left/right freedom is real and stays: the home boards can be in either half.

## Decisions

1. **Player 1 is, by definition, the player at the bottom of the video.** Player 2 is the top one.
   The two back checkers on the top row are therefore always Player 1's.

2. **`Orientation` drops to two values** — `P1HomeRight` (identity) and `P1HomeLeft` — naming
   which half holds the two home boards, i.e. the direction of play. `flipV`, `FlipVertical` and
   the two `P1HomeTop*` values disappear; `TransformPoint` reduces to `col → 11-col` and provably
   never exchanges the rows, which is what makes decision 1 an invariant of the renderer rather
   than a convention it has to remember. This supersedes ADR-0006 decision 1.

3. **The vertical mirror becomes "swap the two players"** — it exchanges names and declared
   colours, never geometry. On the opening position the two gestures produce very nearly the same
   picture (backgammon's near-symmetry: mirroring the board and exchanging the players are almost
   the same operation), which is exactly why they were confusable; the difference is that the
   bottom of the board keeps meaning Player 1. This amends ADR-0006 decision 4, whose WYSIWYG
   principle is otherwise unchanged: the rule is *shown* — the declared names are printed on
   their own side of the preview — not stated in a sentence the user must apply mentally.

4. **A swap is legal at any time, including mid-transcription**, and is a total, involutive
   operation on the session: names, per-Part checker colours, `turns[].player` and
   `results[].winner` (the source of the displayed score). Notation is stored **player-relative**
   (`internal/derive/derive.go`), so no move is rewritten — replaying the same notations with the
   players exchanged yields the mirrored position, which is the intent.

5. **Legacy `p1-home-top-*` documents migrate on load**: the vertical bit is dropped (keeping the
   home boards on their side) and the two players are exchanged. `ParseOrientation` keeps
   accepting every vocabulary this repo has written; `LegacyTopOrientation` reports the case that
   additionally needs the swap.

## Evidence

The corpus's `p1-bottom` prior carries no left/right information and falls back to *right*, so
"the manifests all say the same thing" would have been a weak argument. It was measured instead:
the shipped point reader predicted all 24 regions over 10 full turns per recording, scored against
the `.mat` truth under all **8 hypotheses** (4 dihedral × A/B colour swap).

```
identity wins, n>=60         19 recordings   identity 63-100%, runner-up <= 51%
sample too small              2   JacquesRavier (n=4), GillesDeshayes (n=19)
no crops                      1   vbc15_r1_SarahPartouche (not aligned)
```

Margins where there is material: NicolasPaoli 100.0 vs 19.5 · BaudryThierry 95.5 vs 18.0 ·
EricBenichou 94.5 vs 36.7 · hsbtMars2025 93.4 vs 12.4. The runner-up is almost always
`mirror-V+swap` — the measured form of the near-symmetry invoked in decision 3.

## Consequences

- `internal/bg`: two-value enum, a migration table covering `p1-right`/`p1-left`/`p1-bottom`/
  `p1-home-bottom-*`/`p1-home-top-*`/`""`, and a test asserting the rows are never exchanged.
- `internal/session`: `SwapPlayers()` and the document-level `swapPlayers`/`swapPlies` primitives;
  `Open` migrates legacy documents; `Setup.SwapPlies` carries the setup form's swap so the
  recorded play moves with the names. `rebuildFromDoc` is extracted from `Open` so both paths
  re-derive the board chain the same way.
- GUI: one mirror button plus a swap button; `Player 1 — bottom of the video ▼` field labels; the
  declared names printed on their own row of the preview; `p1Top` gone from the renderer.
- ADR-0006 is **partially superseded**: decisions 2 (boundary transform, never in the core) and 3
  (one closed enum, one transform, migrate the vocabularies) stand unchanged and are still what
  ten code files rely on.
- The two `rawvid/*.lbg` sessions migrate on first load; no manual step.

Relates to ADR-0005 (the renderer), ADR-0006 (superseded in part), and the domain model's
Capture Profile section.
