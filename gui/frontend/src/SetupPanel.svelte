<script>
  // Session Priors + Board Calibration form (ux-spec §10): a standard form —
  // this runs once per Part (and again only for corrections), so no keyboard
  // optimization. The 4 corners are clicked on the current video frame.
  import { createEventDispatcher, onMount } from 'svelte'
  import {
    gridOnFrame, lensActive, DEFAULT_CANONICAL, workspaceRect, clampToWorkspace,
  } from './lib/calibration.js'
  import { orientationName, parseOrientation, flipHorizontal } from './lib/boardGeometry.js'
  import Board from './Board.svelte'

  export let videoEl = null
  export let initial = null // session.Setup for pre-filled correction

  const dispatch = createEventDispatcher()

  let players = ['Player 1', 'Player 2']
  let matchLength = 7
  let clock = true
  // Doubling-cube rules (Session Priors, #24). Match-play defaults.
  let cubeInPlay = true
  let crawford = true
  let jacoby = false
  let beaver = false
  let orientation = 0 // bg.Orientation value (WYSIWYG mirror control, #37)
  // Player 1 is the player at the BOTTOM of the video, always (ADR-0009).
  // Getting the near player into the "Player 1" field is therefore a rename,
  // not a board flip: swapPlies counts the presses so SaveSetup can move the
  // recorded play with the names (an even number cancels out).
  let swapPresses = 0
  let checkerA = '#e7e0d5'
  let checkerB = '#31221c'

  // The opening position, drawn in the orientation preview so the user can see
  // each player's home board and colours and flip until it matches the video.
  const startBoard = (() => {
    const Pts = Array.from({ length: 25 }, () => ({ N: 0, Owner: 0 }))
    const set = (p, n, o) => (Pts[p] = { N: n, Owner: o })
    // Player 1 owns the two back checkers on the 24-point, drawn on the TOP
    // row: its home board — and the player — are at the bottom (ADR-0009).
    set(24, 2, 0); set(13, 5, 0); set(8, 3, 0); set(6, 5, 0) // Player 1
    set(1, 2, 1); set(12, 5, 1); set(17, 3, 1); set(19, 5, 1) // Player 2
    return { Pts, Bar: [0, 0], Off: [0, 0] }
  })()
  // Calibration handles (ADR-0007): four draggable corners + the bar located by
  // four fractions along the top/bottom playing edges, so the reader can rectify
  // each half-board (the bar width is explicit, not a fixed guess).
  let corners = [] // [[x,y]] TL,TR,BR,BL in video-pixel coords
  let barFrac = { tl: 0.47, tr: 0.53, bl: 0.47, br: 0.53 } // along TL-TR / BL-BR
  let dragging = null // handle id ('c0'..'c3','btl','btr','bbr','bbl') or null
  let detecting = false // auto corner-detection running (#47)
  let detectMsg = ''
  // lens is the estimated radial distortion (corpus.Lens shape, null = pinhole).
  // Set by auto-detection only; dragging handles never touches it, and the one
  // manual control is the reset button (ADR-0008 §9 — no coefficient sliders).
  let lens = null
  let videoUrl = ''
  let canvas
  let error = ''

  // detectCorners seeds all eight handles (corners + bar edges) from a single-
  // frame detection on the CURRENT frame (issue #47) — a best-effort start the
  // user refines by dragging. Fast (no video scan); non-blocking.
  async function detectCorners() {
    const fn = window.go?.main?.App?.DetectCorners
    if (!fn) {
      detectMsg = 'Auto-detect unavailable in this build — place the handles by dragging.'
      return
    }
    detecting = true
    detectMsg = 'Detecting the board…'
    try {
      const tick = Math.round((videoEl?.currentTime ?? 0) * 1000)
      const res = await fn(tick)
      if (res?.Corners?.length === 4) {
        corners = res.Corners.map((c) => [...c])
        lens = res.Lens ?? null // a fresh detection replaces the whole estimate
        if (res.BarEdges?.length === 4) {
          const [TL, TR, BR, BL] = corners
          barFrac = {
            tl: fracAlong(res.BarEdges[0], TL, TR),
            tr: fracAlong(res.BarEdges[1], TL, TR),
            br: fracAlong(res.BarEdges[2], BL, BR),
            bl: fracAlong(res.BarEdges[3], BL, BR),
          }
        }
        // Clamped ⇒ the fit put a handle beyond the workspace and the session
        // pulled it back (#61). Say which of the two causes it can be instead
        // of letting the user wonder why the grid is off in one corner.
        detectMsg = res.Clamped
          ? 'The detected board runs past the frame — fix the handles, or this video does not show the whole board.'
          : 'Detected — fine-tune the handles until the grid matches the board.'
        drawFrame()
      } else {
        detectMsg = 'No board detected — place the handles by dragging.'
      }
    } catch (e) {
      // Surface the real reason (auto-detect needs readable board colours on the
      // current frame; it can legitimately fail) instead of a generic message.
      detectMsg = 'Auto-detect failed: ' + (e?.message || String(e)) + ' — place the handles by dragging.'
    }
    detecting = false
  }

  const lerp = (a, b, t) => [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t]

  // barEdges returns [barTL,barTR,barBR,barBL] from the corners and fractions.
  function barEdges() {
    if (corners.length !== 4) return []
    const [TL, TR, BR, BL] = corners
    return [lerp(TL, TR, barFrac.tl), lerp(TL, TR, barFrac.tr), lerp(BL, BR, barFrac.br), lerp(BL, BR, barFrac.bl)]
  }

  // fracAlong is the clamped [0,1] projection of p onto segment a→b.
  function fracAlong(p, a, b) {
    const dx = b[0] - a[0], dy = b[1] - a[1]
    const len2 = dx * dx + dy * dy || 1
    const t = ((p[0] - a[0]) * dx + (p[1] - a[1]) * dy) / len2
    return Math.max(0.02, Math.min(0.98, t))
  }

  // handleList returns the drawable/hit-testable handles.
  function handleList() {
    if (corners.length !== 4) return []
    const be = barEdges()
    return [
      ...corners.map(([x, y], i) => ({ id: 'c' + i, x, y, kind: 'corner', label: String(i + 1) })),
      { id: 'btl', x: be[0][0], y: be[0][1], kind: 'bar' },
      { id: 'btr', x: be[1][0], y: be[1][1], kind: 'bar' },
      { id: 'bbr', x: be[2][0], y: be[2][1], kind: 'bar' },
      { id: 'bbl', x: be[3][0], y: be[3][1], kind: 'bar' },
    ]
  }

  onMount(() => {
    if (initial) {
      if (initial.players?.[0]) players = [...initial.players]
      if (initial.priors) {
        matchLength = initial.priors.matchLength || 7
        clock = !!initial.priors.clock
        // GetSetup resolves these to concrete booleans; default when absent.
        cubeInPlay = initial.priors.cubeInPlay ?? true
        crawford = initial.priors.crawford ?? true
        jacoby = initial.priors.jacoby ?? false
        beaver = initial.priors.beaver ?? false
        orientation = parseOrientation(initial.priors.orientation)
        checkerA = initial.priors.checkerA || checkerA
        checkerB = initial.priors.checkerB || checkerB
      }
      videoUrl = initial.videoUrl || ''
      lens = initial.lens ?? null
    }
    seedCalibration()
    drawFrame()
    window.addEventListener('mousemove', onDrag)
    window.addEventListener('mouseup', endDrag)
    return () => {
      window.removeEventListener('mousemove', onDrag)
      window.removeEventListener('mouseup', endDrag)
    }
  })

  // seedCalibration pre-places the handles so the user adjusts rather than
  // placing from scratch: prior values when correcting, else a default inset.
  // useInitial=false is the reset action — the default inset, whatever the
  // session already held (#61): the one-gesture way back from tangled handles,
  // which "Cancel" cannot offer on a first setup (it is not rendered then).
  function seedCalibration(useInitial = true) {
    const w = videoEl?.videoWidth || 640
    const h = videoEl?.videoHeight || 360
    corners = useInitial && initial?.corners?.length === 4
      ? initial.corners.map((c) => [...c])
      : [[0.15 * w, 0.18 * h], [0.85 * w, 0.18 * h], [0.85 * w, 0.82 * h], [0.15 * w, 0.82 * h]]
    if (!useInitial) {
      barFrac = { tl: 0.47, tr: 0.53, bl: 0.47, br: 0.53 }
      return
    }
    if (initial?.barEdges?.length === 4) {
      const [TL, TR, BR, BL] = corners
      barFrac = {
        tl: fracAlong(initial.barEdges[0], TL, TR),
        tr: fracAlong(initial.barEdges[1], TL, TR),
        br: fracAlong(initial.barEdges[2], BL, BR),
        bl: fracAlong(initial.barEdges[3], BL, BR),
      }
    }
  }

  // workspace is the calibration workspace — the frame plus a 15% margin on
  // every side. The canvas spans it (not the frame), so a handle sitting
  // outside the video is drawn AND reachable by the mouse; before #61 it was
  // clipped away and unrecoverable, since the pointer could never leave the
  // frame. Computed on call, not with `$:` — videoWidth is a DOM property that
  // fills in on loadedmetadata without notifying Svelte.
  const workspace = () => workspaceRect(videoEl?.videoWidth, videoEl?.videoHeight)

  function drawFrame() {
    const ws = workspace()
    if (!canvas || !videoEl || !videoEl.videoWidth || !ws) return
    canvas.width = ws.w
    canvas.height = ws.h
    const ctx = canvas.getContext('2d')
    // The margin reads as "outside the video": neutral fill, then the frame
    // drawn at its true place with a border marking where the capture ends.
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.fillStyle = '#111114'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.setTransform(1, 0, 0, 1, -ws.x, -ws.y) // draw everything in frame px
    ctx.drawImage(videoEl, 0, 0)
    ctx.strokeStyle = '#52525b'
    ctx.lineWidth = Math.max(1, canvas.width / 600)
    ctx.strokeRect(0, 0, videoEl.videoWidth, videoEl.videoHeight)
    // Live dual half-grid: if the 24 cells / bar don't sit on the real triangles,
    // drag the handles until they do (#46).
    const grid = gridOnFrame(corners, barEdges(), DEFAULT_CANONICAL, lens)
    if (grid) {
      ctx.lineWidth = Math.max(1, canvas.width / 600)
      ctx.strokeStyle = '#38bdf8cc'
      for (const line of grid) {
        ctx.beginPath()
        line.forEach(([x, y], i) => (i ? ctx.lineTo(x, y) : ctx.moveTo(x, y)))
        ctx.stroke()
      }
    }
    const r = Math.max(6, canvas.width / 120)
    for (const hd of handleList()) {
      ctx.fillStyle = hd.kind === 'corner' ? '#22c55e' : '#f59e0b'
      ctx.strokeStyle = '#000'
      ctx.lineWidth = 2
      ctx.beginPath()
      ctx.arc(hd.x, hd.y, r, 0, 2 * Math.PI)
      ctx.fill()
      ctx.stroke()
      if (hd.label) {
        ctx.fillStyle = '#fff'
        ctx.font = `bold ${Math.round(r * 1.6)}px sans-serif`
        ctx.fillText(hd.label, hd.x + r, hd.y - r)
      }
    }
  }

  // canvasPt converts a pointer event to FRAME pixels — the coordinate system
  // every handle, the manifest and the Go reader use. Outside the video it is
  // legitimately negative or past the frame size, up to the workspace border.
  function canvasPt(e) {
    const rect = canvas.getBoundingClientRect()
    const ws = workspace()
    const ox = ws ? ws.x : 0
    const oy = ws ? ws.y : 0
    return [
      ((e.clientX - rect.left) / rect.width) * canvas.width + ox,
      ((e.clientY - rect.top) / rect.height) * canvas.height + oy,
    ]
  }

  function startDrag(e) {
    if (!canvas) return
    const p = canvasPt(e)
    let best = null, bestD = Infinity
    for (const hd of handleList()) {
      const d = (hd.x - p[0]) ** 2 + (hd.y - p[1]) ** 2
      if (d < bestD) { bestD = d; best = hd }
    }
    const tol = (canvas.width * 0.04) ** 2
    if (best && bestD <= tol) {
      dragging = best.id
      e.preventDefault()
    }
  }

  function onDrag(e) {
    if (!dragging || corners.length !== 4) return
    const p = canvasPt(e)
    if (dragging[0] === 'c') {
      // Corners are confined to the workspace; the bar handles need no clamp,
      // fracAlong already pins them to the corner segments.
      corners[+dragging[1]] = clampToWorkspace(p, workspace())
      corners = corners
    } else {
      const [TL, TR, BR, BL] = corners
      if (dragging === 'btl') barFrac.tl = fracAlong(p, TL, TR)
      else if (dragging === 'btr') barFrac.tr = fracAlong(p, TL, TR)
      else if (dragging === 'bbl') barFrac.bl = fracAlong(p, BL, BR)
      else if (dragging === 'bbr') barFrac.br = fracAlong(p, BL, BR)
      barFrac = barFrac
    }
    drawFrame()
  }

  function endDrag() {
    dragging = null
  }

  // swapPlayers exchanges the two players — names and colours — leaving the
  // board alone. On the opening position this looks almost exactly like the
  // old vertical mirror, which is why the two were confusable; the difference
  // is that Player 1 stays the bottom player (ADR-0009).
  function swapPlayers() {
    players = [players[1], players[0]]
    const a = checkerA
    checkerA = checkerB
    checkerB = a
    swapPresses += 1
  }

  function save() {
    error = ''
    if (corners.length !== 4) {
      error = 'Place the 4 corner handles on the playing surface.'
      return
    }
    dispatch('save', {
      players,
      videoUrl,
      lens,
      priors: {
        clock,
        matchLength: Number(matchLength),
        orientation: orientationName(orientation),
        checkerA,
        checkerB,
        cubeInPlay,
        crawford,
        jacoby,
        beaver,
      },
      corners,
      barEdges: barEdges(),
      swapPlies: swapPresses % 2 === 1,
    })
  }
