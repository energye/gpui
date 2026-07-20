//go:build !(js && wasm)

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

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
