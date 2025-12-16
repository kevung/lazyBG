package main

import (
	"fmt"
	"lazybg/gnubg"
	"strconv"
)

// CandidateMove represents a move suggested by the gnubg engine
type CandidateMove struct {
	Move string `json:"move"` // Move notation in lazyBG format (e.g., "24/20 13/8")
}

// GetCandidateMoves generates candidate moves for the given position and dice
func (a *App) GetCandidateMoves(position Position) ([]CandidateMove, error) {
	// Check if dice are defined
	if position.Dice[0] == 0 && position.Dice[1] == 0 {
		return []CandidateMove{}, nil
	}

	// Convert lazyBG board to gnubg TanBoard format
	gnubgBoard := positionToGnubgBoard(position)

	// Call gnubg engine to find moves
	// scoreMoves=true to get evaluations, cubeful=false for faster analysis
	pml, err := gnubg.FindMoves(gnubgBoard, position.Dice, position.PlayerOnRoll, true, false)
	if err != nil {
		return nil, fmt.Errorf("error calling gnubg.FindMoves: %v", err)
	}

	// Convert up to 10 best moves to CandidateMove format
	// gnubg returns moves sorted by equity (best first)
	movesNum := pml.GetMovesNum()
	if movesNum > 10 {
		movesNum = 10
	}

	candidates := make([]CandidateMove, 0, movesNum)

	for i := 0; i < movesNum; i++ {
		move := pml.GetMove(i)

		// Convert gnubg move notation to lazyBG notation
		moveStr := gnubgMoveToLazyBG(move, position.PlayerOnRoll)

		candidates = append(candidates, CandidateMove{
			Move: moveStr,
		})
	}

	return candidates, nil
}

// positionToGnubgBoard converts a lazyBG Position to gnubg TanBoard format
func positionToGnubgBoard(position Position) gnubg.TanBoard {
	var board gnubg.TanBoard

	// Convert board points
	// lazyBG: points[0] is white bar, points[1-24] are board points, points[25] is black bar
	// gnubg: TanBoard[0] is always Black's checkers, TanBoard[1] is always White's checkers
	//        gnubg will internally swap if player=1, so we always provide Black's view
	//
	// IMPORTANT: We always set up the board from Black's perspective:
	// - Black (color 0): lazyBG point 1 → gnubg index 0, lazyBG point 24 → gnubg index 23
	// - White (color 1): lazyBG point 24 → gnubg index 0, lazyBG point 1 → gnubg index 23
	//
	// gnubg.FindMoves will swap board[0] and board[1] internally if player==1

	var blackSide [25]int
	var whiteSide [25]int

	for i := 1; i <= 24; i++ {
		point := position.Board.Points[i]
		checkers := point.Checkers

		if checkers > 0 {
			if point.Color == Black {
				// Black checkers: lazyBG 1→gnubg 0, lazyBG 24→gnubg 23
				gnubgPoint := i - 1
				blackSide[gnubgPoint] = checkers
			} else {
				// White checkers: lazyBG 24→gnubg 0, lazyBG 1→gnubg 23
				gnubgPoint := 24 - i
				whiteSide[gnubgPoint] = checkers
			}
		}
	}

	// Handle bar
	// lazyBG: point[25] is black bar, point[0] is white bar
	blackSide[24] = position.Board.Points[25].Checkers
	whiteSide[24] = position.Board.Points[0].Checkers

	// Always provide board from Black's perspective
	// gnubg will swap internally if player==1
	board[0] = blackSide
	board[1] = whiteSide

	return board
}

// gnubgMoveToLazyBG converts a gnubg move to lazyBG notation
// gnubg ALWAYS returns moves from the player's home board perspective:
// - gnubg index 0 = player's home point (where they bear off)
// - gnubg index 23 = player's furthest point from home
// - gnubg index 24 = bar
//
// For lazyBG:
// - Black (player 0): home is point 1 (moves 24→1, bears off at 1)
// - White (player 1): home is point 24 (moves 1→24, bears off at 24)
func gnubgMoveToLazyBG(move gnubg.Move, player int) string {
	playsNum := move.GetPlaysNum()
	if playsNum == 0 {
		return "No move"
	}

	moveStr := ""
	for i := 0; i < playsNum; i++ {
		play := move.GetPlay(i)
		from := play[0] // 0-24 (24 is bar, 0 is home for bearing off)
		to := play[1]   // 0-24 (0 is off)

		var fromStr, toStr string

		// Handle bar
		if from == 24 {
			fromStr = "bar"
		} else {
			// IMPORTANT: gnubg ALWAYS returns moves in the same coordinate system
			// where index 0 = point closest to bearing off for the current player
			// After internal swap for player=1, gnubg still returns in this format
			// So we ALWAYS use the same conversion formula regardless of player
			// gnubg 0 = lazyBG point 1, gnubg 23 = lazyBG point 24
			fromStr = strconv.Itoa(from + 1)
		}

		// Handle bear off
		if to == -1 || to == 25 {
			toStr = "off"
		} else {
			// Same conversion for destination
			toStr = strconv.Itoa(to + 1)
		}

		if i > 0 {
			moveStr += " "
		}
		// gnubg always returns moves from player's bearing-off perspective
		// Our conversion already handles the coordinate transform correctly
		moveStr += fromStr + "/" + toStr
	}

	return moveStr
}

// round3 rounds a float32 to 3 decimal places
func round3(f float32) float32 {
	return float32(int(f*1000+0.5)) / 1000
}
