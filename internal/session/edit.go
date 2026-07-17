// Retroactive editing + cascade re-validation (issue #20; functional-spec
// §5, ux-spec §4): any recorded turn can be edited or deleted; the board
// chain replays forward from the edit and every downstream turn whose
// recorded move is no longer legal on the recomputed board is demoted to a
// Review Item — nothing already entered is deleted or silently overwritten.
package session

import (
	"fmt"

	"lazybg/internal/bg"
	"lazybg/internal/derive"
	"lazybg/internal/engine"
)

// ReasonCascade marks a Review Item opened because an upstream edit made
// this turn's recorded move illegal on the recomputed board.
const ReasonCascade = "cascade"

// CandidatesFor re-opens the entry flow at a past turn: the ranked candidate
// list for the given dice on the board BEFORE that turn. Pure query — the
// session state is untouched.
func (s *Service) CandidatesFor(seq, d1, d2 int) ([]Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d1 < 1 || d1 > 6 || d2 < 1 || d2 > 6 {
		return nil, fmt.Errorf("dice out of range: %d-%d", d1, d2)
	}
	gi, pi, err := s.locate(seq)
	if err != nil {
		return nil, err
	}
	board, err := s.boardBeforeLocked(gi, pi)
	if err != nil {
		return nil, err
	}
	player := s.match.Games[gi].Plies[pi].Player
	moves, err := engine.LegalMoves(bg.Position{Board: board, Dice: bg.Dice{d1, d2}, PlayerOnRoll: player})
	if err != nil {
		return nil, err
	}
	ranked, _ := rankMoves(moves, nil)
	if len(ranked) > MaxCandidates {
		ranked = ranked[:MaxCandidates]
	}
	return candidateViews(ranked), nil
}

// ReplaceTurn edits the recorded turn at seq: new dice and/or notation
// (empty notation = Cannot Move). The move must be physically applicable on
// the board before the turn (engine legality is NOT required — ADR-0001);
// then the chain re-validates forward and flags what broke.
func (s *Service) ReplaceTurn(seq, d1, d2 int, notation string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d1 < 1 || d1 > 6 || d2 < 1 || d2 > 6 {
		return fmt.Errorf("dice out of range: %d-%d", d1, d2)
	}
	gi, pi, err := s.locate(seq)
	if err != nil {
		return err
	}
	ply := &s.match.Games[gi].Plies[pi]
	if ply.Cube != bg.NoCube {
		return fmt.Errorf("turn %d is a cube action — edit not supported for cube plies", seq)
	}
	if notation != "" {
		board, err := s.boardBeforeLocked(gi, pi)
		if err != nil {
			return err
		}
		if _, err := derive.ApplyNotation(board, ply.Player, notation); err != nil {
			return fmt.Errorf("cannot apply %q: %w", notation, err)
		}
	}
	ply.Dice = bg.Dice{d1, d2}
	ply.Notation = notation
	ply.CannotMove = notation == ""

	if s.doc != nil && seq < len(s.doc.Turns) {
		t := &s.doc.Turns[seq]
		t.Dice = [2]int{d1, d2}
		t.Notation = notation
		t.CannotMove = notation == ""
		t.Candidates = nil
		t.ChosenIndex = -1
		t.Cues = []string{"human-edit"}
	}
	return s.recomputeAndFlagLocked(gi, seq)
}

// DeleteTurn removes the recorded turn at seq entirely, then re-validates
// the chain the same way an edit does.
func (s *Service) DeleteTurn(seq int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	gi, pi, err := s.locate(seq)
	if err != nil {
		return err
	}
	g := &s.match.Games[gi]
	g.Plies = append(g.Plies[:pi], g.Plies[pi+1:]...)

	if s.doc != nil && seq < len(s.doc.Turns) {
		s.doc.Turns = append(s.doc.Turns[:seq], s.doc.Turns[seq+1:]...)
	}
	// Shift review references past the deleted turn; drop items on it.
	kept := s.reviews[:0]
	for _, r := range s.reviews {
		if r.TurnSeq == seq {
			continue
		}
		if r.TurnSeq > seq {
			r.TurnSeq--
		}
		kept = append(kept, r)
	}
	s.reviews = kept
	return s.recomputeAndFlagLocked(gi, seq)
}

// locate maps a flat seq to (game index, ply index).
func (s *Service) locate(seq int) (int, int, error) {
	if seq < 0 {
		return 0, 0, fmt.Errorf("seq %d out of range", seq)
	}
	i := 0
	for gi, g := range s.match.Games {
		if seq < i+len(g.Plies) {
			return gi, seq - i, nil
		}
		i += len(g.Plies)
	}
	return 0, 0, fmt.Errorf("seq %d out of range (have %d plies)", seq, i)
}

