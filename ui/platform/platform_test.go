package platform_test

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
)

func TestHeadlessInjectClick(t *testing.T) {
	h := platform.NewHeadless(200, 100)
	defer h.Close()
	h.InjectClick(10, 20)
	evs := h.PumpEvents()
	if len(evs) != 2 {
		t.Fatalf("events=%d want 2", len(evs))
	}
	if evs[0].Pointer != platform.PointerDown || evs[1].Pointer != platform.PointerUp {
		t.Fatalf("kinds %v %v", evs[0].Pointer, evs[1].Pointer)
	}
	if evs[0].X != 10 || evs[0].Y != 20 {
		t.Fatalf("pos=%v,%v", evs[0].X, evs[0].Y)
	}
}

func TestHeadlessResize(t *testing.T) {
	h := platform.NewHeadless(100, 100)
	defer h.Close()
	h.Resize(320, 240)
	w, ht := h.Size()
	if w != 320 || ht != 240 {
		t.Fatalf("size=%dx%d", w, ht)
	}
	evs := h.PumpEvents()
	if len(evs) != 1 || evs[0].Type != platform.EventResize {
		t.Fatalf("events=%v", evs)
	}
}

func TestCapsHas(t *testing.T) {
	c := platform.HeadlessCaps
	if !c.Has(platform.CapPointer) {
		t.Fatal("missing CapPointer")
	}
	if c.Has(platform.CapIME) {
		t.Fatal("unexpected CapIME")
	}
}
