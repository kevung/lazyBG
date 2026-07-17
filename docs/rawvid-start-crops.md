# Start-position crop harvesting (rawvid, no `.mat`)

How to grow the learned point reader's corpus on **new board families** that have
**no ground-truth transcription** — the `rawvid/` captures Kévin supplied
(machine-local, gitignored). The opening layout is fixed *a priori*, so labeled
crops are free once the board is calibrated and the pristine start frame is pinned
— no alignment, no `.mat`.

This is the recurring loop referenced in the pipeline notes; it complements the
truth-forced `align -crops` loop (which needs a `.mat`).

## Why a dedicated path

`align -crops` derives labels from an aligned `.mat`. rawvid captures have none.
But every match *starts* from the standard opening, and that opening is one of a
handful of board symmetries (identity / 180° / horizontal-mirror / vertical-mirror,
× which colour is CheckerA). Determine the symmetry once, pin a pristine frame, and
all 24 point labels are known.

## Tools (`cmd/`, dev-time, like `rectifydbg`)

- `cmd/colprofile` — rectify a manifest (with optional 8-number corner override)
  and report left-felt / bar / right-felt x-positions at mid-height. Use it to
  **tune the 4 corners** until the bar centers on the canonical bar and the two
  half-boards are equal width. All committed manifests share one canonical
  (`pointW 58, barGap 60, offW 24, …`); geometry adaptation is entirely in the
  corners.
- `cmd/gridoverlay` — draw the 24 `PointRegion` cells + bar on the rectified frame
  (sanity-check alignment visually).
- `cmd/cropmontage` — lay the 24 point crops in board arrangement (top p13..p24,
  bottom p12..p1) to read the position.
- `cmd/startcrops` — extract labeled crops from a **known** board at given ticks,
  reusing `align.ExtractCrops` (format parity with the `.mat` path). Board spec:
  `A:p=n,... B:p=n,...` where A = CheckerA = P1, B = CheckerB = P2. Validates 15+15.

## Procedure (per capture)

1. **Frame a candidate start** (`ffmpeg -ss`). Compare a few ticks — the standard
   opening only holds from setup-done until the first checker move.
2. **Tune corners** with `colprofile` until bar-centered + symmetric halves.
   Verify with `gridoverlay` / `cropmontage`.
3. **Determine the symmetry** by matching the observed stacks to a StandardStart
   symmetry (identity / 180° / h-mirror / v-mirror), and which colour is CheckerA.
4. **Pin a pristine window**: track a point that receives the opening move (e.g. an
   empty inner point) across ticks — it stays empty until the first move. Pick
   ticks strictly *before* that, and *before* a hand reaches in. Dice already on
   empty felt are fine (they occlude no stack).
5. **Extract** with `startcrops` over the pristine ticks. **Verify** every crop
   against its label with a labeled montage before keeping.
6. Crops land in `corpus/crops-rawvid/<id>/` (machine-local); feed the fine-tune
   alongside `corpus/crops/*` (consider oversampling the new family).

## Resolved capture: `rawvid_r8_RoosBucur` (Mattias Roos v Catalin Bucur, r8)

New family: **white-on-beige** checkers + **navy** checkers, beige felt,
orange/navy points, ~848×480 top-table. This is the family the blind reader read
as noise (felt≈white checker luma; low contrast).

- **Tuned corners** (TL,TR,BR,BL): `[[298,40],[712,35],[715,427],[296,431]]`.
  colprofile: bar center 395 (canonical 394), half-boards 361/358 — balanced.
  The prior blind corners (right edge ≈700) clipped the right bearoff tray and
  produced a trapezoidally-skewed rectification (left half 17% wider) → the earlier
  crops were mis-cut. **Redone.**
- **Orientation**: horizontal mirror of the standard opening, **CheckerA = white =
  P1** (matches the manifest prior). Verified start layout (canonical points):
  - White (A): `13=2, 24=5, 5=3, 7=5`
  - Dark  (B): `1=5, 12=2, 18=5, 20=3`
