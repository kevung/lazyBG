package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/derive"
	"lazybg/internal/eval"
	"lazybg/internal/fusion"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/perceive/pointnet"
	"lazybg/internal/transcribe"
)

// coherenceWeights are the strengths the next turn's evidence is given when
// re-ranking this turn's candidates.
var coherenceWeights = []float64{0.5, 1.0, 2.0}

// TestRealCorpus_SequenceCoherence answers issue #72: can a turn the blind
// ranker got wrong be decided AFTER THE FACT by its successor?
//
// A backgammon game is a heavily constrained sequence — the board at turn k
// must both come from k-1 and make k+1's observed transition explicable. Today
// DecideAnyDice reasons over one transition at a time and never uses that.
//
// The prototype: for each candidate of turn k, apply it, then ask
// DecideAnyDice how well it can explain turn k+1 from the resulting board. That
// confidence becomes lookahead evidence, and eval.Rescore re-ranks turn k with
// it. What is measured is the RANK OF THE TRUTH before and after — never the
// chain's internal agreement, which a coherence pass can always manufacture
// (docs/experiment-plan.md §6: three cues made more talkative, three losses).
func TestRealCorpus_SequenceCoherence(t *testing.T) {
	if testing.Short() {
		t.Skip("long: several DecideAnyDice calls per turn on real frames")
	}
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("point reader not present: %v", err)
	}
	net, err := pointnet.Load(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	root := repoRoot
	if v := os.Getenv("LAZYBG_CORPUS_ROOT"); v != "" {
		root = v
	}
	limit := 30
	if v := os.Getenv("LAZYBG_RANK_LIMIT"); v != "" {
		limit, _ = strconv.Atoi(v)
	}

	var base []eval.TurnRank
	byWeight := map[float64][]eval.TurnRank{}
	measured := 0

	for _, id := range rankVenues {
		raw, err := os.ReadFile(filepath.Join(root, "corpus/manifest", id+".json"))
		if err != nil {
			continue
		}
		m, err := corpus.Load(raw)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		missing := false
		for _, p := range m.Parts {
			if _, err := os.Stat(filepath.Join(root, p.File)); err != nil {
				missing = true
			}
		}
		matRaw, err := os.ReadFile(filepath.Join(root, m.Transcript))
		if missing || err != nil {
			t.Logf("%s: video or transcript absent, skipped", id)
			continue
		}
		truth, err := matimport.Parse(string(matRaw))
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		replay := derive.Replay(truth)
		read := newCoherenceReader(t, root, m, net)

		// Triples: three aligned turns in a row, each consecutive pair usable,
		// so both transitions are single moves on a common timeline.
		var triples [][3]corpus.Turn
		for j := 2; j < len(m.Turns); j++ {
			a, b, c := m.Turns[j-2], m.Turns[j-1], m.Turns[j]
			if usablePair(replay, a, b) && usablePair(replay, b, c) &&
				a.Part == b.Part && b.Part == c.Part {
				triples = append(triples, [3]corpus.Turn{a, b, c})
			}
		}
		if len(triples) > limit {
			stride := float64(len(triples)) / float64(limit)
			s := make([][3]corpus.Turn, 0, limit)
			for i := 0; i < limit; i++ {
				s = append(s, triples[int(float64(i)*stride)])
			}
			triples = s
		}

		w := fusion.DefaultWeights()
		var vBase []eval.TurnRank
		vByWeight := map[float64][]eval.TurnRank{}
		for _, tr := range triples {
			prevT, curT, nextT := tr[0], tr[1], tr[2]
			ts := replay[curT.Index-1]
			prevObs, ok1 := read(prevT.Part, prevT.TickMs)
			curObs, ok2 := read(curT.Part, curT.TickMs)
			nextObs, ok3 := read(nextT.Part, nextT.TickMs)
			if !ok1 || !ok2 || !ok3 {
				continue
			}
			d, err := boarddiff.DecideAnyDice(
				bg.Position{Board: ts.Pre, PlayerOnRoll: ts.Player},
				prevObs, curObs, curT.TickMs, w, nil)
			if err != nil {
				t.Fatalf("%s turn %d: %v", id, curT.Index, err)
			}
			cands := eval.Candidates(d)
			if len(cands) == 0 {
				continue
			}
			truthPly := bg.Ply{Player: ts.Player, Dice: ts.Dice, Notation: ts.Notation}

			// Lookahead: how well can the NEXT transition be explained if this
			// candidate were true?
			look := make([]float64, len(cands))
			for i, c := range cands {
				after, err := derive.ApplyNotation(ts.Pre, ts.Player, c.Notation)
				if err != nil {
					continue // unapplicable candidate earns no support
				}
				dn, err := boarddiff.DecideAnyDice(
					bg.Position{Board: after, PlayerOnRoll: otherOf(ts.Player)},
					curObs, nextObs, nextT.TickMs, w, nil)
				if err != nil || dn.Top.Notation == "" {
					continue
				}
				look[i] = dn.Confidence
			}
			vBase = append(vBase, eval.RankTruth(d, truthPly))
			for _, lw := range coherenceWeights {
				vByWeight[lw] = append(vByWeight[lw], eval.RankTruth(eval.Rescore(d, look, lw), truthPly))
			}
		}
		if len(vBase) == 0 {
			continue
		}
		measured++
		t.Logf("── %s (%d tours) ──", id, len(vBase))
		logHistogram(t, "  aveugle seul  ", eval.Histogram(vBase))
		for _, lw := range coherenceWeights {
			logHistogram(t, fmt.Sprintf("  + suite w=%.1f ", lw), eval.Histogram(vByWeight[lw]))
		}
		base = append(base, vBase...)
		for _, lw := range coherenceWeights {
			byWeight[lw] = append(byWeight[lw], vByWeight[lw]...)
		}
	}
	if measured == 0 {
		t.Skip("no corpus video present")
	}

	hb := eval.Histogram(base)
	t.Logf("══ TOUTES VENUES (%d recordings, %d tours) ══", measured, len(base))
	logHistogram(t, "  aveugle seul  ", hb)
	for _, lw := range coherenceWeights {
		h := eval.Histogram(byWeight[lw])
		logHistogram(t, fmt.Sprintf("  + suite w=%.1f ", lw), h)
		t.Logf("     verdict w=%.1f : rang-1 %+.1f pts, top-3 %+.1f pts", lw,
			100*(h.WithinTop(1)-hb.WithinTop(1)), 100*(h.WithinTop(3)-hb.WithinTop(3)))
	}
	if hb.N == 0 {
		t.Fatal("no turns measured")
	}
}

