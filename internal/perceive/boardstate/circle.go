package boardstate

import (
	"image"
	"image/color"

	"lazybg/internal/calibrate"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/checker"
	"lazybg/internal/profile"
)

// CircleReader reads an ObservedBoard by DETECTING checker discs (shape), then
// using colour only to assign each disc's owner — the calibrated-classical
// design the detector survey recommends (docs/research/perception-detector-survey.md).
// Unlike the colour-centreline Reader, it does not rely on colour to separate
// checkers from felt, so it survives low-contrast and marbled checker sets.
type CircleReader struct {
	Profile profile.CaptureProfile
	Radius  int            // checker radius in rectified px; 0 -> cb.PointW/2
	Params  checker.Params // detector tuning; zero value uses checker defaults
}

// Read produces an ObservedBoard for points 1..24 from the rectified image.
func (cr CircleReader) Read(img image.Image, cb calibrate.CanonicalBoard) perceive.ObservedBoard {
	r := cr.Radius
	if r == 0 {
		r = cb.PointW / 2
	}
	var ob perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		region, _ := cb.PointRegion(p)
		ob.Points[p] = cr.readPoint(img, region, r)
	}
	return ob
}

// readPoint detects discs within a point's region (padded so rim pixels at the
// region edge still vote) and assigns the owning side by the majority of the
// detected discs' centre colours.
func (cr CircleReader) readPoint(img image.Image, region image.Rectangle, r int) perceive.PointObs {
	const margin = 4
	crop := region.Inset(-margin).Intersect(img.Bounds())
	gray := toGray(img, crop)
	circles := checker.DetectWith(gray, r, cr.Params)

	var aVotes, bVotes int
	for _, c := range circles {
		ix, iy := crop.Min.X+c.X, crop.Min.Y+c.Y
		if !image.Pt(ix, iy).In(region) { // bin to this point only
			continue
		}
		if nearestSide(img.At(ix, iy), cr.Profile) == perceive.A {
			aVotes++
		} else {
			bVotes++
		}
	}
	n := aVotes + bVotes
	if n == 0 {
		return perceive.PointObs{Count: 0, Side: perceive.None, Confidence: 1}
	}
	if aVotes >= bVotes {
		return perceive.PointObs{Count: n, Side: perceive.A, Confidence: 1}
	}
	return perceive.PointObs{Count: n, Side: perceive.B, Confidence: 1}
}

// toGray extracts a rectangle of img as an origin-based grayscale image (Rec.601).
func toGray(img image.Image, r image.Rectangle) *image.Gray {
	g := image.NewGray(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			cr, cg, cb, _ := img.At(r.Min.X+x, r.Min.Y+y).RGBA()
			luma := (299*(cr>>8) + 587*(cg>>8) + 114*(cb>>8)) / 1000
			g.SetGray(x, y, color.Gray{Y: uint8(luma)})
		}
	}
	return g
}

// nearestSide assigns a colour to the nearer of the two declared checker colours.
// Reuses colorDist from the package's centreline Reader.
func nearestSide(c color.Color, p profile.CaptureProfile) perceive.Side {
	if colorDist(c, p.CheckerA) <= colorDist(c, p.CheckerB) {
		return perceive.A
	}
	return perceive.B
}
