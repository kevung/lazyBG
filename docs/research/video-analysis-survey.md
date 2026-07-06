# Deep-research survey — video/image analysis techniques for backgammon transcription

**Status:** complete (first pass, 2026-07-06). **Length rule:** exempt (unbounded — this is the
project's reference survey).
**Method:** automated multi-source deep-research harness — 6 search angles, 27 sources fetched,
112 candidate claims extracted, 25 adversarially verified (3-vote, 2/3-to-kill); 25/25 confirmed,
0 refuted. Synthesized to 9 load-bearing findings. Full run: `wf_a5ee9d2a-922`.

> **Read this first — the single most important caveat.** Almost all high-confidence evidence is
> *transferred from chess* (and other grid board games), **not** backgammon. Two structural
> differences make the transfer partial: (a) a chessboard is an 8×8 **square lattice** whose line
> intersections drive the best board-localization methods — a backgammon board has **triangular
> points and no grid**, so the *homography concept* transfers but the *feature extraction must be
> redesigned*; (b) chess is a **one-piece-per-square, 13-class** problem, whereas backgammon
> requires **counting stacks of two colors** on each of 24 points plus bar and off — a harder
> target. Treat every chess accuracy number below as an optimistic in-distribution ceiling.

---

## 0. Executive summary (the decision)

1. **Architecture that the evidence supports:** a two-stage board reader (classical
   geometric rectification → learned per-point classifier/counter), wrapped in the **multi-cue
   probabilistic fusion** already chosen in `CLAUDE.md`. The chess literature converges on
   exactly this two-stage split and it is the most transferable pattern we found.
2. **The dominant risk is out-of-distribution (OOD) collapse.** A chess model scoring ~99%
   in-distribution fell to **65% per-square** on new real-world photos, producing a *complete
   valid board for only ~30% of images*. This is the quantified justification for **never
   trusting a single detector**, for leaning hard on **Session Priors** and a **one-time
   board calibration**, and for the **human-review gate**.
3. **Feasibility on CPU/offline is confirmed** — a full classical-segmentation + CNN pipeline
   runs on a *handheld* at ~3–4.5 s per still. Because lazyBG only needs to read the board at a
   few **stable/commit frames per turn** (not every frame), CPU-only is comfortably tractable.
4. **Recommended shipped stack: Go single-package app**, gnubg engine in-process, CV inference
   via **ONNX Runtime through `onnxruntime_go`** (dynamic-loaded native lib, cross-platform),
   classical CV via **OpenCV/gocv** or hand-rolled Go, **video decode via a bundled `ffmpeg`
   binary** (CLI, `ffmpeg-go`) or CGO libav (`go-astiav`). **Python is dev-time-only** for model
   training and dataset rendering. Honest caveat: a *literally single static binary* is not
   achievable — you ship a Go binary **plus** an ONNX Runtime shared lib **plus** an ffmpeg
   binary. The goal is a *lightweight, offline, self-contained bundle*, not one file.
5. **Biggest evidence gaps (prototype these first, no prior art to lean on):** dice pip reading
   (esp. transparent/precision dice, motion blur), doubling-cube reading, clock-hit commit
   detection (visual + audio), hand/occlusion handling, and confidence calibration.

---

## 1. Board detection & perspective/distortion correction

**Recommended technique.** Fiducial-free localization with **classical CV** — grayscale →
Gaussian blur → Canny edges → Hough lines / contour extraction → agglomerative clustering — then
**RANSAC** geometric fitting to compute a **projective homography** that rectifies the board to a
canonical top-down view. Optionally augment with a **deliberately tiny CNN** corner/lattice
detector.

**Evidence (chess, high confidence, 3-0).**
- Wölflein & Arandjelović (*Determining Chess Game State From an Image*, J. Imaging 2021,
  arXiv:2104.14963): RANSAC projective transform onto a regular grid, **99.71% corner accuracy**.
- Czyzewski et al. (arXiv:1708.03898): **SLID** (line detection) + **LAPS** (a one-conv-layer,
  12-filter CNN) → **95% board-positioning**, **>99.5% lattice-point** accuracy, beating classical
  alternatives (60% / 74%).
- CVChess (arXiv:2511.11522): rectifies to a 400×400 top-down view via
  `cv2.getPerspectiveTransform`.

**Backgammon adaptation (critical).** Chess corner/lattice detectors exploit the 8×8 square grid,
which a backgammon board lacks. What transfers is the *rectify-to-canonical-view* step; what must
be rebuilt is *how you find the four board corners / the playing surface*. Practical options,
best-first for our MVP:
- **Session-Prior-assisted, one-time calibration.** Because the MVP assumes a **fixed camera**,
  the user clicks the board corners (or the bar/rail landmarks) **once per video**; we compute one
  homography and reuse it for the whole match. This sidesteps the entire OOD-fragile
  auto-localization problem for the first vertical and is by far the most robust starting point.
- **Semi-automatic:** detect the strong rectangular board border / bearoff trays with contour +
  color priors (board color scheme is a declared Session Prior), snap to the user's rough corners.
- **Fully automatic (later):** small learned corner/keypoint detector trained on rendered +
  labeled frames; only pursue once the calibrated path works end-to-end.

**Feasibility.** Classical CV rectification is milliseconds on CPU. One-time calibration is free at
runtime.

---

## 2. Board-state / checker recognition (the counting problem)

**Recommended technique.** Two-stage **per-point** pipeline on the rectified board: (1) an
**occupancy** step (is this point empty?), then (2) a **color + count** step on occupied points.
This mirrors the chess occupancy-then-classify decomposition, which is the best-performing pattern
found.

**Evidence (chess, high confidence, 3-0).**
- Wölflein & Arandjelović: two CNNs — ResNet occupancy (99.96% val) + InceptionV3 12-class piece
  classifier (100% val); **0.23% per-square test error (~28× prior SOTA)**.
- CVChess: ResNet-style residual CNN per rectified square — 98.93% all-squares, 97.11% non-empty.
- ARChessAnalyzer (arXiv:2009.01649): classical segmentation + fine-tuned AlexNet — 93.45% full
  position, running **on-device**.
- A chess "recorder" project deliberately used **square-occupancy detection** (not piece typing)
  specifically to survive pieces being **obscured at low camera heights** — a directly relevant
  robustness lesson.

**Backgammon adaptation (critical).** Chess needs 0/1 piece per square; backgammon needs a **count
(0–15) of a given color** on a triangular point, plus **bar** and **borne-off** counts. This is a
different head:
- **Counting via segmentation (classical, MVP-friendly):** with a known board/checker color scheme
  (Session Prior) and a rectified top-down view, count checkers per point by **color-segmentation +
  blob/stack analysis along each point's axis**. Stacks are regular and axis-aligned after
  rectification, which makes counting tractable classically — no training data needed to start.
- **Learned counter (later):** a small per-point CNN with a **count regression / ordinal
  classification** head, or per-checker detection. Needs labeled/rendered data.
- **Per-point confidence** falls out naturally (segmentation margin, or classifier soft-max /
  ordinal entropy) and feeds the fusion layer.
- **Tall stacks & perspective:** even after rectification, a near-flat camera makes a stack of 5
  look like 2. The MVP's fixed-camera + one-time calibration lets us learn the per-point
  pixel-height→count mapping from the *opening position* (known layout) as a cheap self-calibration.

**Feasibility.** Per-point classical counting is cheap on CPU; small CNNs export to ONNX and run
per stable frame, not per video frame.

---

## 3. Dice & doubling-cube reading  ⚠️ evidence gap

**No verified claims survived** for dice/pip reading (transparent/precision dice, motion blur,
small dice) or for reading the **doubling cube (videau)** value/orientation. This is an evidence
gap in the literature, **not** evidence of infeasibility — it is a **prototype-first** area.

**Leads (unverified / from search, treat as hypotheses).**
- A two-stage "detect die → count pips" approach (detector localizes each die, then a
  classifier/blob-counter reads pips) is the common hobbyist pattern (e.g. a TensorFlow/PyTorch
  dice-counting write-up surfaced but did not pass verification).
- **Transparent/precision dice** are a known-hard CV problem: transparent objects lack disjoint
  boundaries and produce refraction/specular/caustic artifacts. **SuperCaustics** (arXiv:2107.11008)
  is an open-source real-time simulator for transparent objects usable to synthesize training data
  — relevant if we go learned.
- **The engine is a powerful shortcut.** Per `CLAUDE.md`'s fusion design, **dice often need not be
  read at all**: given the pre/post board-diff, the set of dice that makes the move *legal* is
  frequently **unique** (or a tiny set), so the gnubg legality filter *infers* the dice. Read the
  dice visually only as a confirming/­disambiguating cue. This dramatically lowers the bar on dice CV.
- **Cube:** value is a power of two on a large single die, usually near a fixed rail location
  (Session Prior can declare cube presence/side); doubling is a rare discrete event, so a coarse
  detector + human review is acceptable.

**Action:** build a small hand-labeled dice fixture set from the corpus and prototype both a
classical pip-counter and a tiny detector; measure. Design the fusion so the pipeline degrades
gracefully when dice are unreadable.

---

## 4. Clock detection & clock-hit commit signal  ⚠️ evidence gap

**No verified claims survived** for clock presence detection, clock-hit event detection, or
clock-display reading — including **zero sources addressing audio**. Another prototype-first area,
and an important one: the clock-hit is the MVP's primary **Commit Event**.

**Leads / design (hypotheses).**
- **Presence** is a Session Prior — the user declares "clock present" and (ideally) its rough
  location, removing the detection problem for the MVP.
- **Clock-hit as motion/appearance change:** a hand entering a declared clock ROI + the clock's
  own state flip (the raised/lowered plunger, or the active-side indicator) is a strong visual
  cue. Frame-differencing within the clock ROI is cheap and classical.
- **Audio click:** the mechanical clock "tock" is potentially a *cleaner* commit signal than
  vision (unaffected by occlusion). No source evaluated it — but onset detection on the audio
  track is a well-understood, cheap classical technique worth prototyping. Weigh that many
  BackgammonNews clips have commentary/music over the raw audio.
- **Fuse both:** visual ROI change + audio onset → high-confidence commit; either alone → medium.

**Action:** prototype visual-ROI-diff and audio-onset commit detectors independently on labeled
corpus clips; measure precision/recall of turn boundaries.

---

## 5. Hand / occlusion handling & stable-frame selection  ⚠️ evidence gap

**No verified claims survived** specifically, though the chess "square-occupancy to survive
occlusion" lesson (§2) and general occlusion-robustness papers (PMC7916389, arXiv:2006.08914,
arXiv:2107.11008) are adjacent.

