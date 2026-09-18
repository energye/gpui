package render

import (
	"github.com/energye/gpui/gpu/webgpu"
)

// Shared-window budget coordination (R0-5) and shared-device recovery
// registry (R0-6). All state is guarded by shareMu in present_target.go;
// shareGen bumps every time the published device changes (publish or
// shared recovery) so the present path can detect a device swap.
//
// Layering: ui/* consults these helpers (ui → render). render already
// imports gpu/webgpu; ui must never import gpu directly.

// shareTargets tracks every live PresentTarget on the shared device so a
// shared recovery can reconfigure all swapchains exactly once. Keyed by
// target pointer; entries are added on open and removed on Close.
var shareTargets = make(map[*PresentTarget]struct{})

// shareGen is the published-device generation. Bump under shareMu whenever
// shareDevice changes.
var shareGen uint64

// SharedWindowCount reports how many live windows share the process device
// (0 when no share is published). Per-window caches divide process budgets
// by this number (R0-5 fair share).
func SharedWindowCount() int {
	shareMu.Lock()
	defer shareMu.Unlock()
	if shareDevice == nil {
		return 0
	}
	return len(shareTargets)
}

// SharedDeviceGeneration reports the published-device generation.
func SharedDeviceGeneration() uint64 {
	shareMu.Lock()
	defer shareMu.Unlock()
	return shareGen
}

// VramBudgetMB exposes the process VRAM budget in MiB
// (GPUI_VRAM_BUDGET_MB, default 768, 0 = disabled).
func VramBudgetMB() int64 {
	return webgpu.VramBudgetMB()
}

// VramLiveBytes reports the process ledger's live estimate in bytes.
func VramLiveBytes() uint64 {
	return webgpu.VramLiveBytes()
}

// VramLiveCount reports how many allocations the ledger tracks.
func VramLiveCount() int {
	return webgpu.VramLiveCount()
}

// VramPeakBytes reports the high-water mark (R6 post-fix measurement).
func VramPeakBytes() uint64 {
	return webgpu.VramPeakBytes()
}

// VramPressureHigh reports whether the process ledger already holds past
// the low-VRAM waterline (same 80% gate the adapter policy uses to switch
// descriptors). Per-window caches refuse automatic growth past this point
// when more than one window is live (R0-5).
func VramPressureHigh() bool {
	return lowVRAMWaterlineTripped()
}

// pictureCacheBudgetDivisor splits the process budget across its big
// consumers: swapchain surfaces, the picture texture cache, the image
// cache, and staging/vertex buffers. The picture cache of one window may
// therefore use at most budget/windows/divisor bytes. Conservative on
// purpose: refusing growth only forces re-records (correctness fallback),
// while over-growing OOMs every window at native-alloc time.
const pictureCacheBudgetDivisor = 4

// minPictureCacheEntries is the floor for the fair-share entry cap: a
// window always keeps at least the historic default working set.
const minPictureCacheEntries = 64

// PictureCacheFairMax computes the per-window entry ceiling for the
// picture texture cache (R0-5). avgEntryBytes estimates one entry
// (pass 0 to use fallbackBytes); fallbackBytes estimates one entry when
// the cache is still empty (pass 0 for a 256×256 RGBA guess).
// Returns 0 when a single window (or none) is live, meaning "no cap —
// preserve historic single-window behavior exactly".
func PictureCacheFairMax(avgEntryBytes, fallbackBytes uint64) int {
	shareMu.Lock()
	windows := len(shareTargets)
	hasShare := shareDevice != nil
	shareMu.Unlock()
	if !hasShare || windows <= 1 {
		return 0
	}
	return pictureCacheFairMaxN(windows, webgpu.VramBudgetMB(), avgEntryBytes, fallbackBytes)
}

// pictureCacheFairMaxN is the pure fair-share core: budgetMB/(windows×
// divisor) bytes per window converted to entries. 0 budget disables.
func pictureCacheFairMaxN(windows int, budgetMB int64, avgEntryBytes, fallbackBytes uint64) int {
	if windows <= 1 || budgetMB <= 0 {
		return 0
	}
	perEntry := avgEntryBytes
	if perEntry == 0 {
		perEntry = fallbackBytes
	}
	if perEntry == 0 {
		perEntry = 256 * 256 * 4
	}
	fairBytes := uint64(budgetMB) * 1024 * 1024 / uint64(windows) / pictureCacheBudgetDivisor
	fair := int(fairBytes / perEntry)
	if fair < minPictureCacheEntries {
		fair = minPictureCacheEntries
	}
	return fair
}

// registerSharedTarget records a live target on the share.
// Caller must hold shareMu; bumps nothing (the refcount is owned by open).
func registerSharedTarget(t *PresentTarget) {
	if t == nil {
		return
	}
	if shareTargets == nil {
		shareTargets = make(map[*PresentTarget]struct{})
	}
	shareTargets[t] = struct{}{}
}

// unregisterSharedTarget removes a closing target from the share.
func unregisterSharedTarget(t *PresentTarget) {
	shareMu.Lock()
	defer shareMu.Unlock()
	delete(shareTargets, t)
}
