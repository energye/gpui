package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// TestSaveLayer_BudgetRejectsExcessOps: LayerBudget.MaxOps gates SaveLayer.
func TestSaveLayer_BudgetRejectsExcessOps(t *testing.T) {
	dc := render.NewContext(40, 40)
	defer dc.Close()
	dc.BeginFrame()

	pc := rendering.NewPaintContext(dc, 1)
	pc.LayerBudget = &rendering.SaveLayerBudget{MaxOps: 1, MaxArea: 1e9}

	if !pc.SaveLayer(20, 20, 1) {
		t.Fatal("first SaveLayer should be allowed")
	}
	if pc.SaveLayerDepth() != 1 {
		t.Fatalf("depth=%d want 1", pc.SaveLayerDepth())
	}
	if pc.SaveLayer(20, 20, 1) {
		t.Fatal("second SaveLayer must be rejected by MaxOps=1")
	}
	if pc.SaveLayerDepth() != 1 {
		t.Fatal("rejected SaveLayer must not increase depth")
	}
	pc.Restore()
	if pc.SaveLayerDepth() != 0 {
		t.Fatalf("after Restore depth=%d", pc.SaveLayerDepth())
	}
}

// TestSaveLayer_BudgetRejectsArea: MaxArea blocks large layers.
func TestSaveLayer_BudgetRejectsArea(t *testing.T) {
	dc := render.NewContext(40, 40)
	defer dc.Close()
	pc := rendering.NewPaintContext(dc, 1)
	pc.LayerBudget = &rendering.SaveLayerBudget{MaxOps: 10, MaxArea: 100} // 10×10

	if pc.SaveLayer(20, 20, 1) { // area 400
		t.Fatal("SaveLayer area 400 must exceed MaxArea 100")
	}
	if !pc.SaveLayer(5, 5, 1) { // area 25
		t.Fatal("small SaveLayer should pass")
	}
	pc.Restore()
}

// TestSaveLayer_OpacityCompositesGroup: isolated layer at opacity 0.5 over white
// with solid red fill yields a blended (not full) red.
func TestSaveLayer_OpacityCompositesGroup(t *testing.T) {
	dc := render.NewContext(60, 60)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1)
	if !pc.SaveLayer(60, 60, 0.5) {
		t.Fatal("SaveLayer failed")
	}
	rendering.FillRect(pc, 10, 10, 40, 40, 1, 0, 0, 1)
	pc.Restore()

	img := dc.Image()
	rr, gg, bb, _ := img.At(30, 30).RGBA()
	if rr < 0x6000 {
		t.Fatalf("expected red tint #%04x%04x%04x", rr, gg, bb)
	}
	// Not fully opaque red (would be ~ffff,0000,0000).
	if rr > 0xF000 && gg < 0x2000 && bb < 0x2000 {
		t.Fatalf("looks fully opaque — isolation opacity ineffective #%04x%04x%04x", rr, gg, bb)
	}
	if pc.SaveLayerDepth() != 0 {
		t.Fatal("depth leak after Restore")
	}
}

// TestSaveLayer_RestoreNoOpWhenEmpty: extra Restore is safe.
func TestSaveLayer_RestoreNoOpWhenEmpty(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	pc := rendering.NewPaintContext(dc, 1)
	pc.Restore()
	pc.Restore()
	if pc.SaveLayerDepth() != 0 {
		t.Fatal("depth should stay 0")
	}
}

// TestPushClipPath_TriangleClipsFill: triangular clip leaves outside white.
func TestPushClipPath_TriangleClipsFill(t *testing.T) {
	const W, H = 80, 80
	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	pc := rendering.NewPaintContext(dc, 1).WithOrigin(10, 10)
	p := rendering.NewPath()
	// Local triangle covering lower-right of a 50×50 box.
	p.MoveTo(0, 50)
	p.LineTo(50, 50)
	p.LineTo(50, 0)
	p.Close()

	pc.PushClipPath(p)
	rendering.FillRect(pc, 0, 0, 50, 50, 0, 0, 1, 1) // would be blue full box
	pc.PopClip()

	img := dc.Image()
	// Near local (45,45) → abs (55,55) should be inside triangle → blue.
	br, bg, bb, _ := img.At(55, 55).RGBA()
	if bb < 0xA000 {
		t.Fatalf("inside triangle (55,55)=#%04x%04x%04x want blue", br, bg, bb)
	}
	// Near local (5,5) → abs (15,15) outside triangle (top-left) → white.
	or, og, ob, _ := img.At(15, 15).RGBA()
	if or < 0xC000 || og < 0xC000 || ob < 0xC000 {
		t.Fatalf("outside triangle (15,15)=#%04x%04x%04x want white", or, og, ob)
	}
}

// TestSaveLayer_NestedDepth: nested SaveLayer/Restore pairs.
func TestSaveLayer_NestedDepth(t *testing.T) {
	dc := render.NewContext(40, 40)
	defer dc.Close()
	pc := rendering.NewPaintContext(dc, 1)
	if !pc.SaveLayer(40, 40, 1) || !pc.SaveLayer(20, 20, 0.8) {
		t.Fatal("nested SaveLayer failed")
	}
	if pc.SaveLayerDepth() != 2 {
		t.Fatalf("depth=%d want 2", pc.SaveLayerDepth())
	}
	pc.Restore()
	if pc.SaveLayerDepth() != 1 {
		t.Fatalf("depth=%d want 1", pc.SaveLayerDepth())
	}
	pc.Restore()
	if pc.SaveLayerDepth() != 0 {
		t.Fatal("depth leak")
	}
}
