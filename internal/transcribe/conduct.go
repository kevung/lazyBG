// Package transcribe turns a stream of observed stable-board events into a
// bg.Match — the conductor that walks a Recording turn by turn
// (architecture §3). It owns the game-level reasoning the per-turn cues
// cannot see: whose turn it is, dances that leave no board trace, game
// boundaries, and the decided-state chain (each accepted move's exact
// resulting board becomes the next pre-board, so perception noise does not
// accumulate).
package transcribe

import (
	"fmt"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
	"lazybg/internal/derive"
	"lazybg/internal/fusion"
	"lazybg/internal/gate"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/pipeline"
)

// Event is one distinct stable-board observation, in match order.
type Event struct {
	Tick int
	Obs  perceive.ObservedBoard
}

// Options tunes the conductor. Observations must already be in the canonical
// orientation (CheckerA = P1); the video runner applies the orientation prior
// before events reach the conductor.
type Options struct {
	Weights fusion.Weights
	Policy  gate.Policy

	// ShiftSkip: an event whose reading differs from the previous reading by
	// at most this (weighted fraction of points) is a re-read, not a turn.
	// Reading-to-reading comparison cancels stable per-point reader bias.
	ShiftSkip float64
	// NewGame: a mid-stream reading at least this close to the standard start
	// (once a couple of plies were played) closes the game and opens the next.
	// Tolerant threshold: systematic misreads keep even a true start reading
	// below a perfect score.
	NewGame float64
	// MinAccept: below this confidence an event is left unexplained (queued
	// for review) rather than applied — a bad reading must not corrupt the
	// decided-state chain.
	MinAccept float64
	// DancePenalty discounts the off-turn player's explanation when deciding
	// whether the on-roll player really was overtaken (opponent dance).
	DancePenalty float64

	MatchLength int
	Players     [2]string
}

// DefaultOptions returns the hand-set starting tuning (locked decision #4:
// interpretable first, calibrated against labeled corpus later).
func DefaultOptions() Options {
	return Options{
		Weights:      fusion.DefaultWeights(),
		Policy:       gate.Default(),
		ShiftSkip:    0.02,
		NewGame:      0.85,
		MinAccept:    0.2,
		DancePenalty: 0.7,
	}
}

// Outcome is a conducted transcription plus its review queue and counters.
type Outcome struct {
	Match       bg.Match
	Review      []pipeline.ReviewItem
	Skipped     int // events that re-read an unchanged board
	Unexplained int // events no hypothesis could explain acceptably
}

// RunEvents conducts the observed events into games and plies.
func RunEvents(events []Event, o Options) Outcome {
	c := conductor{o: o, state: bg.StandardStart(), onRoll: -1}
	c.openGame(1)
	for _, ev := range events {
		c.step(ev)
	}
	c.closeGame()
	return Outcome{
		Match: bg.Match{
			Length:  o.MatchLength,
			Players: o.Players,
			Games:   c.games,
		},
		Review:      c.review,
		Skipped:     c.skipped,
		Unexplained: c.unexplained,
	}
}

type conductor struct {
	o Options

	games       []bg.Game
	cur         *bg.Game
	state       bg.Board
	onRoll      bg.Player // -1 while unknown (game start)
	score       [2]int
	skipped     int
	unexplained int
	review      []pipeline.ReviewItem

	prevObs perceive.ObservedBoard // last reading; deltas are read-to-read
	hasPrev bool
}

func (c *conductor) openGame(n int) {
	c.games = append(c.games, bg.Game{Number: n, StartScore: c.score})
	c.cur = &c.games[len(c.games)-1]
	c.state = bg.StandardStart()
	c.onRoll = -1
	c.hasPrev = false
}

// closeGame records the result when the decided state shows a borne-off
// winner. Points are 1 until cube perception exists — the review queue and
// eval surface the difference honestly rather than guessing.
func (c *conductor) closeGame() {
	for _, who := range []bg.Player{bg.P1, bg.P2} {
		if c.state.Off[who] == 15 {
			c.cur.Result = &bg.GameResult{Winner: who, Points: 1}
			c.score[who]++
		}
	}
}