- **Pristine window**: ~4–12 s. The first checker reaches point 23 (dark) between
  14 s and 20 s; a hand occludes the left side at 14 s. Ticks 4/6/8/10/12 s are
  clean (two dice rest on empty felt, no occlusion).
- Extracted 5 frames × 24 = 120 crops (40 checker crops in the new family).

Command:

```
startcrops -manifest rawvid/r8_RoosBucur.manifest.json \
  -out corpus/crops-rawvid/rawvid_r8_RoosBucur -ticks 4000,6000,8000,10000,12000 \
  -board "A:13=2,24=5,5=3,7=5 B:1=5,12=2,18=5,20=3"
```

## Resolved capture: `rawvid_r8_KabodiKaraca` (Azad Kabodi v Ibrahim Karaca, r8)

New family: **blue/yellow points on grey felt**, cream (CheckerA `#ececec`) +
near-black (`#101014`) checkers.

- **Corners** (unchanged from the blind manifest — already bar-centered):
  `[[273,22],[731,20],[731,442],[274,444]]`. colprofile: bar center 397
  (canonical 396). Right half is ~7% wider than left (a mild residual the corners
  can't fully square), but every checker crop is captured cleanly — good enough.
- **Orientation**: **180° rotation** of the standard opening, **CheckerA = white =
  P1**. Verified start layout (canonical points):
  - White (A): `1=5, 12=2, 18=5, 20=3`
  - Dark  (B): `13=2, 24=5, 5=3, 7=5`
- **Pristine window**: narrow, ~20–22 s (abs). Setup hands occupy 4–18 s; the dice
  are rolled ~19–20 s (die rests on empty p19); a hand reaches in for the first
  move at ~22–23 s. Ticks 20/21/22 s are clean (finger just grazes empty p19-top at
  22 s — no checker occluded).
- Extracted 3 frames × 24 = 72 crops (24 checker crops in the new family).

Command:

```
startcrops -manifest rawvid/r8_KabodiKaraca.manifest.json \
  -out corpus/crops-rawvid/rawvid_r8_KabodiKaraca -ticks 19000,20000,21000 \
  -board "A:1=5,12=2,18=5,20=3 B:13=2,24=5,5=3,7=5"
```

## Resolved capture: `rawvid_r5_KaracaCiortan` (Ibrahim Karaca v Radu Ciortan, r5)

Family: **orange/black points on cream felt**, cream (CheckerA `#ececec`) +
navy checkers (a second SBGF orange-ish board, distinct camera from Roos).

- **Corners** (unchanged from the blind manifest): `[[285,25],[722,25],[724,435],[283,437]]`.
  colprofile: bar center 405 (canonical 396) — ~9 px right, left half slightly
  wider; every checker crop still captured cleanly. Acceptable; nudge later if a
  retrain wants it tighter.
- **Orientation**: **horizontal mirror** (same as Roos), **CheckerA = white = P1**.
  Verified start layout (canonical points):
  - White (A): `13=2, 24=5, 5=3, 7=5`
  - Dark  (B): `1=5, 12=2, 18=5, 20=3`
- **Pristine window**: ~22–28 s (abs). Two dice rest in the bar/on felt; hands
  reach in for the first move ~30 s. Ticks 22/24/26 s are clean.
- Extracted 3 frames × 24 = 72 crops (24 checker crops in the new family).

Command:

```
startcrops -manifest rawvid/r5_KaracaCiortan.manifest.json \
  -out corpus/crops-rawvid/rawvid_r5_KaracaCiortan -ticks 21000,23000,25000 \
  -board "A:13=2,24=5,5=3,7=5 B:1=5,12=2,18=5,20=3"
```

## Resolved capture: `rawvid_r8_BynellMoulton` (Johan Bynell v Miranda Moulton, r8)

Family: **grey felt, black/white points**, cream (CheckerA `#f0f0f0`) + **red**
(`#b03050`) checkers.

- **Corners** (unchanged): `[[272,28],[728,28],[730,412],[270,414]]`. colprofile:
  bar center 385 (canonical 386), halves 223/233 — well balanced.
- **Orientation**: **180°**, **CheckerA = cream = P1**:
  - Cream (A): `1=5, 12=2, 18=5, 20=3`
  - Red  (B): `13=2, 24=5, 5=3, 7=5`
- **Pristine window**: ~28–34 s (abs), dice resting on felt, no hands.
- 4 frames × 24 = 96 crops (32 checker crops).

```
startcrops -manifest rawvid/r8_BynellMoulton.manifest.json \
  -out corpus/crops-rawvid/rawvid_r8_BynellMoulton -ticks 27000,29000,31000,33000 \
  -board "A:1=5,12=2,18=5,20=3 B:13=2,24=5,5=3,7=5"
```

## Resolved capture: `rawvid_r7_MoultonKandirali` (Paul Moulton v Taner Kandirali, r7)

Family: **wooden board** (tan/brown), **yellow** (CheckerA `#e8c832`) + navy
(`#1a2030`) checkers. Low contrast — `colprofile`'s luma auto-detect fails here
(wood felt ≈ triangle luma); corners were tuned by eye with `gridoverlay`.

- **Corners** (re-tuned, right edge extended to un-clip the tray):
  `[[286,48],[735,46],[731,461],[288,462]]`.
- **Orientation**: **180°**, **CheckerA = yellow = P1**:
  - Yellow (A): `1=5, 12=2, 18=5, 20=3`
  - Navy  (B): `13=2, 24=5, 5=3, 7=5`
- **Pristine window**: wide — the opening is held static and hands-free from ~13 s
  to ~40 s. 4 frames × 24 = 96 crops (32 checker crops).
- **Caveat**: calibration isn't pixel-perfect on the low-contrast wood; a few
  *empty* crops adjacent to tall stacks (p19/p23/p2) carry a sliver of neighbour
  checker. The **occupied** checker crops are clean. Tighten the corners before a
  final retrain if the empties prove noisy.

```
startcrops -manifest rawvid/r7_MoultonKandirali.manifest.json \
  -out corpus/crops-rawvid/rawvid_r7_MoultonKandirali -ticks 14000,19000,24000,29000 \
  -board "A:1=5,12=2,18=5,20=3 B:13=2,24=5,5=3,7=5"
```

## Status: all 5 rawvid families calibrated

| capture | family | orient. | CheckerA | crops |
|---|---|---|---|---|
| RoosBucur | white-on-beige, orange pts | h-mirror | white | 120 |
| KabodiKaraca | blue/yellow on grey | 180° | cream | 72 |
| KaracaCiortan | orange/black on cream | h-mirror | white | 72 |
| BynellMoulton | grey, cream + red | 180° | cream | 96 |
| MoultonKandirali | wood, yellow + navy | 180° | yellow | 96 |

~456 crops total, ~152 of them checker crops across 5 genuinely new board families.
Next: retrain `train_pointreader.py` over `corpus/crops/*` + `corpus/crops-rawvid/*`
(held out by recording; consider oversampling the new families), export LZPN1, and
re-run a blind transcription per family to measure the lift.

**Method note learned across Roos + Kabodi:** the pristine window is bracketed by
(a) setup hands withdrawing and (b) the first-move hand reaching in — often only a
few seconds, sometimes with the dice already resting on empty felt (harmless). Pin
it by tracking an *inner* point that receives the opening move: it stays empty
until the first move, so its first non-empty tick is the upper bound.

The fine-tune (retrain the point reader over `corpus/crops/*` + the new
`corpus/crops-rawvid/*`, held-out by recording) is worth running once ≥2–3 of these
families are in, per the "need new locations, not more of the same" finding.
