import { test } from 'node:test'
import assert from 'node:assert/strict'
import { keyIntent } from './interaction.js'

// A neutral "live" snapshot: no roll typed, nothing selected, no candidates, no
// cube menu, video present. Individual tests override just what they exercise.
const live = (over = {}) => ({
  setupOpen: false,
  inFormField: false,
  firstDigit: null,
  selectedSeq: -1,
  candidateCount: 0,
  highlight: 0,
  cubeCount: 0,
  cubeHighlight: 0,
  editRoll: null,
  hasVideo: true,
  ...over,
})

// --- keyboard ownership -----------------------------------------------------

test('setup form owns the keyboard: every key ignored', () => {
  for (const k of ['1', 'Tab', ' ', 'Escape', 'c', 'ArrowDown']) {
    const i = keyIntent(live({ setupOpen: true }), k)
    assert.equal(i.action, 'none')
    assert.equal(i.preventDefault, false)
  }
})

test('focused text field owns the keyboard: every key ignored', () => {
  const i = keyIntent(live({ inFormField: true }), '3')
  assert.equal(i.action, 'none')
  assert.equal(i.preventDefault, false)
})

// --- Tab navigation ---------------------------------------------------------

test('Tab walks to the next candidate tick', () => {
  const i = keyIntent(live(), 'Tab')
  assert.equal(i.action, 'nav-tick')
  assert.equal(i.dir, 1)
  assert.equal(i.preventDefault, true)
})

test('Shift+Tab walks to the previous candidate tick', () => {
  const i = keyIntent(live(), 'Tab', true)
  assert.equal(i.action, 'nav-tick')
  assert.equal(i.dir, -1)
})

// --- two-digit dice entry ---------------------------------------------------

test('first digit is held (accumulate), not dispatched', () => {
  const i = keyIntent(live(), '5')
  assert.equal(i.action, 'accumulate')
  assert.deepEqual(i.patch, { firstDigit: 5 })
  assert.equal(i.preventDefault, true)
})

test('second digit dispatches enter-dice in live mode and clears firstDigit', () => {
  const i = keyIntent(live({ firstDigit: 6 }), '3')
  assert.equal(i.action, 'enter-dice')
  assert.equal(i.d1, 6)
  assert.equal(i.d2, 3)
  assert.deepEqual(i.patch, { firstDigit: null })
})

test('second digit dispatches edit-dice when a past turn is selected', () => {
  const i = keyIntent(live({ firstDigit: 4, selectedSeq: 2 }), '4')
  assert.equal(i.action, 'edit-dice')
  assert.equal(i.d1, 4)
  assert.equal(i.d2, 4)
})

test('digits 0/7/8/9 are not dice keys (fall through)', () => {
  for (const k of ['0', '7', '8', '9']) {
    const i = keyIntent(live(), k)
    assert.equal(i.action, 'none', `key ${k}`)
    assert.equal(i.preventDefault, false)
  }
})

// --- delete / insert (require a selected turn, no candidates) ----------------

test('Delete / x remove the selected turn when idle', () => {
  for (const k of ['Delete', 'x']) {
    const i = keyIntent(live({ selectedSeq: 3 }), k)
    assert.equal(i.action, 'delete', `key ${k}`)
    assert.equal(i.preventDefault, true)
  }
})

test('Delete is inert without a selection (browser keeps it)', () => {
  const i = keyIntent(live({ selectedSeq: -1 }), 'Delete')
  assert.equal(i.action, 'none')
  assert.equal(i.preventDefault, false)
})

test('Delete is blocked while candidates are showing', () => {
  const i = keyIntent(live({ selectedSeq: 3, candidateCount: 4 }), 'Delete')
  assert.equal(i.action, 'none')
  // 'x'/Delete with candidates present is not swallowed here — it falls through
  // (no default case matches), so the browser keeps it.
  assert.equal(i.preventDefault, false)
})

test('i inserts a skipped turn before the selected one', () => {
  const i = keyIntent(live({ selectedSeq: 1 }), 'i')
  assert.equal(i.action, 'insert')
  assert.equal(i.preventDefault, true)
})

