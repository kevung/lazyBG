<script>
  // Reconstructed-board renderer (ux-spec §6) — "what the app believes".
  // two.js drawing ported from blunderDB's board (ADR-0005): same visual
  // language (grey triangles, dark/white checkers, doubling cube), stripped of
  // blunderDB's stores/interactivity and driven purely by props. Read-only.
  import Two from 'two.js'
  import { onMount } from 'svelte'
  import { isTop, colOf, stack, NCOLS, BAR_COL, OFF_COL } from './lib/boardGeometry.js'

  let {
    board = null,
    cube = { value: 1, owner: 0, centered: true },
    score = [0, 0],
  } = $props()

  const CFG = {
    boardFill: '#1c130c',
    tray: '#0f0a06',
    triFill1: '#8a6b4f',
    triFill2: '#5c4433',
    triStroke: '#00000055',
    checker: ['#2a2320', '#e7e0d5'], // [owner0 dark, owner1 light]
    checkerStroke: ['#000000', '#7c7266'],
    checkerLabel: ['#e7e0d5', '#2a2320'],
    barLine: '#000000',
    text: '#cbb79a',
    cubeFill: '#efe9dd',
    cubeText: '#1c130c',
  }

  let container
  let two = null
  let ro = null

  const sizeOf = () => ({
    width: container?.clientWidth || 520,
    height: container?.clientHeight || 360,
  })

  onMount(() => {
    // Explicit width/height (blunderDB's proven pattern) rather than two.js
    // `fit`, plus a ResizeObserver to stay responsive to the panel size.
    const { width, height } = sizeOf()
    two = new Two({ type: Two.Types.svg, width, height }).appendTo(container)
    draw()
    ro = new ResizeObserver(() => {
      if (!two) return
      const s = sizeOf()
      two.width = s.width
      two.height = s.height
      if (two.renderer && two.renderer.setSize) two.renderer.setSize(s.width, s.height)
      draw()
    })
    ro.observe(container)
    return () => {
      if (ro) ro.disconnect()
      if (two) two.clear()
    }
  })

  // Redraw whenever the data or the canvas size changes.
  $effect(() => {
    board, cube, score
    draw()
  })

  function draw() {
    if (!two) return
    two.clear()
    const W = two.width || 520
    const H = two.height || 360
    // 14 columns wide (12 points + bar + off tray), 11 checker-heights tall.
    const u = Math.min(W / NCOLS, H / 11) * 0.96
    const boardW = u * NCOLS
    const boardH = u * 11
    const x0 = (W - boardW) / 2
    const y0 = (H - boardH) / 2
    const r = u * 0.46
    const colCenter = (col) => x0 + (col + 0.5) * u
    const top = y0
    const bottom = y0 + boardH

    const bg = two.makeRectangle(W / 2, H / 2, boardW, boardH)
    bg.fill = CFG.boardFill
    bg.noStroke()
    const tray = two.makeRectangle(colCenter(OFF_COL), H / 2, u, boardH)
    tray.fill = CFG.tray
    tray.noStroke()

    if (board) {
      drawTriangles(colCenter, top, bottom, u)
      drawCheckers(colCenter, top, bottom, r)
      drawBarAndOff(colCenter, top, bottom, u, r)
    }
    drawCube(colCenter, top, bottom, u)
    drawScore(x0, top, bottom, u)

    const bar = two.makeRectangle(colCenter(BAR_COL), H / 2, u * 0.12, boardH)
    bar.fill = CFG.barLine
    bar.noStroke()

    two.update()
  }

  function drawTriangles(colCenter, top, bottom, u) {
    for (let p = 1; p <= 24; p++) {
      const cx = colCenter(colOf(p))
      const half = u * 0.44
      const tri = isTop(p)
        ? two.makePath(cx - half, top, cx + half, top, cx, top + 5 * u)
        : two.makePath(cx - half, bottom, cx + half, bottom, cx, bottom - 5 * u)
      tri.fill = p % 2 === 0 ? CFG.triFill1 : CFG.triFill2
      tri.stroke = CFG.triStroke
      tri.linewidth = 1
    }
  }

  function drawCheckers(colCenter, top, bottom, r) {
    for (let p = 1; p <= 24; p++) {
      const pt = board.Pts?.[p]
      if (!pt || pt.N === 0) continue
      const cx = colCenter(colOf(p))
      const { drawn, overflow } = stack(pt.N)
      for (let i = 0; i < drawn; i++) {
        const cy = isTop(p) ? top + r + i * (2 * r) : bottom - r - i * (2 * r)
        placeChecker(cx, cy, r, pt.Owner, i === drawn - 1 && overflow ? overflow : 0)
      }
    }
  }

  function placeChecker(cx, cy, r, owner, label) {
    const c = two.makeCircle(cx, cy, r)
    c.fill = CFG.checker[owner] ?? CFG.checker[0]
    c.stroke = CFG.checkerStroke[owner] ?? '#000'
    c.linewidth = 1.5
    if (label) {
      const t = two.makeText(String(label), cx, cy)
      t.size = r
      t.fill = CFG.checkerLabel[owner] ?? '#fff'
      t.alignment = 'center'
      t.baseline = 'middle'
    }
  }

  function drawBarAndOff(colCenter, top, bottom, u, r) {
    const barX = colCenter(BAR_COL)
    const mid = (top + bottom) / 2
    if (board.Bar?.[0] > 0) placeChecker(barX, mid + r + 4, r, 0, board.Bar[0] > 1 ? board.Bar[0] : 0)
    if (board.Bar?.[1] > 0) placeChecker(barX, mid - r - 4, r, 1, board.Bar[1] > 1 ? board.Bar[1] : 0)
    const trayX = colCenter(OFF_COL)
    if (board.Off?.[0]) makeLabel(`▲ ${board.Off[0]}`, trayX, bottom - u * 0.6, u * 0.42)
    if (board.Off?.[1]) makeLabel(`▼ ${board.Off[1]}`, trayX, top + u * 0.6, u * 0.42)
  }

  function drawCube(colCenter, top, bottom, u) {
    const size = u * 0.9
    const x = colCenter(BAR_COL)
    let y = (top + bottom) / 2
    if (!cube.centered) y = cube.owner === 1 ? top + size : bottom - size
    const box = two.makeRoundedRectangle(x, y, size, size, size * 0.18)
    box.fill = CFG.cubeFill
    box.stroke = '#000'
    box.linewidth = 1.5
    const t = two.makeText(String(cube.value ?? 1), x, y)
    t.size = size * 0.55
    t.fill = CFG.cubeText
    t.alignment = 'center'
    t.baseline = 'middle'
  }

  function drawScore(x0, top, bottom, u) {
    makeLabel(`P2  ${score[1] ?? 0}`, x0 + u * 0.9, top + u * 0.5, u * 0.42)
    makeLabel(`P1  ${score[0] ?? 0}`, x0 + u * 0.9, bottom - u * 0.5, u * 0.42)
  }

  function makeLabel(str, x, y, size) {
    const t = two.makeText(str, x, y)
    t.size = size
    t.fill = CFG.text
    t.alignment = 'center'
    t.baseline = 'middle'
  }
</script>

<div class="board" bind:this={container}></div>

<style>
  .board {
    width: 100%;
    aspect-ratio: 14 / 11;
    min-height: 120px;
  }
  .board :global(svg) {
    display: block;
    width: 100%;
    height: 100%;
  }
</style>
