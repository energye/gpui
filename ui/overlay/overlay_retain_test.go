package overlay_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
)

// TestOverlay_AttachToPacket_RetainsImages locks the D15 overlay half:
// images in the overlay band join the packet's retained set (attached
// post-Seal, pre-handoff — same lifetime as main-band images).
func TestOverlay_AttachToPacket_RetainsImages(t *testing.T) {
	main := rendering.NewRenderColorBox(100, 100, 0.5, 0.5, 0.5, 1)
	main.SetRepaintBoundary(true)
	pkt := rendering.BuildFramePacket(main, 1, 1, 100, 100)
	if len(pkt.RetainedImages) != 0 {
		t.Fatalf("imageless main retained=%d want 0", len(pkt.RetainedImages))
	}

	img, err := render.NewImageBuf(4, 4, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	ov := rendering.NewRenderImage(20, 20)
	ov.SetImage(img)
	st := overlay.New()
	st.Insert(overlay.NewEntry(ov, 0, 0, 20, 20))
	st.Layout(100, 100)
	st.AttachToPacket(pkt)

	if len(pkt.RetainedImages) != 1 || pkt.RetainedImages[0] != img {
		t.Fatalf("overlay image not retained: %v", pkt.RetainedImages)
	}
}
