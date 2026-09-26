//go:build !(js && wasm)

package webgpu

import "github.com/energye/gpui/gpu/hal"

// Fence is a GPU synchronization primitive.
// On the wgpu-native backend, fences are no-ops — wgpu-native handles synchronization
// internally via device polling. This type exists for API compatibility.
type Fence struct {
	released bool
}

// Release destroys the fence.
func (f *Fence) Release() {
	if f.released {
		return
	}
	f.released = true
}

// Destroy implements hal.Fence: same as Release.
func (f *Fence) Destroy() { f.Release() }

var _ hal.Fence = (*Fence)(nil)
