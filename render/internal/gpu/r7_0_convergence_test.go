//go:build !nogpu

package gpu

import (
	"testing"
)

// TestR70_DualTexSlotScratchDocuments documents R7.0 dual-tex uniform slot
// reuse: per-run one 256-byte CPU scratch instead of per-op make. Behavioral
// coverage lives in TestP03_* / F1 advanced layer present tests.
func TestR70_DualTexSlotScratchDocuments(t *testing.T) {
	t.Log("R7.0 dual-tex into-dest reuses one uniStride slot per blend run; see dual_tex_blend.go")
}
