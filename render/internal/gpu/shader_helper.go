//go:build !nogpu

package gpu

import (
	"fmt"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/shader"
)

// CompileShaderToSPIRV compiles WGSL source to SPIR-V uint32 slice.
// This is the common shader compilation logic used by all GPU rasterizers.
func CompileShaderToSPIRV(wgslSource string) ([]uint32, error) {
	// Compile WGSL to SPIR-V bytes
	spirvBytes, err := shader.Compile(wgslSource)
	if err != nil {
		return nil, fmt.Errorf("failed to compile shader: %w", err)
	}

	// Convert bytes to uint32 slice for SPIR-V
	// SPIR-V is little-endian 32-bit words
	spirvCode := make([]uint32, len(spirvBytes)/4)
	for i := range spirvCode {
		spirvCode[i] = uint32(spirvBytes[i*4]) |
			uint32(spirvBytes[i*4+1])<<8 |
			uint32(spirvBytes[i*4+2])<<16 |
			uint32(spirvBytes[i*4+3])<<24
	}

	return spirvCode, nil
}

// CreateShaderModule creates a shader module from WGSL source.
func CreateShaderModule(device hal.Device, label string, wgslSource string) (hal.ShaderModule, error) {
	return device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: label,
		WGSL:  wgslSource,
	})
}

// GPUResources holds the device, shader module and pipeline layout common to
// GPU rasterizers; released via DestroyGPUResources.
type GPUResources struct {
	Device         hal.Device
	ShaderModule   hal.ShaderModule
	PipelineLayout hal.PipelineLayout
	BindLayouts    []hal.BindGroupLayout
	Pipelines      []hal.ComputePipeline
}

// Destroy cleans up all GPU resources in the correct order.
func (r *GPUResources) Destroy() {
	if r.Device == nil {
		return
	}

	// Destroy pipelines first
	for _, p := range r.Pipelines {
		if p != nil {
			p.Destroy()
		}
	}

	// Destroy pipeline layout
	if r.PipelineLayout != nil {
		r.PipelineLayout.Destroy()
	}

	// Destroy bind group layouts
	for _, l := range r.BindLayouts {
		if l != nil {
			l.Destroy()
		}
	}

	// Destroy shader module
	if r.ShaderModule != nil {
		r.ShaderModule.Destroy()
	}
}
