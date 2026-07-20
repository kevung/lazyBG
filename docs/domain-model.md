# Domain model — lazyBG

The **ubiquitous language** of lazyBG: the nouns and verbs that code, tests, docs, and
conversation all share. If a concept isn't named here, name it here before you build it. Target
length ~500 lines (`CLAUDE.md` §7). Companion docs: `architecture.md` (how these interact),
`research/video-analysis-survey.md` (why the perception concepts look the way they do).

Two locked decisions shape this model:
- **Calibrated-classical-first MVP** — the perception layer leans on user-declared **Session
  Priors** and a **one-time Board Calibration** rather than fully-automatic recognition (the
  survey showed automatic readers collapse on heterogeneous footage).
- **Multi-cue probabilistic fusion** — independent **Cues** are fused, under a hard engine
  legality filter, into a **Move Decision** with a calibrated **Confidence**.

---

## 1. The map (bounded contexts)

lazyBG has four cooperating vocabularies. Keep them distinct; translate at the seams.

```
  CAPTURE            PERCEPTION              INFERENCE              BACKGAMMON CORE
  (the video)   →    (pixels → evidence) →   (evidence → moves) →   (the game being rebuilt)
  Capture            Cue / Detector          Move Hypothesis        Match / Game / Turn
  Frame / Tick       Stable Board State      Move Decision          Move / Position / Board
  Capture Profile    Commit Event            Confidence / Gate      Dice / Cube / Score
  Board Calibration  Observed Board          Review Item            Engine (legality + prior)
                                                                    │
                                        TRANSCRIPTION (aggregate) ──┘→ Export (.mat)
```

- **Capture** context owns everything about the source video and the user's declared knowledge.
- **Perception** context turns pixels into typed, confidence-bearing **evidence** — and knows
  *nothing* about legality or match scoring.
- **Inference** context fuses evidence into decisions — and knows *nothing* about pixels.
- **Backgammon core** is the pure game model — no video, no confidence. It is the reused
  legacy vocabulary and the engine's world.

This separation is a testing strategy as much as a design one: perception is tested on golden
frames, inference on synthetic cues, core on pure game logic (`CLAUDE.md` §7).

---

## 2. Capture context

### Capture
The source video — one video file. Immutable input. Identified by a content hash + path.
Attributes: container/codec, resolution, aspect ratio, frame rate, duration, and (optionally) an
**audio track**. Everything downstream is keyed by **Tick**.

### Recording & Part (multi-part captures)
A **Recording** is one match's video as an **ordered list of Parts** — a match may be filmed in
several files (a mechanical split of one continuous setup, or separate resumptions where the
camera/board may have shifted; both occur). Each **Part** wraps one Capture with its **own** Active
Span, Board Calibration, and Session Priors — each **inheriting** from the previous Part when the
setup is unchanged. A single-file match is just a Recording with one Part. The **Transcription**
attaches to the Recording; every turn is tagged **(part, tick)**.

### Active Span
The `[begin, end]` interval within a Part where match play actually happens — matches rarely start
at video-start (players talk / set up) and Parts have dead ends. **Match begin** = the first Part's
span-begin; **Match end** = the last Part's span-end. In the corpus these are user-labeled;
auto-detecting them (first roll / final bear-off / handshake) is a later, evaluated capability.
*Rule: perception and export ignore everything outside the Active Spans.*

### Tick (video timecode)
The canonical time coordinate — a precise position in the Capture (frame index and/or
milliseconds). Every piece of evidence, every Move Decision, and every review item carries the
**Tick** at which it occurred, so the user can always jump to the exact video moment. Ticks are
the join key across all contexts. For a multi-part Recording a Tick is a **Recording-global
coordinate `(part, offsetMs)`**. *Rule: a Tick is meaningless without its Capture.*

### Frame
The decoded image at a Tick. Frames are transient (decoded on demand, not all held in memory).
A **Stable Frame** is one selected because the board ROI was motion-free for a window around it.

