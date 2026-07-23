// Board-calibration geometry for the setup screen and the perception overlay
// (ADR-0007, issues #36/#44/#45). Pure functions, unit-tested with node:test.
//
// Calibration is TWO homographies split by the bar: the left and right half-
// boards each map their six point columns from the outer playing edge to the bar
// edge. Given the eight source handles (4 corners + 4 bar edges) we can project
// any canonical point onto the frame (grid drawing, overlay de-projection). When
// bar edges are absent we migrate — a single full-quad homography, reproducing
// the legacy v1 grid — so old sessions still draw. Mirrors internal/calibrate.

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

const colX = (cb, c) => cb.marginX + c * cb.pointW + (c >= 6 ? cb.barGap : 0)

// landmarks returns the eight canonical calibration points in the order
// [TL, TR, BR, BL, barTL, barTR, barBR, barBL] — mirrors calibrate.landmarks().
export function landmarks(cb = DEFAULT_CANONICAL) {
  const { h } = canonicalSize(cb)
  const my = cb.marginY
  const by = h - cb.marginY
  const lx = cb.marginX
  const rx = colX(cb, 11) + cb.pointW // outer right playing edge (before the off tray)
  const blx = cb.marginX + 6 * cb.pointW // bar left edge
  const brx = colX(cb, 6) // bar right edge
  return [
    [lx, my], [rx, my], [rx, by], [lx, by], // TL,TR,BR,BL
    [blx, my], [brx, my], [brx, by], [blx, by], // bar edges
  ]
}

// splitX is the canonical x dividing the two halves (bar centre).
function splitX(cb) {
  const lm = landmarks(cb)
  return (lm[4][0] + lm[5][0]) / 2
}

function rectLoop(x, y, w, h) {
  return [[x, y], [x + w, y], [x + w, y + h], [x, y + h], [x, y]]
}

// canonicalGridLines returns the closed polylines (canonical coords) drawn on the
// frame: the playing border, the 24 point cells, and the central bar.
export function canonicalGridLines(cb = DEFAULT_CANONICAL) {
  const lm = landmarks(cb)
  const { h } = canonicalSize(cb)
  const lines = [[lm[0], lm[1], lm[2], lm[3], lm[0]]] // playing border
  for (let c = 0; c < 12; c++) {
    lines.push(rectLoop(colX(cb, c), cb.marginY, cb.pointW, cb.quadH)) // top cell
    lines.push(rectLoop(colX(cb, c), h - cb.marginY - cb.quadH, cb.pointW, cb.quadH)) // bottom cell
  }
  lines.push(rectLoop(lm[4][0], cb.marginY, lm[5][0] - lm[4][0], h - 2 * cb.marginY)) // bar
  return lines
}

