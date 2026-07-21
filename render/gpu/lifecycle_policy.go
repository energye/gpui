//go:build !nogpu

package gpu

import (
	"os"
	"strings"
	"sync/atomic"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// SurfaceLifecycle is a portable host policy for unpresentable windows
// (minimize / fully obscured) and resume. It is intentionally tiered so
// high-VRAM desktops stay close to Flutter defaults while tight GPUs
// escalate toward Skia freeGpuResources / abandon+recreate.
//
//	Normal   — pause present + Unconfigure only (Flutter-like)
//	Purge    — also DropGPU sessions / offscreen pools (Skia freeGpuResources-ish)
//	Recreate — also AbandonDevice on hide; ForceRecoverHealthy on show (tight VRAM)
//
// Auto selection:
//
//	GPUI_LIFECYCLE=normal|purge|recreate|auto  (default auto)
//	GPUI_LOW_VRAM=1 or integrated/CPU adapter → at least Purge
//	any CreateTexture OOM observed this process → Recreate (adaptive)
type SurfaceLifecycle int

const (
	// LifecycleNormal: Unconfigure while hidden; reconfigure on show.
	LifecycleNormal SurfaceLifecycle = iota
	// LifecyclePurge: Normal + drop per-context GPU sessions while hidden.
	LifecyclePurge
	// LifecycleRecreate: Purge + abandon shared device on hide; healthy recreate on show.
	LifecycleRecreate
)

func (t SurfaceLifecycle) String() string {
	switch t {
	case LifecycleNormal:
		return "normal"
	case LifecyclePurge:
		return "purge"
	case LifecycleRecreate:
		return "recreate"
	default:
		return "unknown"
	}
}

// textureOOMs counts CreateTexture OOM-class failures process-wide.
// Used only to escalate lifecycle tier (not a hard error counter for UI).
var textureOOMs atomic.Uint32

// NoteTextureOOM records a GPU texture allocation failure that looks like OOM.
// Safe from any goroutine. Escalates future ResolveSurfaceLifecycle to Recreate.
func NoteTextureOOM() {
	textureOOMs.Add(1)
}

// TextureOOMCount returns how many OOM-class texture failures were noted.
func TextureOOMCount() uint32 { return textureOOMs.Load() }

// ResetTextureOOMCount clears the adaptive OOM counter (tests only).
func ResetTextureOOMCount() { textureOOMs.Store(0) }

// ResolveSurfaceLifecycle picks the host tier for this process/adapter.
func ResolveSurfaceLifecycle(adpt *webgpu.Adapter) SurfaceLifecycle {
	switch strings.ToLower(os.Getenv("GPUI_LIFECYCLE")) {
	case "normal", "flutter", "light":
		return LifecycleNormal
	case "purge", "skia", "free":
		return LifecyclePurge
	case "recreate", "abandon", "tight":
		return LifecycleRecreate
	case "auto", "":
		// fall through
	default:
		// unknown value → auto
	}

	// Adaptive: once OOM has been seen, prefer recreate for the rest of the process.
	if textureOOMs.Load() > 0 {
		return LifecycleRecreate
	}

	// Portable default: Purge surface-bound GPU memory while unpresentable.
	// Matches Skia freeGpuResources + Flutter detaching GPU layers when the
	// platform surface is gone. Does NOT recreate the Device every Iconify
	// (that is Recreate tier / adaptive after OOM).
	// Opt into Flutter-light Unconfigure-only with GPUI_LIFECYCLE=normal.
	if os.Getenv("GPUI_LOW_VRAM") == "1" {
		return LifecyclePurge
	}
	if adpt != nil {
		info := adpt.Info()
		if info.DeviceType == types.DeviceTypeIntegratedGPU || info.DeviceType == types.DeviceTypeCPU {
			return LifecyclePurge
		}
	}
	return LifecyclePurge
}
