// Package calibrate turns four user-clicked board corners into a homography that
// rectifies a source frame to a canonical, axis-aligned top-down board, and
// defines the canonical grid (where each point / bar / off lives on the
// rectified image). See docs/architecture.md §3 ("calibrate") and
// docs/domain-model.md §2 ("Board Calibration").
//
// In the MVP the camera is fixed, so one calibration serves the whole capture.
package calibrate

import (
	"image"
	"image/color"

	"lazybg/internal/geom"
)

// StackDir says which way checkers stack away from a point's outer edge on the
// rectified board.
type StackDir int

const (
	StackDown StackDir = iota // top-row points: checkers descend from the top edge
	StackUp                   // bottom-row points: checkers rise from the bottom edge
)

// CanonicalBoard describes the geometry of the rectified board image: 24 points
// in two rows of 12 split by a central bar gutter, plus an off (bearoff) tray on
// the right. Point numbering follows the standard layout — top row left→right is
// 13..24, bottom row left→right is 12..1 — so point p maps to a fixed region.
type CanonicalBoard struct {
	MarginX, MarginY int
	PointW, QuadH    int
	BarGap           int
	OffW             int
}

// DefaultCanonical is a sensible rectified-board geometry (860×800) with a
// checker diameter equal to PointW.
func DefaultCanonical() CanonicalBoard {
	return CanonicalBoard{MarginX: 20, MarginY: 20, PointW: 60, QuadH: 360, BarGap: 40, OffW: 60}
}

// Size returns the rectified image dimensions.
func (cb CanonicalBoard) Size() (w, h int) {
	w = cb.MarginX + 12*cb.PointW + cb.BarGap + cb.OffW + cb.MarginX
	h = cb.MarginY + cb.QuadH + centerGap + cb.QuadH + cb.MarginY
	return
}

const centerGap = 40

// columnX returns the left x of grid column c (0..11), accounting for the bar.
func (cb CanonicalBoard) columnX(c int) int {
	x := cb.MarginX + c*cb.PointW
	if c >= 6 {
		x += cb.BarGap
	}
	return x
}

// PointRegion returns the rectangle covering point p's checkers and the
// direction its stack grows. p must be in 1..24.
func (cb CanonicalBoard) PointRegion(p int) (image.Rectangle, StackDir) {
	_, h := cb.Size()
	if p >= 13 && p <= 24 { // top row, left→right 13..24
		c := p - 13
		x := cb.columnX(c)
		return image.Rect(x, cb.MarginY, x+cb.PointW, cb.MarginY+cb.QuadH), StackDown
	}
	// bottom row, left→right 12..1
	c := 12 - p
	x := cb.columnX(c)
	top := h - cb.MarginY - cb.QuadH
	return image.Rect(x, top, x+cb.PointW, h-cb.MarginY), StackUp
}

// BarRegion returns the central gutter rectangle (both players' bar).
func (cb CanonicalBoard) BarRegion() image.Rectangle {
	_, h := cb.Size()
	x0 := cb.MarginX + 6*cb.PointW
	return image.Rect(x0, cb.MarginY, x0+cb.BarGap, h-cb.MarginY)
}

// OffRegion returns the bearoff tray rectangle on the right.
func (cb CanonicalBoard) OffRegion() image.Rectangle {
	w, h := cb.Size()
	return image.Rect(w-cb.MarginX-cb.OffW, cb.MarginY, w-cb.MarginX, h-cb.MarginY)
}

// BoardCalibration maps between a source frame and the canonical board.
type BoardCalibration struct {
	Board       CanonicalBoard
	canon2ideal geom.Mat3 // canonical pixel → undistorted (ideal) source pixel
	ideal2canon geom.Mat3 // ideal source pixel → canonical pixel
	lens        Lens      // radial distortion between ideal and recorded source
	ok          bool
}

