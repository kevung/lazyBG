<script>
  // Session Priors + Board Calibration form (ux-spec §10): a standard form —
  // this runs once per Part (and again only for corrections), so no keyboard
  // optimization. The 4 corners are clicked on the current video frame.
  import { createEventDispatcher, onMount } from 'svelte'

  export let videoEl = null
  export let initial = null // session.Setup for pre-filled correction

  const dispatch = createEventDispatcher()

  let players = ['Player 1', 'Player 2']
  let matchLength = 7
  let clock = true
  let orientation = 'p1-right'
  let checkerA = '#e7e0d5'
  let checkerB = '#31221c'
  let corners = [] // [[x,y] in video pixel coords]
  let canvas
  let error = ''

  onMount(() => {
    if (initial) {
      if (initial.players?.[0]) players = [...initial.players]
      if (initial.priors) {
        matchLength = initial.priors.matchLength || 7
        clock = !!initial.priors.clock
        orientation = initial.priors.orientation || 'p1-right'
        checkerA = initial.priors.checkerA || checkerA
        checkerB = initial.priors.checkerB || checkerB
      }
      corners = (initial.corners ?? []).map((c) => [...c])
    }
    drawFrame()
  })

  function drawFrame() {
    if (!canvas || !videoEl || !videoEl.videoWidth) return
    canvas.width = videoEl.videoWidth
    canvas.height = videoEl.videoHeight
    const ctx = canvas.getContext('2d')
    ctx.drawImage(videoEl, 0, 0)
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
      priors: {
        clock,
        matchLength: Number(matchLength),
        orientation,
        checkerA,
        checkerB,
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
      <label>
        Orientation
        <select bind:value={orientation}>
          <option value="p1-right">Player 1 bears off to the right</option>
          <option value="p1-left">Player 1 bears off to the left</option>
        </select>
      </label>
      <label class="row">
        Checker colors
        <input type="color" bind:value={checkerA} title="Player 1" />
        <input type="color" bind:value={checkerB} title="Player 2" />
      </label>
    </div>

    <h3>Board calibration — click the 4 corners (TL, TR, BR, BL)</h3>
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
    <canvas bind:this={canvas} class="cal" on:click={clickCorner} role="img" aria-label="video frame for corner calibration"></canvas>
    <p class="hint">{corners.length}/4 corners. Clicking after the 4th starts over.</p>

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
  .cal { width: 100%; background: #000; border-radius: 4px; cursor: crosshair; }
  .hint { color: #9ca3af; font-size: 0.8rem; }
  .error { color: #f87171; }
  .actions { display: flex; justify-content: flex-end; gap: 0.5rem; }
  button { cursor: pointer; padding: 0.4rem 1rem; border-radius: 4px; border: 1px solid #52525b; background: #27272a; color: #e4e4e7; }
  button.primary { background: #4338ca; border-color: #4338ca; color: #fff; }
</style>
