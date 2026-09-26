//go:build !(js && wasm)

package webgpu

import (
	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// Sampler represents a texture sampler.
// On the wgpu-native backend, this wraps rwgpu Sampler.
type Sampler struct {
	r        *rwgpu.Sampler
	device   *Device
	released bool
}

// Release destroys the sampler.
func (s *Sampler) Release() {
	if s == nil || s.released {
		return
	}
	s.released = true
	if s.r != nil {
		s.r.Release()
	}
}

// Destroy implements hal.Sampler: same as Release.
func (s *Sampler) Destroy() { s.Release() }

// NativeHandle implements hal.NativeHandle: Rust handle not exposed, returns 0.
func (s *Sampler) NativeHandle() uintptr { return 0 }

var _ hal.Sampler = (*Sampler)(nil)
