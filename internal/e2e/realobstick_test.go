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
	"lazybg/internal/engine"
	"lazybg/internal/eval"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/perceive/pointnet"
	"lazybg/internal/session"
	"lazybg/internal/transcribe"
)

// obsFractions are the instants the observation is taken at, expressed as a
// position between the PREVIOUS turn's settled tick (0.0) and this turn's
// settled tick (1.0).
//
// 1.0 is what the #69 campaign measured: the board after the move, settled.
// 0.0 is the board BEFORE the move — what the app reads if the human types the
// roll the moment the dice land, since EnterDiceAt observes at nowTickMs() and
// functional-spec §4 explicitly refuses to stop the video for entry.
var obsFractions = []float64{0.00, 0.50, 0.75, 0.90, 1.00}

// guardThresholds are the self-diagnosis bars tested for issue #73 option 2:
// use the observation ONLY if some legal candidate for the entered roll comes
// within this agreement of it. A reading of the board BEFORE the move cannot
// be reached by any candidate (every candidate has moved something), so its
// best agreement stays below a perfect one — that gap is the signal, and it
// needs no stable-window search and no perceptual confidence estimate.
var guardThresholds = []float64{0.90, 0.95, 0.98}

// relativeMargins are the bars for the RELATIVE guard: use the observation only
// if some candidate explains it better than "nothing moved yet" does, by at
// least this margin. The comparison board is free — it is the session's own
// current board (s.board). An absolute bar cannot work because it conflates
// "is this reading post-move?" with "is this reading accurate?", and reader
// noise makes the two distributions overlap; a relative test asks only the
// first question, and reader noise hits both sides of it equally.
var relativeMargins = []float64{0.00, 0.01, 0.03}

// relativeGuardedRank stays silent while the board still looks unmoved.
func relativeGuardedRank(moves []engine.LegalMove, pre bg.Board, obs perceive.ObservedBoard, margin float64) []engine.LegalMove {
	best := 0.0
	for _, m := range moves {
		if a := boarddiff.WholeBoardMatch(m.Result, obs); a > best {
			best = a
		}
	}
	if best <= boarddiff.WholeBoardMatch(pre, obs)+margin {
		return session.RankLegalMoves(moves, nil)
	}
	return session.RankLegalMoves(moves, &obs)
}

// guardedRank ranks with the observation only when it passes the bar, else
// falls back to equity-only — the "know when to say nothing" the repo doctrine
// has demanded since three cues were lost to being too talkative.
func guardedRank(moves []engine.LegalMove, obs perceive.ObservedBoard, bar float64) []engine.LegalMove {
	best := 0.0
	for _, m := range moves {
		if a := boarddiff.WholeBoardMatch(m.Result, obs); a > best {
			best = a
		}
	}
	if best < bar {
		return session.RankLegalMoves(moves, nil)
	}
	return session.RankLegalMoves(moves, &obs)
}

