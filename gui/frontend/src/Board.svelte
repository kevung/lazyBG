<script>
  // Reconstructed-board renderer (ux-spec §6) — "what the app believes".
  // two.js drawing ported from blunderDB's board (ADR-0005). Read-only, driven
  // purely by props. Orientation-aware (ADR-0006): the underlying bg.Board is
  // always canonical (P1 home 1..6); this renderer mirrors the GEOMETRY to draw
  // it in the same sense as the video, so the reconstructed board and the video
  // frame are directly comparable. Text (counts, score, cube) stays upright —
  // only positions mirror. The mirror is bar-centred, matching
  // bg.Orientation.TransformPoint (boardGeometry.transformPoint).
  //
  // Player 1 is the player at the BOTTOM of the video, always (ADR-0009): the
  // only mirror is left/right, so P1's half of the drawing never moves.
  import Two from 'two.js'
  import { onMount } from 'svelte'
  import { isTop, colOf, stack, flipH } from './lib/boardGeometry.js'
  import { labelOn, outlineOf } from './lib/checkerColors.js'

  let {
    board = null,
    cube = { value: 1, owner: 0, centered: true },
    score = [0, 0],
    orientation = 0,
    // Optional [P1, P2] names, printed on each player's own side. Player 1's
    // label always sits on the bottom row — that is what the invariant looks
    // like on screen, and it is how the setup panel states it (#63).
    playerLabels = null,
    // Optional [P1, P2] checker fill colours (the declared checkerA/checkerB).
    // When set, they override the defaults so the board matches the video — the
    // WYSIWYG orientation control relies on this (issue #37).
    checkerColors = null,
    // Optional measured board palette {pointA, pointB, felt} (#64). When set,
    // the reconstruction wears the capture's own colours instead of the
    // neutral wooden default, so the two panels differ only where the reading
    // does. Any field may be empty — each falls back on its own.
    boardColors = null,
  } = $props()

  const CFG = {
    boardFill: '#1c130c',
    tray: '#0f0a06',
    triFill1: '#8a6b4f',
    triFill2: '#5c4433',
    triStroke: '#00000055',
    // Fallback fills when no colours are declared, in SetupPanel's order
    // (P1 light, P2 dark) — a table in the other order painted P1 in the
    // colour the form had just assigned to P2 (#62). The rim and the stack
    // digit are NOT tabulated: they are derived from whichever fill actually
    // ends up on the checker, declared or not.
    checker: ['#e7e0d5', '#31221c'], // [owner0 = P1, owner1 = P2]
    barLine: '#000000',
    text: '#cbb79a',
    cubeFill: '#efe9dd',
    cubeText: '#1c130c',
  }

  // 15 columns wide: the bar (slot 0), six point columns each side (slots ±1..±6)
  // and the off tray at slot ±7 on Player 1's home side. 11 checker-heights tall.
  const NSLOTS = 15

  let container
  let two = null
  let ro = null

  const sizeOf = () => ({
    width: container?.clientWidth || 520,
    height: container?.clientHeight || 360,
  })

  onMount(() => {
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

  // Redraw whenever the data, orientation, or the canvas size changes.
  $effect(() => {
    board, cube, score, orientation, playerLabels, checkerColors, boardColors
    draw()
  })

  // Board paint: the measured palette when there is one, else the neutral
  // default. Felt doubles as the surface behind the triangles.
  const paint = () => ({
    felt: boardColors?.felt || CFG.boardFill,
    triA: boardColors?.pointA || CFG.triFill1,
    triB: boardColors?.pointB || CFG.triFill2,
  })

  function draw() {
    if (!two) return
    two.clear()
    const W = two.width || 520
    const H = two.height || 360
    const u = Math.min(W / NSLOTS, H / 11) * 0.96
    const boardW = u * NSLOTS
    const boardH = u * 11
    const cx = W / 2 // bar centre (slot 0)
    const yTop = (H - boardH) / 2
    const yBot = yTop + boardH
    const mid = (yTop + yBot) / 2
    const r = u * 0.46
    const leftX = cx - boardW / 2

    const fH = flipH(orientation)

    const xOf = (slot) => cx + (fH ? -slot : slot) * u
    const slotOf = (p) => colOf(p) - 6 // ±1..±6, never 0 (the bar)
    const onTop = (p) => isTop(p)

    const pal = paint()
    const bgRect = two.makeRectangle(W / 2, H / 2, boardW, boardH)
    bgRect.fill = pal.felt
    bgRect.noStroke()
    const tray = two.makeRectangle(xOf(7), H / 2, u, boardH)
    tray.fill = outlineOf(pal.felt)
    tray.noStroke()

    if (board) {
      drawTriangles(xOf, slotOf, onTop, yTop, yBot, u, pal)
      drawCheckers(xOf, slotOf, onTop, yTop, yBot, r)
      drawBarAndOff(xOf, cx, mid, yTop, yBot, u, r)
    }
    drawCube(cx, yTop, yBot, mid, u)
    drawScore(leftX, yTop, yBot, u)

    const bar = two.makeRectangle(cx, H / 2, u * 0.12, boardH)
    bar.fill = CFG.barLine
    bar.noStroke()

    two.update()
  }

  function drawTriangles(xOf, slotOf, onTop, yTop, yBot, u, pal) {
    for (let p = 1; p <= 24; p++) {
      const cx = xOf(slotOf(p))
      const half = u * 0.44
      const tri = onTop(p)
        ? two.makePath(cx - half, yTop, cx + half, yTop, cx, yTop + 5 * u)
        : two.makePath(cx - half, yBot, cx + half, yBot, cx, yBot - 5 * u)
      tri.fill = p % 2 === 0 ? pal.triA : pal.triB
      tri.stroke = CFG.triStroke
      tri.linewidth = 1
    }
  }

  function drawCheckers(xOf, slotOf, onTop, yTop, yBot, r) {
    for (let p = 1; p <= 24; p++) {
      const pt = board.Pts?.[p]
      if (!pt || pt.N === 0) continue
      const cx = xOf(slotOf(p))
      const top = onTop(p)
      const { drawn, overflow } = stack(pt.N)
      for (let i = 0; i < drawn; i++) {
        const cy = top ? yTop + r + i * (2 * r) : yBot - r - i * (2 * r)
        placeChecker(cx, cy, r, pt.Owner, i === drawn - 1 && overflow ? overflow : 0)
      }
    }
  }

  function placeChecker(cx, cy, r, owner, label) {
    const c = two.makeCircle(cx, cy, r)
    const fill = checkerColors?.[owner] ?? CFG.checker[owner] ?? CFG.checker[0]
    c.fill = fill
    c.stroke = outlineOf(fill)
    c.linewidth = 1.5
    if (label) {
      const t = two.makeText(String(label), cx, cy)
      t.size = r
      t.fill = labelOn(fill)
      t.alignment = 'center'
      t.baseline = 'middle'
    }
  }

  function drawBarAndOff(xOf, barX, mid, yTop, yBot, u, r) {
    // Bar checkers stack inward from the centre on their owner's side.
    if (board.Bar?.[0] > 0) {
      placeChecker(barX, mid + r + 4, r, 0, board.Bar[0] > 1 ? board.Bar[0] : 0)
    }
    if (board.Bar?.[1] > 0) {
      placeChecker(barX, mid - r - 4, r, 1, board.Bar[1] > 1 ? board.Bar[1] : 0)
    }
    const trayX = xOf(7)
    const p1Y = yBot - u * 0.6
    const p2Y = yTop + u * 0.6
    if (board.Off?.[0]) makeLabel(`▲ ${board.Off[0]}`, trayX, p1Y, u * 0.42)
    if (board.Off?.[1]) makeLabel(`▼ ${board.Off[1]}`, trayX, p2Y, u * 0.42)
  }

  function drawCube(barX, yTop, yBot, mid, u) {
    const size = u * 0.9
    let y = mid
    if (!cube.centered) {
      y = cube.owner === 0 ? yBot - size : yTop + size
    }
    const box = two.makeRoundedRectangle(barX, y, size, size, size * 0.18)
    box.fill = CFG.cubeFill
    box.stroke = '#000'
    box.linewidth = 1.5
    const t = two.makeText(String(cube.value ?? 1), barX, y)
    t.size = size * 0.55
    t.fill = CFG.cubeText
    t.alignment = 'center'
    t.baseline = 'middle'
  }

  // Player 1 is the bottom player, so its label is on the bottom row — the
  // invariant is shown, not stated (#63).
  function drawScore(leftX, yTop, yBot, u) {
    const name = (i) => playerLabels?.[i] || `P${i + 1}`
    makeLabel(`${name(0)}  ${score[0] ?? 0}`, leftX + u * 0.9, yBot - u * 0.5, u * 0.42)
    makeLabel(`${name(1)}  ${score[1] ?? 0}`, leftX + u * 0.9, yTop + u * 0.5, u * 0.42)
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
    aspect-ratio: 15 / 11;
    min-height: 120px;
  }
  .board :global(svg) {
    display: block;
    width: 100%;
    height: 100%;
  }
</style>
