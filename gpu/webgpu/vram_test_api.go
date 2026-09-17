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
