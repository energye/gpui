//go:build !nogpu

package gpu

import (
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render/scene"
)

// PipelineCache caches compiled GPU pipelines for rendering operations.
// It manages bind group layouts and pipelines for blit, blend, strip
// rasterization, and compositing operations.
//
// PipelineCache is safe for concurrent read access. Pipeline creation
// is synchronized internally.
type PipelineCache struct {
	mu sync.RWMutex

	// GPU device for pipeline creation
	device *webgpu.Device

	// Shader modules reference
	shaders *ShaderModules

	// Cached render pipelines
	blitPipeline            StubPipelineID
	compositePipeline       StubPipelineID
	nativeBlitPipeline      *webgpu.RenderPipeline
	nativeCompositePipeline *webgpu.RenderPipeline

	// Blend mode pipelines (one per blend mode for now)
	blendPipelines       map[scene.BlendMode]StubPipelineID
	nativeBlendPipelines map[scene.BlendMode]*webgpu.RenderPipeline

	// Compute pipeline for strip rasterization
	stripPipeline       StubComputePipelineID
	nativeStripPipeline *webgpu.ComputePipeline

	// Bind group layouts
	blitLayout                  StubBindGroupLayoutID
	blendLayout                 StubBindGroupLayoutID
	stripLayout                 StubBindGroupLayoutID
	compositeLayout             StubBindGroupLayoutID
	nativeBlitLayout            *webgpu.BindGroupLayout
	nativeBlendLayout           *webgpu.BindGroupLayout
	nativeStripLayout           *webgpu.BindGroupLayout
	nativeCompositeLayout       *webgpu.BindGroupLayout
	nativeCompositeParamsLayout *webgpu.BindGroupLayout

	// Pipeline layouts and shared resources.
	blitPipelineLayout      *webgpu.PipelineLayout
	blendPipelineLayout     *webgpu.PipelineLayout
	stripPipelineLayout     *webgpu.PipelineLayout
	compositePipelineLayout *webgpu.PipelineLayout
	defaultSampler          *webgpu.Sampler

	// State
	initialized bool
}

// StubPipelineID is a placeholder for actual wgpu RenderPipelineID.
// This will be replaced with core.RenderPipelineID when wgpu support is complete.
type StubPipelineID uint64

// StubComputePipelineID is a placeholder for actual wgpu ComputePipelineID.
type StubComputePipelineID uint64

// StubBindGroupLayoutID is a placeholder for actual wgpu BindGroupLayoutID.
type StubBindGroupLayoutID uint64

// StubBindGroupID is a placeholder for actual wgpu BindGroupID.
type StubBindGroupID uint64

// InvalidPipelineID represents an invalid/uninitialized pipeline.
const InvalidPipelineID StubPipelineID = 0

// NewPipelineCache creates a new pipeline cache for the given device.
// It initializes all base pipelines using the provided shader modules.
//
// Returns an error if pipeline creation fails.
func NewPipelineCache(device *webgpu.Device, shaders *ShaderModules) (*PipelineCache, error) {
	if shaders == nil || !shaders.IsValid() {
		return nil, ErrNotImplemented
	}

	pc := &PipelineCache{
		device:               device,
		shaders:              shaders,
		blendPipelines:       make(map[scene.BlendMode]StubPipelineID),
		nativeBlendPipelines: make(map[scene.BlendMode]*webgpu.RenderPipeline),
	}

	// Create base pipelines
	if err := pc.createBlitPipeline(); err != nil {
		return nil, err
	}

	if err := pc.createStripPipeline(); err != nil {
		return nil, err
	}

	if err := pc.createCompositePipeline(); err != nil {
		return nil, err
	}

	pc.initialized = true
	return pc, nil
}

