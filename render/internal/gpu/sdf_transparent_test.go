//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestQueueShape_SkipsZeroAlpha verifies that QueueShape skips shapes with
// zero alpha color (BUG-SDF-001). This prevents transparent fills from
// interfering with subsequent strokes via MSAA coverage weighting.
// Enterprise pattern: Skia nothingToDraw(), Cairo nothing_to_do().
func TestQueueShape_SkipsZeroAlpha(t *testing.T) {
	shared := NewGPUShared()
	rc := shared.NewRenderContext()

	target := render.GPURenderTarget{Width: 100, Height: 100, Data: make([]byte, 100*100*4), Stride: 400}

	// Transparent fill — should be skipped
	transparentPaint := render.NewPaint()
	transparentPaint.SetBrush(render.Solid(render.RGBA{R: 0, G: 0, B: 0, A: 0}))

	shape := render.DetectedShape{
		Kind:    render.ShapeRRect,
		CenterX: 50, CenterY: 50,
		Width: 80, Height: 60,
		CornerRadius: 8,
	}

	err := rc.QueueShape(target, shape, transparentPaint, false)
	if err != nil {
		t.Fatalf("QueueShape(transparent): %v", err)
	}
	if rc.PendingCount() != 0 {
		t.Errorf("transparent fill should be skipped, got %d pending shapes", rc.PendingCount())
	}

	// Visible stroke — should be queued
	visiblePaint := render.NewPaint()
	visiblePaint.SetBrush(render.Solid(render.RGBA{R: 0.2, G: 0.6, B: 0.85, A: 1.0}))
	visiblePaint.SetStroke(render.Stroke{Width: 1.5})

	err = rc.QueueShape(target, shape, visiblePaint, true)
	if err != nil {
		t.Fatalf("QueueShape(visible): %v", err)
	}
	if rc.PendingCount() != 1 {
		t.Errorf("visible stroke should be queued, got %d pending shapes", rc.PendingCount())
	}
}

// TestQueueShape_KeepsSemiTransparent verifies that shapes with partial
// alpha (e.g., 0.5) are NOT skipped — only fully transparent (alpha=0).
func TestQueueShape_KeepsSemiTransparent(t *testing.T) {
	shared := NewGPUShared()
	rc := shared.NewRenderContext()

	target := render.GPURenderTarget{Width: 100, Height: 100, Data: make([]byte, 100*100*4), Stride: 400}

	semiPaint := render.NewPaint()
	semiPaint.SetBrush(render.Solid(render.RGBA{R: 1, G: 0, B: 0, A: 0.5}))

	shape := render.DetectedShape{
		Kind:    render.ShapeCircle,
		CenterX: 50, CenterY: 50,
		RadiusX: 30, RadiusY: 30,
	}

	err := rc.QueueShape(target, shape, semiPaint, false)
	if err != nil {
		t.Fatalf("QueueShape(semi-transparent): %v", err)
	}
	if rc.PendingCount() != 1 {
		t.Errorf("semi-transparent shape should be queued, got %d pending", rc.PendingCount())
	}
}