test('i is inert without a selection', () => {
  const i = keyIntent(live(), 'i')
  assert.equal(i.action, 'none')
  assert.equal(i.preventDefault, false)
})

// --- cube menu capture ------------------------------------------------------

test('cube menu: ArrowDown/j move the highlight down, clamped', () => {
  const down = keyIntent(live({ cubeCount: 3, cubeHighlight: 0 }), 'ArrowDown')
  assert.equal(down.action, 'cube-nav')
  assert.deepEqual(down.patch, { cubeHighlight: 1 })
  const clamp = keyIntent(live({ cubeCount: 3, cubeHighlight: 2 }), 'j')
  assert.deepEqual(clamp.patch, { cubeHighlight: 2 })
})

test('cube menu: ArrowUp/k move the highlight up, clamped at 0', () => {
  const up = keyIntent(live({ cubeCount: 3, cubeHighlight: 2 }), 'k')
  assert.deepEqual(up.patch, { cubeHighlight: 1 })
  const clamp = keyIntent(live({ cubeCount: 3, cubeHighlight: 0 }), 'ArrowUp')
  assert.deepEqual(clamp.patch, { cubeHighlight: 0 })
})

test('cube menu: Space/Enter confirm', () => {
  for (const k of [' ', 'Enter']) {
    assert.equal(keyIntent(live({ cubeCount: 4 }), k).action, 'cube-confirm')
  }
})

test('cube menu: Escape cancels and clears the menu', () => {
  const i = keyIntent(live({ cubeCount: 4 }), 'Escape')
  assert.equal(i.action, 'cube-cancel')
  assert.deepEqual(i.patch, { clearCube: true })
})

test('cube menu swallows unrelated keys (o, p, ArrowLeft)', () => {
  for (const k of ['o', 'p', 'ArrowLeft']) {
    const i = keyIntent(live({ cubeCount: 4 }), k)
    assert.equal(i.action, 'none', `key ${k}`)
    assert.equal(i.preventDefault, true) // swallowed, not passed through
  }
})

test('ordering quirk: digits win over the open cube menu', () => {
  // Tab/digits/delete/insert are resolved before the cube-menu block, matching
  // onKeydown — a digit starts dice entry even with the cube menu open.
  const i = keyIntent(live({ cubeCount: 4 }), '2')
  assert.equal(i.action, 'accumulate')
})

test('ordering quirk: Tab wins over the open cube menu', () => {
  assert.equal(keyIntent(live({ cubeCount: 4 }), 'Tab').action, 'nav-tick')
})

// --- candidate navigation ---------------------------------------------------

test('candidate nav: ArrowDown/j clamp at the last candidate', () => {
  const mid = keyIntent(live({ candidateCount: 3, highlight: 0 }), 'j')
  assert.equal(mid.action, 'cand-nav')
  assert.deepEqual(mid.patch, { highlight: 1 })
  const end = keyIntent(live({ candidateCount: 3, highlight: 2 }), 'ArrowDown')
  assert.deepEqual(end.patch, { highlight: 2 })
})

test('candidate nav: ArrowUp/k clamp at 0', () => {
  const top = keyIntent(live({ candidateCount: 3, highlight: 0 }), 'ArrowUp')
  assert.deepEqual(top.patch, { highlight: 0 })
})

test('candidate nav with no candidates is swallowed but changes nothing', () => {
  const i = keyIntent(live({ candidateCount: 0 }), 'ArrowDown')
  assert.equal(i.action, 'cand-nav')
  assert.deepEqual(i.patch, {})
  assert.equal(i.preventDefault, true)
})

// --- seek -------------------------------------------------------------------

test('ArrowLeft/Right seek when a video is present', () => {
  assert.deepEqual(
    { a: keyIntent(live(), 'ArrowLeft').action, d: keyIntent(live(), 'ArrowLeft').delta },
    { a: 'seek', d: -5 },
  )
  const r = keyIntent(live(), 'ArrowRight')
  assert.equal(r.action, 'seek')
  assert.equal(r.delta, 5)
})

test('seek keys are swallowed (not passed through) even with no video', () => {
  const i = keyIntent(live({ hasVideo: false }), 'ArrowLeft')
  assert.equal(i.action, 'none')
  assert.equal(i.preventDefault, true)
})

