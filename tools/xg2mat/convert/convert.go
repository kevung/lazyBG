// Package convert turns an eXtreme Gammon .xg match into Jellyfish .mat text.
package convert

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kevung/xgparser/xgparser"
)

// Result holds the produced MAT text plus any non-fatal validation warnings.
type Result struct {
	MAT       string
	Warnings  []string
	Games     int
	Length    int32
	Player1   string
	Player2   string
	matchMeta matchMeta
}

type cell struct {
	player int // +1 = player1 (left), -1 = player2 (right)
	text   string
}

type gameOut struct {
	number     int32
	score      [2]int32 // initial score [p1, p2]
	cells      []cell
	winner     int32 // +1 = p1, -1 = p2, 0 = unknown
	points     int32
	resignLike bool
}

// FromFile reads an .xg file and converts it to MAT text.
func FromFile(path string) (*Result, error) {
	imp := xgparser.NewImport(path)
	segs, err := imp.GetFileSegments()
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}
	var recs []interface{}
	ver := int32(-1)
	for _, seg := range segs {
		if seg.Type != xgparser.SegmentXGGameFile {
			continue
		}
		rs, err := xgparser.ParseGameFile(seg.Data, ver)
		if err != nil {
			return nil, fmt.Errorf("parse game file: %w", err)
		}
		for _, r := range rs {
			if h, ok := r.(*xgparser.HeaderMatchEntry); ok {
				ver = h.Version
			}
			recs = append(recs, r)
		}
	}
	return fromRecords(recs)
}

func fromRecords(recs []interface{}) (*Result, error) {
	res := &Result{}
	var games []gameOut
	var cur *gameOut
	cube := 1 // per-game doubling cube value

	flush := func() {
		if cur != nil {
			games = append(games, *cur)
			cur = nil
		}
	}

	for _, rec := range recs {
		switch r := rec.(type) {
		case *xgparser.HeaderMatchEntry:
			res.Length = r.MatchLength
			res.Player1 = clean(r.Player1)
			res.Player2 = clean(r.Player2)
			res.matchMeta = matchMeta{
				event: clean(r.Event),
				site:  clean(r.Location),
				round: clean(r.Round),
				date:  clean(r.Date),
			}
		case *xgparser.HeaderGameEntry:
			flush()
			cube = 1
			cur = &gameOut{number: r.GameNumber, score: [2]int32{r.Score1, r.Score2}}
		case *xgparser.CubeEntry:
			if cur == nil {
				continue
			}
			if r.Double == 1 { // a double was offered by ActiveP
				cube *= 2
				cur.cells = append(cur.cells, cell{int(r.ActiveP), fmt.Sprintf("Doubles => %d", cube)})
				switch r.Take {
				case 1:
					cur.cells = append(cur.cells, cell{-int(r.ActiveP), "Takes"})
				case 0:
					cur.cells = append(cur.cells, cell{-int(r.ActiveP), "Drops"})
				default:
					res.Warnings = append(res.Warnings,
						fmt.Sprintf("game %d: unexpected Take=%d after double", cur.number, r.Take))
				}
			} else if r.Double != 0 && r.Double != -2 && r.Double != -1 {
				// -2 = initial cube entry, 0 = no double, -1 = double not
				// available (e.g. Crawford game). Anything else is unexpected.
				res.Warnings = append(res.Warnings,
					fmt.Sprintf("game %d: unexpected Double=%d", cur.number, r.Double))
			}
		case *xgparser.MoveEntry:
			if cur == nil {
				continue
			}
			if !r.Played { // phantom end-of-game placeholder (moves all zero)
				continue
			}
			text, w := formatMove(r)
			cur.cells = append(cur.cells, cell{int(r.ActiveP), text})
			if w != "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("game %d: %s", cur.number, w))
			}
			if v := replayMismatch(r); v != "" {
				res.Warnings = append(res.Warnings, fmt.Sprintf("game %d: %s [%s]", cur.number, v, text))
			}
		case *xgparser.FooterGameEntry:
			if cur == nil {
				continue
			}
			cur.winner = r.Winner
			cur.points = r.PointsWon
			// resignLike: last checker cell not by winner (best-effort)
			cur.resignLike = int32(lastMovePlayer(cur.cells)) != r.Winner && r.Winner != 0
		}
	}
	flush()

	res.Games = len(games)
	res.MAT = render(res, games)
	return res, nil
}

type matchMeta struct{ event, site, round, date string }

func clean(s string) string {
	s = strings.TrimRight(s, "\x00")
	s = strings.TrimSpace(s)
	return s
}

func lastMovePlayer(cells []cell) int {
	for i := len(cells) - 1; i >= 0; i-- {
		t := cells[i].text
		if strings.Contains(t, ":") { // a dice+move cell
			return cells[i].player
		}
	}
	return 0
}

// formatMove renders "DD: n/m n/m" with hit markers; empty play -> "DD:".
func formatMove(m *xgparser.MoveEntry) (string, string) {
	dice := fmt.Sprintf("%d%d", m.Dice[0], m.Dice[1])

	type pair struct {
		from, to int32
		hit      bool
	}
	var pairs []pair

	// PositionI is in ABSOLUTE (player1) perspective: player1 positive, player2
	// negative, index 1..24 = absolute board points. Moves are in the MOVER's
	// own perspective, so map mover point -> absolute index (mirror for
	// player2) and use the mover's sign for hit detection. (PositionEnd, by
	// contrast, is in the mover's own perspective — see verify.go.)
	s := int8(1)
	if m.ActiveP != 1 {
		s = -1
	}
	absPt := func(v int32) int { // mover point v (0..23) -> absolute index 1..24
		if m.ActiveP == 1 {
			return int(v) + 1
		}
		return 24 - int(v)
	}
	board := m.PositionI
	var warn string
	for i := 0; i+1 < 8; i += 2 {
		f := m.Moves[i]
		if f == -1 {
			break
		}
		t := m.Moves[i+1]
		p := pair{from: f, to: t}
		if t >= 0 && t <= 23 { // destination is a point
			di := absPt(t)
			switch {
			case board[di] == -s: // single opponent blot -> hit
				p.hit = true
				board[di] = s
			case board[di]*s < 0: // 2+ opponents: should be impossible
				warn = fmt.Sprintf("blocked land on point %d", t+1)
			default:
				board[di] += s
			}
		}
		if f >= 0 && f <= 23 {
			board[absPt(f)] -= s
		}
		pairs = append(pairs, p)
	}

	if len(pairs) == 0 {
		return dice + ":", warn
	}

	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].from > pairs[j].from })

	var sb strings.Builder
	sb.WriteString(dice)
	sb.WriteString(":")
	for _, p := range pairs {
		sb.WriteString(" ")
		sb.WriteString(pt(p.from))
		sb.WriteString("/")
		sb.WriteString(pt(p.to))
		if p.hit {
			sb.WriteString("*")
		}
	}
	return sb.String(), warn
}

func pt(v int32) string {
	switch {
	case v == 24:
		return "bar"
	case v < 0: // XG encodes bear-off as a negative destination (-1..-6)
		return "0"
	default:
		return fmt.Sprintf("%d", v+1)
	}
}

