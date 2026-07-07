package calibrate

import (
	"math"

	"lazybg/internal/geom"
)

// Lens models radial lens distortion (single Brown-Conrady k1 term) so a wide
// overhead action-cam frame — whose straight board edges bow outward (barrel) —
// can be undistorted before the homography. A plain homography assumes a pinhole
// camera (straight lines stay straight); real capture cams curve them, which
// leaves the canonical grid slightly misaligned. K1 is a Session-Prior/calibration
// value: negative = barrel (typical action cams), positive = pincushion. The zero
// value (Norm==0) is inactive: distort/undistort are the identity, so existing
// calibrations are unchanged.
type Lens struct {
	K1               float64 // radial coefficient in normalised units
	CenterX, CenterY float64 // distortion centre in source px (usually image centre)
	Norm             float64 // normalising radius in px (e.g. half image width); 0 disables
}

func (l Lens) active() bool { return l.Norm > 0 && l.K1 != 0 }

// distort maps an ideal (pinhole) point to the distorted point the sensor
// records — used per pixel when sampling the real source during Rectify. Direct
// radial polynomial: Rd = Ru(1 + K1·Ru²), with R the radius normalised by Norm.
func (l Lens) distort(p geom.Pt) geom.Pt {
	if !l.active() {
		return p
	}
	dx, dy := p.X-l.CenterX, p.Y-l.CenterY
	ru2 := (dx*dx + dy*dy) / (l.Norm * l.Norm)
	f := 1 + l.K1*ru2
	return geom.Pt{X: l.CenterX + dx*f, Y: l.CenterY + dy*f}
}

// undistort maps a distorted (recorded) point back to the ideal pinhole point —
// used on the clicked corners and when placing detections. It inverts distort by
// solving the radial cubic Rd = Ru + K1·Ru³ for Ru with Newton's method (stable
// for barrel where the naive fixed point diverges at the periphery).
func (l Lens) undistort(p geom.Pt) geom.Pt {
	if !l.active() {
		return p
	}
	dx, dy := p.X-l.CenterX, p.Y-l.CenterY
	rd := math.Hypot(dx, dy) / l.Norm
	if rd < 1e-9 {
		return p
	}
	ru := rd
	for i := 0; i < 20; i++ {
		f := l.K1*ru*ru*ru + ru - rd
		df := 3*l.K1*ru*ru + 1
		if df == 0 {
			break
		}
		ru -= f / df
	}
	scale := ru / rd
	return geom.Pt{X: l.CenterX + dx*scale, Y: l.CenterY + dy*scale}
}
