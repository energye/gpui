//go:build rust

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// ShaderModule represents a compiled shader module.
// On Rust backend, this wraps go-webgpu/webgpu ShaderModule.
type ShaderModule struct {
	r        *rwgpu.ShaderModule
	device   *Device
	released bool
}

// Release destroys the shader module.
func (m *ShaderModule) Release() {
	if m.released {
		return
	}
	m.released = true
	if m.r != nil {
		m.r.Release()
	}
}
