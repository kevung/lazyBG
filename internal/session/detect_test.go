package session

import (
	"testing"

	"lazybg/internal/geom"
)

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

// A fit that stays inside the workspace (frame + 15%) is passed through
// untouched — including handles genuinely OUTSIDE the frame, which four
// committed corpus manifests carry (a corner a few px above the top edge) and
// which read correctly. Clamping onto the frame would silently break those.
func TestClampHandles_LeavesTheWorkspaceAlone(t *testing.T) {
	corners := [4]geom.Pt{{X: 100, Y: -23}, {X: 900, Y: -5}, {X: 900, Y: 600}, {X: 100, Y: 600}}
	bars := [4]geom.Pt{{X: 480, Y: -14}, {X: 520, Y: -14}, {X: 520, Y: 600}, {X: 480, Y: 600}}
	gotC, gotB, clamped := clampHandles(corners, bars, 1000, 800)
	if clamped {
		t.Error("handles within the 15% margin must not be reported as clamped")
	}
	if gotC != corners || gotB != bars {
		t.Errorf("handles moved: corners %v bars %v", gotC, gotB)
	}
}

// Beyond the workspace the handle is pulled back per axis, and the caller is
// told — the GUI turns that into an explicit message rather than a grid that
// is mysteriously wrong in one corner.
func TestClampHandles_PullsBackAndReports(t *testing.T) {
	corners := [4]geom.Pt{{X: -400, Y: -300}, {X: 2000, Y: 100}, {X: 900, Y: 4000}, {X: 100, Y: 600}}
	bars := [4]geom.Pt{{X: 480, Y: -900}, {X: 520, Y: 0}, {X: 520, Y: 600}, {X: 480, Y: 600}}
	gotC, gotB, clamped := clampHandles(corners, bars, 1000, 800)
	if !clamped {
		t.Fatal("expected clamped=true")
	}
	want := [4]geom.Pt{{X: -150, Y: -120}, {X: 1150, Y: 100}, {X: 900, Y: 920}, {X: 100, Y: 600}}
	if gotC != want {
		t.Errorf("corners = %v, want %v", gotC, want)
	}
	// Bar handles are clamped too: autocal produces them independently, and a
	// headless caller has none of the GUI's fracAlong pinning.
	if gotB[0].Y != -120 {
		t.Errorf("bar handles must be clamped as well, got %v", gotB)
	}
}

// An unknown frame size must not move anything — better an unclamped handle
// than one silently teleported by a bad probe.
func TestClampHandles_UnknownFrameSize(t *testing.T) {
	corners := [4]geom.Pt{{X: -400, Y: -300}, {X: 2000, Y: 100}, {X: 900, Y: 4000}, {X: 100, Y: 600}}
	gotC, _, clamped := clampHandles(corners, [4]geom.Pt{}, 0, 0)
	if clamped || gotC != corners {
		t.Errorf("no frame size ⇒ no clamp; got %v clamped=%v", gotC, clamped)
	}
}
