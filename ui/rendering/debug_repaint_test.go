package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func TestDebugRepaint_MarksLivePaintNotReplay(t *testing.T) {
	const W, H = 80, 80
	hot := rendering.NewRenderColorBox(30, 30, 0.2, 0.7, 0.2, 1)
	hot.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(W, H)
	root.Place(hot, 10, 10)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()

	var draws int64
	// Frame 1: live paint → debug overlay.
	dc1 := render.NewContext(W, H)
	pc1 := rendering.NewPaintContext(dc1, 1)
	pc1.BoundaryCache, pc1.UseBoundaryCache = cache, true
	pc1.DebugRepaint = true
	pc1.DebugRepaintDraws = &draws
	cache.BeginFrame()
	root.Paint(pc1)
	if draws < 1 {
		t.Fatalf("live paint debug draws=%d want ≥1", draws)
	}

	// Frame 2: clean Replay → no additional live debug (Replay skips NoteDebugRepaint).
	before := draws
	dc2 := render.NewContext(W, H)
	pc2 := rendering.NewPaintContext(dc2, 1)
	pc2.BoundaryCache, pc2.UseBoundaryCache = cache, true
	pc2.DebugRepaint = true
	pc2.DebugRepaintDraws = &draws
	cache.BeginFrame()
	root.Paint(pc2)
	if draws != before {
		t.Fatalf("clean Replay added debug draws %d→%d (Replay must not count as repaint)", before, draws)
	}

	// Frame 3: dirty hot → live paint + debug again.
	hot.R = 0.9
	hot.MarkNeedsPaint()
	dc3 := render.NewContext(W, H)
	pc3 := rendering.NewPaintContext(dc3, 1)
	pc3.BoundaryCache, pc3.UseBoundaryCache = cache, true
	pc3.DebugRepaint = true
	pc3.DebugRepaintDraws = &draws
	cache.BeginFrame()
	root.Paint(pc3)
	if draws <= before {
		t.Fatalf("dirty paint debug draws=%d want >%d", draws, before)
	}
}
