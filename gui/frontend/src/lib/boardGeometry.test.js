import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  isTop, colOf, stack, BAR_COL,
  transformPoint, flipH, flipHorizontal,
  orientationName, parseOrientation,
  P1_HOME_RIGHT, P1_HOME_LEFT,
} from './boardGeometry.js'

test('isTop splits the two rows at 13', () => {
  assert.equal(isTop(13), true)
  assert.equal(isTop(24), true)
  assert.equal(isTop(12), false)
  assert.equal(isTop(1), false)
})

test('colOf maps points to columns, skipping the bar column', () => {
  assert.equal(colOf(13), 0)
  assert.equal(colOf(18), 5)
  assert.equal(colOf(19), 7)
  assert.equal(colOf(24), 12)
  assert.equal(colOf(12), 0)
  assert.equal(colOf(7), 5)
  assert.equal(colOf(6), 7)
  assert.equal(colOf(1), 12)
  // No point ever lands on the bar column.
  for (let p = 1; p <= 24; p++) assert.notEqual(colOf(p), BAR_COL)
})

const ALL = [P1_HOME_RIGHT, P1_HOME_LEFT]

test('transformPoint identity leaves every point unchanged', () => {
  for (let p = 1; p <= 24; p++) assert.equal(transformPoint(P1_HOME_RIGHT, p), p)
})

test('transformPoint matches the Go bg.Orientation anchors', () => {
  // Same anchors asserted in internal/bg/orientation_test.go.
  assert.equal(transformPoint(P1_HOME_LEFT, 1), 12)
  assert.equal(transformPoint(P1_HOME_LEFT, 6), 7)
  assert.equal(transformPoint(P1_HOME_LEFT, 24), 13)
  assert.equal(transformPoint(P1_HOME_LEFT, 13), 24)
})

// The mirror never moves a point across the rows — the property that keeps
// Player 1 on the bottom row whatever the orientation (ADR-0009).
test('transformPoint never exchanges the rows', () => {
  for (const o of ALL) {
    for (let p = 1; p <= 24; p++) {
      assert.equal(p >= 13, transformPoint(o, p) >= 13)
    }
  }
})

test('transformPoint is an involution and a bijection for every orientation', () => {
  for (const o of ALL) {
    const seen = new Set()
    for (let p = 1; p <= 24; p++) {
      const q = transformPoint(o, p)
      assert.ok(q >= 1 && q <= 24)
      assert.equal(transformPoint(o, q), p) // involution
      seen.add(q)
    }
    assert.equal(seen.size, 24) // bijection
  }
})

test('the only flip is the left/right mirror', () => {
  assert.equal(flipHorizontal(P1_HOME_RIGHT), P1_HOME_LEFT)
  assert.equal(flipHorizontal(flipHorizontal(P1_HOME_RIGHT)), P1_HOME_RIGHT)
  assert.equal(flipH(P1_HOME_LEFT), true)
  assert.equal(flipH(P1_HOME_RIGHT), false)
})

test('orientationName / parseOrientation round-trip and migrate every legacy string', () => {
  for (const o of ALL) assert.equal(parseOrientation(orientationName(o)), o)
  assert.equal(parseOrientation('p1-right'), P1_HOME_RIGHT)
  assert.equal(parseOrientation('p1-bottom'), P1_HOME_RIGHT)
  assert.equal(parseOrientation('p1-home-bottom-right'), P1_HOME_RIGHT)
  assert.equal(parseOrientation('p1-left'), P1_HOME_LEFT)
  assert.equal(parseOrientation('p1-home-bottom-left'), P1_HOME_LEFT)
  // The four-value era's top forms keep the home boards on their side; the
  // player exchange that completes the migration happens in Go, at load.
  assert.equal(parseOrientation('p1-home-top-right'), P1_HOME_RIGHT)
  assert.equal(parseOrientation('p1-home-top-left'), P1_HOME_LEFT)
  assert.equal(parseOrientation(''), P1_HOME_RIGHT)
  assert.equal(parseOrientation('nonsense'), P1_HOME_RIGHT)
})

test('stack caps drawn checkers and reports overflow count', () => {
  assert.deepEqual(stack(0), { drawn: 0, overflow: 0 })
  assert.deepEqual(stack(3), { drawn: 3, overflow: 0 })
  assert.deepEqual(stack(5), { drawn: 5, overflow: 0 })
  assert.deepEqual(stack(8), { drawn: 5, overflow: 8 })
  assert.deepEqual(stack(15, 5), { drawn: 5, overflow: 15 })
})
