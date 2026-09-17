package render

import (
	"testing"
)

// resetVramLedgerForTest seeds the process VRAM ledger with one synthetic
// live entry of liveBytes (or empty for 0) so waterline tests are pure Go.
// Restores the empty ledger on cleanup — never leaks into other tests.
func resetVramLedgerForTest(t *testing.T, liveBytes uint64) {
	t.Helper()
	vramTestReset()
	if liveBytes > 0 {
		vramTestAdd(0xC0FFEE, liveBytes)
	}
	t.Cleanup(func() {
		vramTestReset()
	})
}
