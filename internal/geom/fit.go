package geom

import "math"

// LineConstraint ties the image of a known source point to a known
// destination-space LINE: after fitting, H·Src must lie on L (given as
// a·x+b·y+c=0 homogeneous coefficients). One linear equation — the natural
// way to use a fitted edge whose 1D correspondence is unknown. With
// (a,b) unit-norm the equation's residual is a signed distance in pixels,
// commensurate with the point equations.
type LineConstraint struct {
	Src Pt
	L   [3]float64
}

// HomographyFit least-squares fits the homography mapping src[i] → dst[i]
// over N ≥ 4 correspondences — the overdetermined companion to Homography.
// Both point sets are Hartley-normalized (centroid at the origin, mean
// distance √2) before solving the 8-unknown DLT normal equations, so the fit
// is stable for the pixel-coordinate magnitudes calibration works with.
// Returns false when N < 4 or the system is degenerate (e.g. collinear
// points).
func HomographyFit(src, dst []Pt) (Mat3, bool) {
	return HomographyFitLines(src, dst, nil)
}

// HomographyFitLines is HomographyFit with additional point-on-line
// constraints. The constraints break configurations that are degenerate on
// points alone — notably correspondences confined to two parallel lines
// (a backgammon board's two apex rows), which leave a one-parameter family
// of homographies that the known edge lines pin down (ADR-0008 §4).
func HomographyFitLines(src, dst []Pt, cons []LineConstraint) (Mat3, bool) {
	if len(src) < 4 || len(src) != len(dst) {
		return Mat3{}, false
	}
	ns, ts, ok := normalize(src)
	if !ok {
		return Mat3{}, false
	}
	nd, td, ok := normalize(dst)
	if !ok {
		return Mat3{}, false
	}

	// Normal equations A'A h = A'b for the 2N×8 DLT system with h22 = 1
	// (safe in normalized space for the non-degenerate fits we accept).
	var ata [8][8]float64
	var atb [8]float64
	acc := func(row [8]float64, rhs float64) {
		for i := 0; i < 8; i++ {
			if row[i] == 0 {
				continue
			}
			for j := 0; j < 8; j++ {
				ata[i][j] += row[i] * row[j]
			}
			atb[i] += row[i] * rhs
		}
	}
	for k := range ns {
		x, y := ns[k].X, ns[k].Y
		u, v := nd[k].X, nd[k].Y
		acc([8]float64{x, y, 1, 0, 0, 0, -x * u, -y * u}, u)
		acc([8]float64{0, 0, 0, x, y, 1, -x * v, -y * v}, v)
	}
	// Point-on-line rows: lⁿᵀ·Hn·xn = 0 in normalized space, with the line
	// transformed as lⁿ ∝ Td⁻ᵀ·l and rescaled to unit (a,b) so residuals
	// stay pixel-commensurate with the point rows.
	tdInvForLines, ok := td.Inverse()
	if !ok {
		return Mat3{}, false
	}
	for _, c := range cons {
		xn := ts.Apply(c.Src)
		l := tdInvForLines.TransposeApplyVec(c.L)
		n := math.Hypot(l[0], l[1])
		if n < 1e-12 {
			continue
		}
		l[0], l[1], l[2] = l[0]/n, l[1]/n, l[2]/n
		acc([8]float64{
			l[0] * xn.X, l[0] * xn.Y, l[0],
			l[1] * xn.X, l[1] * xn.Y, l[1],
			l[2] * xn.X, l[2] * xn.Y,
		}, -l[2])
	}
	h, ok := solve8(ata, atb)
	if !ok {
		return Mat3{}, false
	}
	hn := Mat3{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1}

	// Denormalize: H = Td⁻¹ · Hn · Ts.
	tdInv, ok := td.Inverse()
	if !ok {
		return Mat3{}, false
	}
	H := tdInv.Mul(hn).Mul(ts)

	// A degenerate configuration can slip through the solver with a wild
	// answer; verify the fit actually reprojects.
	for k := range src {
		p := H.Apply(src[k])
		if math.IsNaN(p.X) || math.IsInf(p.X, 0) {
			return Mat3{}, false
		}
	}
	return H, true
}

// normalize returns the Hartley-normalized points and the similarity T that
// produced them (p' = T·p). Fails when the points are (near-)coincident or
// collinear enough to make the scale collapse in one direction.
func normalize(pts []Pt) ([]Pt, Mat3, bool) {
	var cx, cy float64
	for _, p := range pts {
		cx += p.X / float64(len(pts))
		cy += p.Y / float64(len(pts))
	}
	var meanD, spreadX, spreadY float64
	for _, p := range pts {
		dx, dy := p.X-cx, p.Y-cy
		meanD += math.Hypot(dx, dy) / float64(len(pts))
		spreadX += math.Abs(dx) / float64(len(pts))
		spreadY += math.Abs(dy) / float64(len(pts))
	}
	if meanD < 1e-9 || spreadX < 1e-6*meanD || spreadY < 1e-6*meanD {
		return nil, Mat3{}, false // coincident or axis-collinear
	}
	s := math.Sqrt2 / meanD
	T := Mat3{s, 0, -s * cx, 0, s, -s * cy, 0, 0, 1}
	out := make([]Pt, len(pts))
	for i, p := range pts {
		out[i] = T.Apply(p)
	}
	return out, T, true
}

