// Throwaway: overlay the 24 PointRegion cells (+ bar) on the rectified board.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"strconv"

	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/geom"
)

func rectBorder(img *image.RGBA, r image.Rectangle, c color.RGBA, th int) {
	for t := 0; t < th; t++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, r.Min.Y+t, c)
			img.SetRGBA(x, r.Max.Y-1-t, c)
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			img.SetRGBA(r.Min.X+t, y, c)
			img.SetRGBA(r.Max.X-1-t, y, c)
		}
	}
}

func main() {
	data, _ := os.ReadFile(os.Args[1])
	m, err := corpus.Load(data)
	if err != nil {
		log.Fatal(err)
	}
	tick, _ := strconv.Atoi(os.Args[2])
	p := m.Parts[0]
	frame, err := capture.FrameAt(p.File, p.Span.BeginMs+tick)
	if err != nil {
		log.Fatal(err)
	}
	var cs [4]geom.Pt
	for i, c := range p.Calibration.Corners {
		cs[i] = geom.Pt{X: c[0], Y: c[1]}
	}
	cb := calibrate.DefaultCanonical()
	if c := p.Calibration.Canonical; c != nil {
		cb = calibrate.CanonicalBoard{MarginX: c.MarginX, MarginY: c.MarginY,
			PointW: c.PointW, QuadH: c.QuadH, BarGap: c.BarGap, OffW: c.OffW}
	}
	lens := calibrate.Lens{}
	if l := p.Calibration.Lens; l != nil {
		lens = calibrate.Lens{K1: l.K1, CenterX: l.CenterX, CenterY: l.CenterY, Norm: l.Norm}
	}
	cal, ok := calibrate.NewWithLens(cs, cb, lens)
	if !ok {
		log.Fatal("degenerate")
	}
	out := cal.Rectify(frame)
	// Draw point cells. Top points (13..24) green, bottom (1..12) blue.
	for pt := 1; pt <= 24; pt++ {
		r, _ := cb.PointRegion(pt)
		col := color.RGBA{0, 255, 0, 255}
		if pt <= 12 {
			col = color.RGBA{0, 200, 255, 255}
		}
		rectBorder(out, r, col, 2)
	}
	rectBorder(out, cb.BarRegion(), color.RGBA{255, 0, 0, 255}, 2)
	f, _ := os.Create(os.Args[3])
	png.Encode(f, out)
	f.Close()
}
