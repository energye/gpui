package scene

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestTextureBudget_EvictsUnderPressure verifies the R14 explicit budget
// (SetBudget) actually drives evictForNew with a fake slot factory: entries
// beyond the pinned cap are dropped oldest-lastUse-first, counted in
// Evictions, and their slots deferred-released — while the automatic sizing
// path (EnsureCapacity) is never raised by the pin.
func TestTextureBudget_EvictsUnderPressure(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0) // automatic size starts at 0 (unlimited)
	alloc, allocs, released := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	tex.SetBudget(2)
	if got := tex.Budget(); got != 2 {
		t.Fatalf("Budget()=%d want 2", got)
	}

	// Insert 3 entries across frames; the budget caps the map at 2.
	for id := uint64(1); id <= 3; id++ {
		tex.BeginFrame()
		if e := tex.allocEntry(id, 10, 10); e == nil {
			t.Fatalf("allocEntry(%d) failed", id)
		}
	}
	if got := tex.Len(); got != 2 {
		t.Fatalf("Len=%d want 2 (budget enforced)", got)
	}
	if tex.Evictions < 1 {
		t.Fatalf("Evictions=%d want >=1", tex.Evictions)
	}
	// Oldest key (1) must be the victim; newest two survive.
	if tex.entries[1] != nil {
		t.Fatal("id=1 (oldest lastUse) should have been evicted")
	}
	if tex.entries[2] == nil || tex.entries[3] == nil {
		t.Fatal("recent entries 2/3 must survive")
	}
	if *allocs != 3 {
		t.Fatalf("allocs=%d want 3 (one per entry)", *allocs)
	}

	// Deferred releases fire after the drain window.
	tex.BeginFrame()
	tex.BeginFrame()
	if len(*released) == 0 {
		t.Fatalf("deferred release never fired for evicted entry")
	}

	// Re-inserting the evicted key allocates again (honest re-record cost).
	tex.BeginFrame()
	if e := tex.allocEntry(1, 10, 10); e == nil {
		t.Fatal("re-entry allocEntry failed")
	}
	if *allocs != 4 {
		t.Fatalf("allocs=%d want 4 (evicted key re-allocates)", *allocs)
	}
}

// TestTextureBudget_ZeroUnsets pins the documented reset semantics:
// SetBudget(0) returns to automatic working-set sizing.
func TestTextureBudget_ZeroUnsets(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 8)
	tex.SetBudget(4)
	tex.SetBudget(0)
	if got := tex.Budget(); got != 0 {
		t.Fatalf("Budget()=%d want 0 after unset", got)
	}
	// With no pin, EnsureCapacity still grows the automatic cap.
	tex.EnsureCapacity(12)
	if got := tex.Len(); got != 0 {
		t.Fatalf("Len=%d want 0", got)
	}
}
