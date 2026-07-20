import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  canonicalSize, canonicalGrid, solveHomography, projectPoint, gridOnFrame, DEFAULT_CANONICAL,
  homographyFromCorners, canonicalPointCenters, roiBBox,
} from './calibration.js'

const approx = (a, b, eps = 1e-6) => assert.ok(Math.abs(a - b) <= eps, `${a} ≈ ${b}`)

test('canonicalSize matches calibrate.DefaultCanonical (860×800)', () => {
  const { w, h } = canonicalSize()
  assert.equal(w, 860)
  assert.equal(h, 800)
})

test('canonicalGrid has the outer border, 24 cells, bar and tray', () => {
  const { lines } = canonicalGrid()
  assert.equal(lines.length, 1 + 24 + 1 + 1) // border + cells + bar + off
  for (const l of lines) assert.deepEqual(l[0], l[l.length - 1]) // closed loops
})

test('solveHomography recovers a pure translation+scale (affine) map', () => {
  // domain unit-ish square -> range scaled by 2 and shifted by (10,20).
  const domain = [[0, 0], [100, 0], [100, 80], [0, 80]]
  const range = domain.map(([x, y]) => [2 * x + 10, 2 * y + 20])
  const H = solveHomography(domain, range)
  assert.ok(H)
  for (let i = 0; i < 4; i++) {
    const [X, Y] = projectPoint(H, domain[i])
    approx(X, range[i][0])
    approx(Y, range[i][1])
  }
  // An interior point follows the same affine map.
  const [mx, my] = projectPoint(H, [50, 40])
  approx(mx, 110)
  approx(my, 100)
})

test('solveHomography handles a genuine perspective quad', () => {
  const domain = [[0, 0], [860, 0], [860, 800], [0, 800]]
  const range = [[100, 120], [980, 90], [1020, 720], [60, 690]] // trapezoid
  const H = solveHomography(domain, range)
  assert.ok(H)
  for (let i = 0; i < 4; i++) {
    const [X, Y] = projectPoint(H, domain[i])
    approx(X, range[i][0], 1e-4)
    approx(Y, range[i][1], 1e-4)
  }
})

test('solveHomography returns null for collinear (degenerate) corners', () => {
  const domain = [[0, 0], [860, 0], [860, 800], [0, 800]]
  const bad = [[0, 0], [10, 0], [20, 0], [30, 0]] // all on a line
  assert.equal(solveHomography(domain, bad), null)
})

test('gridOnFrame projects the outer border exactly onto the clicked corners', () => {
  const corners = [[100, 120], [980, 90], [1020, 720], [60, 690]]
  const grid = gridOnFrame(corners)
  assert.ok(grid)
  const border = grid[0] // first line is the outer rectangle loop
  // Its four corners must land on the clicked corners (TL,TR,BR,BL).
  for (let i = 0; i < 4; i++) {
    approx(border[i][0], corners[i][0], 1e-3)
    approx(border[i][1], corners[i][1], 1e-3)
  }
})

test('gridOnFrame rejects the wrong number of corners', () => {
  assert.equal(gridOnFrame([[0, 0], [1, 1]]), null)
  assert.equal(gridOnFrame(null), null)
})

test('homographyFromCorners maps canonical corners onto the clicked corners', () => {
  const corners = [[100, 120], [980, 90], [1020, 720], [60, 690]]
  const H = homographyFromCorners(corners)
  const { w, h } = canonicalSize()
  const canon = [[0, 0], [w, 0], [w, h], [0, h]]
  for (let i = 0; i < 4; i++) {
    const [X, Y] = projectPoint(H, canon[i])
    approx(X, corners[i][0], 1e-3)
    approx(Y, corners[i][1], 1e-3)
  }
  assert.equal(homographyFromCorners([[0, 0]]), null)
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

test('roiBBox is the corner bounding box expanded by the margin', () => {
  const box = roiBBox([[100, 200], [300, 210], [310, 400], [90, 390]], 0)
  assert.deepEqual(box, { x: 90, y: 200, w: 220, h: 200 })
  const m = roiBBox([[0, 0], [100, 0], [100, 100], [0, 100]], 0.1)
  assert.deepEqual(m, { x: -10, y: -10, w: 120, h: 120 })
  assert.equal(roiBBox(null), null)
})
