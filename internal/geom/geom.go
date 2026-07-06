// Package geom provides the minimal 2D projective geometry lazyBG needs to
// rectify a board: a 3x3 matrix, a homography solver from four point
// correspondences, and the supporting linear algebra — all pure Go, no deps.
// See docs/architecture.md §3 ("calibrate").
package geom

// Pt is a 2D point in floating-point pixel space.
type Pt struct{ X, Y float64 }

// P is a concise Pt constructor: geom.P(x, y).
func P(x, y float64) Pt { return Pt{x, y} }

// Mat3 is a 3x3 matrix in row-major order.
type Mat3 [9]float64

// Identity returns the identity matrix.
func Identity() Mat3 { return Mat3{1, 0, 0, 0, 1, 0, 0, 0, 1} }

// Apply transforms a point by the (projective) matrix, dividing by the
// homogeneous w coordinate.
func (m Mat3) Apply(p Pt) Pt {
	x := m[0]*p.X + m[1]*p.Y + m[2]
	y := m[3]*p.X + m[4]*p.Y + m[5]
	w := m[6]*p.X + m[7]*p.Y + m[8]
	if w == 0 {
		return Pt{}
	}
	return Pt{x / w, y / w}
}

// Mul returns m·n.
func (m Mat3) Mul(n Mat3) Mat3 {
	var r Mat3
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			var s float64
			for k := 0; k < 3; k++ {
				s += m[row*3+k] * n[k*3+col]
			}
			r[row*3+col] = s
		}
	}
	return r
}

// Inverse returns the matrix inverse and whether it is invertible.
func (m Mat3) Inverse() (Mat3, bool) {
	a, b, c := m[0], m[1], m[2]
	d, e, f := m[3], m[4], m[5]
	g, h, i := m[6], m[7], m[8]

	A := e*i - f*h
	B := -(d*i - f*g)
	C := d*h - e*g
	det := a*A + b*B + c*C
	if det == 0 {
		return Mat3{}, false
	}
	inv := 1 / det
	return Mat3{
		A * inv, (c*h - b*i) * inv, (b*f - c*e) * inv,
		B * inv, (a*i - c*g) * inv, (c*d - a*f) * inv,
		C * inv, (b*g - a*h) * inv, (a*e - b*d) * inv,
	}, true
}

// Homography solves the 3x3 projective transform H such that H·src[i] ≈ dst[i]
// for all four correspondences (with H[8] fixed to 1). Returns false if the
// system is degenerate (e.g. collinear points).
func Homography(src, dst [4]Pt) (Mat3, bool) {
	// Build the 8x8 linear system for h0..h7 (h8 = 1). For each point:
	//   h0·x + h1·y + h2 − h6·x·u − h7·y·u = u
	//   h3·x + h4·y + h5 − h6·x·v − h7·y·v = v
	var a [8][8]float64
	var b [8]float64
	for i := 0; i < 4; i++ {
		x, y := src[i].X, src[i].Y
		u, v := dst[i].X, dst[i].Y
		r0 := i * 2
		r1 := r0 + 1
		a[r0] = [8]float64{x, y, 1, 0, 0, 0, -x * u, -y * u}
		b[r0] = u
		a[r1] = [8]float64{0, 0, 0, x, y, 1, -x * v, -y * v}
		b[r1] = v
	}
	h, ok := solve8(a, b)
	if !ok {
		return Mat3{}, false
	}
	return Mat3{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1}, true
}

// solve8 solves an 8x8 linear system A·x = b by Gaussian elimination with
// partial pivoting.
func solve8(a [8][8]float64, b [8]float64) ([8]float64, bool) {
	const n = 8
	for col := 0; col < n; col++ {
		// Partial pivot: find the largest magnitude in this column.
		piv := col
		best := abs(a[col][col])
		for r := col + 1; r < n; r++ {
			if v := abs(a[r][col]); v > best {
				best, piv = v, r
			}
		}
		if best < 1e-12 {
			return [8]float64{}, false
		}
		a[col], a[piv] = a[piv], a[col]
		b[col], b[piv] = b[piv], b[col]

		// Eliminate below.
		for r := col + 1; r < n; r++ {
			factor := a[r][col] / a[col][col]
			if factor == 0 {
				continue
			}
			for c := col; c < n; c++ {
				a[r][c] -= factor * a[col][c]
			}
			b[r] -= factor * b[col]
		}
	}
	// Back-substitution.
	var x [8]float64
	for r := n - 1; r >= 0; r-- {
		s := b[r]
		for c := r + 1; c < n; c++ {
			s -= a[r][c] * x[c]
		}
		x[r] = s / a[r][r]
	}
	return x, true
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