</script>

<div class="overlay">
  <div class="panel">
    <h2>Session setup</h2>
    <p class="sub">
      Declared once per video — everything here seeds the manifest and the future
      automatic cues. Editable any time via “Calibration…”.
    </p>

    <div class="grid">
      <label>Player 1 — bottom of the video ▼ &amp; checker colour
        <span class="name-color">
          <input bind:value={players[0]} />
          <input type="color" bind:value={checkerA} title="Player 1 checker colour" />
        </span>
      </label>
      <label>Player 2 — top of the video ▲ &amp; checker colour
        <span class="name-color">
          <input bind:value={players[1]} />
          <input type="color" bind:value={checkerB} title="Player 2 checker colour" />
        </span>
      </label>
      <label>Match length <input type="number" min="1" bind:value={matchLength} /></label>
      <label class="row"><input type="checkbox" bind:checked={clock} /> Chess clock visible</label>
      <label class="row"><input type="checkbox" bind:checked={cubeInPlay} /> Doubling cube in play</label>
      <label class="row"><input type="checkbox" bind:checked={crawford} disabled={!cubeInPlay} /> Crawford rule</label>
      <label class="row"><input type="checkbox" bind:checked={jacoby} disabled={!cubeInPlay} /> Jacoby rule</label>
      <label class="row"><input type="checkbox" bind:checked={beaver} disabled={!cubeInPlay} /> Beavers allowed</label>
      <label class="wide">Video URL (source, e.g. YouTube — keeps shared sessions portable)
        <input bind:value={videoUrl} placeholder="https://…" />
      </label>
    </div>

    <h3>Board calibration</h3>
    <div class="cal-guide">
      <svg viewBox="0 0 200 130" class="schematic" aria-hidden="true">
        <!-- wooden frame (do NOT click here) -->
        <rect x="1" y="1" width="198" height="128" rx="6" fill="#4a3728" stroke="#2c2016" />
        <!-- playing surface (click its 4 corners) -->
        <rect x="16" y="14" width="168" height="102" fill="#6b503b" stroke="#38bdf8" stroke-width="2" stroke-dasharray="5 3" />
        <!-- centre bar, located by its own draggable edge handles -->
        <rect x="94" y="14" width="12" height="102" fill="#2c2016" />
        <!-- a few triangles to read as a board -->
        {#each [0, 1, 2, 3, 4, 5] as i}
          <polygon points={`${18 + i * 12},14 ${30 + i * 12},14 ${24 + i * 12},58`} fill={i % 2 ? '#8a6b4f' : '#5c4433'} />
          <polygon points={`${112 + i * 12},14 ${124 + i * 12},14 ${118 + i * 12},58`} fill={i % 2 ? '#5c4433' : '#8a6b4f'} />
          <polygon points={`${18 + i * 12},116 ${30 + i * 12},116 ${24 + i * 12},72`} fill={i % 2 ? '#5c4433' : '#8a6b4f'} />
          <polygon points={`${112 + i * 12},116 ${124 + i * 12},116 ${118 + i * 12},72`} fill={i % 2 ? '#8a6b4f' : '#5c4433'} />
        {/each}
        <!-- corner handles (green) -->
        {#each [[16, 14, '1'], [184, 14, '2'], [184, 116, '3'], [16, 116, '4']] as [cx, cy, n]}
          <circle {cx} {cy} r="7" fill="#22c55e" />
          <text x={cx} y={cy + 4} text-anchor="middle" font-size="10" fill="#fff">{n}</text>
        {/each}
        <!-- bar-edge handles (orange) -->
        {#each [[94, 14], [106, 14], [106, 116], [94, 116]] as [cx, cy]}
          <circle {cx} {cy} r="6" fill="#f59e0b" stroke="#000" stroke-width="1" />
        {/each}
      </svg>
      <p class="guide-text">
        Drag the <strong>4 green corner handles</strong> onto the corners of the playing surface —
        the outer tips of the corner triangles, <strong>not</strong> the wooden frame. Then drag the
        <strong>4 orange bar handles</strong> onto the two edges of the centre bar (they slide along
        the top/bottom edges). A blue grid follows live — adjust until its 24 cells sit on the real
        triangles. The dark band around the video is working room: a corner may sit just outside
        the frame when the capture crops the board, and stays draggable there.
      </p>
    </div>
    <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
    <canvas bind:this={canvas} class="cal" on:mousedown={startDrag} role="img" aria-label="video frame for board calibration handles"></canvas>
    <div class="cal-actions">
      <button type="button" class="detect" on:click={detectCorners} disabled={detecting}>
        {detecting ? 'Detecting…' : 'Detect corners'}
      </button>
      <button
        type="button"
        class="detect"
        on:click={() => { seedCalibration(false); detectMsg = 'Handles reset to the default inset.'; drawFrame() }}
      >Reset handles</button>
      <span class="hint">
        {#if detectMsg}{detectMsg}{:else}Drag the green corners and the orange bar handles until the grid matches the board.{/if}
      </span>
    </div>
    {#if lensActive(lens)}
      <div class="cal-actions lens-info">
        <span class="hint">
          Estimated lens distortion: k1={(lens.k1 ?? 0).toFixed(3)}{(lens.k2 ?? 0) !== 0 ? `, k2=${lens.k2.toFixed(3)}` : ''}
          — the grid curves accordingly. Dragging handles never changes it.
        </span>
        <button type="button" on:click={() => { lens = null; drawFrame() }}>Reset to 0</button>
      </div>
    {/if}

    <h3>Orientation</h3>
    <div class="orient">
      <div class="orient-board">
        <Board board={startBoard} {orientation} checkerColors={[checkerA, checkerB]} playerLabels={players} />
      </div>
      <div class="orient-controls">
        <p>
          Match this board to the video frame above — the <strong>same colours in the same
          corners</strong>. Mirror it if the home boards are on the other side; swap the players if
          the one sitting at the <strong>bottom</strong> of the video is the one you entered second.
        </p>
        <div class="orient-buttons">
          <button type="button" on:click={() => (orientation = flipHorizontal(orientation))}>⇄ Mirror left / right</button>
          <button type="button" on:click={swapPlayers}>⇅ Swap the two players</button>
        </div>
        <p class="hint">
          {orientationName(orientation)} — <strong>Player 1 is always the player at the bottom</strong>,
          so the two back checkers on the top row are Player 1's.
        </p>
      </div>
    </div>

    {#if error}<p class="error">{error}</p>{/if}

    <div class="actions">
      {#if initial?.corners?.length === 4}
        <button on:click={() => dispatch('cancel')}>Cancel</button>
      {/if}
      <button class="primary" on:click={save}>Save setup</button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: #000000cc;
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10;
  }
  .panel {
    background: #1f1f23;
    border: 1px solid #3f3f46;
    border-radius: 8px;
    padding: 1.25rem;
    width: min(720px, 92vw);
    max-height: 92vh;
    overflow-y: auto;
  }
  h2 { margin: 0 0 0.25rem; }
  .sub { color: #9ca3af; font-size: 0.85rem; margin-top: 0; }
  .grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.6rem 1rem;
    margin: 0.75rem 0;
  }
  .wide { grid-column: 1 / -1; }
  label { display: flex; flex-direction: column; gap: 0.2rem; font-size: 0.85rem; color: #d4d4d8; }
  label.row { flex-direction: row; align-items: center; gap: 0.5rem; }
  input, select {
    background: #27272a;
    border: 1px solid #52525b;
    color: #e4e4e7;
    border-radius: 4px;
    padding: 0.3rem 0.45rem;
  }
  input[type='color'] { padding: 0; width: 2.2rem; height: 1.6rem; flex: none; }
  .name-color { display: flex; gap: 0.4rem; align-items: center; }
  .name-color input:not([type='color']) { flex: 1; min-width: 0; }
  .cal-guide {
    display: flex;
    gap: 0.75rem;
    align-items: center;
    background: #202024;
    border: 1px solid #3f3f46;
    border-radius: 6px;
    padding: 0.6rem 0.75rem;
    margin: 0.35rem 0 0.6rem;
  }
  .schematic { width: 160px; flex: none; border-radius: 4px; }
  .guide-text { margin: 0; font-size: 0.82rem; color: #d4d4d8; line-height: 1.35; }
  .guide-text strong { color: #38bdf8; font-weight: 600; }
  .cal { width: 100%; background: #000; border-radius: 4px; cursor: grab; }
  .cal:active { cursor: grabbing; }
  .cal-actions { display: flex; align-items: center; gap: 0.6rem; margin-top: 0.35rem; }
  .detect { flex: none; padding: 0.3rem 0.8rem; cursor: pointer; border-radius: 4px; border: 1px solid #52525b; background: #27272a; color: #e4e4e7; }
  .detect:disabled { opacity: 0.6; cursor: default; }
  .hint { color: #9ca3af; font-size: 0.8rem; }
  .orient {
    display: flex;
    gap: 0.9rem;
    align-items: center;
    background: #202024;
    border: 1px solid #3f3f46;
    border-radius: 6px;
    padding: 0.6rem 0.75rem;
    margin: 0.35rem 0 0.6rem;
  }
  .orient-board { width: 300px; flex: none; }
  .orient-controls { font-size: 0.82rem; color: #d4d4d8; line-height: 1.35; }
  .orient-controls strong { color: #38bdf8; font-weight: 600; }
  .orient-controls p { margin: 0 0 0.5rem; }
  .orient-buttons { display: flex; gap: 0.5rem; flex-wrap: wrap; }
  .error { color: #f87171; }
  .actions { display: flex; justify-content: flex-end; gap: 0.5rem; }
  button { cursor: pointer; padding: 0.4rem 1rem; border-radius: 4px; border: 1px solid #52525b; background: #27272a; color: #e4e4e7; }
  button.primary { background: #4338ca; border-color: #4338ca; color: #fff; }
</style>
