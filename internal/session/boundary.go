// Game & match boundaries (issue #19; functional-spec §5b): the app
// recognizes game end from the reconstructed board (all 15 checkers off, or
// a recorded drop) and pre-fills the result — winner, gammon/backgammon —
// for the human to confirm or correct. Match end follows the same principle:
// detected from the running score, offered, never forced.
package session

import (
	"fmt"

	"lazybg/internal/bg"
)

// GameEndProposal is the pre-filled result awaiting human confirmation.
type GameEndProposal struct {
	Winner     int    `json:"winner"`
	Points     int    `json:"points"` // already cube- and gammon-scaled
	Gammon     bool   `json:"gammon"`
	Backgammon bool   `json:"backgammon"`
	Reason     string `json:"reason"` // "bearoff" | "drop"
}

// GameEndResult is what confirming a boundary yields.
type GameEndResult struct {
	Score     [2]int `json:"score"`
	GameCount int    `json:"gameCount"`
	MatchOver bool   `json:"matchOver"`
}

// PendingGameEnd returns the detected (unconfirmed) game end, or nil.
func (s *Service) PendingGameEnd() *GameEndProposal {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.detectGameEndLocked()
}

func (s *Service) detectGameEndLocked() *GameEndProposal {
	g := s.match.Games[len(s.match.Games)-1]
	if g.Result != nil {
		return nil
	}
	cube := s.cube.value
	if cube == 0 {
		cube = 1
	}

	// A recorded drop ends the game for the doubler at the pre-double stake.
	if n := len(g.Plies); n > 0 && g.Plies[n-1].Cube == bg.Drop {
		return &GameEndProposal{
			Winner: int(otherPlayer(g.Plies[n-1].Player)),
			Points: cube,
			Reason: "drop",
		}
	}

	for _, w := range []bg.Player{bg.P1, bg.P2} {
		if s.board.Off[w] != 15 {
			continue
		}
		l := otherPlayer(w)
		gammon := s.board.Off[l] == 0
		backgammon := gammon && loserTrapped(s.board, w, l)
		mult := 1
		if backgammon {
			mult = 3
		} else if gammon {
			mult = 2
		}
		return &GameEndProposal{
			Winner:     int(w),
			Points:     cube * mult,
			Gammon:     gammon,
			Backgammon: backgammon,
			Reason:     "bearoff",
		}
	}
	return nil
}

// loserTrapped reports whether the loser still has a checker on the bar or in
// the winner's home board (the backgammon condition). P1 bears off toward
// point 1, so P1's home is 1..6; P2's is 19..24.
func loserTrapped(b bg.Board, winner, loser bg.Player) bool {
	if b.Bar[loser] > 0 {
		return true
	}
	lo, hi := 1, 6
	if winner == bg.P2 {
		lo, hi = 19, 24
	}
	for p := lo; p <= hi; p++ {
		if b.Pts[p].N > 0 && b.Pts[p].Owner == loser {
			return true
		}
	}
	return false
}

// ConfirmGameEnd closes the current game with the (possibly human-corrected)
// winner and points, banks the score, and opens the next game on a fresh
// board with a recentred cube. It reports match end when the running score
// reaches the match length (never forced — the caller decides what to do).
func (s *Service) ConfirmGameEnd(winner, points int) (GameEndResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if winner != 0 && winner != 1 {
		return GameEndResult{}, fmt.Errorf("winner must be 0 or 1")
	}
	if points < 1 {
		return GameEndResult{}, fmt.Errorf("points must be >= 1")
	}
	g := &s.match.Games[len(s.match.Games)-1]
	if g.Result != nil {
		return GameEndResult{}, fmt.Errorf("game %d already has a result", g.Number)
	}
	g.Result = &bg.GameResult{Winner: bg.Player(winner), Points: points}

	score := g.StartScore
	score[winner] += points

	matchOver := s.match.Length > 0 && (score[0] >= s.match.Length || score[1] >= s.match.Length)
	if !matchOver {
		s.match.Games = append(s.match.Games, bg.Game{
			Number:     g.Number + 1,
			StartScore: score,
		})
		s.board = bg.StandardStart()
		s.cube = cubeState{value: 1}
		s.onRoll = bg.P1 // the human toggles if the other player opens
		s.pending = nil
		s.obs = nil
	}

	if s.doc != nil {
		s.doc.Results = append(s.doc.Results, LBGResult{
			Game:   g.Number,
			Winner: winner,
			Points: points,
		})
		if err := s.save(); err != nil {
			return GameEndResult{}, fmt.Errorf("autosave: %w", err)
		}
	}
	return GameEndResult{Score: score, GameCount: len(s.match.Games), MatchOver: matchOver}, nil
}

// Score returns the running match score (after every closed game).
func (s *Service) Score() [2]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.match.Games[len(s.match.Games)-1]
	score := g.StartScore
	if g.Result != nil {
		score[g.Result.Winner] += g.Result.Points
	}
	return score
}
