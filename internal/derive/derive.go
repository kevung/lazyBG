// Package derive replays an imported bg.Match to reconstruct the board at every
// turn — the "labels for free" step of the corpus (docs/experiment-plan.md §5,
// domain-model §7 "Ground-Truth Derivation"). It applies player-relative move
// notation directly to an absolute board (handling bar entry, bear-off, hits,
// chained hops, and "(n)" groupings), so it needs no engine — the transcription
// is trusted ground truth.
package derive

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"lazybg/internal/bg"
)

const (
	barPt = 25 // player-relative "bar"
	offPt = 0  // player-relative "off"
)

var (
	// ErrUnknownMove marks a turn whose move was not recorded ("????").
	ErrUnknownMove = errors.New("unrecorded move")
	// ErrBoardUnknown marks turns after an unknown one, where the board can no
	// longer be trusted.
	ErrBoardUnknown = errors.New("board unknown after an unrecorded move")
)

// TurnState is one replayed ply: the board before and after, and whether a
// checker move was actually applied. Post is a valid board label iff Err == nil.
type TurnState struct {
	Game     int
	Ply      int
	Player   bg.Player
	Dice     bg.Dice
	Notation string
	Cube     bg.CubeAction
	Pre      bg.Board
	Post     bg.Board
	Applied  bool // a checker move changed the board
	Err      error
}

// Replay walks a match game-by-game from the opening, applying each ply's
// notation. Cube actions and "Cannot Move" leave the board unchanged (still a
// valid label). An unrecorded "????" (or an inconsistent move) flags that turn
// and every later turn in the game as board-unknown.
func Replay(m bg.Match) []TurnState {
	var out []TurnState
	for _, g := range m.Games {
		board := bg.StandardStart()
		unknown := false
		for pi, ply := range g.Plies {
			ts := TurnState{
				Game: g.Number, Ply: pi, Player: ply.Player, Dice: ply.Dice,
				Notation: ply.Notation, Cube: ply.Cube, Pre: board,
			}
			switch {
			case unknown:
				ts.Err = ErrBoardUnknown
				ts.Post = board
			case ply.Cube != bg.NoCube || ply.CannotMove || ply.Notation == "":
				ts.Post = board // no checker movement
			case ply.Notation == "????":
				ts.Err = ErrUnknownMove
				ts.Post = board
				unknown = true
			default:
				nb, err := ApplyNotation(board, ply.Player, ply.Notation)
				if err != nil {
					ts.Err = err
					ts.Post = board
					unknown = true
				} else {
					board = nb
					ts.Post = board
					ts.Applied = true
				}
			}
			out = append(out, ts)
		}
	}
	return out
}

// ApplyNotation applies one ply's player-relative notation to a copy of board
// and returns the result. Errors if the notation is inconsistent with the board.
func ApplyNotation(board bg.Board, player bg.Player, notation string) (bg.Board, error) {
	hops, err := parsePlays(notation)
	if err != nil {
		return board, err
	}
	b := board // struct copy (arrays copy by value)
	for _, h := range hops {
		if err := applyHop(&b, player, h); err != nil {
			return board, err
		}
	}
	return b, nil
}

type hop struct{ from, to int }

// parsePlays turns "24/18/13 6/5*(2)" into ordered hops in player-relative
// coordinates (barPt/offPt sentinels).
func parsePlays(notation string) ([]hop, error) {
	var hops []hop
	for _, tok := range strings.Fields(notation) {
		tok = strings.ReplaceAll(tok, "*", "") // hit markers are hints; we detect hits
		reps := 1
		if i := strings.IndexByte(tok, '('); i >= 0 {
			j := strings.IndexByte(tok, ')')
			if j <= i {
				return nil, fmt.Errorf("bad grouping in %q", tok)
			}
			n, err := strconv.Atoi(tok[i+1 : j])
			if err != nil {
				return nil, fmt.Errorf("bad grouping in %q", tok)
			}
			reps = n
			tok = tok[:i]
		}
		segs := strings.Split(tok, "/")
		if len(segs) < 2 {
			return nil, fmt.Errorf("bad play %q", tok)
		}
		pts := make([]int, len(segs))
		for k, s := range segs {
			p, err := pointOf(s)
			if err != nil {
				return nil, err
			}
			pts[k] = p
		}
		for r := 0; r < reps; r++ {
			for k := 0; k+1 < len(pts); k++ {
				hops = append(hops, hop{from: pts[k], to: pts[k+1]})
			}
		}
	}
	return hops, nil
}

func pointOf(s string) (int, error) {
	switch {
	case strings.EqualFold(s, "bar"):
		return barPt, nil
	case strings.EqualFold(s, "off"):
		return offPt, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 24 {
		return 0, fmt.Errorf("bad point %q", s)
	}
	return n, nil
}

// applyHop moves one checker of player from h.from to h.to, resolving hits.
func applyHop(b *bg.Board, player bg.Player, h hop) error {
	// remove from source
	if h.from == barPt {
		if b.Bar[player] <= 0 {
			return fmt.Errorf("no checker on bar to enter")
		}
		b.Bar[player]--
	} else {
		abs := playerAbs(player, h.from)
		if b.Pts[abs].N <= 0 || b.Pts[abs].Owner != player {
			return fmt.Errorf("no %v checker at point %d (abs %d)", player, h.from, abs)
		}
		b.Pts[abs].N--
		if b.Pts[abs].N == 0 {
			b.Pts[abs] = bg.Point{} // normalize empty points (drop stale owner)
		}
	}

	// place at destination
	if h.to == offPt {
		b.Off[player]++
		return nil
	}
	abs := playerAbs(player, h.to)
	dst := b.Pts[abs]
	switch {
	case dst.N == 0:
		b.Pts[abs] = bg.Point{N: 1, Owner: player}
	case dst.Owner == player:
		b.Pts[abs] = bg.Point{N: dst.N + 1, Owner: player}
	case dst.N == 1: // opponent blot → hit
		b.Bar[other(player)]++
		b.Pts[abs] = bg.Point{N: 1, Owner: player}
	default: // landing on 2+ opponents is illegal
		return fmt.Errorf("point %d (abs %d) blocked by %d opponents", h.to, abs, dst.N)
	}
	return nil
}

// playerAbs maps a player-relative point (1..24) to an absolute board point.
// P1's home is point 1 (identity); P2's home is point 24 (mirror 25-r).
func playerAbs(player bg.Player, r int) int {
	if player == bg.P1 {
		return r
	}
	return 25 - r
}

func other(p bg.Player) bg.Player {
	if p == bg.P1 {
		return bg.P2
	}
	return bg.P1
}
