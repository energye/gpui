package render

import (
	"fmt"
	"os"
	"strings"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// AdapterPolicy controls which GPU is selected.
//
// Three policies only:
//
//	Default — hybrid UI default: prefer integrated when both discrete and
//	          integrated exist (avoids multi-app OOM on small dGPUs).
//	High    — discrete-first (games / explicit performance).
//	Low     — integrated-first (force spare dGPU).
//
// Env: GPUI_POWER=high|low  (unset → Default). No other knobs.
type AdapterPolicy int

const (
	// PolicyDefault prefers integrated on hybrid machines; single-GPU uses that GPU.
	PolicyDefault AdapterPolicy = iota
	// PolicyHigh prefers discrete GPUs; falls back to integrated, then software.
	PolicyHigh
	// PolicyLow prefers integrated / low-power GPUs.
	PolicyLow
)

// Deprecated aliases (same values as above). Prefer PolicyDefault/High/Low.
const (
	PolicyNone            = PolicyDefault
	PolicyHighPerformance = PolicyHigh
	PolicyLowPower        = PolicyLow
	PolicyAuto            = PolicyDefault
)

func (p AdapterPolicy) String() string {
	switch p {
	case PolicyDefault:
		return "default"
	case PolicyHigh:
		return "high"
	case PolicyLow:
		return "low"
	default:
		return "unknown"
	}
}

// ResolveAdapterPolicy reads GPUI_POWER only.
//
//	(unset) / empty     → Default (hybrid prefer iGPU)
//	GPUI_POWER=high     → High (discrete-first)
//	GPUI_POWER=low      → Low (integrated-first)
//
// Legacy aliases still accepted: discrete/dgpu→high, integrated/igpu→low,
// none/auto/default→Default. GPUI_LOW_VRAM is ignored for adapter selection
// (device limits follow adapter type automatically).
func ResolveAdapterPolicy() AdapterPolicy {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GPUI_POWER"))) {
	case "high", "discrete", "dgpu":
		return PolicyHigh
	case "low", "integrated", "igpu":
		return PolicyLow
	default:
		// unset, "", "none", "auto", "default", unknown → Default
		return PolicyDefault
	}
}

func adapterDeviceType(a *webgpu.Adapter) types.DeviceType {
	if a == nil {
		return types.DeviceTypeOther
	}
	return a.Info().DeviceType
}

// preferIntegratedOverDiscrete keeps alt when primary is discrete and alt is
// integrated. Releases the unused adapter.
func preferIntegratedOverDiscrete(primary, alt *webgpu.Adapter) *webgpu.Adapter {
	if primary == nil {
		return alt
	}
	if alt == nil {
		return primary
	}
	if adapterDeviceType(primary) == types.DeviceTypeDiscreteGPU &&
		adapterDeviceType(alt) == types.DeviceTypeIntegratedGPU {
		primary.Release()
		return alt
	}
	alt.Release()
	return primary
}

// RequestAdapterWithPolicy selects an adapter for the instance/surface.
// forceFallback is set when only the software/CPU adapter is available.
func RequestAdapterWithPolicy(
	inst *webgpu.Instance,
	surf *webgpu.Surface,
	policy AdapterPolicy,
) (adpt *webgpu.Adapter, forceFallback bool, err error) {
	if inst == nil {
		return nil, false, fmt.Errorf("nil instance")
	}
	try := func(pref webgpu.PowerPreference, fallback bool) (*webgpu.Adapter, error) {
		opts := &webgpu.RequestAdapterOptions{
			PowerPreference:      pref,
			ForceFallbackAdapter: fallback,
		}
		if surf != nil {
			opts.CompatibleSurface = surf
		}
		return inst.RequestAdapter(opts)
	}

	switch policy {
	case PolicyLow:
		a, e := try(webgpu.PowerPreferenceLowPower, false)
		if e == nil {
			return a, false, nil
		}
		a, e = try(webgpu.PowerPreferenceHighPerformance, false)
		if e == nil {
			return a, false, nil
		}
		a, e = try(webgpu.PowerPreferenceLowPower, true)
		return a, true, e
	case PolicyHigh:
		a, e := try(webgpu.PowerPreferenceHighPerformance, false)
		if e == nil {
			return a, false, nil
		}
		a, e = try(webgpu.PowerPreferenceLowPower, false)
		if e == nil {
			return a, false, nil
		}
		a, e = try(webgpu.PowerPreferenceLowPower, true)
		return a, true, e
	default: // PolicyDefault
		a, e := try(webgpu.PowerPreferenceNone, false)
		if e == nil {
			// Hybrid: bare None often returns dGPU first under Optimus/Vulkan.
			if adapterDeviceType(a) == types.DeviceTypeDiscreteGPU {
				if b, e2 := try(webgpu.PowerPreferenceLowPower, false); e2 == nil {
					a = preferIntegratedOverDiscrete(a, b)
				}
			}
			return a, false, nil
		}
		a, e = try(webgpu.PowerPreferenceLowPower, false)
		if e == nil {
			return a, false, nil
		}
		a, e = try(webgpu.PowerPreferenceHighPerformance, false)
		if e == nil {
			return a, false, nil
		}
		a, e = try(webgpu.PowerPreferenceNone, true)
		return a, true, e
	}
}

