package eval

import (
	"lazybg/internal/bg"
	"lazybg/internal/derive"
)

// MatchScore compares a produced transcription against the ground-truth match
// — the primary "effort saved at bounded error" bundle (experiment-plan §6).
// Plies are aligned per game as a longest common subsequence, so one missed or
// spurious turn does not desynchronize everything after it. Two plies are the
// same when the same player, with the same dice, reaches the same resulting
// board (boards come from replaying both matches — notation spelling
// differences do not matter).
type MatchScore struct {
	GamesTruth, GamesOut int

	TruthCheckerPlies int // truth plies that are not cube actions (incl. dances)
	OutCheckerPlies   int
	Matched           int // aligned identical plies

	TruthCubeActions int // cube plies in the truth — v1 perception cannot see them

	AutoFilled        int // output plies at/above the auto-fill threshold
	AutoFilledCorrect int // …of which aligned with the truth
	Reviewed          int // output plies below the threshold
}

// AutoFillErrors is the guarded metric: confidently wrong plies.
func (s MatchScore) AutoFillErrors() int { return s.AutoFilled - s.AutoFilledCorrect }

// Coverage is the fraction of the truth the pipeline filled in correctly
// without a human.
func (s MatchScore) Coverage() float64 {
	if s.TruthCheckerPlies == 0 {
		return 0
	}
	return float64(s.AutoFilledCorrect) / float64(s.TruthCheckerPlies)
}

// ErrorRate is wrong auto-fills over all auto-fills.
func (s MatchScore) ErrorRate() float64 {
	if s.AutoFilled == 0 {
		return 0
	}
	return float64(s.AutoFillErrors()) / float64(s.AutoFilled)
}

// ReviewRate is the human workload: reviewed plies over produced plies.
func (s MatchScore) ReviewRate() float64 {
	if s.OutCheckerPlies == 0 {
		return 0
	}
	return float64(s.Reviewed) / float64(s.OutCheckerPlies)
}

// alignPly is one checker ply prepared for comparison.
type alignPly struct {
	player bg.Player
	dice   bg.Dice // normalized high-low
	canon  string  // canonical hop multiset; "" for a dance
	conf   float64
	known  bool // notation parsed (unrecorded "????" plies never match)
}

// ScoreMatch scores got against want. threshold is the auto-fill gate the
// pipeline ran with; got plies at/above it count as auto-filled.
func ScoreMatch(got, want bg.Match, threshold float64) MatchScore {
	s := MatchScore{GamesTruth: len(want.Games), GamesOut: len(got.Games)}

	truthGames := checkerPliesByGame(want)
	outGames := checkerPliesByGame(got)
	for _, g := range truthGames {
		s.TruthCheckerPlies += len(g)
	}
	for _, g := range outGames {
		s.OutCheckerPlies += len(g)
		for _, p := range g {
			if p.conf >= threshold {
				s.AutoFilled++
			} else {
				s.Reviewed++
			}
		}
	}
	for _, g := range want.Games {
		for _, p := range g.Plies {
			if p.Cube != bg.NoCube {
				s.TruthCubeActions++
			}
		}
	}

	n := len(truthGames)
	if len(outGames) < n {
		n = len(outGames)
	}
	for gi := 0; gi < n; gi++ {
		matchedOut := lcs(truthGames[gi], outGames[gi])
		for oi, ok := range matchedOut {
			if !ok {
				continue
			}
			s.Matched++
			if outGames[gi][oi].conf >= threshold {
				s.AutoFilledCorrect++
			}
		}
	}
	return s
}

// checkerPliesByGame returns each game\'s non-cube plies in canonical form.
// Identity is (player, dice, canonical hops) rather than resulting boards:
// board-chained identity would charge every ply after one mistake as wrong,
// overstating the human effort a single correction actually costs.
func checkerPliesByGame(m bg.Match) [][]alignPly {
	out := make([][]alignPly, 0, len(m.Games))
	for _, g := range m.Games {
		var plies []alignPly
		for _, p := range g.Plies {
			if p.Cube != bg.NoCube {
				continue
			}
			d := p.Dice
			if d[0] < d[1] {
				d[0], d[1] = d[1], d[0]
			}
			ap := alignPly{player: p.Player, dice: d, conf: p.Confidence, known: true}
			switch {
			case p.CannotMove || p.Notation == "":
				// dance / dice-only: canon stays ""
			case p.Notation == "????":
				ap.known = false
			default:
				canon, err := derive.CanonicalPlays(p.Notation)
				if err != nil {
					ap.known = false
				} else {
					ap.canon = canon
				}
			}
			plies = append(plies, ap)
		}
		out = append(out, plies)
	}
	return out
}

func samePly(a, b alignPly) bool {
	return a.known && b.known &&
		a.player == b.player && a.dice == b.dice && a.canon == b.canon
}

// lcs aligns truth and out plies, returning which out plies matched.
func lcs(truth, out []alignPly) []bool {
	n, m := len(truth), len(out)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if samePly(truth[i], out[j]) {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	matched := make([]bool, m)
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case samePly(truth[i], out[j]) && dp[i][j] == dp[i+1][j+1]+1:
			matched[j] = true
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			i++
		default:
			j++
		}
	}
	return matched
}
