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

import (
	"sync"
	"sync/atomic"
)

// TextureRef is a recordable, logical reference to a device-owned texture
// (Skia GrSurfaceProxy / Flutter ui.Image shape). It carries no device pointer:
// only a process-local id, so it can travel through a display list from the UI
// thread to the raster thread and survive a device rebuild. Playback resolves
// it with ResolveTextureRef into the current device's TextureView.
//
// Who registers it, releases it: the owner of the GPU texture calls
// RegisterTextureRef when the texture is published and ReleaseTextureRef when
// the texture is freed, so a recording can never resolve a dangling pointer.
type TextureRef struct {
	id uint64
}

// Valid reports whether the ref is still published (registered and not yet
// released). Resolution, not the id alone, is authoritative.
func (r TextureRef) Valid() bool {
	if r.id == 0 {
		return false
	}
	textureRefMu.RLock()
	_, ok := textureRefTab[r.id]
	textureRefMu.RUnlock()
	return ok
}

type textureRefEntry struct {
	resolve func() TextureView
}

var (
	textureRefNext atomic.Uint64
	textureRefMu   sync.RWMutex
	textureRefTab  = make(map[uint64]textureRefEntry)
)

// RegisterTextureRef publishes a device texture as a recordable logical ref.
// resolve is called on the raster thread at playback time; it must return the
// view valid on the current device (or a nil view when the texture is gone).
// A nil resolve returns an invalid ref.
func RegisterTextureRef(resolve func() TextureView) TextureRef {
	if resolve == nil {
		return TextureRef{}
	}
	id := textureRefNext.Add(1)
	textureRefMu.Lock()
	textureRefTab[id] = textureRefEntry{resolve: resolve}
	textureRefMu.Unlock()
	return TextureRef{id: id}
}

// ReleaseTextureRef unpublishes a ref. Playback then resolves it as invalid
// and skips the draw instead of sampling a freed texture.
func ReleaseTextureRef(r TextureRef) {
	if !r.Valid() {
		return
	}
	textureRefMu.Lock()
	delete(textureRefTab, r.id)
	textureRefMu.Unlock()
}

// ResolveTextureRef turns a logical ref into the texture view valid on the
// current device. ok=false when the ref was released, never registered, or its
// owner reports the texture is gone.
func ResolveTextureRef(r TextureRef) (TextureView, bool) {
	if r.id == 0 {
		return TextureView{}, false
	}
	textureRefMu.RLock()
	e, ok := textureRefTab[r.id]
	textureRefMu.RUnlock()
	if !ok {
		return TextureView{}, false
	}
	v := e.resolve()
	if v.IsNil() {
		return TextureView{}, false
	}
	return v, true
}