// --- confirm ----------------------------------------------------------------

test('Space confirms the highlighted live candidate, no flag', () => {
  const i = keyIntent(live({ candidateCount: 2, highlight: 1 }), ' ')
  assert.equal(i.action, 'confirm')
  assert.equal(i.flag, false)
})

test('Shift+Space confirms and flags a Review Item', () => {
  const i = keyIntent(live({ candidateCount: 2 }), ' ', true)
  assert.equal(i.action, 'confirm')
  assert.equal(i.flag, true)
})

test('Enter behaves like Space for confirm', () => {
  assert.equal(keyIntent(live({ candidateCount: 1 }), 'Enter').action, 'confirm')
})

test('edit mode: Space applies the picked notation to the past turn', () => {
  const i = keyIntent(live({ selectedSeq: 2, editRoll: '3:5', candidateCount: 3, highlight: 2 }), ' ')
  assert.equal(i.action, 'confirm-edit')
  assert.equal(i.index, 2)
})

test('edit mode without candidates falls back to plain confirm', () => {
  const i = keyIntent(live({ selectedSeq: 2, editRoll: '3:5', candidateCount: 0 }), ' ')
  assert.equal(i.action, 'confirm')
})

test('a selected turn without a re-typed roll confirms live (not edit)', () => {
  const i = keyIntent(live({ selectedSeq: 2, editRoll: null, candidateCount: 3 }), ' ')
  assert.equal(i.action, 'confirm')
})

// --- escape -----------------------------------------------------------------

test('Escape clears entry state and returns to live when a turn is selected', () => {
  const i = keyIntent(live({ selectedSeq: 4, firstDigit: 3, candidateCount: 2 }), 'Escape')
  assert.equal(i.action, 'escape')
  assert.equal(i.backToLive, true)
  assert.deepEqual(i.patch, { firstDigit: null, clearCandidates: true, overrideOpen: false })
})

test('Escape at the live tail clears entry but does not call backToLive', () => {
  const i = keyIntent(live({ selectedSeq: -1, firstDigit: 2 }), 'Escape')
  assert.equal(i.action, 'escape')
  assert.equal(i.backToLive, false)
})

// --- shortcuts: player swap, cube open, override ----------------------------

test('p swaps the on-roll player', () => {
  assert.equal(keyIntent(live(), 'p').action, 'toggle-player')
})

test('c/C open the cube menu only before a roll is entered', () => {
  assert.equal(keyIntent(live(), 'c').action, 'cube-open')
  assert.equal(keyIntent(live(), 'C').action, 'cube-open')
})

test('c is inert (swallowed) once a digit is held or candidates show', () => {
  const held = keyIntent(live({ firstDigit: 3 }), 'c')
  assert.equal(held.action, 'none')
  assert.equal(held.preventDefault, true)
  const withCand = keyIntent(live({ candidateCount: 2 }), 'c')
  assert.equal(withCand.action, 'none')
  assert.equal(withCand.preventDefault, true)
})

test('o/O open free-entry override only when candidates are present', () => {
  const open = keyIntent(live({ candidateCount: 3 }), 'o')
  assert.equal(open.action, 'override-open')
  assert.deepEqual(open.patch, { overrideOpen: true })
  const inert = keyIntent(live({ candidateCount: 0 }), 'O')
  assert.equal(inert.action, 'none')
  assert.equal(inert.preventDefault, true) // swallowed, per onKeydown
})

// --- unhandled keys ---------------------------------------------------------

test('an unbound key is fully ignored', () => {
  for (const k of ['z', 'F5', 'Home', 'a']) {
    const i = keyIntent(live(), k)
    assert.equal(i.action, 'none', `key ${k}`)
    assert.equal(i.preventDefault, false)
  }
})

test('missing state defaults to a safe idle snapshot', () => {
  // Robustness: keyIntent must not throw on a partial/empty state.
  assert.equal(keyIntent(undefined, 'z').action, 'none')
  assert.equal(keyIntent({}, 'Tab').action, 'nav-tick')
})
