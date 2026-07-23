import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  canonicalSize, landmarks, canonicalGridLines, solveHomography, projectPoint,
  buildCalibration, projectCanonical, gridOnFrame, canonicalPointCenters, roiBBox,
  DEFAULT_CANONICAL, distortPoint, projectCanonicalLens, undistortPoint,
  workspaceRect, clampToWorkspace, WORKSPACE_MARGIN, canonicalCheckerSlots, checkerRadiusOnFrame,
} from './calibration.js'

const approx = (a, b, eps = 1e-6) => assert.ok(Math.abs(a - b) <= eps, `${a} ≈ ${b}`)
const approxPt = (p, q, eps = 1e-3) => { approx(p[0], q[0], eps); approx(p[1], q[1], eps) }

// A wide, off-centre bar — the case a fixed-fraction canonical gets wrong.
const CORNERS = [[0, 0], [1200, 0], [1200, 800], [0, 800]]
const BAR = [[500, 0], [740, 0], [740, 800], [500, 800]] // barTL,barTR,barBR,barBL

test('canonicalSize matches calibrate.DefaultCanonical (860×800)', () => {
  const { w, h } = canonicalSize()
  assert.equal(w, 860)
  assert.equal(h, 800)
})

test('landmarks mirror the Go calibrate.landmarks positions', () => {
  const lm = landmarks()
  approxPt(lm[0], [20, 20]) // TL
  approxPt(lm[1], [780, 20]) // TR (outer right playing edge)
  approxPt(lm[3], [20, 780]) // BL
  approxPt(lm[4], [380, 20]) // barTL (bar left edge)
  approxPt(lm[5], [420, 20]) // barTR (bar right edge)
})

test('buildCalibration maps every canonical landmark to its source handle', () => {
  const cal = buildCalibration(CORNERS, BAR)
  const lm = landmarks()
  const handles = [...CORNERS, ...BAR]
  for (let i = 0; i < 8; i++) {
    approxPt(projectCanonical(cal, lm[i]), handles[i])
  }
})

test('buildCalibration migrates (single homography) when bar edges are absent', () => {
  const cal = buildCalibration(CORNERS, null)
  assert.deepEqual(cal.left, cal.right) // collapsed
  // A canonical point projects the same as a plain full-quad homography.
  const { w, h } = canonicalSize()
  const H = solveHomography([[0, 0], [w, 0], [w, h], [0, h]], CORNERS)
  approxPt(projectCanonical(cal, [430, 400]), projectPoint(H, [430, 400]))
})

test('projectCanonical picks the half by the bar centre', () => {
  const cal = buildCalibration(CORNERS, BAR)
  // Left of splitX uses left (→ narrow left half [0,500]); right uses right half.
  const l = projectCanonical(cal, [200, 400]) // canonical left half
  const r = projectCanonical(cal, [700, 400]) // canonical right half
  assert.ok(l[0] < 500 && r[0] > 740, `left ${l[0]} right ${r[0]}`)
})

test('gridOnFrame projects border + 24 cells + bar; bar follows the handles', () => {
  const grid = gridOnFrame(CORNERS, BAR)
  assert.equal(grid.length, 1 + 24 + 1) // border + cells + bar
  const bar = grid[grid.length - 1] // last line is the bar loop
  // Its four corners are the bar-edge handles (loop repeats the first).
  approxPt(bar[0], BAR[0]); approxPt(bar[1], BAR[1]); approxPt(bar[2], BAR[2]); approxPt(bar[3], BAR[3])
})

test('gridOnFrame rejects the wrong number of corners', () => {
  assert.equal(gridOnFrame([[0, 0], [1, 1]], null), null)
  assert.equal(gridOnFrame(null, null), null)
})

test('canonicalPointCenters places points on the calibrate grid', () => {
  const c = canonicalPointCenters()
  assert.deepEqual(c[13], [50, 200]) // top-left cell centre
  assert.deepEqual(c[24], [750, 200]) // top-right cell centre
  assert.deepEqual(c[1], [750, 600]) // bottom-right cell centre
  assert.deepEqual(c[12], [50, 600]) // bottom-left cell centre
  assert.deepEqual(c[7], [350, 600]) // just left of the bar
  assert.deepEqual(c[6], [450, 600]) // just right of the bar
})

