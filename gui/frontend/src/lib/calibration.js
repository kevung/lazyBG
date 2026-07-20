// Board-calibration geometry for the setup screen and the perception overlay
// (ADR-0006, issues #38 and #36). Pure functions, unit-tested with node:test.
//
// The Go side (internal/calibrate) maps the four clicked source corners
// (TL, TR, BR, BL) onto the canonical rectified board's outer rectangle
// (0,0)-(w,h). To draw where the reader actually looks, we invert that: build
// the canonical grid, then project every canonical point back onto the source
// frame through the SAME homography. A visibly-misaligned grid means the corners
// were clicked on the wooden frame instead of the playing surface.

// DEFAULT_CANONICAL mirrors calibrate.DefaultCanonical() (internal/calibrate).
export const DEFAULT_CANONICAL = {
  marginX: 20,
  marginY: 20,
  pointW: 60,
  quadH: 360,
  barGap: 40,
  offW: 60,
  centerGap: 40, // the felt band between the two quads (calibrate centerGap)
}

// canonicalSize returns the rectified image dimensions for a canonical board.
export function canonicalSize(cb = DEFAULT_CANONICAL) {
  const w = cb.marginX + 12 * cb.pointW + cb.barGap + cb.offW + cb.marginX
  const h = cb.marginY + cb.quadH + cb.centerGap + cb.quadH + cb.marginY
  return { w, h }
}

function rectLoop(x, y, w, h) {
  return [[x, y], [x + w, y], [x + w, y + h], [x, y + h], [x, y]]
}

// canonicalGrid returns the closed polylines (in canonical coordinates) that
// depict where the reader expects the board: the outer border, the 24 point
// cells, the central bar gutter, and the bearoff tray.
export function canonicalGrid(cb = DEFAULT_CANONICAL) {
  const { w, h } = canonicalSize(cb)
  const colX = (c) => cb.marginX + c * cb.pointW + (c >= 6 ? cb.barGap : 0)
  const lines = [rectLoop(0, 0, w, h)]
  for (let c = 0; c < 12; c++) {
    lines.push(rectLoop(colX(c), cb.marginY, cb.pointW, cb.quadH)) // top row cell
    lines.push(rectLoop(colX(c), h - cb.marginY - cb.quadH, cb.pointW, cb.quadH)) // bottom row cell
  }
  const barX = cb.marginX + 6 * cb.pointW
  lines.push(rectLoop(barX, cb.marginY, cb.barGap, h - 2 * cb.marginY)) // bar
  lines.push(rectLoop(w - cb.marginX - cb.offW, cb.marginY, cb.offW, h - 2 * cb.marginY)) // off tray
  return { w, h, lines }
}

// solveHomography returns the 3×3 matrix (row-major, 9 numbers, h[8]=1) mapping
// the four domain points to the four range points, in the given order. Points
// are [x,y]. Returns null if the system is degenerate (collinear corners).
export function solveHomography(domain, range) {
  // Each correspondence contributes two rows of an 8×8 system in h0..h7.
  const A = []
  const b = []
  for (let i = 0; i < 4; i++) {
    const [x, y] = domain[i]
    const [X, Y] = range[i]
    A.push([x, y, 1, 0, 0, 0, -x * X, -y * X])
    b.push(X)
    A.push([0, 0, 0, x, y, 1, -x * Y, -y * Y])
    b.push(Y)
  }
  const h = solveLinear(A, b)
  if (!h) return null
  return [h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1]
}

// projectPoint applies a 3×3 homography to a point [x,y].
export function projectPoint(H, [x, y]) {
  const d = H[6] * x + H[7] * y + H[8]
  return [(H[0] * x + H[1] * y + H[2]) / d, (H[3] * x + H[4] * y + H[5]) / d]
}

// projectLines maps every point of every polyline through H.
export function projectLines(H, lines) {
  return lines.map((line) => line.map((p) => projectPoint(H, p)))
}

// gridOnFrame is the convenience path used by the UI: given the four clicked
// source corners (TL, TR, BR, BL), return the canonical grid projected onto the
// source frame, or null if the corners are degenerate.
export function gridOnFrame(corners, cb = DEFAULT_CANONICAL) {
  if (!corners || corners.length !== 4) return null
  const { w, h, lines } = canonicalGrid(cb)
  const domain = [[0, 0], [w, 0], [w, h], [0, h]] // canonical TL,TR,BR,BL
  const H = solveHomography(domain, corners)
  if (!H) return null
  return projectLines(H, lines)
}

// solveLinear solves the n×n system A x = b by Gaussian elimination with partial
// pivoting. Returns the solution vector, or null if the matrix is singular.
function solveLinear(A, b) {
  const n = b.length
  const m = A.map((row, i) => [...row, b[i]])
  for (let col = 0; col < n; col++) {
    let piv = col
    for (let r = col + 1; r < n; r++) {
      if (Math.abs(m[r][col]) > Math.abs(m[piv][col])) piv = r
    }
    if (Math.abs(m[piv][col]) < 1e-12) return null
    ;[m[col], m[piv]] = [m[piv], m[col]]
    const d = m[col][col]
    for (let k = col; k <= n; k++) m[col][k] /= d
    for (let r = 0; r < n; r++) {
      if (r === col) continue
      const f = m[r][col]
      if (f === 0) continue
      for (let k = col; k <= n; k++) m[r][k] -= f * m[col][k]
    }
  }
  return m.map((row) => row[n])
}
