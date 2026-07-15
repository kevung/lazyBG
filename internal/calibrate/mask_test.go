package calibrate

import (
	"image"
	"image/color"
	"testing"
)

// Declared dead zones (rail parking, a clock intruding over the frame) are
// painted with the image median so no reader ever sees their content.
func TestMaskZones(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	felt := color.RGBA{180, 178, 170, 255}
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			img.SetRGBA(x, y, felt)
		}
	}
	// a "spare checker" in the corner
	img.SetRGBA(1, 1, color.RGBA{250, 250, 250, 255})
	img.SetRGBA(2, 1, color.RGBA{250, 250, 250, 255})
	MaskZones(img, []image.Rectangle{image.Rect(0, 0, 4, 3)})
	if got := img.RGBAAt(1, 1); got != felt {
		t.Fatalf("zone not painted with median: %v", got)
	}
	if got := img.RGBAAt(10, 5); got != felt {
		t.Fatalf("outside zone must be untouched: %v", got)
	}
}
