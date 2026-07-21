import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  canonicalSize, landmarks, canonicalGridLines, solveHomography, projectPoint,
  buildCalibration, projectCanonical, gridOnFrame, canonicalPointCenters, roiBBox,
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
