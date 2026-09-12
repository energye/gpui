package scene

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// Oversized pictures must be refused, never baked clipped: a clipped
// texture shows blank past the surface edge, while refusal replays vector
// with damage (raster-cache-miss semantics, content stays correct).
func TestRecordLocal_RefusesOversized(t *testing.T) {
	dc := render.NewContext(100, 50)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc
	pic := RecordPicture(func(r *PictureRecorder) {
		r.FillRect(0, 0, 200, 100, 1, 0, 0, 1)
	})
	big := image.Rect(0, 0, 200, 100)
	if _, ok := tex.recordLocalWith(7, &pic, big, nil); ok {
		t.Fatalf("oversized bounds must be refused, not clipped")
	}
	if *allocs != 0 {
		t.Fatalf("refused record allocated %d textures", *allocs)
	}
	if got := tex.OversizedBounds(7); got != big {
		t.Fatalf("oversized bounds=%v want %v", got, big)
	}
	tex.BeginFrame()
	if got := tex.OversizedBounds(7); !got.Empty() {
		t.Fatalf("oversized stash must clear per frame, got %v", got)
	}
}

// Refusing a band that outgrew the surface must evict the entry an earlier
// fitting band left: otherwise phase 2 blits the stale texture at its
// recorded offset (content baked for another scroll position) and a
// scrolled text band shows stale rows or blank instead of replaying the
// current vector picture (B-box middle-blank: tail band kept blitting at a
// middle scroll).
func TestRecordLocal_RefusalEvictsStaleEntry(t *testing.T) {
	dc := render.NewContext(100, 50)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc
	pic := RecordPicture(func(r *PictureRecorder) {
		r.FillRect(0, 0, 40, 20, 1, 0, 0, 1)
	})
	small := image.Rect(0, 0, 40, 20)
	if _, ok := tex.recordLocalWith(7, &pic, small, nil); !ok {
		t.Fatal("fitting band must record")
	}
	if tex.Len() != 1 {
		t.Fatalf("Len=%d want 1 after fit record", tex.Len())
	}
	big := image.Rect(0, 0, 200, 100)
	if _, ok := tex.recordLocalWith(7, &pic, big, nil); ok {
		t.Fatal("oversized bounds must be refused, not clipped")
	}
	if tex.Len() != 0 {
		t.Fatalf("refusal must evict the stale entry, Len=%d", tex.Len())
	}
	if got := tex.OversizedBounds(7); got != big {
		t.Fatalf("oversized stash=%v want %v (phase-2 damage)", got, big)
	}
}
