//go:build !rust && !(js && wasm)

package webgpu_test

import (
	"errors"
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// =============================================================================
// Device.CreateBuffer — released device, nil desc, various usages
// Covers device_native.go CreateBuffer guard clauses + happy paths
// =============================================================================

func TestCreateBufferReleasedDevice(t *testing.T) {
	_, _, device := newDevice(t)
	device.Release()

	_, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "post-release",
		Size:  64,
		Usage: webgpu.BufferUsageVertex,
	})
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("CreateBuffer after Release: got %v, want ErrReleased", err)
	}
}

// =============================================================================
// Device.CreateTexture — released device, nil desc
// Covers device_native.go CreateTexture guard clauses
// =============================================================================

func TestCreateTextureReleasedDevice(t *testing.T) {
	_, _, device := newDevice(t)
	device.Release()

	_, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "post-release",
		Size:          webgpu.Extent3D{Width: 4, Height: 4, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     webgpu.TextureDimension2D,
		Format:        webgpu.TextureFormatRGBA8Unorm,
		Usage:         webgpu.TextureUsageTextureBinding,
	})
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("CreateTexture after Release: got %v, want ErrReleased", err)
	}
}

// =============================================================================
// Device.CreateSampler — released device, nil desc (creates default sampler)
// Covers device_native.go CreateSampler guard clauses
// =============================================================================

func TestCreateSamplerReleasedDevice(t *testing.T) {
	_, _, device := newDevice(t)
	device.Release()

	_, err := device.CreateSampler(&webgpu.SamplerDescriptor{Label: "post-release"})
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("CreateSampler after Release: got %v, want ErrReleased", err)
	}
}

func TestCreateSamplerNilDescCreatesDefault(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	// nil descriptor should create a sampler with default parameters.
	sampler, err := device.CreateSampler(nil)
	if err != nil {
		t.Fatalf("CreateSampler(nil): %v", err)
	}
	if sampler == nil {
		t.Fatal("CreateSampler(nil) returned nil sampler")
	}
	sampler.Release()
}

// =============================================================================
// Device.CreateShaderModule — released device, nil desc
// Covers device_native.go CreateShaderModule guard clauses
// =============================================================================

func TestCreateShaderModuleReleasedDevice(t *testing.T) {
	_, _, device := newDevice(t)
	device.Release()

	_, err := device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "post-release",
		WGSL:  "@vertex fn vs() -> @builtin(position) vec4f { return vec4f(0.0); }",
	})
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("CreateShaderModule after Release: got %v, want ErrReleased", err)
	}
}

// =============================================================================
// Device.CreateBindGroupLayout — released device, nil desc
// Covers device_native.go CreateBindGroupLayout guard clauses
// =============================================================================

func TestCreateBindGroupLayoutReleasedDevice(t *testing.T) {
	_, _, device := newDevice(t)
	device.Release()

	_, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "post-release",
	})
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("CreateBindGroupLayout after Release: got %v, want ErrReleased", err)
	}
}

// =============================================================================
// Device.CreatePipelineLayout — happy path with multiple bind group layouts
// Covers device_native.go lines 286-331 (bindGroupLayouts copy, bindGroupCount)
// =============================================================================

func TestCreatePipelineLayoutWithLayouts(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	bgl1, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label:   "pl-bgl-0",
		Entries: []webgpu.BindGroupLayoutEntry{},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout 0: %v", err)
	}
	defer bgl1.Release()

	bgl2, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "pl-bgl-1",
		Entries: []webgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: webgpu.ShaderStageVertex,
				Buffer: &types.BufferBindingLayout{
					Type:           types.BufferBindingTypeUniform,
					MinBindingSize: 16,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout 1: %v", err)
	}
	defer bgl2.Release()

	layout, err := device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "two-group-layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{bgl1, bgl2},
	})
	if err != nil {
		t.Fatalf("CreatePipelineLayout: %v", err)
	}
	if layout == nil {
		t.Fatal("CreatePipelineLayout returned nil")
	}
	layout.Release()
}

// =============================================================================
// Device.CreateRenderPipeline — released device
// Covers device_native.go CreateRenderPipeline guard clause
// =============================================================================

func TestCreateRenderPipelineReleasedDevice(t *testing.T) {
	_, _, device := newDevice(t)
	device.Release()

	_, err := device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label: "post-release",
	})
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("CreateRenderPipeline after Release: got %v, want ErrReleased", err)
	}
}

// =============================================================================
// Device.CreateComputePipeline — released device
// Covers device_native.go CreateComputePipeline guard clause
// =============================================================================

func TestCreateComputePipelineReleasedDevice(t *testing.T) {
	_, _, device := newDevice(t)
	device.Release()

	_, err := device.CreateComputePipeline(&webgpu.ComputePipelineDescriptor{
		Label:      "post-release",
		EntryPoint: "main",
	})
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("CreateComputePipeline after Release: got %v, want ErrReleased", err)
	}
}

