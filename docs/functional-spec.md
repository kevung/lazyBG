# Functional spec — unified manual/automatic transcription

Status: **stable draft, Pass 4 of N** (grilling session in progress; may still gain detail as the
session-format and UX passes surface new functional requirements). This document is refined pass by
pass, from global framing to fine detail, per `CLAUDE.md` process. It captures *what* lazyBG's
transcription workflow must do; *how it looks/feels* is `docs/ux-spec.md` (Phase 2); *why* on
hard-to-reverse points is `docs/adr/`.

---

## 1. Framing (Pass 1)

**The core insight this spec is built on:** lazyBG does not have two tools (a "manual
transcriber" and an "automatic pipeline") that later converge. It has **one** tool, one data
model, one UI. What varies over time is only *how many turns the human has to fill in by hand*.

- A **Transcription** is always built from the same primitives already implemented in
  `internal/cue`, `internal/pipeline`, `internal/gate`, `internal/bg`: a `cue.MoveDecision` per
  turn, gated by `gate.Outcome` into auto-fill or `pipeline.ReviewItem`, accumulating into a
  `bg.Match`. `bg.Ply.Confidence` already anticipates a human-entered ply (`Confidence = 0`).
- **"Manual transcription"** is simply the degenerate case where **no automatic detector has
  produced a hypothesis yet**: every Turn Segment starts life as a `pipeline.ReviewItem` with no
  candidates from cues, and the human supplies the `cue.MoveHypothesis` directly. There is no
  second data format, no second code path for "the manual product" vs "the automatic product" —
  today's `bg.Match` / corpus manifest *are* the manual tool's output format, unchanged.
- As detector quality improves (dice cue, better board reader, cube cue, clock-hit), a growing
  fraction of Review Items arrive at the human **pre-populated** with ranked candidates instead of
  empty. The UI does not change; the amount of typing does.
- **Consequence:** every capability built for manual transcription (candidate ranking UI, fast
  keyboard confirm/override, retroactive edit-and-recompute, turn navigation) is *permanently*
  useful — it's the same screen the automatic pipeline's review queue uses, not throwaway
  scaffolding.

## 2. Motivating use case

The immediate driver: producing `.mat` ground truth for videos that currently have none, so they
can be added to the training corpus (`corpus/manifest/`, `docs/experiment-plan.md`). A completed
manual transcription session must therefore yield, without a separate step:

1. A `.mat` file (via the existing `matexport.Write`), and
2. A corpus manifest (`corpus.Manifest`: `Cell`, `Parts[]` with `Priors`/`Calibration`/`Span`,
   `Turns[]` with `{Index, Part, TickMs}`) — built *as the user works*, not derived after the fact
   by realigning (the way `lazybg align` does today for the case where a `.mat` already exists).

This reuses the manifest schema in `internal/corpus` verbatim; no new schema is introduced.

## 3. Session lifecycle (Pass 1 shape — refined in later passes)

1. **Setup (blocking).** Session Priors (clock present?, orientation, colors, players, match
   length, Crawford/Jacoby/Beavers, cube) + one-time Board Calibration (4-corner click per video
   Part, inheritable across Parts) — the same step the automatic pipeline already requires before
   it can run. This guarantees the manifest is always complete and consistent; there is no
   "finish calibration later" state to model. Priors/Calibration remain editable **at any later
   point** in the session (menu/shortcut) — correcting them only updates the Part's Calibration in
   the manifest; already-recorded Plies are untouched, since `bg.Ply` carries no geometry.
2. **Turn-by-turn entry.** The human moves turn to turn (see §4) entering dice + move (or a cube
   action) for each. Every confirmed entry is a `bg.Ply` with `Tick` set and `Confidence = 0`,
   appended to the growing `bg.Match`, and a corresponding `corpus.Turn{Index, Part, TickMs}`
   appended to the manifest.
3. **Autosave, continuous.** Every confirmed decision is persisted immediately (Transcription +
   manifest state + last video position). Closing and reopening the app resumes exactly where the
   user left off — a multi-hour tournament video will not be transcribed in one sitting, and nothing
   is allowed to be lost.
4. **Retroactive edit, any time.** The user may revisit and edit/insert/delete any already-entered
   decision, not just append at the end (see §5).
5. **Export.** `.mat` (`matexport.Write`) and manifest are always in sync with the in-progress
   state — there is no separate "finalize" transform; export is just "write current state."

## 4. Turn-entry ergonomics (functional requirements; exact keybindings are a UX-pass concern)

- **Navigation is assisted, content is not.** The already-reliable turn/commit segmentation
  (`internal/transcribe` `ReadEvents`/stable-window detection, clock-hit) feeds a list of
  candidate turn-boundary ticks the user steps through (next/previous turn) instead of scrubbing
  by hand to find boundaries. This is purely a "where to look" aid — it proposes no dice, no
  move, no board content. Board/dice reading stays 100% manual until those cues are good enough to
  contribute confidently (tracked as ordinary detector improvements, not a special "manual mode"
  concession).
- **Dice entry: two numeric keystrokes.** The two die values are typed directly (e.g. "5" then
  "3"), no mouse, no picker widget — matches the Jellyfish notation already used everywhere
  (`bg.Dice.String()`). The candidate move list (below) appears as soon as both digits land.
- **Checker moves: ranked-candidate entry, not free-text-first.** Once the user enters the dice
  for a turn, the app shows a short ranked list (top 5–10) of legal moves. **The ranking is the
  same fused score the automatic pipeline already computes** (`architecture.md` §4 Step 2 —
  `agree_boarddiff` + engine equity prior, `cue.MoveHypothesis`), not engine equity in isolation:
  when a board-diff/image cue has an opinion on the resulting position (even weakly confident —
  e.g. today's ~85% classical reader), it re-weights the candidates alongside the "near-optimal
  player" equity prior; with no usable image cue it gracefully degrades to equity-only ranking.
  This is the same unification principle as §1 applied one level deeper: manual entry doesn't
  discard whatever automatic evidence already exists, it just isn't *blocked* on evidence being
  strong. The user cycles/selects with 1–2 keystrokes and confirms with a single simple action
  (exact keys: UX pass). This is the primary path, tuned for speed against fast-moving video.
- **Entry never forces the video to stop.** The user can keep watching/playing while typing —
  there is no "pause to enter, resume to continue" requirement. Speed matters more than
  correctness-on-first-try: a wrong keystroke or a turn entered while distracted is cheap to fix
  later precisely *because* retroactive edit (§5) exists as the safety net. The interaction model
  optimizes for uninterrupted forward flow, not for getting every entry right the first time.
- **Manual override always available.** Real matches sometimes contain moves that are actually
  illegal (misplays that stand, or contested/unusual rulings). A free-entry escape hatch must
  always be one step away from the ranked-candidate list — never buried — so the human is never
  blocked from recording what genuinely happened. This does *not* weaken the "legality is a hard
  constraint" rule from `docs/domain-model.md` §6 — that rule governs the *automatic*
  board-diff/dice-inference path (reconciling pixels with physics); it does not apply to a human
  directly reporting an eyewitnessed event. Formalized as ADR-0001
  (`docs/adr/0001-legality-is-a-prior-not-a-wall-for-human-entry.md`); `docs/domain-model.md` §6
  now states the distinction explicitly.
- **Dance (no legal move): automatic, no candidate step.** Once dice are entered, if
  `engine.LegalMoves` returns empty for the current position, the app records `bg.Ply{CannotMove:
  true}` immediately — no candidate list is shown, no extra keystroke to "pick" cannot-move, since
  there is nothing to choose. The manual-override escape hatch (above) still applies for the rare
  case a human witnessed something the engine's dance determination contradicts.
- **Cube decisions: small fixed action set, not generated.** Unlike checker moves, cube actions
  are just `{no action, double, take, drop}` (already `bg.CubeAction`), filtered by cube
  ownership/whose turn it is — there's no combinatorial move generation to rank. They are
  presented directly as the small fixed set; no ranking engine call is required on this path
  (this removes the earlier-flagged need for a new `engine` cube-equity wrapper from the critical
  path — it may still be added later as a nice-to-have annotation, not a blocker).

- **Self-flagged uncertainty.** A dedicated confirm variant lets the user validate a turn while
  marking it "uncertain" (bad footage, occlusion, fast/ambiguous action) instead of only a plain
  confirm. The Ply is applied immediately — the human did commit to an answer, the video keeps
  moving — but a `pipeline.ReviewItem` (Reason `human-flagged`) is opened alongside it for a later,
  unhurried second pass (replay slowed down, zoomed, or just reconsidered with fresh eyes). This
  reuses the existing Review Item machinery for a case the Gate itself would never catch, since the
  entry *is* human-confirmed. See `docs/domain-model.md` §4 (Review Item) for the declared
  exception to its usual invariant.

## 5. Retroactive editing (cascade behavior)

Editing or inserting a decision at turn *k* can invalidate the recorded moves at turns *k+1..n*
(the board they were legal against no longer exists). Behavior:

- The app replays the board chain forward from turn *k* (`derive.ApplyNotation` in sequence).
- Any downstream turn whose recorded move is no longer legal on the recomputed board is **flagged
  as needing review** (demoted to a `pipeline.ReviewItem`, using the concept as it already exists
  for automatic low-confidence turns) — **nothing already entered is deleted or silently
  overwritten.**
- Turns whose recorded move is still legal on the recomputed board are left untouched.
- This means a single upstream correction can ripple into a handful of downstream Review Items,
  exactly the way an automatic detector's low confidence does today — same review queue, same
  resolution flow, no new domain concept.
- **Non-blocking.** A cascade never interrupts the user's current entry flow — the newly-flagged
  Review Items simply join the existing review queue (a discreet counter/badge reflects the
  growing count). The user resolves them whenever they choose, keeping control over their own
  scrubbing pace.

## 5b. Game and match boundaries

- **Game end: auto-detected, human-confirmed.** The app recognizes game end from the reconstructed
  board (one side's checkers all `Off`, or a recorded cube `Drop`) and pre-fills `GameResult`
  (winner, and gammon/backgammon — derivable from whether the loser bore off any checker) rather
  than making the user type it from scratch. The user confirms, or corrects for an unusual ruling.
  Same principle as dance-detection and legality: the engine/board-state always assists, never
  blocks.
- **Match end: same principle.** As soon as cumulative score reaches/exceeds `Match.Length`, the
  app flags it and offers to close out / export the session — one consistent mental model across
  game-end and match-end, not two different behaviors to learn.

## 6. Persistence & scope carried over from the existing domain model (no new decisions needed)

These already exist in `docs/domain-model.md` / `internal/corpus` and simply apply unchanged:
multi-Part (multi-video-file) matches, Game boundaries within a Match, Crawford/Jacoby/Beaver
rules as Session Priors, cube ownership/value across Games. Confirmed in scope for v1 unless a
later pass says otherwise.

## 7. Open items for later passes

- ~~Explicit domain-model.md note distinguishing "hard legality" (automatic inference path) from
  "ranked prior + override" (human-witnessed manual entry path).~~ Resolved: ADR-0001.
- Exact dice-value input mechanism (keys, layout).
- Exact keybindings for candidate cycling / confirm / manual-override entry.
- Whether/how calibration can be corrected mid-session if the user notices it's wrong after
  starting entry (currently: blocking upfront, no revisit path specified yet).
- Error/warning surfacing when a downstream Review Item is created by a cascade edit (does the
  user get pulled there immediately, or does it just join the queue?).
- `.mat`/manifest write cadence detail (every ply vs debounced) — a UX/perf concern, not a
  functional one.
- **The lazyBG session/working file format itself is out of scope here** — §3's "autosave" and
  "retroactive edit" already assume *some* persisted working state exists, but its concrete shape
  (a single file bundling Transcription + manifest + review/flag state + candidate-shown
  traceability, resumable and rich enough to be shared back as corpus feedback) is deliberately
  deferred to its own dedicated spec pass — see `docs/session-format-spec.md` (planned).