### Playback Proxy
A webview-playable copy of a Capture, produced on demand by the bundled ffmpeg **only when** the
OS webview cannot decode the original's codec/container. The GUI's `<video>` plays the Proxy — or
the original directly when it is already playable; **perception always decodes the original
Capture, never the Proxy.** The Proxy is a cache artifact keyed by the Capture's content hash
(gitignored, machine-local, cleanable), never authoritative. *Invariant: a Proxy must **preserve
the original's timeline** — same start, same duration, monotonic timestamps — so a Tick read from
the Proxy equals the Tick on the Capture. Prefer stream-copy remux (`ffmpeg -c copy`, exact);
re-encode only as a fallback, never resampling frame rate nor trimming. Duration parity is checked
on open and a mismatch is surfaced.* See ADR-0004.

### Capture Profile (Session Priors)
**First-class, user-declared constants** that seed and constrain the pipeline — the single
biggest robustness lever (survey §0, §12). Declared once at transcription setup, editable later.
Fields (all optional; each present prior tightens perception and raises baseline confidence):
- **Clock present?** and its rough on-screen location (drives Commit-Event detection).
- **Board orientation** — a closed 4-value **`Orientation`** enum naming the video quadrant that
  holds **Player 1's home (inner) board**: `P1HomeBottomRight` (canonical reference),
  `P1HomeBottomLeft`, `P1HomeTopRight`, `P1HomeTopLeft`. This single fact fixes both the bearing
  direction (left/right) *and* near/far (top/bottom row) — competition footage puts P1 on either
  side. It is a **boundary transform**, never in the core model: applied at perception-in
  (observed point → canonical `bg.Board` number) and display-out (rendering the reconstructed
  board in the video's sense). One `Transform()` owns the dihedral logic (bar stays centered, off
  tray flips with the home). See ADR-0006. *(Supersedes the old `p1-right`/`p1-left` /
  `p1-bottom` strings.)*
- **Board color scheme** — surface, point colors, checker colors (drives color-segmentation).
- **Players** — names and checker colors (Black/White mapping), for the `.mat` metadata.
- **Match length** — target score (e.g. 7-point match), and rule flags (**Crawford**, **Jacoby**,
  **Beaver**).
- **Camera fixed?** — if yes, one Board Calibration serves the whole Capture.
- **Cube in use?** and its side/home.

*Invariant: a Session Prior is a claim by the user, treated as high-confidence but not infallible;
perception may flag contradictions (e.g. checkers detected where the declared color says none).*

### Board Calibration
The mapping from Capture pixels to the **canonical rectified board** (a top-down, axis-aligned
coordinate system with fixed positions for the 24 points, bar, and bearoff trays). In the MVP the
user clicks the **four corners of the playing surface** — the quadrilateral enclosing the 24
triangles *with the bar included at the middle* (one single rectangle), i.e. the outer tips of the
corner triangles — **not** the outer wooden frame. The canonical grid places the points as
fractions of that rectangle, so clicking the wooden border shifts every point. The corner clicks
are ordered (top-left, top-right, bottom-right, bottom-left, camera view); as a non-rectangular
quadrilateral they also encode the perspective distortion the homography corrects. lazyBG computes
one **homography** from them, reused for every Tick. Attributes: source corner points, homography
matrix, the canonical grid definition, and a validity confidence. It is held **per Part** and
inherits from the previous Part when the camera is unchanged. Later: semi- or fully-automatic
calibration (survey §1). *Rule: board reading is undefined without a Board Calibration.*
*UX corollary: the calibration screen must say what to click (playing surface, bar included, not
the frame) and immediately draw the derived grid back onto the frame as validation — see the
**Perception Overlay** concept and `docs/ux-spec.md` §10.*

---

## 3. Perception context

Perception converts Frames into **evidence** — never into moves. Each detector is small and
independently testable against golden frames.