// createBlitPipeline creates the blit (texture copy) pipeline.
func (pc *PipelineCache) createBlitPipeline() error {
	pc.blitLayout = StubBindGroupLayoutID(1)
	pc.blitPipeline = StubPipelineID(1)

	if !pc.hasNativeDevice() {
		return nil
	}

	layout, err := pc.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "gg_blit_bind_group_layout",
		Entries: []types.BindGroupLayoutEntry{
			textureBinding(0),
			samplerBinding(1),
		},
	})
	if err != nil {
		return fmt.Errorf("create blit bind group layout: %w", err)
	}
	pc.nativeBlitLayout = layout

	pipeLayout, err := pc.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "gg_blit_pipeline_layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{pc.nativeBlitLayout},
	})
	if err != nil {
		return fmt.Errorf("create blit pipeline layout: %w", err)
	}
	pc.blitPipelineLayout = pipeLayout

	pipeline, err := pc.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "gg_blit_pipeline",
		Layout: pc.blitPipelineLayout,
		Vertex: webgpu.VertexState{
			Module:     pc.shaders.BlitModule,
			EntryPoint: "vs_main",
		},
		Fragment: &webgpu.FragmentState{
			Module:     pc.shaders.BlitModule,
			EntryPoint: "fs_main",
			Targets:    []types.ColorTargetState{defaultColorTarget()},
		},
		Primitive:   types.DefaultPrimitiveState(),
		Multisample: types.DefaultMultisampleState(),
	})
	if err != nil {
		return fmt.Errorf("create blit pipeline: %w", err)
	}
	pc.nativeBlitPipeline = pipeline

	return nil
}

// createStripPipeline creates the strip rasterization compute pipeline.
func (pc *PipelineCache) createStripPipeline() error {
	pc.stripLayout = StubBindGroupLayoutID(2)
	pc.stripPipeline = StubComputePipelineID(1)

	if !pc.hasNativeDevice() {
		return nil
	}

	layout, err := pc.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "gg_strip_bind_group_layout",
		Entries: []types.BindGroupLayoutEntry{
			readOnlyStorageBinding(0),
			readOnlyStorageBinding(1),
			uniformBinding(2, 32),
			{
				Binding:    3,
				Visibility: types.ShaderStageCompute,
				StorageTexture: &types.StorageTextureBindingLayout{
					Access:        types.StorageTextureAccessWriteOnly,
					Format:        types.TextureFormatRGBA8Unorm,
					ViewDimension: types.TextureViewDimension2D,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create strip bind group layout: %w", err)
	}
	pc.nativeStripLayout = layout

	pipeLayout, err := pc.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "gg_strip_pipeline_layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{pc.nativeStripLayout},
	})
	if err != nil {
		return fmt.Errorf("create strip pipeline layout: %w", err)
	}
	pc.stripPipelineLayout = pipeLayout

	pipeline, err := pc.device.CreateComputePipeline(&webgpu.ComputePipelineDescriptor{
		Label:      "gg_strip_pipeline",
		Layout:     pc.stripPipelineLayout,
		Module:     pc.shaders.StripModule,
		EntryPoint: "cs_main",
	})
	if err != nil {
		return fmt.Errorf("create strip pipeline: %w", err)
	}
	pc.nativeStripPipeline = pipeline

	return nil
}

// createCompositePipeline creates the layer compositing pipeline.
func (pc *PipelineCache) createCompositePipeline() error {
	pc.compositeLayout = StubBindGroupLayoutID(3)
	pc.compositePipeline = StubPipelineID(2)

	if !pc.hasNativeDevice() {
		return nil
	}

	textureLayout, err := pc.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "gg_composite_texture_layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: types.ShaderStageFragment,
				Texture: &types.TextureBindingLayout{
					SampleType:    types.TextureSampleTypeFloat,
					ViewDimension: types.TextureViewDimension2DArray,
				},
			},
			samplerBinding(1),
		},
	})
	if err != nil {
		return fmt.Errorf("create composite texture layout: %w", err)
	}
	pc.nativeCompositeLayout = textureLayout

	paramsLayout, err := pc.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "gg_composite_params_layout",
		Entries: []types.BindGroupLayoutEntry{
			readOnlyStorageBinding(0),
			uniformBinding(1, 16),
		},
	})
	if err != nil {
		return fmt.Errorf("create composite params layout: %w", err)
	}
	pc.nativeCompositeParamsLayout = paramsLayout

	pipeLayout, err := pc.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label: "gg_composite_pipeline_layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{
			pc.nativeCompositeLayout,
			pc.nativeCompositeParamsLayout,
		},
	})
	if err != nil {
		return fmt.Errorf("create composite pipeline layout: %w", err)
	}
	pc.compositePipelineLayout = pipeLayout

	pipeline, err := pc.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "gg_composite_pipeline",
		Layout: pc.compositePipelineLayout,
		Vertex: webgpu.VertexState{
			Module:     pc.shaders.CompositeModule,
			EntryPoint: "vs_main",
		},
		Fragment: &webgpu.FragmentState{
			Module:     pc.shaders.CompositeModule,
			EntryPoint: "fs_main",
			Targets:    []types.ColorTargetState{defaultColorTarget()},
		},
		Primitive:   types.DefaultPrimitiveState(),
		Multisample: types.DefaultMultisampleState(),
	})
	if err != nil {
		return fmt.Errorf("create composite pipeline: %w", err)
	}
	pc.nativeCompositePipeline = pipeline

	return nil
}