// boardBeforeLocked replays the game up to (not including) ply pi.
func (s *Service) boardBeforeLocked(gi, pi int) (bg.Board, error) {
	board := bg.StandardStart()
	for k := 0; k < pi; k++ {
		ply := s.match.Games[gi].Plies[k]
		if ply.Notation == "" || ply.CannotMove {
			continue
		}
		next, err := derive.ApplyNotation(board, ply.Player, ply.Notation)
		if err != nil {
			// A previously-flagged unapplyable ply: skip its board effect.
			continue
		}
		board = next
	}
	return board, nil
}

// recomputeAndFlagLocked replays the edited game forward from the turn at
// flat seq `fromSeq`, re-validating each later ply in that game against the
// recomputed board: still-legal plies are left untouched; ones no longer
// legal open a cascade Review Item (once). Physically applicable moves still
// advance the chain even when flagged — the pixels saw them happen; the
// human arbitrates via the review queue. The live board and cube state are
// rebuilt, and everything autosaves.
func (s *Service) recomputeAndFlagLocked(gi, fromSeq int) error {
	// Base flat seq of this game.
	base := 0
	for k := 0; k < gi; k++ {
		base += len(s.match.Games[k].Plies)
	}

	g := &s.match.Games[gi]
	board := bg.StandardStart()
	cube := cubeState{value: 1}
	for k := 0; k < len(g.Plies); k++ {
		ply := g.Plies[k]
		seq := base + k
		switch {
		case ply.Cube != bg.NoCube:
			s.applyCubeReplayState(&cube, ply)
		case ply.CannotMove:
			if seq > fromSeq && s.hasLegalMoves(board, ply) {
				s.flagCascadeLocked(seq)
			}
		case ply.Notation != "":
			legalResult, legal := s.legalResult(board, ply)
			if legal {
				board = legalResult
				continue
			}
			if seq > fromSeq {
				s.flagCascadeLocked(seq)
			}
			if next, err := derive.ApplyNotation(board, ply.Player, ply.Notation); err == nil {
				board = next // physically possible — keep the chain moving
			}
		}
	}

	// Only rebuild the live state if we edited the current (last) game.
	if gi == len(s.match.Games)-1 {
		s.board = board
		s.cube = cube
		if n := len(g.Plies); n > 0 {
			s.onRoll = otherPlayer(g.Plies[n-1].Player)
		}
		s.pending = nil
		s.obs = nil
	}
	if err := s.save(); err != nil {
		return fmt.Errorf("autosave: %w", err)
	}
	return nil
}

// legalResult reports whether the ply's notation is engine-legal for its
// dice on the board, and if so the resulting position (transposition-safe
// via canonical ply identity).
func (s *Service) legalResult(board bg.Board, ply bg.Ply) (bg.Board, bool) {
	want, err := derive.CanonicalPlays(ply.Notation)
	if err != nil {
		return bg.Board{}, false
	}
	moves, err := engine.LegalMovesUnscored(bg.Position{Board: board, Dice: ply.Dice, PlayerOnRoll: ply.Player})
	if err != nil {
		return bg.Board{}, false
	}
	for _, mv := range moves {
		got, err := derive.CanonicalPlays(mv.Notation)
		if err == nil && got == want {
			return mv.Result, true
		}
	}
	return bg.Board{}, false
}

func (s *Service) hasLegalMoves(board bg.Board, ply bg.Ply) bool {
	moves, err := engine.LegalMovesUnscored(bg.Position{Board: board, Dice: ply.Dice, PlayerOnRoll: ply.Player})
	return err == nil && len(moves) > 0
}

// flagCascadeLocked opens a cascade Review Item on seq unless one is already
// open there.
func (s *Service) flagCascadeLocked(seq int) {
	for _, r := range s.reviews {
		if r.TurnSeq == seq && r.Reason == ReasonCascade && !r.Resolved {
			return
		}
	}
	s.reviews = append(s.reviews, LBGReview{TurnSeq: seq, Reason: ReasonCascade})
}

// applyCubeReplayState mirrors applyCubeReplay onto an explicit state.
func (s *Service) applyCubeReplayState(c *cubeState, ply bg.Ply) {
	if c.value == 0 {
		c.value = 1
	}
	switch ply.Cube {
	case bg.Double:
		c.pending = true
	case bg.Take:
		c.value *= 2
		c.owner = ply.Player
		c.owned = true
		c.pending = false
	case bg.Drop:
		c.pending = false
	}
}
