//go:build !(js && wasm)

package webgpu

import (
	"fmt"

	"github.com/energye/gpui/gpu/types"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// InstanceDescriptor configures instance creation.
// On the wgpu-native backend, Backends and Flags are accepted for API compatibility
// but the Rust wgpu-native handles backend selection internally.
type InstanceDescriptor struct {
	Backends Backends
	Flags    types.InstanceFlags
	// XlibDisplay is Display* for GL/X11 instance association (optional).
	XlibDisplay uintptr
	// XlibScreen is X11 screen index (DefaultScreen).
	XlibScreen int32
}

// Instance is the entry point for GPU operations.
// On the wgpu-native backend, this wraps rwgpu Instance.
type Instance struct {
	r        *rwgpu.Instance
	released bool
}

// CreateInstance creates a new GPU instance.
// If desc is nil, all available backends are used (unless GPUI_BACKEND /
// GPUI_LOW_VRAM env select GL / budgets — see rwgpu.CreateInstance).
func CreateInstance(desc *InstanceDescriptor) (*Instance, error) {
	if err := rwgpu.Init(); err != nil {
		return nil, fmt.Errorf("wgpu: failed to init wgpu-native: %w", err)
	}

	var rDesc *rwgpu.InstanceDescriptor
	if desc != nil {
		rDesc = &rwgpu.InstanceDescriptor{
			Backends:    desc.Backends,
			Flags:       desc.Flags,
			XlibDisplay: desc.XlibDisplay,
			XlibScreen:  desc.XlibScreen,
		}
	}
	// Always call through so env (GPUI_BACKEND / GPUI_LOW_VRAM / budget) applies
	// even when desc is nil.
	ri, err := rwgpu.CreateInstance(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create instance: %w", err)
	}

	return &Instance{r: ri}, nil
}

// RequestAdapter requests a GPU adapter matching the options.
// If opts is nil, the best available adapter is returned.
func (i *Instance) RequestAdapter(opts *RequestAdapterOptions) (*Adapter, error) {
	if i.released {
		return nil, ErrReleased
	}

	var rOpts *rwgpu.RequestAdapterOptions
	if opts != nil {
		rOpts = &rwgpu.RequestAdapterOptions{
			PowerPreference:      opts.PowerPreference,
			ForceFallbackAdapter: opts.ForceFallbackAdapter,
		}
		if opts.CompatibleSurface != nil {
			rOpts.CompatibleSurface = opts.CompatibleSurface.r
		}
	}

	ra, err := i.r.RequestAdapter(rOpts)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to request adapter: %w", err)
	}

	// Convert adapter info.
	rInfo, err := ra.Info()
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to get adapter info: %w", err)
	}
	info := AdapterInfo{
		Name:       rInfo.Description,
		Vendor:     rInfo.Vendor,
		VendorID:   rInfo.VendorID,
		DeviceID:   rInfo.DeviceID,
		Driver:     rInfo.Device,
		DriverInfo: rInfo.Architecture,
		Backend:    convertBackendType(rInfo.BackendType),
		DeviceType: convertAdapterType(rInfo.AdapterType),
	}

	// Convert limits from rwgpu.Limits to gputypes.Limits.
	rLimits := ra.Limits()
	limits := convertLimits(rLimits)

	// Convert features from []FeatureName to Features bitmask.
	rFeatures := ra.Features()
	features := convertFeatures(rFeatures)

	return &Adapter{
		r:        ra,
		info:     info,
		features: features,
		limits:   limits,
		instance: i,
	}, nil
}

// ProcessEvents pumps pending wgpu async callbacks (device-lost, map async, etc.).
// Call periodically from the render loop so device-lost is observed as a Go
// sticky fuse before the next Surface.GetCurrentTexture (which aborts native
// when the parent device is already lost).
func (i *Instance) ProcessEvents() {
	if i == nil || i.released || i.r == nil {
		return
	}
	i.r.ProcessEvents()
}

// Release releases the instance and all associated resources.
func (i *Instance) Release() {
	if i.released {
		return
	}
	i.released = true
	if i.r != nil {
		i.r.Release()
	}
}

