// Package engine is the one place that bridges lazyBG's board model and the
// salvaged gnubg port (docs/domain-model.md §6, architecture §3). It exposes the
// engine's two roles: legality as a hard constraint and equity ranking as a soft
// prior. This is the trickiest coordinate seam in the project — keep it here.
package engine

import (
	"io/fs"
	"strconv"

	"lazybg/gnubg"
	"lazybg/internal/bg"
)

// Init loads the gnubg engine data (weights, bearoff DBs, match-equity tables).
// Call once at startup with the embedded/on-disk data FS.
func Init(dataDir fs.FS) error { return gnubg.Init(dataDir) }

// LegalMove is one legal move: its player-relative notation, the engine's
// equity (higher is better for the player on roll), and the resulting board.
type LegalMove struct {
	Notation string
	Equity   float64
	Result   bg.Board
}

// LegalMoves returns every legal move for the position, ranked best-first by
// equity. An empty slice means the player cannot move (dance).
func LegalMoves(pos bg.Position) ([]LegalMove, error) { return legalMoves(pos, true) }

// LegalMovesUnscored enumerates legal moves WITHOUT the neural-net equity
// evaluation — the fast path for wide hypothesis sweeps (e.g. trying all 21
// rolls). Equity is 0 on every move; order is gnubg's generation order.
func LegalMovesUnscored(pos bg.Position) ([]LegalMove, error) { return legalMoves(pos, false) }

func legalMoves(pos bg.Position, score bool) ([]LegalMove, error) {
	tb := toTanBoard(pos.Board)
	// gnubg's FindMoves moves anBoard[1]; with player==0 it passes the board
	// unswapped, so slot 1 (toTanBoard's P2) would be the mover. Our P1-on-roll
	// therefore needs player==1 and vice versa. The inversion was invisible on
	// mirror-symmetric positions (the standard start) — see the bar-entry
	// regression test.
	pml, err := gnubg.FindMoves(tb, [2]int{pos.Dice[0], pos.Dice[1]}, 1-int(pos.PlayerOnRoll), score, false)
	if err != nil {
		return nil, err
	}
	frame := onRollFrame(pos.Board, pos.PlayerOnRoll)

	n := pml.GetMovesNum()
	out := make([]LegalMove, 0, n)
	for i := 0; i < n; i++ {
		mv := pml.GetMove(i)
		notation, result := applyMove(frame, mv, pos.PlayerOnRoll)
		out = append(out, LegalMove{
			Notation: notation,
			Equity:   float64(mv.GetEquity()),
			Result:   result,
		})
	}
	return out, nil
}

// perspIndex maps an absolute board point (1..24) to a player's gnubg
// perspective index (0..23): P1's home (point 1) is index 0; P2's home (point
// 24) is index 0.
func perspIndex(who bg.Player, absPoint int) int {
	if who == bg.P1 {
		return absPoint - 1
	}
	return 24 - absPoint
}

// absPoint is the inverse of perspIndex.
func absPoint(who bg.Player, idx int) int {
	if who == bg.P1 {
		return idx + 1
	}
	return 24 - idx
}

// toTanBoard builds gnubg's board with P1 ("Black") in slot 0 and P2 ("White")
// in slot 1, each in its own perspective (index 24 = bar). FindMoves swaps
// internally based on the player argument.
func toTanBoard(b bg.Board) gnubg.TanBoard {
	var black, white [25]int
	for i := 1; i <= 24; i++ {
		p := b.Pts[i]
		if p.N == 0 {
			continue
		}
		if p.Owner == bg.P1 {
			black[perspIndex(bg.P1, i)] = p.N
		} else {
			white[perspIndex(bg.P2, i)] = p.N
		}
	}
	black[24] = b.Bar[bg.P1]
	white[24] = b.Bar[bg.P2]
	return gnubg.TanBoard{black, white}
}

// onRollFrame returns the board as [on-roll, opponent], each in its own
// perspective — the frame gnubg's returned plays are expressed in.
func onRollFrame(b bg.Board, onRoll bg.Player) gnubg.TanBoard {
	tb := toTanBoard(b)
	if onRoll == bg.P2 {
		return gnubg.TanBoard{tb[1], tb[0]}
	}
	return tb
}

// fromOnRollFrame converts an on-roll-frame board back to an absolute bg.Board.
func fromOnRollFrame(f gnubg.TanBoard, onRoll bg.Player) bg.Board {
	opp := other(onRoll)
	var b bg.Board
	for k := 0; k <= 23; k++ {
		if f[0][k] > 0 {
			b.Pts[absPoint(onRoll, k)] = bg.Point{N: f[0][k], Owner: onRoll}
		}
		if f[1][k] > 0 {
			b.Pts[absPoint(opp, k)] = bg.Point{N: f[1][k], Owner: opp}
		}
	}
	b.Bar[onRoll] = f[0][24]
	b.Bar[opp] = f[1][24]
	b.Off[onRoll] = 15 - onBoard(f[0])
	b.Off[opp] = 15 - onBoard(f[1])
	return b
}

func onBoard(side [25]int) int {
	s := 0
	for _, v := range side {
		s += v
	}
	return s
}

// applyMove applies a gnubg move to a copy of the on-roll frame, returning the
// player-relative notation (with hit markers) and the resulting absolute board.
func applyMove(frame gnubg.TanBoard, mv gnubg.Move, onRoll bg.Player) (string, bg.Board) {
	f := frame // array copy
	notation := ""
	for k := 0; k < mv.GetPlaysNum(); k++ {
		play := mv.GetPlay(k)
		from, to := play[0], play[1]

		f[0][from]--
		hit := false
		if to >= 0 && to <= 23 {
			f[0][to]++
			opp := 23 - to
			if f[1][opp] == 1 {
				f[1][opp] = 0
				f[1][24]++
				hit = true
			}
		}

		if k > 0 {
			notation += " "
		}
		notation += playNotation(from, to, hit)
	}
	if notation == "" {
		notation = "Cannot Move"
	}
	return notation, fromOnRollFrame(f, onRoll)
}

// playNotation renders one play in player-relative notation (index 0 → point 1,
// 24 → bar, off when the destination leaves the board).
func playNotation(from, to int, hit bool) string {
	var fromStr string
	if from == 24 {
		fromStr = "bar"
	} else {
		fromStr = strconv.Itoa(from + 1)
	}
	var toStr string
	if to < 0 || to > 23 {
		toStr = "off"
	} else {
		toStr = strconv.Itoa(to + 1)
	}
	s := fromStr + "/" + toStr
	if hit {
		s += "*"
	}
	return s
}

func other(p bg.Player) bg.Player {
	if p == bg.P1 {
		return bg.P2
	}
	return bg.P1
}
