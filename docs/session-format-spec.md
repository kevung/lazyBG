# Session file format spec — the lazyBG working file

Status: **stable draft, Pass 2 of N**. Formalizes what `docs/architecture.md` §3 already names but never
specifies: the Transcription's "own working format... not the `.mat`". Depends on
`docs/functional-spec.md` being stable (it is, as of that doc's Pass 4) — this spec defines how
everything decided there gets serialized, resumed, and optionally shared back as corpus feedback.

---

## 1. Framing (Pass 1)

**Purpose, three-fold** (per the user's own framing):
1. **Resume/edit an in-progress transcription** — the concrete persistence substrate for
   `functional-spec.md` §3's "autosave, continuous" and §5's "retroactive edit, any time."
2. **Carry every metadata/manifest fact tied to the video** in one place — Session Priors, Board
   Calibration, Parts/Spans, per-turn ticks — so nothing needed to make sense of the recording is
   scattered across files that can drift out of sync.
3. **Be rich enough to serve as community feedback** — if a user chooses to send their lazyBG
   session file back, it should carry everything useful for improving the automatic
   models/techniques: not just the final answer, but which candidates were shown, which cues
   contributed, what the human overrode and why.

**Core architectural decision:** the lazyBG session file is the **single source of truth**.
`.mat` and `corpus/manifest/*.json` become **projections generated on demand** from it (the same
relationship `domain-model.md` §7 already declares between Transcription and `.mat` — this simply
extends that principle to the corpus manifest too, which today is instead a hand/tool-maintained
file that separately references a `.mat` by path). One edit, one place; export never drifts from
source.

**Video handling: referenced, never embedded.** The session file stays small (text/JSON-scale),
referencing the video by **both** a local path (for resuming on this machine) **and** its
canonical source URL (e.g. the YouTube URL, mirroring `CLAUDE.md` §7's existing "referenced by URL
+ timestamp" corpus policy) plus a fingerprint (hash and/or size+duration — exact mechanism: open
item) to detect a substituted/corrupted file on reopen. The URL makes the session portable: a
recipient without the local file (or on a different machine) can re-locate/re-download the same
source, which matters once sessions are shared back as feedback. Consistent with "large raw videos
are not committed" (`CLAUDE.md` §7) and the "lightweight, no install" goal.

**Encoding: JSON.** Matches `corpus.Manifest`'s existing precedent (readable, diffable, no new
dependency), versioned the same way (`SchemaVersion` field, as the manifest already does).

**Extension: `.lbg`.**

## 3. Per-turn content — candidate traceability (Pass 2)

Each turn records more than the confirmed `(dice, move)`: the **candidate list as shown** (each
candidate's dice/move/score), **which one was confirmed or that a correction was entered**, and
**which Cues contributed to the ranking** (board-diff present/absent and its agreement score,
engine equity prior, any other cue in play) — everything `architecture.md` §4's fusion scoring
already computes transiently, just persisted instead of discarded. This is what makes a shared
`.lbg` file useful for retraining/reweighting fusion later: it captures not just "what happened"
but "what the algorithm thought, and where the human disagreed" — without needing to store any
image data itself (Tick + calibration are enough to re-extract the frame from the video later if
needed).

**Review-queue state:** carried as-is — open `pipeline.ReviewItem`s (including `human-flagged`
ones, `functional-spec.md` §4) and their eventual resolutions are stored directly, no new concept;
this is the same structure the automatic pipeline already produces, just persisted across
sessions instead of living only in memory during a single `transcribe` run.

**Privacy/redaction: out of scope for now.** The target use case is public competition footage
(BackgammonNews/YouTube) with already-public player names; the local absolute path is the only
mild leak and isn't worth a dedicated cleanup mechanism yet. Revisit if private footage becomes a
use case.

**Schema versioning: a `SchemaVersion` int field**, mirroring `corpus.Manifest` exactly — no new
strategy needed, same precedent extended.

## 4. Open items — implementation-level, deferred to the build

- Exact video fingerprint mechanism (hash of what — full file? first N MB? ffprobe metadata?) —
  a technical choice with no real functional tradeoff, doesn't block the spec.
