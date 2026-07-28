package embedder_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
)

// TestRetainedCompositeOnly_SteadyDamageSmall: W2 path — after full warm paint,
// only hot dirty under CompositeOnly must produce damage ≪ surface (static kept
// by not redrawing, LoadOpLoad on real Present).
func TestRetainedCompositeOnly_SteadyDamageSmall(t *testing.T) {
	const W, H = 400, 200
	static := rendering.NewRenderColorBox(80, 80, 0.1, 0.6, 0.2, 1)
	static.SetRepaintBoundary(true)
	hot := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	hot.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(W, H)
	root.Background = &rendering.Color{R: 0.05, G: 0.05, B: 0.08, A: 1}
	root.Place(static, 20, 20)
	root.Place(hot, 300, 120)
	pipe := rendering.NewPipelineOwner(root)
	pipe.FlushLayout(rendering.Size{Width: W, Height: H}, true)

	dc0 := render.NewContext(W, H)
	defer dc0.Close()
	dc0.BeginFrame()
	embedder.PaintPresentTree(dc0, pipe, root, nil, 0.05, 0.05, 0.08, 1, true)

	hot.MarkNeedsPaint()
	if static.NeedsPaint() {
		t.Fatal("static must stay clean")
	}

	dc := render.NewContext(W, H)
	defer dc.Close()
	dc.BeginFrame()
	// Simulate prior frame content still on surface (LoadOpLoad world).
	embedder.PaintPresentTree(dc, pipe, root, nil, 0.05, 0.05, 0.08, 1, true)
	// Clear dirty from full paint, re-dirty only hot.
	if static.NeedsPaint() || hot.NeedsPaint() {
		pipe.FlushPaint(&rendering.PaintContext{}, true)
	}
	hot.R = 0.9
	hot.MarkNeedsPaint()

	dc2 := render.NewContext(W, H)
	defer dc2.Close()
	dc2.BeginFrame()
	// Copy prior pixels conceptually: paint full once into dc2 then composite-only.
	embedder.PaintPresentTree(dc2, pipe, root, nil, 0.05, 0.05, 0.08, 1, true)
	pipe.FlushPaint(&rendering.PaintContext{}, true)
	hot.MarkNeedsPaint()
	dc2.BeginFrame() // new frame damage
	embedder.PaintPresentTreeCompositeOnly(dc2, pipe, root, nil, 0.05, 0.05, 0.08, 1, false)

	area := embedder.DamageAreaLogical(dc2)
	surf := embedder.SurfaceAreaLogical(dc2)
	if area <= 0 {
		t.Fatal("expected some damage from hot")
	}
	ratio := float64(area) / float64(surf)
	if ratio >= 0.5 {
		t.Fatalf("damage_ratio=%.3f area=%d surf=%d want ≪0.5 under retained CompositeOnly", ratio, area, surf)
	}
}

func TestPipelineApp_SetPresentPolicyRetained(t *testing.T) {
	// Host-less: only policy flag + metrics wiring (no Open).
	root := rendering.NewAbsoluteBox(10, 10)
	app := embedder.NewPipelineApp(nil, root, embedder.PipelineOptions{})
	if app.Metrics().PresentPolicy() != scheduler.PresentPolicyFullPaint {
		t.Fatalf("default policy=%q", app.Metrics().PresentPolicy())
	}
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)
	if app.Metrics().PresentPolicy() != scheduler.PresentPolicyRetained {
		t.Fatalf("after set policy=%q", app.Metrics().PresentPolicy())
	}
}
