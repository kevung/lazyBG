package autocal

import "testing"

// pickLock is DetectHandles' lock adjudication. Every candidate below is a
// MEASURED lock from the #59-remnants bench diagnosis (matches, rms resid,
// and the apex count of the lock's own instant set); the wanted winner is
// the lock the opening oracle scored best. The rule under test:
//
//  1. explained fraction (Matches/SetSize) is the primary key — a fit that
//     locks onto a mask diluted by off-board junk, or onto the junk itself,
//     explains a smaller share of its own frame's components than the true
//     board lock explains of its;
//  2. fractions within 0.02 are one-component noise: inside that group the
//     usual (matches desc, resid asc) order decides.
//
// Neither raw matches (a junk-diluted fit can carry MORE) nor raw residual
// (a junk fit can carry the LOWEST — self-consistency is not truth) is safe
// alone: each, used as primary key, regressed real captures on the bench.
func TestPickLock(t *testing.T) {
	cases := []struct {
		name  string
		cands []lockRank
		want  int
	}{
		{
			// GalleGuillaume: the true lock (idx 3) ties fractions with a
			// worse-corner lock (idx 1) and matches decide; the junk-widened
			// 17-match lock (idx 4, fraction 0.85) stays outside the group.
			name: "galle: fraction group, then matches",
			cands: []lockRank{
				{Matches: 16, Resid: 4.240, SetSize: 19},
				{Matches: 15, Resid: 4.190, SetSize: 17},
				{Matches: 13, Resid: 4.670, SetSize: 19},
				{Matches: 16, Resid: 3.793, SetSize: 18},
				{Matches: 17, Resid: 4.254, SetSize: 20},
			},
			want: 3,
		},
		{
			// TristanRemille: the static-junk lock (idx 0) has the BEST
			// residual of all — residual-first picked it and lost 5 bench
			// points. It ties fractions with the true lock (idx 2); matches
			// decide inside the group.
			name: "tristan: junk lock with best residual loses on matches",
			cands: []lockRank{
				{Matches: 12, Resid: 3.873, SetSize: 13},
				{Matches: 12, Resid: 5.399, SetSize: 14},
				{Matches: 13, Resid: 4.578, SetSize: 14},
			},
			want: 2,
		},
		{
			// HanotinDenis: the near-rail k1 lock (idx 1) has the best
			// residual but a clearly lower explained fraction — excluded
			// before residual can mislead.
			name: "hanotin: lens lock excluded by fraction",
			cands: []lockRank{
				{Matches: 15, Resid: 3.085, SetSize: 18},
				{Matches: 14, Resid: 2.983, SetSize: 18},
				{Matches: 14, Resid: 3.603, SetSize: 18},
			},
			want: 0,
		},
		{
			// FredericPicot: same shape — the k1=-0.17 lock (idx 1) explains
			// 0.79 of its set vs 0.89 for the true lock.
			name: "picot: diluted lens lock excluded by fraction",
			cands: []lockRank{
				{Matches: 14, Resid: 2.449, SetSize: 19},
				{Matches: 15, Resid: 2.733, SetSize: 19},
				{Matches: 16, Resid: 3.289, SetSize: 18},
			},
			want: 2,
		},
		{
			// Pilot pair: the confirmed pair adjudicates with the same rule;
			// the 19-match lock explains 0.95 of its set, the 17-match one
			// 0.85.
			name: "pilot pair: fraction alone decides",
			cands: []lockRank{
				{Matches: 19, Resid: 2.776, SetSize: 20},
				{Matches: 17, Resid: 2.496, SetSize: 20},
			},
			want: 0,
		},
		{
			name: "exact tie keeps the earliest (probe order is preference order)",
			cands: []lockRank{
				{Matches: 14, Resid: 3.0, SetSize: 16},
				{Matches: 14, Resid: 3.0, SetSize: 16},
			},
			want: 0,
		},
		{
			name: "equal fraction and matches: residual decides",
			cands: []lockRank{
				{Matches: 14, Resid: 3.5, SetSize: 16},
				{Matches: 14, Resid: 3.0, SetSize: 16},
			},
			want: 1,
		},
		{
			name:  "single candidate",
			cands: []lockRank{{Matches: 9, Resid: 5.0, SetSize: 12}},
			want:  0,
		},
	}
	for _, tc := range cases {
		if got := pickLock(tc.cands); got != tc.want {
			t.Errorf("%s: pickLock = %d, want %d", tc.name, got, tc.want)
		}
	}
}
