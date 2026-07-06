package matimport

import (
	"os"
	"strings"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/matexport"
)

func sample() bg.Match {
	return bg.Match{
		Length:  3,
		Players: [2]string{"Alice", "Bob"},
		Meta: []bg.MetaField{
			{Key: "Site", Value: "Test"},
			{Key: "Player 1", Value: "Alice"},
			{Key: "Player 2", Value: "Bob"},
		},
		Games: []bg.Game{{
			Number:     1,
			StartScore: [2]int{0, 0},
			Plies: []bg.Ply{
				{Player: bg.P2, Dice: bg.Dice{2, 1}, Notation: "24/23 13/11"},
				{Player: bg.P1, Dice: bg.Dice{3, 1}, Notation: "8/5 6/5"},
				{Player: bg.P2, Dice: bg.Dice{6, 6}, CannotMove: true},
				{Player: bg.P1, Cube: bg.Double, CubeValue: 2},
				{Player: bg.P2, Cube: bg.Take},
			},
			Result: &bg.GameResult{Winner: bg.P1, Points: 2},
		}},
	}
}

// Write → Parse → Write must be byte-stable: the importer recovers everything
// the exporter emitted.
func TestRoundTrip_WithWriter(t *testing.T) {
	orig := matexport.Write(sample())
	got, err := Parse(orig)
	if err != nil {
		t.Fatal(err)
	}
	if reexport := matexport.Write(got); reexport != orig {
		t.Errorf("round-trip not stable.\n--- original ---\n%q\n--- re-export ---\n%q", orig, reexport)
	}
}

func TestParse_RecoversFields(t *testing.T) {
	m, err := Parse(matexport.Write(sample()))
	if err != nil {
		t.Fatal(err)
	}
	if m.Length != 3 || m.Players != [2]string{"Alice", "Bob"} {
		t.Errorf("length/players wrong: %d %v", m.Length, m.Players)
	}
	if len(m.Games) != 1 || len(m.Games[0].Plies) != 5 {
		t.Fatalf("games/plies: %d games", len(m.Games))
	}
	p := m.Games[0].Plies
	if p[0].Player != bg.P2 || p[0].Dice != (bg.Dice{2, 1}) || p[0].Notation != "24/23 13/11" {
		t.Errorf("ply0 = %+v", p[0])
	}
	if !p[2].CannotMove || p[2].Dice != (bg.Dice{6, 6}) {
		t.Errorf("ply2 (cannot move) = %+v", p[2])
	}
	if p[3].Cube != bg.Double || p[3].CubeValue != 2 {
		t.Errorf("ply3 (double) = %+v", p[3])
	}
	if p[4].Cube != bg.Take {
		t.Errorf("ply4 (take) = %+v", p[4])
	}
	if m.Games[0].Result == nil || m.Games[0].Result.Winner != bg.P1 || m.Games[0].Result.Points != 2 {
		t.Errorf("result = %+v", m.Games[0].Result)
	}
}

// Parsing a real gnubg-format match (4 games, unknown "????" moves, capitalized
// Bar/Off, groupings, win-on-a-move-row, running scores).
func TestParse_RealMatchFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/mat/match1.txt")
	if err != nil {
		t.Skipf("fixture not present: %v", err)
	}
	m, err := Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if m.Length != 7 {
		t.Errorf("match length = %d, want 7", m.Length)
	}
	if !strings.Contains(m.Players[0], "Kévin") || !strings.Contains(m.Players[1], "Johan") {
		t.Errorf("players = %v", m.Players)
	}
	if len(m.Games) != 4 {
		t.Fatalf("games = %d, want 4", len(m.Games))
	}
	// Running scores from the game headers.
	wantScores := [][2]int{{0, 0}, {0, 2}, {1, 2}, {1, 6}}
	for i, ws := range wantScores {
		if m.Games[i].StartScore != ws {
			t.Errorf("game %d start score = %v, want %v", i+1, m.Games[i].StartScore, ws)
		}
	}
	// Game 1 opening: Johan (right/P2) plays 24/23 13/11.
	g1 := m.Games[0].Plies
	if g1[0].Player != bg.P2 || g1[0].Notation != "24/23 13/11" {
		t.Errorf("game1 ply0 = %+v", g1[0])
	}
	// Winners: Johan (P2) 2, Kévin (P1) 1, Johan (P2) 4, Johan (P2) 1.
	gr := func(w bg.Player, p int) bg.GameResult { return bg.GameResult{Winner: w, Points: p} }
	wantWin := []bg.GameResult{gr(bg.P2, 2), gr(bg.P1, 1), gr(bg.P2, 4), gr(bg.P2, 1)}
	for i, w := range wantWin {
		got := m.Games[i].Result
		if got == nil || *got != w {
			t.Errorf("game %d result = %v, want %v", i+1, got, w)
		}
	}
	// The two "????" unknown moves are captured verbatim.
	if last := lastPly(m.Games[0]); last.Notation != "????" {
		t.Errorf("game1 last ply notation = %q, want ????", last.Notation)
	}
}

func lastPly(g bg.Game) bg.Ply { return g.Plies[len(g.Plies)-1] }
