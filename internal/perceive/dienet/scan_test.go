package dienet

import (
	"image"
	"testing"
)

// NMS must keep the highest-confidence detection of an overlapping cluster
// and drop the rest; disjoint detections all survive.
func TestNMS(t *testing.T) {
	dets := []Det{
		{Box: image.Rect(0, 0, 30, 30), Val: 5, Conf: 0.9},
		{Box: image.Rect(5, 5, 35, 35), Val: 4, Conf: 0.7},   // overlaps the first
		{Box: image.Rect(100, 0, 130, 30), Val: 2, Conf: 0.8}, // disjoint
	}
	out := nms(dets, 0.3)
	if len(out) != 2 {
		t.Fatalf("nms kept %d, want 2: %+v", len(out), out)
	}
	if out[0].Val != 5 || out[1].Val != 2 {
		t.Fatalf("nms kept wrong dets: %+v", out)
	}
}

// ScanBand on a random-weight fixture net must stay inside the band and
// return at most maxDets detections sorted by confidence.
func TestScanBandBounds(t *testing.T) {
	net, err := Load("../../../testdata/dienet/dievalue.bin")
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	band := image.Rect(50, 80, 350, 130)
	dets := ScanBand(net, img, band, []int{28, 36}, 12, 0.0, 4)
	if len(dets) > 4 {
		t.Fatalf("more than maxDets: %d", len(dets))
	}
	for i, d := range dets {
		if !d.Box.In(band.Inset(-40)) {
			t.Errorf("det %d outside band area: %v", i, d.Box)
		}
		if i > 0 && dets[i-1].Conf < d.Conf {
			t.Errorf("dets not sorted by confidence")
		}
	}
}
