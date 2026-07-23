// Pure board geometry for the two.js renderer (issue #33). Keeps lazyBG's
// existing coordinate semantics (bg.Board points 1..24, P1 home bottom-right)
// so the rest of the app is unaffected; only the rendering upgrades to two.js.
// Framework-free → unit-tested with node:test.

export const NCOLS = 14 // 12 point columns + bar column + off tray column
export const BAR_COL = 6 // the bar sits between the two half-boards
export const OFF_COL = 13 // borne-off tray column (far right)
export const MAX_STACK = 5 // checkers drawn before collapsing to a count

// isTop reports whether point p (1..24) is drawn on the top row.
export function isTop(p) {
  return p >= 13
}

// colOf maps a point (1..24) to its column index (0..12). Top row: 13..18 →
// 0..5, 19..24 → 7..12. Bottom row: 12..7 → 0..5, 6..1 → 7..12. The bar column
// (6) is skipped by construction.
export function colOf(p) {
  if (p >= 13 && p <= 18) return p - 13
  if (p >= 19 && p <= 24) return p - 12
  if (p >= 7 && p <= 12) return 12 - p
  return 13 - p // 1..6
}

// Orientation is the board's on-screen orientation (ADR-0006, ADR-0009),
// mirroring the Go bg.Orientation enum: which half of the video holds the two
// home boards. The integer values MUST match internal/bg/orientation.go
// (bit 0 = horizontal mirror) — they cross the Wails boundary as ints.
//
// Player 1 is the player at the BOTTOM of the video, always (ADR-0009), so
// there is no vertical mirror: when the near player is the one entered second,
// the two players are exchanged (session.SwapPlayers), not the board.
export const P1_HOME_RIGHT = 0 // canonical reference / identity
export const P1_HOME_LEFT = 1 // horizontal mirror

// flipH reports whether orientation o mirrors left↔right.
export function flipH(o) {
  return (o & 1) !== 0
}

// flipHorizontal toggles the mirror — the single WYSIWYG mirror button (#37).
export function flipHorizontal(o) {
  return o ^ 1
}

// transformPoint maps between a canonical point (1..24) and the point occupying
// the same on-screen position under orientation o — the JS twin of
// bg.Orientation.TransformPoint, an involution serving both boundaries. The
// renderer mirrors geometry directly (equivalent); this exists for parity tests
// and any data-space use. Points outside 1..24 pass through unchanged. Rows are
// never exchanged: that is what keeps Player 1 at the bottom (ADR-0009).
export function transformPoint(o, p) {
  if (p < 1 || p > 24) return p
  // Cell in the calibrate canonical grid: top row 13..24 at cols 0..11,
  // bottom row 12..1 at cols 0..11.
  let col = p >= 13 ? p - 13 : 12 - p
  const top = p >= 13
  if (flipH(o)) col = 11 - col
  return top ? 13 + col : 12 - col
}

// ORIENTATION_NAMES are the canonical persistence strings, indexed by value —
// they MUST match bg.Orientation.String() (internal/bg/orientation.go).
export const ORIENTATION_NAMES = ['p1-home-right', 'p1-home-left']

// orientationName renders an orientation value as its persistence string.
export function orientationName(o) {
  return ORIENTATION_NAMES[o] ?? ORIENTATION_NAMES[0]
}

// parseOrientation maps a stored string to an orientation value, migrating every
// vocabulary this repo has written (ADR-0006, ADR-0009). The two 'p1-home-top-*'
// forms land on the side that keeps the home boards where they are; the player
// exchange that completes reading such a document happens in Go, at load
// (session.migrateLegacyTopOrientation), so the form never sees them.
export function parseOrientation(s) {
  const i = ORIENTATION_NAMES.indexOf(s)
  if (i >= 0) return i
  if (s === 'p1-home-bottom-left' || s === 'p1-home-top-left' || s === 'p1-left') return P1_HOME_LEFT
  return P1_HOME_RIGHT // p1-home-bottom-right, p1-home-top-right, p1-right, p1-bottom, '', unknown
}

// stack returns how many checkers to draw for a stack of n and, when n exceeds
// the cap, the total to print as a count on the last checker.
export function stack(n, cap = MAX_STACK) {
  if (n <= 0) return { drawn: 0, overflow: 0 }
  if (n <= cap) return { drawn: n, overflow: 0 }
  return { drawn: cap, overflow: n }
}
