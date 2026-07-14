//go:build !(js && wasm)

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// ShaderModule represents a compiled shader module.
// On the wgpu-native backend, this wraps rwgpu ShaderModule.
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