func (c *conductor) step(ev Event) {
	// The delta baseline: the previous reading, or an exact rendering of the
	// game-start state when no reading exists yet (start of a game).
	prev := c.prevObs
	if !c.hasPrev {
		prev = obsExact(c.state)
	}
	// Advance the baseline whatever happens below: every reading supersedes
	// the previous one as "what the table looked like last".
	defer func() { c.prevObs, c.hasPrev = ev.Obs, true }()

	// Re-read of an unchanged table?
	if c.hasPrev && boarddiff.ReadingShift(prev, ev.Obs) <= c.o.ShiftSkip {
		c.skipped++
		return
	}
	// Board reset to the standard start mid-game = next game.
	if len(c.cur.Plies) >= 2 && boarddiff.WholeBoardMatch(bg.StandardStart(), ev.Obs) >= c.o.NewGame {
		c.closeGame()
		c.openGame(c.cur.Number + 1)
		c.hasPrev = false // next event diffs against the exact start
		return
	}

	// Explain the transition: on-roll player first, opponent as the
	// dance-recovery hypothesis.
	type hypo struct {
		who      bg.Player
		decision cue.MoveDecision
		ranking  float64 // confidence after any off-turn discount
	}
	var hs []hypo
	try := func(who bg.Player, discount float64) {
		d, err := boarddiff.DecideAnyDice(
			bg.Position{Board: c.state, PlayerOnRoll: who}, prev, ev.Obs, ev.Tick, c.o.Weights)
		if err != nil || d.Top.Notation == "" {
			return
		}
		hs = append(hs, hypo{who, d, d.Confidence * discount})
	}
	if c.onRoll < 0 {
		try(bg.P1, 1)
		try(bg.P2, 1)
	} else {
		try(c.onRoll, 1)
		try(other(c.onRoll), c.o.DancePenalty)
	}
	bi := -1
	for i := range hs {
		if bi < 0 || hs[i].ranking > hs[bi].ranking {
			bi = i
		}
	}
	if bi < 0 || hs[bi].ranking < c.o.MinAccept {
		c.unexplained++
		c.review = append(c.review, pipeline.ReviewItem{
			Decision: cue.MoveDecision{Tick: ev.Tick},
			Reason:   fmt.Sprintf("event @%dms: no acceptable explanation", ev.Tick),
		})
		return
	}
	h := hs[bi]

	// Same player twice in a row: the opponent danced without a board trace.
	if c.onRoll >= 0 && h.who != c.onRoll {
		dance := bg.Ply{Player: c.onRoll, CannotMove: true, Tick: ev.Tick}
		c.cur.Plies = append(c.cur.Plies, dance)
		c.review = append(c.review, pipeline.ReviewItem{
			Decision: cue.MoveDecision{Player: c.onRoll, Tick: ev.Tick},
			Reason:   "inferred dance: dice unknown (board unchanged)",
		})
	}

	next, err := derive.ApplyNotation(c.state, h.who, h.decision.Top.Notation)
	if err != nil {
		// The engine produced it, so this should not happen; keep the chain
		// intact and surface it.
		c.unexplained++
		c.review = append(c.review, pipeline.ReviewItem{
			Decision: h.decision,
			Reason:   fmt.Sprintf("apply %q: %v", h.decision.Top.Notation, err),
		})
		return
	}
	ply := bg.Ply{
		Player:     h.who,
		Dice:       h.decision.Top.Dice,
		Notation:   h.decision.Top.Notation,
		Tick:       ev.Tick,
		Confidence: h.decision.Confidence,
	}
	if outcome, reason := c.o.Policy.Classify(h.decision); outcome == gate.NeedsReview {
		c.review = append(c.review, pipeline.ReviewItem{Decision: h.decision, Reason: reason})
	}
	c.cur.Plies = append(c.cur.Plies, ply)
	c.state = next
	c.onRoll = other(h.who)
}

// obsExact renders a board as a perfectly-confident observation — the delta
// baseline at a game start, before any real reading exists.
func obsExact(b bg.Board) perceive.ObservedBoard {
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

func other(p bg.Player) bg.Player {
	if p == bg.P1 {
		return bg.P2
	}
	return bg.P1
}