// TransposeApplyVec applies the matrix transpose to a homogeneous 3-vector —
// the covector transform (lines map by the transpose of the inverse point
// map, which is what HomographyFitLines needs for its normalizers).
func (m Mat3) TransposeApplyVec(v [3]float64) [3]float64 {
	return [3]float64{
		m[0]*v[0] + m[3]*v[1] + m[6]*v[2],
		m[1]*v[0] + m[4]*v[1] + m[7]*v[2],
		m[2]*v[0] + m[5]*v[1] + m[8]*v[2],
	}
}

// LineMatch corresponds a whole source-space line to a destination-space
// line: after fitting, the source line must be the pre-image of Dst, i.e.
// Src ∝ Hᵀ·Dst — two linear equations. Lines are a·x+b·y+c=0 coefficient
// triples. W weights the equations relative to single point rows (a line
// typically summarizes several observations). Unlike point rows a line pair
// carries no width/anchor along itself, which makes it the right constraint
// for edges whose extent is unreliable.
type LineMatch struct {
	Src, Dst [3]float64
	W        float64
}

// HomographyFitFeatures is the full-featured fit: point correspondences,
// point-on-line constraints, and line-line correspondences in one least
// squares system.
func HomographyFitFeatures(src, dst []Pt, cons []LineConstraint, lines []LineMatch) (Mat3, bool) {
	if len(src) < 4 || len(src) != len(dst) {
		return Mat3{}, false
	}
	ns, ts, ok := normalize(src)
	if !ok {
		return Mat3{}, false
	}
	nd, td, ok := normalize(dst)
	if !ok {
		return Mat3{}, false
	}
	tdInv, ok := td.Inverse()
	if !ok {
		return Mat3{}, false
	}
	tsInv, ok := ts.Inverse()
	if !ok {
		return Mat3{}, false
	}

	var ata [8][8]float64
	var atb [8]float64
	acc := func(row [8]float64, rhs float64) {
		for i := 0; i < 8; i++ {
			if row[i] == 0 {
				continue
			}
			for j := 0; j < 8; j++ {
				ata[i][j] += row[i] * row[j]
			}
			atb[i] += row[i] * rhs
		}
	}
	for k := range ns {
		x, y := ns[k].X, ns[k].Y
		u, v := nd[k].X, nd[k].Y
		acc([8]float64{x, y, 1, 0, 0, 0, -x * u, -y * u}, u)
		acc([8]float64{0, 0, 0, x, y, 1, -x * v, -y * v}, v)
	}
	unitLine := func(l [3]float64) ([3]float64, bool) {
		n := math.Hypot(l[0], l[1])
		if n < 1e-12 {
			return l, false
		}
		return [3]float64{l[0] / n, l[1] / n, l[2] / n}, true
	}
	for _, c := range cons {
		xn := ts.Apply(c.Src)
		l, ok := unitLine(tdInv.TransposeApplyVec(c.L))
		if !ok {
			continue
		}
		acc([8]float64{
			l[0] * xn.X, l[0] * xn.Y, l[0],
			l[1] * xn.X, l[1] * xn.Y, l[1],
			l[2] * xn.X, l[2] * xn.Y,
		}, -l[2])
	}
	for _, lm := range lines {
		ls, okS := unitLine(tsInv.TransposeApplyVec(lm.Src))
		ld, okD := unitLine(tdInv.TransposeApplyVec(lm.Dst))
		if !okS || !okD {
			continue
		}
		w := lm.W
		if w <= 0 {
			w = 1
		}
		// Hnᵀ·ld ∝ ls  →  cross(Hnᵀ·ld, ls) = 0, three (rank-2) rows.
		l0, l1, l2 := ld[0], ld[1], ld[2]
		s0, s1, s2 := ls[0], ls[1], ls[2]
		acc([8]float64{0, w * l0 * s2, -w * l0 * s1, 0, w * l1 * s2, -w * l1 * s1, 0, w * l2 * s2}, w*l2*s1)
		acc([8]float64{-w * l0 * s2, 0, w * l0 * s0, -w * l1 * s2, 0, w * l1 * s0, -w * l2 * s2, 0}, -w*l2*s0)
		acc([8]float64{w * l0 * s1, -w * l0 * s0, 0, w * l1 * s1, -w * l1 * s0, 0, w * l2 * s1, -w * l2 * s0}, 0)
	}
	h, ok := solve8(ata, atb)
	if !ok {
		return Mat3{}, false
	}
	hn := Mat3{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1}
	H := tdInv.Mul(hn).Mul(ts)
	for k := range src {
		p := H.Apply(src[k])
		if math.IsNaN(p.X) || math.IsInf(p.X, 0) {
			return Mat3{}, false
		}
	}
	return H, true
}
