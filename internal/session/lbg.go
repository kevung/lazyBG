// The .lbg session file: lazyBG's working format and single source of truth
// (docs/session-format-spec.md). JSON, schema-versioned like corpus.Manifest;
// .mat and the corpus manifest are projections generated from it (ticket #22).
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"lazybg/internal/bg"
	"lazybg/internal/corpus"
	"lazybg/internal/derive"
)

// LBGSchemaVersion is the .lbg format version this package writes/understands.
const LBGSchemaVersion = 1

// fingerprintHead is how much of the video file the fingerprint hashes.
// Size + head-hash detects substitution/corruption without reading gigabytes.
const fingerprintHead = 1 << 20

// LBG is the serialized session document.
type LBG struct {
	SchemaVersion int    `json:"schemaVersion"`
	ID            string `json:"id,omitempty"`

	Length  int       `json:"matchLength,omitempty"`
	Players [2]string `json:"players"`

	Parts   []LBGPart   `json:"parts"`
	Turns   []LBGTurn   `json:"turns"`
	Review  []LBGReview `json:"review,omitempty"`
	Results []LBGResult `json:"results,omitempty"`

	// Resume state.
	LastPart   int `json:"lastPart"`
	LastTickMs int `json:"lastTickMs"`
}

// LBGResult is one closed game's confirmed result.
type LBGResult struct {
	Game   int `json:"game"`
	Winner int `json:"winner"`
	Points int `json:"points"`
}

// LBGReview is one review-queue entry: a turn flagged for a second pass —
// by the human themselves (human-flagged) or by a cascade re-validation —
// and whether it has been resolved (pipeline.ReviewItem's persisted form).
type LBGReview struct {
	TurnSeq  int    `json:"turnSeq"`
	Reason   string `json:"reason"`
	Resolved bool   `json:"resolved,omitempty"`
}

// LBGPart is one video file: local path (this machine), canonical URL
// (portability), fingerprint (substitution check), and the Part's setup —
// the same Priors/Calibration/Span vocabulary as the corpus manifest.
type LBGPart struct {
	File        string             `json:"file"`
	URL         string             `json:"url,omitempty"`
	Fingerprint LBGFingerprint     `json:"fingerprint"`
	Priors      corpus.Priors      `json:"priors,omitempty"`
	Calibration corpus.Calibration `json:"calibration,omitempty"`
	Span        corpus.Span        `json:"span,omitempty"`
}

// LBGFingerprint identifies the video file contents: total size + sha256 of
// the first MiB.
type LBGFingerprint struct {
	Size     int64  `json:"size"`
	HeadHash string `json:"headSha256"`
}

// LBGCandidate is one ranked candidate as it was shown to the user.
type LBGCandidate struct {
	Notation string  `json:"notation"`
	Equity   float64 `json:"equity"`
	Score    float64 `json:"score"`
}

// LBGTurn is one recorded decision with its full traceability: what was
// shown, what was picked, which cues contributed (session-format-spec §3).
// ChosenIndex is -1 when the move came from the override escape hatch or an
// automatic dance (nothing was picked from the list).
type LBGTurn struct {
	Game       int    `json:"game"`
	Player     int    `json:"player"`
	Dice       [2]int `json:"dice"`
	Notation   string `json:"notation"`
	CannotMove bool   `json:"cannotMove,omitempty"`
	Cube       string `json:"cube,omitempty"` // "double"/"take"/"drop"
	CubeValue  int    `json:"cubeValue,omitempty"`

	Part   int `json:"part"`
	TickMs int `json:"tickMs"`

	Candidates  []LBGCandidate `json:"candidates,omitempty"`
	ChosenIndex int            `json:"chosenIndex"`
	Cues        []string       `json:"cues,omitempty"`
}

// Fingerprint computes the video file's identity (size + head hash).
func Fingerprint(path string) (LBGFingerprint, error) {
	f, err := os.Open(path)
	if err != nil {
		return LBGFingerprint{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return LBGFingerprint{}, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, fingerprintHead)); err != nil {
		return LBGFingerprint{}, err
	}
	return LBGFingerprint{Size: st.Size(), HeadHash: hex.EncodeToString(h.Sum(nil))}, nil
}

// Create starts a new session backed by a fresh .lbg file for the given
// video. The returned warning is empty on success paths that need no user
// attention.
func Create(lbgPath, videoPath, videoURL string) (*Service, string, error) {
	fp, err := Fingerprint(videoPath)
	if err != nil {
		return nil, "", fmt.Errorf("fingerprint video: %w", err)
	}
	s := New()
	s.lbgPath = lbgPath
	s.doc = &LBG{
		SchemaVersion: LBGSchemaVersion,
		ID:            strippedName(lbgPath),
		Players:       s.match.Players,
		Parts: []LBGPart{{
			File:        videoPath,
			URL:         videoURL,
			Fingerprint: fp,
		}},
	}
	if err := s.save(); err != nil {
		return nil, "", err
	}
	return s, "", nil
}

