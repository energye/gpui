package embedder

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// TestConsumePacketPaintFlagsOnlyRetained guards the full-paint ordering hole:
// only the retained packet path may clear dirty flags before the raster job
// runs. Full-paint frames must keep them until FlushPaint, otherwise a dirty
// boundary with unchanged content is treated as a clean cache Replay.
func TestConsumePacketPaintFlagsOnlyRetained(t *testing.T) {
	const W, H = 80, 80
	hot := rendering.NewRenderColorBox(30, 30, 0.2, 0.7, 0.2, 1)
	hot.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(W, H)
	root.Place(hot, 10, 10)
	pipe := rendering.NewPipelineOwner(root)
	pipe.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	app := &PipelineApp{pipe: pipe}

	hot.MarkNeedsPaint()
	app.consumePacketPaintFlags(false)
	if !hot.NeedsPaint() {
		t.Fatal("full-paint path must keep dirty flags until FlushPaint")
	}

	app.consumePacketPaintFlags(true)
	if hot.NeedsPaint() {
		t.Fatal("retained packet path must consume dirty flags (no live paint)")
	}
}
