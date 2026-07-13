//go:build rust

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// Sampler represents a texture sampler.
// On Rust backend, this wraps go-webgpu/webgpu Sampler.
type Sampler struct {
	r        *rwgpu.Sampler
	device   *Device
	released bool
}

// Release destroys the sampler.
func (s *Sampler) Release() {
	if s.released {
		return
	}
	s.released = true
	if s.r != nil {
		s.r.Release()
	}
}
