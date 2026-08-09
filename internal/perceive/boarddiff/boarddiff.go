// Package boarddiff turns two board readings into a move. Given the pre-move
// position and an observed post-move board, it asks the engine for every legal
// move, then picks the one whose resulting board best matches what was observed
// (docs/architecture.md §3, "boarddiff"). This is where perception, the engine's
// hard legality filter, and fusion meet.
package boarddiff

import (
	"fmt"
	"strings"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
	"lazybg/internal/engine"
	"lazybg/internal/fusion"
	"lazybg/internal/perceive"
)

// Scored is a legal move annotated with how well its result matches the
// observed post-board (Match in [0,1]).
type Scored struct {
	Move  engine.LegalMove
	Match float64
}

// Detect returns the legal moves (deduplicated by resulting board, keeping the
// highest-equity notation) ranked by how well each matches the observed
// post-board, best match first.
func Detect(pre bg.Position, post perceive.ObservedBoard) ([]Scored, error) {
	moves, err := engine.LegalMoves(pre)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]Scored, 0, len(moves))
	for _, mv := range moves { // moves are equity-sorted, so first per board wins
		key := boardSig(mv.Result)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Scored{Move: mv, Match: matchScore(mv.Result, post)})
	}
	// Stable sort by match desc; equity order already breaks ties.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Match > out[j-1].Match; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

// Decide runs the full turn: Detect, then fuse the board-diff evidence (and an
// optional observed dice) with the engine's legality filter and equity prior
// into a MoveDecision.
func Decide(pre bg.Position, post perceive.ObservedBoard, observedDice *bg.Dice, tick int, w fusion.Weights) (cue.MoveDecision, error) {
	scored, err := Detect(pre, post)
	if err != nil {
		return cue.MoveDecision{}, err
	}
	if len(scored) == 0 {
		return cue.MoveDecision{Player: pre.PlayerOnRoll, Tick: tick}, nil // dance
	}

	candidates := make([]fusion.Candidate, 0, len(scored))
	priorMap := make(map[string]float64, len(scored))
	best, worst := scored[0].Move.Equity, scored[0].Move.Equity
	for _, s := range scored {
		if s.Move.Equity > best {
			best = s.Move.Equity
		}
		if s.Move.Equity < worst {
			worst = s.Move.Equity
		}
	}
	for _, s := range scored {
		candidates = append(candidates, fusion.Candidate{Dice: pre.Dice, Notation: s.Move.Notation})
		pr := 1.0
		if best > worst {
			pr = (s.Move.Equity - worst) / (best - worst)
		}
		priorMap[s.Move.Notation] = pr
	}
	prior := func(c fusion.Candidate) float64 { return priorMap[c.Notation] }

	cues := []cue.Cue{{Kind: cue.BoardDiff, Tick: tick, Notation: scored[0].Move.Notation, Confidence: scored[0].Match}}
	if observedDice != nil {
		cues = append(cues, cue.Cue{Kind: cue.DiceValue, Tick: tick, Dice: *observedDice, Confidence: 0.9})
	}
	return fusion.Fuse(pre.PlayerOnRoll, tick, cues, candidates, prior, w), nil
}

// Reorient maps a reading from camera-view canonical regions onto the canonical
// bg point numbering by applying the Orientation prior (ADR-0006). The reading
// at region p belongs to point o.TransformPoint(p); bar/off are unaffected.
// Since every Orientation is an involution this is also its own inverse. The
// perception-in boundary calls it right after the board reader, so every
// downstream consumer sees a canonically-numbered board (P1 home = 1..6).
func Reorient(ob perceive.ObservedBoard, o bg.Orientation) perceive.ObservedBoard {
	if o == bg.P1HomeRight {
		return ob // identity fast path
	}
	var out perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		out.Points[o.TransformPoint(p)] = ob.Points[p]
	}
	return out
}

// matchScore is the confidence-weighted fraction of points (1..24) where the
// candidate board agrees with the observation. Uncertain reads count for less.
func matchScore(cand bg.Board, ob perceive.ObservedBoard) float64 {
	var wsum, agree float64
	for p := 1; p <= 24; p++ {
		o := ob.Points[p]
		w := o.Confidence
		if w < 0.05 {
			w = 0.05
		}
		wsum += w
		if cellAgrees(o, cand.Pts[p]) {
			agree += w
		}
	}
	if wsum == 0 {
		return 0
	}
	return agree / wsum
}

