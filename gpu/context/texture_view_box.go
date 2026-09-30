//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package context

import (
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
)

// textureViewBoxMagic tags handles packed by PackTextureView so Unpack can
// tell them apart from raw concrete pointers. A foreign pointer fails closed
// (Unpack returns nil) instead of decoding garbage.
const textureViewBoxMagic uint64 = 0x6770757676696577 // "gpuvview" in hex

// textureViewBox carries a hal.TextureView through the opaque TextureView
// handle. It is the single encoding for texture-view handles: producers pack
// with PackTextureView, consumers unpack with UnpackTextureView (or
// UnpackTextureViewRaw for res.View.Raw pointer copies).
//
// History: H3 片7d packed this box on the render side while the swapchain
// kept packing the concrete *TextureView pointer. Every consumer decoded the
// box layout, so swapchain frames decoded to garbage views that still tested
// non-nil — presents counted up while every window stayed black. Never put a
// concrete pointer directly into a TextureView handle; always pack the hal
// interface here.
type textureViewBox struct {
	magic uint64
	v     hal.TextureView
}

// PackTextureView boxes a hal.TextureView into an opaque handle.
// A nil view yields the zero handle.
func PackTextureView(v hal.TextureView) TextureView {
	if v == nil {
		return TextureView{}
	}
	b := &textureViewBox{magic: textureViewBoxMagic, v: v}
	return NewTextureView(unsafe.Pointer(b)) //nolint:gosec // opaque handle pattern
}

// UnpackTextureView recovers the hal.TextureView from an opaque handle.
// Returns nil for nil handles and for handles that do not carry a packed
// box (fail closed, never garbage).
func UnpackTextureView(h TextureView) hal.TextureView {
	if h.IsNil() {
		return nil
	}
	b := (*textureViewBox)(h.Pointer())
	if b == nil || b.magic != textureViewBoxMagic {
		return nil
	}
	return b.v
}

// UnpackTextureViewRaw recovers the hal.TextureView from a res.View.Raw
// pointer, which is a copy of a packed handle pointer (queue-time fallback
// path). Returns nil for nil or foreign pointers.
func UnpackTextureViewRaw(p unsafe.Pointer) hal.TextureView {
	if p == nil {
		return nil
	}
	b := (*textureViewBox)(p)
	if b == nil || b.magic != textureViewBoxMagic {
		return nil
	}
	return b.v
}
