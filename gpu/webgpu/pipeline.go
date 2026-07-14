//go:build !(js && wasm)

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// LateSizedBufferGroup holds the shader-required minimum buffer sizes for
// bind group entries whose layout specifies MinBindingSize == 0.
type LateSizedBufferGroup struct {
	ShaderSizes []uint64
}

// RenderPipeline represents a configured render pipeline.
// On the wgpu-native backend, this wraps rwgpu RenderPipeline.
type RenderPipeline struct {
	r        *rwgpu.RenderPipeline
	device   *Device
	released bool
}

// Release destroys the render pipeline.
func (p *RenderPipeline) Release() {
	if p.released {
		return
	}
	p.released = true
	if p.r != nil {
		p.r.Release()
	}
}

// ComputePipeline represents a configured compute pipeline.
// On the wgpu-native backend, this wraps rwgpu ComputePipeline.
type ComputePipeline struct {
	r        *rwgpu.ComputePipeline
	device   *Device
	released bool
}

// Release destroys the compute pipeline.
func (p *ComputePipeline) Release() {
	if p.released {
		return
	}
	p.released = true
	if p.r != nil {
		p.r.Release()
	}
}
