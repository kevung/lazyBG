// Throwaway debug: read the board at a tick with the learned reader and
// score it against the truth start + first turns.
package main

import (
	"image"
	"fmt"
	"log"
	"os"
	"strconv"

	"lazybg/internal/align"
	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/geom"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/perceive/pointnet"
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
	net, err := pointnet.Load(os.Args[3])
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 4 && os.Args[4] == "search" {
		frames := []image.Image{frame}
		for _, dt := range []int{2000, 4000} {
			if f2, err := capture.FrameAt(p.File, p.Span.BeginMs+tick+dt); err == nil {
				frames = append(frames, f2)
			}
		}
		bestCs, k1, s := searchCorners(frames, cs, cb, net)
		fmt.Printf("best %.3f k1=%.3f corners %v\n", s, k1, bestCs)
		b := frame.Bounds()
		cal, _ = calibrate.NewWithLens(bestCs, cb, calibrate.Lens{K1: k1,
			CenterX: float64(b.Dx()) / 2, CenterY: float64(b.Dy()) / 2,
			Norm: float64(b.Dx()) / 2})
	}
	rect := cal.Rectify(frame)
	obs := pointnet.Reader{Net: net}.Read(rect, cb)
	fmt.Println("observed:")
	for pt := 1; pt <= 24; pt++ {
		o := obs.Points[pt]
		fmt.Printf("  pt%2d: side=%d n=%d conf=%.2f\n", pt, o.Side, o.Count, o.Confidence)
	}
	fmt.Printf("start-board match: %.3f\n", boarddiff.WholeBoardMatch(bg.StandardStart(), obs))
	sw := obs
	for pt := 1; pt <= 24; pt++ {
		if sw.Points[pt].Side == perceive.A {
			sw.Points[pt].Side = perceive.B
		} else if sw.Points[pt].Side == perceive.B {
			sw.Points[pt].Side = perceive.A
		}
	}
	fmt.Printf("start-board match SIDE-SWAPPED: %.3f\n", boarddiff.WholeBoardMatch(bg.StandardStart(), sw))
	matB, _ := os.ReadFile(m.Transcript)
	truth, err := matimport.Parse(string(matB))
	if err != nil {
		log.Fatal(err)
	}
	turns := align.TruthTurns(truth)
	for k := 0; k < 6 && k < len(turns); k++ {
		fmt.Printf("turn %d (%s) match: %.3f\n", turns[k].Index, turns[k].Notation,
			boarddiff.WholeBoardMatch(turns[k].Board, obs))
	}
	bestK, bestS := -1, -1.0
	for k := range turns {
		if s := boarddiff.WholeBoardMatch(turns[k].Board, obs); s > bestS {
			bestK, bestS = k, s
		}
	}
	if bestK >= 0 {
		fmt.Printf("BEST truth turn: idx %d (%s) match %.3f\n", turns[bestK].Index, turns[bestK].Notation, bestS)
	}
}
