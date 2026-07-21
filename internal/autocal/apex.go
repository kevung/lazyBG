package autocal

import (
	"math"
	"sort"

	"lazybg/internal/geom"
)

// Apex is one point-triangle component's fitted tip: the intersection of its
// two lateral edges. The apex is the calibration primitive of ADR-0008 §4 —
// unlike the component centroid it is insensitive to base truncation by
// stacked checkers (the checkers hide base rows, but the surviving boundary
// rows still define the same two converging lines), and the line intersection
// is sub-pixel even on a coarse mask.
type Apex struct {
	Pt    geom.Pt
	Down  bool    // triangle points toward larger y (a top-row point)
	Rows  int     // boundary rows that supported the edge fits
	Resid float64 // rms residual of the lateral-edge fits, px
}

// minApexRows is the fewest boundary rows a component may offer and still be
// fitted: below this the two lines are dominated by rasterization noise.
const minApexRows = 8

// ApexComponents fits an apex to every connected component of a (filtered)
// triangle mask. Components whose boundaries do not converge like a triangle
// — blobs, rectangles, noise — are rejected by the per-component checks, so
// the caller can feed the TriangleComponents output directly.
func ApexComponents(mask []bool, w, h int) []Apex {
	seen := make([]bool, len(mask))
	var out []Apex
	var stack []int
	for start := range mask {
		if !mask[start] || seen[start] {
			continue
		}
		stack = append(stack[:0], start)
		seen[start] = true
		// per-row extremes of this component
		minX := map[int]int{}
		maxX := map[int]int{}
		n := 0
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := i%w, i/w
			n++
			if v, ok := minX[y]; !ok || x < v {
				minX[y] = x
			}
			if v, ok := maxX[y]; !ok || x > v {
				maxX[y] = x
			}
			for _, j := range [4]int{i - 1, i + 1, i - w, i + w} {
				if j < 0 || j >= len(mask) || seen[j] || !mask[j] {
					continue
				}
				if (j == i-1 && x == 0) || (j == i+1 && x == w-1) {
					continue
				}
				seen[j] = true
				stack = append(stack, j)
			}
		}
		if n < 30 {
			continue
		}
		if a, ok := fitApex(minX, maxX); ok {
			out = append(out, a)
		}
	}
	return out
}

// fitApex fits the two lateral edges x=a+b·y to a component's per-row
// extremes and intersects them. Returns false when the component does not
// behave like a triangle (edges parallel, no width convergence, apex far
// from the narrow end).
func fitApex(minX, maxX map[int]int) (Apex, bool) {
	ys := make([]int, 0, len(minX))
	for y := range minX {
		ys = append(ys, y)
	}
	sort.Ints(ys)
	if len(ys) < minApexRows {
		return Apex{}, false
	}

	// Orientation: the wider end is the base.
	k := len(ys) / 4
	if k < 1 {
		k = 1
	}
	widthAt := func(y int) float64 { return float64(maxX[y] - minX[y] + 1) }
	var wFirst, wLast float64
	for i := 0; i < k; i++ {
		wFirst += widthAt(ys[i]) / float64(k)
		wLast += widthAt(ys[len(ys)-1-i]) / float64(k)
	}
	if wFirst < 1.5*wLast && wLast < 1.5*wFirst {
		return Apex{}, false // no convergence: not a triangle
	}
	down := wFirst > wLast // base first (small y) ⇒ apex points down

	left := robustLine(ys, minX)
	right := robustLine(ys, maxX)
	if left.n < minApexRows || right.n < minApexRows {
		return Apex{}, false
	}
	if math.Abs(left.b-right.b) < 0.03 {
		return Apex{}, false // near-parallel edges cannot intersect stably
	}
	yApex := (right.a - left.a) / (left.b - right.b)
	xApex := left.a + left.b*yApex

	// The apex must sit at (or extrapolate modestly past) the narrow end.
	yNarrow, yBase := float64(ys[len(ys)-1]), float64(ys[0])
	if !down {
		yNarrow, yBase = float64(ys[0]), float64(ys[len(ys)-1])
	}
	span := math.Abs(yBase - yNarrow)
	toward := (yApex - yNarrow) * sign(yNarrow-yBase) // >0 = beyond the narrow end
	if toward < -0.2*span || toward > 0.6*span {
		return Apex{}, false
	}
	return Apex{
		Pt:    geom.P(xApex, yApex),
		Down:  down,
		Rows:  min(left.n, right.n),
		Resid: math.Max(left.resid, right.resid),
	}, true
}

type line struct {
	a, b  float64 // x = a + b·y
	n     int
	resid float64
}

// robustLine least-squares fits x = a + b·y with one outlier-rejection pass
// (a stray mask pixel on one row must not bend the edge).
func robustLine(ys []int, xs map[int]int) line {
	fit := func(keep func(y int) bool) line {
		var sy, sx, syy, sxy, n float64
		for _, y := range ys {
			if !keep(y) {
				continue
			}
			fy, fx := float64(y), float64(xs[y])
			sy += fy
			sx += fx
			syy += fy * fy
			sxy += fy * fx
			n++
		}
		if n < 2 || syy*n-sy*sy == 0 {
			return line{}
		}
		b := (sxy*n - sy*sx) / (syy*n - sy*sy)
		a := (sx - b*sy) / n
		var ss float64
		var m int
		for _, y := range ys {
			if !keep(y) {
				continue
			}
			r := float64(xs[y]) - (a + b*float64(y))
			ss += r * r
			m++
		}
		return line{a: a, b: b, n: m, resid: math.Sqrt(ss / float64(m))}
	}
	first := fit(func(int) bool { return true })
	if first.n == 0 {
		return first
	}
	thr := math.Max(2.0, 2.5*first.resid)
	return fit(func(y int) bool {
		return math.Abs(float64(xs[y])-(first.a+first.b*float64(y))) <= thr
	})
}

// MatchApexes pairs detected apexes with predicted slot positions (canonical
// point tips projected through a candidate calibration): mutual nearest
// neighbour within maxDist. Slots with no detection (occluded triangles) and
// detections with no slot (spurious components) are simply absent — the
// downstream fit works from whatever correspondences survive.
func MatchApexes(detected, predicted []geom.Pt, maxDist float64) map[int]int {
	nearest := func(p geom.Pt, set []geom.Pt) (int, float64) {
		bi, bd := -1, math.Inf(1)
		for i, q := range set {
			if d := math.Hypot(p.X-q.X, p.Y-q.Y); d < bd {
				bi, bd = i, d
			}
		}
		return bi, bd
	}
	m := map[int]int{}
	for si, p := range predicted {
		di, d := nearest(p, detected)
		if di < 0 || d > maxDist {
			continue
		}
		if back, _ := nearest(detected[di], predicted); back != si {
			continue
		}
		m[si] = di
	}
	return m
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
