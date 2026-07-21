// Multi-Part sessions (issue #26; functional-spec §6): a match can span several
// video files. Each Part carries its own priors/calibration (inheritable from
// the previous Part), every turn records the Part it was transcribed against,
// and the active Part is persisted so resume returns to the right video.
package session

import (
	"fmt"

	"lazybg/internal/corpus"
)

// PartView describes one Part for the GUI's part switcher.
type PartView struct {
	Index  int    `json:"index"`
	File   string `json:"file"`
	URL    string `json:"url"`
	Active bool   `json:"active"`
}

// Parts lists the session's video Parts (issue #26).
func (s *Service) Parts() []PartView {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return nil
	}
	out := make([]PartView, len(s.doc.Parts))
	for i, p := range s.doc.Parts {
		out[i] = PartView{Index: i, File: p.File, URL: p.URL, Active: i == s.activePart}
	}
	return out
}

// ActivePart returns the index of the Part currently being transcribed.
func (s *Service) ActivePart() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activePart
}

// AddPart appends a new video Part (later file of a multi-video match) with an
// inheritable setup — its priors and calibration default to the previous
// Part's until the user calibrates it — makes it the active Part, and returns
// its index. Requires a persisted (.lbg) session.
func (s *Service) AddPart(videoPath, videoURL string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return 0, fmt.Errorf("no session document to add a Part to")
	}
	fp, err := Fingerprint(videoPath)
	if err != nil {
		return 0, fmt.Errorf("fingerprint %q: %w", videoPath, err)
	}
	s.doc.Parts = append(s.doc.Parts, LBGPart{
		File:        videoPath,
		URL:         videoURL,
		Fingerprint: fp,
		Priors:      corpus.Priors{Inherit: true},
		Calibration: corpus.Calibration{Inherit: true},
	})
	s.activePart = len(s.doc.Parts) - 1
	if err := s.save(); err != nil {
		return 0, fmt.Errorf("autosave: %w", err)
	}
	return s.activePart, nil
}

// SetActivePart switches the Part new turns record against (and the video the
// GUI plays / calibrates). Its priors become the live rule set.
func (s *Service) SetActivePart(i int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil || i < 0 || i >= len(s.doc.Parts) {
		return fmt.Errorf("part %d out of range", i)
	}
	s.activePart = i
	s.priors = s.doc.Parts[i].Priors
	return s.save()
}

// activePartIdx returns the clamped active Part index (0 when no doc/parts).
// Callers hold s.mu.
func (s *Service) activePartIdx() int {
	if s.doc == nil || len(s.doc.Parts) == 0 {
		return 0
	}
	if s.activePart < 0 || s.activePart >= len(s.doc.Parts) {
		return 0
	}
	return s.activePart
}