**Design (hypotheses).**
- **Stable-frame selection:** compute inter-frame difference over the board ROI; a **stable
  window** = low motion for N frames. Read the board only on stable frames. Cheap, classical,
  robust. This is the backbone that makes CPU throughput a non-issue.
- **Hand/occlusion gate:** detect large moving skin-colored / foreground blobs over the board;
  suppress board reads while occluded. A small hand detector is optional; frame-differencing +
  color priors go a long way.
- **Experimentation vs commit** (the "players try variations" problem): resolved *structurally*,
  not by CV — only the **last stable board state before the Commit Event** counts (`CLAUDE.md`
  §4). So we don't need to understand intermediate fiddling; we anchor to the commit and take the
  final stable state. This is a key reason the clock-hit MVP is tractable.

**Action:** implement stable-window detection first (it gates everything downstream), then the
occlusion suppressor.

---

## 6. Turn segmentation & commit-event detection

**Design (composes §4–§5).** A **prioritized cascade** of Commit-Event detectors, each emitting a
confidence: clock-hit (visual ROI + audio) → dice-removed/re-thrown (dice appear then disappear) →
long board-stability fallback. The fusion layer takes the highest-confidence commit as the turn
boundary and pairs the **pre-roll stable board** with the **post-move stable board** for diffing.
No direct prior-art verification exists; this is original engineering built on §4–§5 primitives.

