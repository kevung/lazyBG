package boardstate

import (
	"image"
	"image/color"
	"testing"

	"lazybg/internal/calibrate"
	"lazybg/internal/geom"
	"lazybg/internal/perceive"
	"lazybg/internal/profile"
)

var (
	bgColor  = color.RGBA{40, 120, 40, 255}   // board surface (green)
	whiteChk = color.RGBA{245, 245, 245, 255} // CheckerA
	blackChk = color.RGBA{15, 15, 15, 255}    // CheckerB
	prof     = profile.CaptureProfile{CheckerA: whiteChk, CheckerB: blackChk}
)

type stack struct {
	side perceive.Side
	n    int
}

// standard backgammon opening position, A = CheckerA (white), B = CheckerB.
func openingLayout() map[int]stack {
	return map[int]stack{
		24: {perceive.A, 2}, 13: {perceive.A, 5}, 8: {perceive.A, 3}, 6: {perceive.A, 5},
		1: {perceive.B, 2}, 12: {perceive.B, 5}, 17: {perceive.B, 3}, 19: {perceive.B, 5},
	}
}

// renderBoard draws a clean canonical board image for a layout: filled squares
// (diameter = PointW) stacked from each point's outer edge.
func renderBoard(cb calibrate.CanonicalBoard, layout map[int]stack) *image.RGBA {
	w, h := cb.Size()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), bgColor)
	for p, st := range layout {
		region, dir := cb.PointRegion(p)
		col := whiteChk
		if st.side == perceive.B {
			col = blackChk
		}
		for k := 0; k < st.n; k++ {
			var r image.Rectangle
			if dir == calibrate.StackDown {
				y0 := region.Min.Y + k*cb.PointW
				r = image.Rect(region.Min.X, y0, region.Max.X, y0+cb.PointW)
			} else {
				y1 := region.Max.Y - k*cb.PointW
				r = image.Rect(region.Min.X, y1-cb.PointW, region.Max.X, y1)
			}
			fill(img, r, col)
		}
	}
	return img
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func assertLayout(t *testing.T, ob perceive.ObservedBoard, layout map[int]stack, minConf float64) {
	t.Helper()
	for p := 1; p <= 24; p++ {
		got := ob.Points[p]
		want, occupied := layout[p]
		if !occupied {
			if got.Count != 0 || got.Side != perceive.None {
				t.Errorf("point %d: got %+v, want empty", p, got)
			}
			continue
		}
		if got.Count != want.n || got.Side != want.side {
			t.Errorf("point %d: got count=%d side=%v, want count=%d side=%v", p, got.Count, got.Side, want.n, want.side)
		}
		if got.Confidence < minConf {
			t.Errorf("point %d: confidence %.3f < %.3f", p, got.Confidence, minConf)
		}
	}
}

func TestRead_CanonicalOpeningPosition(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	img := renderBoard(cb, openingLayout())
	ob := Reader{Profile: prof}.Read(img, cb)
	assertLayout(t, ob, openingLayout(), 0.9)
}

func TestRead_EmptyBoard(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	img := renderBoard(cb, nil)
	ob := Reader{Profile: prof}.Read(img, cb)
	for p := 1; p <= 24; p++ {
		if ob.Points[p].Count != 0 {
			t.Errorf("empty board: point %d read as %+v", p, ob.Points[p])
		}
	}
}

// End-to-end through calibration: warp a canonical board into a perspective
// "camera" frame, rectify it back, and confirm the reader still recovers the
// counts (the calibrated-classical path the MVP relies on).
func TestRead_ThroughPerspectiveRectify(t *testing.T) {
	cb := calibrate.DefaultCanonical()
	layout := map[int]stack{24: {perceive.A, 2}, 6: {perceive.A, 5}, 1: {perceive.B, 3}, 13: {perceive.B, 5}}
	canon := renderBoard(cb, layout)

	// Build a source frame in which the board occupies a slanted quad.
	srcCorners := [4]geom.Pt{geom.P(80, 60), geom.P(900, 40), geom.P(950, 760), geom.P(30, 700)}
	src := warpToSource(canon, cb, srcCorners, 1000, 820)

	cal, ok := calibrate.New(srcCorners, cb)
	if !ok {
		t.Fatal("calibration failed")
	}
	rect := cal.Rectify(src)
	ob := Reader{Profile: prof}.Read(rect, cb)
	assertLayout(t, ob, layout, 0.6) // some edge blur from resampling → lower conf bar
}

// warpToSource renders the canonical image into a source frame such that the
// canonical corners land on srcCorners (the inverse of Rectify).
func warpToSource(canon *image.RGBA, cb calibrate.CanonicalBoard, srcCorners [4]geom.Pt, w, h int) *image.RGBA {
	cw, ch := cb.Size()
	canonCorners := [4]geom.Pt{geom.P(0, 0), geom.P(float64(cw), 0), geom.P(float64(cw), float64(ch)), geom.P(0, float64(ch))}
	src2canon, _ := geom.Homography(srcCorners, canonCorners)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(out, out.Bounds(), bgColor)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cp := src2canon.Apply(geom.Pt{X: float64(x), Y: float64(y)})
			cx, cy := int(cp.X+0.5), int(cp.Y+0.5)
			if cx >= 0 && cx < cw && cy >= 0 && cy < ch {
				out.SetRGBA(x, y, canon.RGBAAt(cx, cy))
			}
		}
	}
	return out
}
