//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

// viewHandle packing is owned by gpu/context (single encoding for all
// texture-view handles): pack here, unpack anywhere, including the swapchain
// frame handle produced by the backend swapchain. Before the fix this file boxed the
// view locally while the swapchain packed the concrete pointer, and every
// window went black while presents still counted — see
// gpu/context/texture_view_box.go.
package gpu

import (
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
)

// packView boxes a hal.TextureView into an opaque handle.
// Returns the zero handle for a nil view.
func packView(v hal.TextureView) gpucontext.TextureView {
	return gpucontext.PackTextureView(v)
}

// unpackView recovers the hal.TextureView from an opaque handle.
// Returns nil for nil or foreign handles.
func unpackView(h gpucontext.TextureView) hal.TextureView {
	return gpucontext.UnpackTextureView(h)
}

// unpackRawView recovers the hal.TextureView from a res.View.Raw pointer,
// which is a copy of a packed handle pointer (queue-time fallback path).
// Returns nil for nil or foreign pointers.
func unpackRawView(p unsafe.Pointer) hal.TextureView {
	return gpucontext.UnpackTextureViewRaw(p)
}
