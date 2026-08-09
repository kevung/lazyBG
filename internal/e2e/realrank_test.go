package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/calibrate"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/cue"
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

// rankVenues are the recordings the rank campaign runs on: three distinct
// venues, each picked for having the most consecutively-aligned truth turns
// (issue #69 asks for >= 3 venues; a fourth is included for margin).
var rankVenues = []string{
	"2025-05_hsbtMarseille_main-r1_PavicevicNina", // Marseille 2025 (the pilot capture)
	"2025-11_hsbtParis_r3_MarieAnneJeannel",       // Paris 2025
	"2025-10_cdfAngers_r1_EricBenichou",           // Angers 2025
	"2025-10_vbc_r1_JamesMcNaughtan",              // VBC 2025 (two Parts)
}

// TestRealCorpus_TruthRankAcrossVenues measures WHERE THE TRUTH SITS in the
// ranked candidate lists the product produces — the number issue #69 needs and
// that coverage/error rate cannot show, because they only ever look at the top
// candidate.
//
// Three lists are measured per turn, because the product has three:
//
//	equity      what the review UI shows TODAY (nothing calls SetObservation,
//	            so rankMoves runs with a nil observation: pure engine equity)
//	equity+obs  what it would show if the post-move reading were wired in
//	blind       DecideAnyDice with the dice unknown — the list that feeds
//	            auto-fill, and the only one whose top candidate can be gated
//
// The measurement is TRUTH-ANCHORED: it reads the board at the manifest's
// aligned ticks and hands the conductor's job (which turn is this? whose is
// it?) to the ground truth. That deliberately removes turn-segmentation error
// so the ranking itself is what is being measured. It is therefore an upper
// bound on what the blind pipeline can do, and a fair estimate of the assisted
// path, where the human supplies the segmentation by scrubbing anyway.
func TestRealCorpus_TruthRankAcrossVenues(t *testing.T) {
	if testing.Short() {
		t.Skip("long: decodes hundreds of real frames")
	}
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("point reader not present: %v", err)
	}
	net, err := pointnet.Load(modelPath)
	if err != nil {
		t.Fatal(err)
	}

	// Worktrees carry the committed manifests but not the machine-local
	// videos; LAZYBG_CORPUS_ROOT points the campaign at the checkout that has
	// them (same convention as the autocal bench).
	root := repoRoot
	if v := os.Getenv("LAZYBG_CORPUS_ROOT"); v != "" {
		root = v
	}

	// Turns measured per recording. Each costs ~3 s of perception plus a
	// DecideAnyDice that grows with the position's legal-move count, so the
	// default keeps a full run inside about an hour; the campaign that answers
	// issue #69 raises it. Sampling is EVEN across the recording, never the
	// first N — early-game positions are systematically easier.
	limit := 60
	if v := os.Getenv("LAZYBG_RANK_LIMIT"); v != "" {
		limit, _ = strconv.Atoi(v)
	}

	var all struct{ equity, obs, blind []eval.TurnRank }
	measured := 0
	for _, id := range rankVenues {
		r := measureVenue(t, root, id, net, limit)
		if r == nil {
			continue
		}
		measured++
		all.equity = append(all.equity, r.equity...)
		all.obs = append(all.obs, r.obs...)
		all.blind = append(all.blind, r.blind...)
		t.Logf("── %s (%d turns) ───────────────", id, len(r.equity))
		logHistogram(t, "  equity     ", eval.Histogram(r.equity))
		logHistogram(t, "  equity+obs ", eval.Histogram(r.obs))
		logHistogram(t, "  blind      ", eval.Histogram(r.blind))
	}
	if measured == 0 {
		t.Skip("no corpus video present")
	}
	t.Logf("══ ALL VENUES (%d recordings, %d turns) ══", measured, len(all.equity))
	logHistogram(t, "  equity     ", eval.Histogram(all.equity))
	logHistogram(t, "  equity+obs ", eval.Histogram(all.obs))
	logHistogram(t, "  blind      ", eval.Histogram(all.blind))

	if measured < 3 {
		t.Skipf("only %d venues present locally; issue #69 wants >= 3", measured)
	}
	// Regression floor, not a target: the campaign's measured numbers are the
	// deliverable. This only catches a wholesale collapse of the ranking.
	if h := eval.Histogram(all.blind); h.N > 0 && h.WithinTop(5) == 0 {
		t.Error("the blind ranker never contains the truth — something is structurally broken")
	}
}

