package dice

import (
	"image"
	"image/color"
	"reflect"
	"testing"
)

// pip layouts on a 3x3 grid (cell index = row*3 + col).
var pipCells = map[int][]int{
	1: {4},
	2: {0, 8},
	3: {0, 4, 8},
	4: {0, 2, 6, 8},
	5: {0, 2, 4, 6, 8},
	6: {0, 2, 3, 5, 6, 8},
}

// renderDie draws a light die face with dark pips for the given value.
func renderDie(img *image.RGBA, x0, y0, size, value int) {
	face := color.RGBA{205, 205, 200, 255}
	for y := y0; y < y0+size; y++ {
		for x := x0; x < x0+size; x++ {
			img.SetRGBA(x, y, face)
		}
	}
	pr := size / 10
	for _, cell := range pipCells[value] {
		col, row := cell%3, cell/3
		cx := x0 + (2*col+1)*size/6
		cy := y0 + (2*row+1)*size/6
		fillDisc(img, cx, cy, pr, color.RGBA{35, 35, 40, 255})
	}
}

func fillDisc(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r && image.Pt(x, y).In(img.Bounds()) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func newCanvas(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 120 // neutral background
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255 // alpha
	}
	return img
}

const size = 60

func TestReadDice_EachValue(t *testing.T) {
	for v := 1; v <= 6; v++ {
		img := newCanvas(120, 120)
		renderDie(img, 30, 30, size, v)
		got := ReadDice(img, img.Bounds(), size/10, float64(size))
		if !reflect.DeepEqual(got, []int{v}) {
			t.Errorf("value %d: read %v, want [%d]", v, got, v)
		}
	}
}

func TestReadDice_Pair(t *testing.T) {
	img := newCanvas(260, 120)
	renderDie(img, 20, 30, size, 3)
	renderDie(img, 20+size+80, 30, size, 5) // 80px gap between faces
	got := ReadDice(img, img.Bounds(), size/10, float64(size))
	if !reflect.DeepEqual(got, []int{3, 5}) {
		t.Errorf("pair: read %v, want [3 5]", got)
	}
}
