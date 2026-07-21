package autocal

import (
	"math"
	"sort"

	"lazybg/internal/calibrate"
	"lazybg/internal/geom"
)

// FitResult is a correspondence-fitted calibration in mask coordinates.
type FitResult struct {
	Corners  [4]geom.Pt
	BarEdges [4]geom.Pt
	// Lens is the admitted radial distortion (mask-space centre/norm;
	// coefficients are dimensionless). Zero value = pinhole: the nested
	// admission (ADR-0008 §5) only fills it when it pays its way.
	Lens    calibrate.Lens
	Matches int     // apex↔slot correspondences the final fit used
	Resid   float64 // rms reprojection of the matched apexes, px
}

// fitMinMatchesHalf is the fewest correspondences a half-board homography
// may be fitted from. Four is the algebraic minimum; one extra point makes a
// single bad apex visible in the residual instead of silently absorbed.
const fitMinMatchesHalf = 5

// FitHandles refines seed calibration handles against a filtered triangle
// mask by explicit correspondences (ADR-0008 §4), using only primitives that
// are invariant across board makes:
//
//   - each triangle's APEX (the intersection of its two lateral edges) is a
//     point correspondence — apexes sit at column centres with the true
//     pitch, regardless of how wide the triangles are drawn;
//   - the two OUTER EDGES (the per-row consensus lines through the intact
//     triangle bases) are line↔line correspondences — they pin the
//     transverse extent that apexes alone leave free (two parallel apex
//     rows are a degenerate configuration), again without trusting any
//     width.
//
// Triangle base WIDTH is deliberately never trusted: real points are
// narrower than their column, and any width-derived constraint drags the
// scale (measured: ~17% shrink on the pilot).
//
// The seed only initializes the apex↔slot matching; when it is nowhere near
// the board (the mask quad can go wild), a seed-free bootstrap indexes the
// apexes instead. Two extra rounds re-match with the improved calibration.
// All points are mask-space; the caller scales. Returns ok=false — keep the
// seed — when correspondences or outer edges are missing or the final fit
// does not track the apexes tightly; failure is detectable, so the fallback
// is clean.
func FitHandles(mask []bool, w, h int, corners, barEdges [4]geom.Pt, cb calibrate.CanonicalBoard) (FitResult, bool) {
	return FitApexes(ApexComponents(mask, w, h), w, h, corners, barEdges, cb)
}

