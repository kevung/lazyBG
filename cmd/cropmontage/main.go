// Throwaway: lay the 24 PointRegion crops in canonical board arrangement.
// Top row p13..p24 (left→right), bottom row p12..p1 (left→right).
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"strconv"

	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/geom"
)

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
	rect := cal.Rectify(frame)

	cw, chh := cb.PointW, cb.QuadH
	pad := 6
	cols, rows := 12, 2
	W := cols*(cw+pad) + pad
	H := rows*(chh+pad) + pad
	out := image.NewRGBA(image.Rect(0, 0, W, H))
	draw.Draw(out, out.Bounds(), &image.Uniform{color.RGBA{40, 40, 40, 255}}, image.Point{}, draw.Src)

	place := func(col, row, pt int) {
		r, _ := cb.PointRegion(pt)
		dstX := pad + col*(cw+pad)
		dstY := pad + row*(chh+pad)
		dst := image.Rect(dstX, dstY, dstX+cw, dstY+chh)
		draw.Draw(out, dst, rect, r.Min, draw.Src)
	}
	// top row: p13..p24 at cols 0..11
	for c := 0; c < 12; c++ {
		place(c, 0, 13+c)
	}
	// bottom row: p12..p1 at cols 0..11
	for c := 0; c < 12; c++ {
		place(c, 1, 12-c)
	}
	f, _ := os.Create(os.Args[3])
	png.Encode(f, out)
	f.Close()
}
