// Throwaway debug tool: render the rectified canonical board for a manifest.
package main

import (
	"fmt"
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
	cal, ok := calibrate.New(cs, cb)
	if !ok {
		log.Fatal("degenerate")
	}
	out := cal.Rectify(frame)
	f, _ := os.Create(os.Args[3])
	png.Encode(f, out)
	f.Close()
	fmt.Println("wrote", os.Args[3])
}
