package boarddiff

import (
	"os"
	"testing"

	"lazybg/internal/bg"
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