// TestRealCorpus_ObservationTickSensitivity measures how fast the perceptual
// ranking decays when the observation is taken at the wrong instant — the open
// question issue #73 turns on.
//
// The concern is not that an early reading is merely uninformative. It is that
// it votes: WholeBoardMatch will rank highest the candidates whose resulting
// board most resembles what was read, and if what was read is the PRE-move
// board, the candidates that change least win. A wrong observation can push the
// truth below where pure equity would have left it.
func TestRealCorpus_ObservationTickSensitivity(t *testing.T) {
	if testing.Short() {
		t.Skip("long: decodes real frames at several instants per turn")
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
	limit := 40
	if v := os.Getenv("LAZYBG_RANK_LIMIT"); v != "" {
		limit, _ = strconv.Atoi(v)
	}

	// equity-only is the floor the perceptual ranking must never fall below:
	// if a badly-timed observation ranks WORSE than no observation at all, the
	// app is better off staying silent.
	byFrac := map[float64][]eval.TurnRank{}
	guarded := map[guardKey][]eval.TurnRank{}
	relGuarded := map[guardKey][]eval.TurnRank{}
	var equity []eval.TurnRank
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
		read := newTickReader(t, root, m, net)

		var pairs [][2]corpus.Turn
		for j := 1; j < len(m.Turns); j++ {
			if usablePair(replay, m.Turns[j-1], m.Turns[j]) {
				pairs = append(pairs, [2]corpus.Turn{m.Turns[j-1], m.Turns[j]})
			}
		}
		if len(pairs) > limit {
			stride := float64(len(pairs)) / float64(limit)
			sampled := make([][2]corpus.Turn, 0, limit)
			for i := 0; i < limit; i++ {
				sampled = append(sampled, pairs[int(float64(i)*stride)])
			}
			pairs = sampled
		}

		venueRanks := map[float64][]eval.TurnRank{}
		for _, pr := range pairs {
			prevTurn, curTurn := pr[0], pr[1]
			if prevTurn.Part != curTurn.Part {
				continue // a Part cut between the two ticks: no common timeline
			}
			ts := replay[curTurn.Index-1]
			truthPly := bg.Ply{Player: ts.Player, Dice: ts.Dice, Notation: ts.Notation}
			moves, err := engine.LegalMoves(bg.Position{
				Board: ts.Pre, Dice: ts.Dice, PlayerOnRoll: ts.Player})
			if err != nil {
				t.Fatalf("%s turn %d: %v", id, curTurn.Index, err)
			}
			eqRank := eval.RankTruth(
				decisionOf(ts.Player, ts.Dice, session.RankLegalMoves(moves, nil), session.MaxCandidates),
				truthPly)

			ok := true
			perFrac := map[float64]eval.TurnRank{}
			for _, f := range obsFractions {
				tick := prevTurn.TickMs + int(f*float64(curTurn.TickMs-prevTurn.TickMs))
				ob, got := read(curTurn.Part, tick)
				if !got {
					ok = false
					break
				}
				perFrac[f] = eval.RankTruth(
					decisionOf(ts.Player, ts.Dice, session.RankLegalMoves(moves, &ob), session.MaxCandidates),
					truthPly)
			}
			if !ok {
				continue
			}
			equity = append(equity, eqRank)
			for f, r := range perFrac {
				venueRanks[f] = append(venueRanks[f], r)
				byFrac[f] = append(byFrac[f], r)
			}
			for _, margin := range relativeMargins {
				for _, f := range []float64{0.00, 1.00} {
					tick := prevTurn.TickMs + int(f*float64(curTurn.TickMs-prevTurn.TickMs))
					ob, _ := read(curTurn.Part, tick)
					k := guardKey{margin, f}
					relGuarded[k] = append(relGuarded[k], eval.RankTruth(
						decisionOf(ts.Player, ts.Dice, relativeGuardedRank(moves, ts.Pre, ob, margin), session.MaxCandidates),
						truthPly))
				}
			}
			for _, bar := range guardThresholds {
				for _, f := range []float64{0.00, 1.00} {
					tick := prevTurn.TickMs + int(f*float64(curTurn.TickMs-prevTurn.TickMs))
					ob, _ := read(curTurn.Part, tick)
					k := guardKey{bar, f}
					guarded[k] = append(guarded[k], eval.RankTruth(
						decisionOf(ts.Player, ts.Dice, guardedRank(moves, ob, bar), session.MaxCandidates),
						truthPly))
				}
			}
		}
		if len(venueRanks[1.0]) == 0 {
			continue
		}
		measured++
		t.Logf("── %s (%d tours) ──", id, len(venueRanks[1.0]))
		for _, f := range obsFractions {
			logHistogram(t, fmt.Sprintf("  obs @%.2f ", f), eval.Histogram(venueRanks[f]))
		}
	}
	if measured == 0 {
		t.Skip("no corpus video present")
	}

	t.Logf("══ TOUTES VENUES (%d recordings, %d tours) ══", measured, len(equity))
	eq := eval.Histogram(equity)
	logHistogram(t, "  équité seule", eq)
	for _, f := range obsFractions {
		logHistogram(t, fmt.Sprintf("  obs @%.2f ", f), eval.Histogram(byFrac[f]))
	}

	// The decision this test exists to inform: is an observation taken at the
	// WRONG instant worse than no observation at all? If so, "observe at
	// nowTickMs()" is not a safe default and issue #73 must pick an instant.
	t.Logf("── garde auto-diagnostique (option 2 de #73) ──")
	for _, bar := range guardThresholds {
		logHistogram(t, fmt.Sprintf("  bar %.2f, obs pré-coup ", bar), eval.Histogram(guarded[guardKey{bar, 0.00}]))
		logHistogram(t, fmt.Sprintf("  bar %.2f, obs post-coup", bar), eval.Histogram(guarded[guardKey{bar, 1.00}]))
	}

	t.Logf("── garde RELATIVE (le plateau a-t-il seulement bougé ?) ──")
	for _, margin := range relativeMargins {
		logHistogram(t, fmt.Sprintf("  marge %.2f, pré-coup ", margin), eval.Histogram(relGuarded[guardKey{margin, 0.00}]))
		logHistogram(t, fmt.Sprintf("  marge %.2f, post-coup", margin), eval.Histogram(relGuarded[guardKey{margin, 1.00}]))
	}

	worst := eval.Histogram(byFrac[0.0])
	t.Logf("verdict: obs pré-coup rang-1 %.1f%% vs équité seule %.1f%%",
		100*worst.WithinTop(1), 100*eq.WithinTop(1))
	if eq.N == 0 || worst.N == 0 {
		t.Fatal("no turns measured")
	}
}

// newTickReader returns a board reader over a manifest's Parts, caching each
// (part, tick) so the same instant is decoded once.
func newTickReader(t *testing.T, root string, m corpus.Manifest, net *pointnet.Net) func(part, tick int) (perceive.ObservedBoard, bool) {
	t.Helper()
	readers := make([]func(tick int) (perceive.ObservedBoard, bool), len(m.Parts))
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

// guardKey indexes a guarded measurement by (bar, observation instant).
type guardKey struct{ bar, frac float64 }
