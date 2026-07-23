// Checker ink derived from the checker's own fill (issue #62).
//
// A checker is drawn with three colours — fill, outline, and the digit printed
// on an over-tall stack — but only the fill is declared by the user. The other
// two used to come from a table indexed by owner that assumed P1 dark / P2
// light, which is the OPPOSITE of the form's defaults: the digit was then
// written light-on-light for P1 and dark-on-dark for P2, invisible in both
// cases, and the fallback palette painted P1 in P2's colour.
//
// A fixed table cannot serve an arbitrary declared pair anyway — the corpus
// runs teal/yellow, blue/white, ivory/charcoal, and nothing stops a user from
// declaring two mid browns. So derive both from the fill.
// Framework-free → unit-tested with node:test.

// parseHex accepts "#rgb", "#rrggbb" and the same without the hash, and
// returns [r,g,b] in 0..255. Anything else is null — callers degrade, they
// never throw: a bad colour must not take the board renderer down.
export function parseHex(s) {
  if (typeof s !== 'string') return null
  const h = s.trim().replace(/^#/, '')
  if (!/^[0-9a-fA-F]+$/.test(h)) return null
  if (h.length === 3) {
    return [0, 1, 2].map((i) => parseInt(h[i] + h[i], 16))
  }
  if (h.length === 6) {
    return [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16))
  }
  return null
}

// luminance is the WCAG relative luminance of a fill, 0 (black) to 1 (white).
// Unparseable fills read as mid grey so the derived ink stays defined.
export function luminance(fill) {
  const rgb = parseHex(fill)
  if (!rgb) return 0.5
  const lin = rgb.map((v) => {
    const c = v / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2]
}

const INK_DARK = '#111111'
const INK_LIGHT = '#f5f5f5'

// labelOn returns the colour of the stack-count digit printed on a checker of
// this fill: whichever of near-black / near-white the fill is furthest from.
// An unparseable fill reads as dark — two.js leaves such a shape unpainted over
// the board's dark surface, so light ink is the one that stays visible.
export function labelOn(fill) {
  if (!parseHex(fill)) return INK_LIGHT
  return luminance(fill) > 0.45 ? INK_DARK : INK_LIGHT
}

// outlineOf returns the checker's rim: the fill pushed away from itself in the
// readable direction (a light checker gets a darker rim, a dark one a lighter
// rim) so a checker never dissolves into a point or into the checker below it.
// The push is multiplicative on the fill's own channels, so the rim keeps the
// checker's hue instead of introducing a foreign colour; pure black and pure
// white cannot be scaled, so they fall back to a fixed step.
export function outlineOf(fill) {
  const rgb = parseHex(fill)
  if (!rgb) return '#000000'
  const light = luminance(fill) <= 0.45
  const out = rgb.map((v) => (light ? v + (255 - v) * 0.45 + 24 : v * 0.45))
  return '#' + out.map((v) => Math.max(0, Math.min(255, Math.round(v))).toString(16).padStart(2, '0')).join('')
}
