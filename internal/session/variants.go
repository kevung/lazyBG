// Confirm variants (issue #17; functional-spec §4, ux-spec §2, ADR-0001):
// the plain confirm's two siblings — flag-as-uncertain and free-entry
// override — plus the automatic dance. All three keep the human unblocked.
package session

import (
	"fmt"

	"lazybg/internal/bg"
	"lazybg/internal/derive"
)

// ReasonHumanFlagged marks a Review Item the transcriber opened on their own
// freshly-entered turn (bad footage, occlusion) — applied AND queued.
const ReasonHumanFlagged = "human-flagged"

// ReviewItemView is one open review-queue entry.
type ReviewItemView struct {
	ID       int    `json:"id"` // index into the session's review list
	TurnSeq  int    `json:"turnSeq"`
	Reason   string `json:"reason"`
	TickMs   int    `json:"tickMs"`
	Player   int    `json:"player"`
	Dice     string `json:"dice"`
	Notation string `json:"notation"`
}

// DiceResult is what dice entry yields: either a ranked candidate list, or —
// when the position is a dance — the already-recorded Cannot Move ply.
type DiceResult struct {
	Candidates []Candidate `json:"candidates"`
	Danced     bool        `json:"danced"`
	Ply        *PlyView    `json:"ply,omitempty"`
}

// EnterDiceAt is EnterDice plus the automatic dance: if the roll yields zero
// legal moves the turn records itself as Cannot Move at tickMs — no candidate
// step, nothing to choose (functional-spec §4).
func (s *Service) EnterDiceAt(d1, d2, tickMs int) (DiceResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cands, err := s.enterDiceLocked(d1, d2)
	if err != nil {
		return DiceResult{}, err
	}
	if len(cands) > 0 {
		return DiceResult{Candidates: cands}, nil
	}
	ply, err := s.applyPlyLocked(bg.Ply{
		Player:     s.pending.player,
		Dice:       s.pending.dice,
		CannotMove: true,
		Tick:       tickMs,
	}, s.board, -1, tickMs)
	if err != nil {
		return DiceResult{}, err
	}
	return DiceResult{Danced: true, Ply: &ply}, nil
}

// ConfirmFlag is Confirm plus a human-flagged Review Item: the ply is applied
// immediately (the human committed; the video keeps moving) and a review
// entry opens alongside it for an unhurried second pass.
func (s *Service) ConfirmFlag(index, tickMs int) (PlyView, error) {
	ply, err := s.Confirm(index, tickMs)
	if err != nil {
		return PlyView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openReviewLocked(s.turnCount()-1, ReasonHumanFlagged)
	if err := s.save(); err != nil {
		return PlyView{}, fmt.Errorf("autosave: %w", err)
	}
	return ply, nil
}

// Override records what the human witnessed, bypassing the candidate list.
// Engine legality is NOT checked (ADR-0001) — only physical possibility is
// (a hop from an empty point cannot produce a board). Empty notation records
// a Cannot Move. Dice must have been entered first.
func (s *Service) Override(notation string, tickMs int) (PlyView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.pending
	if p == nil {
		return PlyView{}, fmt.Errorf("no pending dice — enter the roll first")
	}
	ply := bg.Ply{Player: p.player, Dice: p.dice, Tick: tickMs}
	board := s.board
	if notation == "" {
		ply.CannotMove = true
	} else {
		next, err := derive.ApplyNotation(s.board, p.player, notation)
		if err != nil {
			return PlyView{}, fmt.Errorf("cannot apply %q: %w", notation, err)
		}
		ply.Notation = notation
		board = next
	}
	return s.applyPlyLocked(ply, board, -1, tickMs)
}

// applyPlyLocked commits a ply (from any entry path), advances the chain,
// persists, and clears the pending turn. chosenIndex is -1 for override/dance.
func (s *Service) applyPlyLocked(ply bg.Ply, nextBoard bg.Board, chosenIndex, tickMs int) (PlyView, error) {
	p := s.pending
	g := &s.match.Games[len(s.match.Games)-1]
	g.Plies = append(g.Plies, ply)
	s.board = nextBoard
	s.onRoll = otherPlayer(ply.Player)
	s.pending = nil
	s.obs = nil

	if s.doc != nil {
		var cands []LBGCandidate
		var cues []string
		if p != nil {
			cands = make([]LBGCandidate, len(p.cands))
			for i, c := range p.cands {
				cands[i] = LBGCandidate{Notation: c.mv.Notation, Equity: c.mv.Equity, Score: c.score}
			}
			cues = p.cues
		}
		s.doc.Turns = append(s.doc.Turns, LBGTurn{
			Game:        g.Number,
			Player:      int(ply.Player),
			Dice:        [2]int{ply.Dice[0], ply.Dice[1]},
			Notation:    ply.Notation,
			CannotMove:  ply.CannotMove,
			Part:        0,
			TickMs:      tickMs,
			Candidates:  cands,
			ChosenIndex: chosenIndex,
			Cues:        cues,
		})
		s.doc.LastTickMs = tickMs
		if err := s.save(); err != nil {
			return PlyView{}, fmt.Errorf("autosave: %w", err)
		}
	}
	return plyView(len(g.Plies)-1, g.Number, ply), nil
}

// openReviewLocked opens a review item on the turn at seq (index across the
// whole move list).
func (s *Service) openReviewLocked(turnSeq int, reason string) {
	s.reviews = append(s.reviews, LBGReview{TurnSeq: turnSeq, Reason: reason})
}

// ReviewItems returns the open (unresolved) review-queue entries.
func (s *Service) ReviewItems() []ReviewItemView {
	s.mu.Lock()
	defer s.mu.Unlock()
	moves := s.movesLocked()
	var out []ReviewItemView
	for i, r := range s.reviews {
		if r.Resolved {
			continue
		}
		v := ReviewItemView{ID: i, TurnSeq: r.TurnSeq, Reason: r.Reason}
		if r.TurnSeq >= 0 && r.TurnSeq < len(moves) {
			m := moves[r.TurnSeq]
			v.TickMs = m.TickMs
			v.Player = m.Player
			v.Dice = m.Dice
			v.Notation = m.Notation
		}
		out = append(out, v)
	}
	return out
}

// turnCount is the total recorded plies across games (callers hold no lock —
// it takes none; only use where racing is impossible or the lock is held...
// callers in this file hold the pattern: Confirm released the lock first).
func (s *Service) turnCount() int {
	n := 0
	for _, g := range s.match.Games {
		n += len(g.Plies)
	}
	return n
}
