package scene_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	"github.com/energye/gpui/ui/scene"
)

// TestComposite_RejectsUnsealed locks R2-1: the raster handoff is sealed
// packets only — a hand-rolled unsealed packet is refused with zero counts,
// never rasterized half-built.
func TestComposite_RejectsUnsealed(t *testing.T) {
	pkt := &scene.FramePacket{FrameID: 1} // deliberately unsealed
	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	tex := scene.NewPictureTextureCache(dc, 0)

	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if !st.RejectedUnsealed {
		t.Fatal("unsealed packet must be refused")
	}
	if st.RasterLayerCount != 0 || st.SkippedLayerCount != 0 || st.ReplayedOps != 0 {
		t.Fatalf("refused composite must have zero counts: %+v", st)
	}
}