// solveHomography returns the 3×3 matrix (row-major, 9 numbers, h[8]=1) mapping
// the four domain points to the four range points, in the given order. Points
// are [x,y]. Returns null if the system is degenerate (collinear corners).
export function solveHomography(domain, range) {
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

// buildCalibration returns { left, right, splitX } — two canonical→IDEAL-source
// homographies split at the bar — from the four corners (TL,TR,BR,BL) and four
// bar edges (barTL,barTR,barBR,barBL). With no valid bar edges it migrates: a
// single full-quad homography used for both halves (legacy v1). Null if degenerate.
//
// The eight handles are points of the RECORDED frame, so an active lens is
// undistorted out of them first — exactly what calibrate.NewSplitWithLens does.
// Skipping that step (the pre-#61 behaviour) put every drawn grid vertex at
// distort(handle) instead of handle: the grid visibly detached from the handles
// on lens-active captures, and did not show what the Go reader actually uses.
export function buildCalibration(corners, barEdges, cb = DEFAULT_CANONICAL, lens = null) {
  if (!corners || corners.length !== 4) return null
  const lm = landmarks(cb)
  const sx = splitX(cb)
  const ideal = (p) => undistortPoint(lens, p)
  if (barEdges && barEdges.length === 4) {
    const leftCanon = [lm[0], lm[4], lm[7], lm[3]]
    const leftSrc = [corners[0], barEdges[0], barEdges[3], corners[3]].map(ideal)
    const rightCanon = [lm[5], lm[1], lm[2], lm[6]]
    const rightSrc = [barEdges[1], corners[1], corners[2], barEdges[2]].map(ideal)
    const left = solveHomography(leftCanon, leftSrc)
    const right = solveHomography(rightCanon, rightSrc)
    if (!left || !right) return null
    return { left, right, splitX: sx }
  }
  // Migrate: single homography from the full canonical rect to the corners.
  const { w, h } = canonicalSize(cb)
  const H = solveHomography([[0, 0], [w, 0], [w, h], [0, h]], corners.map(ideal))
  if (!H) return null
  return { left: H, right: H, splitX: sx }
}

// projectCanonical projects a canonical point onto the frame via the half whose
// homography owns it (left of the bar centre → left, else right).
export function projectCanonical(cal, [x, y]) {
  return projectPoint(x < cal.splitX ? cal.left : cal.right, [x, y])
}

// lensActive mirrors calibrate.Lens.active(): the zero value is the identity.
export function lensActive(lens) {
  return !!lens && lens.norm > 0 && ((lens.k1 || 0) !== 0 || (lens.k2 || 0) !== 0)
}

// distortPoint maps an ideal (pinhole) point to the recorded point — mirrors
// calibrate.Lens.distort (Brown-Conrady k1·r² + k2·r⁴, radius normalised by
// lens.norm around lens.centerX/centerY). Identity when the lens is inactive.
export function distortPoint(lens, [x, y]) {
  if (!lensActive(lens)) return [x, y]
  const dx = x - lens.centerX
  const dy = y - lens.centerY
  const r2 = (dx * dx + dy * dy) / (lens.norm * lens.norm)
  const f = 1 + (lens.k1 || 0) * r2 + (lens.k2 || 0) * r2 * r2
  return [lens.centerX + dx * f, lens.centerY + dy * f]
}

// undistortPoint maps a RECORDED point back to the ideal (pinhole) point —
// mirrors calibrate.Lens.undistort: Newton on the radial quintic
// Rd = Ru + k1·Ru³ + k2·Ru⁵ (stable for barrel, where the naive fixed point
// diverges at the periphery). Identity when the lens is inactive.
export function undistortPoint(lens, [x, y]) {
  if (!lensActive(lens)) return [x, y]
  const dx = x - lens.centerX
  const dy = y - lens.centerY
  const rd = Math.hypot(dx, dy) / lens.norm
  if (rd < 1e-9) return [x, y]
  const k1 = lens.k1 || 0
  const k2 = lens.k2 || 0
  let ru = rd
  for (let i = 0; i < 25; i++) {
    const ru2 = ru * ru
    const f = k2 * ru2 * ru2 * ru + k1 * ru2 * ru + ru - rd
    const df = 5 * k2 * ru2 * ru2 + 3 * k1 * ru2 + 1
    if (df === 0) break
    ru -= f / df
  }
  const scale = ru / rd
  return [lens.centerX + dx * scale, lens.centerY + dy * scale]
}

// projectCanonicalLens projects a canonical point onto the RECORDED frame:
// homography first, then the lens — how every overlay dot must be placed when
// a lens is set, or the drawing silently lies on distorted captures.
export function projectCanonicalLens(cal, lens, p) {
  return distortPoint(lens, projectCanonical(cal, p))
}

// sampleLine subdivides a canonical polyline so that, once projected and
// distorted, straight canonical segments render as honest curves. maxStep is
// in canonical units.
function sampleLine(line, maxStep) {
  const out = [line[0]]
  for (let i = 1; i < line.length; i++) {
    const [x0, y0] = line[i - 1]
    const [x1, y1] = line[i]
    const n = Math.max(1, Math.ceil(Math.hypot(x1 - x0, y1 - y0) / maxStep))
    for (let k = 1; k <= n; k++) out.push([x0 + ((x1 - x0) * k) / n, y0 + ((y1 - y0) * k) / n])
  }
  return out
}

// gridOnFrame projects the calibration grid onto the frame from the eight
// handles, or null if degenerate. Every vertex goes through its half's
// homography; with an active lens each segment is sampled and distorted so
// the drawn grid curves exactly like the recorded board (ADR-0008 §9).
export function gridOnFrame(corners, barEdges, cb = DEFAULT_CANONICAL, lens = null) {
  const cal = buildCalibration(corners, barEdges, cb, lens)
  if (!cal) return null
  const lines = canonicalGridLines(cb)
  if (!lensActive(lens)) {
    return lines.map((line) => line.map((p) => projectCanonical(cal, p)))
  }
  return lines.map((line) => sampleLine(line, 20).map((p) => projectCanonicalLens(cal, lens, p)))
}

// canonicalPointCenters returns the canonical-space centre [x,y] of each point's
// stack region, indexed 1..24 (index 0 unused) — matches calibrate.PointRegion.
export function canonicalPointCenters(cb = DEFAULT_CANONICAL) {
  const { h } = canonicalSize(cb)
  const out = new Array(25)
  for (let p = 1; p <= 24; p++) {
    if (p >= 13) {
      const c = p - 13
      out[p] = [colX(cb, c) + cb.pointW / 2, cb.marginY + cb.quadH / 2]
    } else {
      const c = 12 - p
      out[p] = [colX(cb, c) + cb.pointW / 2, h - cb.marginY - cb.quadH / 2]
    }
  }
  return out
}

// canonicalCheckerSlots returns the canonical-space centre [x,y] of each of a
// point's `count` checkers, in stacking order: outer edge first, growing toward
// the felt band — down for the top row (13..24), up for the bottom row (1..12),
// matching calibrate.PointRegion's StackDir.
//
// This is how the perception overlay draws what the LEARNED reader believes:
// one disc per checker it counted, where a real checker would sit. Drawing the
// reader's own belief is what keeps the overlay honest — the alternative, a
// second unmeasured detector, drifts silently from the pipeline it depicts.
//
// A stack taller than the quadrant holds is compressed (real checkers overlap on
// a crowded point too) so it never spills into the felt or off the board.
export function canonicalCheckerSlots(p, count, cb = DEFAULT_CANONICAL) {
  if (!(count > 0)) return []
  const { h } = canonicalSize(cb)
  const down = p >= 13
  const x = colX(cb, down ? p - 13 : 12 - p) + cb.pointW / 2
  const base = down ? cb.marginY : h - cb.marginY
  const step = Math.min(cb.pointW, cb.quadH / count)
  const out = new Array(count)
  for (let k = 0; k < count; k++) {
    const along = k * step + step / 2
    out[k] = [x, down ? base + along : base - along]
  }
  return out
}

// checkerRadiusOnFrame returns the on-frame radius (in whatever pixels `proj`
// returns) of a checker sitting at canonical point `pt`. It measures rather than
// assumes: half the projected distance between two canonical points one checker
// diameter apart, sampled at that spot, so a checker at the far end of a
// perspective-squashed board draws smaller — exactly as it appears.
export function checkerRadiusOnFrame(proj, pt, cb = DEFAULT_CANONICAL) {
  const [x, y] = pt
  const a = proj(x, y - cb.pointW / 2)
  const b = proj(x, y + cb.pointW / 2)
  return Math.max(2, Math.hypot(b[0] - a[0], b[1] - a[1]) / 2)
}

// WORKSPACE_MARGIN is how far outside the frame a calibration handle may sit,
// as a fraction of the frame's width/height. It mirrors autocal.quadInBounds'
// 0.15 (internal/autocal/autocal.go) on purpose: everything auto-calibration
// still considers a plausible board is representable — and grabbable — in the
// GUI, and nothing beyond it is. Real captures need this: four committed corpus
// manifests carry a corner a few pixels ABOVE the frame (the playing surface's
// top edge is cropped) and read correctly, so clamping onto the frame itself
// would silently rewrite working calibrations.
export const WORKSPACE_MARGIN = 0.15

// workspaceRect is the drawing/interaction area for the calibration handles:
// the video frame expanded by WORKSPACE_MARGIN on every side, in frame pixels
// (so its origin is negative). Null for a frame of unknown size.
export function workspaceRect(videoW, videoH) {
  if (!videoW || !videoH) return null
  const mx = WORKSPACE_MARGIN * videoW
  const my = WORKSPACE_MARGIN * videoH
  return { x: -mx, y: -my, w: videoW + 2 * mx, h: videoH + 2 * my }
}

// clampToWorkspace confines a handle to the workspace, per axis (so a handle
// dragged past a corner slides along the border instead of sticking). A null
// rect leaves the point untouched — an unknown frame size must not move handles.
export function clampToWorkspace([x, y], rect) {
  if (!rect) return [x, y]
  return [
    Math.min(Math.max(x, rect.x), rect.x + rect.w),
    Math.min(Math.max(y, rect.y), rect.y + rect.h),
  ]
}

// roiBBox returns the axis-aligned bounding box {x,y,w,h} of the given source
// points (corners, optionally plus bar edges), expanded by marginFrac. Returns
// null when no points are given.
export function roiBBox(points, marginFrac = 0.04) {
  if (!points || points.length === 0) return null
  let minX = points[0][0], maxX = minX, minY = points[0][1], maxY = minY
  for (const [x, y] of points) {
    minX = Math.min(minX, x); maxX = Math.max(maxX, x)
    minY = Math.min(minY, y); maxY = Math.max(maxY, y)
  }
  const mx = (maxX - minX) * marginFrac
  const my = (maxY - minY) * marginFrac
  return { x: minX - mx, y: minY - my, w: maxX - minX + 2 * mx, h: maxY - minY + 2 * my }
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
