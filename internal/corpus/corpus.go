// Package corpus loads and validates a Recording manifest — the labeling
// artifact that ties a match's video Part(s) to its .mat transcript, capture
// cell, per-Part Session Priors + Board Calibration, and the per-turn commit
// ticks (docs/experiment-plan.md §7). One Recording = one match; a Part is one
// ordered video file. Priors/calibration may be inherited from the prior Part
// when the setup is unchanged.
package corpus

import (
	"encoding/json"
	"fmt"
)

// SchemaVersion is the manifest format version this package understands.
const SchemaVersion = 1

// Manifest is one Recording's labels.
type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	ID            string `json:"id"`
	Transcript    string `json:"transcript"` // path to the .mat ground truth
	Cell          Cell   `json:"cell"`
	Parts         []Part `json:"parts"`
	Turns         []Turn `json:"turns"`
}

// Cell records the corpus variety-matrix labels for per-cell reporting.
type Cell struct {
	Angle      string `json:"angle"`
	Colors     string `json:"colors"`
	Resolution string `json:"resolution"`
	Dice       string `json:"dice"`
	Audio      string `json:"audio"`
}

// Part is one ordered video file with its setup.
type Part struct {
	File        string      `json:"file"`
	Priors      Priors      `json:"priors"`
	Calibration Calibration `json:"calibration"`
	Span        Span        `json:"span"`
}

// Priors are the user-declared Session Priors for a Part. Inherit copies the
// prior Part's (resolved) priors.
type Priors struct {
	Inherit     bool   `json:"inherit,omitempty"`
	Clock       bool   `json:"clock,omitempty"`
	MatchLength int    `json:"matchLength,omitempty"`
	CheckerA    string `json:"checkerA,omitempty"`
	CheckerB    string `json:"checkerB,omitempty"`
	Orientation string `json:"orientation,omitempty"`
	// ClockROI is the chess clock's box in frame coordinates
	// (x0,y0,x1,y1) — the clock-hit commit detector's search region.
	ClockROI [4]int `json:"clockROI,omitempty"`
}

// Calibration is the four board corners in this Part's frame (order
// TL,TR,BR,BL). Inherit copies the prior Part's (resolved) calibration.
// Canonical optionally overrides the rectified-board geometry — tuned per
// capture so rectification preserves the source's aspect (circles stay
// circular for the shape-first reader); absent means the library default.
type Calibration struct {
	Inherit bool `json:"inherit,omitempty"`
	// Version is the calibration schema version. 0/absent = v1 (four corners,
	// single homography, bar at the canonical default). 2 = the two-homography
	// bar-split model with BarEdges present (ADR-0007).
	Version int          `json:"version,omitempty"`
	Corners [][2]float64 `json:"corners,omitempty"`
	// BarEdges are the four bar-edge handles (order barTL,barTR,barBR,barBL) in
	// frame coordinates. Present ⇒ two-homography calibration; absent ⇒ v1
	// (migrated by placing the bar at the canonical default, reproducing v1).
	BarEdges  [][2]float64 `json:"barEdges,omitempty"`
	Canonical *Canonical   `json:"canonical,omitempty"`
	// Lens optionally declares the capture camera's radial distortion;
	// absent means a pinhole camera (plain homography).
	Lens *Lens `json:"lens,omitempty"`
	// Masks are canonical-space dead zones [x0,y0,x1,y1] painted neutral
	// before any board read — rail areas where spare checkers park, a clock
	// or score card intruding over the frame (world-model dead zones).
	Masks [][4]int `json:"masks,omitempty"`
	// OpeningScore is the per-point read score (of 24) achieved on the
	// settled opening when this calibration was made — the calibration
	// quality proxy that drives the per-recording auto-fill gate.
	OpeningScore int `json:"openingScore,omitempty"`
}

// Lens mirrors calibrate.Lens for the manifest: single-k1 radial distortion
// of the capture camera (negative = barrel, typical wide action cams). Kept
// as a plain struct so the schema package stays dependency-free.
type Lens struct {
	K1      float64 `json:"k1"`
	CenterX float64 `json:"centerX"`
	CenterY float64 `json:"centerY"`
	Norm    float64 `json:"norm"`
}

// Canonical mirrors calibrate.CanonicalBoard for the manifest (kept as a
// plain struct so the schema package stays dependency-free).
type Canonical struct {
	MarginX int `json:"marginX"`
	MarginY int `json:"marginY"`
	PointW  int `json:"pointW"`
	QuadH   int `json:"quadH"`
	BarGap  int `json:"barGap"`
	OffW    int `json:"offW"`
}

// Span is the active play region within a Part, in milliseconds.
type Span struct {
	BeginMs int `json:"beginMs"`
	EndMs   int `json:"endMs"`
}

// Turn is one labeled commit: which Part and video tick the turn was committed.
type Turn struct {
	Index  int `json:"index"`
	Part   int `json:"part"`
	TickMs int `json:"tickMs"`
}

// Load unmarshals, resolves inheritance, and validates a manifest.
func Load(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.resolveInherit(); err != nil {
		return Manifest{}, err
	}
	if err := m.validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// resolveInherit copies priors/calibration forward from the previous Part where
// a Part declares inherit. Inheriting on the first Part is an error.
func (m *Manifest) resolveInherit() error {
	for i := range m.Parts {
		if m.Parts[i].Priors.Inherit {
			if i == 0 {
				return fmt.Errorf("part 0: priors cannot inherit (no prior part)")
			}
			p := m.Parts[i-1].Priors
			p.Inherit = false
			m.Parts[i].Priors = p
		}
		if m.Parts[i].Calibration.Inherit {
			if i == 0 {
				return fmt.Errorf("part 0: calibration cannot inherit (no prior part)")
			}
			c := m.Parts[i-1].Calibration
			c.Inherit = false
			m.Parts[i].Calibration = c
		}
	}
	return nil
}

func (m *Manifest) validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schemaVersion %d, want %d", m.SchemaVersion, SchemaVersion)
	}
	if m.ID == "" {
		return fmt.Errorf("id is required")
	}
	if m.Transcript == "" {
		return fmt.Errorf("transcript is required")
	}
	if len(m.Parts) == 0 {
		return fmt.Errorf("at least one part is required")
	}
	for i, p := range m.Parts {
		if p.File == "" {
			return fmt.Errorf("part %d: file is required", i)
		}
		if len(p.Calibration.Corners) != 4 {
			return fmt.Errorf("part %d: calibration needs 4 corners, got %d", i, len(p.Calibration.Corners))
		}
		if p.Span.BeginMs < 0 || p.Span.EndMs <= p.Span.BeginMs {
			return fmt.Errorf("part %d: span [%d,%d] is empty or negative", i, p.Span.BeginMs, p.Span.EndMs)
		}
	}
	prevIndex := 0
	for k, tn := range m.Turns {
		if tn.Part < 0 || tn.Part >= len(m.Parts) {
			return fmt.Errorf("turn %d: part %d out of range", tn.Index, tn.Part)
		}
		sp := m.Parts[tn.Part].Span
		if tn.TickMs < sp.BeginMs || tn.TickMs > sp.EndMs {
			return fmt.Errorf("turn %d: tick %d outside part %d span [%d,%d]", tn.Index, tn.TickMs, tn.Part, sp.BeginMs, sp.EndMs)
		}
		if tn.Index <= prevIndex {
			return fmt.Errorf("turn %d (position %d): index must strictly increase", tn.Index, k)
		}
		prevIndex = tn.Index
	}
	return nil
}
