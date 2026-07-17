// BoardAt: prefix replay of the recorded chain (issue #16's board panel;
// issue #20's cascade re-validation builds on it).
package session

import (
	"fmt"

	"lazybg/internal/bg"
	"lazybg/internal/derive"
)

// BoardAt returns the reconstructed board immediately after the ply at seq
// (0-based across the whole move list); -1 returns the starting position.
func (s *Service) BoardAt(seq int) (bg.Board, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if seq < -1 {
		return bg.Board{}, fmt.Errorf("seq %d out of range", seq)
	}
	board := bg.StandardStart()
	if seq == -1 {
		return board, nil
	}
	i := 0
	for _, g := range s.match.Games {
		board = bg.StandardStart() // each game opens fresh
		for _, ply := range g.Plies {
			if ply.Notation != "" && !ply.CannotMove {
				next, err := derive.ApplyNotation(board, ply.Player, ply.Notation)
				if err != nil {
					return bg.Board{}, fmt.Errorf("replay seq %d (%s): %w", i, ply.Notation, err)
				}
				board = next
			}
			if i == seq {
				return board, nil
			}
			i++
		}
	}
	return bg.Board{}, fmt.Errorf("seq %d out of range (have %d plies)", seq, i)
}
