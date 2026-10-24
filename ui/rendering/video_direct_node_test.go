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

// The RGBA fallback shape still feeds without a device: the bridge keeps a
// CPU shadow and DrawCurrent paints it through the counted fallback. V-2:
// a rejected pool path must not drop the frame.
func TestRenderVideoUploadPlanes_RGBAShapeRoutes(t *testing.T) {
	br := render.NewVideoBridge(nil)
	defer br.Close()
	v := NewRenderVideo(64, 36, br)
	pix := make([]byte, 64*36*4)
	if !v.UploadPlanes(64, 36, nil, nil, pix) {
		t.Fatal("device-less RGBA upload = false, want true (CPU shadow staged)")
	}
	if st := br.Stats(); st.Frames != 1 || st.Uploads != 0 {
		t.Fatalf("staged stats=%+v, want frames=1 uploads=0", st)
	}
	dc := render.NewContext(64, 64)
	defer dc.Close()
	direct, ok := br.DrawCurrent(dc, render.VideoDrawOptions{X: 0, Y: 0, DstWidth: 32, DstHeight: 18})
	if !ok || direct {
		t.Fatalf("DrawCurrent direct=%v ok=%v, want false/true", direct, ok)
	}
	if st := br.Stats(); st.Fallbacks != 1 {
		t.Fatalf("stats=%+v, want fallbacks=1", st)
	}
	if got := dc.LastCPUFallbackReason(); got != "video:direct-fallback" {
		t.Fatalf("fallback reason=%q, want video:direct-fallback", got)
	}
}

// Odd-size NV12 must fall back to the RGBA pix at the node feed port instead
// of dropping the frame; with a row-pitch-rejecting bridge it reaches the
// screen through the counted CPU shadow.
func TestRenderVideoUploadPlanes_OddPlanesFallBackToRGBA(t *testing.T) {
	br := render.NewVideoBridge(nil)
	defer br.Close()
	v := NewRenderVideo(33, 17, br)
	y := make([]byte, 33*17)
	uv := make([]byte, 33*17/2)
	pix := make([]byte, 33*17*4)
	if !v.UploadPlanes(33, 17, y, uv, pix) {
		t.Fatal("odd-size planes + RGBA pix = false, want true (RGBA fallback)")
	}
	if v.NodeStats().PlanesLast {
		t.Fatal("odd-size planes reported the NV12 path, want RGBA fallback")
	}
	if st := br.Stats(); st.Frames != 1 || st.Uploads != 0 {
		t.Fatalf("stats=%+v, want frames=1 uploads=0 (33px row is not 256-aligned)", st)
	}
	dc := render.NewContext(64, 64)
	defer dc.Close()
	if direct, ok := br.DrawCurrent(dc, render.VideoDrawOptions{X: 0, Y: 0, DstWidth: 33, DstHeight: 17}); !ok || direct {
		t.Fatalf("DrawCurrent direct=%v ok=%v, want false/true", direct, ok)
	}
	if st := br.Stats(); st.Fallbacks != 1 {
		t.Fatalf("stats=%+v, want fallbacks=1", st)
	}
}
