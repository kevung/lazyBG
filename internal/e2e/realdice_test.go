package e2e

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"lazybg/internal/capture"
	"lazybg/internal/derive"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive/dice"
	"lazybg/internal/transcribe"
)

// TestRealCorpus_DiceValueAccuracy measures the classical pip reader on real
// aligned frames: every aligned turn's tick shows the dice that produced it
// (value known from the .mat), so the reader's value accuracy is measurable
// without any localization labels. The dice zone is the rectified board's
// central band (between the two point rows) plus the inner edges of the
// quads — where competition dice land.
func TestRealCorpus_DiceValueAccuracy(t *testing.T) {
	if testing.Short() {
		t.Skip("long: decodes real video frames")
	}
	m, video := loadPilot(t)
	if len(m.Turns) < 20 {
		t.Skipf("pilot manifest has only %d aligned turns", len(m.Turns))
	}
	matBytes, err := os.ReadFile(filepath.Join(repoRoot, m.Transcript))
	if err != nil {
		t.Skip(err)
	}
	truth, err := matimport.Parse(string(matBytes))
	if err != nil {
		t.Fatal(err)
	}
	states := derive.Replay(truth)
	cal, cb, _, err := transcribe.PartSetup(m.Parts[0])
	if err != nil {
		t.Fatal(err)
	}
	w, h := cb.Size()
	// central felt band, widened into the quads by one checker row
	band := image.Rect(cb.MarginX, cb.MarginY+cb.QuadH-cb.PointW, w-cb.MarginX, h-cb.MarginY-cb.QuadH+cb.PointW)

	sampled, exact, oneDie, withPair := 0, 0, 0, 0
	step := len(m.Turns) / 40
	if step < 1 {
		step = 1
	}
	for i := 0; i < len(m.Turns); i += step {
		tn := m.Turns[i]
		if tn.Index-1 >= len(states) {
			continue
		}
		ts := states[tn.Index-1]
		if ts.Dice[0] == 0 {
			continue
		}
		frame, err := capture.FrameAt(video, tn.TickMs)
		if err != nil {
			continue
		}
		rect := cal.Rectify(frame)
		vals := dice.ReadDice(rect, band, 5, 42)
		sampled++
		want := []int{ts.Dice[0], ts.Dice[1]}
		if want[0] > want[1] {
			want[0], want[1] = want[1], want[0]
		}
		if len(vals) >= 2 {
			withPair++
			// vals sorted ascending; compare the best-matching pair
			got := []int{vals[len(vals)-2], vals[len(vals)-1]}
			if got[0] > got[1] {
				got[0], got[1] = got[1], got[0]
			}
			if got[0] == want[0] && got[1] == want[1] {
				exact++
			} else if got[0] == want[0] || got[1] == want[1] || got[0] == want[1] || got[1] == want[0] {
				oneDie++
			}
		} else if len(vals) == 1 {
			if vals[0] == want[0] || vals[0] == want[1] {
				oneDie++
			}
		}
	}
	t.Logf("dice value accuracy on %d aligned frames: exact pair %d (%.0f%%), one die %d, detected-pair rate %d",
		sampled, exact, 100*float64(exact)/float64(max(sampled, 1)), oneDie, withPair)
	if sampled < 10 {
		t.Skipf("too few samples (%d)", sampled)
	}
	// Measurement test: log-first; the floor only guards total breakage.
	if withPair == 0 {
		t.Error("the pip reader never detected a dice pair on real frames")
	}
}