// FitApexes is FitHandles on an already-extracted apex set — the entry point
// for multi-instant aggregation (ADR-0008 §7), where apexes detected at
// several spaced instants are merged before one fit.
func FitApexes(apexes []Apex, w, h int, corners, barEdges [4]geom.Pt, cb calibrate.CanonicalBoard) (FitResult, bool) {
	if len(apexes) < 2*fitMinMatchesHalf {
		return FitResult{}, false
	}
	pts := make([]geom.Pt, len(apexes))
	for i, a := range apexes {
		pts[i] = a.Pt
	}

	cal, ok := calibrate.NewFromHandles(corners, barEdges[:], cb, calibrate.Lens{})
	if !ok {
		return FitResult{}, false
	}
	pitchOf := func(c [4]geom.Pt) float64 {
		return math.Hypot(c[1].X-c[0].X, c[1].Y-c[0].Y) / 13
	}
	pitch := pitchOf(corners)
	split := cb.BarCenterX()
	lm := cb.Landmarks()
	_, ch := cb.Size()
	// apexY is the top-row apex line's canonical y — the effective triangle
	// length, which varies per board make (#11: estimate geometry instead of
	// assuming DefaultCanonical). It is re-estimated each round from the
	// fitted homographies, with top/bottom symmetry imposed (true of every
	// board). A wrong fixed value makes the apex points fight the outer-edge
	// lines and skews the whole fit (measured: 14px residual on the pilot).
	apexY := float64(cb.MarginY + cb.QuadH)
	apexCanon := func(p int) geom.Pt {
		base := cb.PointApex(p)
		if p >= 13 {
			return geom.P(base.X, apexY)
		}
		return geom.P(base.X, float64(ch)-apexY)
	}
	canonTop := [3]float64{0, 1, -float64(cb.MarginY)}
	canonBot := [3]float64{0, 1, -float64(ch - cb.MarginY)}
	leftLM, rightLM := [4]int{0, 3, 4, 7}, [4]int{1, 2, 5, 6}

	res := FitResult{Corners: corners, BarEdges: barEdges}
	var lastL, lastImgL, lastR, lastImgR []geom.Pt
	var lastBases []baseCand
	for round, radiusFrac := range []float64{0.45, 0.42, 0.35, 0.30} {
		predicted := make([]geom.Pt, 24)
		for p := 1; p <= 24; p++ {
			predicted[p-1] = cal.ToSource(apexCanon(p))
		}
		matches := MatchApexes(pts, predicted, radiusFrac*pitch)
		if round == 0 && orientationValid(matches, apexes) < 2*fitMinMatchesHalf {
			// The seed is nowhere near the board: fall back to seed-free
			// indexing — rows from apex orientation, absolute columns
			// anchored by the bar gap.
			if bm, ok := bootstrapMatches(apexes, cb); ok {
				matches = bm
			}
		}

		// Orientation gate: a top-row slot must match a downward triangle;
		// a match violating it is a mask artefact, not a point.
		var canonL, imgL, canonR, imgR []geom.Pt
		var bases []baseCand
		count := 0
		for slot, det := range matches {
			p := slot + 1
			topRow := p >= 13
			a := apexes[det]
			if a.Down != topRow {
				continue
			}
			count++
			bases = append(bases, baseCand{
				top: topRow,
				bl:  geom.P(a.EdgeL[0]+a.EdgeL[1]*a.BaseY, a.BaseY),
				br:  geom.P(a.EdgeR[0]+a.EdgeR[1]*a.BaseY, a.BaseY),
			})
			canon := apexCanon(p)
			if canon.X < split {
				canonL = append(canonL, canon)
				imgL = append(imgL, pts[det])
			} else {
				canonR = append(canonR, canon)
				imgR = append(imgR, pts[det])
			}
		}
		if len(canonL) < fitMinMatchesHalf || len(canonR) < fitMinMatchesHalf {
			return FitResult{}, false
		}
		ff := fitWithLens(canonL, imgL, canonR, imgR, bases, calibrate.Lens{}, canonTop, canonBot)
		if !ff.ok {
			return FitResult{}, false
		}
		projectHandles(&res, ff, calibrate.Lens{}, lm, leftLM, rightLM)
		res.Matches = count
		res.Resid = ff.resid
		lastL, lastImgL, lastR, lastImgR, lastBases = canonL, imgL, canonR, imgR, bases

		// Re-estimate the apex line from this round's homographies: project
		// the matched apexes back to canonical and take the symmetric
		// median. (The bar-edge x stays canonical: columns are pitch-true.)
		if invL, okIL := ff.hL.Inverse(); okIL {
			if invR, okIR := ff.hR.Inverse(); okIR {
				var ds []float64
				for slot, det := range matches {
					p := slot + 1
					a := apexes[det]
					if a.Down != (p >= 13) {
						continue
					}
					inv := invL
					if apexCanon(p).X >= split {
						inv = invR
					}
					c := inv.Apply(pts[det])
					if p >= 13 {
						ds = append(ds, c.Y)
					} else {
						ds = append(ds, float64(ch)-c.Y)
					}
				}
				if y, ok := medianF(ds); ok {
					lo, hi := 0.30*float64(ch), 0.48*float64(ch)
					apexY = math.Min(math.Max(y, lo), hi)
				}
			}
		}

		cal, ok = calibrate.NewFromHandles(res.Corners, res.BarEdges[:], cb, calibrate.Lens{})
		if !ok {
			return FitResult{}, false
		}
		pitch = pitchOf(res.Corners)
	}

	// Nested lens admission (ADR-0008 §5): pinhole → k1 → k1+k2, each extra
	// coefficient kept only if it cuts the correspondence residual by a
	// significant margin, else exactly 0 — Lens's zero-is-identity contract.
	const admitFrac = 0.88
	const admitGainPx = 0.15 // a coefficient must also buy real pixels, not
	// shave noise off an already-converged residual
	admits := func(rNew, rOld float64) bool {
		return rNew < admitFrac*rOld && rOld-rNew > admitGainPx
	}
	mkLens := func(k1, k2 float64) calibrate.Lens {
		if k1 == 0 && k2 == 0 {
			return calibrate.Lens{}
		}
		return calibrate.Lens{K1: k1, K2: k2, CenterX: float64(w) / 2, CenterY: float64(h) / 2, Norm: float64(w) / 2}
	}
	residOf := func(l calibrate.Lens) float64 {
		ff := fitWithLens(lastL, lastImgL, lastR, lastImgR, lastBases, l, canonTop, canonBot)
		if !ff.ok {
			return math.Inf(1)
		}
		return ff.resid
	}
	lens := calibrate.Lens{}
	r0 := res.Resid
	k1, r1 := golden1D(func(k float64) float64 { return residOf(mkLens(k, 0)) }, -0.35, 0.35)
	if admits(r1, r0) {
		lens = mkLens(k1, 0)
		k2, r2 := golden1D(func(k float64) float64 { return residOf(mkLens(k1, k)) }, -0.25, 0.25)
		if admits(r2, r1) {
			if k1b, r2b := golden1D(func(k float64) float64 { return residOf(mkLens(k, k2)) }, -0.35, 0.35); r2b < r2 {
				k1, r2 = k1b, r2b
			}
			lens = mkLens(k1, k2)
		}
	}
	if lens.K1 != 0 || lens.K2 != 0 {
		ff := fitWithLens(lastL, lastImgL, lastR, lastImgR, lastBases, lens, canonTop, canonBot)
		if ff.ok && ff.resid < res.Resid {
			projectHandles(&res, ff, lens, lm, leftLM, rightLM)
			res.Resid = ff.resid
			res.Lens = lens
		}
	}

	// The fit must actually track the apexes: a residual beyond a fraction
	// of the pitch means the correspondences were wrong (a column
	// misassignment shows up as ~a full pitch). Honest residuals on real
	// footage run ~0.16·pitch — apex noise plus true per-column geometry
	// variance — so the gate sits above that, far below misassignment.
	if res.Resid > 0.20*pitch {
		return FitResult{}, false
	}
	return res, true
}

