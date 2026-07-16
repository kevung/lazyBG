package main

import (
	"fmt"
	"image"
	"math/rand"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/geom"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/perceive/pointnet"
)

// searchCorners random-restarts + hill-climbs the 8 corner coordinates to
// maximize the learned-reader start-board match on an opening frame.
func searchCorners(frames []image.Image, init [4]geom.Pt, cb calibrate.CanonicalBoard, net *pointnet.Net) ([4]geom.Pt, float64, float64) {
	frame := frames[0]
	b := frame.Bounds()
	mkLens := func(k1 float64) calibrate.Lens {
		return calibrate.Lens{K1: k1,
			CenterX: float64(b.Dx()) / 2, CenterY: float64(b.Dy()) / 2,
			Norm: float64(b.Dx()) / 2}
	}
	score := func(cs [4]geom.Pt, k1 float64) float64 {
		cal, ok := calibrate.NewWithLens(cs, cb, mkLens(k1))
		if !ok {
			return -1
		}
		tot := 0.0
		for _, f := range frames {
			obs := pointnet.Reader{Net: net}.Read(cal.Rectify(f), cb)
			tot += boarddiff.WholeBoardMatch(bg.StandardStart(), obs)
		}
		return tot / float64(len(frames))
	}
	// coarse K1 sweep first: barrel distortion is the dominant unknown
	best, bestK, bestS := init, 0.0, score(init, 0)
	fmt.Printf("init score %.3f (k1=0)\n", bestS)
	for _, k1 := range []float64{-0.30, -0.24, -0.18, -0.12, -0.06, 0.06} {
		if s := score(init, k1); s > bestS {
			best, bestK, bestS = init, k1, s
			fmt.Printf("k1 %.2f: %.3f\n", k1, s)
		}
	}
	rng := rand.New(rand.NewSource(7))
	cur, curK, curS := best, bestK, bestS
	for step := 0; step < 150; step++ {
		cand, candK := cur, curK
		amp := 12.0 * (1 - float64(step)/170)
		for c := 0; c < 4; c++ {
			if rng.Intn(2) == 0 {
				cand[c].X += (rng.Float64()*2 - 1) * amp
				cand[c].Y += (rng.Float64()*2 - 1) * amp
			}
		}
		if rng.Intn(3) == 0 {
			candK += (rng.Float64()*2 - 1) * 0.03 * (1 - float64(step)/170)
		}
		if s := score(cand, candK); s > curS {
			cur, curK, curS = cand, candK, s
			if s > bestS {
				best, bestK, bestS = cand, candK, s
				fmt.Printf("step %3d: %.3f k1=%.3f %v\n", step, s, candK, cand)
			}
		}
	}
	return best, bestK, bestS
}
