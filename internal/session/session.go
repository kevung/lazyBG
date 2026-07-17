// Package session is the transcription session service: the Wails-agnostic
// core the GUI (and any future headless/REST mode) drives (ADR-0003). It owns
// the growing bg.Match, the current board chain, the pending turn and its
// ranked candidates — the manual half of the unified manual/automatic model
// (docs/functional-spec.md §1, §4).
//
// The engine must be initialized (engine.Init) before any Service is used.
package session

import (
	"fmt"
	"sync"

	"lazybg/internal/bg"
	"lazybg/internal/engine"
)

// MaxCandidates caps the ranked candidate list shown to the user
// (functional-spec §4: "top 5–10").
const MaxCandidates = 10

// Candidate is one ranked (dice-implied) checker-move proposal.
type Candidate struct {
	Notation string  `json:"notation"`
	Equity   float64 `json:"equity"`
	// Score is the fused ranking score. Until the board-diff cue contributes
	// (ticket #15) it equals Equity.
	Score float64 `json:"score"`
}

// PlyView is the move-list projection of one recorded ply.
type PlyView struct {
	Index    int    `json:"index"`
	Game     int    `json:"game"`
	Player   int    `json:"player"`
	Dice     string `json:"dice"`
	Notation string `json:"notation"`
	TickMs   int    `json:"tickMs"`
}

type pendingTurn struct {
	dice   bg.Dice
	cands  []engine.LegalMove
	player bg.Player
}

// Service is one in-progress transcription session.
type Service struct {
	mu      sync.Mutex
	match   bg.Match
	board   bg.Board
	onRoll  bg.Player
	pending *pendingTurn

	// Persistence (nil/empty for pure in-memory sessions): the .lbg document
	// this session autosaves to after every confirmed decision.
	lbgPath string
	doc     *LBG
}

// New starts a fresh session on a standard board. Session Priors and
// calibration are the setup flow's concern (ticket #14); the skeleton hardcodes
// a bare match shell.
func New() *Service {
	return &Service{
		match: bg.Match{
			Length:  0, // unlimited until priors set a match length
			Players: [2]string{"Player 1", "Player 2"},
			Games:   []bg.Game{{Number: 1}},
		},
		board:  bg.StandardStart(),
		onRoll: bg.P1,
	}
}

// SetTurnPlayer declares who the pending/current turn belongs to — the
// transcriber watches who actually plays (first turn of a game, mainly).
// Allowed any time before the turn is confirmed; alternation resumes from it.
func (s *Service) SetTurnPlayer(player int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if player != 0 && player != 1 {
		return fmt.Errorf("player must be 0 or 1, got %d", player)
	}
	s.onRoll = bg.Player(player)
	if s.pending != nil {
		// Re-rank for the new player on the same dice.
		p := s.pending
		s.pending = nil
		if _, err := s.enterDiceLocked(p.dice[0], p.dice[1]); err != nil {
			return err
		}
	}
	return nil
}

// EnterDice records the observed roll for the pending turn and returns the
// ranked candidate list (equity-ranked legal moves, best first). Re-entering
// dice before confirming replaces the pending turn — error recovery is cheap
// by design (functional-spec §4). An empty list means the player dances.
func (s *Service) EnterDice(d1, d2 int) ([]Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enterDiceLocked(d1, d2)
}

func (s *Service) enterDiceLocked(d1, d2 int) ([]Candidate, error) {
	if d1 < 1 || d1 > 6 || d2 < 1 || d2 > 6 {
		return nil, fmt.Errorf("dice out of range: %d-%d", d1, d2)
	}
	pos := bg.Position{Board: s.board, Dice: bg.Dice{d1, d2}, PlayerOnRoll: s.onRoll}
	moves, err := engine.LegalMoves(pos)
	if err != nil {
		return nil, err
	}
	if len(moves) > MaxCandidates {
		moves = moves[:MaxCandidates]
	}
	s.pending = &pendingTurn{dice: bg.Dice{d1, d2}, cands: moves, player: s.onRoll}
	return candidateViews(moves), nil
}

// Candidates re-returns the pending turn's ranked list (nil if no dice entered).
func (s *Service) Candidates() []Candidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return nil
	}
	return candidateViews(s.pending.cands)
}

// Confirm applies the pending turn's candidate at index, stamped with the
// video tick. Confidence is 0 — the bg.Ply convention for human-entered moves.
// The board chain advances to the engine's resulting position and the player
// on roll alternates.
func (s *Service) Confirm(index, tickMs int) (PlyView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.pending
	if p == nil {
		return PlyView{}, fmt.Errorf("no pending dice — enter the roll first")
	}
	if index < 0 || index >= len(p.cands) {
		return PlyView{}, fmt.Errorf("candidate index %d out of range (0..%d)", index, len(p.cands)-1)
	}
	mv := p.cands[index]
	ply := bg.Ply{
		Player:     p.player,
		Dice:       p.dice,
		Notation:   mv.Notation,
		Tick:       tickMs,
		Confidence: 0,
	}
	g := &s.match.Games[len(s.match.Games)-1]
	g.Plies = append(g.Plies, ply)
	s.board = mv.Result
	s.onRoll = otherPlayer(p.player)
	s.pending = nil

	// Autosave: every confirmed decision is persisted immediately, with its
	// full candidate traceability (functional-spec §3, session-format-spec §3).
	if s.doc != nil {
		cands := make([]LBGCandidate, len(p.cands))
		for i, c := range p.cands {
			cands[i] = LBGCandidate{Notation: c.Notation, Equity: c.Equity, Score: c.Equity}
		}
		s.doc.Turns = append(s.doc.Turns, LBGTurn{
			Game:        g.Number,
			Player:      int(ply.Player),
			Dice:        [2]int{p.dice[0], p.dice[1]},
			Notation:    ply.Notation,
			Part:        0,
			TickMs:      tickMs,
			Candidates:  cands,
			ChosenIndex: index,
			Cues:        []string{"engine-equity"},
		})
		s.doc.LastTickMs = tickMs
		if err := s.save(); err != nil {
			return PlyView{}, fmt.Errorf("autosave: %w", err)
		}
	}
	return plyView(len(g.Plies)-1, g.Number, ply), nil
}

// Moves returns the move-list projection of every recorded ply, in order.
func (s *Service) Moves() []PlyView {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []PlyView
	for _, g := range s.match.Games {
		for i, ply := range g.Plies {
			out = append(out, plyView(i, g.Number, ply))
		}
	}
	return out
}

// Board returns the current position (after every confirmed ply).
func (s *Service) Board() bg.Board {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.board
}

// OnRoll returns the player the pending/next turn belongs to.
func (s *Service) OnRoll() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int(s.onRoll)
}

func candidateViews(moves []engine.LegalMove) []Candidate {
	out := make([]Candidate, len(moves))
	for i, mv := range moves {
		out[i] = Candidate{Notation: mv.Notation, Equity: mv.Equity, Score: mv.Equity}
	}
	return out
}

func plyView(index, game int, ply bg.Ply) PlyView {
	return PlyView{
		Index:    index,
		Game:     game,
		Player:   int(ply.Player),
		Dice:     ply.Dice.String(),
		Notation: ply.Notation,
		TickMs:   ply.Tick,
	}
}

func otherPlayer(p bg.Player) bg.Player {
	if p == bg.P1 {
		return bg.P2
	}
	return bg.P1
}
