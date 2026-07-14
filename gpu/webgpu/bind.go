//go:build !(js && wasm)

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// BindGroupLayout defines the structure of resource bindings for shaders.
// On the wgpu-native backend, this wraps rwgpu BindGroupLayout.
type BindGroupLayout struct {
	r        *rwgpu.BindGroupLayout
	device   *Device
	released bool
}

// Release destroys the bind group layout.
func (l *BindGroupLayout) Release() {
	if l.released {
		return
	}
	l.released = true
	if l.r != nil {
		l.r.Release()
	}
}

// PipelineLayout defines the bind group layout arrangement for a pipeline.
// On the wgpu-native backend, this wraps rwgpu PipelineLayout.
type PipelineLayout struct {
	r        *rwgpu.PipelineLayout
	device   *Device
	released bool
}

// Release destroys the pipeline layout.
func (l *PipelineLayout) Release() {
	if l.released {
		return
	}
	l.released = true
	if l.r != nil {
		l.r.Release()
	}
}

// LateBufferBindingInfo records the actual buffer binding size for a layout entry
// with MinBindingSize == 0.
type LateBufferBindingInfo struct {
	BindingIndex uint32
	Size         uint64
}

// BindGroup represents bound GPU resources for shader access.
// On the wgpu-native backend, this wraps rwgpu BindGroup.
type BindGroup struct {
	r        *rwgpu.BindGroup
	device   *Device
	released bool
}

// Release marks the bind group for destruction.
func (g *BindGroup) Release() {
	if g.released {
		return
	}
	g.released = true
	if g.r != nil {
		g.r.Release()
	}
}
