package autocal

import (
	"image"
	"image/color"
)

// AutoColorCandidates derives PLAUSIBLE board color schemes from the median
// frame, ranked by mask evidence — the hypothesize half of fixing the
// corpus's dominant detection failure (#56): on captures where the table
// dominates the frame centre, the single-answer AutoColors picks the table
// as felt and whatever saturated junk borders it as point colors, while the
// real board loses the vote. Instead of guessing once, emit the top felt
// candidates × the point-color pairs adjacent to each, and let the caller's
// verifier (the correspondence fit) decide which hypothesis is a board.
//
// Candidate #0 is exactly the legacy AutoColors answer, so a caller taking
// only the first candidate behaves as before.
func AutoColorCandidates(med *image.RGBA, max int) []Colors {
	if max < 1 {
		max = 1
	}
	// Unsaturated felts first (the common case — keeps candidate #0 legacy),
	// then the dominant centre colors REGARDLESS of saturation: blue- or
	// green-felt boards (2026-05 Marseille) have no unsaturated felt at all,
	// and the fit-verifier makes wrong extra hypotheses harmless (#57).
	felts := topBins(feltHistogram(med), 2)
	for _, fb := range topBins(anyFeltHistogram(med), 3) {
		dup := false
		for _, o := range felts {
			if binsNeighbor(fb, o) {
				dup = true
				break
			}
		}
		if !dup {
			felts = append(felts, fb)
		}
	}
	var out []Colors
	seen := map[Colors]bool{}
	add := func(c Colors) {
		if len(out) < max && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	for _, fb := range felts {
		felt := color.RGBA{uint8(fb.r*24 + 12), uint8(fb.g*24 + 12), uint8(fb.b*24 + 12), 255}
		clusters := feltAdjacentClusters(med, fb, felt, 3)
		if len(clusters) == 0 {
			continue
		}
		// Pairs in evidence order; a single cluster stands in for both
		// point colors (legacy behaviour for one-color boards).
		pairs := [][2]int{{0, 1}, {0, 2}, {1, 2}}
		if len(clusters) == 1 {
			pairs = [][2]int{{0, 0}}
		}
		for _, p := range pairs {
			if p[0] >= len(clusters) || p[1] >= len(clusters) {
				continue
			}
			add(Colors{PointA: clusters[p[0]], PointB: clusters[p[1]], Felt: felt})
		}
	}
	return out
}

type colorBin struct{ r, g, b int }

func binOf(r, g, b uint8) colorBin { return colorBin{int(r) / 24, int(g) / 24, int(b) / 24} }

func binsNeighbor(a, b colorBin) bool {
	return abs(a.r-b.r) <= 1 && abs(a.g-b.g) <= 1 && abs(a.b-b.b) <= 1
}

// feltHistogram counts the unsaturated mid-brightness bins of the frame
// centre — the felt candidates.
func feltHistogram(med *image.RGBA) map[colorBin]int {
	b := med.Bounds()
	w, h := b.Dx(), b.Dy()
	hist := map[colorBin]int{}
	for y := h / 5; y < h*4/5; y++ {
		for x := w / 5; x < w*4/5; x++ {
			i := med.PixOffset(b.Min.X+x, b.Min.Y+y)
			r, g, bl := med.Pix[i], med.Pix[i+1], med.Pix[i+2]
			mx, mn := max(r, max(g, bl)), min(r, min(g, bl))
			if int(mx)-int(mn) < 30 && mx > 60 && mx < 235 {
				hist[binOf(r, g, bl)]++
			}
		}
	}
	return hist
}

// anyFeltHistogram counts ALL mid-brightness bins of the frame centre —
// saturated felts included. Extreme brightness stays excluded: near-black
// shadows and blown-white frames are never a playing surface.
func anyFeltHistogram(med *image.RGBA) map[colorBin]int {
	b := med.Bounds()
	w, h := b.Dx(), b.Dy()
	hist := map[colorBin]int{}
	for y := h / 5; y < h*4/5; y++ {
		for x := w / 5; x < w*4/5; x++ {
			i := med.PixOffset(b.Min.X+x, b.Min.Y+y)
			r, g, bl := med.Pix[i], med.Pix[i+1], med.Pix[i+2]
			mx := max(r, max(g, bl))
			if mx > 40 && mx < 245 {
				hist[binOf(r, g, bl)]++
			}
		}
	}
	return hist
}

// topBins returns up to n bins by count, greedily excluding quantization
// neighbours of already-chosen bins (they are the same physical surface).
// Ties break lexicographically so candidate order is deterministic (map
// iteration order must never decide which color is PointA).
func topBins(hist map[colorBin]int, n int) []colorBin {
	lessBin := func(a, b colorBin) bool {
		if a.r != b.r {
			return a.r < b.r
		}
		if a.g != b.g {
			return a.g < b.g
		}
		return a.b < b.b
	}
	var out []colorBin
	for len(out) < n {
		best, bestN := colorBin{}, 0
		for k, c := range hist {
			if c < bestN || (c == bestN && (bestN == 0 || !lessBin(k, best))) {
				continue
			}
			dup := false
			for _, o := range out {
				if binsNeighbor(k, o) {
					dup = true
					break
				}
			}
			if !dup {
				best, bestN = k, c
			}
		}
		if bestN == 0 {
			break
		}
		out = append(out, best)
	}
	return out
}

// feltAdjacentClusters returns up to n averaged cluster colors (by count,
// quantization-distinct) among the saturated-enough pixels that differ from
// the felt yet sit adjacent to it — the legacy point-color logic,
// parameterized by the felt hypothesis.
func feltAdjacentClusters(med *image.RGBA, feltBin colorBin, felt color.RGBA, n int) []color.RGBA {
	b := med.Bounds()
	w, h := b.Dx(), b.Dy()
	at := func(x, y int) (uint8, uint8, uint8) {
		i := med.PixOffset(b.Min.X+x, b.Min.Y+y)
		return med.Pix[i], med.Pix[i+1], med.Pix[i+2]
	}
	isFelt := func(x, y int) bool {
		if x < 0 || x >= w || y < 0 || y >= h {
			return false
		}
		r, g, bl := at(x, y)
		return binOf(r, g, bl) == feltBin
	}
	counts := map[colorBin]int{}
	sums := map[colorBin][4]int{}
	const reach = 8
	for y := h / 5; y < h*4/5; y++ {
		for x := w / 5; x < w*4/5; x++ {
			r, g, bl := at(x, y)
			mx, mn := max(r, max(g, bl)), min(r, min(g, bl))
			if int(mx)-int(mn) < 18 {
				continue
			}
			dr := float64(r) - float64(felt.R)
			dg := float64(g) - float64(felt.G)
			db := float64(bl) - float64(felt.B)
			if dr*dr+dg*dg+db*db < 45*45 {
				continue
			}
			adj := 0
			for _, d := range [4][2]int{{reach, 0}, {-reach, 0}, {0, reach}, {0, -reach}} {
				if isFelt(x+d[0], y+d[1]) {
					adj++
				}
			}
			if adj < 1 {
				continue
			}
			k := binOf(r, g, bl)
			counts[k]++
			s := sums[k]
			s[0] += int(r)
			s[1] += int(g)
			s[2] += int(bl)
			s[3]++
			sums[k] = s
		}
	}
	var out []color.RGBA
	for _, k := range topBins(counts, n) {
		s := sums[k]
		if s[3] == 0 {
			continue
		}
		out = append(out, color.RGBA{uint8(s[0] / s[3]), uint8(s[1] / s[3]), uint8(s[2] / s[3]), 255})
	}
	return out
}
