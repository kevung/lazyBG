import { test } from 'node:test'
import assert from 'node:assert/strict'
import { snapSpeed, skip, formatTime } from './video.js'

test('snapSpeed clamps to [0.25, 4] and snaps to 0.25 steps', () => {
  assert.equal(snapSpeed(1), 1)
  assert.equal(snapSpeed(1.1), 1) // nearest 0.25
  assert.equal(snapSpeed(1.13), 1.25)
  assert.equal(snapSpeed(0.05), 0.25) // clamp low
  assert.equal(snapSpeed(9), 4) // clamp high
  assert.equal(snapSpeed(0.37), 0.25)
  assert.equal(snapSpeed(0.38), 0.5)
})

test('skip clamps within [0, duration]', () => {
  assert.equal(skip(10, 5, 100), 15)
  assert.equal(skip(10, -5, 100), 5)
  assert.equal(skip(2, -5, 100), 0) // clamp to start
  assert.equal(skip(98, 5, 100), 100) // clamp to end
  assert.equal(skip(50, 5, NaN), 55) // unknown duration → no upper clamp
})

test('formatTime renders m:ss and h:mm:ss', () => {
  assert.equal(formatTime(0), '0:00')
  assert.equal(formatTime(5), '0:05')
  assert.equal(formatTime(65), '1:05')
  assert.equal(formatTime(3661), '1:01:01')
  assert.equal(formatTime(NaN), '0:00')
  assert.equal(formatTime(-3), '0:00')
})
