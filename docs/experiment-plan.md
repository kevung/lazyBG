# Experiment & data-collection plan — lazyBG

How we turn real match footage + `.mat` transcriptions into a labeled corpus that both **measures**
the pipeline's robustness and **trains** its learned upgrades. Companion docs: `domain-model.md`
(the entities), `architecture.md` (the pipeline), `research/video-analysis-survey.md` (why the
detectors look the way they do). Target ~500 lines.

---

## 0. The reframing insight (why this is cheap)

A `.mat` transcription already encodes **the board state and the dice at every turn**: replay it
from the opening and you know, exactly, the position after each move and the dice that produced it.
So **board-state and dice labels are derived for free** from the transcription. The *only* label a
`.mat` lacks is **when** — the video tick each turn was committed.

Labeling therefore collapses to:

| Label | Cost | Source |
|---|---|---|
| Session Priors (clock?, colors, orientation, match length…) | cheap, once per Part | declared |
| Board Calibration (4 corners) | cheap, once per Part (fixed camera) | 4 clicks |
| Active Span (match begin/end within a Part) | cheap, 2 stamps per Part | scrubber |
| Board state + dice at each turn | **free** | derived from `.mat` |
| **Video tick of each turn** | the one real cost — 1 stamp/turn | scrubber (pilot), then semi-auto |

Everything downstream (per-cue eval, learned-model training crops) is generated from these.

---

## 1. Locked decisions

1. **Goal = validation AND training** from one labeled corpus.
2. **Tick alignment = pilot-manual → semi-auto.** Hand-label a few clips for gold ticks; use them
   to build/validate the **commit detector**; then semi-auto-align the rest (detector proposes
   ticks, `.mat` move-count constrains, human corrects).
3. **Multi-part model.** Recording = ordered Parts; per-Part Span + calibration + priors,
   inheritable when the setup is unchanged. Turns tagged `(part, tick)`. See `domain-model.md`.
4. **Variety = MVP-vertical-first, stratified** (clock + fixed camera + opaque checkers locked;
   vary board colors → angle/height → resolution/codec → dice type; casual/handheld/online later).
5. **Metric = effort-saved at bounded error** — auto-fill coverage, auto-fill error rate, review
   rate; auto-fill precision guarded above all; per-cue diagnostics behind it.
6. **Transcript format = `.mat` (Jellyfish)** — build one importer (ideas from
   `legacy_v0:frontend/src/utils/matchParser.js`).
7. **Labeling = purpose-built mini scrubber** — pre-loads the `.mat` move list, stamps each turn's
   commit tick in order, marks the Span and the 4 calibration corners, writes the manifest. Seeds
   the app's review UI.

---

## 2. Corpus variety matrix

The MVP vertical is **fixed**; we vary the high-risk axes *within* it, adding new verticals later.

**Fixed for the MVP tier:** competition footage, chess clock present, camera roughly fixed and
overhead-ish, opaque checkers, dice re-thrown each turn.