// Open resumes a session from its .lbg file, replaying the recorded turns to
// rebuild the board chain. A non-empty warning flags a missing or substituted
// video file — the session still opens; the transcription data is intact.
func Open(lbgPath string) (*Service, string, error) {
	raw, err := os.ReadFile(lbgPath)
	if err != nil {
		return nil, "", err
	}
	var doc LBG
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "", fmt.Errorf("parse %s: %w", lbgPath, err)
	}
	if doc.SchemaVersion > LBGSchemaVersion {
		return nil, "", fmt.Errorf("%s: schema version %d is newer than this build understands (%d)",
			lbgPath, doc.SchemaVersion, LBGSchemaVersion)
	}
	if len(doc.Parts) == 0 {
		return nil, "", fmt.Errorf("%s: no video part", lbgPath)
	}

	s := New()
	s.lbgPath = lbgPath
	s.doc = &doc
	if doc.Players[0] != "" {
		s.match.Players = doc.Players
	}
	s.match.Length = doc.Length

	resultOf := make(map[int]LBGResult, len(doc.Results))
	for _, r := range doc.Results {
		resultOf[r.Game] = r
	}
	openGame := func(n int) {
		// Close the previous game with its stored result and start game n on
		// a fresh board (the multi-game replay path).
		g := &s.match.Games[len(s.match.Games)-1]
		if r, ok := resultOf[g.Number]; ok && g.Result == nil {
			g.Result = &bg.GameResult{Winner: bg.Player(r.Winner), Points: r.Points}
		}
		score := g.StartScore
		if g.Result != nil {
			score[g.Result.Winner] += g.Result.Points
		}
		s.match.Games = append(s.match.Games, bg.Game{Number: n, StartScore: score})
		s.board = bg.StandardStart()
		s.cube = cubeState{value: 1}
		s.onRoll = bg.P1
	}

	// Replay the recorded turns to rebuild board + alternation. A Cannot
	// Move (or empty override) leaves the board as-is; cube actions rebuild
	// the cube state.
	for i, t := range doc.Turns {
		for t.Game > s.match.Games[len(s.match.Games)-1].Number {
			openGame(s.match.Games[len(s.match.Games)-1].Number + 1)
		}
		if t.Cube != "" {
			s.applyCubeReplay(t.Cube, bg.Player(t.Player))
			g := &s.match.Games[len(s.match.Games)-1]
			g.Plies = append(g.Plies, bg.Ply{
				Player:    bg.Player(t.Player),
				Cube:      cubeActionOf(t.Cube),
				CubeValue: t.CubeValue,
				Tick:      t.TickMs,
			})
			s.onRoll = otherPlayer(bg.Player(t.Player))
			continue
		}
		if !t.CannotMove && t.Notation != "" {
			board, err := derive.ApplyNotation(s.board, bg.Player(t.Player), t.Notation)
			if err != nil {
				return nil, "", fmt.Errorf("%s: replay turn %d (%s): %w", lbgPath, i+1, t.Notation, err)
			}
			s.board = board
		}
		s.onRoll = otherPlayer(bg.Player(t.Player))
		g := &s.match.Games[len(s.match.Games)-1]
		g.Plies = append(g.Plies, bg.Ply{
			Player:     bg.Player(t.Player),
			Dice:       bg.Dice{t.Dice[0], t.Dice[1]},
			Notation:   t.Notation,
			CannotMove: t.CannotMove,
			Tick:       t.TickMs,
			Confidence: 0,
		})
	}
	if g := &s.match.Games[len(s.match.Games)-1]; g.Result == nil {
		if r, ok := resultOf[g.Number]; ok {
			g.Result = &bg.GameResult{Winner: bg.Player(r.Winner), Points: r.Points}
		}
	}
	s.reviews = append([]LBGReview(nil), doc.Review...)

	warn := ""
	part := doc.Parts[0]
	if fp, err := Fingerprint(part.File); err != nil {
		warn = fmt.Sprintf("video file %s is missing or unreadable (%v) — fix the path to keep transcribing; the transcription itself is intact", part.File, err)
	} else if fp != part.Fingerprint {
		warn = fmt.Sprintf("video file %s does not match the recorded fingerprint — it may have been replaced or re-encoded; ticks may no longer line up", part.File)
	}
	return s, warn, nil
}

// save writes the document atomically (temp file + rename).
func (s *Service) save() error {
	if s.doc == nil || s.lbgPath == "" {
		return nil // in-memory session (tests, demo) — nothing to persist
	}
	s.doc.Players = s.match.Players
	s.doc.Length = s.match.Length
	s.doc.Review = s.reviews
	data, err := json.MarshalIndent(s.doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.lbgPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.lbgPath)
}

// SetVideoPos records the last-worked video position (resume state) and
// persists it.
func (s *Service) SetVideoPos(tickMs int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return
	}
	s.doc.LastTickMs = tickMs
	_ = s.save() // best-effort; the next confirm persists it anyway
}

// LastVideoPos returns the last-worked video position (0 for new sessions).
func (s *Service) LastVideoPos() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return 0
	}
	return s.doc.LastTickMs
}

// VideoPath returns the session's (first Part's) local video path.
func (s *Service) VideoPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil || len(s.doc.Parts) == 0 {
		return ""
	}
	return s.doc.Parts[0].File
}

func strippedName(path string) string {
	base := filepath.Base(path)
	return base[:len(base)-len(filepath.Ext(base))]
}

func cubeActionOf(name string) bg.CubeAction {
	switch name {
	case "double":
		return bg.Double
	case "take":
		return bg.Take
	case "drop":
		return bg.Drop
	}
	return bg.NoCube
}
