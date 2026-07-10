# Automatic board detection & calibration — research survey

What the chess/go/board-game detection literature teaches lazyBG's
auto-calibration (finding the 4 playing-surface corners + span
automatically), and where our implemented approach sits. Compiled 2026-07;
sources inline.

## Executive summary

1. **There is no published work on backgammon board localization.** The two
   existing backgammon CV projects both punt: BackgammonCV asks the user to
   "click clockwise on the 4 extreme points" then `getPerspectiveTransform`
   ([christiancorro/BackgammonCV], verified in source); JvitorS23's checker
   detector also takes manual corners — and even *with* hand calibration
   reports only 40% full-board matches (90% per-point), confirming that
   per-point reading, not calibration, is the accuracy bottleneck.
2. **The chess literature's precision machinery does not transfer** — it is
   all grid-based: chesscog's RANSAC snaps line intersections to an integer
   lattice ([arXiv:2104.14963]; 99.7-100% localization but on synthetic
   renders), Czyzewski's LAPS classifies lattice-point patches
   ([arXiv:1708.03898], 95% vs 60% baselines), Go readers exploit two
   families of grid lines in Hough space ([Imago thesis]). Backgammon has no
   lattice: only the frame, the bar, and 24 triangles.
3. **What does transfer:**
   - *Hypothesize-and-verify against known geometry* (chesscog's core
     pattern): sample a homography, project ALL features, count inliers
     against the known model. Our variant — validating a candidate
     calibration by **reading the known standard-start position** — is this
     pattern with a stronger oracle, and it is implemented
     (`internal/autocal`: pilot capture reaches 21/24 opening read = parity
     with hand-picked corners).
   - *Color segmentation + known topology* for non-grid boards — what every
     Catan project does (HSV ranges + hex topology). Our
     median-frame + point-color mask + felt-ring component filter is this
     family.
   - *Calibrate once, then track drift* for fixed-camera video (Rocamgo,
     AutoGoRecorder, [PhotoKifu/VideoKifu, arXiv:1508.03269] — track small
     grid movements between frames instead of re-detecting). Matches our
     observed board drift during matches; cheap NCC/feature tracking on the
     board frame is the next step there.
   - *Robustness tricks*: Czyzewski's SLID (multi-CLAHE line extraction,
     M-estimator merging) and CPS (crop-warp-repeat until convergence) are
     geometry-agnostic and reusable with a backgammon-specific score.
   - *Synthetic training* if a learned corner regressor is ever needed:
     chesscog trained on 5,000 Blender renders + 2-photo fine-tuning;
     ChessReD provides the honest counterpoint below.
4. **The honest generalization warning, three times over:** classical
   pipelines that hit 99%+ on controlled/synthetic data collapse on
   heterogeneous real photos — end-to-end ChessReD results: the best
   classical pipeline manages ~2% full-board on real smartphone photos vs
   15% for a CNN ([arXiv:2310.04086]); CVChess: 98.9% in-distribution →
   65% per-square out-of-distribution ([arXiv:2511.11522]); the Kifubara
   developer: "traditional CV… just too fragile… ended up training
   specialized NNs" for board finding. **Practical consensus: one-time
   user-assisted 4-corner calibration is the near-universal fallback.**
   lazyBG should treat auto-calibration as a *confidence-scored proposal*
   that pre-fills the corner UI — exactly how `lazybg align`/manifests are
   structured — not as a must-succeed step.
5. **Backgammon-specific cautions:** triangles are occluded by checkers
   mid-game → calibrate on the opening/empty board (we do: temporal median
   over the opening minutes); the 12-period self-similarity of the point
   pattern creates a one-period translation ambiguity that only the
   frame/bar can break (Imago documents the analogous "grid shifted by one
   line" local minimum for Go); template matching (SIFT/ORB) against a
   reference photo fails across board models — per-capture only.

## Where our implementation stands (2026-07)

`internal/autocal` implements: temporal median (hands/dice erased) →
point-color mask (declared or auto-derived colors: felt = dominant
unsaturated center tone, point colors = saturated clusters adjacent to
felt) → component filter (size, aspect, felt-ring) → extreme-projection
quad + margin expansion → opening-scan (span begin for free) →
corner/edge/quad hill-climb on the opening-read score.

- Pilot (straight-on): **21/24 opening read — manual parity.** ✔
- Oblique vbc capture (~10° rotation): 16/24 — the translation-only
  refinement moves cannot absorb rotation. Next steps, in order of expected
  cost: (a) add rotation/shear group moves to the hill-climb; (b) replace
  the extreme-projection initial quad with a line-based one (the board
  frame's 4 strong edges via LSD/Hough + the SLID tricks); (c) a tiny
  corner-regression CNN trained on boardsynth renders + our labeled
  captures (chesscog's recipe), reusing the LZPN1 pure-Go runtime.

## Key sources

[chesscog, arXiv:2104.14963](https://arxiv.org/abs/2104.14963) ([code](https://github.com/georg-wolflein/chesscog));
[Czyzewski et al., arXiv:1708.03898](https://arxiv.org/abs/1708.03898) ([neural-chessboard](https://github.com/maciejczyzewski/neural-chessboard));
[LiveChess2FEN, arXiv:2012.06858](https://arxiv.org/abs/2012.06858) (reuses SLID/LAPS; 95% board detection on Jetson);
[End-to-End Chess Recognition / ChessReD, arXiv:2310.04086](https://arxiv.org/abs/2310.04086) (10,800 real photos, corner
annotations <2px; classical pipelines collapse on it);
[CVChess, arXiv:2511.11522](https://arxiv.org/html/2511.11522v3) (largest-contour quad; OOD cliff);
[Imago Go thesis](http://tomasm.cz/imago_files/go_image_recognition.pdf) (Hough line families, HMM/Viterbi over video
reads — independently validates our fusion-over-time design);
[PhotoKifu/VideoKifu, arXiv:1508.03269](https://arxiv.org/abs/1508.03269) (calibrate once, track drift);
[Kifu-Snap directory](https://www.remi-coulom.fr/kifu-snap/); Kifubara practitioner report (OGS forums);
Catan projects ([Vieja/Catan-Image-Recognition](https://github.com/Vieja/Catan-Image-Recognition), CMU Catan-omous);
[christiancorro/BackgammonCV](https://github.com/christiancorro/BackgammonCV);
[JvitorS23/backgammon-checker-detection-openCV](https://github.com/JvitorS23/backgammon-checker-detection-openCV);
[Kolatacz Medium write-up](https://medium.com/@nitzankolatacz/how-i-used-machine-learning-to-beat-my-friends-at-backgammon-fb541ec1c0e5)
(classical rectangle/Hough attempts failed; YOLO on 30 overhead images);
[ORB](https://ieeexplore.ieee.org/document/6126544/).
