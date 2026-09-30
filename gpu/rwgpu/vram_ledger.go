//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rwgpu

import (
	"github.com/energye/gpui/gpu/hal"
)

// Thin re-exports over the shared hal ledger (gpu/hal/vram_ledger.go).
// The ledger moved to hal so both backends
// charge one process account; these wrappers keep existing callers and
// tests compiling unchanged. New code calls hal directly.

// VramLiveBytes reports the ledger total for diagnostics and tests.
func VramLiveBytes() uint64 {
	return hal.VramLiveBytes()
}

// VramLiveCount reports how many allocations the ledger tracks.
func VramLiveCount() int {
	return hal.VramLiveCount()
}

// VramPeakBytes reports the high-water mark since process start (or the
// last VramTestReset), for post-fix measurement via WR_MEMDIG.
func VramPeakBytes() uint64 {
	return hal.VramPeakBytes()
}

// VramBudgetMB exposes the process VRAM budget (GPUI_VRAM_BUDGET_MB,
// default 768, 0 = disabled) so upper layers can derive watermarks from the
// same number the gate enforces — one budget, no second constant.
func VramBudgetMB() int64 {
	return hal.VramBudgetMB()
}

// VramTestReset clears the ledger. Test-only: lets render waterline tests
// seed exact live totals without a GPU.
func VramTestReset() {
	hal.VramTestReset()
}

// VramTestAdd records a synthetic live entry. Test-only companion to
// VramTestReset (bypasses the cpu-mode skip so tests are hermetic).
func VramTestAdd(handle uintptr, need uint64) {
	hal.VramTestAdd(handle, need)
}