type venueRanks struct{ equity, obs, blind []eval.TurnRank }

// measureVenue runs one recording, returning nil when its video or transcript
// is not on this machine.
func measureVenue(t *testing.T, root, id string, net *pointnet.Net, limit int) *venueRanks {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "corpus/manifest", id+".json"))
	if err != nil {
		t.Logf("%s: manifest absent, skipped", id)
		return nil
	}
	m, err := corpus.Load(raw)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	for _, p := range m.Parts {
		if _, err := os.Stat(filepath.Join(root, p.File)); err != nil {
			t.Logf("%s: video absent, skipped", id)
			return nil
		}
	}
	matRaw, err := os.ReadFile(filepath.Join(root, m.Transcript))
	if err != nil {
		t.Logf("%s: transcript absent, skipped", id)
		return nil
	}
	truth, err := matimport.Parse(string(matRaw))
	if err != nil {
		t.Fatalf("%s: parse .mat: %v", id, err)
	}
	replay := derive.Replay(truth)

	// Per-Part perception setup.
	type partSetup struct {
		cal    calibrate.BoardCalibration
		cb     calibrate.CanonicalBoard
		orient bg.Orientation
		video  string
	}
	setups := make([]partSetup, len(m.Parts))
	for i, p := range m.Parts {
		cal, cb, _, err := transcribe.PartSetup(p)
		if err != nil {
			t.Fatalf("%s part %d: %v", id, i, err)
		}
		orient, _ := bg.ParseOrientation(p.Priors.Orientation)
		setups[i] = partSetup{cal, cb, orient, filepath.Join(root, p.File)}
	}

	// Board readings are cached: each aligned tick serves as the "current"
	// reading of its own turn and the "previous" reading of the next one.
	type tickKey struct{ part, tick int }
	cache := map[tickKey]perceive.ObservedBoard{}
	readAt := func(part, tick int) (perceive.ObservedBoard, bool) {
		k := tickKey{part, tick}
		if ob, ok := cache[k]; ok {
			return ob, true
		}
		s := setups[part]
		frame, err := capture.FrameAt(s.video, tick)
		if err != nil {
			return perceive.ObservedBoard{}, false
		}
		ob := boarddiff.Reorient(pointnet.Reader{Net: net}.Read(s.cal.RectifyMasked(frame), s.cb), s.orient)
		cache[k] = ob
		return ob, true
	}

	// Every pair that can bracket a single move, then an even subsample.
	var pairs [][2]corpus.Turn
	for j := 1; j < len(m.Turns); j++ {
		if usablePair(replay, m.Turns[j-1], m.Turns[j]) {
			pairs = append(pairs, [2]corpus.Turn{m.Turns[j-1], m.Turns[j]})
		}
	}
	total := len(pairs)
	if limit > 0 && total > limit {
		stride := float64(total) / float64(limit)
		sampled := make([][2]corpus.Turn, 0, limit)
		for i := 0; i < limit; i++ {
			sampled = append(sampled, pairs[int(float64(i)*stride)])
		}
		pairs = sampled
	}

	w := fusion.DefaultWeights()
	out := &venueRanks{}
	for _, pr := range pairs {
		prevTurn, curTurn := pr[0], pr[1]
		ts := replay[curTurn.Index-1]

		curObs, ok := readAt(curTurn.Part, curTurn.TickMs)
		if !ok {
			continue
		}
		prevObs, ok := readAt(prevTurn.Part, prevTurn.TickMs)
		if !ok {
			continue
		}

		truthPly := bg.Ply{Player: ts.Player, Dice: ts.Dice, Notation: ts.Notation}

		// (1) and (2): the dice are known — the assisted path.
		moves, err := engine.LegalMoves(bg.Position{
			Board: ts.Pre, Dice: ts.Dice, PlayerOnRoll: ts.Player})
		if err != nil {
			t.Fatalf("%s turn %d: %v", id, curTurn.Index, err)
		}
		eqDec := decisionOf(ts.Player, ts.Dice, session.RankLegalMoves(moves, nil), session.MaxCandidates)
		obsDec := decisionOf(ts.Player, ts.Dice, session.RankLegalMoves(moves, &curObs), session.MaxCandidates)

		// (3): the dice are unknown — the blind path that feeds auto-fill.
		blindDec, err := boarddiff.DecideAnyDice(
			bg.Position{Board: ts.Pre, PlayerOnRoll: ts.Player},
			prevObs, curObs, curTurn.TickMs, w, nil)
		if err != nil {
			t.Fatalf("%s turn %d: DecideAnyDice: %v", id, curTurn.Index, err)
		}

		mark := func(r eval.TurnRank) eval.TurnRank { r.Index = curTurn.Index; return r }
		out.equity = append(out.equity, mark(eval.RankTruth(eqDec, truthPly)))
		out.obs = append(out.obs, mark(eval.RankTruth(obsDec, truthPly)))
		out.blind = append(out.blind, mark(eval.RankTruth(blindDec, truthPly)))
	}
	if len(out.equity) == 0 {
		t.Logf("%s: no usable consecutive turn pair", id)
		return nil
	}
	if total > len(out.equity) {
		t.Logf("%s: %d turns measured, evenly sampled from %d usable pairs", id, len(out.equity), total)
	}
	return out
}