// New builds a calibration from four source-image corners of the board, given in
// order top-left, top-right, bottom-right, bottom-left. Returns ok=false if the
// corners are degenerate. No lens distortion is applied.
func New(srcCorners [4]geom.Pt, cb CanonicalBoard) (BoardCalibration, bool) {
	return NewWithLens(srcCorners, cb, Lens{})
}

// NewWithLens is New with radial lens distortion. The clicked corners are on the
// recorded (distorted) frame; they are undistorted to ideal space before the
// homography, and Rectify re-distorts when sampling the real source. An inactive
// lens (zero value) makes this identical to New.
func NewWithLens(srcCorners [4]geom.Pt, cb CanonicalBoard, lens Lens) (BoardCalibration, bool) {
	w, h := cb.Size()
	ideal := srcCorners
	if lens.active() {
		for i := range ideal {
			ideal[i] = lens.undistort(srcCorners[i])
		}
	}
	canon := [4]geom.Pt{geom.P(0, 0), geom.P(float64(w), 0), geom.P(float64(w), float64(h)), geom.P(0, float64(h))}
	c2i, ok1 := geom.Homography(canon, ideal)
	i2c, ok2 := geom.Homography(ideal, canon)
	if !ok1 || !ok2 {
		return BoardCalibration{}, false
	}
	return BoardCalibration{Board: cb, canon2ideal: c2i, ideal2canon: i2c, lens: lens, ok: true}, true
}

// PointRegion delegates to the canonical board.
func (c BoardCalibration) PointRegion(p int) (image.Rectangle, StackDir) {
	return c.Board.PointRegion(p)
}

// ToCanonical maps a source-frame point into canonical board coordinates,
// undistorting first when a lens is set.
func (c BoardCalibration) ToCanonical(p geom.Pt) geom.Pt {
	return c.ideal2canon.Apply(c.lens.undistort(p))
}

// Rectify warps src into the canonical top-down board image via inverse mapping
// and bilinear sampling. Pixels that fall outside src become transparent black.
func (c BoardCalibration) Rectify(src image.Image) *image.RGBA {
	w, h := c.Board.Size()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if !c.ok {
		return dst
	}
	b := src.Bounds()
	for v := 0; v < h; v++ {
		for u := 0; u < w; u++ {
			ideal := c.canon2ideal.Apply(geom.Pt{X: float64(u), Y: float64(v)})
			sp := c.lens.distort(ideal)
			dst.SetRGBA(u, v, bilinear(src, b, sp.X, sp.Y))
		}
	}
	return dst
}

// bilinear samples src at floating-point (x,y), returning transparent black when
// the sample falls outside the image bounds.
func bilinear(src image.Image, b image.Rectangle, x, y float64) color.RGBA {
	if x < float64(b.Min.X) || x > float64(b.Max.X-1) || y < float64(b.Min.Y) || y > float64(b.Max.Y-1) {
		return color.RGBA{}
	}
	x0 := int(x)
	y0 := int(y)
	fx := x - float64(x0)
	fy := y - float64(y0)
	c00 := rgba(src.At(x0, y0))
	c10 := rgba(src.At(x0+1, y0))
	c01 := rgba(src.At(x0, y0+1))
	c11 := rgba(src.At(x0+1, y0+1))
	lerp := func(a, b, c, d float64) uint8 {
		top := a*(1-fx) + b*fx
		bot := c*(1-fx) + d*fx
		return uint8(top*(1-fy) + bot*fy + 0.5)
	}
	return color.RGBA{
		R: lerp(float64(c00.R), float64(c10.R), float64(c01.R), float64(c11.R)),
		G: lerp(float64(c00.G), float64(c10.G), float64(c01.G), float64(c11.G)),
		B: lerp(float64(c00.B), float64(c10.B), float64(c01.B), float64(c11.B)),
		A: lerp(float64(c00.A), float64(c10.A), float64(c01.A), float64(c11.A)),
	}
}

func rgba(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}
