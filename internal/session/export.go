// Exports (issue #22; session-format-spec §1): .mat and the corpus manifest
// are projections generated on demand from the .lbg session — the single
// source of truth. Exportable at any point, never a separate finalize step,
// so the three can never drift apart.
package session

import (
	"encoding/json"
	"fmt"
	"os"

	"lazybg/internal/corpus"
	"lazybg/internal/matexport"
)

// ExportMat writes the current match as a Jellyfish .mat file.
func (s *Service) ExportMat(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.WriteFile(path, []byte(matexport.Write(s.match)), 0o644)
}

// ExportManifest writes the corpus Recording manifest projection —
// calibration, priors, spans and per-turn ticks straight from the session,
// no post-hoc realignment (unlike `lazybg align`, which exists for videos
// whose .mat predates lazyBG). matPath is recorded as the manifest's
// ground-truth transcript reference.
func (s *Service) ExportManifest(path, matPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return fmt.Errorf("no .lbg session to project")
	}
	man := corpus.Manifest{
		SchemaVersion: corpus.SchemaVersion,
		ID:            s.doc.ID,
		Transcript:    matPath,
	}
	// The corpus loader requires a non-empty Active Span; when the setup
	// declared none, synthesize it from the recorded ticks (0 → last tick).
	lastTick := s.doc.LastTickMs
	for _, t := range s.doc.Turns {
		if t.TickMs > lastTick {
			lastTick = t.TickMs
		}
	}
	if lastTick < 1 {
		lastTick = 1
	}
	for _, p := range s.doc.Parts {
		span := p.Span
		if span.EndMs <= span.BeginMs {
			span = corpus.Span{BeginMs: 0, EndMs: lastTick}
		}
		man.Parts = append(man.Parts, corpus.Part{
			File:        p.File,
			Priors:      p.Priors,
			Calibration: p.Calibration,
			Span:        span,
		})
	}
	for i, t := range s.doc.Turns {
		man.Turns = append(man.Turns, corpus.Turn{
			Index:  i + 1,
			Part:   t.Part,
			TickMs: t.TickMs,
		})
	}
	data, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
