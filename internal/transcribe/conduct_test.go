package transcribe

import (
	"os"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/engine"
	"lazybg/internal/perceive"
)

func TestMain(m *testing.M) {
	if err := engine.Init(os.DirFS("../..")); err != nil {
		panic("engine init: " + err.Error())
	}
	os.Exit(m.Run())
}

// obsFrom fabricates a clean, fully-confident observation of a board.
func obsFrom(b bg.Board) perceive.ObservedBoard {
	var ob perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		c := b.Pts[p]
		if c.N == 0 {
			ob.Points[p] = perceive.PointObs{Confidence: 1}
			continue
		}
		side := perceive.A
		if c.Owner == bg.P2 {
			side = perceive.B
		}
		ob.Points[p] = perceive.PointObs{Count: c.N, Side: side, Confidence: 1}
	}
	return ob
}

// best applies the engine's best move for the roll and returns the new board.
func best(t *testing.T, b bg.Board, who bg.Player, d bg.Dice) bg.Board {
	t.Helper()
	moves, err := engine.LegalMoves(bg.Position{Board: b, Dice: d, PlayerOnRoll: who})
	if err != nil {
		t.Fatal(err)
	}
	if len(moves) == 0 {
		t.Fatalf("no legal move for %v %v", who, d)
	}
	return moves[0].Result
}

// script plays a fixed opening sequence with the engine's best moves and
// returns the board after each ply.
func script(t *testing.T) (boards []bg.Board, players []bg.Player, dice []bg.Dice) {
	t.Helper()
	rolls := []struct {
		who bg.Player
		d   bg.Dice
	}{
		{bg.P1, bg.Dice{3, 1}},
		{bg.P2, bg.Dice{4, 2}},
		{bg.P1, bg.Dice{6, 1}},
		{bg.P2, bg.Dice{5, 3}},
		{bg.P1, bg.Dice{4, 3}},
		{bg.P2, bg.Dice{2, 1}},
	}
	b := bg.StandardStart()
	for _, r := range rolls {
		b = best(t, b, r.who, r.d)
		boards = append(boards, b)
		players = append(players, r.who)
		dice = append(dice, r.d)
	}
	return
}

func events(boards []bg.Board) []Event {
	var evs []Event
	for i, b := range boards {
		evs = append(evs, Event{Tick: (i + 1) * 10000, Obs: obsFrom(b)})
	}
	return evs
}

// A clean alternating game must be reproduced ply for ply: right players,
// right dice (the rolls are inferred, never observed), right notations.
func TestRunEvents_CleanSequence(t *testing.T) {
	boards, players, dice := script(t)
	out := RunEvents(events(boards), DefaultOptions())

	if len(out.Match.Games) != 1 {
		t.Fatalf("games = %d, want 1", len(out.Match.Games))
	}
	plies := out.Match.Games[0].Plies
	if len(plies) != len(boards) {
		t.Fatalf("plies = %d, want %d", len(plies), len(boards))
	}
	for i, p := range plies {
		if p.Player != players[i] {
			t.Errorf("ply %d: player %v, want %v", i, p.Player, players[i])
		}
		if !sameDice(p.Dice, dice[i]) {
			t.Errorf("ply %d: dice %v, want %v", i, p.Dice, dice[i])
		}
		if p.Tick != (i+1)*10000 {
			t.Errorf("ply %d: tick %d, want %d", i, p.Tick, (i+1)*10000)
		}
	}
}

// Repeated observations of the same position are no-ops, not phantom plies.
func TestRunEvents_SkipsNoChange(t *testing.T) {
	boards, _, _ := script(t)
	var evs []Event
	for i, b := range boards {
		evs = append(evs, Event{Tick: (i + 1) * 10000, Obs: obsFrom(b)})
		evs = append(evs, Event{Tick: (i+1)*10000 + 3000, Obs: obsFrom(b)}) // re-read
	}
	out := RunEvents(evs, DefaultOptions())
	if got := len(out.Match.Games[0].Plies); got != len(boards) {
		t.Fatalf("plies = %d, want %d (duplicates must be skipped)", got, len(boards))
	}
	if out.Skipped < len(boards) {
		t.Errorf("skipped = %d, want >= %d", out.Skipped, len(boards))
	}
}

