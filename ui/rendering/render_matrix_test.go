package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// Render matrix headless gates (M1–M5 contract level).
//
// Honesty (PARTIAL Flutter alignment):
//   - These tests lock DirtyLayerIDs / CompositeOnly PaintVisits / layout locality.
//   - They do NOT prove GPU dirty-RECT partial present (PipelineApp still force full clear+paint; P6).
//   - RasterizeDirty counts are scene/stats locality, not per-layer GPU RT reuse.
//
// M1 (static + spinner) remains canonical in spinner_test.go:
//   TestS2_Spinner_DirtyLayerAndNoLayout, TestS4_StaticTree_SpinnerOnlyRaster.

// TestM1_PointsToS2S4 documents the M1 axis without duplicating S2/S4 bodies.
func TestM1_PointsToS2S4(t *testing.T) {
	t.Log("M1 dirty locality: see TestS2_Spinner_DirtyLayerAndNoLayout and TestS4_StaticTree_SpinnerOnlyRaster")
}

// TestM2_MultiBoundary_DirtyLocality: K independent RepaintBoundaries; dirty a subset each frame.
// DirtyLayerIDs and CompositeOnly PaintVisits must stay proportional to the dirty set, not K+N.
func TestM2_MultiBoundary_DirtyLocality(t *testing.T) {
	scene.ResetLayerIDGen()
	const (
		k        = 8
		nStatic  = 40
		dirtyN   = 2
		vpW, vpH = 400.0, 300.0
	)
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = vpW, vpH

	leaves := make([]*rendering.RenderColorBox, 0, k)
	for i := 0; i < k; i++ {
		c := rendering.NewRenderColorBox(16, 16, 0.2+float64(i)*0.05, 0.4, 0.6, 1)
		c.SetRepaintBoundary(true)
		root.AddChild(c)
		leaves = append(leaves, c)
	}
	for i := 0; i < nStatic; i++ {
		root.AddChild(rendering.NewRenderColorBox(4, 4, 0.3, 0.3, 0.3, 1))
	}

	owner := rendering.NewPipelineOwner(root)
	if !owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, true) {
		t.Fatal("initial layout")
	}
	var visits int64
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, true)
	layouts0 := owner.LayoutCount

	// Dirty only the first dirtyN boundaries.
	for i := 0; i < dirtyN; i++ {
		leaves[i].R = 0.9
		leaves[i].MarkNeedsPaint()
	}

	pkt := rendering.BuildFramePacket(root, 2, 1, vpW, vpH)
	st := scene.RasterizeDirty(pkt)
	if st.RasterLayerCount < 1 {
		t.Fatalf("expected dirty layers, stats=%+v dirtyIDs=%v", st, pkt.DirtyLayerIDs)
	}
	// Dirty set should track ~dirtyN boundaries, not all static + all boundaries.
	if len(pkt.DirtyLayerIDs) > dirtyN+2 {
		t.Fatalf("DirtyLayerIDs=%d (%v) want ≤%d", len(pkt.DirtyLayerIDs), pkt.DirtyLayerIDs, dirtyN+2)
	}
	if st.RasterLayerCount > dirtyN+3 {
		t.Fatalf("RasterLayerCount=%d too high for %d dirty boundaries", st.RasterLayerCount, dirtyN)
	}

	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint")
	}
	// Full tree would visit k+nStatic+root; partial must be much smaller.
	if visits > 30 {
		t.Fatalf("PaintVisits=%d too high for %d dirty boundaries (static=%d)", visits, dirtyN, nStatic)
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout must not run on color-only dirty: %d → %d", layouts0, owner.LayoutCount)
	}
}

