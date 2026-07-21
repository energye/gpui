//go:build !nogpu

package gpu

import (
	"fmt"
	"os"
	"strings"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// AdapterPolicy controls which GPU is selected.
//
// Default is discrete-first (HighPerformance): use the dedicated GPU when
// present, fall back to integrated only if no discrete adapter is available.
// LowPower / GPUI_LOW_VRAM opt into integrated-first for hybrid machines that
// must minimize dedicated VRAM (wgpu-native Vulkan on small NVIDIA cards can
// reserve ~200–320 MiB even for a clear present).
type AdapterPolicy int

const (
	// PolicyHighPerformance prefers discrete GPUs; falls back to integrated, then software.
	PolicyHighPerformance AdapterPolicy = iota
	// PolicyLowPower prefers integrated / low-power GPUs (min dedicated VRAM).
	PolicyLowPower
	// PolicyAuto is an alias of discrete-first (same order as HighPerformance).
	// Kept for env GPUI_POWER=auto / GPUI_AUTO_VRAM=1 compatibility.
	PolicyAuto
)

func (p AdapterPolicy) String() string {
	switch p {
	case PolicyHighPerformance:
		return "high"
	case PolicyLowPower:
		return "low"
	case PolicyAuto:
		return "auto"
	default:
		return "unknown"
	}
}

// ResolveAdapterPolicy reads GPUI_POWER / GPUI_LOW_VRAM / GPUI_AUTO_VRAM.
//
//	GPUI_POWER=high|low|auto   (auto = discrete-first, same as high)
//	GPUI_LOW_VRAM=1            → LowPower (integrated-first)
//	GPUI_AUTO_VRAM=1           → same as default high (discrete-first)
//
// Default when nothing set: PolicyHighPerformance — discrete if present,
// integrated only when no discrete adapter is available.
func ResolveAdapterPolicy() AdapterPolicy {
	switch strings.ToLower(os.Getenv("GPUI_POWER")) {
	case "high", "discrete", "dgpu":
		return PolicyHighPerformance
	case "low", "integrated", "igpu":
		return PolicyLowPower
	case "auto":
		// Discrete-first (same order as HighPerformance).
		return PolicyAuto
	}
	if os.Getenv("GPUI_LOW_VRAM") == "1" {
		return PolicyLowPower
	}
	// Default: discrete GPU when available; iGPU only as fallback.
	return PolicyHighPerformance
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
	case PolicyLowPower:
		// Integrated-first (explicit GPUI_LOW_VRAM / GPUI_POWER=low).
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
	default: // HighPerformance and Auto: discrete first, then integrated, then software.
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
	}
}

// DeviceDescriptorForAdapter picks LowVRAM limits for integrated/CPU adapters
// or when GPUI_LOW_VRAM=1.
func DeviceDescriptorForAdapter(label string, adpt *webgpu.Adapter) *webgpu.DeviceDescriptor {
	if os.Getenv("GPUI_LOW_VRAM") == "1" {
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
