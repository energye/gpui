package overlay_test

import (
	"testing"

	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
)

// TestOverlay_AttachToPacket_SecondAttachRefused locks R2-3: the overlay
// tail runs exactly once — a second attach is refused instead of silently
// duplicating overlay dirties.
func TestOverlay_AttachToPacket_SecondAttachRefused(t *testing.T) {
	main := rendering.NewRenderColorBox(100, 100, 0.5, 0.5, 0.5, 1)
	main.SetRepaintBoundary(true)
	pkt := rendering.BuildFramePacket(main, 1, 1, 100, 100)

	st := overlay.New()
	ov := rendering.NewRenderColorBox(20, 20, 1, 0, 0, 1)
	ov.SetRepaintBoundary(true)
	st.Insert(overlay.NewEntry(ov, 0, 0, 20, 20))
	st.Layout(100, 100)
	if !st.AttachToPacket(pkt) {
		t.Fatal("first attach must succeed")
	}
	n := len(pkt.DirtyLayerIDs)
	if st.AttachToPacket(pkt) {
		t.Fatal("second attach must be refused")
	}
	if len(pkt.DirtyLayerIDs) != n {
		t.Fatal("refused attach must not duplicate dirties")
	}
	if !pkt.OverlaySealed {
		t.Fatal("attached packet must carry the tail flag")
	}
}
