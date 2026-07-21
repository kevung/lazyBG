# UX spec — the review/transcription screen

Status: **stable draft, Pass 3 of N**. Answers *how it looks/feels*; *what it must do* is
`docs/functional-spec.md`, *how it's persisted* is `docs/session-format-spec.md`. Written in terms
of the locked toolkit (Wails v2 + Svelte, ADR-0002) — concrete components, not abstractions.

---

## 1. Framing (Pass 1)

Per `functional-spec.md` §1, there is **one screen**, not two: the manual-entry experience and the
automatic-pipeline review experience are the same UI, differing only in how many turns already
carry candidates when the human arrives at them. Everything below describes that one screen.

**Single window, four zones always visible** — video scrubber, move list, board render, review
queue — no step-by-step wizard, no screen switching mid-flow. Editor-style (closer to subtitling
software than a setup wizard): the video stays visible and playable at all times while the move
list/board/review queue stay in keyboard reach, matching the "never forces the video to stop"
requirement (`functional-spec.md` §4). Setup (Session Priors + Calibration, `functional-spec.md`
§3 step 1) is the one true exception — a blocking step before the four-zone screen appears, since
it's a one-time per-Part prerequisite, not part of the turn-by-turn loop.

## 2. Core turn-entry interaction (Pass 2)

Per turn: type the two dice digits → the ranked candidate list appears, one entry pre-highlighted
(top-ranked) → **↑/↓ (or J/K)** moves the highlight through the list → **Space/Enter** confirms the
highlighted candidate and auto-advances to the next turn (dice entry again). Typical turn: 2 digit
keys + 0-2 arrow presses + 1 confirm key, hands never leave the keyboard.

**Confirm variants, one modifier away, never the default:**
- **Space/Enter** — confirm the highlighted candidate, plain.
- **Shift+Space** (or equivalent) — confirm the highlighted candidate *and* open a `human-flagged`
  Review Item alongside it (`functional-spec.md` §4 self-flagged uncertainty).
- **A dedicated key** (e.g. "O" for Override) — opens free-entry manual input instead of picking
  from the candidate list, for the rare genuinely-illegal-move case (`functional-spec.md` §4,
  ADR-0001).

## 3. Turn navigation

**Tab / Shift+Tab** move to the next/previous candidate-tick turn *without* entering anything —
free movement (rewatch, step back before typing). Confirming a candidate (§2) already
auto-advances on its own; Tab is for moving without committing.

## 4. Retroactive editing

The move-list panel (always visible, §1) is clickable: clicking a past turn selects it for
edit — re-opens the same dice→candidates→confirm flow at that point, and the video jumps to its
tick. Not keyboard-optimized (mouse is fine — editing the past isn't the speed-critical forward
path). Any resulting cascade (`functional-spec.md` §5) surfaces as a badge/counter on the review
queue panel — never a popup or forced redirect (functional-spec.md's non-blocking decision).

## 6. Board render panel

Two sub-views side by side:

- The **reconstructed board** (rendered from `bg.Board` — confirms "what the app believes"),
  **orientation-aware**: it takes the `Orientation` prop and draws in the *same sense as the video*
  (tray on the same side, homes in the same corners) via the dihedral transform, so the two
  sub-views are directly comparable. It no longer hardcodes P1-home-bottom-right (ADR-0006).
- The **video frame at the current tick**, **cropped to the calibrated ROI** (the corner
  quadrilateral + a small margin, so the small bottom-right image is filled with the playing area,
  not background), carrying the **Perception Overlay** (domain model §3): layer 1 the calibration
  grid, layer 2 per-point occupancy tinted by confidence, layer 3 raw checker circles / dice pips
  — with toggles to declutter. The overlay recomputes only on a **stabilised frame** (pause /
  seek-settle / step, debounced, cached per tick); during continuous playback only the grid stays
  drawn (§ the CPU-only constraint). Exposing detections to the GUI needs a new thin
  `gui/app.go` binding (run the readers for a tick, de-project through the homography, return
  {grid, `ObservedBoard`, circles, dice}) — today `app.go` exposes only the plies-replayed board.

## 7. Review queue panel

A list of open `pipeline.ReviewItem`s (cascade-flagged or `human-flagged`), each clickable exactly
like a move-list entry (§4) — one resolution mechanism for the whole app, no separate review-mode
screen or key set. Selecting a Review Item jumps the video to its tick and re-opens the same
dice→candidates→confirm flow already used everywhere else.

## 9. Cube-decision entry

**"C"** opens the small fixed cube menu (no action / double / take / drop, filtered by who
currently holds the cube) — same grammar as everything else (arrows to move, Space to confirm),
just a separate entry point since a cube decision precedes a dice roll in the normal flow rather
than following it.

## 10. Setup screen & mid-session correction

A standard form (mouse, ordinary fields) for Session Priors, then calibrate the board on the
displayed frame — no keyboard optimization needed, this happens once per Part, not per turn. A
persistent **"Calibration…"** menu item/shortcut is reachable at any time during the four-zone
screen and re-opens this same form pre-filled with current values for correction
(`functional-spec.md` §3) — no separate mid-session-specific UI.

**Calibration is eight draggable handles** (ADR-0007), not four clicks — because the board reader
needs the bar located, not just the outer rectangle. A schematic + one line states what to place:
the **four corners of the playing surface** (outer triangle tips, *not* the wooden frame) and the
**four bar-edge points**. The four corners drag freely; the four bar points slide along the top and
bottom playing edges (so the bar stays a clean strip and its width is explicit). Handles are
pre-seeded (default corners, bar ~5% centred) so the user *adjusts* rather than places from scratch.
A **live preview draws both half-board grids** (24 cells) back onto the frame as the user drags — if
the cells don't sit on the real triangles, drag until they do. The live grid is the real safety net;
the schematic prevents the first mistake. Automatic corner detection, when added, only *seeds* the
handles — never the final word.

**Orientation is declared WYSIWYG, by mirroring** (ADR-0006) — not a dropdown. The
orientation-aware reconstructed board is shown beside the video frame with two **mirror buttons**
(horizontal / vertical = the four dihedral states); the user flips the synthetic board until it
coincides with the video (same colors in the same corners, home on the correct side). What they
set *is* the `Orientation`, and it simultaneously validates the checker-color assignment. This
replaces the old two-value "P1 bears off left/right" `<select>`.

## 11. Pass 3 checkpoint

Core interaction model, all four zones, and setup are now specified end to end. Remaining gaps are
lower-stakes visual/layout detail (exact panel sizes/positions, move-list row format, review-queue
badge visuals, video scrubber controls beyond play/pause) — implementation-level polish rather
than structural decisions, revisited if real ergonomics during the walking-skeleton milestone
demand it (mirroring how the UI toolkit itself was deferred to "real ergonomics, not up front").

