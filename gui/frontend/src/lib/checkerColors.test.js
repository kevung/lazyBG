import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseHex, luminance, labelOn, outlineOf } from './checkerColors.js'

test('parseHex accepts the three CSS forms and rejects the rest', () => {
  assert.deepEqual(parseHex('#e7e0d5'), [231, 224, 213])
  assert.deepEqual(parseHex('e7e0d5'), [231, 224, 213])
  assert.deepEqual(parseHex('#fff'), [255, 255, 255])
  assert.equal(parseHex('rebeccapurple'), null)
  assert.equal(parseHex(''), null)
  assert.equal(parseHex(null), null)
})

test('luminance orders the declared defaults the way the eye does', () => {
  assert.ok(luminance('#ffffff') === 1)
  assert.ok(luminance('#000000') === 0)
  assert.ok(luminance('#e7e0d5') > 0.6) // Player 1 default: light
  assert.ok(luminance('#31221c') < 0.1) // Player 2 default: dark
})

test('labelOn writes dark on a light checker and light on a dark one', () => {
  // The bug this fixes: both defaults used to take the SAME label colour from a
  // table indexed by owner, so one of the two was always invisible.
  assert.equal(labelOn('#e7e0d5'), '#111111')
  assert.equal(labelOn('#31221c'), '#f5f5f5')
  assert.notEqual(labelOn('#e7e0d5'), labelOn('#31221c'))
})

test('labelOn stays legible on an arbitrary declared pair', () => {
  // Two mid browns: a fixed palette cannot serve both, a derived one can.
  for (const fill of ['#8a6b4f', '#5c4433', '#2f6f6a', '#d4af37', '#1e3a8a']) {
    const label = labelOn(fill)
    const lf = luminance(fill)
    const ll = luminance(label)
    assert.ok(Math.abs(lf - ll) > 0.35, `${fill} vs ${label}: too close`)
  }
})

test('outlineOf separates the rim from the fill in the readable direction', () => {
  const dark = outlineOf('#e7e0d5')
  const light = outlineOf('#31221c')
  assert.ok(luminance(dark) < luminance('#e7e0d5'), 'light fill gets a darker outline')
  assert.ok(luminance(light) > luminance('#31221c'), 'dark fill gets a lighter outline')
  // Extremes must not clip to the fill itself.
  assert.notEqual(outlineOf('#ffffff'), '#ffffff')
  assert.notEqual(outlineOf('#000000'), '#000000')
})

test('an unparseable fill degrades instead of throwing', () => {
  assert.equal(labelOn('nonsense'), '#f5f5f5')
  assert.equal(outlineOf('nonsense'), '#000000')
})
