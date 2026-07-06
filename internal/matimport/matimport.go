// Package matimport parses a Jellyfish `.mat` / `.txt` match file into a
// bg.Match — the reverse of internal/matexport. It is robust to both our own
// writer's output and real gnubg/XG files (variable move-number width, "Wins N
// point [and the match]", unknown "????" moves, win text sharing a move row,
// capitalized Bar/Off). The move notation strings are kept verbatim; replaying
// them to boards is internal/derive's job.
package matimport

import (
	"regexp"
	"strconv"
	"strings"

	"lazybg/internal/bg"
)

var (
	metaRe    = regexp.MustCompile(`^;\s*\[([^\]]+?)\s+"(.*)"\]\s*$`)
	lenRe     = regexp.MustCompile(`^\s*(\d+)\s+point match`)
	gameRe    = regexp.MustCompile(`^\s*Game\s+(\d+)`)
	headerRe  = regexp.MustCompile(`^\s*(\S.*?)\s+:\s+(\d+)\s{2,}(\S.*?)\s+:\s+(\d+)\s*$`)
	moveRe    = regexp.MustCompile(`^\s*(\d+)\)\s?(.*)$`)
	gapRe     = regexp.MustCompile(`\s{2,}`)
	diceRe    = regexp.MustCompile(`^(\d)(\d):\s*(.*)$`)
	winRe     = regexp.MustCompile(`Wins\s+(\d+)\s+point`)
	cubeValRe = regexp.MustCompile(`Doubles\s*=>\s*(\d+)`)
)

// Parse reads a .mat document into a bg.Match.
func Parse(s string) (bg.Match, error) {
	s = strings.TrimPrefix(s, "\uFEFF")
	var m bg.Match
	var cur *bg.Game
	needHeader := false

	flush := func() {
		if cur != nil {
			m.Games = append(m.Games, *cur)
			cur = nil
		}
	}

	for _, line := range strings.Split(s, "\n") {
		switch {
		case metaRe.MatchString(line):
			mm := metaRe.FindStringSubmatch(line)
			// Preserve metadata verbatim. Do NOT derive Players from the
			// [Player 1]/[Player 2] tags: in real gnubg files those can be
			// opposite to the column order. Player identity (left=P1, right=P2)
			// comes from the game header below.
			m.Meta = append(m.Meta, bg.MetaField{Key: strings.TrimSpace(mm[1]), Value: mm[2]})

		case lenRe.MatchString(line):
			m.Length, _ = strconv.Atoi(lenRe.FindStringSubmatch(line)[1])

		case gameRe.MatchString(line):
			flush()
			n, _ := strconv.Atoi(gameRe.FindStringSubmatch(line)[1])
			cur = &bg.Game{Number: n}
			needHeader = true

		case needHeader && headerRe.MatchString(line):
			hm := headerRe.FindStringSubmatch(line)
			s1, _ := strconv.Atoi(hm[2])
			s2, _ := strconv.Atoi(hm[4])
			cur.StartScore = [2]int{s1, s2}
			if m.Players[0] == "" {
				m.Players[0] = strings.TrimSpace(hm[1])
			}
			if m.Players[1] == "" {
				m.Players[1] = strings.TrimSpace(hm[3])
			}
			needHeader = false

		case cur != nil && moveRe.MatchString(line):
			left, right := splitCells(moveRe.FindStringSubmatch(line)[2])
			parseCell(cur, bg.P1, left)
			parseCell(cur, bg.P2, right)

		case cur != nil && strings.Contains(line, "Wins"):
			// Standalone win line; column decides the winner.
			player := bg.P1
			if idx := strings.Index(line, "Wins"); idx >= 39 {
				player = bg.P2
			}
			applyWin(cur, player, line)
		}
	}
	flush()
	return m, nil
}

// splitCells divides a move row's content (after "N) ") into the left (P1) and
// right (P2) cells, splitting on the first run of 2+ spaces.
func splitCells(rest string) (left, right string) {
	parts := gapRe.Split(rest, 2)
	left = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		right = strings.TrimSpace(parts[1])
	}
	return
}

// parseCell interprets one cell and appends the resulting Ply (if any) to the
// game, or records a win if the cell carries "Wins".
func parseCell(g *bg.Game, player bg.Player, cell string) {
	if cell == "" {
		return
	}
	if strings.Contains(cell, "Wins") {
		applyWin(g, player, cell)
		return
	}
	ply := bg.Ply{Player: player}
	switch {
	case strings.Contains(cell, "Doubles"):
		ply.Cube = bg.Double
		if cm := cubeValRe.FindStringSubmatch(cell); cm != nil {
			ply.CubeValue, _ = strconv.Atoi(cm[1])
		}
	case strings.EqualFold(cell, "Takes"):
		ply.Cube = bg.Take
	case strings.EqualFold(cell, "Drops"):
		ply.Cube = bg.Drop
	default:
		dm := diceRe.FindStringSubmatch(cell)
		if dm == nil {
			return // unrecognized cell; ignore
		}
		d0, _ := strconv.Atoi(dm[1])
		d1, _ := strconv.Atoi(dm[2])
		ply.Dice = bg.Dice{d0, d1}
		move := strings.TrimSpace(dm[3])
		if strings.EqualFold(move, "Cannot Move") {
			ply.CannotMove = true
		} else {
			ply.Notation = move // verbatim, incl. "????" / hit markers / groupings
		}
	}
	g.Plies = append(g.Plies, ply)
}

func applyWin(g *bg.Game, player bg.Player, text string) {
	pts := 0
	if wm := winRe.FindStringSubmatch(text); wm != nil {
		pts, _ = strconv.Atoi(wm[1])
	}
	g.Result = &bg.GameResult{Winner: player, Points: pts}
}
