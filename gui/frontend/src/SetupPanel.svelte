<script>
  // Session Priors + Board Calibration form (ux-spec §10): a standard form —
  // this runs once per Part (and again only for corrections), so no keyboard
  // optimization. The 4 corners are clicked on the current video frame.
  import { createEventDispatcher, onMount } from 'svelte'
  import { gridOnFrame } from './lib/calibration.js'
  import {
    orientationName, parseOrientation, flipHorizontal, flipVertical,
  } from './lib/boardGeometry.js'
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
  let checkerA = '#e7e0d5'
  let checkerB = '#31221c'

  // The opening position, drawn in the orientation preview so the user can see
  // each player's home board and colours and flip until it matches the video.
  const startBoard = (() => {
    const Pts = Array.from({ length: 25 }, () => ({ N: 0, Owner: 0 }))
    const set = (p, n, o) => (Pts[p] = { N: n, Owner: o })
    set(24, 2, 0); set(13, 5, 0); set(8, 3, 0); set(6, 5, 0) // Player 1
    set(1, 2, 1); set(12, 5, 1); set(17, 3, 1); set(19, 5, 1) // Player 2
    return { Pts, Bar: [0, 0], Off: [0, 0] }
  })()
  let corners = [] // [[x,y] in video pixel coords]
  let videoUrl = ''
  let canvas
  let error = ''

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
      corners = (initial.corners ?? []).map((c) => [...c])
      videoUrl = initial.videoUrl || ''
    }
    drawFrame()
  })

  function drawFrame() {
    if (!canvas || !videoEl || !videoEl.videoWidth) return
    canvas.width = videoEl.videoWidth
    canvas.height = videoEl.videoHeight
    const ctx = canvas.getContext('2d')
    ctx.drawImage(videoEl, 0, 0)
    // Once all 4 corners are set, project the canonical grid back onto the
    // frame (#38): if the 24 cells / bar / tray don't sit on the real board,
    // the corners were clicked on the wooden frame — re-click.
    const grid = corners.length === 4 ? gridOnFrame(corners) : null
    if (grid) {
      ctx.lineWidth = Math.max(1, canvas.width / 600)
      ctx.strokeStyle = '#38bdf8cc'
      for (const line of grid) {
        ctx.beginPath()
        line.forEach(([x, y], i) => (i ? ctx.lineTo(x, y) : ctx.moveTo(x, y)))
        ctx.stroke()
      }
    }
    for (const [i, [x, y]] of corners.entries()) {
      ctx.fillStyle = '#22c55e'
      ctx.beginPath()
      ctx.arc(x, y, 8, 0, 2 * Math.PI)
      ctx.fill()
      ctx.fillStyle = '#fff'
      ctx.font = '16px sans-serif'
      ctx.fillText(String(i + 1), x + 10, y - 10)
    }
  }

  function clickCorner(e) {
    if (!canvas) return
    const rect = canvas.getBoundingClientRect()
    const x = ((e.clientX - rect.left) / rect.width) * canvas.width
    const y = ((e.clientY - rect.top) / rect.height) * canvas.height
    if (corners.length >= 4) corners = []
    corners = [...corners, [x, y]]
    drawFrame()
  }

  function save() {
    error = ''
    if (corners.length !== 4) {
      error = 'Click the 4 board corners (TL, TR, BR, BL) on the frame.'
      return
    }
    dispatch('save', {
      players,
      videoUrl,
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
      <label>Player 1 <input bind:value={players[0]} /></label>
      <label>Player 2 <input bind:value={players[1]} /></label>
      <label>Match length <input type="number" min="1" bind:value={matchLength} /></label>
      <label class="row"><input type="checkbox" bind:checked={clock} /> Chess clock visible</label>
      <label class="row"><input type="checkbox" bind:checked={cubeInPlay} /> Doubling cube in play</label>
      <label class="row"><input type="checkbox" bind:checked={crawford} disabled={!cubeInPlay} /> Crawford rule</label>
      <label class="row"><input type="checkbox" bind:checked={jacoby} disabled={!cubeInPlay} /> Jacoby rule</label>
      <label class="row"><input type="checkbox" bind:checked={beaver} disabled={!cubeInPlay} /> Beavers allowed</label>
      <label class="row">
        Checker colors
        <input type="color" bind:value={checkerA} title="Player 1" />
        <input type="color" bind:value={checkerB} title="Player 2" />
      </label>
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
        <!-- centre bar, included in the rectangle -->
        <rect x="94" y="14" width="12" height="102" fill="#2c2016" />
        <!-- a few triangles to read as a board -->
        {#each [0, 1, 2, 3, 4, 5] as i}
          <polygon points={`${18 + i * 12},14 ${30 + i * 12},14 ${24 + i * 12},58`} fill={i % 2 ? '#8a6b4f' : '#5c4433'} />
          <polygon points={`${112 + i * 12},14 ${124 + i * 12},14 ${118 + i * 12},58`} fill={i % 2 ? '#5c4433' : '#8a6b4f'} />
          <polygon points={`${18 + i * 12},116 ${30 + i * 12},116 ${24 + i * 12},72`} fill={i % 2 ? '#5c4433' : '#8a6b4f'} />
          <polygon points={`${112 + i * 12},116 ${124 + i * 12},116 ${118 + i * 12},72`} fill={i % 2 ? '#8a6b4f' : '#5c4433'} />
        {/each}
        <!-- corner markers, in click order -->
        {#each [[16, 14, '1'], [184, 14, '2'], [184, 116, '3'], [16, 116, '4']] as [cx, cy, n]}
          <circle {cx} {cy} r="7" fill="#22c55e" />
          <text x={cx} y={cy + 4} text-anchor="middle" font-size="10" fill="#fff">{n}</text>
        {/each}
      </svg>
      <p class="guide-text">
        Click the <strong>4 corners of the playing surface</strong> — the outer tips of the corner
        triangles, with the <strong>bar included</strong> in the middle (one rectangle).
        <strong>Not</strong> the wooden frame. Order: <strong>1</strong> top-left,
        <strong>2</strong> top-right, <strong>3</strong> bottom-right, <strong>4</strong> bottom-left.
        After the 4th click, a blue grid is drawn on the frame — if its cells don't sit on the real
        triangles, re-click.
      </p>
    </div>
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
    <canvas bind:this={canvas} class="cal" on:click={clickCorner} role="img" aria-label="video frame for corner calibration"></canvas>
    <p class="hint">{corners.length}/4 corners. Clicking after the 4th starts over.</p>

    <h3>Orientation</h3>
    <div class="orient">
      <div class="orient-board">
        <Board board={startBoard} {orientation} checkerColors={[checkerA, checkerB]} />
      </div>
      <div class="orient-controls">
        <p>
          Flip until this board matches the video frame above — the <strong>same colours in the
          same corners</strong>, each player's home board on the correct side. This sets the board
          orientation and confirms the checker-colour assignment in one step.
        </p>
        <div class="orient-buttons">
          <button type="button" on:click={() => (orientation = flipHorizontal(orientation))}>⇄ Mirror left / right</button>
          <button type="button" on:click={() => (orientation = flipVertical(orientation))}>⇅ Mirror top / bottom</button>
        </div>
        <p class="hint">{orientationName(orientation)}</p>
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
  input[type='color'] { padding: 0; width: 2.2rem; height: 1.6rem; }
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
  .cal { width: 100%; background: #000; border-radius: 4px; cursor: crosshair; }
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
