<script>
  // Custom control bar replacing the webview's native <video> controls, which
  // vary per platform and offer neither ±5s nor a speed slider (ADR-0004).
  // Bound to the <video> element's media properties in App.svelte.
  import { snapSpeed, skip, formatTime, MIN_SPEED, MAX_SPEED, SPEED_STEP } from './lib/video.js'

  export let currentTime = 0
  export let duration = 0
  export let paused = true
  export let playbackRate = 1

  const togglePlay = () => (paused = !paused)
  const back5 = () => (currentTime = skip(currentTime, -5, duration))
  const fwd5 = () => (currentTime = skip(currentTime, 5, duration))
  const resetSpeed = () => (playbackRate = 1)
  // Guard the snap: keyboard/programmatic changes may land off-grid.
  $: playbackRate = snapSpeed(playbackRate)
</script>

<div class="controls">
  <button class="ctl" on:click={togglePlay} title={paused ? 'Play' : 'Pause'}>
    {paused ? '▶' : '⏸'}
  </button>
  <button class="ctl" on:click={back5} title="Reculer 5s">⏪5s</button>
  <button class="ctl" on:click={fwd5} title="Avancer 5s">5s⏩</button>

  <input
    class="seek"
    type="range"
    min="0"
    max={duration || 0}
    step="0.1"
    bind:value={currentTime}
    aria-label="Position dans la vidéo"
  />
  <span class="time">{formatTime(currentTime)} / {formatTime(duration)}</span>

  <span class="speed-wrap" title="Vitesse (double-clic = 1×)">
    <input
      class="speed"
      type="range"
      min={MIN_SPEED}
      max={MAX_SPEED}
      step={SPEED_STEP}
      bind:value={playbackRate}
      on:dblclick={resetSpeed}
      aria-label="Vitesse de lecture"
    />
    <button class="rate" on:click={resetSpeed}>{playbackRate.toFixed(2)}×</button>
  </span>
</div>

<style>
  .controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.4rem 0.6rem;
    background: #141417;
    border-top: 1px solid #333;
    box-sizing: border-box;
  }
  .ctl {
    background: #27272a;
    border: 1px solid #3f3f46;
    color: #e4e4e7;
    border-radius: 4px;
    padding: 0.25rem 0.5rem;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .ctl:hover { background: #3f3f46; }
  .seek { flex: 1; min-width: 4rem; cursor: pointer; }
  .time {
    color: #9ca3af;
    font-variant-numeric: tabular-nums;
    font-size: 0.8rem;
    white-space: nowrap;
  }
  .speed-wrap { display: flex; align-items: center; gap: 0.3rem; }
  .speed { width: 6rem; cursor: pointer; }
  .rate {
    background: none;
    border: none;
    color: #a5b4fc;
    font-variant-numeric: tabular-nums;
    font-size: 0.8rem;
    cursor: pointer;
    min-width: 2.8rem;
  }
</style>
