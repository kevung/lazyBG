<script>
  // Walking skeleton (issue #12): video + dice entry + ranked candidates +
  // keyboard confirm + minimal move list. The four-zone layout is issue #16.
  //
  // Bindings come from the Wails runtime: window.go.main.App.*
  // (no generated wailsjs imports — keeps the frontend buildable standalone).
  const api = () => window.go?.main?.App

  let videoUrl = ''
  let videoEl
  let moves = []
  let candidates = []
  let highlight = 0
  let onRoll = 0
  let firstDigit = null // first die typed, waiting for the second
  let error = ''

  const playerName = (p) => (p === 0 ? 'Player 1' : 'Player 2')

  let resumeTickMs = 0
  let warning = ''

  async function openVideo() {
    error = ''
    warning = ''
    try {
      const res = await api().OpenVideoDialog()
      if (res) {
        // Cache-bust so a newly picked file replaces the old <video> source.
        videoUrl = res.videoUrl + '?t=' + Date.now()
        moves = res.moves ?? []
        candidates = []
        firstDigit = null
        onRoll = res.onRoll
        warning = res.warning ?? ''
        resumeTickMs = res.lastTickMs ?? 0
      }
    } catch (e) {
      error = String(e)
    }
  }

  // Resume exactly where the user left off (session-format-spec §1).
  function onVideoReady() {
    if (videoEl && resumeTickMs > 0) {
      videoEl.currentTime = resumeTickMs / 1000
      resumeTickMs = 0
    }
  }

  // Persist the last-worked position whenever playback pauses.
  function onVideoPause() {
    if (videoEl) api().SetVideoPos(Math.round(videoEl.currentTime * 1000))
  }

  async function enterDice(d1, d2) {
    error = ''
    try {
      candidates = await api().EnterDice(d1, d2)
      highlight = 0
    } catch (e) {
      error = String(e)
      candidates = []
    }
  }

  async function confirmHighlight() {
    if (!candidates.length) return
    error = ''
    try {
      const tickMs = videoEl ? Math.round(videoEl.currentTime * 1000) : 0
      const ply = await api().Confirm(highlight, tickMs)
      moves = [...moves, ply]
      candidates = []
      firstDigit = null
      onRoll = await api().OnRoll()
    } catch (e) {
      error = String(e)
    }
  }

  async function togglePlayer() {
    error = ''
    try {
      await api().SetTurnPlayer(onRoll === 0 ? 1 : 0)
      onRoll = await api().OnRoll()
      if (candidates.length) candidates = await api().Candidates?.() ?? candidates
    } catch (e) {
      error = String(e)
    }
  }

  function onKeydown(e) {
    // Don't steal keys from form fields (none yet, but harmless).
    if (e.target && /INPUT|TEXTAREA|SELECT/.test(e.target.tagName)) return

    if (e.key >= '1' && e.key <= '6') {
      const d = Number(e.key)
      if (firstDigit === null) {
        firstDigit = d
      } else {
        const d1 = firstDigit
        firstDigit = null
        enterDice(d1, d)
      }
      e.preventDefault()
      return
    }
    switch (e.key) {
      case 'ArrowDown':
      case 'j':
        if (candidates.length) highlight = Math.min(highlight + 1, candidates.length - 1)
        e.preventDefault()
        break
      case 'ArrowUp':
      case 'k':
        if (candidates.length) highlight = Math.max(highlight - 1, 0)
        e.preventDefault()
        break
      case ' ':
      case 'Enter':
        confirmHighlight()
        e.preventDefault()
        break
      case 'Escape':
        firstDigit = null
        candidates = []
        e.preventDefault()
        break
      case 'p':
        togglePlayer()
        e.preventDefault()
        break
    }
  }
</script>

<svelte:window on:keydown={onKeydown} />

<main>
  <section class="video-zone">
    {#if videoUrl}
      <!-- svelte-ignore a11y-media-has-caption -->
      <video
        bind:this={videoEl}
        src={videoUrl}
        controls
        on:loadedmetadata={onVideoReady}
        on:pause={onVideoPause}
      ></video>
    {:else}
      <button class="open" on:click={openVideo}>Open match video…</button>
    {/if}
  </section>

  <aside class="entry-zone">
    {#if warning}
      <p class="warning">{warning}</p>
    {/if}
    <div class="turn">
      On roll: <strong>{playerName(onRoll)}</strong>
      <span class="hint">(p to switch)</span>
    </div>

    <div class="dice">
      {#if firstDigit !== null}
        Dice: <strong>{firstDigit}–?</strong>
      {:else if candidates.length}
        Pick a move (↑/↓, Space confirms)
      {:else}
        Type the two dice digits (1–6)
      {/if}
    </div>

    {#if candidates.length}
      <ol class="candidates">
        {#each candidates as c, i}
          <li class:selected={i === highlight}>
            <span class="notation">{c.notation}</span>
            <span class="equity">{c.equity.toFixed(3)}</span>
          </li>
        {/each}
      </ol>
    {/if}

    {#if error}
      <p class="error">{error}</p>
    {/if}

    <h3>Moves</h3>
    <ol class="moves">
      {#each moves as m}
        <li>
          <span class="who">{playerName(m.player)}</span>
          <span class="dice-lbl">{m.dice}:</span>
          <span class="notation">{m.notation}</span>
          <span class="tick">{(m.tickMs / 1000).toFixed(1)}s</span>
        </li>
      {/each}
    </ol>
  </aside>
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: system-ui, sans-serif;
    background: #1b1b1f;
    color: #e4e4e7;
  }
  main {
    display: grid;
    grid-template-columns: 1fr 380px;
    height: 100vh;
  }
  .video-zone {
    display: flex;
    align-items: center;
    justify-content: center;
    background: #000;
  }
  video {
    max-width: 100%;
    max-height: 100vh;
  }
  .open {
    font-size: 1.2rem;
    padding: 1rem 2rem;
    cursor: pointer;
  }
  .entry-zone {
    padding: 1rem;
    overflow-y: auto;
    border-left: 1px solid #333;
  }
  .turn { margin-bottom: 0.5rem; }
  .hint { color: #888; font-size: 0.85em; }
  .dice { margin-bottom: 0.75rem; color: #a5b4fc; }
  .candidates {
    list-style: none;
    padding: 0;
    margin: 0 0 1rem;
  }
  .candidates li {
    display: flex;
    justify-content: space-between;
    padding: 0.3rem 0.5rem;
    border-radius: 4px;
  }
  .candidates li.selected {
    background: #3730a3;
    color: #fff;
  }
  .equity { color: #9ca3af; font-variant-numeric: tabular-nums; }
  .candidates li.selected .equity { color: #c7d2fe; }
  .moves {
    list-style: none;
    padding: 0;
    margin: 0;
    font-size: 0.9rem;
  }
  .moves li {
    display: flex;
    gap: 0.5rem;
    padding: 0.15rem 0;
  }
  .who { color: #888; width: 4.5rem; }
  .dice-lbl { color: #a5b4fc; }
  .tick { margin-left: auto; color: #666; }
  .error { color: #f87171; }
  .warning {
    color: #fbbf24;
    background: #78350f33;
    padding: 0.5rem;
    border-radius: 4px;
    font-size: 0.85rem;
  }
</style>
