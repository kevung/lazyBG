package session

import "testing"

// With no video attached, DetectCorners fails gracefully (no panic, no corners)
// so the GUI can fall back to manual dragging.
func TestDetectCorners_NoVideo(t *testing.T) {
	s := New()
	got, err := s.DetectCorners()
	if err == nil {
		t.Fatal("expected an error with no video")
	}
	if got.Corners != nil {
		t.Errorf("expected no corners on failure, got %v", got.Corners)
	}
}
