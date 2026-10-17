package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// TestDebugRepaint_MarksLivePaintNotReplay: the R12b overlay counts live
// repaints only — a Replay must not bump the counter. The hot boundary needs
// content worth caching (naive gate: 7 ops) and enough unchanged frames to be
// admitted (access threshold 3), so the test settles it before the replay
// check.
func TestDebugRepaint_MarksLivePaintNotReplay(t *testing.T) {
	const W, H = 80, 80
	blue := 0.2
	hot := rendering.NewRenderPictureBox(30, 30, func(r *scene.PictureRecorder, ox, oy float64) {
		r.FillRect(ox, oy, 30, 30, 0.2, 0.7, 0.2, 1)
		for i := 0; i < 6; i++ {
			r.FillRect(ox+float64(2+i*4), oy+2, 3, 20, 0.1, 0.1, blue, 1)
		}
	})
	hot.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(W, H)
	root.Place(hot, 10, 10)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()

	var draws int64
	paint := func() (rerecord, skip int64) {
		dc := render.NewContext(W, H)
		defer dc.Close()
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		pc.DebugRepaint = true
		pc.DebugRepaintDraws = &draws
		cache.BeginFrame()
		root.Paint(pc)
		rerecord, skip, _, _ = cache.FrameCounts()
		return rerecord, skip
	}

	// Frame 1: live paint → debug overlay.
	paint()
	if draws < 1 {
		t.Fatalf("live paint debug draws=%d want ≥1", draws)
	}
	// Frames 2-4: held back by the access threshold, still live (overlay grows).
	for i := 0; i < 3; i++ {
		before := draws
		paint()
		if draws <= before {
			t.Fatalf("held-back frame %d must still live-paint (draws %d→%d)", i, before, draws)
		}
	}
	// Frame 5: admitted (rerecord=1) — still a live paint.
	if rr, _ := paint(); rr != 1 {
		t.Fatalf("admit frame rerecord=%d want 1", rr)
	}

	// Frame 6: clean Replay → no additional live debug (Replay skips NoteDebugRepaint).
	before := draws
	rr, skip := paint()
	if rr != 0 || skip != 1 {
		t.Fatalf("clean frame rerecord=%d skip=%d want 0/1 (must be a Replay)", rr, skip)
	}
	if draws != before {
		t.Fatalf("clean Replay added debug draws %d→%d (Replay must not count as repaint)", before, draws)
	}

	// Frame 7: dirty hot → live paint + debug again.
	blue = 0.9
	hot.MarkNeedsPaint()
	dirtyBefore := draws
	paint()
	if draws <= dirtyBefore {
		t.Fatalf("dirty paint debug draws=%d want >%d", draws, dirtyBefore)
	}
}
