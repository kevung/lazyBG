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

Two sub-views side by side: the **reconstructed board** (rendered from `bg.Board`, independent of
video quality — confirms "what the app believes") and the **raw video frame at the current tick**
(`capture.FrameAt`, for visual sanity-check against reality). Both already exist as primitives; no
new data-side work, just laying them out together.

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

A standard form (mouse, ordinary fields) for Session Priors, then click the 4 board corners on the
displayed frame — no keyboard optimization needed, this happens once per Part, not per turn. A
persistent **"Calibration…"** menu item/shortcut is reachable at any time during the four-zone
screen and re-opens this same form pre-filled with current values for correction
(`functional-spec.md` §3) — no separate mid-session-specific UI.

## 11. Pass 3 checkpoint

Core interaction model, all four zones, and setup are now specified end to end. Remaining gaps are
lower-stakes visual/layout detail (exact panel sizes/positions, move-list row format, review-queue
badge visuals, video scrubber controls beyond play/pause) — implementation-level polish rather
than structural decisions, revisited if real ergonomics during the walking-skeleton milestone
demand it (mirroring how the UI toolkit itself was deferred to "real ergonomics, not up front").

