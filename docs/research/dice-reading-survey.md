# Dice reading from real match video — research survey

State of the art and a concrete plan for lazyBG's dice cue: presence/removal
detection (a commit signal) and value reading (a fusion cue), on ~20-35 px
dice thrown on the board felt, CPU-only, offline. Compiled 2026-07 from a
web sweep (sources inline; items marked *unverified* could not be confirmed
by fetching the primary source).

## Executive summary

1. **Nothing off-the-shelf matches our conditions.** Every open project and
   paper assumes large (≥100 px) top-down dice under controlled light; every
   casino system uses a dedicated shaker camera or RFID dice. The two
   backgammon-transcription products that exist (TrackGammon, DigiGammon)
   both *constrain the setup* (partner boards / mounted overhead webcam)
   rather than solve unconstrained footage — independent validation of our
   Session-Priors philosophy. DigiGammon wants a GPU; our CPU-only target is
   a differentiator.
2. **The winning recipe is assembled from proven parts, all cheap:**
   - *Localization for free*: skip full-frame detection entirely. Board
     Calibration already gives a rectified ROI where dice are ~25 px native;
     full-frame nano-detectors lose ~2/3 of their accuracy on objects this
     small at 640 input (YOLOv5n AP_S 12.7 vs AP_L 35.4,
     [YOGA, arXiv:2307.05945]) and cost 56-320 ms/frame on CPU, while a
     board-ROI crop keeps native resolution for one slice of the cost.
   - *Appearance/removal events*: the "abandoned/removed object" literature
     solves exactly this with **dual-background models** (fast+slow learning
     rates; a settled die appears in the slow foreground only), a per-blob
     persistence FSM with an explicit occlusion state (a hand pass must not
     reset a latched die), and an edge-energy test to tell appeared from
     removed ([Sensors 2019, PMC6928649]; [EURASIP JIVP 2011];
     [PMC11510867]). Near-perfect on the PETS/ABODA benchmarks, no training
     data, O(ROI) running averages. Important negative result: *generic*
     background subtraction scores only F≈0.65-0.78 on the CDnet
     "intermittent object motion" category — the dual-background/FSM
     specialization is not optional ([arXiv:1811.05255]).
   - *Values*: two-stage — classical pip-blob/contour proposal (our existing
     `perceive/dice` Hough machinery) + a **tiny CNN classifier on 32-64 px
     crops** (~150-300k params, <1 ms/crop even in pure Go). Two-stage
     doubled full-frame dice accuracy vs one-shot detection in the one
     head-to-head found (32%→64% fully-correct frames, [Towards Data
     Science two-stage dice]); chess-cv proves 156k-param crop classifiers
     reach 99.9% synthetic / 92-98% real ([github.com/S1M0N38/chess-cv]).
     d6 face classification itself is MNIST-easy ([nell-byler/dice_detection]:
     LeNet-5, 250 images, 20 min of training).
   - *Verification*: check pip clusters against the six known spatial pip
     layouts — the verification module of the NTUST stereo patent
     ([US8724888B2]) transfers monocularly.
   - *Audio*: no prior art on dice-rattle detection specifically, but a dice
     throw is a burst of 3-10 percussive onsets in ~0.5 s; spectral-flux
     onset detection is the standard tool, microseconds of CPU (aubio). Use
     as a confidence booster, never required (commentary/music overlays).
3. **Training data**: no public dataset of small dice on felt exists. Seed
   data that is legally usable: Roboflow public "6-Sided Dice" (359 images,
   Public Domain), Kaggle nell-byler d6 set; but the domain gap is large.
   Our own weak supervision is better: the .mat gives the two dice values
   per turn without location — combine the classical proposer (pseudo-boxes)
   with value labels to bootstrap, and hard-mine where proposal count ≠ 2.
4. **Transparent/precision dice remain the evidence gap** (confirmed): the
   only project claiming to handle glass dice is [Kishaan/Dice-Detection]
   (report unread); nell-byler names translucent dice as the main failure
   mode. Keep them out of MVP scope.

## Prototype plan

1. **Dice-zone events** (classical, no training): rectified board ROI minus
   the point columns/bar → motion gate (skip hand frames — the standard
   trick in every board-game project) → dual running-average background
   (rates ≈0.1-0.3 fast / 0.03-0.06 slow) → blob 15-50 px persisting ≥0.5 s
   ⇒ die appeared (tick = first stable frame); interior-matches-felt ⇒
   removed. Validate against the pilot's aligned turn ticks (dice removal
   should precede each roll).