// When the same player is the best explanation twice in a row, the opponent
// danced in between: a CannotMove ply must be inserted and queued for review
// (its dice were never seen and cannot be inferred from an unchanged board).
func TestRunEvents_InsertsDanceOnParityBreak(t *testing.T) {
	b1 := best(t, bg.StandardStart(), bg.P1, bg.Dice{3, 1})
	b2 := best(t, b1, bg.P1, bg.Dice{6, 1}) // P1 again: P2 danced between
	out := RunEvents([]Event{
		{Tick: 10000, Obs: obsFrom(b1)},
		{Tick: 20000, Obs: obsFrom(b2)},
	}, DefaultOptions())

	plies := out.Match.Games[0].Plies
	if len(plies) != 3 {
		t.Fatalf("plies = %d, want 3 (move, inserted dance, move): %+v", len(plies), plies)
	}
	if plies[1].Player != bg.P2 || !plies[1].CannotMove {
		t.Errorf("ply 1 = %+v, want P2 CannotMove", plies[1])
	}
	if len(out.Review) == 0 {
		t.Error("an inserted dance must be queued for review")
	}
}

// A board back at the standard start mid-stream is a new game.
func TestRunEvents_NewGameBoundary(t *testing.T) {
	b1 := best(t, bg.StandardStart(), bg.P1, bg.Dice{3, 1})
	b2 := best(t, b1, bg.P2, bg.Dice{4, 2})
	g2first := best(t, bg.StandardStart(), bg.P2, bg.Dice{5, 3})
	out := RunEvents([]Event{
		{Tick: 10000, Obs: obsFrom(b1)},
		{Tick: 20000, Obs: obsFrom(b2)},
		{Tick: 30000, Obs: obsFrom(bg.StandardStart())},
		{Tick: 40000, Obs: obsFrom(g2first)},
	}, DefaultOptions())

	if len(out.Match.Games) != 2 {
		t.Fatalf("games = %d, want 2", len(out.Match.Games))
	}
	if n := len(out.Match.Games[0].Plies); n != 2 {
		t.Errorf("game 1 plies = %d, want 2", n)
	}
	g2 := out.Match.Games[1].Plies
	if len(g2) != 1 || g2[0].Player != bg.P2 {
		t.Errorf("game 2 = %+v, want one P2 ply", g2)
	}
}

// A corrupted point reading must not derail the inference: legality + the
// clean majority of points still pick the right move.
func TestRunEvents_NoisyPointStillRecovers(t *testing.T) {
	boards, players, dice := script(t)
	evs := events(boards)
	// Corrupt one uninvolved point on the third event: misread 12 as empty.
	ob := evs[2].Obs
	ob.Points[12] = perceive.PointObs{Count: 0, Side: perceive.None, Confidence: 0.3}
	evs[2].Obs = ob

	out := RunEvents(evs, DefaultOptions())
	plies := out.Match.Games[0].Plies
	if len(plies) != len(boards) {
		t.Fatalf("plies = %d, want %d", len(plies), len(boards))
	}
	for i := range plies {
		if plies[i].Player != players[i] || !sameDice(plies[i].Dice, dice[i]) {
			t.Errorf("ply %d: %v %v, want %v %v", i, plies[i].Player, plies[i].Dice, players[i], dice[i])
		}
	}
}

func sameDice(a, b bg.Dice) bool {
	return (a[0] == b[0] && a[1] == b[1]) || (a[0] == b[1] && a[1] == b[0])
}

// Observed dice on the events must flow into the decisions: same clean
// sequence, but with dice observations attached — the plies keep the right
// moves and gain confidence over the blind run.
func TestRunEvents_ObservedDiceBoostConfidence(t *testing.T) {
	boards, _, dice := script(t)
	blind := events(boards)
	hinted := events(boards)
	for i := range hinted {
		hinted[i].Dice = dice[i]
		hinted[i].DiceConf = 0.9
	}

	outBlind := RunEvents(blind, DefaultOptions())
	outHinted := RunEvents(hinted, DefaultOptions())

	pb := outBlind.Match.Games[0].Plies
	ph := outHinted.Match.Games[0].Plies
	if len(ph) != len(pb) {
		t.Fatalf("hinted plies %d != blind %d", len(ph), len(pb))
	}
	better := 0
	for i := range ph {
		if !sameDice(ph[i].Dice, dice[i]) {
			t.Errorf("ply %d: hinted dice %v, want %v", i, ph[i].Dice, dice[i])
		}
		if ph[i].Confidence > pb[i].Confidence {
			better++
		}
	}
	if better < len(ph)/2 {
		t.Errorf("only %d/%d plies gained confidence from dice observations", better, len(ph))
	}
}