// halfFit is one lens candidate's two fitted half homographies (in IDEAL,
// undistorted space) and the apex reprojection rms.
type halfFit struct {
	hL, hR geom.Mat3
	resid  float64
	ok     bool
}

// fitWithLens undistorts the observed features with the candidate lens,
// recomputes the outer-edge consensus lines (they curve under distortion, so
// they must be refit on undistorted points), and fits both half homographies.
func fitWithLens(canonL, imgL, canonR, imgR []geom.Pt, bases []baseCand, lens calibrate.Lens, canonTop, canonBot [3]float64) halfFit {
	und := func(ps []geom.Pt) []geom.Pt {
		out := make([]geom.Pt, len(ps))
		for i, p := range ps {
			out[i] = lens.Undistort(p)
		}
		return out
	}
	uL, uR := und(imgL), und(imgR)
	ubases := make([]baseCand, len(bases))
	for i, b := range bases {
		b.bl = lens.Undistort(b.bl)
		b.br = lens.Undistort(b.br)
		ubases[i] = b
	}
	topLine, okTop := rowEdgeLine(ubases, true)
	botLine, okBot := rowEdgeLine(ubases, false)
	if !okTop && !okBot {
		return halfFit{}
	}
	var lines []geom.LineMatch
	if okTop {
		lines = append(lines, geom.LineMatch{Src: canonTop, Dst: topLine, W: 4})
	}
	if okBot {
		lines = append(lines, geom.LineMatch{Src: canonBot, Dst: botLine, W: 4})
	}
	hL, okL := geom.HomographyFitFeatures(canonL, uL, nil, lines)
	hR, okR := geom.HomographyFitFeatures(canonR, uR, nil, lines)
	if !okL || !okR {
		return halfFit{}
	}
	return halfFit{hL: hL, hR: hR, resid: rmsReproj(hL, canonL, uL, hR, canonR, uR), ok: true}
}

// projectHandles maps the canonical landmarks through a half fit back to
// RECORDED space (distorting when a lens is set) and writes them into res.
func projectHandles(res *FitResult, ff halfFit, lens calibrate.Lens, lm [8]geom.Pt, leftLM, rightLM [4]int) {
	for _, li := range leftLM {
		p := lens.Distort(ff.hL.Apply(lm[li]))
		if li < 4 {
			res.Corners[li] = p
		} else {
			res.BarEdges[li-4] = p
		}
	}
	for _, ri := range rightLM {
		p := lens.Distort(ff.hR.Apply(lm[ri]))
		if ri < 4 {
			res.Corners[ri] = p
		} else {
			res.BarEdges[ri-4] = p
		}
	}
}

// golden1D minimizes f over [a,b] by golden-section search (f assumed
// unimodal over the bracket, true of the smooth lens-residual curves).
func golden1D(f func(float64) float64, a, b float64) (float64, float64) {
	const phi = 0.6180339887498949
	x1 := b - phi*(b-a)
	x2 := a + phi*(b-a)
	f1, f2 := f(x1), f(x2)
	for i := 0; i < 40; i++ {
		if f1 < f2 {
			b, x2, f2 = x2, x1, f1
			x1 = b - phi*(b-a)
			f1 = f(x1)
		} else {
			a, x1, f1 = x1, x2, f2
			x2 = a + phi*(b-a)
			f2 = f(x2)
		}
	}
	if f1 < f2 {
		return x1, f1
	}
	return x2, f2
}

