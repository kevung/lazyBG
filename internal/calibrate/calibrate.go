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

// landmarks returns the eight canonical calibration points in the order
// [TL, TR, BR, BL, barTL, barTR, barBR, barBL]: the four playing-surface corners
// (outer triangle tips) and the four bar-edge points — the canonical counterparts
// of the eight source handles (ADR-0007).
func (cb CanonicalBoard) landmarks() [8]geom.Pt {
	_, h := cb.Size()
	my, by := float64(cb.MarginY), float64(h-cb.MarginY)
	lx := float64(cb.MarginX)                 // outer left playing edge
	rx := float64(cb.columnX(11) + cb.PointW) // outer right playing edge (before the off tray)
	blx := float64(cb.MarginX + 6*cb.PointW)  // bar left edge (end of column 5)
	brx := float64(cb.columnX(6))             // bar right edge (start of column 6)
	return [8]geom.Pt{
		geom.P(lx, my), geom.P(rx, my), geom.P(rx, by), geom.P(lx, by), // TL,TR,BR,BL
		geom.P(blx, my), geom.P(brx, my), geom.P(brx, by), geom.P(blx, by), // bar edges
	}
}

// homographyPair is one half-board's canonical↔ideal-source homography.
type homographyPair struct {
	c2i geom.Mat3 // canonical pixel → undistorted (ideal) source pixel
	i2c geom.Mat3 // ideal source pixel → canonical pixel
}

// BoardCalibration maps between a source frame and the canonical board via TWO
// homographies split by the bar (ADR-0007): the left and right half-boards are
// rectified independently, so the bar width/skew is explicit and each half fits
// its own plane (tolerating the hinge fold). A migrated four-corner calibration
// collapses both halves to the same map — identical to the old single homography.
// Masks are optional canonical-space dead zones applied by RectifyMasked.
type BoardCalibration struct {
	Board  CanonicalBoard
	Masks  []image.Rectangle // canonical-space dead zones (RectifyMasked)
	left   homographyPair    // canonical x < splitX
	right  homographyPair    // canonical x >= splitX
	splitX float64           // canonical x dividing the two halves (bar centre)
	lens   Lens              // radial distortion between ideal and recorded source
	ok     bool
}

// RectifyMasked is Rectify followed by painting the calibration's declared
// dead zones — the entry point every board reader should use.
func (c BoardCalibration) RectifyMasked(src image.Image) *image.RGBA {
	out := c.Rectify(src)
	MaskZones(out, c.Masks)
	return out
}

// New builds a calibration from four source-image corners of the board, given in
// order top-left, top-right, bottom-right, bottom-left. The bar is placed at the
// canonical default fraction (the legacy single-homography behaviour, reproduced
// exactly); prefer NewSplit with explicit bar edges. No lens distortion.
func New(srcCorners [4]geom.Pt, cb CanonicalBoard) (BoardCalibration, bool) {
	return NewWithLens(srcCorners, cb, Lens{})
}

// NewWithLens is New with radial lens distortion. It migrates the four corners to
// the eight-point model by synthesising the bar edges from the canonical default
// (via the single full-quad homography), so both half-homographies collapse to
// that map — behaviour identical to the pre-ADR-0007 single homography.
func NewWithLens(srcCorners [4]geom.Pt, cb CanonicalBoard, lens Lens) (BoardCalibration, bool) {
	w, h := cb.Size()
	ideal4 := srcCorners
	if lens.active() {
		for i := range ideal4 {
			ideal4[i] = lens.undistort(srcCorners[i])
		}
	}
	fullCanon := [4]geom.Pt{geom.P(0, 0), geom.P(float64(w), 0), geom.P(float64(w), float64(h)), geom.P(0, float64(h))}
	hc2i, ok := geom.Homography(fullCanon, ideal4) // canonical → ideal source
	if !ok {
		return BoardCalibration{}, false
	}
	var ideal8 [8]geom.Pt
	for i, lm := range cb.landmarks() {
		ideal8[i] = hc2i.Apply(lm)
	}
	return buildSplit(ideal8, cb, lens)
}

// NewSplit builds a two-homography calibration from the eight source handles, in
// order [TL, TR, BR, BL, barTL, barTR, barBR, barBL]. No lens distortion.
func NewSplit(pts [8]geom.Pt, cb CanonicalBoard) (BoardCalibration, bool) {
	return NewSplitWithLens(pts, cb, Lens{})
}

// NewSplitWithLens is NewSplit with radial lens distortion: the recorded handles
// are undistorted to ideal space before fitting the two homographies.
func NewSplitWithLens(pts [8]geom.Pt, cb CanonicalBoard, lens Lens) (BoardCalibration, bool) {
	ideal := pts
	if lens.active() {
		for i := range ideal {
			ideal[i] = lens.undistort(pts[i])
		}
	}
	return buildSplit(ideal, cb, lens)
}

