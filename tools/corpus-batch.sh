#!/usr/bin/env bash
# Batch auto-calibration of the corpus: for every single-video recording
# directory holding a video + a .mat and no manifest yet, run
# `lazybg autocal` and (optionally, with ALIGN=1) `lazybg align` to label it.
# Sequential on purpose: one ffmpeg at a time keeps memory pressure low on a
# loaded machine; raise J at your own risk on a quiet one.
set -u
cd "$(dirname "$0")/.."
mkdir -p corpus/manifest corpus/crops
BIN=${BIN:-go run ./cmd/lazybg}
for dir in corpus/*/*/; do
  case "$dir" in corpus/_*|corpus/manifest/*|corpus/crops/*) continue;; esac
  mats=("$dir"*.mat); vids=("$dir"*.mkv)
  [ -e "${mats[0]}" ] && [ -e "${vids[0]}" ] || continue
  [ "${#vids[@]}" -eq 1 ] || { echo "SKIP multi-part: $dir"; continue; }
  event=$(basename "$(dirname "$dir")"); slug=$(basename "$dir")
  id="${event}_${slug}"
  manifest="corpus/manifest/${id}.json"
  [ -e "$manifest" ] && continue
  echo "=== autocal $id"
  if ! $BIN autocal -video "${vids[0]}" -transcript "${mats[0]}" -id "$id" -out-manifest "$manifest"; then
    echo "FAILED autocal: $id"
    continue
  fi
  if [ "${ALIGN:-0}" = 1 ]; then
    echo "=== align $id"
    $BIN align -manifest "$manifest" -write-manifest -crops "corpus/crops/${id}" || echo "FAILED align: $id"
  fi
done
echo "batch done"
