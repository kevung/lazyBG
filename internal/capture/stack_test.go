package capture

import (
	"image"
	"image/color"
	"testing"
)

func flat(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// The median of an odd stack must ignore a single outlier frame — the
// property that makes stacking robust to a hand passing over the die.
func TestMedianStackRejectsOutlier(t *testing.T) {
	steady := color.RGBA{100, 150, 200, 255}
	occluded := color.RGBA{10, 10, 10, 255}
	out := MedianStack([]*image.RGBA{
		flat(4, 4, steady),
		flat(4, 4, occluded),
		flat(4, 4, steady),
	})
	if got := out.RGBAAt(2, 2); got != steady {
		t.Fatalf("median = %v, want steady %v", got, steady)
	}
}

// With noisy-but-centered samples the median lands on the middle value —
// the denoising effect that should sharpen 3-5px pips.
func TestMedianStackPicksMiddle(t *testing.T) {
	out := MedianStack([]*image.RGBA{
		flat(2, 2, color.RGBA{90, 90, 90, 255}),
		flat(2, 2, color.RGBA{100, 100, 100, 255}),
		flat(2, 2, color.RGBA{110, 110, 110, 255}),
		flat(2, 2, color.RGBA{95, 95, 95, 255}),
		flat(2, 2, color.RGBA{105, 105, 105, 255}),
	})
	want := color.RGBA{100, 100, 100, 255}
	if got := out.RGBAAt(0, 0); got != want {
		t.Fatalf("median = %v, want %v", got, want)
	}
}

func TestMedianStackSingleAndEmpty(t *testing.T) {
	c := color.RGBA{1, 2, 3, 255}
	if got := MedianStack([]*image.RGBA{flat(1, 1, c)}).RGBAAt(0, 0); got != c {
		t.Fatalf("single-frame stack = %v, want identity %v", got, c)
	}
	if MedianStack(nil) != nil {
		t.Fatal("empty stack should return nil")
	}
}
