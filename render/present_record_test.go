package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestPresentResize_RecordOnlyHandshake locks the D8 record-only contract
// headlessly (no GPU/X11 needed — the existing X11 tests cover the apply
// side): Resize stores the size and arms the pending flag for the raster
// thread's present boundary; it never touches the surface synchronously.
// Coalescing: rapid resizes collapse to the latest size with the pending
// flag intact (a same-size no-op must not clear a pending resize).
func TestPresentResize_RecordOnlyHandshake(t *testing.T) {
	pt := &render.PresentTarget{}
	if err := pt.Resize(200, 150, 1.0); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if w, h := pt.LogicalSize(); w != 200 || h != 150 {
		t.Fatalf("logical=%dx%d want 200x150", w, h)
	}
	if !pt.InFullRecovery() {
		t.Fatal("recorded resize must arm recovery (pending flag)")
	}

	// Storm of resizes before any present: latest wins, pending survives.
	if err := pt.Resize(320, 240, 2.0); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if err := pt.Resize(320, 240, 2.0); err != nil {
		t.Fatalf("same-size Resize: %v", err)
	}
	if w, h := pt.LogicalSize(); w != 320 || h != 240 {
		t.Fatalf("logical=%dx%d want latest 320x240", w, h)
	}
	if s := pt.Scale(); s != 2.0 {
		t.Fatalf("scale=%v want 2.0", s)
	}
	if !pt.InFullRecovery() {
		t.Fatal("same-size no-op must not clear the pending resize")
	}

	// Degenerate inputs clamp, never error (except closed).
	if err := pt.Resize(0, -5, 0); err != nil {
		t.Fatalf("Resize clamp: %v", err)
	}
	if w, h := pt.LogicalSize(); w != 1 || h != 1 {
		t.Fatalf("clamped logical=%dx%d want 1x1", w, h)
	}
	if err := pt.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := pt.Resize(100, 100, 1.0); err == nil {
		t.Fatal("resize on closed target must fail")
	}
}
