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

// BoardFromObserved converts a perception reading into an absolute board. sideA
// declares which player CheckerA (perceive.A) belongs to — an orientation prior.
func BoardFromObserved(ob perceive.ObservedBoard, sideA bg.Player) bg.Board {
	var b bg.Board
	for p := 1; p <= 24; p++ {
		o := ob.Points[p]
		if o.Count == 0 || o.Side == perceive.None {
			continue
		}
		who := sideA
		if o.Side == perceive.B {
			who = otherPlayer(sideA)
		}
		b.Pts[p] = bg.Point{N: o.Count, Owner: who}
	}
	return b
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

func otherPlayer(p bg.Player) bg.Player {
	if p == bg.P1 {
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
