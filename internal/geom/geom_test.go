package geom

import (
	"math"
	"testing"
)

const eps = 1e-6

func ptClose(a, b Pt) bool {
	return math.Abs(a.X-b.X) < 1e-4 && math.Abs(a.Y-b.Y) < 1e-4
}

func TestHomography_MapsCorrespondences(t *testing.T) {
	// A perspective-ish source quad mapped to a canonical 100x80 rectangle.
	src := [4]Pt{{10, 12}, {190, 5}, {205, 160}, {0, 150}}
	dst := [4]Pt{{0, 0}, {100, 0}, {100, 80}, {0, 80}}

	h, ok := Homography(src, dst)
	if !ok {
		t.Fatal("Homography failed to solve")
	}
	for i := range src {
		got := h.Apply(src[i])
		if !ptClose(got, dst[i]) {
			t.Errorf("corner %d: Apply(%v) = %v, want %v", i, src[i], got, dst[i])
		}
	}
}

func TestHomography_InverseRoundTrip(t *testing.T) {
	src := [4]Pt{{10, 12}, {190, 5}, {205, 160}, {0, 150}}
	dst := [4]Pt{{0, 0}, {100, 0}, {100, 80}, {0, 80}}
	h, _ := Homography(src, dst)
	inv, ok := h.Inverse()
	if !ok {
		t.Fatal("Inverse failed")
	}
	// A point inside the source quad should survive a there-and-back trip.
	p := Pt{100, 90}
	back := inv.Apply(h.Apply(p))
	if !ptClose(back, p) {
		t.Errorf("round trip: %v -> %v, want %v", p, back, p)
	}
}

func TestMat3_MulInverseIsIdentity(t *testing.T) {
	src := [4]Pt{{10, 12}, {190, 5}, {205, 160}, {0, 150}}
	dst := [4]Pt{{0, 0}, {100, 0}, {100, 80}, {0, 80}}
	h, _ := Homography(src, dst)
	inv, _ := h.Inverse()
	prod := h.Mul(inv)
	// Normalize by prod[8] and compare to identity.
	id := Identity()
	for i := 0; i < 9; i++ {
		if math.Abs(prod[i]/prod[8]-id[i]) > 1e-6 {
			t.Errorf("H*H^-1 not identity at [%d]: got %v", i, prod[i]/prod[8])
		}
	}
}

func TestIdentity_Apply(t *testing.T) {
	if got := Identity().Apply(Pt{3, 7}); !ptClose(got, Pt{3, 7}) {
		t.Errorf("Identity.Apply = %v, want {3,7}", got)
	}
}
