// Command lazybg is, for now, an end-to-end demo of the transcription spine
// driven by the real gnubg engine. For each turn it simulates a strong player
// playing the engine's top move, "observes" the resulting board (standing in for
// the not-yet-built video front-end), then recovers the move via boarddiff +
// fusion and gates it — finally exporting a Jellyfish .mat.
//
// The real app (video scrubber ↔ move list ↔ review queue) arrives at the UI
// milestone (docs/architecture.md §3, §8).
package main

import (
	"fmt"
	"log"
	"os"

	"lazybg"
	"lazybg/internal/bg"
	"lazybg/internal/cue"
	"lazybg/internal/engine"
	"lazybg/internal/fusion"
	"lazybg/internal/gate"
	"lazybg/internal/matexport"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boarddiff"
)

func main() {
	if err := engine.Init(lazybg.DataFS); err != nil {
		log.Fatalf("engine init: %v", err)
	}

	// A fixed sequence of rolls; the engine chooses each move.
	rolls := []struct {
		who  bg.Player
		dice bg.Dice
	}{
		{bg.P1, bg.Dice{3, 1}},
		{bg.P2, bg.Dice{6, 5}},
		{bg.P1, bg.Dice{5, 4}},
		{bg.P2, bg.Dice{3, 2}},
	}

	board := bg.StandardStart()
	policy := gate.Default()
	var plies []bg.Ply
	tick := 0

	for _, r := range rolls {
		tick += 1000
		ply, next, dec, err := playTurn(board, r.dice, r.who, tick)
		if err != nil {
			log.Fatalf("turn (%v %v): %v", r.who, r.dice, err)
		}
		outcome, _ := policy.Classify(dec)
		fmt.Fprintf(os.Stderr, "tick %5d  %-5s rolls %s → %-14s conf=%.2f  [%s]\n",
			tick, playerName(r.who), r.dice, dec.Top.Notation, dec.Confidence, outcome)
		plies = append(plies, ply)
		board = next
	}

	m := bg.Match{
		Length:  3,
		Players: [2]string{"Alice", "Bob"},
		Meta: []bg.MetaField{
			{Key: "Site", Value: "lazyBG engine demo"},
			{Key: "Player 1", Value: "Alice"},
			{Key: "Player 2", Value: "Bob"},
		},
		Games: []bg.Game{{Number: 1, Plies: plies}},
	}
	fmt.Fprint(os.Stdout, matexport.Write(m))
}

// playTurn asks the engine for the best move, "observes" its result (simulating
// perception), then recovers the move via boarddiff + fusion — exercising the
// real pipeline end-to-end. Returns the auto-fill ply, the resulting board, and
// the decision.
func playTurn(board bg.Board, dice bg.Dice, who bg.Player, tick int) (bg.Ply, bg.Board, cue.MoveDecision, error) {
	pre := bg.Position{Board: board, Dice: dice, PlayerOnRoll: who}
	moves, err := engine.LegalMoves(pre)
	if err != nil {
		return bg.Ply{}, board, cue.MoveDecision{}, err
	}
	if len(moves) == 0 { // dance
		ply := bg.Ply{Player: who, Dice: dice, CannotMove: true, Tick: tick}
		return ply, board, cue.MoveDecision{Player: who, Tick: tick}, nil
	}

	played := moves[0]                 // a strong player plays the engine's best move
	observed := observe(played.Result) // stand-in for the board-state reader
	dec, err := boarddiff.Decide(pre, observed, &dice, tick, fusion.DefaultWeights())
	if err != nil {
		return bg.Ply{}, board, cue.MoveDecision{}, err
	}
	ply := bg.Ply{
		Player:     who,
		Dice:       dice,
		Notation:   dec.Top.Notation,
		Tick:       tick,
		Confidence: dec.Confidence,
	}
	return ply, played.Result, dec, nil
}

// observe fabricates a clean, fully-confident reading of a board (stand-in for
// the board-state reader).
func observe(b bg.Board) perceive.ObservedBoard {
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

func playerName(p bg.Player) string {
	if p == bg.P1 {
		return "Alice"
	}
	return "Bob"
}
