# Architecture — lazyBG

> **Status: STUB.** To be written (~500 lines) after the deep-research survey confirms the tech
> stack. The high-level pipeline shape is sketched in `CLAUDE.md` §5.

## To specify

- Module boundaries (ingestion, cues, fusion, review UX, export) and their interfaces.
- Data flow and the per-turn processing loop.
- The confidence model and the fusion algorithm (interpretable-first).
- The engine integration seam (legality filter + move-ranking prior).
- The `.mat` (Jellyfish) export mapping.
- Tech stack / language (deferred to `research/video-analysis-survey.md`).
