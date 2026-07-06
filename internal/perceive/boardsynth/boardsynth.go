// Package boardsynth renders synthetic backgammon boards and warps them into
// perspective "camera" frames. It backs the perception tests today and is the
// seed of the dev-time synthetic-data pipeline the survey recommends
// (research/video-analysis-survey.md §11). Not imported by the shipped binary.
package boardsynth

import (
	"image"
	"image/color"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/geom"
	"lazybg/internal/perceive"
)

// Stack is a run of checkers of one side on a point.
type Stack struct {
	Side perceive.Side
	N    int
}

// Colors is the palette used to draw a board.
type Colors struct {
	Background color.RGBA
	A          color.RGBA // CheckerA
	B          color.RGBA // CheckerB
}

// DefaultColors is a high-contrast palette that segments cleanly.
func DefaultColors() Colors {
	return Colors{
		Background: color.RGBA{40, 120, 40, 255},
		A:          color.RGBA{245, 245, 245, 255},
		B:          color.RGBA{15, 15, 15, 255},
	}
}

// LayoutFromBoard projects an absolute board to a render layout (P1→A, P2→B).
func LayoutFromBoard(b bg.Board) map[int]Stack {
	m := make(map[int]Stack)
	for p := 1; p <= 24; p++ {
		c := b.Pts[p]
		if c.N == 0 {
			continue
		}
		side := perceive.A
		if c.Owner == bg.P2 {
			side = perceive.B
		}
		m[p] = Stack{Side: side, N: c.N}
	}
	return m
}

// Render draws a clean canonical board: filled squares (diameter = PointW)
// stacked from each point's outer edge.
func Render(cb calibrate.CanonicalBoard, layout map[int]Stack, col Colors) *image.RGBA {
	w, h := cb.Size()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	Fill(img, img.Bounds(), col.Background)
	for p, st := range layout {
		region, dir := cb.PointRegion(p)
		c := col.A
		if st.Side == perceive.B {
			c = col.B
		}
		for k := 0; k < st.N; k++ {
			var r image.Rectangle
			if dir == calibrate.StackDown {
				y0 := region.Min.Y + k*cb.PointW
				r = image.Rect(region.Min.X, y0, region.Max.X, y0+cb.PointW)
			} else {
				y1 := region.Max.Y - k*cb.PointW
				r = image.Rect(region.Min.X, y1-cb.PointW, region.Max.X, y1)
			}
			Fill(img, r, c)
		}
	}
	return img
}

// WarpToSource renders a canonical image into a w×h source frame such that the
// canonical corners land on srcCorners — the inverse of calibrate.Rectify. The
// area outside the board is filled with bg.
func WarpToSource(canon *image.RGBA, cb calibrate.CanonicalBoard, srcCorners [4]geom.Pt, w, h int, bgc color.RGBA) *image.RGBA {
	cw, ch := cb.Size()
	canonCorners := [4]geom.Pt{geom.P(0, 0), geom.P(float64(cw), 0), geom.P(float64(cw), float64(ch)), geom.P(0, float64(ch))}
	src2canon, _ := geom.Homography(srcCorners, canonCorners)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	Fill(out, out.Bounds(), bgc)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cp := src2canon.Apply(geom.P(float64(x), float64(y)))
			cx, cy := int(cp.X+0.5), int(cp.Y+0.5)
			if cx >= 0 && cx < cw && cy >= 0 && cy < ch {
				out.SetRGBA(x, y, canon.RGBAAt(cx, cy))
			}
		}
	}
	return out
}

// Fill paints a rectangle of an RGBA image a solid color.
func Fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}
