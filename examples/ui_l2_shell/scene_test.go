package main

import (
	"testing"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/platform"
)

func TestScene_Integration_TapPanFocusOverlay(t *testing.T) {
	s := NewScene(480, 360)
	s.Layout()

	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: 40, Y: 40})
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, X: 42, Y: 41})
	if s.SnapshotMetrics().Taps != 1 {
		t.Fatalf("taps=%d", s.SnapshotMetrics().Taps)
	}

	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: 230, Y: 50})
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerMove, X: 230, Y: 80})
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, X: 230, Y: 90})
	m := s.SnapshotMetrics()
	if m.PanStarts < 1 {
		t.Fatalf("panStarts=%d updates=%d", m.PanStarts, m.PanUpdates)
	}

	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: 50, Y: 160})
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, X: 50, Y: 160})
	if s.SnapshotMetrics().FocusLabel != "focus-a" {
		t.Fatalf("focus=%q", s.SnapshotMetrics().FocusLabel)
	}
	s.HandleEvent(platform.Event{Type: platform.EventKey, KeyCode: focus.KeyTab, Pressed: true})
	if s.SnapshotMetrics().FocusLabel != "focus-b" {
		t.Fatalf("after tab focus=%q", s.SnapshotMetrics().FocusLabel)
	}

	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: 50, Y: 255})
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, X: 50, Y: 255})
	if s.SnapshotMetrics().OverlayCount < 1 {
		t.Fatalf("overlay count=%d", s.SnapshotMetrics().OverlayCount)
	}
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: 10, Y: 10})
	if s.SnapshotMetrics().OverlayCount != 0 {
		t.Fatalf("overlay after close=%d", s.SnapshotMetrics().OverlayCount)
	}

	if s.SnapshotMetrics().ArenaDepth != 0 {
		t.Fatalf("arena depth=%d", s.SnapshotMetrics().ArenaDepth)
	}
}

func TestScene_KeyO_TogglesOverlay(t *testing.T) {
	s := NewScene(480, 360)
	s.Layout()
	s.HandleEvent(platform.Event{Type: platform.EventKey, Rune: 'o', Pressed: true})
	if s.SnapshotMetrics().OverlayCount < 1 {
		t.Fatal("expected overlay")
	}
	s.HandleEvent(platform.Event{Type: platform.EventKey, Rune: 'o', Pressed: true})
	if s.SnapshotMetrics().OverlayCount != 0 {
		t.Fatal("expected closed")
	}
}

// Example layout clamp: pan box 200×110 at (200,24); handle 44×44 stays inside.
func TestScene_PanHandle_ClampedInsideSlate(t *testing.T) {
	const (
		panX, panY   = 200.0, 24.0
		panW, panH   = 200.0, 110.0
		knobW, knobH = 44.0, 44.0
	)
	s := NewScene(480, 360)
	s.Layout()

	x0, y0 := s.PanKnobOffset()
	wantX0 := panX + (panW-knobW)/2
	wantY0 := panY + (panH-knobH)/2
	if x0 != wantX0 || y0 != wantY0 {
		t.Fatalf("rest handle=(%.0f,%.0f) want (%.0f,%.0f)", x0, y0, wantX0, wantY0)
	}

	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: x0 + 20, Y: y0 + 20})
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerMove, X: x0 + 20 - 500, Y: y0 + 20 - 500})
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, X: x0 - 500, Y: y0 - 500})
	xl, yl := s.PanKnobOffset()
	if xl != panX || yl != panY {
		t.Fatalf("left-top clamp handle=(%.1f,%.1f) want (%.1f,%.1f)", xl, yl, panX, panY)
	}

	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerDown, X: xl + 10, Y: yl + 10})
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerMove, X: xl + 10 + 800, Y: yl + 10 + 800})
	s.HandleEvent(platform.Event{Type: platform.EventPointer, Pointer: platform.PointerUp, X: xl + 800, Y: yl + 800})
	xr, yr := s.PanKnobOffset()
	wantXR := panX + panW - knobW
	wantYR := panY + panH - knobH
	if xr != wantXR || yr != wantYR {
		t.Fatalf("right-bottom clamp handle=(%.1f,%.1f) want (%.1f,%.1f)", xr, yr, wantXR, wantYR)
	}
}
