package autocal

import (
	"math"
	"sort"

	"lazybg/internal/calibrate"
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
	// Strict first (no outlier tolerance, unambiguous rows only): captures
	// that lock strictly must keep locking exactly as before — the tolerant
	// variant exists to ADD locks, never to shadow good ones (the ratchet
	// caught it doing exactly that).
	if m, ok := bootstrapMode(apexes, cb, true); ok {
		return m, ok
	}
	return bootstrapMode(apexes, cb, false)
}

func bootstrapMode(apexes []Apex, cb calibrate.CanonicalBoard, strict bool) (map[int]int, bool) {
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

	type rowCand struct {
		cols  map[int]int // column → apex index
		barT  float64     // 1D param of the bar centre under this alignment
		score float64
	}
	type rowFit struct {
		cands          []rowCand // plausible alignments, best first
		mx, my, ux, uy float64
		pitch          float64 // image px per canonical PointW (approx.)
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

		// assign maps each apex to its nearest column under the 1D map
		// tHat(x). Collisions drop the FARTHER apex (a spurious component
		// must not invalidate the whole hypothesis) at a fixed score
		// penalty; the survivors' residuals are pitch-relative.
		const outlierPenalty = 0.20
		maxOutliers := 1
		if strict {
			maxOutliers = 0
		}
		assign := func(tHat func(float64) float64, scale float64) (map[int]int, float64, int) {
			cols := map[int]int{} // col → k (index into t/order space)
			resid := map[int]float64{}
			outliers := 0
			for _, k := range order {
				bc, bd := -1, math.Inf(1)
				for c := 0; c < 12; c++ {
					if d := math.Abs(t[k] - tHat(canonX[c])); d < bd {
						bc, bd = c, d
					}
				}
				r := bd / scale
				if prev, taken := cols[bc]; taken {
					if r < resid[bc] {
						cols[bc] = k
						resid[bc] = r
					}
					_ = prev
					outliers++
					continue
				}
				cols[bc] = k
				resid[bc] = r
			}
			score := outlierPenalty * float64(outliers)
			rs := make([]int, 0, len(resid))
			for c := range resid {
				rs = append(rs, c)
			}
			sort.Ints(rs)
			for _, c := range rs {
				score += resid[c] * resid[c]
			}
			return cols, score, outliers
		}
		// projective1D least-squares fits t ≈ (a·x + b) / (1 + c·x) on the
		// assigned pairs — a linear system in (a,b,c) — absorbing the
		// along-row perspective a plain affine map cannot (its residual is
		// what used to push real captures over the acceptance gate).
		projective1D := func(cols map[int]int) (func(float64) float64, bool) {
			if len(cols) < 4 {
				return nil, false
			}
			var m [3][3]float64
			var rhs [3]float64
			ks := make([]int, 0, len(cols))
			for c := range cols {
				ks = append(ks, c)
			}
			sort.Ints(ks)
			for _, c := range ks {
				k := cols[c]
				x, tv := canonX[c], t[k]
				row := [3]float64{x, 1, -tv * x}
				for i := 0; i < 3; i++ {
					for j := 0; j < 3; j++ {
						m[i][j] += row[i] * row[j]
					}
					rhs[i] += row[i] * tv
				}
			}
			// Solve the 3×3 system by elimination.
			a := [3][4]float64{}
			for i := 0; i < 3; i++ {
				copy(a[i][:3], m[i][:])
				a[i][3] = rhs[i]
			}
			for col := 0; col < 3; col++ {
				piv := col
				for r := col + 1; r < 3; r++ {
					if math.Abs(a[r][col]) > math.Abs(a[piv][col]) {
						piv = r
					}
				}
				if math.Abs(a[piv][col]) < 1e-12 {
					return nil, false
				}
				a[col], a[piv] = a[piv], a[col]
				d := a[col][col]
				for k := col; k < 4; k++ {
					a[col][k] /= d
				}
				for r := 0; r < 3; r++ {
					if r == col {
						continue
					}
					f := a[r][col]
					for k := col; k < 4; k++ {
						a[r][k] -= f * a[col][k]
					}
				}
			}
			pa, pb, pc := a[0][3], a[1][3], a[2][3]
			return func(x float64) float64 {
				den := 1 + pc*x
				if math.Abs(den) < 1e-9 {
					return math.Inf(1)
				}
				return (pa*x + pb) / den
			}, true
		}

		var cands []rowCand
		bestScore := math.Inf(1)
		var bestPitch float64
		for lo := 0; lo < 12; lo++ {
			for hi := lo + n - 1 - maxOutliers; hi < 12; hi++ {
				if hi <= lo {
					continue
				}
				aLin := (t[order[n-1]] - t[order[0]]) / (canonX[hi] - canonX[lo])
				if aLin <= 0 {
					continue
				}
				bLin := t[order[0]] - aLin*canonX[lo]
				scale := aLin * float64(cb.PointW)
				lin := func(x float64) float64 { return aLin*x + bLin }
				cols, _, _ := assign(lin, scale)
				tHat := lin
				if p, ok := projective1D(cols); ok {
					tHat = p
				}
				cols, score, _ := assign(tHat, scale)
				if len(cols) < 4 || len(cols) < n-maxOutliers {
					continue
				}
				if score < bestScore {
					bestScore, bestPitch = score, scale
				}
				cands = append(cands, rowCand{cols: cols, barT: tHat(cb.BarCenterX()), score: score})
			}
		}
		if math.IsInf(bestScore, 1) || bestScore > 0.06*float64(n) {
			return rowFit{}, false // no hypothesis tracks the grid tightly
		}
		// Keep the plausible alignments (within 2.5× the best): a sparse row
		// can be honestly ambiguous on its own — the caller disambiguates
		// with the OTHER row's bar position. In strict mode a row must be
		// unambiguous by itself (second-best at least 2× worse), as the
		// original bootstrap demanded.
		var keep []rowCand
		for _, c := range cands {
			if c.score <= 2.5*bestScore+1e-9 {
				keep = append(keep, c)
			}
		}
		if strict && len(keep) > 1 {
			sort.Slice(keep, func(a, b int) bool { return keep[a].score < keep[b].score })
			if keep[1].score < 2*keep[0].score+1e-9 {
				return rowFit{}, false // ambiguous: refuse to guess strictly
			}
			keep = keep[:1]
		}
		sort.Slice(keep, func(a, b int) bool { return keep[a].score < keep[b].score })
		if len(keep) > 4 {
			keep = keep[:4]
		}
		return rowFit{cands: keep, mx: mx, my: my, ux: ux, uy: uy, pitch: bestPitch}, true
	}

	top, okT := fitRow(rows[0])
	bot, okB := fitRow(rows[1])
	if !okT || !okB {
		return nil, false
	}
	// Cross-row disambiguation: both rows must place the bar at (nearly)
	// the same image abscissa. A sparse row is often ambiguous alone; the
	// pairing that agrees on the bar (and has the least combined residual)
	// resolves it — a shifted indexing on one row cannot agree.
	pitch := math.Max(top.pitch, bot.pitch)
	barX := func(rf rowFit, c rowCand) float64 { return rf.mx + c.barT*rf.ux }
	bestPair := [2]int{-1, -1}
	bestCost := math.Inf(1)
	for i, tc := range top.cands {
		for j, bc := range bot.cands {
			mismatch := math.Abs(barX(top, tc) - barX(bot, bc))
			if mismatch > 0.75*pitch {
				continue
			}
			cost := tc.score + bc.score + mismatch/pitch
			if cost < bestCost {
				bestCost, bestPair = cost, [2]int{i, j}
			}
		}
	}
	if bestPair[0] < 0 {
		return nil, false
	}
	m := map[int]int{}
	for c, k := range top.cands[bestPair[0]].cols {
		m[(13+c)-1] = rows[0][k] // top row, left→right 13..24
	}
	for c, k := range bot.cands[bestPair[1]].cols {
		m[(12-c)-1] = rows[1][k] // bottom row, left→right 12..1
	}
	return m, true
}
