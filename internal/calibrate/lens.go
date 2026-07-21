package calibrate

import (
	"math"

	"lazybg/internal/geom"
)

// Lens models radial lens distortion (Brown-Conrady k1 + k2 terms) so a wide
// overhead action-cam frame — whose straight board edges bow outward (barrel) —
// can be undistorted before the homography. A plain homography assumes a pinhole
// camera (straight lines stay straight); real capture cams curve them, which
// leaves the canonical grid slightly misaligned. K1/K2 are Session-Prior /
// calibration values: negative K1 = barrel (typical action cams), positive =
// pincushion; K2 (r⁴) refines strongly deformed optics (fisheye) and stays 0
// for ordinary lenses — the estimator admits it only when it pays its way
// (ADR-0008 §5). The zero value (Norm==0, or both coefficients 0) is inactive:
// distort/undistort are the identity, so existing calibrations are unchanged.
type Lens struct {
	K1               float64 // r² radial coefficient in normalised units
	K2               float64 // r⁴ radial coefficient in normalised units
	CenterX, CenterY float64 // distortion centre in source px (usually image centre)
	Norm             float64 // normalising radius in px (e.g. half image width); 0 disables
}

func (l Lens) active() bool { return l.Norm > 0 && (l.K1 != 0 || l.K2 != 0) }

// distort maps an ideal (pinhole) point to the distorted point the sensor
// records — used per pixel when sampling the real source during Rectify. Direct
// radial polynomial: Rd = Ru(1 + K1·Ru² + K2·Ru⁴), with R the radius normalised
// by Norm.
func (l Lens) distort(p geom.Pt) geom.Pt {
	if !l.active() {
		return p
	}
	dx, dy := p.X-l.CenterX, p.Y-l.CenterY
	ru2 := (dx*dx + dy*dy) / (l.Norm * l.Norm)
	f := 1 + l.K1*ru2 + l.K2*ru2*ru2
	return geom.Pt{X: l.CenterX + dx*f, Y: l.CenterY + dy*f}
}

// undistort maps a distorted (recorded) point back to the ideal pinhole point —
// used on the clicked corners and when placing detections. It inverts distort by
// solving the radial quintic Rd = Ru + K1·Ru³ + K2·Ru⁵ for Ru with Newton's
// method (stable for barrel where the naive fixed point diverges at the
// periphery).
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
	for i := 0; i < 25; i++ {
		ru2 := ru * ru
		f := l.K2*ru2*ru2*ru + l.K1*ru2*ru + ru - rd
		df := 5*l.K2*ru2*ru2 + 3*l.K1*ru2 + 1
		if df == 0 {
			break
		}
		ru -= f / df
	}
	scale := ru / rd
	return geom.Pt{X: l.CenterX + dx*scale, Y: l.CenterY + dy*scale}
}

// Distort maps an ideal (pinhole) point to the recorded point — exported for
// the lens estimator and the GUI's honest curved overlay (ADR-0008 §9).
func (l Lens) Distort(p geom.Pt) geom.Pt { return l.distort(p) }

// Undistort maps a recorded point back to the ideal pinhole point.
func (l Lens) Undistort(p geom.Pt) geom.Pt { return l.undistort(p) }