// GetBlitPipeline returns the blit pipeline.
func (pc *PipelineCache) GetBlitPipeline() StubPipelineID {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.blitPipeline
}

// GetBlendPipeline returns the pipeline for the specified blend mode.
// Pipelines are created on demand and cached.
func (pc *PipelineCache) GetBlendPipeline(mode scene.BlendMode) StubPipelineID {
	pc.mu.RLock()
	pipeline, ok := pc.blendPipelines[mode]
	pc.mu.RUnlock()

	if ok {
		return pipeline
	}

	// Create pipeline for this blend mode
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Double-check after acquiring write lock
	if pipeline, ok = pc.blendPipelines[mode]; ok {
		return pipeline
	}

	pipeline = pc.createBlendPipeline(mode)
	pc.blendPipelines[mode] = pipeline

	return pipeline
}

// createBlendPipeline creates a render pipeline for a specific blend mode.
func (pc *PipelineCache) createBlendPipeline(mode scene.BlendMode) StubPipelineID {
	if pc.blendLayout == 0 {
		pc.blendLayout = StubBindGroupLayoutID(4)
	}

	if pc.hasNativeDevice() && pc.nativeBlendLayout == nil {
		if err := pc.createNativeBlendResources(); err != nil {
			return InvalidPipelineID
		}
	}

	if pc.hasNativeDevice() {
		pipeline, err := pc.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
			Label:  fmt.Sprintf("gg_blend_pipeline_%d", mode),
			Layout: pc.blendPipelineLayout,
			Vertex: webgpu.VertexState{
				Module:     pc.shaders.BlendModule,
				EntryPoint: "vs_main",
			},
			Fragment: &webgpu.FragmentState{
				Module:     pc.shaders.BlendModule,
				EntryPoint: "fs_main",
				Targets:    []types.ColorTargetState{defaultColorTarget()},
			},
			Primitive:   types.DefaultPrimitiveState(),
			Multisample: types.DefaultMultisampleState(),
		})
		if err != nil {
			return InvalidPipelineID
		}
		pc.nativeBlendPipelines[mode] = pipeline
	}

	return StubPipelineID(100 + uint64(mode))
}

func (pc *PipelineCache) createNativeBlendResources() error {
	layout, err := pc.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "gg_blend_bind_group_layout",
		Entries: []types.BindGroupLayoutEntry{
			textureBinding(0),
			textureBinding(1),
			samplerBinding(2),
			uniformBinding(3, 16),
		},
	})
	if err != nil {
		return fmt.Errorf("create blend bind group layout: %w", err)
	}
	pc.nativeBlendLayout = layout

	pipeLayout, err := pc.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "gg_blend_pipeline_layout",
		BindGroupLayouts: []*webgpu.BindGroupLayout{pc.nativeBlendLayout},
	})
	if err != nil {
		return fmt.Errorf("create blend pipeline layout: %w", err)
	}
	pc.blendPipelineLayout = pipeLayout
	return nil
}

// GetStripPipeline returns the strip rasterization compute pipeline.
func (pc *PipelineCache) GetStripPipeline() StubComputePipelineID {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.stripPipeline
}

// GetCompositePipeline returns the compositing pipeline.
func (pc *PipelineCache) GetCompositePipeline() StubPipelineID {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.compositePipeline
}

