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
	// Corners are the 4 playing-surface corners in frame coordinates (TL,TR,BR,BL).
	Corners [][2]float64 `json:"corners"`
	// BarEdges are the 4 bar-edge handles (barTL,barTR,barBR,barBL). Present ⇒
	// two-homography calibration (ADR-0007); absent ⇒ v1 (migrated).
	BarEdges [][2]float64 `json:"barEdges,omitempty"`
	// Lens is the capture camera's radial distortion (nil = pinhole). Filled
	// by auto-detection; the GUI can only reset it to nil (ADR-0008 §9).
	Lens *corpus.Lens `json:"lens,omitempty"`
	// VideoURL is the capture's canonical source (e.g. the YouTube URL) —
	// the portability half of the video reference (session-format-spec §1).
	VideoURL string `json:"videoUrl"`
	// SwapPlies asks for the recorded play to change hands along with this
	// setup: the form's "swap the two players" button already exchanged the
	// names and the colours it holds, but who played each turn and who won
	// each game lives in the document (ADR-0009). Odd number of presses ⇒ true.
	SwapPlies bool `json:"swapPlies,omitempty"`
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
	s.priors = setup.Priors
	if s.doc != nil {
		if len(s.doc.Parts) == 0 {
			return fmt.Errorf("no video part to attach the setup to")
		}
		s.doc.Parts[s.activePartIdx()].Priors = setup.Priors
		s.doc.Parts[s.activePartIdx()].Calibration.Corners = setup.Corners
		s.doc.Parts[s.activePartIdx()].Calibration.BarEdges = setup.BarEdges
		s.doc.Parts[s.activePartIdx()].Calibration.Lens = setup.Lens
		if len(setup.BarEdges) == 4 {
			s.doc.Parts[s.activePartIdx()].Calibration.Version = 2 // two-homography (ADR-0007)
		} else {
			s.doc.Parts[s.activePartIdx()].Calibration.Version = 0
		}
		if setup.VideoURL != "" {
			s.doc.Parts[s.activePartIdx()].URL = setup.VideoURL
		}
		if setup.SwapPlies {
			s.doc.swapPlies()
		}
		if err := s.save(); err != nil {
			return fmt.Errorf("autosave: %w", err)
		}
		if setup.SwapPlies {
			// Re-derive the board chain: the same player-relative notations
			// replayed by the other player give the mirrored position.
			if err := s.rebuildFromDoc(); err != nil {
				return fmt.Errorf("re-derive after swapping the players: %w", err)
			}
		}
	}
	return nil
}

// GetSetup returns the current setup (pre-filled correction form).
func (s *Service) GetSetup() Setup {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Setup{Players: s.match.Players, Priors: s.priors}
	if s.doc != nil && len(s.doc.Parts) > 0 {
		out.Corners = s.doc.Parts[s.activePartIdx()].Calibration.Corners
		out.BarEdges = s.doc.Parts[s.activePartIdx()].Calibration.BarEdges
		out.Lens = s.doc.Parts[s.activePartIdx()].Calibration.Lens
		out.VideoURL = s.doc.Parts[s.activePartIdx()].URL
	}
	// Fill the cube-rule pointers to their resolved values so the setup form
	// shows concrete defaults (cube+Crawford on, Jacoby+Beaver off) for a fresh
	// session (issue #24).
	out.Priors = out.Priors.WithCubeDefaults()
	return out
}

// Orientation returns the parsed board-orientation prior of the (first) Part —
// P1HomeRight when unset. It drives the reconstructed board's on-screen
// orientation (display-out boundary, ADR-0006). Player 1 is always the bottom
// player, so this says only which half holds the home boards (ADR-0009).
func (s *Service) Orientation() bg.Orientation {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil || len(s.doc.Parts) == 0 {
		return bg.P1HomeRight
	}
	o, _ := bg.ParseOrientation(s.doc.Parts[s.activePartIdx()].Priors.Orientation)
	return o
}

// SetupDone reports whether the blocking setup step is complete (the 4
// corners exist — the one thing turn entry cannot proceed without).
func (s *Service) SetupDone() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.doc != nil && len(s.doc.Parts) > 0 && len(s.doc.Parts[s.activePartIdx()].Calibration.Corners) == 4
}
