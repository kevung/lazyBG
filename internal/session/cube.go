// Cube-decision entry (issue #18; functional-spec §4, ux-spec §9): the small
// fixed action set — no move generation, no ranking. Which actions are legal
// falls out of the cube state: centered → the player on roll may double;
// double pending → the opponent takes or drops; owned elsewhere → nothing.
package session

import (
	"fmt"

	"lazybg/internal/bg"
)

// cubeState tracks the live doubling cube.
type cubeState struct {
	value   int       // 1 (centered) then 2, 4, ...
	owner   bg.Player // meaningful only when owned
	owned   bool      // false = centered
	pending bool      // a double awaits take/drop; dice entry is blocked
}

// CubeActions returns the actions available to the player on roll, in the
// menu's display order.
func (s *Service) CubeActions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cube.pending {
		return []string{"take", "drop"}
	}
	if !s.cube.owned || s.cube.owner == s.onRoll {
		return []string{"double"}
	}
	return nil
}

// CubeValue returns the current cube value (1 = centered, never doubled).
func (s *Service) CubeValue() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cube.value == 0 {
		return 1
	}
	return s.cube.value
}

// EnterCube records a cube action for the player on roll at the video tick.
// After a double the decision passes to the opponent; after a take the
// doubler is back on roll. A drop ends the game — the boundary handling
// (GameResult) is issue #19's; the drop ply itself is recorded here.
func (s *Service) EnterCube(action string, tickMs int) (PlyView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cube.value == 0 {
		s.cube.value = 1
	}
	if s.detectGameEndLocked() != nil {
		return PlyView{}, fmt.Errorf("the game is over — confirm the result first")
	}
	ply := bg.Ply{Player: s.onRoll, Tick: tickMs}
	switch action {
	case "double":
		if s.cube.pending {
			return PlyView{}, fmt.Errorf("a double is already pending — take or drop first")
		}
		if s.cube.owned && s.cube.owner != s.onRoll {
			return PlyView{}, fmt.Errorf("the cube is owned by the opponent")
		}
		ply.Cube = bg.Double
		ply.CubeValue = s.cube.value * 2
		s.cube.pending = true
	case "take":
		if !s.cube.pending {
			return PlyView{}, fmt.Errorf("no double to take")
		}
		ply.Cube = bg.Take
		s.cube.value *= 2
		s.cube.owner = s.onRoll
		s.cube.owned = true
		s.cube.pending = false
	case "drop":
		if !s.cube.pending {
			return PlyView{}, fmt.Errorf("no double to drop")
		}
		ply.Cube = bg.Drop
		s.cube.pending = false
	default:
		return PlyView{}, fmt.Errorf("unknown cube action %q (double/take/drop)", action)
	}

	s.pending = nil // a cube action supersedes any half-entered dice
	g := &s.match.Games[len(s.match.Games)-1]
	g.Plies = append(g.Plies, ply)
	s.onRoll = otherPlayer(ply.Player)

	if s.doc != nil {
		s.doc.Turns = append(s.doc.Turns, LBGTurn{
			Game:        g.Number,
			Player:      int(ply.Player),
			Cube:        action,
			CubeValue:   ply.CubeValue,
			Part:        0,
			TickMs:      tickMs,
			ChosenIndex: -1,
		})
		s.doc.LastTickMs = tickMs
		if err := s.save(); err != nil {
			return PlyView{}, fmt.Errorf("autosave: %w", err)
		}
	}
	return plyView(len(g.Plies)-1, g.Number, ply), nil
}

// applyCubeReplay reconstructs the live cube state from a recorded action
// (the .lbg Open replay path).
func (s *Service) applyCubeReplay(action string, player bg.Player) {
	if s.cube.value == 0 {
		s.cube.value = 1
	}
	switch action {
	case "double":
		s.cube.pending = true
	case "take":
		s.cube.value *= 2
		s.cube.owner = player
		s.cube.owned = true
		s.cube.pending = false
	case "drop":
		s.cube.pending = false
	}
}
