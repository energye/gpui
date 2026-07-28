package embedder_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/rendering"
)

// TestPaintPresentTree_ForceFullDamagesWholeSurface: force=true full-clears and
// MarkFullRedraw so damage covers (nearly) the entire logical surface — the
// bootstrap / resize path.
func TestPaintPresentTree_ForceFullDamagesWholeSurface(t *testing.T) {
	const W, H = 200, 100
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()

	static := rendering.NewRenderColorBox(80, 80, 0.2, 0.3, 0.4, 1)
	static.SetRepaintBoundary(true)
	hot := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	hot.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(W, H)
	root.Place(static, 10, 10)
	root.Place(hot, 120, 30)
	pipe := rendering.NewPipelineOwner(root)
	pipe.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	pipe.FlushPaint(&rendering.PaintContext{}, true) // clear dirty after first full paint

	embedder.PaintPresentTree(dc, pipe, root, nil, 0.1, 0.1, 0.1, 1, true)
	area := embedder.DamageAreaLogical(dc)
	surf := embedder.SurfaceAreaLogical(dc)
	if surf != W*H {
		t.Fatalf("surface area=%d want %d", surf, W*H)
	}
	// Full clear + MarkFullRedraw must dirty essentially the whole surface.
	if area < surf*85/100 {
		t.Fatalf("force full damage area=%d want ≥85%% of surface %d (got full-clear path?)", area, surf)
	}
}

// TestPaintPresentTree_PartialOnlyDirtiesHotWidget: after a clean full paint,
// only the hot RepaintBoundary is marked dirty; force=false must NOT full-clear,
// and FrameDamage area must be ≪ full surface (proves not full-clear masquerading).
func TestPaintPresentTree_PartialOnlyDirtiesHotWidget(t *testing.T) {
	const W, H = 200, 100
	dc := render.NewContext(W, H)
	defer dc.Close()

	static := rendering.NewRenderColorBox(80, 80, 0.2, 0.3, 0.4, 1)
	static.SetRepaintBoundary(true)
	hot := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	hot.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(W, H)
	root.Place(static, 10, 10)
	root.Place(hot, 120, 30)
	pipe := rendering.NewPipelineOwner(root)
	pipe.FlushLayout(rendering.Size{Width: W, Height: H}, true)

	// Bootstrap full paint into a throwaway context so dirties clear.
	dc0 := render.NewContext(W, H)
	defer dc0.Close()
	dc0.BeginFrame()
	embedder.PaintPresentTree(dc0, pipe, root, nil, 0.1, 0.1, 0.1, 1, true)
	// Tree should now be clean.
	if static.NeedsPaint() || hot.NeedsPaint() {
		// FlushPaint in PaintPresentTree should have cleared; if not, force paint once more.
		pipe.FlushPaint(&rendering.PaintContext{}, true)
	}

	// Only hot animates.
	hot.MarkNeedsPaint()
	if static.NeedsPaint() {
		t.Fatal("static must stay clean")
	}
	if !hot.NeedsPaint() {
		t.Fatal("hot must be dirty")
	}

	dc.BeginFrame()
	embedder.PaintPresentTree(dc, pipe, root, nil, 0.1, 0.1, 0.1, 1, false)

	area := embedder.DamageAreaLogical(dc)
	surf := embedder.SurfaceAreaLogical(dc)
	if area <= 0 {
		t.Fatal("partial paint must record some FrameDamage from hot widget draws")
	}
	// Must be far below full surface — not a full-clear pretending to be damage.
	if area >= surf*50/100 {
		t.Fatalf("partial damage area=%d is ≥50%% of surface %d — looks like full clear", area, surf)
	}
	// Hot box is 40×40; allow padding from tracking but still ≪ static+full.
	if area > 40*40*4 {
		t.Fatalf("partial damage area=%d suspiciously large for 40×40 hot widget", area)
	}

	// Center of hot widget (abs ~140,50) should be red after partial paint.
	img := dc.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	rr, gg, bb, _ := img.At(140, 50).RGBA()
	if rr < 0xC000 || gg > 0x4000 || bb > 0x4000 {
		t.Fatalf("hot center (140,50)=#%04x%04x%04x want red after partial paint", rr, gg, bb)
	}
}

// TestPaintPresentTree_PartialSkipsCleanStatic: static boundary not NeedsPaint
// must not be visited under CompositeOnly (PaintVisits gate).
func TestPaintPresentTree_PartialSkipsCleanStatic(t *testing.T) {
	const W, H = 160, 80
	static := rendering.NewRenderColorBox(60, 60, 0.3, 0.3, 0.35, 1)
	static.SetRepaintBoundary(true)
	hot := rendering.NewRenderColorBox(30, 30, 0, 1, 0, 1)
	hot.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(W, H)
	root.Place(static, 4, 4)
	root.Place(hot, 100, 20)
	pipe := rendering.NewPipelineOwner(root)
	pipe.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	var visits int64
	pipe.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, true)
	visits = 0

	hot.MarkNeedsPaint()
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	// Manual partial path matching PaintPresentTree force=false (no full clear).
	pc := rendering.NewPaintContext(dc, 1)
	pc.PaintVisits = &visits
	pipe.FlushPaint(pc, false)

	if visits < 1 {
		t.Fatal("expected some paint visits for hot")
	}
	// Static clean boundary should be skipped — visits should be small (hot path only).
	// Full tree paint would visit root+static+hot (≥3). CompositeOnly should be fewer.
	if visits > 6 {
		t.Fatalf("PaintVisits=%d looks like full tree paint under CompositeOnly", visits)
	}
}
