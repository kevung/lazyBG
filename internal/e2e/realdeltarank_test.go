package e2e

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/derive"
	"lazybg/internal/engine"
	"lazybg/internal/eval"
	"lazybg/internal/fusion"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/perceive/pointnet"
	"lazybg/internal/session"
	"lazybg/internal/transcribe"
)

// TestRealCorpus_DeltaVsWholeBoardRanking answers issue #74: the assisted
// candidate list is scored with WholeBoardMatch, whose own doc comment says it
// is "NOT a per-move discriminator" — it averages agreement over all 24 points
// while two candidates for one roll differ on 2-4. DeltaMatch, which
// DecideAnyDice already uses, scores only the points that changed.
//
// Three lines per turn, all with the dice known (the assisted path):
//
//	equity        no observation at all — the silent-fallback regime
//	whole-board   what the app ships (session.rankMoves)
//	delta         the candidate, at the cost of retaining the PREVIOUS reading
//
// The verdict this must produce is the honest one, including "no": the
// whole-board line already sits at 93.8% rank-1 (issue #69), so the delta line
// has at most ~6 points to win, against real added state in the session.
func TestRealCorpus_DeltaVsWholeBoardRanking(t *testing.T) {
	if testing.Short() {
		t.Skip("long: decodes real frames")
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
	limit := 60
	if v := os.Getenv("LAZYBG_RANK_LIMIT"); v != "" {
		limit, _ = strconv.Atoi(v)
	}

	var allEq, allWhole, allDelta []eval.TurnRank
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
		read := newDeltaTickReader(t, root, m, net)

		var pairs [][2]corpus.Turn
		for j := 1; j < len(m.Turns); j++ {
			if usablePair(replay, m.Turns[j-1], m.Turns[j]) {
				pairs = append(pairs, [2]corpus.Turn{m.Turns[j-1], m.Turns[j]})
			}
		}
		if len(pairs) > limit {
			stride := float64(len(pairs)) / float64(limit)
			s := make([][2]corpus.Turn, 0, limit)
			for i := 0; i < limit; i++ {
				s = append(s, pairs[int(float64(i)*stride)])
			}
			pairs = s
		}

		var eq, whole, delta []eval.TurnRank
		for _, pr := range pairs {
			prevTurn, curTurn := pr[0], pr[1]
			ts := replay[curTurn.Index-1]
			curObs, ok := read(curTurn.Part, curTurn.TickMs)
			if !ok {
				continue
			}
			prevObs, ok := read(prevTurn.Part, prevTurn.TickMs)
			if !ok {
				continue
			}
			moves, err := engine.LegalMoves(bg.Position{
				Board: ts.Pre, Dice: ts.Dice, PlayerOnRoll: ts.Player})
			if err != nil {
				t.Fatalf("%s turn %d: %v", id, curTurn.Index, err)
			}
			truthPly := bg.Ply{Player: ts.Player, Dice: ts.Dice, Notation: ts.Notation}
			rank := func(order []engine.LegalMove) eval.TurnRank {
				return eval.RankTruth(
					decisionOf(ts.Player, ts.Dice, order, session.MaxCandidates), truthPly)
			}
			eq = append(eq, rank(session.RankLegalMoves(moves, nil)))
			whole = append(whole, rank(session.RankLegalMoves(moves, &curObs)))
			delta = append(delta, rank(rankByDelta(moves, ts.Pre, prevObs, curObs)))
		}
		if len(eq) == 0 {
			continue
		}
		measured++
		t.Logf("── %s (%d tours) ──", id, len(eq))
		logHistogram(t, "  équité      ", eval.Histogram(eq))
		logHistogram(t, "  plateau     ", eval.Histogram(whole))
		logHistogram(t, "  delta       ", eval.Histogram(delta))
		allEq = append(allEq, eq...)
		allWhole = append(allWhole, whole...)
		allDelta = append(allDelta, delta...)
	}
	if measured == 0 {
		t.Skip("no corpus video present")
	}
	hw, hd := eval.Histogram(allWhole), eval.Histogram(allDelta)
	t.Logf("══ TOUTES VENUES (%d recordings, %d tours) ══", measured, len(allEq))
	logHistogram(t, "  équité      ", eval.Histogram(allEq))
	logHistogram(t, "  plateau     ", hw)
	logHistogram(t, "  delta       ", hd)
	t.Logf("verdict: rang-1 plateau %.1f%% → delta %.1f%% (%+.1f pts) ; top-3 %.1f%% → %.1f%%",
		100*hw.WithinTop(1), 100*hd.WithinTop(1),
		100*(hd.WithinTop(1)-hw.WithinTop(1)),
		100*hw.WithinTop(3), 100*hd.WithinTop(3))
}

// rankByDelta mirrors session.rankMoves' blend exactly, swapping the board-diff
// term for DeltaMatch. Kept in the test because #74 may well conclude "not
// worth the added session state" — production code should not grow a variant
// before the measurement earns it.
func rankByDelta(moves []engine.LegalMove, pre bg.Board, prev, cur perceive.ObservedBoard) []engine.LegalMove {
	if len(moves) == 0 {
		return nil
	}
	w := fusion.DefaultWeights()
	best, worst := moves[0].Equity, moves[0].Equity
	for _, m := range moves {
		if m.Equity > best {
			best = m.Equity
		}
		if m.Equity < worst {
			worst = m.Equity
		}
	}
	type scored struct {
		mv    engine.LegalMove
		score float64
	}
	out := make([]scored, len(moves))
	for i, m := range moves {
		prior := 1.0
		if best > worst {
			prior = (m.Equity - worst) / (best - worst)
		}
		agree := boarddiff.DeltaMatch(pre, m.Result, prev, cur)
		out[i] = scored{m, (w.BoardDiff*agree + w.Engine*prior) / (w.BoardDiff + w.Engine)}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	ranked := make([]engine.LegalMove, len(out))
	for i, s := range out {
		ranked[i] = s.mv
	}
	return ranked
}

// newDeltaTickReader caches one board reading per (part, tick).
func newDeltaTickReader(t *testing.T, root string, m corpus.Manifest, net *pointnet.Net) func(part, tick int) (perceive.ObservedBoard, bool) {
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
