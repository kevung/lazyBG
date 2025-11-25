package main

const (
	NumPoints = 24
	BlackBar  = 25
	WhiteBar  = 0
	None      = -1
	Black     = 0
	White     = 1
)

const (
	Unlimited    = -1
	PostCrawford = 0
	Crawford     = 1
)

const (
	CheckerAction = iota
	CubeAction
)

const (
	NoDouble = iota
	Double
	ReDouble
	TooGood
	Take
	Pass
	Beaver
)

const (
	DatabaseVersion      = "1.2.0"
	TranscriptionVersion = "1.0.0"
)

// Transcription models
type TranscriptionMetadata struct {
	Site        string `json:"site"`
	MatchID     string `json:"matchId"`
	Event       string `json:"event"`
	Round       string `json:"round"`
	Player1     string `json:"player1"`
	Player2     string `json:"player2"`
	EventDate   string `json:"eventDate"`
	EventTime   string `json:"eventTime"`
	Variation   string `json:"variation"`
	Unrated     string `json:"unrated"`
	Crawford    string `json:"crawford"`
	CubeLimit   string `json:"cubeLimit"`
	Transcriber string `json:"transcriber"`
	MatchLength int    `json:"matchLength"`
}

type MoveData struct {
	Dice      string `json:"dice"`
	Move      string `json:"move"`
	IsIllegal bool   `json:"isIllegal"`
	IsGala    bool   `json:"isGala"`
}

type CubeActionData struct {
	Player   int    `json:"player"`
	Action   string `json:"action"`
	Value    int    `json:"value"`
	Response string `json:"response,omitempty"`
}

type TranscriptionMove struct {
	MoveNumber  int             `json:"moveNumber"`
	Player1Move *MoveData       `json:"player1Move,omitempty"`
	Player2Move *MoveData       `json:"player2Move,omitempty"`
	CubeAction  *CubeActionData `json:"cubeAction,omitempty"`
}

type GameWinner struct {
	Player int `json:"player"`
	Points int `json:"points"`
}

type TranscriptionGame struct {
	GameNumber   int                 `json:"gameNumber"`
	Player1Score int                 `json:"player1Score"`
	Player2Score int                 `json:"player2Score"`
	Moves        []TranscriptionMove `json:"moves"`
	Winner       *GameWinner         `json:"winner,omitempty"`
}

type Transcription struct {
	Metadata TranscriptionMetadata `json:"metadata"`
	Games    []TranscriptionGame   `json:"games"`
}

type Point struct {
	Checkers int `json:"checkers"`
	Color    int `json:"color"`
}

type Cube struct {
	Owner int `json:"owner"`
	Value int `json:"value"`
}

type Board struct {
	Points  [NumPoints + 2]Point `json:"points"`
	Bearoff [2]int               `json:"bearoff"`
}

type Position struct {
	ID           int64  `json:"id"` // Add ID field
	Board        Board  `json:"board"`
	Cube         Cube   `json:"cube"`
	Dice         [2]int `json:"dice"`
	Score        [2]int `json:"score"`
	PlayerOnRoll int    `json:"player_on_roll"`
	DecisionType int    `json:"decision_type"`
	HasJacoby    int    `json:"has_jacoby"` // Add HasJacoby field
	HasBeaver    int    `json:"has_beaver"` // Add HasBeaver field
}

func initializeBoard() Board {
	var board Board

	board.Points[1] = Point{2, White}
	board.Points[12] = Point{5, White}
	board.Points[17] = Point{3, White}
	board.Points[19] = Point{5, White}

	board.Points[24] = Point{2, Black}
	board.Points[13] = Point{5, Black}
	board.Points[8] = Point{3, Black}
	board.Points[6] = Point{5, Black}
	return board
}

func InitializePosition() Position {
	var position Position

	position.Board = initializeBoard()
	position.Cube = Cube{None, 0}
	position.Dice = [2]int{3, 1}
	position.Score = [2]int{7, 7}
	position.PlayerOnRoll = Black
	position.DecisionType = CheckerAction

	return position
}

func (p *Position) MatchesCheckerPosition(filter Position) bool {
	for i := 0; i < len(p.Board.Points); i++ {
		if filter.Board.Points[i].Checkers > 0 {
			if p.Board.Points[i].Color != filter.Board.Points[i].Color || p.Board.Points[i].Checkers < filter.Board.Points[i].Checkers {
				return false
			}
		}
	}
	return true
}
