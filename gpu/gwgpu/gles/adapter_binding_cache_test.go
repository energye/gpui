//go:build linux && !(js && wasm)

package gles

import (
	"testing"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
)

// Binding cache must fail closed: nil EGL never fast-hits, and teardown
// clears the record so a recycled handle can never match a stale entry.
func TestAdapterContext_BindingCache_NilSafe(t *testing.T) {
	ctx := NewAdapterContext(nil, &gl.Context{}, false)
	ctx.boundValid = true
	ctx.boundIsPbuffer = true
	if _, err := ctx.TryLock(); err == nil {
		t.Fatal("TryLock(nil EGL) must fail even with cache armed")
	}
	ctx.boundValid = true
	ctx.boundIsPbuffer = false
	if _, err := ctx.TryLockForSurface(0x1234); err == nil {
		t.Fatal("TryLockForSurface(nil EGL) must fail even with cache armed")
	}
	if ctx.boundValid {
		t.Fatal("failed bind must clear boundValid")
	}
}

func TestAdapterContext_BindingCache_UnlockClears(t *testing.T) {
	ctx := NewAdapterContext(nil, &gl.Context{}, false)
	_ = ctx.Lock()
	ctx.boundValid = true
	ctx.boundIsPbuffer = true
	ctx.Unlock()
	if ctx.boundValid {
		t.Fatal("Unlock must clear boundValid")
	}
	_ = ctx.Lock()
	ctx.boundValid = true
	ctx.UnlockKeepBinding()
	if !ctx.boundValid {
		t.Fatal("UnlockKeepBinding must keep boundValid")
	}
	ctx.Destroy()
	if ctx.boundValid {
		t.Fatal("Destroy must clear boundValid")
	}
	ctx.InvalidateBinding()
}