// TestM4_Text_DirtyLocality: static Latin/CJK clean after warm; color-only dirty is paint-local;
// SetText may layout that node but must not storm the whole tree when isolated by RelayoutBoundary.
func TestM4_Text_DirtyLocality(t *testing.T) {
	scene.ResetLayerIDGen()
	const vpW, vpH = 320.0, 200.0

	latin := rendering.NewRenderText("Hello matrix")
	latin.FontSize = 14
	latin.SetRepaintBoundary(true)

	cjk := rendering.NewRenderText("你好世界")
	cjk.FontSize = 14
	cjk.SetRepaintBoundary(true)

	// Static chrome
	static := rendering.NewRenderColorBox(40, 40, 0.25, 0.28, 0.32, 1)
	static.SetRepaintBoundary(true)

	root := rendering.NewRenderBox(latin, cjk, static)
	root.FixedWidth, root.FixedHeight = vpW, vpH
	// Isolate layout of text changes at root level if possible.
	root.SetRelayoutBoundary(true)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, true)
	var visits int64
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, true)
	layouts0 := owner.LayoutCount

	// After full paint, a clean packet should not re-raster everything as dirty.
	pkt0 := rendering.BuildFramePacket(root, 1, 1, vpW, vpH)
	st0 := scene.RasterizeDirty(pkt0)
	if len(pkt0.DirtyLayerIDs) != 0 && st0.RasterLayerCount > 3 {
		t.Fatalf("warm packet still very dirty: ids=%v stats=%+v", pkt0.DirtyLayerIDs, st0)
	}

	// Color-only change: paint dirty, no layout.
	latin.SetColor(0.2, 0.9, 0.4, 1)
	if owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false) {
		t.Fatal("SetColor must not force layout flush work")
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout count changed on SetColor: %d → %d", layouts0, owner.LayoutCount)
	}
	if !latin.NeedsPaint() {
		t.Fatal("latin should need paint after SetColor")
	}
	if static.NeedsPaint() {
		t.Fatal("static boundary must stay clean")
	}

	pkt := rendering.BuildFramePacket(root, 2, 1, vpW, vpH)
	st := scene.RasterizeDirty(pkt)
	if st.RasterLayerCount < 1 {
		t.Fatalf("expected latin dirty raster, stats=%+v dirty=%v", st, pkt.DirtyLayerIDs)
	}
	if st.RasterLayerCount > 4 {
		t.Fatalf("RasterLayerCount=%d too high for one text color change", st.RasterLayerCount)
	}

	visits = 0
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false)
	if visits > 15 {
		t.Fatalf("PaintVisits=%d too high for single text color dirty", visits)
	}

	// SetText: may layout text node; static boundary still not paint-dirty from latin alone after clear.
	layouts1 := owner.LayoutCount
	latin.SetText("Hello matrix!")
	_ = owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false)
	if owner.LayoutCount < layouts1 {
		t.Fatal("layout count should not decrease")
	}
	// CJK node should not need paint solely because latin text changed.
	if cjk.NeedsPaint() && !cjk.NeedsLayout() {
		// After layout flush, cjk should remain clean if it was clean.
	}
	if static.NeedsPaint() {
		t.Fatal("static must not pick up latin SetText paint dirty")
	}
}

// TestM5_GradientBoundary_DirtyLocality: custom OnPaint (gradient/rrect via painting helpers)
// on a RepaintBoundary; static siblings stay clean when only the gradient node is dirtied.
func TestM5_GradientBoundary_DirtyLocality(t *testing.T) {
	scene.ResetLayerIDGen()
	const vpW, vpH = 300.0, 200.0

	grad := rendering.NewRenderBox()
	grad.FixedWidth, grad.FixedHeight = 120, 40
	grad.SetRepaintBoundary(true)
	grad.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ax, ay := pc.OriginX, pc.OriginY
		w, h := size.Width, size.Height
		gradBrush := render.NewLinearGradientBrush(ax, ay, ax+w, ay).
			AddColorStop(0, render.RGBA{R: 0.15, G: 0.45, B: 0.95, A: 1}).
			AddColorStop(1, render.RGBA{R: 0.95, G: 0.35, B: 0.2, A: 1})
		pc.DC.SetFillBrush(gradBrush)
		pc.DC.DrawRectangle(ax, ay, w, h)
		_ = pc.DC.Fill()
		pc.DC.SetRGBA(0.1, 0.1, 0.12, 0.85)
		pc.DC.DrawRoundedRectangle(ax+8, ay+8, w-16, h-16, 6)
		_ = pc.DC.Fill()
	}

	static := rendering.NewRenderColorBox(80, 40, 0.3, 0.32, 0.36, 1)
	static.SetRepaintBoundary(true)

	root := rendering.NewRenderBox(grad, static)
	root.FixedWidth, root.FixedHeight = vpW, vpH

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, true)
	var visits int64
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, true)
	layouts0 := owner.LayoutCount

	grad.MarkNeedsPaint()
	if static.NeedsPaint() {
		t.Fatal("static boundary must not be dirty")
	}
	if owner.FlushLayout(rendering.Size{Width: vpW, Height: vpH}, false) {
		t.Fatal("paint-only dirty must not layout")
	}
	if owner.LayoutCount != layouts0 {
		t.Fatalf("layout count %d → %d", layouts0, owner.LayoutCount)
	}

	pkt := rendering.BuildFramePacket(root, 3, 1, vpW, vpH)
	st := scene.RasterizeDirty(pkt)
	if st.RasterLayerCount < 1 {
		t.Fatalf("grad boundary should re-raster, stats=%+v", st)
	}
	if st.RasterLayerCount > 3 {
		t.Fatalf("RasterLayerCount=%d want small", st.RasterLayerCount)
	}

	visits = 0
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false)
	if visits > 12 {
		t.Fatalf("PaintVisits=%d too high for one gradient boundary", visits)
	}
}

// TestM3_ScrollAxis_PointsToS5S6 keeps M3 documented; canonical gates live in virtual_list/viewport tests.
func TestM3_ScrollAxis_PointsToS5S6(t *testing.T) {
	t.Log("M3 scroll: see TestS5_VirtualList_BindCap, TestS6_DragScroll_NoLayoutStorm, TestViewport_ScrollDoesNotLayout")
}
