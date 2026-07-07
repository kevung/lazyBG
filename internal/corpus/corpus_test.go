package corpus

import "testing"

// A two-part manifest where part 2 inherits priors + calibration from part 1.
const twoPart = `{
  "schemaVersion": 1,
  "id": "hsbtMars2025-r1",
  "transcript": "corpus/2025-05_hsbtMarseille/main-r1/main-r1.mat",
  "cell": {"angle":"overhead","colors":"green-yellow","resolution":"720p","dice":"opaque","audio":"table"},
  "parts": [
    {"file":"a.mkv",
     "priors":{"clock":true,"matchLength":7,"checkerA":"#e1ded2","checkerB":"#464850","orientation":"p1-bottom"},
     "calibration":{"corners":[[203,54],[825,46],[818,614],[200,628]]},
     "span":{"beginMs":6000,"endMs":1380000}},
    {"file":"b.mkv",
     "priors":{"inherit":true},
     "calibration":{"inherit":true},
     "span":{"beginMs":3000,"endMs":720000}}
  ],
  "turns": [
    {"index":1,"part":0,"tickMs":51200},
    {"index":2,"part":1,"tickMs":58900}
  ]
}`

func TestLoad_ValidResolvesInherit(t *testing.T) {
	m, err := Load([]byte(twoPart))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(m.Parts))
	}
	// Part 2 must have inherited part 1's priors + calibration.
	if m.Parts[1].Priors.MatchLength != 7 || m.Parts[1].Priors.CheckerA != "#e1ded2" {
		t.Errorf("priors not inherited: %+v", m.Parts[1].Priors)
	}
	if len(m.Parts[1].Calibration.Corners) != 4 || m.Parts[1].Calibration.Corners[2] != [2]float64{818, 614} {
		t.Errorf("calibration not inherited: %+v", m.Parts[1].Calibration.Corners)
	}
	// Part 2 keeps its own span.
	if m.Parts[1].Span.BeginMs != 3000 {
		t.Errorf("span overwritten: %+v", m.Parts[1].Span)
	}
}

func mustFail(t *testing.T, doc, why string) {
	t.Helper()
	if _, err := Load([]byte(doc)); err == nil {
		t.Errorf("expected error (%s), got nil", why)
	}
}

func TestLoad_Rejects(t *testing.T) {
	mustFail(t, `{"schemaVersion":99,"id":"x","transcript":"t","parts":[{"file":"a","calibration":{"corners":[[0,0],[1,0],[1,1],[0,1]]},"span":{"beginMs":0,"endMs":1}}]}`, "wrong schema version")
	// inherit on the first part (nothing to inherit from)
	mustFail(t, `{"schemaVersion":1,"id":"x","transcript":"t","parts":[{"file":"a","priors":{"inherit":true},"calibration":{"corners":[[0,0],[1,0],[1,1],[0,1]]},"span":{"beginMs":0,"endMs":1}}]}`, "inherit on first part")
	// only 3 corners
	mustFail(t, `{"schemaVersion":1,"id":"x","transcript":"t","parts":[{"file":"a","calibration":{"corners":[[0,0],[1,0],[1,1]]},"span":{"beginMs":0,"endMs":1}}]}`, "3 corners")
	// span begin >= end
	mustFail(t, `{"schemaVersion":1,"id":"x","transcript":"t","parts":[{"file":"a","calibration":{"corners":[[0,0],[1,0],[1,1],[0,1]]},"span":{"beginMs":10,"endMs":10}}]}`, "empty span")
	// turn references a non-existent part
	mustFail(t, `{"schemaVersion":1,"id":"x","transcript":"t","parts":[{"file":"a","calibration":{"corners":[[0,0],[1,0],[1,1],[0,1]]},"span":{"beginMs":0,"endMs":100}}],"turns":[{"index":1,"part":5,"tickMs":10}]}`, "bad part index")
	// turn tick outside its part's span
	mustFail(t, `{"schemaVersion":1,"id":"x","transcript":"t","parts":[{"file":"a","calibration":{"corners":[[0,0],[1,0],[1,1],[0,1]]},"span":{"beginMs":0,"endMs":100}}],"turns":[{"index":1,"part":0,"tickMs":500}]}`, "tick outside span")
	// non-increasing turn indices
	mustFail(t, `{"schemaVersion":1,"id":"x","transcript":"t","parts":[{"file":"a","calibration":{"corners":[[0,0],[1,0],[1,1],[0,1]]},"span":{"beginMs":0,"endMs":100}}],"turns":[{"index":2,"part":0,"tickMs":10},{"index":2,"part":0,"tickMs":20}]}`, "non-increasing turn index")
}
