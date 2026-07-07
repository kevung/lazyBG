// Package eval scores perception output against .mat-derived ground truth
// (docs/experiment-plan.md §6). This file covers the per-cue board-state metric:
// per-point accuracy and full-board exact-match, comparing an ObservedBoard (a
// noisy reading) to the true bg.Board. The primary effort-saved metrics
// (auto-fill coverage / error / review rate) come once the commit detector and
// fusion drive whole-Recording runs; this per-cue metric is available now and
// quantifies reader quality per frame (feeding the classical-vs-learned decision).
package eval

import (
	"lazybg/internal/bg"
	"lazybg/internal/perceive"
)

// BoardResult is the per-point comparison of a reading to the truth board.
type BoardResult struct {
	Points  int // points compared (24)
	Correct int // points whose count AND owning side match truth
}

// PerPoint is the fraction of points read exactly (count + side).
func (r BoardResult) PerPoint() float64 {
	if r.Points == 0 {
		return 0
	}
	return float64(r.Correct) / float64(r.Points)
}

// Exact reports whether every point matched — the survey's full-board metric,
// which is far rarer than per-point (do not target it from vision alone).
func (r BoardResult) Exact() bool { return r.Points > 0 && r.Correct == r.Points }

// ScoreBoard compares an ObservedBoard to the derived truth board over points
// 1..24. A point matches when the observed count and side equal the truth
// (empty truth points must read empty). Owner mapping: P1→A, P2→B.
func ScoreBoard(obs perceive.ObservedBoard, truth bg.Board) BoardResult {
	r := BoardResult{Points: 24}
	for p := 1; p <= 24; p++ {
		tp := truth.Pts[p]
		wantSide := perceive.None
		if tp.N > 0 {
			wantSide = perceive.A
			if tp.Owner == bg.P2 {
				wantSide = perceive.B
			}
		}
		g := obs.Points[p]
		gotSide := g.Side
		if g.Count == 0 {
			gotSide = perceive.None
		}
		if g.Count == tp.N && gotSide == wantSide {
			r.Correct++
		}
	}
	return r
}