// baseCand is one matched triangle's base-corner observation, a candidate
// support for its row's outer-edge line.
type baseCand struct {
	top    bool
	bl, br geom.Pt // observed base corners (edge lines at the base row)
}

// rowEdgeLine finds one row's outer-edge line through the intact triangle
// bases. Checker stacks truncate bases strictly toward the board centre and
// colour bleed can leak past the edge, so neither least squares nor
// one-sided trimming is safe. Instead: try the line through every base-point
// pair (a dense RANSAC — the sets are tiny), keep candidates with solid
// inlier support, and choose the OUTERMOST such line — the physical edge is
// by construction the outer envelope of the honest observations. The final
// line is least-squares refit on the winner's inliers. Returns the image
// line as a·x+b·y+c=0.
func rowEdgeLine(cands []baseCand, top bool) ([3]float64, bool) {
	var xs, ys []float64
	for _, c := range cands {
		if c.top != top {
			continue
		}
		xs = append(xs, c.bl.X, c.br.X)
		ys = append(ys, c.bl.Y, c.br.Y)
	}
	if len(xs) < 6 {
		return [3]float64{}, false
	}
	minX, maxX := xs[0], xs[0]
	for _, x := range xs {
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
	}
	span := maxX - minX
	if span < 1 {
		return [3]float64{}, false
	}
	type cand struct {
		c0, c1 float64
		in     []int
		outer  float64 // mean outward y of inliers (sign-adjusted)
	}
	inliersOf := func(c0, c1 float64) ([]int, float64) {
		var in []int
		var sum float64
		for i := range xs {
			if r := ys[i] - (c0 + c1*xs[i]); math.Abs(r) <= 1.5 {
				in = append(in, i)
				if top {
					sum -= ys[i]
				} else {
					sum += ys[i]
				}
			}
		}
		if len(in) == 0 {
			return nil, math.Inf(-1)
		}
		return in, sum / float64(len(in))
	}
	var all []cand
	maxN := 0
	for i := 0; i < len(xs); i++ {
		for j := i + 1; j < len(xs); j++ {
			dx := xs[j] - xs[i]
			if math.Abs(dx) < 0.25*span {
				continue // short-baseline pairs give unstable slopes
			}
			c1 := (ys[j] - ys[i]) / dx
			if math.Abs(c1) > 0.35 {
				continue // outer edges are near-horizontal in detection space
			}
			c0 := ys[i] - c1*xs[i]
			in, outer := inliersOf(c0, c1)
			if len(in) < 4 {
				continue
			}
			all = append(all, cand{c0, c1, in, outer})
			if len(in) > maxN {
				maxN = len(in)
			}
		}
	}
	if maxN < 4 {
		return [3]float64{}, false
	}
	best := -1
	for k, c := range all {
		if len(c.in) < maxN-2 || len(c.in) < 4 {
			continue
		}
		if best < 0 || c.outer > all[best].outer {
			best = k
		}
	}
	if best < 0 {
		return [3]float64{}, false
	}
	// Least-squares refit on the winning inliers.
	var sx, sy, sxx, sxy, n float64
	for _, i := range all[best].in {
		sx += xs[i]
		sy += ys[i]
		sxx += xs[i] * xs[i]
		sxy += xs[i] * ys[i]
		n++
	}
	if sxx*n-sx*sx == 0 {
		return [3]float64{}, false
	}
	c1 := (sxy*n - sx*sy) / (sxx*n - sx*sx)
	c0 := (sy - c1*sx) / n
	return [3]float64{-c1, 1, -c0}, true
}

// medianF returns the median of a non-empty slice.
func medianF(v []float64) (float64, bool) {
	if len(v) == 0 {
		return 0, false
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)/2], true
}

// orientationValid counts matches whose apex direction agrees with the
// slot's row — the same gate the fit applies.
func orientationValid(matches map[int]int, apexes []Apex) int {
	n := 0
	for slot, det := range matches {
		if apexes[det].Down == (slot+1 >= 13) {
			n++
		}
	}
	return n
}

func rmsReproj(hL geom.Mat3, cL, iL []geom.Pt, hR geom.Mat3, cR, iR []geom.Pt) float64 {
	var ss float64
	n := 0
	acc := func(h geom.Mat3, canon, img []geom.Pt) {
		for k := range canon {
			p := h.Apply(canon[k])
			dx, dy := p.X-img[k].X, p.Y-img[k].Y
			ss += dx*dx + dy*dy
			n++
		}
	}
	acc(hL, cL, iL)
	acc(hR, cR, iR)
	if n == 0 {
		return math.Inf(1)
	}
	return math.Sqrt(ss / float64(n))
}
