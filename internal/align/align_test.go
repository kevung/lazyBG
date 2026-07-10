package align

import (
	"testing"

	"lazybg/internal/bg"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive"
	"lazybg/internal/transcribe"
)

// A small two-game truth in .mat form — no engine needed, derive replays it.
const truthMat = ` 7 point match

 Game 1
 Alice : 0                        Bob : 0
  1) 31: 8/5 6/5                  42: 8/4 6/4
  2) 61: 13/7 8/7                 53: 8/3 6/3
  3) 43: 24/20 20/17*             61:

 Game 2
 Alice : 0                        Bob : 0
  1)                              21: 13/11 24/23
`

func load(t *testing.T) bg.Match {
	t.Helper()
	m, err := matimport.Parse(truthMat)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func obsOf(b bg.Board) perceive.ObservedBoard {
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

func TestTruthTurns(t *testing.T) {
	turns := TruthTurns(load(t))
	// 6 applied checker plies (the 61 dance has no board to locate).
	if len(turns) != 6 {
		t.Fatalf("turns = %d, want 6", len(turns))
	}
	if turns[0].Notation != "8/5 6/5" || turns[0].Player != bg.P1 {
		t.Errorf("turn 0 = %+v", turns[0])
	}
	if turns[5].Game != 2 {
		t.Errorf("turn 5 game = %d, want 2", turns[5].Game)
	}
	// Index is the global 1-based position in replay order (dance included).
	if turns[5].Index != 7 {
		t.Errorf("turn 5 index = %d, want 7", turns[5].Index)
	}
}

// The aligner must pick, for each truth board, a monotonically later event —
// tolerating re-reads (duplicates), a systematic misread, and spurious
// events that match no truth board.
func TestAlign_NoisyEventStream(t *testing.T) {
	m := load(t)
	turns := TruthTurns(m)

	bias := func(ob perceive.ObservedBoard) perceive.ObservedBoard {
		ob.Points[24].Count = 1 // stable corner misread
		return ob
	}
	junk := obsOf(bg.StandardStart())
	for p := 1; p <= 12; p++ {
		junk.Points[p] = perceive.PointObs{Count: p % 3, Side: perceive.A, Confidence: 0.4}
	}

	var events []transcribe.Event
	add := func(tick int, ob perceive.ObservedBoard) {
		events = append(events, transcribe.Event{Tick: tick, Obs: ob})
	}
	add(1000, bias(obsOf(bg.StandardStart())))
	add(10000, bias(obsOf(turns[0].Board)))
	add(12000, bias(obsOf(turns[0].Board))) // re-read
	add(20000, bias(obsOf(turns[1].Board)))
	add(25000, junk) // hand / dice garbage
	add(30000, bias(obsOf(turns[2].Board)))
	add(40000, bias(obsOf(turns[3].Board)))
	add(45000, bias(obsOf(turns[4].Board)))
	// game 2: reset + first move
	add(50000, bias(obsOf(bg.StandardStart())))
	add(60000, bias(obsOf(turns[5].Board)))

	got := Align(turns, events)
	if len(got) != len(turns) {
		t.Fatalf("assignments = %d, want %d", len(got), len(turns))
	}
	wantEvent := []int{1, 3, 5, 6, 7, 9}
	for k, ev := range got {
		if ev != wantEvent[k] {
			t.Errorf("turn %d assigned event %d, want %d", k, ev, wantEvent[k])
		}
	}
}

// A turn whose board never shows up (fully occluded) is skipped (-1), not
// force-assigned to a wrong event.
func TestAlign_SkipsMissingTurn(t *testing.T) {
	m := load(t)
	turns := TruthTurns(m)

	var events []transcribe.Event
	add := func(tick int, ob perceive.ObservedBoard) {
		events = append(events, transcribe.Event{Tick: tick, Obs: ob})
	}
	add(10000, obsOf(turns[0].Board))
	// turns[1] never observed
	add(30000, obsOf(turns[2].Board))
	add(40000, obsOf(turns[3].Board))
	add(50000, obsOf(turns[4].Board))
	add(60000, obsOf(turns[5].Board))

	got := Align(turns, events)
	if got[1] != -1 {
		t.Errorf("turn 1 assigned event %d, want -1 (never visible)", got[1])
	}
	for k, want := range []int{0, -1, 1, 2, 3, 4} {
		if got[k] != want {
			t.Errorf("turn %d = %d, want %d", k, got[k], want)
		}
	}
}