func otherOf(p bg.Player) bg.Player {
	if p == bg.P1 {
		return bg.P2
	}
	return bg.P1
}

// newCoherenceReader caches one board reading per (part, tick); a triple shares
// two of its three ticks with each neighbour.
func newCoherenceReader(t *testing.T, root string, m corpus.Manifest, net *pointnet.Net) func(part, tick int) (perceive.ObservedBoard, bool) {
	t.Helper()
	readers := make([]func(int) (perceive.ObservedBoard, bool), len(m.Parts))
	for i, p := range m.Parts {
		cal, cb, _, err := transcribe.PartSetup(p)
		if err != nil {
			t.Fatalf("part %d: %v", i, err)
		}
		orient, _ := bg.ParseOrientation(p.Priors.Orientation)
		video := filepath.Join(root, p.File)
		readers[i] = func(tick int) (perceive.ObservedBoard, bool) {
			frame, err := capture.FrameAt(video, tick)
			if err != nil {
				return perceive.ObservedBoard{}, false
			}
			return boarddiff.Reorient(pointnet.Reader{Net: net}.Read(cal.RectifyMasked(frame), cb), orient), true
		}
	}
	type key struct{ part, tick int }
	cache := map[key]perceive.ObservedBoard{}
	return func(part, tick int) (perceive.ObservedBoard, bool) {
		k := key{part, tick}
		if ob, ok := cache[k]; ok {
			return ob, true
		}
		ob, ok := readers[part](tick)
		if !ok {
			return perceive.ObservedBoard{}, false
		}
		cache[k] = ob
		return ob, true
	}
}