// NativeBlitPipeline returns the WebGPU blit pipeline for runtime rendering.
func (pc *PipelineCache) NativeBlitPipeline() *webgpu.RenderPipeline {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.nativeBlitPipeline
}

// NativeBlendPipeline returns the WebGPU blend pipeline for the specified mode.
func (pc *PipelineCache) NativeBlendPipeline(mode scene.BlendMode) *webgpu.RenderPipeline {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.nativeBlendPipelines[mode]
}

// NativeStripPipeline returns the WebGPU strip compute pipeline.
func (pc *PipelineCache) NativeStripPipeline() *webgpu.ComputePipeline {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.nativeStripPipeline
}

// NativeCompositePipeline returns the WebGPU composite render pipeline.
func (pc *PipelineCache) NativeCompositePipeline() *webgpu.RenderPipeline {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.nativeCompositePipeline
}

// GetBlitLayout returns the bind group layout for blit operations.
func (pc *PipelineCache) GetBlitLayout() StubBindGroupLayoutID {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.blitLayout
}

// GetBlendLayout returns the bind group layout for blend operations.
func (pc *PipelineCache) GetBlendLayout() StubBindGroupLayoutID {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.blendLayout
}

// GetStripLayout returns the bind group layout for strip compute.
func (pc *PipelineCache) GetStripLayout() StubBindGroupLayoutID {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.stripLayout
}

// HasNativePipelines reports whether the runtime WebGPU pipelines were created.
func (pc *PipelineCache) HasNativePipelines() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.nativeBlitPipeline != nil &&
		pc.nativeStripPipeline != nil &&
		pc.nativeCompositePipeline != nil
}

// IsInitialized returns true if the cache has been initialized.
func (pc *PipelineCache) IsInitialized() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.initialized
}

// Close releases all pipeline resources.
func (pc *PipelineCache) Close() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	releaseRenderPipeline(&pc.nativeBlitPipeline)
	releaseRenderPipeline(&pc.nativeCompositePipeline)
	for mode, p := range pc.nativeBlendPipelines {
		if p != nil {
			p.Release()
		}
		delete(pc.nativeBlendPipelines, mode)
	}
	if pc.nativeStripPipeline != nil {
		pc.nativeStripPipeline.Release()
		pc.nativeStripPipeline = nil
	}
	releaseBindGroupLayout(&pc.nativeBlitLayout)
	releaseBindGroupLayout(&pc.nativeBlendLayout)
	releaseBindGroupLayout(&pc.nativeStripLayout)
	releaseBindGroupLayout(&pc.nativeCompositeLayout)
	releaseBindGroupLayout(&pc.nativeCompositeParamsLayout)
	releasePipelineLayout(&pc.blitPipelineLayout)
	releasePipelineLayout(&pc.blendPipelineLayout)
	releasePipelineLayout(&pc.stripPipelineLayout)
	releasePipelineLayout(&pc.compositePipelineLayout)
	if pc.defaultSampler != nil {
		pc.defaultSampler.Release()
		pc.defaultSampler = nil
	}

	pc.blitPipeline = InvalidPipelineID
	pc.stripPipeline = 0
	pc.compositePipeline = InvalidPipelineID
	pc.blendPipelines = nil
	if pc.shaders != nil {
		pc.shaders.Release()
	}
	pc.shaders = nil
	pc.blitLayout = 0
	pc.blendLayout = 0
	pc.stripLayout = 0
	pc.compositeLayout = 0
	pc.initialized = false
}

// BlendPipelineCount returns the number of cached blend pipelines.
// Useful for debugging and monitoring.
func (pc *PipelineCache) BlendPipelineCount() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.blendPipelines)
}

// WarmupBlendPipelines pre-creates pipelines for commonly used blend modes.
// This avoids pipeline compilation stutter during first use.
func (pc *PipelineCache) WarmupBlendPipelines() {
	commonModes := []scene.BlendMode{
		scene.BlendNormal,
		scene.BlendMultiply,
		scene.BlendScreen,
		scene.BlendOverlay,
		scene.BlendSourceOver,
	}

	for _, mode := range commonModes {
		_ = pc.GetBlendPipeline(mode)
	}
}

// BindGroupBuilder helps construct bind groups for rendering.
type BindGroupBuilder struct {
	device *webgpu.Device
	layout StubBindGroupLayoutID
}

