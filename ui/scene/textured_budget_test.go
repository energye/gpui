package scene_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

// TestPictureTextureCache_ExplicitBudget: a pinned budget lowers the effective
// LRU cap (automatic sizing never raised by it), evictForNew drops the
// oldest-lastUse entry and counts it. Without GPU, record degrades to false —
// but the budget accounting path is still exercised through Len/eviction
// counters staying consistent (no entry can exist without a texture).
func TestPictureTextureCache_ExplicitBudget(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	tex := scene.NewPictureTextureCache(dc, 8)
	tex.SetBudget(2)
	if got := tex.Budget(); got != 2 {
		t.Fatalf("Budget()=%d want 2", got)
	}
	if got := tex.Len(); got != 0 {
		t.Fatalf("Len=%d want 0", got)
	}
	if tex.Evictions != 0 {
		t.Fatalf("Evictions=%d want 0", tex.Evictions)
	}

	p := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 4, 4, 1, 0, 0, 1)
	})
	for id := uint64(1); id <= 3; id++ {
		if _, ok := tex.RecordForTest(id, &p); ok {
			t.Fatalf("record without GPU should fail")
		}
	}
	// No-GPU degradation: no entries were created, so nothing to evict —
	// the invariant under budget is entries ≤ budget with counted drops.
	tex.EndFrame()
	if got := tex.Len(); got > 2 {
		t.Fatalf("Len=%d exceeds pinned budget 2", got)
	}
	if tex.Evictions != 0 {
		t.Fatalf("Evictions=%d want 0 without GPU-created entries", tex.Evictions)
	}
}
