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

// Event is one distinct stable-board observation, in match order. Dice, when
// non-zero, is the dice roll observed on the felt for this turn (from the
// dice-event detector + pip reading) with its confidence — a fusion cue, not
// a requirement.
type Event struct {
	Tick     int
	Part     int // which manifest Part the tick belongs to (multi-video matches)
	Obs      perceive.ObservedBoard
	Dice     bg.Dice
	DiceConf float64
}

// diceCue renders the event's observed dice as a fusion cue (nil if absent).
func (e Event) diceCue() *cue.Cue {
	if e.Dice == (bg.Dice{}) || e.DiceConf <= 0 {
		return nil
	}
	return &cue.Cue{Kind: cue.DiceValue, Tick: e.Tick, Dice: e.Dice, Confidence: e.DiceConf}
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
	// (once the game has clearly left the opening, see GameDiverge) closes the
	// game and opens the next. Tolerant threshold: systematic misreads keep
	// even a true start reading below a perfect score.
	NewGame float64
	// GameDiverge: the decided board must fall at least this far from the
	// standard start (WholeBoardMatch <= GameDiverge) at some point before a
	// return-to-start counts as a new game. No real game revisits the opening
	// a few plies in, so a near-start reading in the opening is a misread, not
	// a boundary — this gate blocks the spurious early reset the learned reader
	// can trip (rawvid regression: the pilot's one game split in two).
	GameDiverge float64
	// MinAccept: below this confidence an event is left unexplained (queued
	// for review) rather than applied — a bad reading must not corrupt the
	// decided-state chain.
	MinAccept float64
	// CommitPenalty scales a decision's confidence when NO clock press was
	// observed near the event (Commits non-empty): a turn normally ends
	// with a press, so a press-less event is a weaker claim. Soft — the
	// press detector covers ~77% of turns, so absence must not veto.
	CommitPenalty float64
	// CommitWindowMs bounds "near" for the press test.
	CommitWindowMs int
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
		Weights:        fusion.DefaultWeights(),
		Policy:         gate.Default(),
		ShiftSkip:      0.02,
		NewGame:        0.85,
		GameDiverge:    0.6,
		MinAccept:      0.2,
		DancePenalty:   0.7,
		CommitPenalty:  0.85,
		CommitWindowMs: 10000,
	}
}

// Outcome is a conducted transcription plus its review queue and counters.
type Outcome struct {
	Match       bg.Match
	Review      []pipeline.ReviewItem
	Skipped     int // events that re-read an unchanged board
	Unexplained int // events no hypothesis could explain acceptably
}

// RunEvents conducts the observed events into games and plies. commits,
// when non-empty, are clock-press ticks (the Commit cue): events without a
// press nearby take the CommitPenalty on their confidence.
func RunEvents(events []Event, o Options) Outcome {
	return RunEventsWithCommits(events, nil, o)
}

// RunEventsWithCommits is RunEvents plus the clock-press commit ticks.
func RunEventsWithCommits(events []Event, commits []int, o Options) Outcome {
	c := conductor{o: o, state: bg.StandardStart(), onRoll: -1, commits: commits}
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

	prevObs  perceive.ObservedBoard // last reading; deltas are read-to-read
	hasPrev  bool
	diverged bool  // decided state has left the opening this game (GameDiverge)
	commits  []int // clock-press ticks (sorted); empty = no commit cue
}

// nearCommit reports whether a clock press lies within the commit window of
// tick (presses trail the settled board by a few seconds).
func (c *conductor) nearCommit(tick int) bool {
	for _, p := range c.commits {
		if p >= tick-c.o.CommitWindowMs/2 && p <= tick+c.o.CommitWindowMs {
			return true
		}
	}
	return false
}

func (c *conductor) openGame(n int) {
	c.games = append(c.games, bg.Game{Number: n, StartScore: c.score})
	c.cur = &c.games[len(c.games)-1]
	c.state = bg.StandardStart()
	c.onRoll = -1
	c.hasPrev = false
	c.diverged = false
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

	// Once the decided state has clearly left the opening, a later return to
	// the start is a real game boundary. Tracked on the decided state (not the
	// raw reading) so a single noisy read can neither set nor clear it.
	if !c.diverged && boarddiff.WholeBoardMatch(bg.StandardStart(), obsExact(c.state)) <= c.o.GameDiverge {
		c.diverged = true
	}

	// Board reset to the standard start mid-game = next game. Tested
	// BEFORE the unchanged-table skip: after a reset the table stays
	// visually unchanged across stable windows, and when the first reset
	// read scored below the bar the skip swallowed every retest — one
	// noisy read cost the whole boundary (rawvid blind sweep). Gated on
	// c.diverged: no real game returns to the start before leaving the
	// opening, so a near-start reading in the opening is a misread, not a
	// boundary (rawvid regression: a spurious early reset split one game).
	if c.diverged && len(c.cur.Plies) >= 2 && boarddiff.WholeBoardMatch(bg.StandardStart(), ev.Obs) >= c.o.NewGame {
		c.closeGame()
		c.openGame(c.cur.Number + 1)
		c.hasPrev = false // next event diffs against the exact start
		return
	}
	// Re-read of an unchanged table?
	if c.hasPrev && boarddiff.ReadingShift(prev, ev.Obs) <= c.o.ShiftSkip {
		c.skipped++
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
			bg.Position{Board: c.state, PlayerOnRoll: who}, prev, ev.Obs, ev.Tick, c.o.Weights, ev.diceCue())
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
	conf := h.decision.Confidence
	if len(c.commits) > 0 && !c.nearCommit(ev.Tick) {
		conf *= c.o.CommitPenalty
		h.decision.Confidence = conf
	}
	ply := bg.Ply{
		Player:     h.who,
		Dice:       h.decision.Top.Dice,
		Notation:   h.decision.Top.Notation,
		Tick:       ev.Tick,
		Confidence: conf,
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
