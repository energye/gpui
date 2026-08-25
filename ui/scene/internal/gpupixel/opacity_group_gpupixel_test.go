// GPU pixel contract tests for group opacity (C5 engine hole).
//
// Reproduces the two paths an α<1 RenderOpacity subtree can take:
//   - immediate paint (paintPresentTree / RenderOpacity.Paint):
//     render.Context.PushLayerIsolated → offscreen RT → PopLayer composite;
//   - retained composite (CompositeFramePacketTextured):
//     scene.OpacityLayer → dc.PushLayer(Normal, α) F1 fast path → blit.
//
// The C5 window loses BOTH cards' pixels while R6 (one card) renders fine,
// so these tests exercise one and two stacked groups per frame.
package gpupixel

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/scene"
)

// backdrop cell blue and float-card orange from ui_wr_c5_anim_over_static.
var (
	c5CellBG   = [3]float64{0.37 * (1 - 0.08*3), 0.50 * (0.8 + 0.1*1), 0.72} // cell(3,1)
	c5Orange   = [3]float64{0.90, 0.45, 0.15}
	c5CardAlpha = 0.75
)

func wantBlend() [3]uint8 {
	var out [3]uint8
	for i := range out {
		v := c5CardAlpha*c5Orange[i] + (1-c5CardAlpha)*c5CellBG[i]
		out[i] = uint8(v*255 + 0.5)
	}
	return out
}

func sampleCenter(t *testing.T, dc *render.Context, view render.TextureView, W, H, x, y int) (uint8, uint8, uint8) {
	t.Helper()
	out := render.NewContext(W, H)
	defer out.Close()
	out.ClearWithColor(render.Black)
	out.DrawGPUTexture(view, 0, 0, W, H)
	if err := out.FlushGPU(); err != nil {
		t.Fatalf("composite for readback: %v", err)
	}
	img := out.Image()
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func near(a, w uint8) bool {
	d := int(a) - int(w)
	if d < 0 {
		d = -d
	}
	return d <= 12
}

// TestImmediate_IsolatedLayer_OneCard: PushLayerIsolated group opacity must
// composite the child draw over the backdrop (RenderOpacity.Paint contract).
func TestImmediate_IsolatedLayer_OneCard(t *testing.T) {
	requireGPU(t)
	const W, H = 200, 120
	dc := render.NewContext(W, H)
	defer dc.Close()

	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	dc.BeginFrame()
	// backdrop: solid cell-blue panel
	dc.SetRGB(c5CellBG[0], c5CellBG[1], c5CellBG[2])
	dc.DrawRectangle(0, 0, W, H)
	_ = dc.Fill()
	// opacity card (RenderOpacity.Paint → SaveLayer → PushLayerIsolated)
	dc.PushLayerIsolated(c5CardAlpha)
	dc.SetRGB(c5Orange[0], c5Orange[1], c5Orange[2])
	dc.DrawRectangle(20, 20, 130, 80)
	_ = dc.Fill()
	dc.PopLayer()
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("flush: %v", err)
	}

	r, g, b := sampleCenter(t, dc, view, W, H, 85, 60)
	w := wantBlend()
	if !near(r, w[0]) || !near(g, w[1]) || !near(b, w[2]) {
		t.Fatalf("isolated-layer card lost: got rgb=%d,%d,%d want %d,%d,%d", r, g, b, w[0], w[1], w[2])
	}
}

// TestImmediate_IsolatedLayer_TwoCards: two sequential isolated groups in one
// frame (C5 has OPA + FLOAT). The second card's PopLayer mid-frame flush must
// not discard either group's content.
func TestImmediate_IsolatedLayer_TwoCards(t *testing.T) {
	requireGPU(t)
	const W, H = 200, 120
	dc := render.NewContext(W, H)
	defer dc.Close()

	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	dc.BeginFrame()
	dc.SetRGB(c5CellBG[0], c5CellBG[1], c5CellBG[2])
	dc.DrawRectangle(0, 0, W, H)
	_ = dc.Fill()

	for _, x := range []int{10, 100} {
		dc.PushLayerIsolated(c5CardAlpha)
		dc.SetRGB(c5Orange[0], c5Orange[1], c5Orange[2])
		dc.DrawRectangle(float64(x), 20, 80, 80)
		_ = dc.Fill()
		dc.PopLayer()
	}
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("flush: %v", err)
	}

	w := wantBlend()
	for _, x := range []int{50, 140} {
		r, g, b := sampleCenter(t, dc, view, W, H, x, 60)
		if !near(r, w[0]) || !near(g, w[1]) || !near(b, w[2]) {
			t.Fatalf("card@%d lost: got rgb=%d,%d,%d want %d,%d,%d", x, r, g, b, w[0], w[1], w[2])
		}
	}
}

// TestRetained_OpacityBoundaryGroupPixels: retained path — Boundary >
// Opacity(0.75) > Picture over a static backdrop picture. Mirrors the C5
// floatHost structure; steady frame must blit the card through the F1 group
// with correct group alpha.
func TestRetained_OpacityBoundaryGroupPixels(t *testing.T) {
	requireGPU(t)
	const W, H = 320, 200
	dc := render.NewContext(W, H)
	defer dc.Close()

	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushOffset(0, 0)
	// backdrop picture: cell-blue panel
	bp := b.AddPicture(false)
	bp.SetCacheKey(9100)
	bp.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, W, H, c5CellBG[0], c5CellBG[1], c5CellBG[2], 1)
	})
	// boundary > opacity > card picture (floatHost shape)
	b.PushBoundary(170, 40, "opacity", false)
	b.PushOpacity(c5CardAlpha)
	cp := b.AddPicture(true)
	cp.SetCacheKey(9101)
	cp.Record(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 130, 80, c5Orange[0], c5Orange[1], c5Orange[2], 1)
	})
	_ = cp
	b.Pop() // opacity
	b.Pop() // boundary
	b.Pop() // root offset
	pkt := b.BuildPacket(1, 1, W, H)

	tex := scene.NewPictureTextureCache(dc, 8)
	for i := 0; i < 3; i++ { // warm-up presents like the real window
		dc.SetRGB(0.08, 0.09, 0.11)
		dc.DrawRectangle(0, 0, W, H)
		_ = dc.Fill()
		if err := dc.PresentFrame(view, W, H, func() error { return nil }); err != nil {
			t.Fatalf("warm-up present %d: %v", i, err)
		}
	}

	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if !tex.Has(9101) {
		t.Fatalf("card picture not cached")
	}
	if st.ReplayedOps != 0 {
		t.Fatalf("frame replayed %d ops want 0 (blit path)", st.ReplayedOps)
	}
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("flush: %v", err)
	}

	r, g, bb := sampleCenter(t, dc, view, W, H, 235, 80) // card center
	w := wantBlend()
	if !near(r, w[0]) || !near(g, w[1]) || !near(bb, w[2]) {
		t.Fatalf("retained opacity-group card lost: got rgb=%d,%d,%d want %d,%d,%d",
			r, g, bb, w[0], w[1], w[2])
	}
}
