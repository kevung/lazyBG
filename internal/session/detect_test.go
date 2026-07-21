package session

import "testing"

// With no video attached, DetectCorners fails gracefully (no panic, no corners)
// so the GUI can fall back to manual dragging.
func TestDetectCorners_NoVideo(t *testing.T) {
	s := New()
	got, err := s.DetectCorners(0)
	if err == nil {
		t.Fatal("expected an error with no video")
	}
	if got.Corners != nil || got.BarEdges != nil {
		t.Errorf("expected no handles on failure, got %+v", got)
	}
}