func cellAgrees(o perceive.PointObs, c bg.Point) bool {
	if o.Count == 0 {
		return c.N == 0
	}
	return o.Count == c.N && c.N > 0 && playerOf(o.Side) == c.Owner
}

// playerOf maps a perception Side to a player under the default orientation
// (CheckerA = P1). Orientation is a Session Prior; this is the skeleton default.
func playerOf(s perceive.Side) bg.Player {
	if s == perceive.B {
		return bg.P2
	}
	return bg.P1
}

func boardSig(b bg.Board) string {
	var sb strings.Builder
	for i := 1; i <= 24; i++ {
		code := 0
		if b.Pts[i].N > 0 {
			code = int(b.Pts[i].Owner) + 1
		}
		fmt.Fprintf(&sb, "%d.%d,", b.Pts[i].N, code)
	}
	fmt.Fprintf(&sb, "b%d.%d", b.Bar[0], b.Bar[1])
	return sb.String()
}

// rolls21 is every distinct dice roll (high die first).
var rolls21 = func() []bg.Dice {
	var out []bg.Dice
	for a := 1; a <= 6; a++ {
		for b := 1; b <= a; b++ {
			out = append(out, bg.Dice{a, b})
		}
	}
	return out
}()

// signedCount encodes a point as +N for player A/P1, -N for player B/P2 and
// 0 for empty — the space where reading deltas live.
func signedObs(o perceive.PointObs) int {
	if o.Count == 0 || o.Side == perceive.None {
		return 0
	}
	if o.Side == perceive.B {
		return -o.Count
	}
	return o.Count
}

func signedPt(c bg.Point) int {
	if c.N == 0 {
		return 0
	}
	if c.Owner == bg.P2 {
		return -c.N
	}
	return c.N
}

// ReadingShift is the confidence-weighted fraction of points whose reading
// changed between two consecutive observations. 0 means the two readings are
// identical — the same physical position seen twice. It compares readings to
// READINGS, so a systematic per-point misread (a tall stack always counted
// one short, an occluded corner always missed) cancels out instead of making
// every event look like a move.
func ReadingShift(prev, cur perceive.ObservedBoard) float64 {
	var wsum, moved float64
	for p := 1; p <= 24; p++ {
		w := weight(prev.Points[p].Confidence, cur.Points[p].Confidence)
		wsum += w
		if signedObs(prev.Points[p]) != signedObs(cur.Points[p]) {
			moved += w
		}
	}
	if wsum == 0 {
		return 0
	}
	return moved / wsum
}

// WholeBoardMatch is the confidence-weighted per-point agreement of a reading
// with an exact board — tolerant scanning for landmark positions (e.g. "is
// the table back at the standard start?"), NOT a per-move discriminator.
func WholeBoardMatch(b bg.Board, obs perceive.ObservedBoard) float64 {
	return matchScore(b, obs)
}

func weight(a, b float64) float64 {
	w := a
	if b < w {
		w = b
	}
	if w < 0.05 {
		w = 0.05
	}
	return w
}

// DeltaMatch scores a candidate move in delta space: the reading change
// observed between the previous and current observations must equal the
// board change the candidate predicts. Comparing deltas — not absolute
// values — makes stable per-point reader bias invisible (it appears
// identically in both readings). Scored only over points where either side
// changed; 1.0 when the deltas agree everywhere.
//
// This is the per-move discriminator WholeBoardMatch explicitly is not: it
// ignores the ~20 points a single move leaves untouched, instead of averaging
// the answer away across them. Exported so callers that must rank candidate
// moves — and any harness measuring such a ranking — use this rather than
// WholeBoardMatch. It costs one extra input: the reading from BEFORE the move.
func DeltaMatch(pre, cand bg.Board, prev, cur perceive.ObservedBoard) float64 {
	var wsum, agree float64
	for p := 1; p <= 24; p++ {
		predicted := signedPt(cand.Pts[p]) - signedPt(pre.Pts[p])
		observed := signedObs(cur.Points[p]) - signedObs(prev.Points[p])
		if predicted == 0 && observed == 0 {
			continue
		}
		w := weight(prev.Points[p].Confidence, cur.Points[p].Confidence)
		wsum += w
		switch d := predicted - observed; {
		case d == 0:
			agree += w
		case d == 1 || d == -1:
			agree += 0.4 * w // near miss: one checker of the delta misread
		}
	}
	if wsum == 0 {
		return 1
	}
	return agree / wsum
}

