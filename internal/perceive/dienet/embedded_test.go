package dienet

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

// The shipped die-value model (data/models/dievalue.bin, dielabel v4:
// 65.3% per-die / 99.6% junk rejection on held-out recordings) must load
// from raw bytes — the embed.FS path the CLI default uses — and reproduce
// the folded PyTorch logits recorded at export time.
func TestEmbeddedModelParity(t *testing.T) {
	raw, err := os.ReadFile("../../../data/models/dievalue.bin")
	if err != nil {
		t.Fatal(err)
	}
	net, err := LoadBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	vec, err := os.ReadFile("../../../data/models/dietestvec.bin")
	if err != nil {
		t.Fatal(err)
	}
	nIn := 3 * In * In
	if len(vec) != 4*(nIn+NClasses) {
		t.Fatalf("dietestvec.bin: %d bytes, want %d", len(vec), 4*(nIn+NClasses))
	}
	x := make([]float32, nIn)
	for i := range x {
		x[i] = math.Float32frombits(binary.LittleEndian.Uint32(vec[4*i:]))
	}
	want := make([]float32, NClasses)
	for i := range want {
		want[i] = math.Float32frombits(binary.LittleEndian.Uint32(vec[4*(nIn+i):]))
	}
	got := net.Forward(x)
	for i := range want {
		if d := math.Abs(float64(got[i] - want[i])); d > 1e-3 {
			t.Errorf("logit %d: got %.5f want %.5f (Δ %.5f)", i, got[i], want[i], d)
		}
	}
}
