# ml/ — dev-time model training (Python)

Python is **dev-time only** (CLAUDE.md §3, locked decision 10): models are
trained here, exported to ONNX, and shipped inside the Go app. Nothing in
this directory runs on a user's machine.

## The learned point reader

`train_pointreader.py` trains the first learned upgrade (experiment-plan §8
step 8): a tiny CNN that classifies one rectified board-point crop into 13
classes (empty, A×1..6+, B×1..6+). It replaces the classical shape-first
reader's per-point decision, whose ~85% real-frame accuracy is the measured
blocker for confident dice+move inference (see
`internal/e2e/realtranscribe_test.go`).

Training data comes from the corpus labeling machine — no hand labeling:

```bash
# 1. anchor a recording's ground truth to its video + extract labeled crops
go run ./cmd/lazybg align -manifest corpus/manifest/<id>.json \
    -write-manifest -crops corpus/crops/<id>

# 2. set up the environment (once)
cd ml
uv venv --python 3.13 .venv
uv pip install --python .venv/bin/python \
    --index-url https://download.pytorch.org/whl/cpu torch
uv pip install --python .venv/bin/python onnx onnxruntime pillow numpy

# 3. train (splits by GAME to avoid near-duplicate leakage)
.venv/bin/python train_pointreader.py --crops ../corpus/crops/* --out out
```

Outputs: `out/pointreader.onnx` (+ `pointreader.json` metadata,
`report.txt` with per-class recall and the confusion matrix).

## Growing the dataset

Every new corpus manifest (4 corner clicks + colors + span) unlocks a whole
match of labeled crops via step 1. Diversity across board color schemes and
angles matters more than raw volume (experiment-plan §2).

## Swapping the shipped weights: the e2e duel decides, not per-crop accuracy

`tools/retrain.sh` overwrites `data/models/pointreader.bin` directly. **Do not
let it.** Per-crop validation accuracy saturated long ago (~98%) and no longer
discriminates between candidates (issue #40): candidates with equal-or-better
per-crop scores have repeatedly *lost* the blind-transcription duel. Export the
candidate somewhere outside `data/models/`, then duel it:

```bash
# candidate (train + export elsewhere)
ml/.venv/bin/python ml/train_pointreader.py --crops corpus/crops/* --out ml/out \
  --epochs 40 --val-recordings 2025-05_hsbtMarseille_or-r1_HanotinDenis \
  --exclude-recordings 2025-05_hsbtMarseille_main-r1_PavicevicNina
ml/.venv/bin/python ml/export_go.py --model ml/out/pointreader.pt --out /tmp/cand.bin

# duel: same binary, same manifests, only -model differs
GOMEMLIMIT=2GiB go run ./cmd/lazybg eval -manifest corpus/manifest/<id>.json \
  -limit-ms 1800000 -model {data/models/pointreader.bin | /tmp/cand.bin}
```

Swap only if the candidate wins on **matched plies** and does not raise
auto-fill errors at any gate. The metric that matters is the whole pipeline's
output, not the reader's.

### Duel log

| date | candidate | per-crop val | pilot30 matched | Picot15 matched | verdict |
|---|---|---|---|---|---|
| 2026-07-26 | retrain #4, + r3 Lafon crops (48.9k) | 98.33% | 16 (= baseline) but errors up at every gate (@0.70 6 vs 4) | **6** (baseline 12) | **rejected** — shipped weights kept |

The 2026-07-26 candidate is the clearest case yet of the issue-#40 rule: best
per-crop score on record, and it halved matched plies on the held-out capture.
