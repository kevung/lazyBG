// Package bg holds the pure backgammon-core domain types shared across lazyBG.
// No video, no confidence — see docs/domain-model.md §5. These types are the
// export subject and the engine's world.
package bg

import "strconv"

// Player identifies one of the two players. P1 is the left column in .mat
// exports, P2 the right column.
type Player int

const (
	P1 Player = iota
	P2
)

// Dice holds the two dice of a roll (each 1..6). The zero value means "no dice".
type Dice [2]int

// String renders dice in gnubg/Jellyfish two-digit form, e.g. Dice{2,1} -> "21".
// The zero value renders as "".
func (d Dice) String() string {
	if d[0] == 0 && d[1] == 0 {
		return ""
	}
	return strconv.Itoa(d[0]) + strconv.Itoa(d[1])
}

// CubeAction is a doubling-cube action recorded on a Ply.
type CubeAction int

const (
	NoCube CubeAction = iota
	Double
	Take
	Drop
)

// Ply is one player's recorded action: a dice roll + checker move, or a cube
// action. Tick and Confidence are the bridge back to perception
// (docs/domain-model.md §5): they record when in the video the move happened and
// how sure the pipeline was.
type Ply struct {
	Player     Player
	Dice       Dice
	Notation   string // checker move, e.g. "24/23 13/11"; "" when none / cube action
	CannotMove bool   // true -> rendered as "Cannot Move"
	Cube       CubeAction
	CubeValue  int     // for Double: the value doubled to
	Tick       int     // ms into the capture; 0 if unknown
	Confidence float64 // [0,1]; 0 if unknown / human-entered
}

// GameResult records how a game ended.
type GameResult struct {
	Winner Player
	Points int // points won, already scaled by the cube (1/2/3 × cube value)
}

// Game is one game within a match.
type Game struct {
	Number     int
	StartScore [2]int // score at game start, indexed by Player
	Plies      []Ply
	Result     *GameResult // nil while unfinished
}

// MetaField is one ordered header field, rendered as `; [Key "Value"]`.
type MetaField struct{ Key, Value string }

// Match is a whole backgammon match — the subject of a .mat export.
type Match struct {
	Length  int         // point-match length
	Players [2]string   // display names, indexed by Player
	Meta    []MetaField // ordered metadata header fields
	Games   []Game
}
