package scene

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// Oversized pictures must be refused, never baked clipped: a texture that
// cannot hold the whole footprint shows blank past the surface edge, while
// refusal replays vector with damage (raster-cache-miss semantics, content
// stays correct).
func TestRecordLocal_RefusesOversized(t *testing.T) {
	dc := render.NewContext(100, 50)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc
	big := image.Rect(0, 0, 300, 150) // > oversizeSurfaceMultiple× the surface
	if _, ok := tex.recordLocal(7, fillRecording(big), big); ok {
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

// A full-width band legitimately measures a couple of pixels past the surface:
// the recording footprint carries the conservative AA/stroke outset (a 1px
// hairline at the band edge records a 2px miter pad on each side). Such a band
// must still cache — C11's HUD measured 1204×76 on a 1200×800 window and the
// old extent refusal replayed it vector every frame. The texture is allocated
// to the exact footprint (+AA pad) so no painted pixel is cut.
func TestRecordLocal_CachesPadBleedAtExactSize(t *testing.T) {
	dc := render.NewContext(1200, 800)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	b := image.Rect(-2, -2, 1202, 74)
	if _, ok := tex.recordLocal(46, fillRecording(b), b); !ok {
		t.Fatal("pad-only surface overflow must cache, not replay vector")
	}
	if *allocs == 0 {
		t.Fatal("admitted band must allocate a texture")
	}
	tex.mu.Lock()
	e := tex.entries[46]
	tex.mu.Unlock()
	if e == nil || e.contentSlot < 0 {
		t.Fatal("entry must exist after record")
	}
	s := &e.slots[e.contentSlot]
	if s.w != 1206 || s.h != 78 {
		t.Fatalf("texture=%dx%d want 1206x78 (exact footprint + AA pad; clamping cuts the far edge)", s.w, s.h)
	}
	if e.off != image.Pt(-2, -2) {
		t.Fatalf("record offset=%v want (-2,-2) (content keeps its position)", e.off)
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
	small := image.Rect(0, 0, 40, 20)
	if _, ok := tex.recordLocal(7, fillRecording(small), small); !ok {
		t.Fatal("fitting band must record")
	}
	if tex.Len() != 1 {
		t.Fatalf("Len=%d want 1 after fit record", tex.Len())
	}
	big := image.Rect(0, 0, 300, 150)
	if _, ok := tex.recordLocal(7, fillRecording(small), big); ok {
		t.Fatal("oversized bounds must be refused, not clipped")
	}
	if tex.Len() != 0 {
		t.Fatalf("refusal must evict the stale entry, Len=%d", tex.Len())
	}
	if got := tex.OversizedBounds(7); got != big {
		t.Fatalf("oversized stash=%v want %v (phase-2 damage)", got, big)
	}
}
