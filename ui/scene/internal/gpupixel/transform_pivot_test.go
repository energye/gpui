package gpupixel

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// TestTransformPivot_CenterMatchesVectorPaint pins the C8 v2.4 fix: the
// retained composite path must rotate around the subtree CENTER (same as
// RenderTransform.Paint's vector path), not the layer origin. Regression:
// steady frames spun around the top-left corner while post-resize vector
// repaints spun around the center — two different visual results for the
// same tree.
func TestTransformPivot_CenterMatchesVectorPaint(t *testing.T) {
	requireGPU(t)
	const W, H = 100, 100

	build := func() *rendering.RenderColorBox {
		inner := rendering.NewRenderColorBox(40, 40, 0.85, 0.30, 0.28, 1) // red
		tr := rendering.NewRenderTransform(inner)
		tr.SetRotation(math.Pi / 4) // 45°
		root := rendering.NewAbsoluteBox(W, H)
		root.Place(tr, 30, 30) // transform at (30,30), size 40x40 → center (50,50)
		rendering.NewPipelineOwner(root).FlushLayout(rendering.Size{Width: W, Height: H}, true)
		return inner
	}

	sample := func(dc *render.Context, x, y int) (uint8, uint8, uint8) {
		img := dc.Image()
		r, g, b, _ := img.At(x, y).RGBA()
		return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
	}

	inner := build()
	// Vector path: live paint via PaintContext.
	vecDC := render.NewContext(W, H)
	defer vecDC.Close()
	pc := &rendering.PaintContext{DC: vecDC, Scale: 1}
	rootObj := inner.Parent()
	for rootObj.Parent() != nil {
		rootObj = rootObj.Parent()
	}
	rootObj.Paint(pc)

	// Retained path: build layer tree + textured composite.
	b := rendering.BuildLayerTree(rootObj)
	pkt := b.BuildPacket(1, 1, W, H)
	retDC := render.NewContext(W, H)
	defer retDC.Close()
	tex := scene.NewPictureTextureCache(retDC, 8)
	scene.CompositeFramePacketTextured(pkt, retDC, tex)

	// Rotation-invariant center: both paths must agree it's red.
	vr, vg, vb := sample(vecDC, 50, 50)
	rr, rg, rb := sample(retDC, 50, 50)
	if vr < 150 || vg > 110 || vb > 110 {
		t.Fatalf("vector center rgb=%d,%d,%d want red (sanity)", vr, vg, vb)
	}
	if rr < 150 || rg > 110 || rb > 110 {
		t.Fatalf("retained center rgb=%d,%d,%d want red — retained path pivots wrong", rr, rg, rb)
	}
}
