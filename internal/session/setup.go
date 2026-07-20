// Session Priors + Board Calibration setup (issue #14; functional-spec §3,
// ux-spec §10): the blocking first step of a session, stored in the .lbg
// Part with the corpus manifest's vocabulary, editable at any later moment —
// recorded plies carry no geometry, so a correction never touches them.
package session

import (
	"fmt"

	"lazybg/internal/bg"
	"lazybg/internal/corpus"
)

// Setup is the user-declared session setup for the (first) Part.
type Setup struct {
	Players [2]string     `json:"players"`
	Priors  corpus.Priors `json:"priors"`
	// Corners are the 4 board corners in frame coordinates (TL,TR,BR,BL).
	Corners [][2]float64 `json:"corners"`
	// VideoURL is the capture's canonical source (e.g. the YouTube URL) —
	// the portability half of the video reference (session-format-spec §1).
	VideoURL string `json:"videoUrl"`
}

// SaveSetup stores the setup (initial or correction) and applies the match
// shell fields (players, length). Recorded turns are never touched.
func (s *Service) SaveSetup(setup Setup) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(setup.Corners) != 4 {
		return fmt.Errorf("board calibration needs exactly 4 corners, got %d", len(setup.Corners))
	}
	if setup.Players[0] != "" {
		s.match.Players = setup.Players
	}
	s.match.Length = setup.Priors.MatchLength
	if s.doc != nil {
		if len(s.doc.Parts) == 0 {
			return fmt.Errorf("no video part to attach the setup to")
		}
		s.doc.Parts[0].Priors = setup.Priors
		s.doc.Parts[0].Calibration.Corners = setup.Corners
		if setup.VideoURL != "" {
			s.doc.Parts[0].URL = setup.VideoURL
		}
		if err := s.save(); err != nil {
			return fmt.Errorf("autosave: %w", err)
		}
	}
	return nil
}

// GetSetup returns the current setup (pre-filled correction form).
func (s *Service) GetSetup() Setup {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Setup{Players: s.match.Players}
	if s.doc != nil && len(s.doc.Parts) > 0 {
		out.Priors = s.doc.Parts[0].Priors
		out.Corners = s.doc.Parts[0].Calibration.Corners
		out.VideoURL = s.doc.Parts[0].URL
	}
	return out
}

// Orientation returns the parsed board-orientation prior of the (first) Part —
// P1HomeBottomRight when unset. It drives the reconstructed board's on-screen
// orientation (display-out boundary, ADR-0006).
func (s *Service) Orientation() bg.Orientation {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil || len(s.doc.Parts) == 0 {
		return bg.P1HomeBottomRight
	}
	o, _ := bg.ParseOrientation(s.doc.Parts[0].Priors.Orientation)
	return o
}

// SetupDone reports whether the blocking setup step is complete (the 4
// corners exist — the one thing turn entry cannot proceed without).
func (s *Service) SetupDone() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc != nil && len(s.doc.Parts) > 0 && len(s.doc.Parts[0].Calibration.Corners) == 4
}
