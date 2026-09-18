package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// TestBuildPacket_BareEntrySeals locks R2-2: every packet leaves BuildPacket
// sealed — the bare scene entry seals too, so the raster-side Sealed check
// holds for test-built packets as well as production (wrapper) ones.
func TestBuildPacket_BareEntrySeals(t *testing.T) {
	root := rendering.NewRenderColorBox(64, 64, 1, 0, 0, 1)
	b := rendering.BuildLayerTree(root)
	pkt := b.BuildPacket(1, 1, 64, 64)
	if pkt == nil || !pkt.IsSealed() {
		t.Fatal("bare BuildPacket must hand off sealed")
	}
}
