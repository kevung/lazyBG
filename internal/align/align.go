// Package align anchors a ground-truth transcription to its video: for each
// truth turn it finds the stable-board event where that turn's resulting
// board first appears (experiment-plan §4 "semi-auto alignment"). The truth
// gives the exact board sequence, so alignment is robust to a noisy reader —
// each candidate event is scored against a KNOWN board and a monotonic
// dynamic program picks the globally best assignment. The aligned ticks are
// the one label a .mat lacks; from them, labeled training crops fall out for
// free (§5 "labels for free").
package align

import (
	"lazybg/internal/bg"
	"lazybg/internal/derive"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/transcribe"
)

// Turn is one locatable ground-truth turn: a checker ply that changed the
// board, with the exact board it produced.
type Turn struct {
	Index    int // 1-based position in the match's full replay order
	Game     int
	Player   bg.Player
	Dice     bg.Dice
	Notation string
	Board    bg.Board // the truth board after this ply
}

// TruthTurns extracts the locatable turns of a match: applied checker plies
// only (cube actions and dances leave no board of their own to find).
func TruthTurns(m bg.Match) []Turn {
	var out []Turn
	for i, ts := range derive.Replay(m) {
		if ts.Err != nil || !ts.Applied {
			continue
		}
		out = append(out, Turn{
			Index:    i + 1,
			Game:     ts.Game,
			Player:   ts.Player,
			Dice:     ts.Dice,
			Notation: ts.Notation,
			Board:    ts.Post,
		})
	}
	return out
}

// Alignment tuning. Scores are WholeBoardMatch values in [0,1]; an assignment
// only pays if it clears the baseline (below it, skipping the turn wins).
const (
	scoreBaseline = 0.78 // ≈ what a WRONG board scores against a noisy reading
	skipTurnCost  = 0.02 // small: prefer skipping over a sub-baseline match
)

// Align assigns each truth turn to an event index (or -1 when its board never
// shows), with strictly increasing event indices — a monotonic best-path DP.
func Align(turns []Turn, events []transcribe.Event) []int {
	K, N := len(turns), len(events)
	if K == 0 || N == 0 {
		out := make([]int, K)
		for i := range out {
			out[i] = -1
		}
		return out
	}

	// Emission scores.
	s := make([][]float64, K)
	for k := range s {
		s[k] = make([]float64, N)
		for i := range s[k] {
			s[k][i] = boarddiff.WholeBoardMatch(turns[k].Board, events[i].Obs) - scoreBaseline
		}
	}

	// dp[k][i]: best total aligning turns[0..k-1] using events[0..i-1].
	// Moves: skip event (i-1), skip turn (k-1, cost), assign turn k-1 to
	// event i-1 (must beat 0 to be worth taking; enforced by the skip path).
	dp := make([][]float64, K+1)
	from := make([][]byte, K+1) // 'e' skip event, 't' skip turn, 'a' assign
	for k := 0; k <= K; k++ {
		dp[k] = make([]float64, N+1)
		from[k] = make([]byte, N+1)
	}
	for k := 1; k <= K; k++ {
		dp[k][0] = dp[k-1][0] - skipTurnCost
		from[k][0] = 't'
	}
	for k := 1; k <= K; k++ {
		for i := 1; i <= N; i++ {
			best, via := dp[k][i-1], byte('e')
			if v := dp[k-1][i] - skipTurnCost; v > best {
				best, via = v, 't'
			}
			if v := dp[k-1][i-1] + s[k-1][i-1]; v > best {
				best, via = v, 'a'
			}
			dp[k][i], from[k][i] = best, via
		}
	}

	out := make([]int, K)
	for i := range out {
		out[i] = -1
	}
	for k, i := K, N; k > 0; {
		switch from[k][i] {
		case 'a':
			out[k-1] = i - 1
			k--
			i--
		case 't':
			k--
		default:
			i--
		}
	}
	return out
}
