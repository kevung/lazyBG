# Perception detector survey — checker & dice reading

Focused follow-up to `video-analysis-survey.md`, triggered by the 2026-07-07 real-pixel spike
(see memory `perception-spike-findings`). The spike proved homography rectification works but
that **color-only** checker classification fails across heterogeneous boards (grey felt ≈ white
checkers; marbled swirl ≈ felt; white-on-white low contrast). This survey answers: *what detector
design should the board-state reader and dice reader actually use, CPU-only/offline?*

Method: 5-angle fanned web search → 22 sources → 96 claims → 3-vote adversarial verification;
22 claims confirmed, 2 refuted. Sources cited inline. (The automated synthesis step hit a session
limit; this write-up is hand-synthesized from the verified claim set.)

---

## 1. The headline: near-exact prior art exists, and it validates the plan

**`JvitorS23/backgammon-checker-detection-openCV`** implements *precisely* lazyBG's proposed
board-state pipeline: use the four board corners to find a homography, perspective-transform to a
top-down image "where the checkers are all circular," then **detect checkers with the Hough Circle
Transform** — not color segmentation.
[[JvitorS23]](https://github.com/JvitorS23/backgammon-checker-detection-openCV)

Its reported accuracy is the single most useful number in this survey:
> **90 % per-point (pip) checker-count accuracy, but only 40 % full-board-state accuracy.**

Two things follow, both important:
1. **Edge/shape circle detection is the right primitive** — it works without relying on color, so
   it survives the low-contrast (white-on-white) and marbled cases that broke our color reader.
2. **Per-point is easy; full-board-exact is hard.** This independently reproduces our own spike
   pattern (per-point plausible, full board garbage) and is *corroborated by a second, unrelated
   source*: an end-to-end deep chess model (ChessReD) achieves only **15.26 % exact full-board**
   configuration on real photos and the authors call that state-of-the-art (~7× prior work),
   "reflect[ing] the difficulty of the problem."
   [[ChessReD]](https://arxiv.org/pdf/2310.04086)

**Architectural vindication:** lazyBG must *not* chase full-board-exact from vision alone. The
whole multi-cue design — engine legality filter + dice cross-cue + fusion + human review queue —
exists precisely to convert "90 % per-point" into "correct match." The 40 %/15 % ceilings are the
strongest evidence yet that the fusion+review architecture is the right call, not a single
omniscient reader.

The other backgammon system, **`christiancorro/BackgammonCV`**, went fully learned: one YOLOv4
detector with 8 classes (dice `1..6`, `disk_b`, `disk_w`) reads checkers *and* dice together.
[[BackgammonCV]](https://github.com/christiancorro/BackgammonCV) But it trains on **digital board
screenshots** (Arkadium online platform), so it never faces felt texture, marbled/translucent
checkers, glare, or oblique angles — the exact heterogeneity that matters here.
[[Arkadium]](https://huggingface.co/datasets/ArkadiumInc/ArkadiumBackgammon) Useful proof that a
single learned detector *can* do both jobs; not proof it survives real footage.

---

## 2. Recommended board-state reader: **detect by shape, label by color**

The recipe, combining the prior art with our calibration leverage:

1. **Rectify** via the homography (already working).
2. **Detect checkers as circles per point region** with Canny → Hough Circle Transform. Crucially,
   the Circle Hough Transform operates on the **binary edge map, not raw color pixels**
   [[scikit-image]](https://scikit-image.org/docs/stable/auto_examples/edges/plot_circular_elliptical_hough_transform.html)
   — so detection is contrast/shape-driven and colour-independent. This is the fix for
   white-on-white and marbled sets.
3. **Exploit calibration for the radius prior.** Generic circle-detection papers struggle because
   they search a wide radius range; we *know* the checker diameter from the board geometry, so
   `minRadius`/`maxRadius` collapse to a tight band around one value — a large robustness win the
   literature doesn't have. (Threshold/radius is repeatedly cited as *the* critical parameter
   governing success. [[coin-CHT]](https://www.academia.edu/34236636/Detecting_Coins_with_Different_Radii_based_on_Hough_Transform_in_Noisy_and_Deformed_Image))
4. **Use colour only for ownership.** Once a disc is located, sample its interior and assign to the
   nearest **calibration-sampled** checker-colour centroid (A/B). Colour never has to separate
   checker-from-felt (the failure mode); it only splits checker-A-from-checker-B, an easy call.
   This is exactly the poker-chip recipe — *"chip counting based on colour segmentation combined
   with the Hough Circles Transform"*
   [[PokerVision]](https://web.fe.up.pt/~niadr/PUBLICATIONS/LIACC_publications_2011_12/pdf/C62_Poker_Vision_Playing_PM_LPR_LFT.pdf)
   and chip-stack patents that *"identify the chips ... using a Circle Hough Transform"* then
   *"detect the edges of the chips to separate out individual chips."*
   [[chip-patent]](https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/11948425)
5. **Count** = number of discs on the point; the along-axis run-length is a cross-check.

**CPU/offline is fine.** HoughCircles (Gaussian blur → grayscale → `HOUGH_GRADIENT`) runs live on a
mobile **ARM CPU** (Nexus 7) counting coins
[[coins-ARM]](https://wyattsmcall1.github.io/documents/FinalReportECE420.pdf); a modern
information-compression circle detector runs a full image in ~0.59 s on an i5 (vs 1.6–61 s for
Hough/RANSAC variants)
[[fast-circle]](https://www.mdpi.com/1424-8220/22/19/7267). Per *point region* (small crop) with a
known radius, cost is negligible.

**Honest caveats (verified):**
- Circle detection **degrades on cluttered/textured backgrounds**: F-measure 0.99 on clean
  geometry but **0.70** on a cluttered dataset [[fast-circle]], precision/recall 1.00→0.71 similarly
  [[PMC9572816]](https://pmc.ncbi.nlm.nih.gov/articles/PMC9572816/). Our textured felt + coloured
  point-triangles *are* that clutter — the radius prior and per-point cropping mitigate it, but this
  must be measured, not assumed.
- The claim that CHT is "robust to noise and missing points" was **refuted 0-3** by verification —
  do not rely on it for badly broken rims. Occlusion tolerance is bounded (occluded arc < 0.4 of
  circumference) [[PMC9572816]]. Overhead shingled stacks mostly satisfy this; oblique/occluded
  cases won't.

---

## 3. The reserved upgrade: a tiny per-point CNN (fed by .mat truth)

When shape+colour plateaus (the 40 % full-board ceiling, heavy clutter, oblique angles), the
survey strongly supports a **small per-cell classifier**, not a big detector:

- **`chess-cv`** classifies each square with a **SimpleCNN, ~156k params, 32×32 input**, at
  **99.90 %** per-square accuracy, trained **entirely on ~93,000 synthetic images** spanning
  **55 board styles**. [[chess-cv]](https://github.com/S1M0N38/chess-cv) This is the template:
  a per-point crop → tiny CNN → (count, owner) class. 156k params is trivial for CPU/ONNX.
- The **OOD lesson is the training data**, not the architecture: 55 styles → generalization. We get
  the equivalent for free — **.mat-derived ground truth** labels every point crop across ~35 real
  matches, and synthetic board rendering (already prototyped: `internal/perceive/boardsynth`) adds
  unlimited style variety. This is the auto-generated-training-set path the corpus was designed for.
- Contrast with **full-board one-shot** learned reading, which is genuinely hard (ChessReD 15 %
  [[ChessReD]]). The per-cell framing (chess-cv 99.9 %) is the winning decomposition — and it's the
  same per-point structure our reader already has.

When classical loses decisively: the poker-dealer robotics case found *"Hough lines, contour
analysis and edge detection had low success rates recognizing chips in stacks"* under variable
lighting/stacking and switched to a **learned instance-segmentation model (RF-DETR, 89 mAP)**.
[[roboflow]](https://blog.roboflow.com/computer-vision-robotic-poker-dealer/) That's the honest
signal for *when* to escalate: variable lighting + dense stacking. Backgammon overhead stacks are
more benign than side-view chip stacks, so classical has a better shot here — but the escalation
path is proven.

**Decision rule (board-state reader):** ship shape+colour (Hough-circle, calibration-derived radii,
colour-for-owner); measure per-point and full-board per corpus cell; if per-point < ~95 % or a cell
collapses, train the tiny per-point CNN on .mat-derived + synthetic crops for that regime. Both
feed the *same* per-point interface, so the swap is local.

---

## 4. Recommended dice reader: classical pip-blobs first, tiny CNN in reserve

Classical pip reading is mature, cheap, and well-attested:

- **median blur (glare) → grayscale/threshold → `SimpleBlobDetector`** (keep round blobs via
  inertia-ratio ≈ 0.6) → **cluster pip centroids (DBSCAN)** into individual dice → count pips per
  cluster. [[golsteyn]](https://golsteyn.com/writing/dice/),
  [[jwolle1]](https://github.com/jwolle1/Dice_Counter_OpenCV) A dice patent describes the same
  segment→bright-blob→validate-by-geometry recipe, using die-edge positions to disambiguate pip
  configuration. [[dice-patent]](https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/6609710)
- **Localization:** the `.mat` gives the *value* but not the *location*; at inference, constrain to
  a declared **dice-tray ROI** (Session Prior) or find the die by its bright face, then count pips
  inside. For training, value is labelled for free by the `.mat`.
- **Transparent/precision dice** remain the flagged risk. One fetched source *claimed* classical
  prior art for transparent dice exists (contradicting our earlier "no prior art"), but that claim
  did **not** pass into the verified set — treat as unconfirmed. Prototype on our own transparent-dice
  corpus cell; if pip-blobs fail, escalate.
- **Learned option:** dice detection+counting with a nano detector (YOLOv8n) is demonstrated, and a
  per-ROI tiny CNN is even cheaper. Since `.mat` supervises the value directly, a learned dice
  classifier is trivially trainable if classical stalls.

**Decision rule (dice reader):** classical pip-blob + ROI first; per-cell measure; escalate the
hard cells (transparent, glare) to a small learned dice-ROI classifier trained on `.mat`-labelled
crops.

---

## 5. Board-geometry calibration (the spike's other gap)

Our even-grid `DefaultCanonical` misaligned a real wide-bar board, scattering checkers onto phantom
points. The literature offer here is thin/indirect, so this stays an engineering task: move from
"4 corners + assumed even grid" to **geometry that matches the real board** — either declare/measure
the bar width and point pitch as calibration parameters, or fit the grid to the detected point
triangles. Note that the shape-first reader (§2) is *more forgiving* of small grid error than the
color reader was, because a Hough circle is found by its own rim wherever it sits in the crop, not
by a centreline hitting the right column — so improving geometry and adopting circle detection
compound.

---

## 6. Bottom line

- **Checker reader:** rectify → **Hough-circle detection with calibration-derived radii** → colour
  only to assign owner. Directly attested by backgammon + poker-chip prior art; CPU-cheap; survives
  low-contrast/marbled. Reserve a **tiny per-point CNN** (chess-cv-style, 156k params) trained on
  `.mat`-derived + synthetic crops for cells where classical plateaus.
- **Dice reader:** classical **blob-detect pips + DBSCAN cluster**, ROI-constrained; reserve a tiny
  learned dice classifier for transparent/glare cells.
- **Do not target full-board-exact from vision.** 90 % per-point is the realistic ceiling
  (JvitorS23); the engine-legality + fusion + review architecture is what makes that sufficient —
  and the survey's two independent low full-board numbers (40 %, 15 %) confirm that design.
- **Training data is a solved problem for us:** `.mat`-derived ground truth + `boardsynth` synthetic
  rendering give labelled per-point and dice crops at scale across styles — the exact ingredient
  (55-style synthetic training) that made the per-cell CNN generalize.

### Sources
Backgammon: [JvitorS23](https://github.com/JvitorS23/backgammon-checker-detection-openCV),
[BackgammonCV](https://github.com/christiancorro/BackgammonCV),
[Arkadium dataset](https://huggingface.co/datasets/ArkadiumInc/ArkadiumBackgammon).
Chips/coins: [Poker Vision (FEUP)](https://web.fe.up.pt/~niadr/PUBLICATIONS/LIACC_publications_2011_12/pdf/C62_Poker_Vision_Playing_PM_LPR_LFT.pdf),
[chip-count patent](https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/11948425),
[Roboflow poker dealer](https://blog.roboflow.com/computer-vision-robotic-poker-dealer/),
[coins on ARM](https://wyattsmcall1.github.io/documents/FinalReportECE420.pdf),
[coin CHT](https://www.academia.edu/34236636/Detecting_Coins_with_Different_Radii_based_on_Hough_Transform_in_Noisy_and_Deformed_Image).
Circle detection: [Fast circle / Sensors 2022](https://www.mdpi.com/1424-8220/22/19/7267),
[PMC9572816](https://pmc.ncbi.nlm.nih.gov/articles/PMC9572816/),
[scikit-image CHT](https://scikit-image.org/docs/stable/auto_examples/edges/plot_circular_elliptical_hough_transform.html).
Learned per-cell: [chess-cv](https://github.com/S1M0N38/chess-cv),
[ChessReD](https://arxiv.org/pdf/2310.04086).
Dice: [golsteyn](https://golsteyn.com/writing/dice/),
[jwolle1](https://github.com/jwolle1/Dice_Counter_OpenCV),
[dice patent](https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/6609710).
