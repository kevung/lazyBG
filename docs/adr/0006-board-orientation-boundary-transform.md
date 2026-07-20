---
status: accepted
---

# Board orientation is a boundary transform, never in the core model

## Context

A grilling session (2026-07-21) started from three GUI complaints — the reconstructed board
ignored a declared "bears off to the left"; the bottom-right video image showed no detected
elements; and it was unclear what to click for the 4 calibration corners. The root cause was a
under-specified domain model around **calibration + orientation**, not three isolated bugs.

Investigation found the orientation prior was **captured and persisted but never consumed**:
`Priors.Orientation` (`p1-right`/`p1-left`) flowed GUI → session → manifest and stopped there.
The two functions meant to apply it — `transcribe.obsFlipSides` and
`boarddiff.BoardFromObserved(sideA)` — were **dead code (zero call sites)**, and they only ever
addressed *checker-color* assignment, never the left/right/top/bottom *geometry* the user was
declaring. Meanwhile three incompatible orientation vocabularies coexisted with no enum or
validation: `p1-right`/`p1-left` (GUI `SetupPanel.svelte`), `p1-bottom` (corpus builder
`cmd/lazybg/main.go`), and prose ("which side / home direction / bar side") in the domain model.

The reconstructed-board renderer (`Board.svelte` + `lib/boardGeometry.js`) hardcodes one layout
(P1 home bottom-right) and never receives orientation as a prop.

## Decisions

1. **Orientation has four values, not two.** A backgammon board seen top-down has a dihedral
   symmetry; exactly four configurations preserve the "bar in the middle, two rows" structure.
   Competition footage puts Player 1 on the near *or* far side depending on the video, so a
   two-value left/right prior (which implicitly pins P1 to the bottom row) is insufficient. The
   canonical enumeration names each by **the video quadrant holding Player 1's home (inner) board**:
   `P1HomeBottomRight` (the identity / reference), `P1HomeBottomLeft` (horizontal mirror),
   `P1HomeTopRight` (vertical mirror), `P1HomeTopLeft` (180° rotation).

2. **Orientation is a boundary transform, never in the core.** The engine (`bg.Board`), the
   `.mat`, move inference, and the match-equity tables all use a **fixed canonical numbering**
   (P1 home = points 1–6, bottom-right) — the immutable gnubg reuse contract. Orientation is
   applied only at the **two edges**:
   - **Perception-in:** the board-state reader produces an `ObservedBoard` in camera-view
     canonical pixels; the inverse orientation maps each observed point to its canonical
     `bg.Board` number before it reaches Inference.
   - **Display-out:** `Board.svelte` receives the orientation and applies the dihedral transform
     so the reconstructed board is drawn **in the same sense as the video** (tray on the same
     side, homes in the same corners).
   Between the edges, `bg.Board` / engine / `.mat` remain orientation-agnostic.

3. **One closed enum, one transform, migrate the three vocabularies.** A single `Orientation`
   type (the four values above) is the sole representation. It owns the dihedral logic in one
   tested place — `Transform(canonicalPoint) → videoQuadrantPoint` and its inverse, including
   the **bar** (stays centered) and the **off / bearoff tray** (flips side with the home). The
   GUI strings and the corpus builder's `p1-bottom` are migrated onto this enum; `p1-right`/
   `p1-left`/`p1-bottom` are removed.

4. **The user declares orientation WYSIWYG, by mirroring.** Because the reconstructed board is
   now orientation-aware (decision 2, display-out), the calibration UI *is* that render: shown
   beside the video frame with two mirror buttons (horizontal / vertical = the four dihedral
   states). The user flips the synthetic board until it coincides with the video — same colors in
   the same corners, home on the correct side. What they set *is* the orientation, and it
   simultaneously validates the checker-color assignment (`checkerA`/`checkerB`). No abstract
   dropdown, no mental projection.

## Consequences

- A new `Orientation` type (likely `internal/bg` or `internal/session`) with an exhaustive
  dihedral transform test (all 4 values × the 24 points + bar + off, round-trip identity).
- `boarddiff.BoardFromObserved` / `transcribe.obsFlipSides` are replaced by (or fold into) the
  perception-in application of `Orientation`, addressing geometry *and* color together.
- `Board.svelte` gains an `orientation` prop; `lib/boardGeometry.js` applies the transform in the
  point→column/row mapping instead of the current hardcoded layout.
- `SetupPanel.svelte`'s two-value `<select>` is replaced by the WYSIWYG mirror control.
- Corpus manifests carrying `p1-bottom`/`p1-right`/`p1-left` need a one-time migration to the enum.
- Unblocks the reconstructed board matching the video and correct point-numbering under any camera
  setup; it does **not** by itself add the perception overlay (see the domain model's **Perception
  Overlay** concept and its own ticket).

Supersedes the inert `Priors.Orientation` string. Relates to ADR-0005 (two.js board renderer,
which the display-out transform extends) and the domain model §Board / §Capture Profile updates.
