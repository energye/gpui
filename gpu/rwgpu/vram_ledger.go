package rwgpu

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/energye/gpui/gpu/types"
)

// Process VRAM ledger (Skia setResourceCacheLimit pattern): every texture
// and buffer creation estimates its bytes from the descriptor and charges a
// process-wide account; every Release/Destroy refunds it. When an estimate
// would exceed the budget, creation fails fast with an OOM error WITHOUT
// calling native — a doomed native attempt can pin driver heap blocks and
// turn one transient failure into a retry storm (measured 2026-09-17: a
// 3.66MB depth miss grew driver usage +540MB in 3s on a 1GB card).
//
// The ledger is an estimate, not a driver query: WebGPU exposes no heap
// totals (WGPUAdapterInfo has no memory fields by spec). It bounds OUR
// process demand so multi-window/multi-program coexistence degrades
// gracefully (fail fast → 1x1 fallback → latched retry) instead of dying
// in the driver.
//
// Budget: GPUI_VRAM_BUDGET_MB (default 768, 0 = disable). The default sits
// between a single heavy window (~100MB measured) and a 1GB card minus a
// desktop (~650MB free): five chase-class windows fit, the sixth degrades
// instead of killing the process.

var vramLedger = struct {
	sync.Mutex
	bytes map[uintptr]uint64
	total uint64
}{bytes: make(map[uintptr]uint64)}

// vramBudgetMB reads the budget once per call (cheap env read; creation is
// not a hot path relative to a driverAlloc). 0 disables enforcement.
func vramBudgetMB() int64 {
	if v := os.Getenv("GPUI_VRAM_BUDGET_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return int64(n)
		}
	}
	return 768
}

// vramBytesPerTexel maps formats to bytes per sample. Unknown formats use
// 4 (the common RGBA8/depth24+stencil cost); over-estimating fails safe.
func vramBytesPerTexel(f types.TextureFormat) uint64 {
	switch f {
	case types.TextureFormatR8Unorm, types.TextureFormatR8Snorm,
		types.TextureFormatR8Uint, types.TextureFormatR8Sint:
		return 1
	case types.TextureFormatR16Float, types.TextureFormatR16Uint,
		types.TextureFormatR16Sint, types.TextureFormatRG8Unorm:
		return 2
	case types.TextureFormatRGBA16Float, types.TextureFormatRGBA32Float,
		types.TextureFormatDepth32Float:
		return 8
	default:
		return 4
	}
}

// vramTextureBytes estimates one texture in bytes: extent × layers × samples
// × bytes-per-texel, with a 4/3 mip-chain factor when mipmapped.
func vramTextureBytes(size types.Extent3D, layers, mips, samples uint32, f types.TextureFormat) uint64 {
	w, h := uint64(size.Width), uint64(size.Height)
	if w == 0 || h == 0 {
		return 0
	}
	n := w * h * uint64(layers) * uint64(samples) * vramBytesPerTexel(f)
	if mips > 1 {
		n = n * 4 / 3
	}
	return n
}

// vramCheck charges need bytes against the budget. Nil return means go ahead
// and call native; non-nil is an OOM error shaped for IsGPUOutOfMemory and
// render.IsGPUOutOfMemory (both match "out of memory"/"not enough memory").
// CPU-only sessions (standalone probe contexts) draw nothing to the window
// device and must not consume its budget: their allocs free almost
// immediately and double-count against the window's headroom.
func vramCheck(op string, need uint64) error {
	if os.Getenv("GOGPU_RENDER_MODE") == "cpu" {
		return nil
	}
	capMB := vramBudgetMB()
	if capMB <= 0 || need == 0 {
		return nil
	}
	vramLedger.Lock()
	defer vramLedger.Unlock()
	if vramLedger.total+need > uint64(capMB)*1024*1024 {
		return &WGPUError{Op: op, Message: fmt.Sprintf(
			"process VRAM budget exceeded: need %.2f MiB, live %.2f MiB, budget %d MiB (GPUI_VRAM_BUDGET_MB)",
			float64(need)/(1024*1024), float64(vramLedger.total)/(1024*1024), capMB)}
	}
	return nil
}

// vramAdd records a live allocation. Call only after native success.
// CPU-only sessions skip the ledger (see vramCheck); refund stays
// unconditional so a mode flip between create and release never leaks.
func vramAdd(handle uintptr, need uint64) {
	if handle == 0 || need == 0 {
		return
	}
	if os.Getenv("GOGPU_RENDER_MODE") == "cpu" {
		return
	}
	vramLedger.Lock()
	defer vramLedger.Unlock()
	// Reused handle slot (should not happen; native handles are unique while
	// live): refund the stale entry first so the total never double-counts.
	if old, ok := vramLedger.bytes[handle]; ok {
		vramLedger.total -= old
	}
	vramLedger.bytes[handle] = need
	vramLedger.total += need
}

// vramForget refunds an allocation. Idempotent; safe to call from both
// Destroy and a following Release (the second is a no-op).
func vramForget(handle uintptr) {
	if handle == 0 {
		return
	}
	vramLedger.Lock()
	defer vramLedger.Unlock()
	if old, ok := vramLedger.bytes[handle]; ok {
		vramLedger.total -= old
		delete(vramLedger.bytes, handle)
	}
}

// VramLiveBytes reports the ledger total for diagnostics and tests.
func VramLiveBytes() uint64 {
	vramLedger.Lock()
	defer vramLedger.Unlock()
	return vramLedger.total
}

// VramLiveCount reports how many allocations the ledger tracks.
func VramLiveCount() int {
	vramLedger.Lock()
	defer vramLedger.Unlock()
	return len(vramLedger.bytes)
}
