package matexport

import (
	"strings"
	"testing"

	"lazybg/internal/bg"
)

// sampleMatch builds a small but representative match: an opening played by the
// right-column player (P2), a normal reply, a "cannot move", and a cube sequence.
func sampleMatch() bg.Match {
	return bg.Match{
		Length:  3,
		Players: [2]string{"Alice", "Bob"}, // P1=left=Alice, P2=right=Bob
		Meta: []bg.MetaField{
			{Key: "Site", Value: "Test"},
			{Key: "Player 1", Value: "Alice"},
			{Key: "Player 2", Value: "Bob"},
		},
		Games: []bg.Game{{
			Number:     1,
			StartScore: [2]int{0, 0},
			Plies: []bg.Ply{
				{Player: bg.P2, Dice: bg.Dice{2, 1}, Notation: "24/23 13/11"}, // opening by right col
				{Player: bg.P1, Dice: bg.Dice{3, 1}, Notation: "8/5 6/5"},
				{Player: bg.P2, Dice: bg.Dice{6, 6}, CannotMove: true},
				{Player: bg.P1, Cube: bg.Double, CubeValue: 2},
				{Player: bg.P2, Cube: bg.Take},
			},
			Result: &bg.GameResult{Winner: bg.P1, Points: 2},
		}},
	}
}

func lines(s string) []string { return strings.Split(s, "\n") }

func hasLine(t *testing.T, out, want string) {
	t.Helper()
	for _, l := range lines(out) {
		if l == want {
			return
		}
	}
	t.Errorf("expected an exact line:\n  %q\nnot found in output:\n%s", want, out)
}

func TestWrite_HeaderAndMatchLength(t *testing.T) {
	out := Write(sampleMatch())
	if !strings.HasPrefix(out, "\uFEFF") {
		t.Errorf("output should start with a UTF-8 BOM")
	}
	// The BOM attaches to line 1, so match the Site field with Contains.
	if !strings.Contains(out, `; [Site "Test"]`) {
		t.Errorf("missing Site metadata line in:\n%s", out)
	}
	hasLine(t, out, `; [Player 1 "Alice"]`)
	hasLine(t, out, "3 point match")
	hasLine(t, out, " Game 1")
}

func TestWrite_GameScoreHeader_RightColumnAtCol39(t *testing.T) {
	out := Write(sampleMatch())
	// left: " Alice : 0"; right name begins at column 39.
	want := " Alice : 0" + strings.Repeat(" ", 39-len(" Alice : 0")) + "Bob : 0"
	hasLine(t, out, want)
}

func TestWrite_MoveRows_ColumnsAndTokens(t *testing.T) {
	out := Write(sampleMatch())
	// Opening by the right-column player sits alone on row 1 (left blank),
	// right content begins at column 39 (5 for "%3d) " + 34 pad).
	hasLine(t, out, "  1) "+strings.Repeat(" ", 34)+"21: 24/23 13/11")
	// Row 2 pairs P1's first move (left) with P2's "Cannot Move" (right).
	hasLine(t, out, "  2) "+pad("31: 8/5 6/5", 34)+"66: Cannot Move")
	// Row 3 pairs P1's double (left) with P2's take (right).
	hasLine(t, out, "  3) "+pad(" Doubles => 2", 34)+" Takes")
}

func TestWrite_WinLine(t *testing.T) {
	out := Write(sampleMatch())
	// Winner is P1 (left column): win text aligned under the left column,
	// with the 5-char move-number gutter.
	hasLine(t, out, "     "+strings.TrimRight("Wins 2 points", " "))
}

func TestWrite_NoTrailingWhitespace(t *testing.T) {
	out := Write(sampleMatch())
	for i, l := range lines(out) {
		if l != strings.TrimRight(l, " ") {
			t.Errorf("line %d has trailing whitespace: %q", i+1, l)
		}
	}
}

func TestDice_String(t *testing.T) {
	if got := (bg.Dice{2, 1}).String(); got != "21" {
		t.Errorf("Dice{2,1}.String() = %q, want %q", got, "21")
	}
	if got := (bg.Dice{}).String(); got != "" {
		t.Errorf("zero Dice.String() = %q, want empty", got)
	}
}
