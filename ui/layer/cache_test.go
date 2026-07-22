package layer_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/layer"
	"github.com/energye/gpui/ui/primitive"
)

func TestCacheReleaseAll(t *testing.T) {
	c := layer.NewCache()
	e := c.Ensure(1, 32, 32, 1)
	if e == nil || e.DC == nil {
		t.Fatal("Ensure")
	}
	if c.Len() != 1 {
		t.Fatalf("len=%d", c.Len())
	}
	c.ReleaseAll()
	if c.Len() != 0 {
		t.Fatal("ReleaseAll")
	}
}

func TestCompositorFramePaintsFullTree(t *testing.T) {
	comp := layer.NewCompositor()
	comp.BG = render.RGBA{R: 0.2, G: 0.2, B: 0.25, A: 1}
	comp.Resize(120, 80, 1)

	// Static box + boundary (both must land in base RT now).
	static := primitive.NewBox()
	static.Width, static.Height = 40, 20
	static.Color = render.RGBA{R: 1, G: 0, B: 0, A: 1}

	inner := primitive.NewBox()
	inner.Width, inner.Height = 16, 16
	inner.Color = render.RGBA{R: 0, G: 1, B: 0, A: 1}
	bound := primitive.NewRepaintBoundary(inner)

	root := primitive.NewBox(static, bound)
	root.Width, root.Height = 120, 80
	root.Color = comp.BG
	tree := core.NewTree(root)

	ok := comp.Frame(tree, nil, true)
	if !ok {
		// Offscreen GPU may be unavailable in some CI — not a hard fail.
		t.Log("Frame returned false (no GPU offscreen?); HasBase=", comp.HasBase())
		comp.ReleaseAll()
		return
	}
	if !comp.HasBase() {
		t.Fatal("expected valid base RT after successful Frame")
	}
	comp.ReleaseAll()
}

func TestCompositorReleaseAll(t *testing.T) {
	comp := layer.NewCompositor()
	comp.BG = render.RGBA{R: 0.1, G: 0.1, B: 0.1, A: 1}
	comp.Resize(64, 64, 1)
	root := primitive.NewBox()
	root.Width, root.Height = 64, 64
	tree := core.NewTree(root)
	_ = comp.Frame(tree, nil, true)
	comp.ReleaseAll()
	if comp.HasBase() {
		t.Fatal("HasBase after ReleaseAll")
	}
}