// =============================================================================
// Device.CreateCommandEncoder — released device
// Covers device_native.go CreateCommandEncoder guard clause
// =============================================================================

func TestCreateCommandEncoderReleasedDevice(t *testing.T) {
	_, _, device := newDevice(t)
	device.Release()

	_, err := device.CreateCommandEncoder(nil)
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("CreateCommandEncoder after Release: got %v, want ErrReleased", err)
	}
}

func TestCreateCommandEncoderWithLabel(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{
		Label: "labeled-encoder",
	})
	if err != nil {
		t.Fatalf("CreateCommandEncoder with label: %v", err)
	}
	if enc == nil {
		t.Fatal("CreateCommandEncoder with label returned nil")
	}
	enc.DiscardEncoding()
}

// =============================================================================
// Device.CreateBindGroup — with buffer resources + late binding info
// Covers device_native.go CreateBindGroup lines 334-436
// (collectBindGroupResources, buildBindGroupEntryMap, late buffer binding info)
// =============================================================================

func TestCreateBindGroupWithBufferResources(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "bg-resource-buf",
		Size:  128,
		Usage: webgpu.BufferUsageUniform | webgpu.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	bgl, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "bg-resource-layout",
		Entries: []webgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: webgpu.ShaderStageVertex,
				Buffer: &types.BufferBindingLayout{
					Type:           types.BufferBindingTypeUniform,
					MinBindingSize: 128,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	defer bgl.Release()

	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "bg-with-resources",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, Buffer: buf, Offset: 0, Size: 128},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}
	if bg == nil {
		t.Fatal("CreateBindGroup returned nil")
	}
	bg.Release()
}

func TestCreateBindGroupWithLateBufferBindingInfo(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "late-bind-buf",
		Size:  512,
		Usage: webgpu.BufferUsageStorage | webgpu.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	// MinBindingSize == 0 triggers late buffer binding info path.
	bgl, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "late-bind-layout",
		Entries: []webgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: webgpu.ShaderStageCompute,
				Buffer: &types.BufferBindingLayout{
					Type:           types.BufferBindingTypeStorage,
					MinBindingSize: 0, // late binding
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	defer bgl.Release()

	// Size == 0 means "rest of buffer" path. Offset must be aligned to 256.
	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "bg-late-binding",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, Buffer: buf, Offset: 256, Size: 0},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}
	if bg == nil {
		t.Fatal("CreateBindGroup returned nil")
	}
	bg.Release()
}

func TestCreateBindGroupWithExplicitSize(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "explicit-size-buf",
		Size:  256,
		Usage: webgpu.BufferUsageStorage | webgpu.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer: %v", err)
	}
	defer buf.Release()

	bgl, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "explicit-size-layout",
		Entries: []webgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: webgpu.ShaderStageCompute,
				Buffer: &types.BufferBindingLayout{
					Type:           types.BufferBindingTypeStorage,
					MinBindingSize: 0,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	defer bgl.Release()

	// Explicit Size > 0 — does not trigger "rest of buffer" path.
	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "bg-explicit-size",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, Buffer: buf, Offset: 0, Size: 128},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}
	bg.Release()
}

func TestCreateBindGroupMultipleBufferEntries(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	buf1, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "multi-buf-1",
		Size:  64,
		Usage: webgpu.BufferUsageUniform | webgpu.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer 1: %v", err)
	}
	defer buf1.Release()

	buf2, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "multi-buf-2",
		Size:  128,
		Usage: webgpu.BufferUsageStorage | webgpu.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("CreateBuffer 2: %v", err)
	}
	defer buf2.Release()

	bgl, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "multi-buf-layout",
		Entries: []webgpu.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: webgpu.ShaderStageVertex,
				Buffer: &types.BufferBindingLayout{
					Type:           types.BufferBindingTypeUniform,
					MinBindingSize: 64,
				},
			},
			{
				Binding:    1,
				Visibility: webgpu.ShaderStageFragment,
				Buffer: &types.BufferBindingLayout{
					Type:           types.BufferBindingTypeReadOnlyStorage,
					MinBindingSize: 128,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroupLayout: %v", err)
	}
	defer bgl.Release()

	bg, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "bg-multi-buf",
		Layout: bgl,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, Buffer: buf1, Offset: 0, Size: 64},
			{Binding: 1, Buffer: buf2, Offset: 0, Size: 128},
		},
	})
	if err != nil {
		t.Fatalf("CreateBindGroup: %v", err)
	}
	bg.Release()
}

// =============================================================================
// Device.CreateFence + operations — full lifecycle
// Covers device_native.go lines 747-820 + fence.go
// =============================================================================

