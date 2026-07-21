package autocal

import (
	"math"
	"sort"

	"lazybg/internal/calibrate"
	"lazybg/internal/geom"
)

// bootstrapMatches assigns detected apexes to canonical slots WITHOUT any
// seed calibration — the rescue path when the mask quad went wild (a rotated
// principal axis produces a diamond seed hundreds of px off, ADR-0008's real
// corpus failure mode). Each apex already knows its row (a top-row triangle
// points down); within a row, apexes are ordered along the row's principal
// direction and aligned to the 12 canonical column positions by trying every
// (leftmost, rightmost)-slot hypothesis — the bar's wide gap makes the true
// alignment score far better than any shifted one, which anchors ABSOLUTE
// column indices even with outer triangles missing. Slot keys are point-1
// (0..23); values index into apexes.
func bootstrapMatches(apexes []Apex, cb calibrate.CanonicalBoard) (map[int]int, bool) {
	var rows [2][]int // 0 = top row (Down apexes), 1 = bottom row
	for i, a := range apexes {
		if a.Down {
			rows[0] = append(rows[0], i)
		} else {
			rows[1] = append(rows[1], i)
		}
	}
	if len(rows[0]) < 5 || len(rows[1]) < 5 {
		return nil, false
	}

	// Canonical apex x per column (same for both rows).
	var canonX [12]float64
	for c := 0; c < 12; c++ {
		canonX[c] = cb.PointApex(13 + c).X
	}

	type rowFit struct {
		cols  map[int]int // column → apex index
		barPt geom.Pt     // image point of the bar centre on this row's axis
		pitch float64     // image px per canonical PointW
	}
	fitRow := func(idxs []int) (rowFit, bool) {
		// Principal direction of the row's apexes.
		var mx, my float64
		for _, i := range idxs {
			mx += apexes[i].Pt.X / float64(len(idxs))
			my += apexes[i].Pt.Y / float64(len(idxs))
		}
		var sxx, sxy, syy float64
		for _, i := range idxs {
			dx, dy := apexes[i].Pt.X-mx, apexes[i].Pt.Y-my
			sxx += dx * dx
			sxy += dx * dy
			syy += dy * dy
		}
		theta := 0.5 * math.Atan2(2*sxy, sxx-syy)
		ux, uy := math.Cos(theta), math.Sin(theta)
		if ux < 0 { // keep the axis pointing image-rightward: column order
			ux, uy = -ux, -uy
		}
		t := make([]float64, len(idxs))
		order := make([]int, len(idxs))
		for k, i := range idxs {
			t[k] = (apexes[i].Pt.X-mx)*ux + (apexes[i].Pt.Y-my)*uy
			order[k] = k
		}
		sort.Slice(order, func(a, b int) bool { return t[order[a]] < t[order[b]] })

		n := len(idxs)
		best, second := math.Inf(1), math.Inf(1)
		var bestLo, bestHi int
		for lo := 0; lo < 12; lo++ {
			for hi := lo + n - 1; hi < 12; hi++ {
				tLo, tHi := t[order[0]], t[order[n-1]]
				a := (tHi - tLo) / (canonX[hi] - canonX[lo])
				if a <= 0 {
					continue
				}
				b := tLo - a*canonX[lo]
				// Assign every apex to its nearest column under (a,b).
				used := map[int]bool{}
				score := 0.0
				valid := true
				for _, k := range order {
					bc, bd := -1, math.Inf(1)
					for c := 0; c < 12; c++ {
						if d := math.Abs(t[k] - (a*canonX[c] + b)); d < bd {
							bc, bd = c, d
						}
					}
					if used[bc] {
						valid = false
						break
					}
					used[bc] = true
					r := bd / (a * float64(cb.PointW)) // pitch-relative residual
					score += r * r
				}
				if !valid {
					continue
				}
				if score < best {
					second = best
					best, bestLo, bestHi = score, lo, hi
				} else if score < second {
					second = score
				}
			}
		}
		if math.IsInf(best, 1) || best > 0.05*float64(n) {
			return rowFit{}, false // no hypothesis tracks the grid tightly
		}
		if second < 2*best+1e-9 {
			return rowFit{}, false // ambiguous alignment: refuse to guess
		}
		// Rebuild the winning assignment.
		a := (t[order[n-1]] - t[order[0]]) / (canonX[bestHi] - canonX[bestLo])
		b := t[order[0]] - a*canonX[bestLo]
		cols := map[int]int{}
		for _, k := range order {
			bc, bd := -1, math.Inf(1)
			for c := 0; c < 12; c++ {
				if d := math.Abs(t[k] - (a*canonX[c] + b)); d < bd {
					bc, bd = c, d
				}
			}
			cols[bc] = idxs[k]
		}
		tBar := a*cb.BarCenterX() + b
		return rowFit{
			cols:  cols,
			barPt: geom.P(mx+tBar*ux, my+tBar*uy),
			pitch: a * float64(cb.PointW),
		}, true
	}

	top, okT := fitRow(rows[0])
	bot, okB := fitRow(rows[1])
	if !okT || !okB {
		return nil, false
	}
	// Cross-row consistency: both rows must place the bar at (nearly) the
	// same image abscissa — a shifted indexing on one row fails this.
	if math.Abs(top.barPt.X-bot.barPt.X) > 0.75*math.Max(top.pitch, bot.pitch) {
		return nil, false
	}

	m := map[int]int{}
	for c, det := range top.cols {
		m[(13+c)-1] = det // top row, left→right 13..24
	}
	for c, det := range bot.cols {
		m[(12-c)-1] = det // bottom row, left→right 12..1
	}
	return m, true
}
