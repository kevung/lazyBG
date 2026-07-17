<script>
  // Walking skeleton (issue #12): video + dice entry + ranked candidates +
  // keyboard confirm + minimal move list. The four-zone layout is issue #16.
  //
  // Bindings come from the Wails runtime: window.go.main.App.*
  // (no generated wailsjs imports — keeps the frontend buildable standalone).
  import Board from './Board.svelte'
  import SetupPanel from './SetupPanel.svelte'

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
        refreshReview()
        backToLive()
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
    maybeOpenSetup() // blocking first step for a fresh session
  }

  // Persist the last-worked position whenever playback pauses.
  function onVideoPause() {
    if (videoEl) api().SetVideoPos(Math.round(videoEl.currentTime * 1000))
    snapshotFrame()
  }

  let reviewCount = 0
  let overrideOpen = false
  let overrideText = ''

  const nowTickMs = () => (videoEl ? Math.round(videoEl.currentTime * 1000) : 0)

  let reviewItems = []

  async function refreshReview() {
    try {
      reviewItems = (await api().ReviewItems()) ?? []
      reviewCount = reviewItems.length
    } catch { /* non-fatal */ }
  }

  // Selecting a review item = selecting its turn (one resolution mechanism,
  // ux-spec §7): jump to the tick, re-open the entry flow.
  function selectReviewItem(it) {
    selectTurn(it.turnSeq)
  }

  async function markReviewed(it) {
    error = ''
    try {
      await api().MarkReviewed(it.turnSeq)
      refreshReview()
    } catch (e) {
      error = String(e)
    }
  }

  async function enterDice(d1, d2) {
    error = ''
    try {
      const res = await api().EnterDice(d1, d2, nowTickMs())
      if (res.danced) {
        // Automatic dance: recorded with no candidate step.
        moves = [...moves, res.ply]
        candidates = []
        onRoll = await api().OnRoll()
        checkGameEnd()
        refreshBoard()
      } else {
        candidates = res.candidates ?? []
        highlight = 0
      }
    } catch (e) {
      error = String(e)
      candidates = []
    }
  }

  async function confirmHighlight(flagUncertain = false) {
    if (!candidates.length) return
    error = ''
    try {
      const tickMs = nowTickMs()
      const ply = flagUncertain
        ? await api().ConfirmFlag(highlight, tickMs)
        : await api().Confirm(highlight, tickMs)
      moves = [...moves, ply]
      candidates = []
      firstDigit = null
      onRoll = await api().OnRoll()
      if (flagUncertain) refreshReview()
      checkGameEnd()
      selectedSeq = -1
      refreshBoard()
    } catch (e) {
      error = String(e)
    }
  }

  async function submitOverride() {
    error = ''
    try {
      const ply = await api().Override(overrideText.trim(), nowTickMs())
      moves = [...moves, ply]
      candidates = []
      firstDigit = null
      overrideOpen = false
      overrideText = ''
      onRoll = await api().OnRoll()
      checkGameEnd()
      selectedSeq = -1
      refreshBoard()
    } catch (e) {
      error = String(e)
    }
  }

  let cubeOptions = []
  let cubeHighlight = 0
  let setupOpen = false
  let setupInitial = null

  async function maybeOpenSetup() {
    try {
      const done = await api().SetupDone()
      if (!done) {
        setupInitial = await api().GetSetup()
        setupOpen = true
      }
    } catch { /* non-fatal */ }
  }

  let exportMsg = ''

  async function exportProjections() {
    error = ''
    exportMsg = ''
    try {
      const paths = await api().ExportDialog()
      if (paths && paths[0]) exportMsg = `Saved ${paths[0]} + manifest`
    } catch (e) {
      error = String(e)
    }
  }

  async function openCalibration() {
    setupInitial = await api().GetSetup()
    setupOpen = true
  }

  async function onSetupSave(e) {
    error = ''
    try {
      await api().SaveSetup(e.detail)
      setupOpen = false
    } catch (err) {
      error = String(err)
    }
  }
  let boardState = null // reconstructed board shown in the board panel
  let selectedSeq = -1 // -1 = live (following the latest turn)
  let frameCanvas // raw-frame snapshot beside the reconstructed board

  async function refreshBoard() {
    try {
      boardState =
        selectedSeq >= 0 ? await api().BoardAt(selectedSeq) : await api().BoardState()
    } catch { /* non-fatal */ }
  }

  // Copy the current video frame into the snapshot canvas (ux-spec §6:
  // reconstructed board and raw frame side by side).
  function snapshotFrame() {
    if (!videoEl || !frameCanvas || !videoEl.videoWidth) return
    const ctx = frameCanvas.getContext('2d')
    frameCanvas.width = videoEl.videoWidth
    frameCanvas.height = videoEl.videoHeight
    ctx.drawImage(videoEl, 0, 0)
  }

  // Select a past turn: video jumps to its tick, the board panel shows the
  // reconstructed position after it, and typing dice re-opens the same
  // entry flow at that turn (edit mode, ux-spec §4).
  function selectTurn(seq) {
    selectedSeq = seq
    candidates = []
    firstDigit = null
    const m = moves[seq]
    if (m && videoEl) videoEl.currentTime = m.tickMs / 1000
    refreshBoard()
  }

  async function editDice(d1, d2) {
    error = ''
    try {
      candidates = await api().CandidatesFor(selectedSeq, d1, d2)
      highlight = 0
      editRoll = [d1, d2]
    } catch (e) {
      error = String(e)
      candidates = []
    }
  }

  let editRoll = null

  async function confirmEdit(notation) {
    error = ''
    try {
      await api().ReplaceTurn(selectedSeq, editRoll[0], editRoll[1], notation)
      moves = await api().Moves()
      candidates = []
      firstDigit = null
      editRoll = null
      refreshReview() // a cascade may have flagged downstream turns
      refreshBoard()
      onRoll = await api().OnRoll()
    } catch (e) {
      error = String(e)
    }
  }

  async function deleteSelected() {
    if (selectedSeq < 0) return
    error = ''
    try {
      await api().DeleteTurn(selectedSeq)
      moves = await api().Moves()
      refreshReview()
      backToLive()
      onRoll = await api().OnRoll()
    } catch (e) {
      error = String(e)
    }
  }

  function backToLive() {
    selectedSeq = -1
    refreshBoard()
  }

  // Tab / Shift+Tab: free movement across the recorded turn ticks without
  // committing anything (ux-spec §3). Segmentation-fed candidate ticks plug
  // in once calibration exists (issue #14); past the last recorded tick we
  // step by a fixed 5s.
  function navTick(dir) {
    if (!videoEl) return
    const now = Math.round(videoEl.currentTime * 1000)
    const ticks = [...new Set(moves.map((m) => m.tickMs).filter((t) => t > 0))].sort((a, b) => a - b)
    let target
    if (dir > 0) {
      target = ticks.find((t) => t > now + 50)
      if (target === undefined) target = now + 5000
    } else {
      const prev = ticks.filter((t) => t < now - 50)
      target = prev.length ? prev[prev.length - 1] : Math.max(0, now - 5000)
    }
    videoEl.currentTime = target / 1000
  }
  let gameEnd = null // pending GameEndProposal
  let endWinner = 0
  let endPoints = 1
  let matchOver = false
  let score = [0, 0]

  async function checkGameEnd() {
    try {
      gameEnd = await api().PendingGameEnd()
      if (gameEnd) {
        endWinner = gameEnd.winner
        endPoints = gameEnd.points
      }
    } catch { /* non-fatal */ }
  }

  async function confirmGameEnd() {
    error = ''
    try {
      const res = await api().ConfirmGameEnd(Number(endWinner), Number(endPoints))
      gameEnd = null
      score = res.score
      matchOver = res.matchOver
      moves = await api().Moves()
      onRoll = await api().OnRoll()
      selectedSeq = -1
      refreshBoard()
    } catch (e) {
      error = String(e)
    }
  }

  async function openCubeMenu() {
    error = ''
    try {
      cubeOptions = (await api().CubeActions()) ?? []
      cubeHighlight = 0
      if (!cubeOptions.length) error = 'No cube action available (cube owned by the opponent)'
    } catch (e) {
      error = String(e)
    }
  }

  async function confirmCube() {
    if (!cubeOptions.length) return
    error = ''
    try {
      const ply = await api().EnterCube(cubeOptions[cubeHighlight], nowTickMs())
      moves = [...moves, ply]
      cubeOptions = []
      candidates = []
      firstDigit = null
      onRoll = await api().OnRoll()
      checkGameEnd()
      selectedSeq = -1
      refreshBoard()
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
    if (setupOpen) return // the setup form owns the keyboard
    // Don't steal keys from form fields.
    if (e.target && /INPUT|TEXTAREA|SELECT/.test(e.target.tagName)) return

    if (e.key === 'Tab') {
      navTick(e.shiftKey ? -1 : 1)
      e.preventDefault()
      return
    }
    if (e.key >= '1' && e.key <= '6') {
      const d = Number(e.key)
      if (firstDigit === null) {
        firstDigit = d
      } else {
        const d1 = firstDigit
        firstDigit = null
        if (selectedSeq >= 0) {
          editDice(d1, d) // edit mode: same flow, at the selected turn
        } else {
          enterDice(d1, d)
        }
      }
      e.preventDefault()
      return
    }
    if ((e.key === 'Delete' || e.key === 'x') && selectedSeq >= 0 && !candidates.length) {
      deleteSelected()
      e.preventDefault()
      return
    }
    if (cubeOptions.length) {
      switch (e.key) {
        case 'ArrowDown':
        case 'j':
          cubeHighlight = Math.min(cubeHighlight + 1, cubeOptions.length - 1)
          break
        case 'ArrowUp':
        case 'k':
          cubeHighlight = Math.max(cubeHighlight - 1, 0)
          break
        case ' ':
        case 'Enter':
          confirmCube()
          break
        case 'Escape':
          cubeOptions = []
          break
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
        if (selectedSeq >= 0 && editRoll && candidates.length) {
          confirmEdit(candidates[highlight].notation)
        } else {
          // Shift+Space = confirm AND flag uncertain (ux-spec §2).
          confirmHighlight(e.shiftKey)
        }
        e.preventDefault()
        break
      case 'Escape':
        firstDigit = null
        candidates = []
        overrideOpen = false
        if (selectedSeq >= 0) backToLive()
        e.preventDefault()
        break
      case 'p':
        togglePlayer()
        e.preventDefault()
        break
      case 'c':
      case 'C':
        // Cube menu: a separate entry point since a cube decision precedes
        // the roll (ux-spec §9).
        if (firstDigit === null && !candidates.length) openCubeMenu()
        e.preventDefault()
        break
      case 'o':
      case 'O':
        // The override escape hatch — one key away from the candidate list,
        // never the default (ADR-0001). Needs the dice entered first.
        if (candidates.length) overrideOpen = true
        e.preventDefault()
        break
    }
  }
</script>

<svelte:window on:keydown={onKeydown} />

<main>
  <div class="left-col">
    <section class="video-zone">
      {#if videoUrl}
        <!-- svelte-ignore a11y-media-has-caption -->
        <video
          bind:this={videoEl}
          src={videoUrl}
          controls
          on:loadedmetadata={onVideoReady}
          on:pause={onVideoPause}
          on:seeked={snapshotFrame}
        ></video>
      {:else}
        <button class="open" on:click={openVideo}>Open match video…</button>
      {/if}
    </section>

    <section class="board-zone">
      <div class="board-half">
        <h4>
          Reconstructed
          {#if selectedSeq >= 0}
            (after turn {selectedSeq + 1})
            <button class="linklike" on:click={backToLive}>back to live</button>
          {/if}
        </h4>
        <Board board={boardState} />
      </div>
      <div class="board-half">
        <h4>Video frame</h4>
        <canvas bind:this={frameCanvas} class="frame"></canvas>
      </div>
    </section>
  </div>

  <aside class="entry-zone">
    {#if warning}
      <p class="warning">{warning}</p>
    {/if}
    {#if exportMsg}
      <p class="exportmsg">{exportMsg}</p>
    {/if}
    <div class="turn">
      <button class="linklike" on:click={openCalibration} title="Session Priors + Board Calibration">Calibration…</button>
      <button class="linklike" on:click={exportProjections} title="Write .mat + corpus manifest from the current state">Export…</button>
      <span class="score">{score[0]}–{score[1]}</span>
      On roll: <strong>{playerName(onRoll)}</strong>
      <span class="hint">(p to switch)</span>
      {#if reviewCount > 0}
        <span class="badge" title="turns to review">{reviewCount} to review</span>
      {/if}
    </div>

    {#if overrideOpen}
      <div class="override">
        <label for="override-input">Override — record what you saw (blank = Cannot Move):</label>
        <!-- svelte-ignore a11y-autofocus -->
        <input
          id="override-input"
          autofocus
          bind:value={overrideText}
          placeholder="e.g. 24/20 13/10"
          on:keydown={(e) => {
            if (e.key === 'Enter') submitOverride()
            if (e.key === 'Escape') { overrideOpen = false; overrideText = '' }
            e.stopPropagation()
          }}
        />
      </div>
    {/if}

    {#if matchOver}
      <p class="matchover">Match over — {score[0]}:{score[1]}. Use Export… to write the .mat + manifest.</p>
    {/if}

    {#if gameEnd}
      <div class="gameend">
        <strong>Game over</strong> ({gameEnd.reason}{gameEnd.backgammon ? ', backgammon' : gameEnd.gammon ? ', gammon' : ''})
        <label>
          Winner:
          <select bind:value={endWinner}>
            <option value={0}>{playerName(0)}</option>
            <option value={1}>{playerName(1)}</option>
          </select>
        </label>
        <label>
          Points:
          <input type="number" min="1" bind:value={endPoints} />
        </label>
        <button on:click={confirmGameEnd}>Confirm result</button>
      </div>
    {/if}

    {#if cubeOptions.length}
      <ol class="candidates cube-menu">
        {#each cubeOptions as opt, i}
          <li class:selected={i === cubeHighlight}>
            <span class="notation">{opt}</span>
          </li>
        {/each}
      </ol>
    {/if}

    {#if selectedSeq >= 0}
      <p class="editing">
        Editing turn {selectedSeq + 1} — type new dice, pick, Space applies.
        Delete/x removes the turn. Esc returns to live.
      </p>
    {/if}

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
      {#each moves as m, seq}
        <li class:selected={seq === selectedSeq}>
          <button class="rowbtn" on:click={() => selectTurn(seq)}>
            <span class="who">{playerName(m.player)}</span>
            <span class="dice-lbl">{m.cube ? '' : m.dice + ':'}</span>
            <span class="notation">
              {#if m.cube === 'double'}Doubles to {m.cubeValue}{:else if m.cube}{m.cube[0].toUpperCase() + m.cube.slice(1)}{:else if m.cannotMove}Cannot Move{:else}{m.notation}{/if}
            </span>
            <span class="tick">{(m.tickMs / 1000).toFixed(1)}s</span>
          </button>
        </li>
      {/each}
    </ol>

    <section class="review-zone">
      <h3>Review queue {#if reviewCount}<span class="badge">{reviewCount}</span>{/if}</h3>
      {#if !reviewItems.length}
        <p class="hint">Nothing to review.</p>
      {:else}
        <ol class="moves review-list">
          {#each reviewItems as it}
            <li>
              <button class="rowbtn" on:click={() => selectReviewItem(it)}>
                <span class="who">turn {it.turnSeq + 1}</span>
                <span class="reason">{it.reason}</span>
                <span class="notation">{it.notation || (it.dice ? it.dice : '')}</span>
                <span class="tick">{(it.tickMs / 1000).toFixed(1)}s</span>
              </button>
              <button class="okbtn" title="looked again — it's right" on:click={() => markReviewed(it)}>✓</button>
            </li>
          {/each}
        </ol>
      {/if}
    </section>
  </aside>
</main>

{#if setupOpen}
  <SetupPanel {videoEl} initial={setupInitial} on:save={onSetupSave} on:cancel={() => (setupOpen = false)} />
{/if}

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
  .left-col {
    display: grid;
    grid-template-rows: 1fr auto;
    min-width: 0;
  }
  .board-zone {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem;
    padding: 0.5rem;
    background: #141417;
    border-top: 1px solid #333;
  }
  .board-half h4 {
    margin: 0 0 0.3rem;
    font-size: 0.8rem;
    color: #9ca3af;
    font-weight: 500;
  }
  .frame {
    width: 100%;
    background: #000;
    border-radius: 4px;
    min-height: 40px;
  }
  .linklike {
    background: none;
    border: none;
    color: #a5b4fc;
    cursor: pointer;
    font-size: 0.75rem;
    text-decoration: underline;
    padding: 0;
    margin-left: 0.4rem;
  }
  .rowbtn {
    all: unset;
    display: flex;
    gap: 0.5rem;
    width: 100%;
    cursor: pointer;
    padding: 0.15rem 0.25rem;
    border-radius: 3px;
  }
  .rowbtn:hover { background: #27272a; }
  .moves li.selected .rowbtn { background: #3730a3; }
  .review-zone {
    margin-top: 1rem;
    border-top: 1px solid #333;
    padding-top: 0.5rem;
  }
  .review-list li {
    display: flex;
    align-items: center;
    gap: 0.25rem;
  }
  .reason {
    color: #fbbf24;
    font-size: 0.75rem;
  }
  .okbtn {
    background: #14532d;
    border: 1px solid #16a34a55;
    color: #4ade80;
    border-radius: 4px;
    cursor: pointer;
    padding: 0 0.4rem;
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
    padding: 0;
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
  .badge {
    margin-left: 0.5rem;
    background: #b45309;
    color: #fff;
    border-radius: 999px;
    padding: 0.1rem 0.6rem;
    font-size: 0.75rem;
  }
  .override {
    margin: 0.5rem 0;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.85rem;
  }
  .gameend {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    background: #14532d33;
    border: 1px solid #16a34a55;
    border-radius: 6px;
    padding: 0.6rem;
    margin-bottom: 0.75rem;
    font-size: 0.9rem;
  }
  .gameend select, .gameend input {
    margin-left: 0.4rem;
    background: #27272a;
    border: 1px solid #52525b;
    color: #e4e4e7;
    border-radius: 4px;
    padding: 0.2rem 0.4rem;
    width: 8rem;
  }
  .gameend button {
    align-self: flex-start;
    cursor: pointer;
  }
  .exportmsg {
    color: #4ade80;
    font-size: 0.8rem;
  }
  .matchover {
    color: #4ade80;
    font-weight: 600;
  }
  .score {
    margin-right: 0.6rem;
    color: #a5b4fc;
    font-variant-numeric: tabular-nums;
  }
  .editing {
    color: #f0abfc;
    background: #701a7533;
    padding: 0.4rem 0.5rem;
    border-radius: 4px;
    font-size: 0.8rem;
  }
  .override input {
    padding: 0.35rem 0.5rem;
    background: #27272a;
    border: 1px solid #52525b;
    color: #e4e4e7;
    border-radius: 4px;
  }
</style>
