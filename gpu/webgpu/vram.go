//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package webgpu

import (
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// VramLiveBytes reports the process VRAM ledger total (same account the
// texture/buffer/pipeline/sampler/surface gate enforces). Re-exported so
// render (adapter watermarks) reads one number without importing rwgpu.
func VramLiveBytes() uint64 {
	return rwgpu.VramLiveBytes()
}

// VramBudgetMB exposes the process VRAM budget (GPUI_VRAM_BUDGET_MB,
// default 768, 0 = disabled) from the same source the gate enforces.
func VramBudgetMB() int64 {
	return rwgpu.VramBudgetMB()
}

// VramLiveCount reports how many allocations the ledger tracks.
func VramLiveCount() int {
	return rwgpu.VramLiveCount()
}

// VramPeakBytes reports the high-water mark.
func VramPeakBytes() uint64 {
	return rwgpu.VramPeakBytes()
}

func vramTestResetImpl() {
	rwgpu.VramTestReset()
}

func vramTestAddImpl(handle uintptr, need uint64) {
	rwgpu.VramTestAdd(handle, need)
}