### Cue (Detector output)
One typed piece of evidence, with a **Confidence** and a **Tick**. A Cue asserts something a
single detector observed; it may be wrong, partial, or absent. Cue kinds:
- **Turn-End / Commit Cue** — "a turn boundary occurred here" (from clock-hit, dice-removed, or
  board-stability). Anchors segmentation.
- **Dice Cue** — "the dice show `d1,d2`" (or *absent* when dice not visible).
- **Board-State Cue** — an **Observed Board** at a stable instant (per-point occupancy + color +
  count, each with per-point confidence).
- **Board-Diff Cue** — "between these two stable boards, these checkers moved" → candidate
  move(s).
- **Cube Cue** — "the cube shows value V on side S" or "a double occurred here".
- **Engine Cue** — legality verdict + move-ranking prior (from the gnubg Engine).

*Rule: a Cue is immutable evidence. Detectors emit Cues; the Fusion consumes them. Detectors never
consult each other (independence is what makes agreement meaningful).*

### Detector
The producer of a Cue — a small unit (clock-event, dice-value, board-state reader, board-diff,
cube-reader). Each Detector has a stable input contract (a Frame or Frame pair + Board Calibration
+ relevant Session Priors) and a typed Cue output. Detectors are the golden-frame unit-test
surface.

### Stable Board State / Observed Board
The board occupancy at a **stable instant**: for each of the 24 points, plus **bar** and **off**
for each color, an estimated **count** and a **per-point confidence**. Produced by the
board-state Detector on a Stable Frame via the Board Calibration. It is an *observation*, not a
validated Position — it may be illegal or internally inconsistent (that's the Inference layer's
problem to resolve). *Invariant: an Observed Board records what was seen and how sure, nothing
more.*

### Perception Overlay
The **visual back-projection of perception evidence onto the Capture frame** — a read-only,
GUI-facing view, not a new detector. For a given (stable) Tick it draws, in **Capture pixel
space** (detections de-projected through the Board Calibration homography, `boardsynth.WarpToSource`
machinery), three layers over the video frame cropped to the calibrated ROI:
1. **Calibration grid** — the board quadrilateral + the 24 point cells + bar + off, derived from
   the corners (not detected). Shows *where the app believes each point is* → validates the corner
   clicks. Cheap geometry; the only layer kept live during playback.
2. **Per-point occupancy** — the `ObservedBoard` count + color per point, tinted by per-point
   confidence (uncertain = flagged).
3. **Fine detections** — raw checker circles (`checker.Detect`) and dice pips (`dice.ReadPips`),
   for detector debugging.
It runs **only on a stabilised frame** (pause / seek-settle / step, debounced, cached per Tick),
never per-frame during continuous playback — perception cares about **Stable Board States**, and
CPU-only is a locked constraint. *Rule: the overlay is a projection of existing evidence; it adds
no state to the Transcription and never edits the game.* See `docs/ux-spec.md` §6.

### Commit Event
The anchor that defeats the **"players try variations before deciding"** problem. A Commit Event
marks turn-end (clock hit / dice removed & re-thrown / long board-stability). **Only the last
Stable Board State before a Commit Event counts** as that turn's committed post-move position;
intermediate fiddling is ignored by construction. Attributes: kind, Tick, confidence, and the
Cues that support it. *Rule: perception never has to "understand" experimentation — it anchors to
the commit and takes the final stable state.*

### Turn Segment
The span between two consecutive Commit Events: a **pre-roll Stable Board State** (start) and a
**post-move Stable Board State** (end), plus any Dice/Cube Cues observed within. The Turn Segment
is the unit handed to Inference. *Invariant: a Turn Segment pairs exactly two stable boards to
diff.*

---

## 4. Inference context

Inference fuses Cues into decisions. It knows game rules and confidence — never pixels.

