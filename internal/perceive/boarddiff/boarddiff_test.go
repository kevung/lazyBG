package boarddiff

import (
	"os"
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/cue"
	"lazybg/internal/engine"
	"lazybg/internal/fusion"
	"lazybg/internal/perceive"
)

func TestMain(m *testing.M) {
	if err := engine.Init(os.DirFS("../../..")); err != nil {
		panic("engine init: " + err.Error())
	}
	os.Exit(m.Run())
}

// obsFromBoard fabricates a clean, fully-confident observation of a board (the
// stand-in for what the board-state reader would produce).
func obsFromBoard(b bg.Board) perceive.ObservedBoard {
	var ob perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		c := b.Pts[p]
		if c.N == 0 {
			ob.Points[p] = perceive.PointObs{Confidence: 1}
			continue
		}
		side := perceive.A
		if c.Owner == bg.P2 {
			side = perceive.B
		}
		ob.Points[p] = perceive.PointObs{Count: c.N, Side: side, Confidence: 1}
	}
	return ob
}

// bestResult returns the engine's top move and its resulting board for a position.
func bestResult(t *testing.T, pos bg.Position) engine.LegalMove {
	t.Helper()
	moves, err := engine.LegalMoves(pos)
	if err != nil || len(moves) == 0 {
		t.Fatalf("no moves: %v", err)
	}
	return moves[0]
}

func TestDetect_RecoversPlayedMove(t *testing.T) {
	pre := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	played := bestResult(t, pre) // "8/5 6/5"
	post := obsFromBoard(played.Result)

	scored, err := Detect(pre, post)
	if err != nil {
		t.Fatal(err)
	}
	if scored[0].Move.Notation != "8/5 6/5" {
		t.Errorf("recovered %q, want %q", scored[0].Move.Notation, "8/5 6/5")
	}
	if scored[0].Match < 0.999 {
		t.Errorf("exact observation should match ~1.0, got %.3f", scored[0].Match)
	}
	if scored[1].Match >= scored[0].Match {
		t.Errorf("a different move should match worse than the played one")
	}
}

func TestDecide_EndToEnd_AutoFillsWithDice(t *testing.T) {
	pre := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	post := obsFromBoard(bestResult(t, pre).Result)
	dice := bg.Dice{3, 1}

	d, err := Decide(pre, post, &dice, 4200, fusion.DefaultWeights())
	if err != nil {
		t.Fatal(err)
	}
	if d.Top.Notation != "8/5 6/5" {
		t.Fatalf("decided %q, want %q", d.Top.Notation, "8/5 6/5")
	}
	if d.Confidence < 0.8 {
		t.Errorf("confidence %.3f should clear the auto-fill gate (0.8)", d.Confidence)
	}
	if d.Tick != 4200 {
		t.Errorf("decision lost its tick: %d", d.Tick)
	}
}

// Even with the dice never observed, a unique legal board-diff should still be
// recovered — the engine's legality filter rescues the missing dice.
func TestDecide_DiceAbsent_StillRecovers(t *testing.T) {
	pre := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	post := obsFromBoard(bestResult(t, pre).Result)

	d, err := Decide(pre, post, nil, 0, fusion.DefaultWeights())
	if err != nil {
		t.Fatal(err)
	}
	if d.Top.Notation != "8/5 6/5" || d.Confidence < 0.8 {
		t.Errorf("dice-absent decision = %q conf %.3f, want 8/5 6/5 ≥0.8", d.Top.Notation, d.Confidence)
	}
}

// A single mis-read point should not flip the decision, only dent confidence.
func TestDecide_NoisyObservation_Robust(t *testing.T) {
	pre := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	post := obsFromBoard(bestResult(t, pre).Result)
	post.Points[8] = perceive.PointObs{Count: 1, Side: perceive.A, Confidence: 0.5} // mis-read

	scored, _ := Detect(pre, post)
	if scored[0].Move.Notation != "8/5 6/5" {
		t.Errorf("noisy read flipped the move to %q", scored[0].Move.Notation)
	}
	if scored[0].Match >= 1.0 || scored[0].Match < 0.85 {
		t.Errorf("noisy match should be high-but-imperfect, got %.3f", scored[0].Match)
	}
}

