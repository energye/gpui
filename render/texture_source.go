//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import "sync"

// TextureSource is content that lives in a GPU texture and therefore cannot be
// written into a display list: the pixels exist only as a handle the producer
// rewrites in place (a decoded video frame is the case this was built for).
// A retained scene graph refers to it by id (scene.TextureLayer), so the layer
// tree carries a number instead of a pointer to something mutable, and the
// raster thread resolves the number at paint time.
//
// THREAD CONTRACT: PaintTexture runs on the raster thread while the producer
// keeps feeding new content from wherever it decodes. The implementation owns
// that synchronization — the engine calls no other method of a source to help.
type TextureSource interface {
	// PaintTexture draws the source's current content into dc at opts. dc's
	// current transform applies (same user-space convention as every other
	// draw). It reports direct=true when the zero-upload GPU path was used and
	// ok=false when nothing was drawn (released source, no frame yet, no
	// device) — a false report is a hole the caller must not damage over.
	PaintTexture(dc *Context, opts TexturePaintOptions) (direct, ok bool)
}

// TexturePaintOptions locates one texture paint on the target context.
// Opacity 0 means opaque, matching VideoDrawOptions; a source that cannot
// honour a field (interpolation of a fixed-format texture) draws its default
// rather than failing the paint.
type TexturePaintOptions struct {
	X, Y    float64
	Width   float64
	Height  float64
	Opacity float64
	Interp  InterpolationMode
	// DrawIndex is which view of this source is being painted, in the paint
	// order the layer tree produced (0 = first view). A node that shows the
	// same texture twice (main view plus picture-in-picture) emits one layer
	// per view with increasing indices; the packet travels one way, so
	// per-view evidence has to be counted where the paint happens.
	// 0 = single view. Sources that do not distinguish views ignore it.
	DrawIndex int
}

var textureRegistry = struct {
	sync.RWMutex
	byID map[uint64]TextureSource
	next uint64
}{byID: make(map[uint64]TextureSource)}

// RegisterTextureSource adds src to the registry and returns its id (monotonic
// from 1; 0 is reserved for "no source"). A nil source is not registered and
// reports 0, so callers fail closed instead of storing a nil entry.
//
// Ownership follows the resource discipline: whoever registers must
// UnregisterTextureSource at teardown, otherwise the id keeps resolving to a
// source whose native texture may already be gone.
func RegisterTextureSource(src TextureSource) uint64 {
	if src == nil {
		return 0
	}
	textureRegistry.Lock()
	defer textureRegistry.Unlock()
	textureRegistry.next++
	if textureRegistry.next == 0 {
		textureRegistry.next = 1
	}
	id := textureRegistry.next
	textureRegistry.byID[id] = src
	return id
}

// UnregisterTextureSource removes id. Unregistering a paint in flight is the
// producer's problem — the contract is that the source outlives every frame
// that names its id, which is why the bridge unregisters only in Close.
// Idempotent: an unknown id is a no-op.
func UnregisterTextureSource(id uint64) {
	if id == 0 {
		return
	}
	textureRegistry.Lock()
	defer textureRegistry.Unlock()
	delete(textureRegistry.byID, id)
}

// GetTextureSource resolves an id captured by a retained layer. ok=false means
// the source is gone (closed window, unregistered producer): the caller paints
// nothing and reports a miss rather than guessing a texture.
func GetTextureSource(id uint64) (src TextureSource, ok bool) {
	if id == 0 {
		return nil, false
	}
	textureRegistry.RLock()
	defer textureRegistry.RUnlock()
	src, ok = textureRegistry.byID[id]
	return src, ok
}

// LiveTextureSources counts registered sources. Teardown assertions use it to
// prove every producer that registered also unregistered (资源谁建谁放).
func LiveTextureSources() int {
	textureRegistry.RLock()
	defer textureRegistry.RUnlock()
	return len(textureRegistry.byID)
}