// usablePair reports whether two aligned turns can bracket a single move: same
// game, and nothing that MOVED A CHECKER between them. A gap the alignment
// skipped would make the previous reading stale by several moves, and the
// read-to-read delta meaningless.
func usablePair(replay []derive.TurnState, prev, cur corpus.Turn) bool {
	if prev.Index < 1 || cur.Index < 1 || cur.Index > len(replay) || prev.Index >= cur.Index {
		return false
	}
	a, b := replay[prev.Index-1], replay[cur.Index-1]
	if a.Game != b.Game || b.Err != nil || !b.Applied {
		return false
	}
	for i := prev.Index; i < cur.Index-1; i++ { // strictly between
		if replay[i].Applied {
			return false
		}
	}
	return true
}

// decisionOf turns a ranked move list into the decision shape RankTruth reads,
// capped the way the UI caps it.
func decisionOf(player bg.Player, dice bg.Dice, order []engine.LegalMove, cap int) cue.MoveDecision {
	if len(order) == 0 {
		return cue.MoveDecision{Player: player}
	}
	if len(order) > cap {
		order = order[:cap]
	}
	d := cue.MoveDecision{Player: player}
	d.Top = cue.MoveHypothesis{Dice: dice, Notation: order[0].Notation}
	for _, m := range order[1:] {
		d.Alternatives = append(d.Alternatives, cue.MoveHypothesis{Dice: dice, Notation: m.Notation})
	}
	return d
}

func logHistogram(t *testing.T, label string, h eval.RankHistogram) {
	t.Helper()
	if h.N == 0 {
		t.Logf("%s n=0", label)
		return
	}
	pct := func(n int) string { return fmt.Sprintf("%4.1f%%", 100*float64(n)/float64(h.N)) }
	t.Logf("%s n=%-4d top1=%s  2-3=%s  4+=%s  absent=%s (dont roll jamais proposé: %d)",
		label, h.N, pct(h.Top1), pct(h.Rank2to3), pct(h.Rank4Plus), pct(h.Absent), h.AbsentRollMissing)
}