// NewBindGroupBuilder creates a new bind group builder.
func NewBindGroupBuilder(device *webgpu.Device, layout StubBindGroupLayoutID) *BindGroupBuilder {
	return &BindGroupBuilder{
		device: device,
		layout: layout,
	}
}

// Build creates the bind group. Currently returns a stub.
func (b *BindGroupBuilder) Build() StubBindGroupID {
	// TODO: When wgpu is ready, create actual bind group
	return StubBindGroupID(1)
}

// CreateBlitBindGroup creates a bind group for blit operations.
func (pc *PipelineCache) CreateBlitBindGroup(tex *GPUTexture) StubBindGroupID {
	// TODO: When wgpu is ready:
	// entries := []gputypes.BindGroupEntry{
	//     {Binding: 0, TextureView: tex.ViewID()},
	//     {Binding: 1, Sampler: pc.defaultSampler},
	// }
	// return core.CreateBindGroup(pc.device, pc.blitLayout, entries)

	return StubBindGroupID(1)
}

// CreateBlendBindGroup creates a bind group for blend operations.
func (pc *PipelineCache) CreateBlendBindGroup(tex *GPUTexture, params *BlendParams) StubBindGroupID {
	// TODO: When wgpu is ready:
	// Upload params to uniform buffer
	// Create bind group with texture, sampler, and params buffer

	return StubBindGroupID(2)
}

// CreateStripBindGroup creates a bind group for strip compute operations.
func (pc *PipelineCache) CreateStripBindGroup(
	headerBuffer StubBufferID,
	coverageBuffer StubBufferID,
	outputTex *GPUTexture,
	params *StripParams,
) StubBindGroupID {
	// TODO: When wgpu is ready:
	// Create bind group with buffers, texture, and params

	return StubBindGroupID(3)
}

// StubBufferID is a placeholder for actual wgpu BufferID.
type StubBufferID uint64

func (pc *PipelineCache) hasNativeDevice() bool {
	return pc.device != nil && pc.shaders != nil && pc.shaders.HasNativeModules()
}

func textureBinding(binding uint32) types.BindGroupLayoutEntry {
	return types.BindGroupLayoutEntry{
		Binding:    binding,
		Visibility: types.ShaderStageFragment,
		Texture: &types.TextureBindingLayout{
			SampleType:    types.TextureSampleTypeFloat,
			ViewDimension: types.TextureViewDimension2D,
		},
	}
}

func samplerBinding(binding uint32) types.BindGroupLayoutEntry {
	return types.BindGroupLayoutEntry{
		Binding:    binding,
		Visibility: types.ShaderStageFragment,
		Sampler: &types.SamplerBindingLayout{
			Type: types.SamplerBindingTypeFiltering,
		},
	}
}

func uniformBinding(binding uint32, minSize uint64) types.BindGroupLayoutEntry {
	return types.BindGroupLayoutEntry{
		Binding:    binding,
		Visibility: types.ShaderStagesAll,
		Buffer: &types.BufferBindingLayout{
			Type:           types.BufferBindingTypeUniform,
			MinBindingSize: minSize,
		},
	}
}

func readOnlyStorageBinding(binding uint32) types.BindGroupLayoutEntry {
	return types.BindGroupLayoutEntry{
		Binding:    binding,
		Visibility: types.ShaderStagesAll,
		Buffer: &types.BufferBindingLayout{
			Type: types.BufferBindingTypeReadOnlyStorage,
		},
	}
}

func defaultColorTarget() types.ColorTargetState {
	return types.ColorTargetState{
		Format:    types.TextureFormatRGBA8Unorm,
		WriteMask: types.ColorWriteMaskAll,
	}
}

func releaseRenderPipeline(p **webgpu.RenderPipeline) {
	if *p != nil {
		(*p).Release()
		*p = nil
	}
}

func releaseBindGroupLayout(l **webgpu.BindGroupLayout) {
	if *l != nil {
		(*l).Release()
		*l = nil
	}
}

func releasePipelineLayout(l **webgpu.PipelineLayout) {
	if *l != nil {
		(*l).Release()
		*l = nil
	}
}