**Axes we deliberately sample (priority order, from the survey's risk register):**

| Axis | Levels (MVP tier) | Why (survey) |
|---|---|---|
| Board color scheme | ≥3 distinct (dark/light/wood; contrasting vs low-contrast points) | color-seg reader is scheme-sensitive |
| Camera angle / height | overhead, ~45°, low/oblique | rectification + stack-count degrade with obliquity |
| Resolution / codec | SD, HD, 4K; ≥2 containers | robustness across captures is first-class |
| Dice type | opaque → transparent/precision | **no prior art**; hardest cue |
| Lighting | even, directional, glare/shadow | segmentation + reflections |
| Multi-part | single-file and 2–3-part matches | new domain reality; exercise `(part,tick)` |
| Audio | raw table audio vs commentary/music over | commit audio-onset viability |

**Later tiers (post-MVP):** clock-absent, handheld/moving camera, casual/home boards, online/2D
screen recordings.

**Cell target:** ≥2 Recordings per occupied cell for the MVP tier; the matrix is sparse by
design — cover the corners and the highest-risk combinations first, not the full cross-product.
Log every cell we *don't* cover (no silent gaps — `CLAUDE.md` §7).

**Corpus source:** the BackgammonNews channel + any user-supplied matches with `.mat`. Large raw
videos stay under `corpus/` (gitignored), referenced by the manifest (URL/path + timecodes). Small
hand-labeled golden frames are committed under `testdata/`.

---

## 3. The data model (labels)

Full definitions in `domain-model.md`; the label-bearing shape:

```
Recording                      one match's video
  transcript: path to .mat     the move-level ground truth
  cell:       matrix labels     (angle, colors, resolution, dice, …) for per-cell reporting
  parts: [ Part … ]            ordered video files
     Part
       file:        path
       priors:      SessionPriors      (inherit: bool)
       calibration: 4 corners          (inherit: bool)
       span:        {beginMs, endMs}   active play region in this file
  turns: [ {index, part, tickMs} … ]   the per-turn alignment (the labeled ticks)
```

Match begin = `parts[0].span.beginMs`; match end = `parts[last].span.endMs`. Turn `k`'s frame is
`part[turns[k].part]` decoded at `turns[k].tickMs`.

---

## 4. Labeling protocol

### Pilot (manual)
For each Recording, using the **mini scrubber**:
1. Declare **Session Priors** and the matrix **cell** labels.
2. For each Part: mark the **4 calibration corners** (or inherit) and the **Active Span**.
3. Import the `.mat`; the scrubber shows the ordered move list. Play/scrub the video and **stamp
   the commit tick** for each turn in sequence (the clock-hit / dice-removed instant). The
   move-count is fixed by the `.mat`, so the tool tracks progress and flags miscounts.
4. Save the **manifest** (JSON).

### Scaling (semi-auto)
Once the **commit detector** exists (built + validated on pilot ticks): it proposes candidate
commit ticks for a new Recording; align the proposed sequence to the `.mat` move-count (a
monotonic 1-D alignment); the human only **corrects** mismatches in the scrubber. Each corrected
Recording adds gold ticks that further improve the detector.

### Labeling quality
- Ticks are the *commit instant* (clock hit), consistently defined, so they double as clock-hit
  training/eval labels.
- A second pass (or a different labeler) spot-checks a sample for tick consistency (± a few
  frames is fine; the pipeline reads a stable window around the tick, not the exact frame).

---

## 5. Ground-truth derivation (labels for free)

From `import(.mat)` → `bg.Match`, replay move-by-move (reusing `engine.LegalMoves` / move
application) to produce, per turn `k`:
- the **pre-Position** (board + on-roll + dice),
- the **dice** played,
- the **resulting board** (post-move),
- the **cube/score** state.

Combined with the labeled `(part, tick)` and the Part's calibration, this yields a **labeled
record** per turn:

```
turn k → {
  frame:       decode(part, tick)          # the settled commit frame
  rectified:   calibrate.Rectify(frame)    # canonical board
  boardTruth:  resulting board (from .mat) # per-point count+color labels
  diceTruth:   dice (from .mat)            # dice-value label
  commitTick:  tick                        # clock-hit label
}
```

This single record feeds **both** goals:
- **Validation:** compare the pipeline's `ObservedBoard` / dice / commit to the truth.
- **Training:** crop per-point regions → count/color samples; crop the dice region → value
  samples; the audio/visual window around `commitTick` → clock-hit samples.

*Caveat:* the `.mat` gives the dice **value** but not its **location** in the frame. For dice
training, either use a declared **dice-tray ROI** (Session Prior) or add a light one-time dice-box
annotation in the scrubber. Not needed for validation (value comparison suffices).

---

## 6. Metrics & evaluation protocol

### Primary — effort-saved at bounded error (per corpus cell)
Run the pipeline over a Recording and compare its Move Decisions to the derived truth:
- **Auto-fill coverage** = auto-filled turns / total turns.
- **Auto-fill error rate** = wrong auto-fills / auto-filled turns. **Guarded hardest** — a
  confident wrong move is worse than a review. Target: near-zero.
- **Review rate** = needs-review turns / total turns.
- **Net effort saved** ≈ coverage − (cost of catching auto-fill errors). Reported per cell so we
  see *where* robustness holds and breaks.

The gate threshold is tuned so auto-fill error stays under the bound; coverage is then whatever the
cues can safely deliver. Robustness = high coverage at bounded error *across cells*, not just on
easy footage.

### Secondary — per-cue diagnostics
- **Board-state:** per-point accuracy, full-board exact-match rate (survey's OOD metric).
- **Dice:** value accuracy (when visible), and how often legality-inference recovers unseen dice.
- **Commit / turn segmentation:** precision/recall of turn boundaries vs the labeled ticks; count
  error (extra/missed turns).
- **Calibration:** reprojection error of the labeled corners.
- **Match span (later):** begin/end tick error once a boundary detector exists.

### Protocol
- **Train/test split = hold out whole Recordings.** Never split a Recording's turns across
  train/test (frames within a match are highly correlated → leakage). Reserve a fixed set of
  Recordings, spanning cells, as a **test set** touched only for final evaluation.
- **Golden frames:** extract a few settled commit frames per pilot Recording into `testdata/` for
  fast per-cue unit tests (dice, board-state) independent of video decoding.
- **Regression:** the eval harness runs on the committed golden set in CI-style `go test`;
  full-clip eval is a manual/dev-time run over `corpus/`.

---

## 7. Manifest schema (JSON)

Committed under `corpus/manifest/` (the schema; the big videos are not committed). Sketch:

```json
{
  "id": "backgammonnews-2024-aachen-r3",
  "transcript": "corpus/mat/aachen-r3.mat",
  "cell": { "angle": "overhead", "colors": "wood-lowcontrast",
            "resolution": "1080p", "dice": "opaque", "audio": "commentary" },
  "parts": [
    { "file": "corpus/video/aachen-r3-part1.mp4",
      "priors": { "clock": true, "matchLength": 7, "checkerA": "#eee", "checkerB": "#111",
                  "orientation": "p1-bottom", "inherit": false },
      "calibration": { "corners": [[80,60],[900,40],[950,760],[30,700]], "inherit": false },
      "span": { "beginMs": 42000, "endMs": 1380000 } },
    { "file": "corpus/video/aachen-r3-part2.mp4",
      "priors": { "inherit": true }, "calibration": { "inherit": true },
      "span": { "beginMs": 3000, "endMs": 720000 } }
  ],
  "turns": [ { "index": 1, "part": 0, "tickMs": 51200 },
             { "index": 2, "part": 0, "tickMs": 58900 } ]
}
```

`internal/corpus` loads/validates this; `inherit: true` copies priors/calibration from the prior
Part. The schema is versioned.

---

## 8. Enabling infrastructure — build order

Each a worktree, TDD, merged to `main` (see the approved plan for detail):

1. **ffmpeg-backed `capture.Source`** — real frame decode behind the existing interface (bundled
   `ffmpeg` CLI via `ffmpeg-go`; `go-astiav` later for exact seek).
2. **`.mat` importer** — parse `.mat` → `bg.Match`; round-trips with the writer.
3. **Ground-truth derivation** — replay `bg.Match` → per-turn `(Position, dice, resulting board)`.
4. **Manifest schema + loader** (`internal/corpus`).
5. **Labeling mini-scrubber** (`cmd/lblbg` or an app mode).
6. **Eval harness** (`internal/eval` + `cmd/evalbg`) → the metric bundle per cell.
7. **`commit` detector** (audio onset + visual clock-ROI diff), supervised by pilot ticks →
   semi-auto alignment.
8. **Learned upgrades** (dev-time Python → ONNX → `onnxruntime_go`): board-state counter, dice
   detector, trained on derived crops.

Steps 1–4 are the pilot prerequisites; 5–6 close the loop; 7 unlocks scaling; 8 is the learned
tier the corpus was designed to feed.

---

## 9. Pilot (first milestone)

Build 1–4, then with the scrubber (5) hand-label **2–3 MVP-vertical Recordings**, derive labels
(3), and run the eval harness (6) for **baseline numbers**. Success = the derived board/dice labels
match reality on spot-checked turns, and we get a first coverage/error/review reading. This proves
the entire loop before scaling the matrix or building the detector.

---

## 10. Risks & open questions

- **Commit-tick definition drift** across labelers → fix a written convention (the clock-hit
  instant) and spot-check.
- **Dice localization for training** → declared dice-tray ROI vs light annotation (see §5).
- **Multi-part cuts mid-turn** → assume cuts fall between turns; flag any Recording that violates
  it rather than modeling partial turns.
- **`.mat` completeness** → the derivation needs cube actions, resigns, and the exact opening;
  validate the importer against gnubg/XG-produced `.mat` files.
- **Audio availability** → many clips have commentary/music, not raw table audio; the commit
  detector must not depend on audio alone (visual ROI fallback).
- **Corpus size vs statistical power** → per-cell N is small at first; report confidence honestly,
  grow the matrix before drawing strong robustness conclusions.