// DecideAnyDice explains an observed board transition when the dice were NOT
// seen: it tries every distinct roll, asks the engine for the legal moves of
// each, and scores every candidate (dice, move) by how well its predicted board
// delta matches the observed reading delta (see DeltaMatch), blended with a
// WITHIN-ROLL equity prior (architecture §5: "if the dice were never visible,
// infer the dice set that makes the observed board-diff legal"). The prior is
// within-roll because equity cannot separate two rolls reaching the SAME
// board (1-1 as 8/7 7/6 6/5 6/5 equals 3-1 as 8/5 6/5) — what separates them
// is that a strong player plays near the top of their actual roll, so a line
// that is a poor play for its roll is an unlikely explanation. pre.Dice is
// ignored. Confidence uses the fusion runner-up rule.
//
// observed, when non-nil, is a DiceValue cue: candidates whose roll matches
// it earn the fusion dice weight scaled by the cue's confidence — enough to
// overturn the within-roll prior when two rolls reach the same board, which
// is exactly the ambiguity a dice reading exists to break.
//
// Cost: a fast unscored enumeration prunes the 21 rolls first; the neural-net
// equity evaluation runs only for rolls that can still win — a roll can
// overtake the best match only if it recovers the match deficit through the
// engine prior and the dice-agreement weight.
func DecideAnyDice(pre bg.Position, prev, cur perceive.ObservedBoard, tick int, w fusion.Weights, observed *cue.Cue) (cue.MoveDecision, error) {
	wDice := 0.0
	if observed != nil && (observed.Dice != (bg.Dice{}) || len(observed.DicePMF) > 0) {
		wDice = w.Dice * observed.Confidence
	}
	// diceAgree is each roll's agreement with the observation in [0,1]:
	// soft (mass relative to the PMF's mode) when a distribution is given,
	// binary exact-pair match otherwise.
	maxPMF := 0.0
	if observed != nil {
		for _, v := range observed.DicePMF {
			if v > maxPMF {
				maxPMF = v
			}
		}
	}
	diceAgreeOf := func(d bg.Dice) float64 {
		if observed == nil {
			return 0
		}
		if maxPMF > 0 {
			hi, lo := d[0], d[1]
			if lo > hi {
				hi, lo = lo, hi
			}
			return observed.DicePMF[bg.Dice{hi, lo}] / maxPMF
		}
		o := observed.Dice
		if (d[0] == o[0] && d[1] == o[1]) || (d[0] == o[1] && d[1] == o[0]) {
			return 1
		}
		return 0
	}
	// Phase 1: unscored sweep — best diff match per roll.
	bestMatch := make([]float64, len(rolls21))
	maxMatch := -1.0
	anyLegal := false
	for ri, d := range rolls21 {
		p := pre
		p.Dice = d
		moves, err := engine.LegalMovesUnscored(p)
		if err != nil {
			return cue.MoveDecision{}, err
		}
		if len(moves) == 0 {
			bestMatch[ri] = -1
			continue
		}
		anyLegal = true
		bm := -1.0
		for _, mv := range moves {
			if m := DeltaMatch(pre.Board, mv.Result, prev, cur); m > bm {
				bm = m
			}
		}
		bestMatch[ri] = bm
		if bm > maxMatch {
			maxMatch = bm
		}
	}
	if !anyLegal {
		return cue.MoveDecision{Player: pre.PlayerOnRoll, Tick: tick}, nil
	}

	// Phase 2: scored evaluation of the rolls that can still win.
	margin := 1.0 // keep everything when the board-diff weight is degenerate
	if w.BoardDiff > 0 {
		margin = (w.Engine + wDice) / w.BoardDiff
	}
	type cand struct {
		dice      bg.Dice
		notation  string
		match     float64
		prior     float64
		diceAgree float64
		score     float64
	}
	const perRoll = 3 // top matches kept per roll; the rest cannot win
	var cands []cand
	bestBySig := map[string]int{} // resulting-board signature -> index in cands
	for ri, d := range rolls21 {
		if bestMatch[ri] < maxMatch-margin-1e-9 {
			continue
		}
		p := pre
		p.Dice = d
		moves, err := engine.LegalMoves(p)
		if err != nil {
			return cue.MoveDecision{}, err
		}
		if len(moves) == 0 {
			continue
		}
		bestEq, worstEq := moves[0].Equity, moves[0].Equity
		for _, mv := range moves {
			if mv.Equity > bestEq {
				bestEq = mv.Equity
			}
			if mv.Equity < worstEq {
				worstEq = mv.Equity
			}
		}
		type rollCand struct {
			mv    engine.LegalMove
			match float64
			prior float64
		}
		rcs := make([]rollCand, 0, len(moves))
		seen := map[string]bool{}
		for _, mv := range moves { // equity-sorted: first per board is the best line
			sig := boardSig(mv.Result)
			if seen[sig] {
				continue
			}
			seen[sig] = true
			pr := 1.0
			if bestEq > worstEq {
				pr = (mv.Equity - worstEq) / (bestEq - worstEq)
			}
			rcs = append(rcs, rollCand{mv, DeltaMatch(pre.Board, mv.Result, prev, cur), pr})
		}
		// Keep the top diff matches of this roll.
		for i := 1; i < len(rcs); i++ {
			for j := i; j > 0 && rcs[j].match > rcs[j-1].match; j-- {
				rcs[j], rcs[j-1] = rcs[j-1], rcs[j]
			}
		}
		if len(rcs) > perRoll {
			rcs = rcs[:perRoll]
		}
		for _, rc := range rcs {
			agree := diceAgreeOf(d)
			c := cand{d, rc.mv.Notation, rc.match, rc.prior, agree, 0}
			sig := boardSig(rc.mv.Result)
			if j, ok := bestBySig[sig]; ok {
				// Same resulting board under another roll: keep the likelier
				// explanation — including what the observed dice say, else
				// the dedupe silently discards the very interpretation the
				// dice cue exists to rescue.
				if c.match+c.prior+c.diceAgree*wDice*4 > cands[j].match+cands[j].prior+cands[j].diceAgree*wDice*4 {
					cands[j] = c
				}
				continue
			}
			bestBySig[sig] = len(cands)
			cands = append(cands, c)
		}
	}
	if len(cands) == 0 {
		return cue.MoveDecision{Player: pre.PlayerOnRoll, Tick: tick}, nil
	}

	wsum := w.BoardDiff + w.Engine + wDice
	if wsum <= 0 {
		wsum = 1
	}
	for i := range cands {
		cands[i].score = (w.BoardDiff*cands[i].match + w.Engine*cands[i].prior + wDice*cands[i].diceAgree) / wsum
	}
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && cands[j].score > cands[j-1].score; j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}

	const runnerUpPenalty = 0.3 // mirror fusion.Fuse
	conf := cands[0].score
	if len(cands) > 1 {
		conf -= runnerUpPenalty * cands[1].score
	}
	if conf < 0 {
		conf = 0
	}
	support := []cue.Kind{cue.BoardDiff, cue.EnginePrior}
	if cands[0].diceAgree > 0 {
		support = append(support, cue.DiceValue)
	}
	d := cue.MoveDecision{
		Player: pre.PlayerOnRoll,
		Tick:   tick,
		Top: cue.MoveHypothesis{
			Dice: cands[0].dice, Notation: cands[0].notation,
			Confidence: conf, Support: support,
		},
		Confidence: conf,
	}
	for _, c := range cands[1:min(len(cands), 5)] {
		d.Alternatives = append(d.Alternatives, cue.MoveHypothesis{
			Dice: c.dice, Notation: c.notation, Confidence: c.score,
			Support: []cue.Kind{cue.BoardDiff, cue.EnginePrior},
		})
	}
	return d, nil
}
