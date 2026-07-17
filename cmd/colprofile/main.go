// Throwaway: given a manifest (+optional corner override), rectify and report
// left-felt-start, bar extent, right-felt-end at mid-height — to tune corners.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/geom"
)

func main() {
	data, _ := os.ReadFile(os.Args[1])
	m, err := corpus.Load(data)
	if err != nil { log.Fatal(err) }
	tick, _ := strconv.Atoi(os.Args[2])
	p := m.Parts[0]
	var cs [4]geom.Pt
	for i, c := range p.Calibration.Corners { cs[i] = geom.Pt{X: c[0], Y: c[1]} }
	if len(os.Args) > 3 && os.Args[3] != "" { // override: 8 nums
		f := strings.Split(os.Args[3], ",")
		for i := 0; i < 4; i++ {
			x, _ := strconv.ParseFloat(f[2*i], 64)
			y, _ := strconv.ParseFloat(f[2*i+1], 64)
			cs[i] = geom.Pt{X: x, Y: y}
		}
	}
	frame, err := capture.FrameAt(p.File, p.Span.BeginMs+tick)
	if err != nil { log.Fatal(err) }
	cb := calibrate.DefaultCanonical()
	if c := p.Calibration.Canonical; c != nil {
		cb = calibrate.CanonicalBoard{MarginX: c.MarginX, MarginY: c.MarginY, PointW: c.PointW, QuadH: c.QuadH, BarGap: c.BarGap, OffW: c.OffW}
	}
	cal, ok := calibrate.NewWithLens(cs, cb, calibrate.Lens{})
	if !ok { log.Fatal("degenerate") }
	rect := cal.Rectify(frame)
	b := rect.Bounds()
	W, H := b.Dx(), b.Dy()
	y0, y1 := H/2-20, H/2+20
	luma := func(x, y int) int { r,g,bb,_ := rect.At(x,y).RGBA(); return int((299*(r>>8)+587*(g>>8)+114*(bb>>8))/1000) }
	col := make([]int, W)
	for x := 0; x < W; x++ {
		s, n := 0, 0
		for y := y0; y < y1; y++ { s += luma(x,y); n++ }
		col[x] = s / n
	}
	dark := func(x int) bool { return col[x] < 90 }
	// left felt start: first bright after x=5
	lf := 0
	for x := 5; x < W; x++ { if col[x] > 130 { lf = x; break } }
	// right felt end: last bright before offW/edge
	rf := 0
	for x := W-1; x > 0; x-- { if col[x] > 130 { rf = x; break } }
	// bar = widest dark run in middle third
	bs, be, curS := -1, -1, -1
	for x := W/3; x < 2*W/3; x++ {
		if dark(x) { if curS < 0 { curS = x } } else {
			if curS >= 0 { if bs < 0 || x-curS > be-bs { bs, be = curS, x }; curS = -1 }
		}
	}
	// canonical expectations
	cbL := cb.MarginX
	barR := cb.BarRegion()
	fmt.Printf("corners=%v\n", cs)
	fmt.Printf("leftFelt=%d (canon marginX=%d) rightFelt=%d\n", lf, cbL, rf)
	fmt.Printf("bar=[%d,%d] center=%d width=%d (canon bar=[%d,%d] center=%d)\n",
		bs, be, (bs+be)/2, be-bs, barR.Min.X, barR.Max.X, (barR.Min.X+barR.Max.X)/2)
	fmt.Printf("leftHalf=%d rightHalf=%d (want equal)\n", bs-lf, rf-be)
}
