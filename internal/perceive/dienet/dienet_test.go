package dienet

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

// Torch parity: the fixture pair (random-weight model + input/expected-logits
// vector, both written by ml/export_die.py --random-tiny) must reproduce the
// folded PyTorch logits bit-for-bit within float tolerance.
func TestForwardMatchesTorch(t *testing.T) {
	net, err := Load("../../../testdata/dienet/dievalue.bin")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../../testdata/dienet/dietestvec.bin")
	if err != nil {
		t.Fatal(err)
	}
	nIn := 3 * In * In
	if len(raw) != 4*(nIn+NClasses) {
		t.Fatalf("dietestvec.bin: %d bytes, want %d", len(raw), 4*(nIn+NClasses))
	}
	x := make([]float32, nIn)
	for i := range x {
		x[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
	}
	want := make([]float32, NClasses)
	for i := range want {
		want[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*(nIn+i):]))
	}
	got := net.Forward(x)
	for i := range want {
		if d := math.Abs(float64(got[i] - want[i])); d > 1e-3 {
			t.Errorf("logit %d: got %.5f want %.5f (Δ %.5f)", i, got[i], want[i], d)
		}
	}
}
