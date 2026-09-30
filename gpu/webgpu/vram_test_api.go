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

// VramTestReset clears the process VRAM ledger. Test-only forwarder so
// render waterline tests can seed exact live totals without a GPU.
func VramTestReset() {
	vramTestResetImpl()
}

// VramTestAdd records a synthetic live entry. Test-only companion to
// VramTestReset (bypasses the cpu-mode skip so tests are hermetic).
func VramTestAdd(handle uintptr, need uint64) {
	vramTestAddImpl(handle, need)
}
