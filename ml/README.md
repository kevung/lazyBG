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
