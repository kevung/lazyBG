package convert

import (
	"fmt"

	"github.com/kevung/xgparser/xgparser"
)

// replayMismatch applies the recorded move to PositionI and compares the 24
// board points against PositionEnd. Returns "" when they match. Bar/off counts
// (indices 0 and 25) are ignored. This is the ground-truth correctness check:
// if the rendered from/to/hit are right, the point occupancy must match.
func replayMismatch(m *xgparser.MoveEntry) string {
	s := int8(1)
	if m.ActiveP != 1 {
		s = -1
	}
	absPt := func(v int32) int {
		if m.ActiveP == 1 {
			return int(v) + 1
		}
		return 24 - int(v)
	}
	board := m.PositionI // absolute (player1) perspective
	for i := 0; i+1 < 8; i += 2 {
		f := m.Moves[i]
		if f == -1 {
			break
		}
		t := m.Moves[i+1]
		if f >= 0 && f <= 23 {
			board[absPt(f)] -= s
		}
		if t >= 0 && t <= 23 {
			if board[absPt(t)] == -s {
				board[absPt(t)] = s // hit
			} else {
				board[absPt(t)] += s
			}
		}
	}
	// PositionEnd is in the MOVER's perspective. Convert our absolute after-
	// board to the mover's frame before comparing: for player1 they coincide;
	// for player2, mirror (25-p) and negate.
	for p := 1; p <= 24; p++ {
		want := m.PositionEnd[p]
		var got int8
		if m.ActiveP == 1 {
			got = board[p]
		} else {
			got = -board[25-p]
		}
		if got != want {
			return fmt.Sprintf("replay!=end at pt%d got=%d want=%d", p, got, want)
		}
	}
	return ""
}
