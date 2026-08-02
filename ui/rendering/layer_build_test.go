package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// TestBuildLayerTree_LeafPictures: every leaf records its own content into a
// PictureLayer and binds a stable cache key (EnsureCacheID).
func TestBuildLayerTree_LeafPictures(t *testing.T) {
	box := rendering.NewRenderColorBox(60, 40, 1, 0, 0, 1)
	box.MarkNeedsPaint()
	b := rendering.BuildLayerTree(box)
	pkt := b.BuildPacket(1, 1, 100, 80)

	pictureCount := 0
	cacheKeyCount := 0
	scene.Walk(pkt.Root, func(l scene.Layer) {
		if pl, ok := l.(*scene.PictureLayer); ok {
			pictureCount++
			if pl.CacheKey != 0 {
				cacheKeyCount++
			}
			if pl.Picture.OpCount() == 0 {
				t.Fatalf("leaf picture has no ops")
			}
			b := pl.Picture.Bounds
			if b.Dx() != 60 || b.Dy() != 40 {
				t.Fatalf("Bounds=%v want 60x40", b)
			}
		}
	})
	if pictureCount < 1 {
		t.Fatalf("no picture layers built")
	}
	if cacheKeyCount != pictureCount {
		t.Fatalf("cache keys=%d want %d (all leaves keyed)", cacheKeyCount, pictureCount)
	}
}

// TestBuildLayerTree_CacheKeyStable: rebuilds produce the same cache keys for
// the same tree (cross-frame texture cache identity).
func TestBuildLayerTree_CacheKeyStable(t *testing.T) {
	box := rendering.NewRenderColorBox(60, 40, 1, 0, 0, 1)
	first := rendering.BuildLayerTree(box).BuildPacket(1, 1, 100, 80)
	second := rendering.BuildLayerTree(box).BuildPacket(2, 1, 100, 80)

	var k1, k2 uint64
	scene.Walk(first.Root, func(l scene.Layer) {
		if pl, ok := l.(*scene.PictureLayer); ok {
			k1 = pl.CacheKey
		}
	})
	scene.Walk(second.Root, func(l scene.Layer) {
		if pl, ok := l.(*scene.PictureLayer); ok {
			k2 = pl.CacheKey
		}
	})
	if k1 == 0 || k1 != k2 {
		t.Fatalf("cache keys unstable: %d vs %d", k1, k2)
	}
}
