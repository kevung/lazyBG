// Pure helpers for the custom video control bar (ADR-0004). Kept framework-free
// so they can be unit-tested with node:test; the Svelte component wires them to
// the <video> element's currentTime / playbackRate.

export const MIN_SPEED = 0.25
export const MAX_SPEED = 4
export const SPEED_STEP = 0.25

// snapSpeed clamps v to [MIN_SPEED, MAX_SPEED] and snaps it to the 0.25 grid.
export function snapSpeed(v) {
  const clamped = Math.min(MAX_SPEED, Math.max(MIN_SPEED, v))
  return Math.round(clamped / SPEED_STEP) * SPEED_STEP
}

// skip returns current + delta seconds, clamped to [0, duration]. An unknown
// (NaN) duration leaves the upper bound open.
export function skip(current, delta, duration) {
  let t = current + delta
  if (t < 0) t = 0
  if (Number.isFinite(duration) && t > duration) t = duration
  return t
}

// formatTime renders seconds as m:ss (or h:mm:ss past an hour). Invalid or
// negative input renders 0:00.
export function formatTime(sec) {
  if (!Number.isFinite(sec) || sec < 0) sec = 0
  const s = Math.floor(sec % 60)
  const m = Math.floor((sec / 60) % 60)
  const h = Math.floor(sec / 3600)
  const ss = String(s).padStart(2, '0')
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${ss}`
  return `${m}:${ss}`
}
