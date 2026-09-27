//go:build !nogpu

package gpu

import (
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
)

// viewHolder boxes a hal.TextureView for transport through the opaque
// gpucontext.TextureView handle (片7d: Pointer 透传改装 hal holder).
//
// Before 片7d the handle carried a concrete texture-view pointer and every
// consumer recovered it with a pointer cast plus a concrete-type assertion
// on .Texture(). After 片7d producers only hold the hal iface,
// so the handle carries a pointer to this holder instead; consumers recover
// the hal iface with unpackView and never name a concrete type.
//
// Lifetime: the holder is heap-allocated; the handle's unsafe.Pointer keeps
// it alive while the handle (or a res.View.Raw copy of its pointer) is
// reachable. The boxed view itself is owned by its producer (texture cache /
// brush-cover retain); the holder never destroys it.
type viewHolder struct {
	v hal.TextureView
}

// packView boxes a hal.TextureView into an opaque handle.
// Returns the zero handle for a nil view.
func packView(v hal.TextureView) gpucontext.TextureView {
	if v == nil {
		return gpucontext.TextureView{}
	}
	h := &viewHolder{v: v}
	return gpucontext.NewTextureView(unsafe.Pointer(h)) //nolint:gosec // opaque handle pattern
}

// unpackView recovers the hal.TextureView from an opaque handle.
// Returns nil for nil handles.
func unpackView(h gpucontext.TextureView) hal.TextureView {
	if h.IsNil() {
		return nil
	}
	return (*viewHolder)(h.Pointer()).v
}

// unpackRawView recovers the hal.TextureView from a res.View.Raw pointer,
// which is a copy of a packed handle pointer (queue-time fallback path).
// Returns nil for nil pointers.
func unpackRawView(p unsafe.Pointer) hal.TextureView {
	if p == nil {
		return nil
	}
	return (*viewHolder)(p).v
}
