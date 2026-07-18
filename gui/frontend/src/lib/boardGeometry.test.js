import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isTop, colOf, stack, BAR_COL } from './boardGeometry.js'

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

test('stack caps drawn checkers and reports overflow count', () => {
  assert.deepEqual(stack(0), { drawn: 0, overflow: 0 })
  assert.deepEqual(stack(3), { drawn: 3, overflow: 0 })
  assert.deepEqual(stack(5), { drawn: 5, overflow: 0 })
  assert.deepEqual(stack(8), { drawn: 5, overflow: 8 })
  assert.deepEqual(stack(15, 5), { drawn: 5, overflow: 15 })
})