2. **Value reading**: reuse `perceive/checker` at pip radius inside each
   event blob → pip count; verify pip layout geometry; emit `cue.DiceValue`
   with confidence from blob/pip margins.
3. **Learned upgrade** (when classical value reading plateaus): crop dataset
   from event blobs labeled by the .mat values (weak supervision; discard
   ambiguous double-blob cases), 13-class → 6-class crop CNN like the point
   reader, same LZPN1 pure-Go runtime.
4. **Metrics**: per-turn dice-value accuracy vs truth; event
   precision/recall vs aligned ticks; effect on eval coverage when the dice
   cue joins fusion.

## Key sources

Projects: [jwolle1/Dice_Counter_OpenCV](https://github.com/jwolle1/Dice_Counter_OpenCV) (GPL-3.0),
[golsteyn dice blog](https://golsteyn.com/writing/dice/) (blob+DBSCAN, no license),
[Kishaan/Dice-Detection](https://github.com/Kishaan/Dice-Detection) (video, glass dice, no license),
[nell-byler/dice_detection](https://github.com/nell-byler/dice_detection) (BSD-3, SSD MobileNet + d6 CNN),
[N3VERS4YDIE/dice-recognition](https://github.com/N3VERS4YDIE/dice-recognition) (MIT, YOLOv8 weights),
[christiancorro/BackgammonCV](https://github.com/christiancorro/BackgammonCV) (GPL-3.0 — dice faces as
YOLOv4-tiny classes `1..6,disk_b,disk_w`; manual 4-corner calibration).
Datasets: [Roboflow 6-Sided Dice](https://public.roboflow.com/object-detection/dice) (Public Domain, 359 imgs),
[Kaggle nellbyler/d6-dice](https://www.kaggle.com/datasets/nellbyler/d6-dice),
[Roboflow backgammon-eofws](https://universe.roboflow.com/boardgamedetection/backgammon-eofws) (49 imgs, *unverified*).
Papers/patents: [Huang 2008, Sensors 8(2):1212](https://pmc.ncbi.nlm.nih.gov/articles/PMC3927534/) (sic-bo, 100% under controlled CCD),
Hsu et al. CAIP 2011 / Optical Eng. 2013 (uncontrolled illumination, multi-view, *numbers unverified*),
[US8724888B2](https://patents.google.com/patent/US8724888B2/en) (stereo dice recognition; pip-layout verification),
[US8998698B2](https://patents.google.com/patent/US8998698B2/en) (RFID dice — the industry's vision bypass),
[SPIE 2423 SORTE 1995](https://www.spiedigitallibrary.org/conference-proceedings-of-spie/2423/0000/Automated-detection-and-classification-of-dice/10.1117/12.205506.short).
Detection scale/cost: [YOGA](https://arxiv.org/pdf/2307.05945) (AP_S collapse),
[small-object floor ~10-16px](https://joelhuang.dev/blog/small-object-detection),
[NanoDet-Plus](https://github.com/RangiLyu/nanodet) (8.3 ms @416 on i7-8700),
[SAHI](https://arxiv.org/abs/2202.06934) (linear cost in slices — a board crop is cheaper),
[two-stage dice, TDS](https://towardsdatascience.com/a-two-stage-stage-approach-to-counting-dice-values-with-tensorflow-and-pytorch-e5620e5fa0a3/),
[chess-cv](https://github.com/S1M0N38/chess-cv).
Events: [PMC6928649](https://pmc.ncbi.nlm.nih.gov/articles/PMC6928649/) (dual background, 50:500 rates, illumination reset),
[PMC11510867](https://pmc.ncbi.nlm.nih.gov/articles/PMC11510867/) (FSM with occlusion state, λ ranges),
[SFO survey, CVIU 2016](https://www.researchgate.net/publication/304780006_Detection_of_stationary_foreground_objects_A_survey),
[CDnet IOM scores](https://arxiv.org/pdf/1811.05255), [ViBe](https://www.telecom.uliege.be/publi/publications/mvd/VanDroogenbroeck2014ViBe/) (0.3 ms/frame),
IBM appeared/removed edge-energy patent family ([US10043105](https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/10043105)).
Products: [TrackGammon](https://trackgammon.com/), [DigiGammon](https://www.digigammon.com/),
[Interblock DRS](https://www.interblockgaming.com/product/automated/automated-craps/), [Tangiamo](https://www.tangiamo.com/automatic-dice-recognition/).
