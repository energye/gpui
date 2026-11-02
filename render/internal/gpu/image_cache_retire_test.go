//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// ImageCache hands textures leaving the cache to a retire hook instead of
// destroying them inline: the currently encoded frame may still sample the
// view, so the session defers release to a submission-completion point.
// Without a hook (tests / standalone caches) the pair is destroyed
// immediately.

func TestImageCache_RetireHookDefersRelease(t *testing.T) {
	c := &ImageCache{entries: make(map[imageCacheKey]*imageCacheEntry)}
	var gotTex hal.Texture
	var gotView hal.TextureView
	calls := 0
	c.SetRetireHook(func(tex hal.Texture, view hal.TextureView) {
		calls++
		gotTex, gotView = tex, view
	})
	tex := &testTexture{}
	view := &testTextureView{}
	entry := &imageCacheEntry{texture: tex, view: view, bytes: 64, gen: 1}
	c.entries[imageCacheKey{gen: 7, levels: 1}] = entry
	c.usedBytes = 64

	c.removeEntry(imageCacheKey{gen: 7, levels: 1}, entry) // evict / size-replace route
	if calls != 1 || gotTex != hal.Texture(tex) || gotView != hal.TextureView(view) {
		t.Fatalf("hook calls=%d want 1 with the entry pair", calls)
	}
	if tex.destroyed || view.destroyed {
		t.Fatalf("hook-installed cache must NOT destroy the pair inline")
	}
	if c.usedBytes != 0 || len(c.entries) != 0 {
		t.Fatalf("entry bookkeeping not cleared: used=%d entries=%d", c.usedBytes, len(c.entries))
	}
}

func TestImageCache_NoHookDestroysImmediately(t *testing.T) {
	c := &ImageCache{entries: make(map[imageCacheKey]*imageCacheEntry)}
	tex := &testTexture{}
	view := &testTextureView{}
	entry := &imageCacheEntry{texture: tex, view: view, bytes: 64, gen: 1}

	c.retireEntry(entry)
	if !tex.destroyed || !view.destroyed {
		t.Fatalf("without a hook the pair must be destroyed immediately")
	}

	// Destroy path: every live entry goes through the same outlet.
	c.entries[imageCacheKey{gen: 9, levels: 1}] = &imageCacheEntry{texture: &testTexture{}, view: &testTextureView{}, bytes: 64, gen: 2}
	c.Destroy()
	if len(c.entries) != 0 {
		t.Fatalf("Destroy must clear entries, got %d", len(c.entries))
	}
}
