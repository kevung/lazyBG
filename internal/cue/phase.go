package cue

// Turn-phase attribution (world-model v1). A turn's visible lifecycle is
//
//	... [turn i-1 commits] → roll → think → move → [turn i commits] ...
//
// so the dice OF turn i appear strictly between turn i-1's commit tick and
// turn i's own commit tick. The hand-labeled dice audit (2026-07, 132 crops)
// measured the old fixed window [-15s,+4s] at only ~25-60% correct: dice
// appearing shortly after a tick belong to the NEXT turn, and appearances
// within a few seconds before a tick are contaminated by the mover's hands.

// AttributeRoll assigns a dice-appearance tick to the turn whose roll phase
// contains it. turnTicks are the commit ticks of consecutive turns, ascending.
// settleMs guards the band just before a tick (the move being played);
// clearMs guards the band just after (dice removal / hands). Returns the turn
// index, with ok=false when the appearance is ambiguous (inside a guard band)
// or outside every phase; the index is still the best guess where one exists
// (-1 when there is none).
func AttributeRoll(turnTicks []int, appear, settleMs, clearMs int) (int, bool) {
	if len(turnTicks) == 0 {
		return -1, false
	}
	for i, tick := range turnTicks {
		lo := 0
		if i > 0 {
			lo = turnTicks[i-1]
		}
		if appear >= tick {
			continue // belongs to a later turn (or past the end)
		}
		if appear < lo {
			return i, false // before the previous commit: should not happen
		}
		ok := appear > lo+clearMs && appear <= tick-settleMs
		return i, ok
	}
	return -1, false
}
