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

// stack returns how many checkers to draw for a stack of n and, when n exceeds
// the cap, the total to print as a count on the last checker.
export function stack(n, cap = MAX_STACK) {
  if (n <= 0) return { drawn: 0, overflow: 0 }
  if (n <= cap) return { drawn: n, overflow: 0 }
  return { drawn: cap, overflow: n }
}
