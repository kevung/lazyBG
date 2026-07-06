// Package profile holds the user-declared Session Priors that seed perception
// (docs/domain-model.md §2, "Capture Profile"). This is the skeleton subset —
// just what the board-state reader needs; clock/orientation/etc. join as their
// detectors land.
package profile

import "image/color"

// CaptureProfile carries the declared constants that constrain the CV.
type CaptureProfile struct {
	CheckerA color.RGBA // one player's checker color
	CheckerB color.RGBA // the other player's checker color
}
