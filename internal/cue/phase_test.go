package cue

import "testing"

// The measured attribution data (dice hand-label round 1): appearances more
// than ~3s before a turn's tick belong to that turn's roll; appearances from
// ~1s before the tick onward belong to the NEXT turn's roll. Interval
// semantics: turn i's roll phase spans from just after turn i-1's tick to
// settleMs before turn i's own tick.
func TestAttributeRoll(t *testing.T) {
	ticks := []int{60000, 90000, 120000}
	cases := []struct {
		appear int
		want   int
		ok     bool
	}{
		{40000, 0, true},   // long before turn 0's tick: turn 0's roll
		{56000, 0, true},   // 4s before tick 0: still turn 0
		{58500, 0, false},  // inside settle band before tick 0: ambiguous
		{61000, 1, false},  // just after tick 0: ambiguous (hands/removal)
		{63000, 1, true},   // clear of tick 0: turn 1's roll
		{86000, 1, true},   // 4s before tick 1: turn 1
		{89500, 1, false},  // settle band of tick 1
		{100000, 2, true},  // between ticks 1 and 2: turn 2
		{119000, 2, false}, // settle band of tick 2
		{125000, -1, false}, // after the last tick: no turn
	}
	for _, c := range cases {
		got, ok := AttributeRoll(ticks, c.appear, 3000, 2000)
		if got != c.want || ok != c.ok {
			t.Errorf("AttributeRoll(%d) = (%d,%v), want (%d,%v)", c.appear, got, ok, c.want, c.ok)
		}
	}
}

func TestAttributeRollEmpty(t *testing.T) {
	if got, ok := AttributeRoll(nil, 1000, 3000, 2000); got != -1 || ok {
		t.Fatalf("empty ticks: got (%d,%v)", got, ok)
	}
}
