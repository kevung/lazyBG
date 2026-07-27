// Pure keyboard-intent reducer for the transcription/review screen.
//
// App.svelte's onKeydown is a single large switch whose branching — two-digit
// dice accumulation, edit-vs-live dispatch, the cube menu swallowing navigation,
// the override escape hatch gating on an entered roll — is exactly the intricate
// logic that was "compile/vet-checked only" (issue #42) because it never ran on a
// webkit build machine. This module extracts that branching into a pure function
// so every guard is unit-testable under `node --test`, with no Wails runtime.
//
// Division of labour: keyIntent decides *what* a key means given a state
// snapshot and returns an intent; the component owns the async Go bindings and
// the DOM (video element, focus) and executes it. The intent carries:
//   - action:        a string the component switches on to run the effect
//   - preventDefault: whether the component should call e.preventDefault()
//   - patch:         reactive-state fields to assign before the effect runs
//   - payload:       action-specific fields (dir, d1/d2, delta, flag, index)
//
// The branch ORDER mirrors onKeydown deliberately: Tab, digits, delete and
// insert are resolved BEFORE the cube-menu capture, so they win even while the
// cube menu is open — matching the shipped behaviour, quirks included.

const DIGITS = new Set(['1', '2', '3', '4', '5', '6'])

// A key that is ignored entirely (not even preventDefault'd), so the browser /
// form fields keep it.
const IGNORE = Object.freeze({ action: 'none', preventDefault: false })

// A key that is consumed (preventDefault) but triggers no effect or state change
// — e.g. an arrow with nothing to navigate, or a guarded shortcut whose guard
// failed. Matches onKeydown calling e.preventDefault() unconditionally in those
// branches.
const SWALLOW = Object.freeze({ action: 'none', preventDefault: true })

/**
 * @param {object} state snapshot of the reactive component state:
 *   setupOpen, inFormField (booleans); firstDigit (number|null);
 *   selectedSeq (number, -1 = live); candidateCount, highlight (numbers);
 *   cubeCount, cubeHighlight (numbers); editRoll (truthy when editing a roll);
 *   hasVideo (boolean).
 * @param {string} key the KeyboardEvent.key
 * @param {boolean} shiftKey
 * @returns {{action: string, preventDefault: boolean, patch?: object, [k: string]: any}}
 */
export function keyIntent(state, key, shiftKey = false) {
  const {
    setupOpen = false,
    inFormField = false,
    firstDigit = null,
    selectedSeq = -1,
    candidateCount = 0,
    highlight = 0,
    cubeCount = 0,
    cubeHighlight = 0,
    editRoll = null,
    hasVideo = false,
  } = state || {}

  // The setup form and any focused text field own the keyboard.
  if (setupOpen || inFormField) return IGNORE

  const hasCandidates = candidateCount > 0
  const selected = selectedSeq >= 0

  // 1. Tab / Shift+Tab — walk candidate ticks without committing anything.
  if (key === 'Tab') {
    return { action: 'nav-tick', dir: shiftKey ? -1 : 1, preventDefault: true }
  }

  // 2. Two-digit dice entry: first digit is held, the second dispatches.
  if (DIGITS.has(key)) {
    const d = Number(key)
    if (firstDigit === null) {
      return { action: 'accumulate', patch: { firstDigit: d }, preventDefault: true }
    }
    return {
      action: selected ? 'edit-dice' : 'enter-dice',
      d1: firstDigit,
      d2: d,
      patch: { firstDigit: null },
      preventDefault: true,
    }
  }

  // 3. Delete the selected turn — only when not mid-candidate.
  if ((key === 'Delete' || key === 'x') && selected && !hasCandidates) {
    return { action: 'delete', preventDefault: true }
  }

  // 4. Insert a skipped turn before the selected one (issue #25).
  if (key === 'i' && selected && !hasCandidates) {
    return { action: 'insert', preventDefault: true }
  }

  // 5. While the cube menu is open it captures navigation/confirm; any other
  //    key is swallowed (onKeydown preventDefaults + returns unconditionally).
  if (cubeCount > 0) {
    switch (key) {
      case 'ArrowDown':
      case 'j':
        return { action: 'cube-nav', patch: { cubeHighlight: Math.min(cubeHighlight + 1, cubeCount - 1) }, preventDefault: true }
      case 'ArrowUp':
      case 'k':
        return { action: 'cube-nav', patch: { cubeHighlight: Math.max(cubeHighlight - 1, 0) }, preventDefault: true }
      case ' ':
      case 'Enter':
        return { action: 'cube-confirm', preventDefault: true }
      case 'Escape':
        return { action: 'cube-cancel', patch: { clearCube: true }, preventDefault: true }
      default:
        return SWALLOW
    }
  }

  // 6. Default context: candidate navigation, confirm, seek, and shortcuts.
  switch (key) {
    case 'ArrowDown':
    case 'j':
      return { action: 'cand-nav', patch: hasCandidates ? { highlight: Math.min(highlight + 1, candidateCount - 1) } : {}, preventDefault: true }
    case 'ArrowUp':
    case 'k':
      return { action: 'cand-nav', patch: hasCandidates ? { highlight: Math.max(highlight - 1, 0) } : {}, preventDefault: true }
    case 'ArrowLeft':
      return hasVideo ? { action: 'seek', delta: -5, preventDefault: true } : SWALLOW
    case 'ArrowRight':
      return hasVideo ? { action: 'seek', delta: 5, preventDefault: true } : SWALLOW
    case ' ':
    case 'Enter':
      // In edit mode with a re-typed roll, Space applies the picked notation to
      // the past turn; otherwise it confirms the highlighted live candidate.
      // Shift+Space additionally opens a human-flagged Review Item (ux-spec §2).
      if (selected && editRoll && hasCandidates) {
        return { action: 'confirm-edit', index: highlight, preventDefault: true }
      }
      return { action: 'confirm', flag: shiftKey, preventDefault: true }
    case 'Escape':
      // Abandon any in-progress entry; if a past turn was selected, return to
      // the live tail.
      return { action: 'escape', backToLive: selected, patch: { firstDigit: null, clearCandidates: true, overrideOpen: false }, preventDefault: true }
    case 'p':
      return { action: 'toggle-player', preventDefault: true }
    case 'c':
    case 'C':
      // Cube decision precedes a roll, so it is a separate entry point — only
      // valid before a roll is being entered (ux-spec §9).
      if (firstDigit === null && !hasCandidates) return { action: 'cube-open', preventDefault: true }
      return SWALLOW
    case 'o':
    case 'O':
      // The override free-entry hatch — one key from the candidate list, never
      // the default (ADR-0001); needs the dice entered first.
      if (hasCandidates) return { action: 'override-open', patch: { overrideOpen: true }, preventDefault: true }
      return SWALLOW
    default:
      return IGNORE
  }
}
