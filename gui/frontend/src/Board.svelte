<script>
  // Reconstructed-board renderer: draws a bg.Board (Pts[25]{N,Owner},
  // Bar[2], Off[2]) as SVG — "what the app believes" (ux-spec §6).
  // Point 24 is top-left, 13 top-right side of the bar mirror... layout:
  // top row: 13..18 | bar | 19..24 (left→right), bottom row: 12..7 | bar | 6..1.
  export let board = null

  const W = 520
  const H = 360
  const PW = W / 14 // point width (12 points + bar + off column)
  const TRI = 140 // triangle height
  const R = 14 // checker radius

  // Column x for each point 1..24 (standard orientation, P1 home bottom-right).
  function colX(p) {
    if (p >= 13 && p <= 18) return (p - 13) * PW // top-left block
    if (p >= 19 && p <= 24) return (p - 12) * PW // top-right block (skip bar col 6)
    if (p >= 7 && p <= 12) return (12 - p) * PW // bottom-left block
    return (13 - p) * PW // 6..1 bottom-right (skip bar)
  }
  const isTop = (p) => p >= 13

  function checkers(pt, p) {
    if (!pt || pt.N === 0) return []
    const n = Math.min(pt.N, 5)
    const out = []
    for (let i = 0; i < n; i++) {
      const y = isTop(p) ? R + 4 + i * (2 * R - 2) : H - R - 4 - i * (2 * R - 2)
      out.push({ y, label: i === n - 1 && pt.N > 5 ? pt.N : null })
    }
    return out
  }
</script>

{#if board}
  <svg viewBox="0 0 {W} {H}" class="board">
    <rect width={W} height={H} fill="#3f2d1d" rx="6" />
    <rect x={6 * PW} width={PW} height={H} fill="#2a1e13" />
    {#each Array(24) as _, i}
      {@const p = i + 1}
      {@const x = colX(p)}
      <polygon
        points="{x},{isTop(p) ? 0 : H} {x + PW},{isTop(p) ? 0 : H} {x + PW / 2},{isTop(p) ? TRI : H - TRI}"
        fill={p % 2 === (isTop(p) ? 1 : 0) ? '#8a6b4f' : '#5c4433'}
      />
      {#each checkers(board.Pts[p], p) as c}
        <circle cx={x + PW / 2} cy={c.y} r={R} fill={board.Pts[p].Owner === 0 ? '#e7e0d5' : '#31221c'} stroke="#00000066" />
        {#if c.label}
          <text x={x + PW / 2} y={c.y + 4} text-anchor="middle" font-size="12" fill={board.Pts[p].Owner === 0 ? '#333' : '#eee'}>{c.label}</text>
        {/if}
      {/each}
    {/each}
    <!-- bar -->
    {#if board.Bar[0] > 0}
      <circle cx={6.5 * PW} cy={H / 2 + 24} r={R} fill="#e7e0d5" stroke="#000" />
      <text x={6.5 * PW} y={H / 2 + 28} text-anchor="middle" font-size="12" fill="#333">{board.Bar[0]}</text>
    {/if}
    {#if board.Bar[1] > 0}
      <circle cx={6.5 * PW} cy={H / 2 - 24} r={R} fill="#31221c" stroke="#000" />
      <text x={6.5 * PW} y={H / 2 - 20} text-anchor="middle" font-size="12" fill="#eee">{board.Bar[1]}</text>
    {/if}
    <!-- off trays -->
    <text x={13.5 * PW} y={H - 8} text-anchor="middle" font-size="11" fill="#cbb">off {board.Off[0]}</text>
    <text x={13.5 * PW} y={16} text-anchor="middle" font-size="11" fill="#cbb">off {board.Off[1]}</text>
  </svg>
{/if}

<style>
  .board {
    width: 100%;
    height: auto;
    display: block;
  }
</style>
