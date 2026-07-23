package autocal

// lockRank is the comparable evidence of one verified correspondence-fit
// lock: how many apex↔slot correspondences the fit used, how tightly it
// tracked them, and how many apexes its own instant's mask offered.
type lockRank struct {
	Matches int
	Resid   float64
	SetSize int // apex count of the lock's own instant set
}

// fraction is the share of its own frame's apex evidence the lock explains.
func (r lockRank) fraction() float64 {
	if r.SetSize <= 0 {
		return 0
	}
	return float64(r.Matches) / float64(r.SetSize)
}

// pickLock adjudicates between verified locks (returns the winning index).
//
// Primary key: the EXPLAINED FRACTION (matches / own-set apex count). A true
// board lock explains almost every component of its frame's triangle mask; a
// fit diluted by off-board junk (score sheets, trays — the quad stretches to
// cover board + junk and matches MORE apexes than the true lock) or locked
// onto static junk outright (which can carry the LOWEST residual of all —
// self-consistency is not truth) explains a visibly smaller share of its own
// evidence. Both failure shapes were measured on the bench; raw matches and
// raw residual each regressed real captures when used as the primary key.
//
// Fractions within fractionTol are one-component noise: inside that group
// the usual (matches desc, resid asc) order decides. Exact ties keep the
// earliest candidate — probe order is preference order.
func pickLock(cands []lockRank) int {
	const fractionTol = 0.02
	bestF := 0.0
	for _, c := range cands {
		if f := c.fraction(); f > bestF {
			bestF = f
		}
	}
	best := -1
	for i, c := range cands {
		if c.fraction() < bestF-fractionTol {
			continue
		}
		if best < 0 || c.Matches > cands[best].Matches ||
			(c.Matches == cands[best].Matches && c.Resid < cands[best].Resid) {
			best = i
		}
	}
	return best
}
