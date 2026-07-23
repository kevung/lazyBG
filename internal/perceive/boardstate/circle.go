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

// DiscParams is the disc-detector tuning the shipped readers run with (the value
// both the transcribe runner and auto-calibration pass). Exported so the GUI's
// perception overlay draws the discs the pipeline actually sees rather than a
// differently-tuned second opinion.
var DiscParams = checker.Params{PeakFrac: 0.38}

// discMargin pads a point region before detection so a disc whose rim straddles
// the region edge still gathers its votes.
const discMargin = 4

// DetectDiscs returns the checker discs found in each point's stack region,
// indexed 1..24, in img (rectified) coordinates.
//
// It detects region by region rather than in one board-wide pass on purpose.
// checker's thresholds are relative to the image it is handed — rim candidates
// at EdgeFrac × the max gradient, centres at PeakFrac × the max accumulator — so
// a board-wide pass lets the single highest-contrast disc set the bar for the
// whole board and silence every dimmer one (measured on the pilot's settled
// opening: 21 discs for 30 checkers, four of them on no point at all). Per
// region, "contrast-relative" means relative to that point's own contrast, which
// is the behaviour the detector was designed for.
//
// Detections are binned back to the region that owns them, so the padding never
// reports a disc twice.
func DetectDiscs(img image.Image, cb calibrate.CanonicalBoard, radius int, p checker.Params) [25][]checker.Circle {
	var out [25][]checker.Circle
	for pt := 1; pt <= 24; pt++ {
		region, _ := cb.PointRegion(pt)
		out[pt] = detectInRegion(img, region, radius, p)
	}
	return out
}

// detectInRegion runs the disc detector over one padded point region and returns
// the centres that belong to it, in img coordinates.
func detectInRegion(img image.Image, region image.Rectangle, radius int, p checker.Params) []checker.Circle {
	crop := region.Inset(-discMargin).Intersect(img.Bounds())
	var out []checker.Circle
	for _, c := range checker.DetectWith(toGray(img, crop), radius, p) {
		ix, iy := crop.Min.X+c.X, crop.Min.Y+c.Y
		if !image.Pt(ix, iy).In(region) { // bin to this point only
			continue
		}
		out = append(out, checker.Circle{X: ix, Y: iy, Score: c.Score})
	}
	return out
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

// readPoint detects discs within a point's region and assigns the owning side by
// the majority of the detected discs' centre colours.
func (cr CircleReader) readPoint(img image.Image, region image.Rectangle, r int) perceive.PointObs {
	var aVotes, bVotes int
	for _, c := range detectInRegion(img, region, r, cr.Params) {
		if nearestSide(img.At(c.X, c.Y), cr.Profile) == perceive.A {
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