### Move Hypothesis
A candidate `(Dice, Move)` for a Turn Segment, with a **Confidence** and provenance (which Cues
support it). Multiple Hypotheses compete per turn. Hypotheses arise from the Board-Diff Cue
constrained by legality; when dice were unseen, from **inferring the dice set that makes the diff
legal** (often unique — survey §3); when the diff is ambiguous, from the Engine prior + dice.
*Rule: every Hypothesis must be legal given its dice (hard filter) — illegal candidates are
discarded before scoring.*

### Fusion
The step that correlates independent Cues into ranked Move Hypotheses with a **joint Confidence**.
Interpretable-first: a **hard legality filter** (Engine) + a transparent weighted/Bayesian
combination of the soft Cues (`CLAUDE.md` §3.4); **Dempster–Shafer / Dynamic Belief Fusion** is
the evaluated upgrade (survey §7). Agreement between independent Cues raises Confidence; conflict
lowers it. *Invariant: Fusion is pure — (Cues → ranked Hypotheses) with no side effects; this is
the synthetic-input test surface.*

### Move Decision
The chosen outcome for a turn: the top **Move Hypothesis**, its **joint Confidence**, its **Tick**,
and the ranked runner-up candidates. A Move Decision is either **auto-filled** (Confidence ≥
threshold) or **needs-review** (below threshold, or a hard conflict). *Rule: a Move Decision always
carries its Tick and its top-K alternatives so review is one keystroke.*

### Confidence
A calibrated scalar in [0,1] attached to Cues, Hypotheses, and Decisions. **Calibration is an
open problem** (survey §7): until labeled data exists, thresholds are deliberately conservative so
the system **over-refers to review** rather than silently auto-filling a wrong move. Later:
temperature scaling / reliability diagrams from accumulated transcriptions.

### Gate
The policy mapping a Move Decision's Confidence to **auto-fill** vs **needs-review**. A single,
inspectable threshold policy (may vary by Session Priors — e.g. stricter when the cube is live).
*Rule: the Gate is where product tuning lives; it is pure and unit-tested.*

