//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render/text"
)

func TestS42_SyncAtlasTextures_PartialThenHit(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()

	eng := NewGlyphMaskEngine()
	// Rasterize a few glyphs via Layout if we have a font — or Put directly on atlas.
	atlas := eng.Atlas()
	mask := make([]byte, 16*16)
	for i := range mask {
		mask[i] = byte(i)
	}
	key := text.MakeGlyphMaskKey(9, 1, 14, 0, 0)
	if _, err := atlas.Put(key, mask, 16, 16, 0, 0, 0); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := eng.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("Sync1: %v", err)
	}
	bytes1, regions1, partial1, full1 := eng.LastUploadStats()
	t.Logf("sync1 bytes=%d regions=%d partial=%d full=%d", bytes1, regions1, partial1, full1)
	if regions1 != 1 {
		t.Fatalf("want 1 region, got %d", regions1)
	}
	// First texture creation forces full-page upload path.
	if full1 < 1 && partial1 < 1 {
		t.Fatal("expected at least one upload")
	}
	hits, misses, _, _ := eng.AtlasStats()
	_ = hits
	_ = misses

	// Second sync with no new glyphs: zero upload bytes, hit path.
	if err := eng.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("Sync2: %v", err)
	}
	bytes2, regions2, _, _ := eng.LastUploadStats()
	if bytes2 != 0 || regions2 != 0 {
		t.Fatalf("hit-only frame should upload nothing, bytes=%d regions=%d", bytes2, regions2)
	}

	// Add another small glyph → dirty again; texture exists → prefer partial.
	key2 := text.MakeGlyphMaskKey(9, 2, 14, 0, 0)
	if _, err := atlas.Put(key2, mask, 16, 16, 0, 0, 0); err != nil {
		t.Fatalf("Put2: %v", err)
	}
	if err := eng.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("Sync3: %v", err)
	}
	bytes3, regions3, partial3, full3 := eng.LastUploadStats()
	t.Logf("sync3 bytes=%d regions=%d partial=%d full=%d", bytes3, regions3, partial3, full3)
	if regions3 != 1 {
		t.Fatalf("want 1 region after second glyph, got %d", regions3)
	}
	if partial3 != 1 {
		t.Fatalf("expected partial upload for second glyph (texture already exists), full=%d partial=%d", full3, partial3)
	}
	// Partial should be far smaller than full 1024*1024.
	if bytes3 >= 1024*1024 {
		t.Fatalf("partial upload too large: %d", bytes3)
	}
	if d := eng.LastDeferredUploadBytes(); d != 0 {
		t.Fatalf("small upload must not defer, deferred=%d", d)
	}
}

func TestUploadBudget_DefersLargeFirst(t *testing.T) {
	full := text.GlyphMaskDirtyUpload{Index: 0, FullPage: true}
	small := text.GlyphMaskDirtyUpload{Index: 1, X: 0, Y: 0, W: 64, H: 64}
	pageBytes := func(idx int) int {
		if idx == 0 {
			return 1024 * 1024
		}
		return 0
	}
	// Budget below the full page but above the small region: full page
	// defers, small region still uploads the same frame.
	out := applyGlyphUploadBudget(pageBytes, []text.GlyphMaskDirtyUpload{full, small}, 256*1024)
	if len(out) != 2 {
		t.Fatalf("want 2 entries back, got %d", len(out))
	}
	var sawDeferredFull, sawLiveSmall bool
	for _, up := range out {
		if up.FullPage && up.Deferred {
			sawDeferredFull = true
		}
		if !up.FullPage && !up.Deferred && up.W == 64 && up.H == 64 {
			sawLiveSmall = true
		}
	}
	if !sawDeferredFull || !sawLiveSmall {
		t.Fatalf("budget must defer the full page and keep the small region: %+v", out)
	}
	// Within budget: nothing defers.
	clean := applyGlyphUploadBudget(pageBytes, []text.GlyphMaskDirtyUpload{small}, 256*1024)
	if clean[0].Deferred {
		t.Fatalf("within-budget upload must not defer: %+v", clean)
	}
}
