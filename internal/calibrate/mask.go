package calibrate

import (
	"image"
	"sort"
)

// MaskZones paints declared dead zones of a rectified board with the image's
// median color (the felt dominates a rectified board, so the median is a
// neutral fill). Zones are canonical-space rectangles declared per capture in
// the manifest — rail areas where players park spare checkers, a clock or
// score card intruding over the frame — content that is physically present
// inside the calibrated quad but is not game state (world-model dead zones).
func MaskZones(img *image.RGBA, zones []image.Rectangle) {
	if len(zones) == 0 {
		return
	}
	fill := medianColor(img)
	for _, z := range zones {
		z = z.Intersect(img.Bounds())
		for y := z.Min.Y; y < z.Max.Y; y++ {
			for x := z.Min.X; x < z.Max.X; x++ {
				i := img.PixOffset(x, y)
				img.Pix[i], img.Pix[i+1], img.Pix[i+2] = fill[0], fill[1], fill[2]
			}
		}
	}
}

func medianColor(img *image.RGBA) [3]uint8 {
	b := img.Bounds()
	// sample a coarse grid: exact per-pixel medians are not worth the pass
	var ch [3][]uint8
	for y := b.Min.Y; y < b.Max.Y; y += 7 {
		for x := b.Min.X; x < b.Max.X; x += 7 {
			i := img.PixOffset(x, y)
			ch[0] = append(ch[0], img.Pix[i])
			ch[1] = append(ch[1], img.Pix[i+1])
			ch[2] = append(ch[2], img.Pix[i+2])
		}
	}
	var out [3]uint8
	for c := range ch {
		sort.Slice(ch[c], func(a, b int) bool { return ch[c][a] < ch[c][b] })
		out[c] = ch[c][len(ch[c])/2]
	}
	return out
}