// NewFromHandles builds a calibration from four corners (TL,TR,BR,BL) plus an
// optional four bar edges (barTL,barTR,barBR,barBL). With bar edges it is the
// two-homography model (ADR-0007); without (nil or wrong length) it migrates the
// four corners, reproducing the legacy single-homography behaviour. This is the
// single entry point the session/transcribe build sites use.
func NewFromHandles(corners [4]geom.Pt, barEdges []geom.Pt, cb CanonicalBoard, lens Lens) (BoardCalibration, bool) {
	if len(barEdges) == 4 {
		return NewSplitWithLens([8]geom.Pt{
			corners[0], corners[1], corners[2], corners[3],
			barEdges[0], barEdges[1], barEdges[2], barEdges[3],
		}, cb, lens)
	}
	return NewWithLens(corners, cb, lens)
}

// buildSplit fits the left/right half homographies from eight IDEAL (already
// undistorted) source points against the canonical landmarks.
func buildSplit(ideal [8]geom.Pt, cb CanonicalBoard, lens Lens) (BoardCalibration, bool) {
	lm := cb.landmarks()
	// Left half quad (TL, barTL, barBL, BL); right half quad (barTR, TR, BR, barBR).
	leftCanon := [4]geom.Pt{lm[0], lm[4], lm[7], lm[3]}
	leftIdeal := [4]geom.Pt{ideal[0], ideal[4], ideal[7], ideal[3]}
	rightCanon := [4]geom.Pt{lm[5], lm[1], lm[2], lm[6]}
	rightIdeal := [4]geom.Pt{ideal[5], ideal[1], ideal[2], ideal[6]}
	lc2i, ok1 := geom.Homography(leftCanon, leftIdeal)
	li2c, ok2 := geom.Homography(leftIdeal, leftCanon)
	rc2i, ok3 := geom.Homography(rightCanon, rightIdeal)
	ri2c, ok4 := geom.Homography(rightIdeal, rightCanon)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return BoardCalibration{}, false
	}
	return BoardCalibration{
		Board:  cb,
		left:   homographyPair{lc2i, li2c},
		right:  homographyPair{rc2i, ri2c},
		splitX: (lm[4].X + lm[5].X) / 2, // bar centre
		lens:   lens,
		ok:     true,
	}, true
}

// PointRegion delegates to the canonical board.
func (c BoardCalibration) PointRegion(p int) (image.Rectangle, StackDir) {
	return c.Board.PointRegion(p)
}

// ToCanonical maps a source-frame point into canonical board coordinates,
// undistorting first when a lens is set, and choosing the half whose homography
// places it on the correct side of the bar.
func (c BoardCalibration) ToCanonical(p geom.Pt) geom.Pt {
	ideal := c.lens.undistort(p)
	if lc := c.left.i2c.Apply(ideal); lc.X <= c.splitX {
		return lc
	}
	return c.right.i2c.Apply(ideal)
}

// Rectify warps src into the canonical top-down board image via inverse mapping
// and bilinear sampling, using the left homography for canonical pixels left of
// the bar centre and the right homography otherwise. Pixels outside src become
// transparent black.
func (c BoardCalibration) Rectify(src image.Image) *image.RGBA {
	w, h := c.Board.Size()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if !c.ok {
		return dst
	}
	b := src.Bounds()
	for v := 0; v < h; v++ {
		for u := 0; u < w; u++ {
			pair := c.left
			if float64(u) >= c.splitX {
				pair = c.right
			}
			ideal := pair.c2i.Apply(geom.Pt{X: float64(u), Y: float64(v)})
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

// PointApex returns point p's triangle-tip position in canonical space: the
// column centre at the inner end of the point's quadrant. These are the
// predicted slots the correspondence fit (ADR-0008) matches detected apexes
// against. p must be in 1..24.
func (cb CanonicalBoard) PointApex(p int) geom.Pt {
	r, dir := cb.PointRegion(p)
	x := float64(r.Min.X) + float64(cb.PointW)/2
	if dir == StackDown {
		return geom.P(x, float64(r.Max.Y))
	}
	return geom.P(x, float64(r.Min.Y))
}

// ToSource maps a canonical point to the recorded source pixel — the inverse
// of ToCanonical: the half is chosen by the canonical x against the bar
// centre, and lens distortion is re-applied after the homography. This is how
// the correspondence fit (ADR-0008) predicts where each canonical landmark
// should appear in the frame.
func (c BoardCalibration) ToSource(p geom.Pt) geom.Pt {
	pair := c.left
	if p.X >= c.splitX {
		pair = c.right
	}
	return c.lens.distort(pair.c2i.Apply(p))
}

// Landmarks exposes the eight canonical calibration points in handle order
// [TL, TR, BR, BL, barTL, barTR, barBR, barBL] — what a fitted half
// homography is projected through to recover source-space handles.
func (cb CanonicalBoard) Landmarks() [8]geom.Pt { return cb.landmarks() }

// BarCenterX returns the canonical x of the bar's centre — the split line
// between the two half-board homographies.
func (cb CanonicalBoard) BarCenterX() float64 {
	r := cb.BarRegion()
	return float64(r.Min.X+r.Max.X) / 2
}
