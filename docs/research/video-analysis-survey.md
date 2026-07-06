# Deep-research survey — video/image analysis techniques for backgammon transcription

> **Status: PENDING.** This is the placeholder for the comprehensive state-of-the-art survey
> that gates the architecture and the tech-stack decision. It is intentionally **exempt from the
> ~500-line doc rule** — it should be as broad and detailed as the subject requires.

## Scope (to be filled in)

Per-cue technique survey, each judged on CPU-only / offline / modest-PC feasibility:

- Board detection & perspective correction (homography, fiducial-free board localization).
- Board-state / checker recognition (classical color segmentation vs. learned detectors).
- Dice detection & pip reading, including transparent/precision dice.
- Clock presence & clock-hit event detection (visual + optional audio).
- Hand / occlusion handling and stable-frame selection.
- Turn segmentation & "commit" detection.
- On-device inference options (OpenCV, ONNX Runtime, tract, small YOLO-class models) and their
  Go/Python runtime trade-offs.
- Relevant datasets, pretrained models, and boardgame-vision prior art.
- Recommended stack + justification against the locked constraints.

See `CLAUDE.md` §3 for the locked decisions and open questions this survey must resolve.