func TestFenceFullLifecycle(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	fence, err := device.CreateFence()
	if err != nil {
		t.Fatalf("CreateFence: %v", err)
	}

	// Check status.
	_, err = device.GetFenceStatus(fence)
	if err != nil {
		t.Fatalf("GetFenceStatus: %v", err)
	}

	// Reset.
	if err := device.ResetFence(fence); err != nil {
		t.Fatalf("ResetFence: %v", err)
	}

	// Release via deprecated path.
	device.DestroyFence(fence)
}

// =============================================================================
// Device.GetSurfaceCapabilities — nil surface, released adapter
// Covers adapter_native.go GetSurfaceCapabilities
// =============================================================================

func TestGetSurfaceCapabilitiesNilSurface(t *testing.T) {
	_, adapter := newAdapter(t)
	defer adapter.Release()

	caps := adapter.GetSurfaceCapabilities(nil)
	if caps != nil {
		t.Error("GetSurfaceCapabilities(nil) should return nil")
	}
}

func TestGetSurfaceCapabilitiesReleasedAdapter(t *testing.T) {
	_, adapter := newAdapter(t)
	adapter.Release()

	caps := adapter.GetSurfaceCapabilities(nil)
	if caps != nil {
		t.Error("GetSurfaceCapabilities on released adapter should return nil")
	}
}

func TestGetSurfaceCapabilitiesCoreOnlyPath(t *testing.T) {
	_, adapter := newAdapter(t)
	defer adapter.Release()

	// With a non-nil surface (from HAL wrap) on a mock adapter, this tests
	// the core-only path which returns default capabilities.
	surface := webgpu.NewSurfaceFromHAL(nil, "test-surface")
	caps := adapter.GetSurfaceCapabilities(surface)
	// On mock adapter without HAL, expect the core-only defaults.
	if caps != nil {
		// If caps are returned, Fifo should be present (spec guaranteed).
		hasFifo := false
		for _, pm := range caps.PresentModes {
			if pm == webgpu.PresentModeFifo {
				hasFifo = true
				break
			}
		}
		if !hasFifo {
			t.Error("SurfaceCapabilities should include PresentModeFifo")
		}
	}
}

// =============================================================================
// Device.FreeCommandBuffer — with a real encoder
// Covers device_native.go lines 825-838 (halBuffer() path)
// =============================================================================

func TestFreeCommandBufferAfterFinish(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	enc, err := device.CreateCommandEncoder(&webgpu.CommandEncoderDescriptor{
		Label: "free-cb-encoder",
	})
	if err != nil {
		t.Fatalf("CreateCommandEncoder: %v", err)
	}

	cb, err := enc.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	// FreeCommandBuffer should not panic. After this the command buffer handle
	// is invalid.
	device.FreeCommandBuffer(cb)
}

// =============================================================================
// Device.WaitIdle — released device
// Covers device_native.go WaitIdle guard clause
// =============================================================================

func TestWaitIdleReleasedDevice(t *testing.T) {
	_, _, device := newDevice(t)
	device.Release()

	err := device.WaitIdle()
	if !errors.Is(err, webgpu.ErrReleased) {
		t.Errorf("WaitIdle after Release: got %v, want ErrReleased", err)
	}
}

// =============================================================================
// Device.WaitIdle — success path + maintainAfterIdle internals
// Covers device_native.go WaitIdle happy path
// =============================================================================

func TestWaitIdleSucceeds(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()
	requireHAL(t, device)

	if err := device.WaitIdle(); err != nil {
		t.Fatalf("WaitIdle: %v", err)
	}
}

// =============================================================================
// Device.maintainAfterIdle — nil-queue early return
// Covers device_native.go maintainAfterIdle guard clause (d.queue == nil)
// =============================================================================

func TestMaintainAfterIdleNilQueue(t *testing.T) {
	d := webgpu.NewBareDeviceForTest()
	d.TestMaintainAfterIdle() // must not panic
}

// =============================================================================
// Device.PushErrorScope / PopErrorScope — nested scopes
// Covers device_native.go lines 840-848
// =============================================================================

func TestErrorScopeNestedFilters(t *testing.T) {
	_, _, device := newDevice(t)
	defer device.Release()

	// Push three nested scopes with different filters.
	device.PushErrorScope(webgpu.ErrorFilterValidation)
	device.PushErrorScope(webgpu.ErrorFilterOutOfMemory)
	device.PushErrorScope(webgpu.ErrorFilterInternal)

	// Pop in reverse order. All should be nil since no errors were generated.
	for i, name := range []string{"Internal", "OutOfMemory", "Validation"} {
		gpuErr := device.PopErrorScope()
		if gpuErr != nil {
			t.Errorf("PopErrorScope[%d] (%s): got non-nil error: %v", i, name, gpuErr)
		}
	}
}