// DecideAnyDice must recover BOTH the dice and the move from a board
// transition alone — the "infer the dice from the diff" rescue path
// (docs/architecture.md §5) used whenever the dice cue is absent.
func TestDecideAnyDice_RecoversDiceAndMove(t *testing.T) {
	pre := bg.Position{Board: bg.StandardStart(), PlayerOnRoll: bg.P1}
	// A strong, distinctive play: 3-1 -> 8/5 6/5.
	truth := bg.Position{Board: bg.StandardStart(), Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	played := bestResult(t, truth)
	post := obsFromBoard(played.Result)

	d, err := DecideAnyDice(pre, obsFromBoard(pre.Board), post, 42, fusion.DefaultWeights(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Top.Dice; !(got == bg.Dice{3, 1} || got == bg.Dice{1, 3}) {
		t.Errorf("dice = %v, want 3-1", got)
	}
	if d.Top.Notation != played.Notation {
		t.Errorf("notation = %q, want %q", d.Top.Notation, played.Notation)
	}
	if d.Player != bg.P1 || d.Tick != 42 {
		t.Errorf("player/tick = %v/%d, want P1/42", d.Player, d.Tick)
	}
	if d.Confidence < 0.5 {
		t.Errorf("confidence %.3f too low for a clean unambiguous transition", d.Confidence)
	}
}

// Reading-to-reading shift: identical readings are 0 even when both carry the
// same systematic misread; a real move shows up as a shift.
func TestReadingShift(t *testing.T) {
	start := bg.StandardStart()
	clean := obsFromBoard(start)
	if s := ReadingShift(clean, clean); s != 0 {
		t.Errorf("identical readings shift = %v, want 0", s)
	}

	// Same systematic bias in both readings (point 6 always read one short).
	biased := clean
	biased.Points[6].Count = 4
	if s := ReadingShift(biased, biased); s != 0 {
		t.Errorf("stable bias shift = %v, want 0 (bias must cancel)", s)
	}

	pre := bg.Position{Board: start, Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	post := obsFromBoard(bestResult(t, pre).Result)
	if s := ReadingShift(clean, post); s <= 0 {
		t.Errorf("real move shift = %v, want > 0", s)
	}
}

// Delta matching must see through a stable per-point misread: the same bias
// in the previous and current readings cancels, and the true move is
// recovered with confidence.
func TestDecideAnyDice_StableBiasCancels(t *testing.T) {
	start := bg.StandardStart()
	pre := bg.Position{Board: start, Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	played := bestResult(t, pre) // 8/5 6/5

	bias := func(ob perceive.ObservedBoard) perceive.ObservedBoard {
		ob.Points[24].Count = 1 // corner stack always under-read
		ob.Points[12].Count = 4 // opponent mid stack always under-read
		return ob
	}
	prev := bias(obsFromBoard(start))
	cur := bias(obsFromBoard(played.Result))

	d, err := DecideAnyDice(bg.Position{Board: start, PlayerOnRoll: bg.P1}, prev, cur, 0, fusion.DefaultWeights(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Top.Notation != played.Notation {
		t.Errorf("recovered %q, want %q despite stable bias", d.Top.Notation, played.Notation)
	}
	if d.Confidence < 0.5 {
		t.Errorf("confidence %.3f, want >= 0.5 — stable bias should not cost confidence", d.Confidence)
	}
}

// WholeBoardMatch is the tolerant landmark check: a biased reading of the
// start is still close to it, a mid-game board is not.
func TestWholeBoardMatch(t *testing.T) {
	start := bg.StandardStart()
	biased := obsFromBoard(start)
	biased.Points[24].Count = 1
	biased.Points[6].Count = 4
	if m := WholeBoardMatch(start, biased); m < 0.85 {
		t.Errorf("biased start reads %.3f vs start, want >= 0.85", m)
	}
	pre := bg.Position{Board: start, Dice: bg.Dice{6, 5}, PlayerOnRoll: bg.P1}
	mid := obsFromBoard(bestResult(t, pre).Result)
	pre2 := bg.Position{Board: bestResult(t, pre).Result, Dice: bg.Dice{6, 5}, PlayerOnRoll: bg.P2}
	mid = obsFromBoard(bestResult(t, pre2).Result)
	if m := WholeBoardMatch(start, mid); m > 0.95 {
		t.Errorf("mid-game board reads %.3f vs start, too close", m)
	}
}

// An observed dice cue must be able to overturn the within-roll prior when
// two rolls reach the SAME board: 1-1 played 8/7 7/6 6/5 6/5 equals 3-1
// played 8/5 6/5; without the cue the prior picks 3-1, with a confident
// 1-1 observation the decision must follow the dice.
func TestDecideAnyDice_ObservedDiceDisambiguates(t *testing.T) {
	start := bg.StandardStart()
	pre := bg.Position{Board: start, PlayerOnRoll: bg.P1}
	truth := bg.Position{Board: start, Dice: bg.Dice{3, 1}, PlayerOnRoll: bg.P1}
	post := obsFromBoard(bestResult(t, truth).Result)
	prev := obsFromBoard(start)
	w := fusion.DefaultWeights()

	// Baseline: no cue -> the within-roll prior chooses 3-1.
	d, err := DecideAnyDice(pre, prev, post, 0, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !(d.Top.Dice == bg.Dice{3, 1} || d.Top.Dice == bg.Dice{1, 3}) {
		t.Fatalf("baseline dice = %v, want 3-1", d.Top.Dice)
	}

	// With a confident 1-1 observation the same transition reads as 1-1.
	obs := &cue.Cue{Kind: cue.DiceValue, Dice: bg.Dice{1, 1}, Confidence: 0.9}
	d, err = DecideAnyDice(pre, prev, post, 0, w, obs)
	if err != nil {
		t.Fatal(err)
	}
	if (d.Top.Dice != bg.Dice{1, 1}) {
		t.Errorf("hinted dice = %v, want 1-1", d.Top.Dice)
	}
	if len(d.Top.Support) == 0 {
		t.Error("decision should record its supporting cues")
	}
}

// A dice observation that agrees with the board-diff explanation raises the
// decision's confidence relative to no observation at all.
func TestDecideAnyDice_AgreeingDiceRaiseConfidence(t *testing.T) {
	start := bg.StandardStart()
	pre := bg.Position{Board: start, PlayerOnRoll: bg.P1}
	truth := bg.Position{Board: start, Dice: bg.Dice{6, 1}, PlayerOnRoll: bg.P1}
	post := obsFromBoard(bestResult(t, truth).Result)
	prev := obsFromBoard(start)
	w := fusion.DefaultWeights()

	plain, err := DecideAnyDice(pre, prev, post, 0, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	obs := &cue.Cue{Kind: cue.DiceValue, Dice: bg.Dice{6, 1}, Confidence: 0.9}
	hinted, err := DecideAnyDice(pre, prev, post, 0, w, obs)
	if err != nil {
		t.Fatal(err)
	}
	if hinted.Confidence <= plain.Confidence {
		t.Errorf("hinted conf %.3f <= plain conf %.3f, want a boost from agreement",
			hinted.Confidence, plain.Confidence)
	}
}
