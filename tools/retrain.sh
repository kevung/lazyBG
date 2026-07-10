#!/usr/bin/env bash
# One-shot post-batch retraining of the learned point reader.
# Assumes ml/.venv exists (see ml/README.md). Excludes the duplicate pilot
# manifest (same video under two ids) and holds out whole recordings for an
# honest cross-capture validation.
set -eu
cd "$(dirname "$0")/.."
VAL=${VAL:-2025-05_hsbtMarseille_or-r1_HanotinDenis}
EXCLUDE=${EXCLUDE:-2025-05_hsbtMarseille_main-r1_PavicevicNina}
PY=${PY:-ml/.venv/bin/python}
$PY ml/train_pointreader.py --crops corpus/crops/* --out ml/out \
  --epochs "${EPOCHS:-40}" --val-recordings "$VAL" --exclude-recordings "$EXCLUDE"
$PY ml/export_go.py --model ml/out/pointreader.pt --out data/models/pointreader.bin
cp ml/out/pointreader.json data/models/
echo "retrained; run: GOMEMLIMIT=2GiB go test -p 1 ./internal/e2e/ -run TestRealCorpus_Learned -v"
