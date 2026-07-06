package convert

import (
	"fmt"
	"strings"
)

const rightCol = 30 // left column padded to this width before player2's cell

func render(res *Result, games []gameOut) string {
	var b strings.Builder

	// Metadata comments (parser reads Event/Site/Round/EventDate from these).
	m := res.matchMeta
	if m.event != "" {
		fmt.Fprintf(&b, "; [Event \"%s\"]\n", m.event)
	}
	if m.site != "" {
		fmt.Fprintf(&b, "; [Site \"%s\"]\n", m.site)
	}
	if m.round != "" {
		fmt.Fprintf(&b, "; [Round \"%s\"]\n", m.round)
	}
	if d := matDate(m.date); d != "" {
		fmt.Fprintf(&b, "; [EventDate \"%s\"]\n", d)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, " %d point match\n\n", res.Length)

	for _, g := range games {
		fmt.Fprintf(&b, " Game %d\n", g.number)
		left := fmt.Sprintf("%s : %d", res.Player1, g.score[0])
		fmt.Fprintf(&b, " %s%s : %d\n", padTo(left, rightCol), res.Player2, g.score[1])

		for _, ln := range layout(g.cells) {
			b.WriteString(ln)
			b.WriteString("\n")
		}

		if g.points > 0 {
			word := "points"
			if g.points == 1 {
				word = "point"
			}
			wins := fmt.Sprintf("Wins %d %s", g.points, word)
			if g.winner == 1 { // player1 -> left column
				fmt.Fprintf(&b, "      %s\n", wins)
			} else { // player2 -> right column
				fmt.Fprintf(&b, "%s%s\n", strings.Repeat(" ", 34), wins)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// layout places temporally-ordered cells into numbered two-column rows.
func layout(cells []cell) []string {
	type row struct{ l, r string }
	var rows []row
	for _, c := range cells {
		if c.player == 1 { // player1 always begins a new row (left column)
			rows = append(rows, row{l: c.text})
		} else { // player2 fills the current row's right column, or starts one
			if len(rows) == 0 || rows[len(rows)-1].r != "" {
				rows = append(rows, row{})
			}
			rows[len(rows)-1].r = c.text
		}
	}

	out := make([]string, 0, len(rows))
	for i, rw := range rows {
		n := i + 1
		if rw.r == "" {
			out = append(out, fmt.Sprintf("%3d) %s", n, rw.l))
		} else {
			out = append(out, fmt.Sprintf("%3d) %s%s", n, padTo(rw.l, rightCol), rw.r))
		}
	}
	return out
}

func padTo(s string, w int) string {
	if len(s) >= w {
		return s + "   " // guarantee >=3 spaces so the column split is detected
	}
	return s + strings.Repeat(" ", w-len(s))
}

// matDate converts "2025-11-15 05:00:00" to "2025.11.15".
func matDate(s string) string {
	if len(s) < 10 {
		return ""
	}
	d := s[:10]
	d = strings.ReplaceAll(d, "-", ".")
	if len(d) != 10 || d[4] != '.' || d[7] != '.' {
		return ""
	}
	return d
}
