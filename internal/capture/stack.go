package capture

import (
	"image"
	"sort"
)

// MedianStack combines same-sized frames into one image by taking the
// per-pixel, per-channel median. For a subject that holds still across the
// stack (a settled die) this averages away compression noise while a strict
// mean would not survive a transient occluder (a hand); the median ignores
// any minority of outlier frames. All images must share identical bounds;
// the result is re-based at (0,0). Returns nil for an empty stack.
func MedianStack(imgs []*image.RGBA) *image.RGBA {
	if len(imgs) == 0 {
		return nil
	}
	b := imgs[0].Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	vals := make([]uint8, len(imgs))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			for c := 0; c < 4; c++ {
				for i, img := range imgs {
					ib := img.Bounds()
					vals[i] = img.Pix[img.PixOffset(ib.Min.X+x, ib.Min.Y+y)+c]
				}
				sort.Slice(vals, func(a, b int) bool { return vals[a] < vals[b] })
				out.Pix[out.PixOffset(x, y)+c] = vals[len(vals)/2]
			}
		}
	}
	return out
}
