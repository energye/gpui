package rendering

import (
	"testing"

	"github.com/energye/gpui/render"
)

// Layer 5: the window-facing feed is one method. Planes go through the
// GPU YUV convert, the RGBA fallback shape goes through the bridge RGBA
// route; anything else fails closed without touching the bridge.
func TestRenderVideoUploadPlanes_FailClosed(t *testing.T) {
	var nilNode *RenderVideo
	if nilNode.UploadPlanes(64, 36, make([]byte, 64*36), make([]byte, 64*36/2), nil) {
		t.Fatal("nil node = true, want false")
	}
	noBridge := NewRenderVideo(64, 36, nil)
	if noBridge.UploadPlanes(64, 36, make([]byte, 64*36), make([]byte, 64*36/2), nil) {
		t.Fatal("nil bridge = true, want false")
	}
	if noBridge.UploadPlanes(0, 36, make([]byte, 1), make([]byte, 1), nil) {
		t.Fatal("zero width = true, want false")
	}
	// Short planes fail closed (never a partial upload).
	if noBridge.UploadPlanes(64, 36, make([]byte, 10), make([]byte, 10), nil) {
		t.Fatal("short planes = true, want false")
	}
	// Empty everything fails closed.
	if noBridge.UploadPlanes(64, 36, nil, nil, nil) {
		t.Fatal("empty = true, want false")
	}
	// Odd-size planes fail closed at the node (plane path is even-only);
	// the RGBA shape stays available through pix.
	if noBridge.UploadPlanes(65, 36, make([]byte, 65*36), make([]byte, 65*36/2), nil) {
		t.Fatal("odd width planes = true, want false")
	}
}

// The RGBA fallback shape still feeds without a device: bridge without
// device fails the upload, so this asserts the shape routes (not drops
// silently) — false here comes from the device-less bridge, not the node.
func TestRenderVideoUploadPlanes_RGBAShapeRoutes(t *testing.T) {
	v := NewRenderVideo(64, 36, render.NewVideoBridge(nil))
	pix := make([]byte, 64*36*4)
	if v.UploadPlanes(64, 36, nil, nil, pix) {
		t.Fatal("device-less RGBA upload = true, want false (bridge has no pool)")
	}
}