test('roiBBox bounds all given points, expanded by the margin', () => {
  const box = roiBBox([...CORNERS, ...BAR], 0)
  assert.deepEqual(box, { x: 0, y: 0, w: 1200, h: 800 })
  assert.equal(roiBBox(null), null)
  assert.equal(roiBBox([]), null)
})

test('canonicalGridLines returns closed loops', () => {
  for (const line of canonicalGridLines()) {
    assert.deepEqual(line[0], line[line.length - 1])
  }
})

test('distortPoint: inactive lens is the identity, barrel pushes outward', () => {
  assert.deepEqual(distortPoint(null, [10, 20]), [10, 20])
  assert.deepEqual(distortPoint({ k1: 0, k2: 0, centerX: 0, centerY: 0, norm: 100 }, [10, 20]), [10, 20])
  const lens = { k1: -0.2, k2: 0, centerX: 100, centerY: 100, norm: 100 }
  const [x, y] = distortPoint(lens, [200, 100]) // 1 normalised radius to the right
  assert.ok(x < 200, 'barrel pulls periphery toward the centre')
  assert.equal(y, 100)
})

test('gridOnFrame: active lens curves the drawn grid', () => {
  const corners = [[100, 100], [500, 100], [500, 400], [100, 400]]
  const straight = gridOnFrame(corners, null)
  const lens = { k1: -0.15, k2: 0, centerX: 300, centerY: 250, norm: 300 }
  const curved = gridOnFrame(corners, null, DEFAULT_CANONICAL, lens)
  assert.ok(straight && curved)
  // The curved border has more vertices (sampled) and is not collinear: the
  // midpoint of its top border deviates from the straight chord.
  assert.ok(curved[0].length > straight[0].length)
  const top = curved[0]
  const first = top[0]
  const mid = top[Math.floor(top.length / 8)] // a point along the top edge
  assert.ok(Math.abs(mid[1] - first[1]) > 0.5, 'top border must bow under barrel distortion')
})

test('projectCanonicalLens: homography then lens', () => {
  const corners = [[100, 100], [500, 100], [500, 400], [100, 400]]
  const cal = buildCalibration(corners, null)
  const lens = { k1: -0.1, k2: 0.02, centerX: 300, centerY: 250, norm: 300 }
  const p = projectCanonical(cal, [430, 40])
  assert.deepEqual(projectCanonicalLens(cal, lens, [430, 40]), distortPoint(lens, p))
})

// Both directions are asserted only for radially MONOTONE lenses. A strong
// barrel (k1 ≤ ~-0.2 at r > 1) caps the recordable radius — ru - 0.2·ru³ peaks
// at 0.86 — so beyond that cap no ideal point maps there and distort∘undistort
// has nothing to invert. calibrate.Lens.undistort has the identical limit; the
// admitted fits stay well inside it.
test('undistortPoint inverts distortPoint (mirrors calibrate.Lens.undistort)', () => {
  assert.deepEqual(undistortPoint(null, [10, 20]), [10, 20])
  for (const lens of [
    { k1: -0.08, k2: 0, centerX: 300, centerY: 250, norm: 300 }, // barrel
    { k1: 0.12, k2: 0, centerX: 300, centerY: 250, norm: 300 }, // pincushion
    { k1: -0.18, k2: 0.03, centerX: 300, centerY: 250, norm: 300 }, // k1+k2
  ]) {
    for (const p of [[300, 250], [600, 250], [0, 0], [610, 505], [120, 480]]) {
      approxPt(undistortPoint(lens, distortPoint(lens, p)), p, 1e-6)
      approxPt(distortPoint(lens, undistortPoint(lens, p)), p, 1e-6)
    }
  }
})

