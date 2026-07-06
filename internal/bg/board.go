package bg

// Board is the absolute checker layout (docs/domain-model.md §5). Convention,
// matching the gnubg port: P1's home is point 1 (P1 moves 24→1, bears off at 1);
// P2's home is point 24 (P2 moves 1→24, bears off at 24).
type Board struct {
	Pts [25]Point // index 1..24 used; index 0 unused
	Bar [2]int    // checkers on the bar, indexed by Player
	Off [2]int    // checkers borne off, indexed by Player
}

// Point is one board point's occupancy. Owner is meaningful only when N > 0.
type Point struct {
	N     int
	Owner Player
}

// Position bundles a board with the pending roll and who is on roll — the unit
// handed to the engine.
type Position struct {
	Board        Board
	Dice         Dice
	PlayerOnRoll Player
}

// StandardStart returns the opening position.
func StandardStart() Board {
	var b Board
	set := func(p, n int, who Player) { b.Pts[p] = Point{N: n, Owner: who} }
	// P1 (home 1): 24-point, midpoint 13, 8-point, 6-point.
	set(24, 2, P1)
	set(13, 5, P1)
	set(8, 3, P1)
	set(6, 5, P1)
	// P2 (home 24): mirror.
	set(1, 2, P2)
	set(12, 5, P2)
	set(17, 3, P2)
	set(19, 5, P2)
	return b
}

// Count returns the total checkers a player has on the board points (excludes
// bar and off).
func (b Board) onPoints(who Player) int {
	total := 0
	for i := 1; i <= 24; i++ {
		if b.Pts[i].N > 0 && b.Pts[i].Owner == who {
			total += b.Pts[i].N
		}
	}
	return total
}

// Checkers returns a player's total accounted checkers (points + bar + off).
func (b Board) Checkers(who Player) int {
	return b.onPoints(who) + b.Bar[who] + b.Off[who]
}
