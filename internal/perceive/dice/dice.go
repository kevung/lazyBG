// Package dice reads dice values (pip counts) from a rectified frame by the
// classical recipe the detector survey recommends
// (docs/research/perception-detector-survey.md, question D): detect pips as small
// circles, cluster the pip centres into individual dice, and count pips per
// cluster. Pip detection reuses the shape-based circle detector (a pip is just a
// small disc), so it is colour-agnostic like the checker reader. The .mat
// supplies the value for free as a cross-check / training label; localisation is
// constrained to a declared dice-tray ROI (a Session Prior).
package dice

import (
	"image"
	"image/color"

	"lazybg/internal/perceive/checker"
)

// ReadPips returns the centres (in img coordinates) of pip-sized discs found in
// roi. pipRadius is the pip radius in pixels (a calibration value).
func ReadPips(img image.Image, roi image.Rectangle, pipRadius int) []image.Point {
	roi = roi.Intersect(img.Bounds())
	g := image.NewGray(image.Rect(0, 0, roi.Dx(), roi.Dy()))
	for y := 0; y < roi.Dy(); y++ {
		for x := 0; x < roi.Dx(); x++ {
			r, gg, b, _ := img.At(roi.Min.X+x, roi.Min.Y+y).RGBA()
			g.SetGray(x, y, color.Gray{Y: uint8((299*(r>>8) + 587*(gg>>8) + 114*(b>>8)) / 1000)})
		}
	}
	var pts []image.Point
	for _, c := range checker.Detect(g, pipRadius) {
		pts = append(pts, image.Pt(roi.Min.X+c.X, roi.Min.Y+c.Y))
	}
	return pts
}

// ReadDice detects pips in roi and clusters them into dice, returning each die's
// value (pip count, clamped to 1..6) sorted ascending. eps is the single-linkage
// distance (px) that groups pips of the same die — roughly the die face size: it
// must exceed the within-die pip span yet stay below the gap between dice (a
// calibration value). The .mat's known dice count/values are a strong downstream
// cross-check when clustering is ambiguous.
func ReadDice(img image.Image, roi image.Rectangle, pipRadius int, eps float64) []int {
	pts := ReadPips(img, roi, pipRadius)
	var vals []int
	for _, cl := range cluster(pts, eps) {
		v := len(cl)
		if v >= 1 && v <= 6 {
			vals = append(vals, v)
		}
	}
	sortInts(vals)
	return vals
}

// cluster groups points by single-linkage: two points join the same cluster if
// within eps of each other (transitively).
func cluster(pts []image.Point, eps float64) [][]image.Point {
	n := len(pts)
	seen := make([]bool, n)
	eps2 := eps * eps
	var out [][]image.Point
	for i := 0; i < n; i++ {
		if seen[i] {
			continue
		}
		queue := []int{i}
		seen[i] = true
		var grp []image.Point
		for len(queue) > 0 {
			j := queue[0]
			queue = queue[1:]
			grp = append(grp, pts[j])
			for k := 0; k < n; k++ {
				if seen[k] {
					continue
				}
				dx := float64(pts[j].X - pts[k].X)
				dy := float64(pts[j].Y - pts[k].Y)
				if dx*dx+dy*dy <= eps2 {
					seen[k] = true
					queue = append(queue, k)
				}
			}
		}
		out = append(out, grp)
	}
	return out
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}