### Review Item
A queued needs-review Move Decision: the Tick (jump-to-video), the pre-ranked top-K candidate
`(dice, move)`s, the conflicting Cues, and the reason it was flagged. The human confirms a
candidate or enters a correction; the resolved move flows back into the Transcription — and
becomes **labeled training data** for the future learned fusion/readers. *Invariant: resolving a
Review Item is the only way a low-confidence turn enters the final Transcription* — with one
declared exception: a human can flag their **own** freshly-entered turn as uncertain (bad footage,
occlusion). That Ply is applied to the Transcription immediately (it isn't "low-confidence" by the
Gate — it's a confirmed human entry) *and* a Review Item is opened alongside it, Reason
`human-flagged`, purely to invite a second look later. See `docs/functional-spec.md` §4.

---

## 5. Backgammon core

The pure game model — reused legacy vocabulary, and the Engine's world. No video, no confidence.

### Match
A contest to a target **Score** (e.g. 7-point match). Owns: the two **Players** (names, checker
colors), the target length, rule flags (**Crawford**, **Jacoby**, **Beaver**), and an ordered list
of **Games**.

### Game
One game within a Match, from opening roll to a win worth 1× (single), 2× (**gammon**), or 3×
(**backgammon**), scaled by the **Cube** value. Owns an ordered list of **Turns**, the winner, and
the points awarded. The **Crawford Game** is the first game after a player reaches
`target − 1` and is played with the cube disabled; games after it are **post-Crawford**.

### Turn (Ply)
One player's action: a **Dice** roll followed by a checker **Move**, and/or a **Cube Action**
(double / take / drop / beaver). Attributes: player on roll, dice, the move, any cube action, and
the originating **Tick** + **Confidence** (the bridge back to perception). *Invariant: turns
alternate players except across a double/take, which does not change who is on roll.*

### Move
The checker play for a Turn, in standard notation: a set of checker relocations like `24/13`,
`bar/21`, `13/7*` (hit), `6/off` (bear-off). A Move is legal *only* relative to a **Position** and
**Dice**. *Rule: legality is decided by the Engine, not re-implemented ad hoc.*

### Position
The complete game state at an instant: the **Board**, the **Cube** (value + owner), the **Score**,
who is on roll, and the pending **Dice**/decision type. This is the structure handed to the Engine.
Distinct from an **Observed Board**: a Position is validated and complete; an Observed Board is a
noisy perception.

### Board
Checker layout: 24 **Points** (each holding N checkers of one color), the **Bar** (per color), and
**Off**/bearoff (per color). Points are numbered 1–24 from a player's perspective; **the core fixes
one canonical numbering (P1 home = 1–6) and never varies it** — the gnubg reuse contract. The
**`Orientation`** enum (Capture Profile) translates only at the edges (perception-in, display-out),
never here (ADR-0006). Colors: **Black** and **White** (with the Player↔color mapping declared in
the Capture Profile).

### Dice
The two dice for a Turn (`d1,d2`, each 1–6). **Doubles** yield four moves of the same pip value.
Dice may be **observed** (Dice Cue) or **inferred** (from legality when unseen).

### Cube (videau / doubling cube)
The doubling cube: a **value** (1 centered, then 2/4/…/64) and an **owner** (which player may next
double, or centered). **Cube Actions**: double, take, drop (pass), and **beaver** (money-play
immediate redouble). Governed by **Crawford** (no cube in the Crawford game) and **Jacoby**
(gammons/backgammons score as singles unless the cube has been turned). *Rule: cube state affects
scoring and the Engine's cubeful evaluations; the MVP may treat cube actions as review-heavy since
they are rare, discrete events.*

### Score
Points accumulated by each Player toward the Match target. Updated when a Game ends: `game value =
win multiplier (1/2/3) × Cube value`, subject to Crawford/Jacoby. Drives the Match-equity context
that the Engine reads.

---

## 6. The Engine

The salvaged pure-Go **gnubg** port (`gnubg/`, `CLAUDE.md` §8). Two roles, sharply distinguished:
- **Legality = hard constraint — on the automatic path only.** Given a Position + Dice, the Engine
  enumerates the *legal* moves. Fusion (reconciling pixels with physics) discards any Hypothesis
  not in this set. This is the cheapest, strongest cue and it is already in hand. **This filter
  does not bind human-witnessed entry**: a person directly recording what happened on video may
  have seen a move that was actually illegal (a misplay that stood, a disputed ruling) — real
  matches contain these, and the Transcription must still be able to record them. See ADR-0001.
- **Move-ranking = soft prior.** The Engine scores legal moves by equity (win/gammon/backgammon
  probabilities). Strong players usually play near-top moves, so ranking is a *prior* over which
  legal move was played — **never** an override of direct visual evidence (a blunder must still be
  transcribable), and, for human entry, never a requirement (the ranked list is the fast default
  path; a free-entry escape hatch is always one step away, unbound by legality). *Rule: the Engine
  informs Confidence; it never fabricates a move the pixels contradict, and never blocks a human's
  direct report of what they saw.*

Interface (existing): `gnubg.Init(fs.FS)`, then `gnubg.FindMoves(board, dice, player, scoreMoves,
cubeful)` returns moves ranked by equity. See `analysis.go` on `legacy_v0` for the board-coordinate
translation seam (the trickiest integration point).

---

## 7. Transcription (aggregate root) & Export

### Transcription
The whole match being rebuilt from a **Recording** — the **aggregate root** tying every context
together. Owns: the Recording (its ordered Parts, each with a Capture Profile and Board
Calibration), the ordered **Move Decisions** (auto-filled + human-resolved), the **Review queue**,
and Match metadata. A Transcription is the editable working document; it is *not* the export
format. *Invariant: every move in a Transcription is either auto-filled above threshold or
human-resolved — nothing enters silently below the Gate.*

### Export (.mat / Jellyfish)
The canonical output (`CLAUDE.md` §3.2): a **`.mat` (Jellyfish) / `.txt`** match file readable by
gnubg, XG, and BGBlitz. Export maps the Transcription's Games/Turns/Moves/Cube actions + Match
metadata into the Jellyfish text layout. *Rule: the `.mat` is a projection of the Transcription;
round-tripping (import a `.mat`, re-export) should be stable.*

### Ground-Truth Derivation
The reverse direction, used to build the labeled corpus (`experiment-plan.md`): **import** a `.mat`
and **replay** it move-by-move to reconstruct, per turn, the exact **Position**, **Dice**, and
resulting **Board**. A `.mat` thus supplies board-state and dice labels *for free* — the only label
it lacks is **when** (the turn's Tick). Paired with a labeled Tick and the Part's calibration, each
turn yields a `(commit-frame, board-state, dice, commit-tick)` record that feeds both **validation**
(compare to what perception read) and **training** (crops for learned readers).

### Corpus / Manifest
The **Corpus** is the collection of labeled **Recordings** used to measure and train the pipeline.
Its **Manifest** is the committed JSON index — Recordings → Parts (file, span, priors, calibration
corners, inherit flags) → transcript ref → per-turn `(part, tick)` labels + matrix **cell** tags.
Large raw videos stay gitignored under `corpus/`; small hand-labeled golden frames are committed
under `testdata/` (`CLAUDE.md` §7).

---

## 8. Lifecycle — one turn's journey

```
Commit Event (clock hit) ──anchors──► Turn Segment
   pre-roll Stable Board ─┐
   post-move Stable Board ─┴─► Board-Diff Cue ─┐
   Dice Cue (or absent) ───────────────────────┤
   Cube Cue (if any) ──────────────────────────┼──► FUSION ──► ranked Move Hypotheses
   Engine (legality filter + ranking prior) ───┘        │        (each legal, scored)
                                                         ▼
                                                  Move Decision (+joint Confidence, Tick)
                                                         │
                                     Gate: Confidence ≥ threshold?
                                        ├─ yes ─► auto-fill ─► Transcription
                                        └─ no  ─► Review Item ─► human ─► Transcription
                                                                              │
                                                                       Export .mat
```

---

## 9. Invariants & rules (quick reference)

1. Every observation, decision, and review item carries a **Tick** (+ its Capture).
2. **Session Priors** are trusted-but-checkable; contradictions are surfaced, not ignored.
3. Board reading requires a **Board Calibration**; the MVP calibrates once (fixed camera).
4. **Detectors are independent**; agreement between them is what raises Confidence.
5. Only the **last Stable Board State before a Commit Event** is a turn's committed position.
6. Every **Move Hypothesis** is **legal** given its dice (hard Engine filter) before scoring.
7. The Engine ranks but **never overrides** visual evidence (blunders must be transcribable).
8. **Confidence** is calibrated conservatively; when unsure, **defer to review**.
9. Nothing enters the **Transcription** below the Gate without human resolution.
10. The **`.mat`** export is a projection of the Transcription; round-trips are stable.

---

## 10. Naming conventions for code

Use these exact terms in package, type, and test names so code reads like this document:
`Capture`, `Tick`, `CaptureProfile` (a.k.a. Session Priors), `BoardCalibration`, `Orientation`,
`Cue`, `Detector`, `CommitEvent`, `StableBoardState` / `ObservedBoard`, `PerceptionOverlay`,
`TurnSegment`, `MoveHypothesis`, `Fusion`, `MoveDecision`, `Confidence`, `Gate`, `ReviewItem`,
`Transcription`, and the core
`Match / Game / Turn / Move / Position / Board / Dice / Cube / Score`. When a name here disagrees
with a legacy identifier, **this document wins**; port the legacy name deliberately.
