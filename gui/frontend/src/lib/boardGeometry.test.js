import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  isTop, colOf, stack, BAR_COL,
  transformPoint, flipH, flipV, flipHorizontal, flipVertical,
  P1_HOME_BOTTOM_RIGHT, P1_HOME_BOTTOM_LEFT, P1_HOME_TOP_RIGHT, P1_HOME_TOP_LEFT,
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

const ALL = [P1_HOME_BOTTOM_RIGHT, P1_HOME_BOTTOM_LEFT, P1_HOME_TOP_RIGHT, P1_HOME_TOP_LEFT]

test('transformPoint identity leaves every point unchanged', () => {
  for (let p = 1; p <= 24; p++) assert.equal(transformPoint(P1_HOME_BOTTOM_RIGHT, p), p)
})

test('transformPoint matches the Go bg.Orientation anchors', () => {
  // Same anchors asserted in internal/bg/orientation_test.go.
  assert.equal(transformPoint(P1_HOME_BOTTOM_LEFT, 1), 12)
  assert.equal(transformPoint(P1_HOME_BOTTOM_LEFT, 6), 7)
  assert.equal(transformPoint(P1_HOME_BOTTOM_LEFT, 24), 13)
  assert.equal(transformPoint(P1_HOME_TOP_RIGHT, 1), 24)
  assert.equal(transformPoint(P1_HOME_TOP_RIGHT, 6), 19)
  assert.equal(transformPoint(P1_HOME_TOP_LEFT, 1), 13)
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

test('flip helpers toggle the expected mirror bits', () => {
  assert.equal(flipHorizontal(P1_HOME_BOTTOM_RIGHT), P1_HOME_BOTTOM_LEFT)
  assert.equal(flipVertical(P1_HOME_BOTTOM_RIGHT), P1_HOME_TOP_RIGHT)
  assert.equal(flipVertical(flipHorizontal(P1_HOME_BOTTOM_RIGHT)), P1_HOME_TOP_LEFT)
  assert.equal(flipH(P1_HOME_TOP_LEFT), true)
  assert.equal(flipV(P1_HOME_TOP_LEFT), true)
  assert.equal(flipH(P1_HOME_TOP_RIGHT), false)
})

test('stack caps drawn checkers and reports overflow count', () => {
  assert.deepEqual(stack(0), { drawn: 0, overflow: 0 })
  assert.deepEqual(stack(3), { drawn: 3, overflow: 0 })
  assert.deepEqual(stack(5), { drawn: 5, overflow: 0 })
  assert.deepEqual(stack(8), { drawn: 5, overflow: 8 })
  assert.deepEqual(stack(15, 5), { drawn: 5, overflow: 15 })
})