// convertLimits converts rwgpu Limits to gputypes.Limits.
//
//nolint:dupl // Symmetric field-by-field mapping (rwgpu→gputypes vs gputypes→rwgpu), not real duplication.
func convertLimits(rl rwgpu.Limits) types.Limits {
	return types.Limits{
		MaxTextureDimension1D:                     rl.MaxTextureDimension1D,
		MaxTextureDimension2D:                     rl.MaxTextureDimension2D,
		MaxTextureDimension3D:                     rl.MaxTextureDimension3D,
		MaxTextureArrayLayers:                     rl.MaxTextureArrayLayers,
		MaxBindGroups:                             rl.MaxBindGroups,
		MaxBindGroupsPlusVertexBuffers:            rl.MaxBindGroupsPlusVertexBuffers,
		MaxBindingsPerBindGroup:                   rl.MaxBindingsPerBindGroup,
		MaxDynamicUniformBuffersPerPipelineLayout: rl.MaxDynamicUniformBuffersPerPipelineLayout,
		MaxDynamicStorageBuffersPerPipelineLayout: rl.MaxDynamicStorageBuffersPerPipelineLayout,
		MaxSampledTexturesPerShaderStage:          rl.MaxSampledTexturesPerShaderStage,
		MaxSamplersPerShaderStage:                 rl.MaxSamplersPerShaderStage,
		MaxStorageBuffersPerShaderStage:           rl.MaxStorageBuffersPerShaderStage,
		MaxStorageTexturesPerShaderStage:          rl.MaxStorageTexturesPerShaderStage,
		MaxUniformBuffersPerShaderStage:           rl.MaxUniformBuffersPerShaderStage,
		MaxUniformBufferBindingSize:               rl.MaxUniformBufferBindingSize,
		MaxStorageBufferBindingSize:               rl.MaxStorageBufferBindingSize,
		MinUniformBufferOffsetAlignment:           rl.MinUniformBufferOffsetAlignment,
		MinStorageBufferOffsetAlignment:           rl.MinStorageBufferOffsetAlignment,
		MaxVertexBuffers:                          rl.MaxVertexBuffers,
		MaxBufferSize:                             rl.MaxBufferSize,
		MaxVertexAttributes:                       rl.MaxVertexAttributes,
		MaxVertexBufferArrayStride:                rl.MaxVertexBufferArrayStride,
		MaxInterStageShaderVariables:              rl.MaxInterStageShaderVariables,
		MaxColorAttachments:                       rl.MaxColorAttachments,
		MaxColorAttachmentBytesPerSample:          rl.MaxColorAttachmentBytesPerSample,
		MaxComputeWorkgroupStorageSize:            rl.MaxComputeWorkgroupStorageSize,
		MaxComputeWorkgroupSizeX:                  rl.MaxComputeWorkgroupSizeX,
		MaxComputeWorkgroupSizeY:                  rl.MaxComputeWorkgroupSizeY,
		MaxComputeWorkgroupSizeZ:                  rl.MaxComputeWorkgroupSizeZ,
		MaxComputeWorkgroupsPerDimension:          rl.MaxComputeWorkgroupsPerDimension,
		MaxComputeInvocationsPerWorkgroup:         rl.MaxComputeInvocationsPerWorkgroup,
	}
}

// convertFeatures converts rwgpu []FeatureName to gputypes.Features bitmask.
// Maps each rwgpu FeatureName constant to the corresponding gputypes.Feature bit.
// Unrecognized features are silently ignored (wgpu-native extensions not in our bitmask).
func convertFeatures(names []rwgpu.FeatureName) types.Features {
	var features types.Features
	for _, name := range names {
		if f, ok := featureMap[name]; ok {
			features |= types.Features(f)
		}
	}
	return features
}

// featureMap maps rwgpu FeatureName constants to gputypes.Feature bitmask bits.
// Only features that exist in both rwgpu and gputypes are included.
var featureMap = map[rwgpu.FeatureName]types.Feature{
	rwgpu.FeatureNameDepthClipControl:        types.FeatureDepthClipControl,
	rwgpu.FeatureNameDepth32FloatStencil8:    types.FeatureDepth32FloatStencil8,
	rwgpu.FeatureNameTextureCompressionBC:    types.FeatureTextureCompressionBC,
	rwgpu.FeatureNameTextureCompressionETC2:  types.FeatureTextureCompressionETC2,
	rwgpu.FeatureNameTextureCompressionASTC:  types.FeatureTextureCompressionASTC,
	rwgpu.FeatureNameIndirectFirstInstance:   types.FeatureIndirectFirstInstance,
	rwgpu.FeatureNameShaderF16:               types.FeatureShaderF16,
	rwgpu.FeatureNameRG11B10UfloatRenderable: types.FeatureRG11B10UfloatRenderable,
	rwgpu.FeatureNameBGRA8UnormStorage:       types.FeatureBGRA8UnormStorage,
	rwgpu.FeatureNameFloat32Filterable:       types.FeatureFloat32Filterable,
	rwgpu.FeatureNameTimestampQuery:          types.FeatureTimestampQuery,
	rwgpu.FeatureNameSubgroups:               types.FeatureSubgroupOperations,
}

// convertBackendType maps rwgpu BackendType to gputypes.Backend.
func convertBackendType(bt rwgpu.BackendType) types.Backend {
	switch bt {
	case rwgpu.BackendTypeVulkan:
		return types.BackendVulkan
	case rwgpu.BackendTypeMetal:
		return types.BackendMetal
	case rwgpu.BackendTypeD3D12:
		return types.BackendDX12
	case rwgpu.BackendTypeOpenGL, rwgpu.BackendTypeOpenGLES:
		return types.BackendGL
	default:
		return types.BackendEmpty
	}
}

// convertAdapterType maps rwgpu AdapterType to gputypes.DeviceType.
func convertAdapterType(at rwgpu.AdapterType) types.DeviceType {
	switch at {
	case rwgpu.AdapterTypeDiscreteGPU:
		return types.DeviceTypeDiscreteGPU
	case rwgpu.AdapterTypeIntegratedGPU:
		return types.DeviceTypeIntegratedGPU
	case rwgpu.AdapterTypeCPU:
		return types.DeviceTypeCPU
	default:
		return types.DeviceTypeOther
	}
}