---

## 7. Evidence fusion (the heart of the system)

**Recommended: interpretable-first, with two viable formalisms.**

- **Dempster–Shafer / Dynamic Belief Fusion (DBF)** (arXiv:1511.03183, high confidence, 3-0):
  assigns belief mass to *target / non-target / an explicit "uncertain"* hypothesis and combines
  heterogeneous detectors via **Dempster's rule** into one joint score. The explicit **uncertain
  mass maps directly onto the auto-fill-vs-review gate** — an unusually clean fit for lazyBG.
  Caveat: DBF is an object-detection fusion method; adapting it to fuse dice / board-diff / engine
  cues under a hard legality filter is *transfer*, not a demonstrated result.
- **Bayesian weighted fusion** (the `CLAUDE.md` default): equally interpretable, hand-set priors,
  no training data. The survey did **not** find a head-to-head benchmark, so this remains a
  legitimate first choice; DBF is the upgrade to evaluate.

**Hard legality + soft prior.** Both formalisms sit *on top of* the gnubg **hard legality filter**
(candidate moves must be legal given the dice) and the **move-ranking soft prior** (strong players
play near-top moves). Legality is the cheapest, strongest cue we have and it is already in-hand
(the salvaged engine).

**Confidence calibration**  ⚠️ evidence gap. No claim survived on calibration specifically (a
general reference on NN calibration, temperature scaling, surfaced but wasn't verified). Because
the gate decision (auto-fill vs review) hinges on *calibrated* confidence, plan an explicit
**calibration pass** (e.g. temperature scaling / reliability diagrams) once labeled transcriptions
exist. Until then, set conservative thresholds so the system over-refers to human review.

---

## 8. On-device inference stack & the Go-vs-Python decision

**Recommendation: Go for the shipped app; Python dev-time-only.** Rationale, with honest caveats:

- **`onnxruntime_go`** (github.com/yalue/onnxruntime_go, high confidence, 3-0) lets Go run ONNX
  models on CPU cross-platform by **dynamically loading** the ONNX Runtime shared library at
  runtime. Bundled shared libs exist for Windows AMD64, Linux ARM64, macOS ARM64; **other targets
  (notably Linux AMD64) require supplying the official prebuilt `.so`** via env var. **Caveat:** the
  deliverable is therefore *a Go binary + a shipped `.so/.dll/.dylib`*, not one static file — but
  it keeps the **gnubg engine in-process** and avoids a Python sidecar entirely.
- **Rust `ort`** (github.com/pykeio/ort, high confidence, 3-0) is a comparable ONNX Runtime wrapper
  with CPU as guaranteed fallback — a fine alternative **if** Go bindings prove limiting, but it
  **forfeits in-process reuse of the Go gnubg engine** (would need FFI or a sidecar). Not
  recommended given our engine is Go.
- **Python + engine-as-sidecar:** richest CV/training ecosystem, but heaviest bundle (PyInstaller
  200 MB+), which fights the "lightweight" constraint. Reserve Python for **training and dataset
  rendering only**.
- **Classical CV:** OpenCV via **gocv** (CGO) or hand-rolled Go for the simple ops (blur, Canny,
  homography apply, frame-diff, color-segmentation). gocv adds a native OpenCV dependency to the
  bundle; for the MVP much of the classical work is simple enough to avoid a full OpenCV dep, which
  is worth weighing.

**Net:** ship a Go app that embeds gnubg, runs small ONNX detectors via `onnxruntime_go`, does
classical CV in Go/gocv, and decodes video via bundled ffmpeg. Train models in Python, export ONNX.

---

## 9. Video decoding / frame extraction

**Finding (high confidence, 3-0): decoding forces a native/external dependency — a truly
self-contained single binary is not achievable purely.** Two Go paths:
- **`go-astiav`** (github.com/asticode/go-astiav): low-level libav bindings; **requires CGO +
  FFmpeg n8.0 built from source** (MSYS2/MinGW64 on Windows). Best for **frame-accurate seeking**
  (needed to jump to a commit tick and grab the exact stable frame).
- **`ffmpeg-go`** (github.com/u2takey/ffmpeg-go): **pure-Go**, but merely **shells out to an
  `ffmpeg` CLI** that must be on `$PATH`.

**Implication:** either **bundle a static `ffmpeg` binary** alongside the app (CLI approach —
simplest cross-platform, recommended for MVP) or accept a **CGO build linked against FFmpeg**
(astiav — better seeking, heavier build). Rust equivalents exist (`ffmpeg-sidecar` wraps the CLI;
`rusty_ffmpeg` FFI-binds the libs) if the stack shifts. `goav` is **unmaintained — avoid.**

---

## 10. Prior art (transferable lessons)

No backgammon-specific system surfaced. The transferable corpus is **chess** (and general
board-game) state recognition:
- **Wölflein & Arandjelović 2021** (arXiv:2104.14963) — the single most transferable paper:
  end-to-end rectify→occupancy→classify, synthetic training, ~0.23% per-square error.
- **Czyzewski et al.** (arXiv:1708.03898) — SLID+LAPS localization; the "tiny CNN is enough" lesson.
- **ARChessAnalyzer / Mehta 2020** (arXiv:2009.01649) — proof of **on-device** CPU feasibility.
- **CVChess** (arXiv:2511.11522) — the **honest OOD failure** data point (99%→65%).
- **Chess Position ID via synthetic images** (ResearchGate 337786787) — synthetic-data fine-tuning.
- **Dynamic Belief Fusion** (arXiv:1511.03183) — the fusion formalism.
- **SuperCaustics** (arXiv:2107.11008) — transparent-object simulation for training data.
Key cross-cutting lesson: **occupancy-first survives occlusion**; **synthetic data scales**;
**OOD is where these systems die** — which is exactly what multi-cue fusion + priors + review
defends against.

---

## 11. Datasets & training strategy

- **Synthetic 3D rendering** (high confidence, 3-0) is the proven cheap path: chess work rendered
  ~4,888 positions with randomized camera angles (45–60°), lighting, and off-centre pieces →
  ~105k occupied + ~208k empty samples, an order of magnitude more than manual datasets, training
  to 0.23% per-square error. **For backgammon:** render a 3D board with domain randomization
  (board color schemes, checker materials incl. translucent, dice, lighting, camera pose) to
  bootstrap the checker-counter and any learned detectors. **Caveat:** synthetic-only is *itself a
  cause of OOD collapse* — mix in **real labeled frames** from the corpus (domain randomization +
  a little real data).
- **Cheap real labels:** the **BackgammonNews** corpus (`CLAUDE.md` §7) provides diverse real
  captures. Bootstrap labels by running the calibrated classical reader, then **human-correct via
  the review UI** — every corrected transcription becomes labeled data (the same loop that later
  trains the *learned fusion* model). Store small hand-labeled **golden frames** under `testdata/`;
  keep large raw video out of git (referenced by URL + timecode).
- **Dev-time-only heavy pipeline:** train/render in Python with GPU if available; **export ONNX**;
  ship CPU inference. Nothing GPU-dependent reaches the user.

---

## 12. Risk register (ranked)

1. **OOD accuracy collapse on heterogeneous footage** (quantified: 99%→65%, ~30% full-board
   yield). *Mitigation:* Session Priors + one-time board calibration + classical color-segmentation
   for the MVP + multi-cue fusion + human review. Do **not** ship a single learned board reader.
2. **Dice / transparent-dice / cube reading has no prior art.** *Mitigation:* infer dice from
   board-diff legality (engine); treat visual dice as a confirming cue; prototype early; degrade
   gracefully.
3. **Clock-hit commit detection has no prior art (esp. audio).** *Mitigation:* Session-Prior clock
   location + visual ROI-diff + audio-onset; prototype both, fuse.
4. **Backgammon stack-counting ≠ chess one-piece-per-square.** *Mitigation:* rectified classical
   counting + self-calibration from the known opening layout; learned ordinal head later.
5. **Confidence calibration unproven.** *Mitigation:* conservative thresholds now; temperature
   scaling once labeled data exists; over-refer to review early.
6. **"Single binary" is a myth here** (ONNX RT lib + ffmpeg needed). *Mitigation:* target a
   lightweight *bundle*; document the shipped artifacts.
7. **Latency numbers are per-still, not video throughput.** *Mitigation:* process only stable/commit
   frames; never every frame.

---

## 13. Open questions to resolve by prototyping (not more reading)

1. Dice + cube reading on CPU for transparent/precision dice + motion blur — classical pip-blob vs
   tiny detector; what accuracy, and can engine-legality inference make it optional?
2. Is the **audio click** a more reliable commit signal than visual clock-state change, and how to
   fuse them? (No source addressed audio.)
3. Does the chess occupancy-then-classify pipeline actually adapt to **counting stacked checkers**
   (regression/ordinal head vs per-checker detection)?
4. Measured **Go (`onnxruntime_go`) vs Python** CPU throughput/accuracy for our specific small
   models — decide on data, not README claims.

---

## 14. Annotated sources

*Primary (peer-reviewed / papers):* arXiv:2104.14963 (Wölflein & Arandjelović, chess game state) ·
arXiv:1708.03898 (Czyzewski, SLID+LAPS) · arXiv:2009.01649 (ARChessAnalyzer, on-device) ·
arXiv:2511.11522 (CVChess, OOD failure) · arXiv:1511.03183 (Dynamic Belief Fusion) ·
arXiv:2107.11008 (SuperCaustics, transparent objects) · PMC8321354 · ResearchGate 337786787
(synthetic chess) · arXiv:2410.15206, arXiv:2509.15045 (datasets/training) · PMC7916389,
arXiv:2006.08914, ScienceDirect S2667305325000870 (robustness/occlusion).
*Implementation (project docs, primary but self-described):* github.com/yalue/onnxruntime_go ·
github.com/pykeio/ort · github.com/asticode/go-astiav · github.com/u2takey/ffmpeg-go ·
github.com/CCExtractor/rusty_ffmpeg · crates.io/crates/ffmpeg-sidecar · github.com/giorgisio/goav
(unmaintained) · github.com/hybridgroup/gocv (issue #1169).
*Secondary/blog (context only):* Roboflow chess-recording blog · TowardsDataScience dice-counting ·
geoffpleiss NN-calibration.

*Evidence hygiene:* accuracies are largely self-reported, in-distribution, often on the authors'
own synthetic splits; the lone OOD number (65%) is the more honest predictor for real footage.
Stack claims are project READMEs; no independent cross-platform benchmark of onnxruntime_go vs ort
vs Python was found. Six sub-problems (dice, cube, clock, audio, occlusion, calibration) returned
**no** verified claims — genuine gaps, flagged above as prototype-first.