// minStorageBuffersPerShaderStage matches render/internal/gpu (Vello coarse pass needs 9).
const minStorageBuffersPerShaderStage = 9

// DeviceDescriptor returns a DeviceDescriptor with limits suitable for GPUI
// render (including Vello compute). Use this when RequestDevice for a
// window/swapchain device that will be injected via SetDeviceProvider.
//
// Important: do NOT pass adapter.Limits() wholesale as RequiredLimits — some
// backends advertise enormous MaxStorageBuffersPerShaderStage (e.g. 524288)
// and requesting those as required can OOM device creation or tiny allocations.
// This helper starts from WebGPU DefaultLimits and only raises storage buffers.
func DeviceDescriptor(label string) *webgpu.DeviceDescriptor {
	limits := webgpu.DefaultLimits()
	if limits.MaxStorageBuffersPerShaderStage < minStorageBuffersPerShaderStage {
		limits.MaxStorageBuffersPerShaderStage = minStorageBuffersPerShaderStage
	}
	return &webgpu.DeviceDescriptor{
		Label:          label,
		RequiredLimits: limits,
	}
}

// DeviceDescriptorLowVRAM returns limits aimed at 1–2GB GPUs (e.g. 940MX).
// RequiredLimits are still *minimums the device must support* (not pre-alloc),
// but we avoid advertising 256MiB buffer / 128MiB storage binding floors that
// encourage large Vulkan heap reservations on some wgpu-native paths.
//
// Suitable for solid/UI present + modest meshes. Heavy compute/Vello paths
// may need the default DeviceDescriptor.
func DeviceDescriptorLowVRAM(label string) *webgpu.DeviceDescriptor {
	limits := webgpu.DefaultLimits()
	if limits.MaxStorageBuffersPerShaderStage < minStorageBuffersPerShaderStage {
		limits.MaxStorageBuffersPerShaderStage = minStorageBuffersPerShaderStage
	}
	// Tighten large "capability floors" that UI compositing never needs.
	limits.MaxBufferSize = 64 * 1024 * 1024               // 64 MiB (was 256)
	limits.MaxStorageBufferBindingSize = 32 * 1024 * 1024 // 32 MiB (was 128)
	limits.MaxTextureDimension2D = 4096                   // 4k enough for UI windows
	limits.MaxTextureDimension1D = 4096
	limits.MaxTextureArrayLayers = 64
	limits.MaxBindingsPerBindGroup = 128
	limits.MaxNonSamplerBindings = 10000
	return &webgpu.DeviceDescriptor{
		Label:          label,
		RequiredLimits: limits,
	}
}

// DeviceDescriptorForAdapter picks tighter LowVRAM limits for integrated/CPU
// adapters. Discrete uses full UI defaults. No env override.
func DeviceDescriptorForAdapter(label string, adpt *webgpu.Adapter) *webgpu.DeviceDescriptor {
	if adpt != nil {
		info := adpt.Info()
		if info.DeviceType == types.DeviceTypeIntegratedGPU || info.DeviceType == types.DeviceTypeCPU {
			return DeviceDescriptorLowVRAM(label)
		}
	}
	return DeviceDescriptor(label)
}
