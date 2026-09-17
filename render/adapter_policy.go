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
// adapters — plus an automatic ledger-waterline trip: when the process VRAM
// ledger already holds ≥80% of GPUI_VRAM_BUDGET_MB (768MB default), any
// adapter gets LowVRAM limits even without GPUI_LOW_VRAM=1 (hands-free on
// 1GB cards; full 256MiB buffer floors encourage heap reservations the
// driver cannot fit, and even a 3.66MB texture fails while 660MB reads free
// — measured 2026-09-17; raw vkAllocateMemory of the same size succeeds, so
// the cliff is reservation sizing, not the heap). GPUI_LOW_VRAM=1 forces
// LowVRAM on any adapter regardless of waterline. No env override for
// adapter selection (that stays policy-driven).
func DeviceDescriptorForAdapter(label string, adpt *webgpu.Adapter) *webgpu.DeviceDescriptor {
	if os.Getenv("GPUI_LOW_VRAM") == "1" || os.Getenv("GPUI_LOW_VRAM") == "true" {
		return DeviceDescriptorLowVRAM(label)
	}
	if lowVRAMWaterlineTripped() {
		return DeviceDescriptorLowVRAM(label)
	}
	if adpt != nil {
		info := adpt.Info()
		if info.DeviceType == types.DeviceTypeIntegratedGPU || info.DeviceType == types.DeviceTypeCPU {
			return DeviceDescriptorLowVRAM(label)
		}
	}
	return DeviceDescriptor(label)
}

// lowVRAMWaterlineFraction trips the automatic LowVRAM descriptor at 80% of
// the process VRAM budget: reservation sizing (not heap exhaustion) is the
// observed cliff, so the switch must fire while headroom still exists, not
// after the first OOM. GPUI_LOW_VRAM_WATERLINE_PCT overrides (1–100);
// GPUI_LOW_VRAM=1 bypasses the waterline entirely (always LowVRAM).
const lowVRAMWaterlineFraction = 0.8

// lowVRAMWaterlineTripped reports whether the process ledger already holds
// past the waterline of the budget. Budget ≤0 (disabled) never trips.
func lowVRAMWaterlineTripped() bool {
	budget := webgpu.VramBudgetMB()
	if budget <= 0 {
		return false
	}
	fraction := lowVRAMWaterlineFraction
	if v := os.Getenv("GPUI_LOW_VRAM_WATERLINE_PCT"); v != "" {
		var n int64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 1 && n <= 100 {
			fraction = float64(n) / 100.0
		}
	}
	live := webgpu.VramLiveBytes()
	return live >= uint64(float64(budget)*1024*1024*fraction)
}