// The regression test for the JS/Go mismatch (issue #61): the handles are points
// of the RECORDED frame, so buildCalibration must undistort them exactly like
// calibrate.NewSplitWithLens does — otherwise the drawn grid corner lands on
// distort(handle) instead of the handle, tens of pixels away.
test('buildCalibration undistorts the handles: each grid landmark lands back on its handle', () => {
  const lens = { k1: -0.18, k2: 0.03, centerX: 600, centerY: 400, norm: 600 }
  const cal = buildCalibration(CORNERS, BAR, DEFAULT_CANONICAL, lens)
  const lm = landmarks()
  const handles = [...CORNERS, ...BAR]
  for (let i = 0; i < 8; i++) {
    approxPt(projectCanonicalLens(cal, lens, lm[i]), handles[i], 1e-3)
  }
})

test('gridOnFrame: with an active lens the border corners sit ON the handles', () => {
  const lens = { k1: -0.18, k2: 0.03, centerX: 600, centerY: 400, norm: 600 }
  const grid = gridOnFrame(CORNERS, BAR, DEFAULT_CANONICAL, lens)
  const border = grid[0]
  approxPt(border[0], CORNERS[0], 1e-3) // TL
  const bar = grid[grid.length - 1]
  approxPt(bar[0], BAR[0], 1e-3) // barTL
})

test('workspaceRect is the frame expanded by the 15% handle margin', () => {
  assert.equal(WORKSPACE_MARGIN, 0.15)
  assert.deepEqual(workspaceRect(1000, 800), { x: -150, y: -120, w: 1300, h: 1040 })
  assert.equal(workspaceRect(0, 0), null)
})

test('clampToWorkspace clamps per axis and leaves inside points untouched', () => {
  const r = workspaceRect(1000, 800)
  assert.deepEqual(clampToWorkspace([500, 400], r), [500, 400])
  assert.deepEqual(clampToWorkspace([-40, 900], r), [-40, 900]) // inside the margin
  assert.deepEqual(clampToWorkspace([-900, 400], r), [-150, 400]) // x only
  assert.deepEqual(clampToWorkspace([5000, -5000], r), [1150, -120])
  assert.deepEqual(clampToWorkspace([5, 5], null), [5, 5]) // no rect ⇒ no clamp
})

test('canonicalCheckerSlots stacks from the point base inward', () => {
  // Top row (13..24) grows downward from the outer edge; bottom row upward.
  const top = canonicalCheckerSlots(13, 3)
  assert.deepEqual(top.map((s) => s[0]), [50, 50, 50]) // column centre
  assert.deepEqual(top.map((s) => s[1]), [50, 110, 170]) // marginY + (k+0.5)*pointW
  const bottom = canonicalCheckerSlots(1, 2)
  assert.deepEqual(bottom.map((s) => s[0]), [750, 750])
  assert.deepEqual(bottom.map((s) => s[1]), [750, 690]) // h - marginY - (k+0.5)*pointW
})

test('canonicalCheckerSlots compresses a stack too tall for the quadrant', () => {
  // quadH 360 / pointW 60 holds 6 checkers; 9 must still fit inside the quad.
  const slots = canonicalCheckerSlots(13, 9)
  assert.equal(slots.length, 9)
  const first = slots[0][1]
  const last = slots[8][1]
  assert.ok(first >= DEFAULT_CANONICAL.marginY, `first ${first} inside the quad`)
  assert.ok(last <= DEFAULT_CANONICAL.marginY + DEFAULT_CANONICAL.quadH, `last ${last} inside the quad`)
  // Still monotonic, still in stacking order.
  for (let i = 1; i < slots.length; i++) assert.ok(slots[i][1] > slots[i - 1][1])
})

test('canonicalCheckerSlots returns nothing for an empty point', () => {
  assert.deepEqual(canonicalCheckerSlots(13, 0), [])
})

test('checkerRadiusOnFrame measures the projected checker size', () => {
  const identity = (x, y) => [x, y]
  assert.equal(checkerRadiusOnFrame(identity, [100, 100]), DEFAULT_CANONICAL.pointW / 2)
  // A projection that halves y halves the drawn checker.
  const squashed = (x, y) => [x, y / 2]
  assert.equal(checkerRadiusOnFrame(squashed, [100, 100]), DEFAULT_CANONICAL.pointW / 4)
  // Never degenerates to zero, whatever the projection does.
  assert.ok(checkerRadiusOnFrame(() => [0, 0], [100, 100]) >= 2)
})
