---
status: accepted
---

# Board calibration is two homographies split by the bar, not one

## Context

A second grilling session (2026-07-21), after testing the GUI on real footage, found the
calibration grid does not sit on the real triangles even when the four clicked corners are placed
correctly on the board. Two distinct problems surfaced:

1. **A display bug (separate, trivial):** the main reconstructed board did not receive the declared
   `checkerA`/`checkerB` colours, so P1/P2 rendered with the opposite (default) fills — breaking the
   user's core verification move, which is *visually comparing the reconstructed board to the video*.
   Fixed by passing `checkerColors` to the main `<Board>`. Orientation wiring is sound; the colour
   inversion merely looked like an orientation fault.

2. **The calibration model is too rigid.** `calibrate.CanonicalBoard` fixes every internal
   proportion — margins, `BarGap` (≈4.7% of width), point widths — as constants, and `New` maps the
   four clicked corners onto the **full** canonical rectangle `(0,0)-(w,h)`. A single planar
   homography captures perspective *exactly*, so perspective is **not** the problem: the problem is
   that the fixed internal subdivisions do not match a real board. The **bar width is the dominant
   error** (real boards vary widely), margins second. Even perfect outer corners misplace the 24
   cells. The grid's outer border always lands on the clicked corners (homography construction), which
   *looks* like a good calibration while the interior is wrong.

## Decisions

1. **Two homographies, split by the bar.** The playing surface is calibrated as two half-board
   quads — left `[TL, barTL, barBL, BL]`, right `[barTR, TR, BR, barBR]` — each with its own
   homography onto a canonical half. Within a half, the six points are evenly spaced and fill the
   space from the outer playing edge to the bar edge — true for **every** backgammon board — so the
   subdivision is exact with no guessed proportion. This also tolerates the slight central **fold**
   at the board's hinge (each half fits its own plane), which one homography cannot.

2. **Eight draggable handles.** Calibration captures four outer corners (freely draggable) plus four
   bar-edge points (`barTL, barTR, barBL, barBR`) constrained to slide along the top/bottom playing
   edges, so the bar is always a clean strip and its width/skew are explicit. A live preview draws
   both half-grids as the user drags; handles are pre-seeded (default corners, bar ~5% centred) so
   the user adjusts rather than places from scratch. Manual dragging is the reliable, primary path;
   auto-detection (best-effort seed only, never final) is deferred and non-blocking.

3. **Bearoff and cube are out of the calibration perimeter.** The eight handles cover the 24 points
   and the bar only. Bearoff is derived from game state (not read from the tray) as today, so the off
   tray leaves the canonical. The doubling cube is out of scope for this redesign: a centred cube sits
   on the (calibrated) bar, and a cube on a side rail is a future *cube-perception* concern handled by
   a dedicated rail ROI prior (the clock-ROI pattern), not by widening the point grid.

4. **Versioned format + deterministic migration (single code path).** The stored calibration gains a
   schema version and now holds eight source points. Existing four-corner manifests migrate
   deterministically: the four bar/outer landmark positions of the old canonical are pushed through
   the old single homography to synthesise the eight source points, so the two-homography result
   **reproduces the old grid pixel-for-pixel** (the two half-homographies agree with the one homography
   on their sub-quads for coplanar points). The pilot manifest and its e2e stay green; re-calibrating
   improves accuracy. No permanent legacy branch — one code path, one migration.

## Consequences

- `internal/calibrate` builds from eight points into two homographies; `Rectify` samples each
  canonical half through its own homography into a single combined rectified image, so the board
  reader (`boardstate`) and `PointRegion` keep their current API — the change is localised to
  construction and sampling. The new canonical drops margins and the off tray (points fill each half
  edge-to-edge).
- `corpus.Calibration` gains a version + the eight points (4 corners + 4 bar edges); a migration
  upgrades v1 (four-corner) manifests on load/convert. `session` and `transcribe.PartSetup` build the
  two-homography calibration.
- `gui/frontend/src/lib/calibration.js` moves from one quad to two (grid + overlay de-projection);
  `SetupPanel.svelte` replaces click-only capture with eight draggable handles and a live dual-grid
  preview.
- Supersedes the single-homography assumption in ADR-0006 and `docs/domain-model.md` §Board
  Calibration. Perspective handling is unchanged (homography); only the internal-proportion rigidity
  is removed. Auto-calibration (`internal/autocal`) becomes a seed source later, not a dependency.
