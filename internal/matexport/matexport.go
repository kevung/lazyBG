// Package matexport writes a bg.Match to the Jellyfish `.mat` / `.txt` match
// format (docs/architecture.md §3, "matexport"). This is the canonical lazyBG
// export, readable by gnubg, XG, and BGBlitz.
//
// Skeleton scope: the column geometry, cube tokens, "Cannot Move", and the
// two-column pairing (including the right-column opening offset) are faithful to
// the legacy exporter. Full fidelity to every gnubg edge case (score
// progression between games, resignations) is a later matexport-milestone
// concern; round-trip import lives with it.
package matexport

import (
	"fmt"
	"strings"

	"lazybg/internal/bg"
)

// Column geometry, matching the legacy exporter: the right column begins at
// character 39. The move-number gutter ("%3d) ") is 5 chars, so the left
// content field is padded to 34 (39 − 5).
const (
	rightCol  = 39
	gutter    = 5
	leftWidth = rightCol - gutter // 34
)

// Write renders the whole match as a .mat string (UTF-8, BOM-prefixed).
func Write(m bg.Match) string {
	var b strings.Builder
	b.WriteString("\uFEFF") // BOM, as in the reference files

	for _, f := range m.Meta {
		fmt.Fprintf(&b, "; [%s \"%s\"]\n", f.Key, f.Value)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "%d point match\n", m.Length)
	b.WriteString("\n")

	for _, g := range m.Games {
		writeGame(&b, m, g)
	}
	return b.String()
}

func writeGame(b *strings.Builder, m bg.Match, g bg.Game) {
	fmt.Fprintf(b, " Game %d\n", g.Number)

	left := fmt.Sprintf(" %s : %d", m.Players[bg.P1], g.StartScore[bg.P1])
	spacing := rightCol - len(left)
	if spacing < 1 {
		spacing = 1
	}
	right := fmt.Sprintf("%s : %d", m.Players[bg.P2], g.StartScore[bg.P2])
	b.WriteString(rstrip(left+strings.Repeat(" ", spacing)+right) + "\n")

	for i, row := range rowsOf(g.Plies) {
		line := fmt.Sprintf("%3d) ", i+1) + pad(row[bg.P1], leftWidth) + row[bg.P2]
		b.WriteString(rstrip(line) + "\n")
	}

	if g.Result != nil {
		writeWinLine(b, *g.Result)
	}
	b.WriteString("\n")
}

func writeWinLine(b *strings.Builder, r bg.GameResult) {
	txt := fmt.Sprintf("Wins %d point%s", r.Points, plural(r.Points))
	var line string
	if r.Winner == bg.P1 {
		line = strings.Repeat(" ", gutter) + pad(txt, leftWidth)
	} else {
		line = strings.Repeat(" ", gutter) + pad("", leftWidth) + txt
	}
	b.WriteString(rstrip(line) + "\n")
}

// rowsOf lays plies into two-column rows. Left column = P1, right = P2. If the
// first ply belongs to the right column (P2 won the opening roll), it sits alone
// on row 1 with a blank left cell, matching the Jellyfish convention. Otherwise
// a ply forces a new row whenever its column in the current row is already used.
func rowsOf(plies []bg.Ply) [][2]string {
	var out [][2]string
	var cur [2]string
	var used [2]bool
	flush := func() {
		out = append(out, cur)
		cur = [2]string{}
		used = [2]bool{}
	}

	i := 0
	if len(plies) > 0 && plies[0].Player == bg.P2 {
		cur[bg.P2] = content(plies[0])
		used[bg.P2] = true
		flush()
		i = 1
	}
	for ; i < len(plies); i++ {
		col := plies[i].Player
		if used[col] {
			flush()
		}
		cur[col] = content(plies[i])
		used[col] = true
	}
	if used[bg.P1] || used[bg.P2] {
		flush()
	}
	return out
}

// content renders one ply's cell text (without column padding).
func content(p bg.Ply) string {
	switch p.Cube {
	case bg.Double:
		return fmt.Sprintf(" Doubles => %d", p.CubeValue)
	case bg.Take:
		return " Takes"
	case bg.Drop:
		return " Drops"
	}
	d := p.Dice.String()
	switch {
	case p.CannotMove:
		return d + ": Cannot Move"
	case p.Notation == "":
		return d + ":"
	default:
		return d + ": " + p.Notation
	}
}

func pad(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func rstrip(s string) string { return strings.TrimRight(s, " ") }

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
