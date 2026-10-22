package raster

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

func TestBackendApplyBrushLinearGradient(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(100, 100)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	// Linear gradient brush
	grad := recording.NewLinearGradientBrush(0, 0, 100, 0).
		AddColorStop(0, render.Red).
		AddColorStop(1, render.Blue)

	rect := recording.NewRect(0, 0, 100, 100)
	backend.FillRect(rect, grad)

	err = backend.End()
	if err != nil {
		t.Fatalf("End: %v", err)
	}
}

func TestBackendApplyBrushRadialGradient(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(100, 100)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	grad := recording.NewRadialGradientBrush(50, 50, 0, 50).
		AddColorStop(0, render.White).
		AddColorStop(1, render.Black)

	rect := recording.NewRect(0, 0, 100, 100)
	backend.FillRect(rect, grad)

	_ = backend.End()
}

func TestBackendApplyBrushSweepGradient(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(100, 100)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	grad := recording.NewSweepGradientBrush(50, 50, 0).
		AddColorStop(0, render.Red).
		AddColorStop(0.5, render.Green).
		AddColorStop(1, render.Blue)

	rect := recording.NewRect(0, 0, 100, 100)
	backend.FillRect(rect, grad)

	_ = backend.End()
}

func TestBackendApplyStrokeBrush(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(100, 100)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	// Stroke with gradient
	grad := recording.NewLinearGradientBrush(0, 0, 100, 0).
		AddColorStop(0, render.Red).
		AddColorStop(1, render.Blue)

	path := render.NewPath()
	path.MoveTo(10, 50)
	path.LineTo(90, 50)

	stroke := recording.Stroke{
		Width:      3.0,
		Cap:        recording.LineCapSquare,
		Join:       recording.LineJoinBevel,
		MiterLimit: 4.0,
	}
	backend.StrokePath(path, grad, stroke)

	_ = backend.End()
}

func TestBackendSetClip(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(100, 100)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	path := render.NewPath()
	path.Rectangle(10, 10, 80, 80)
	backend.SetClip(path, recording.FillRuleNonZero)

	// SetClip with nil should not panic
	backend.SetClip(nil, recording.FillRuleNonZero)

	// ClearClip should not panic
	backend.ClearClip()

	_ = backend.End()
}

func TestBackendFillPathNil(t *testing.T) {
	backend := NewBackend()
	_ = backend.Begin(100, 100)

	// Nil path should not panic
	backend.FillPath(nil, recording.NewSolidBrush(render.Red), recording.FillRuleNonZero)

	_ = backend.End()
}

func TestBackendStrokePathNil(t *testing.T) {
	backend := NewBackend()
	_ = backend.Begin(100, 100)

	// Nil path should not panic
	backend.StrokePath(nil, recording.NewSolidBrush(render.Red), recording.Stroke{Width: 2})

	_ = backend.End()
}

func TestBackendDrawImageNil(t *testing.T) {
	backend := NewBackend()
	_ = backend.Begin(100, 100)

	// Nil and disposed images should not panic on any image entry point.
	backend.DrawImage(nil, render.DrawImageOptions{X: 0, Y: 0, Opacity: 1})
	backend.DrawImageNine(nil, image.Rect(1, 1, 2, 2), recording.NewRect(0, 0, 50, 50))
	backend.DrawImageQuad(nil, [4]render.Point{}, render.QuadDrawOptions{})
	backend.DrawImageRounded(nil, 0, 0, 4)
	backend.DrawImageCircular(nil, 10, 10, 8)
	backend.DrawAtlas(nil, []render.AtlasSprite{{DstX: 0, DstY: 0, DstW: 4, DstH: 4}}, render.AtlasDrawOptions{})
	backend.DrawDepthSprites(nil, []render.DepthSprite{}, render.DepthDrawOptions{})

	_ = backend.End()
}

func TestBackendCanvasOpsNoPanic(t *testing.T) {
	backend := NewBackend()
	if err := backend.Begin(60, 60); err != nil {
		t.Fatalf("Begin: %v", err)
	}

	backend.ClipRect(recording.NewRect(4, 4, 20, 20))
	backend.SetTextDecoration(render.TextDecorationUnderline)
	backend.DrawVertices([]render.Point{{X: 5, Y: 5}, {X: 25, Y: 5}, {X: 5, Y: 25}},
		[]render.RGBA{{R: 1, A: 1}}, render.VertexModeTriangles, recording.NewSolidBrush(render.Red))
	backend.DrawVertices([]render.Point{{X: 5, Y: 5}, {X: 25, Y: 5}, {X: 5, Y: 25}},
		nil, render.VertexModeTriangles, nil)
	backend.DrawGlyphs(nil, nil, 10, 10, false, nil)
	backend.ApplyBlur(2)
	backend.ApplyColorMatrix([20]float32{1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0})
	backend.ApplyDropShadow(1, 1, 2, render.Black)
	backend.ApplyGrayscale()
	backend.PushLayer(render.BlendNormal, 0.5)
	backend.PushLayerIsolated(0.5)
	backend.PushBackdropLayer(render.BlendNormal, 1)
	backend.PopLayer()
	backend.PopLayer()
	backend.PopLayer()

	if err := backend.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
}

func TestBackendDrawText(t *testing.T) {
	backend := NewBackend()
	_ = backend.Begin(100, 100)

	// DrawText with nil face should not panic
	backend.DrawText("hello", 10, 10, nil, recording.NewSolidBrush(render.Black))

	_ = backend.End()
}

func TestBackendFillRuleEvenOdd(t *testing.T) {
	backend := NewBackend()
	_ = backend.Begin(100, 100)

	path := render.NewPath()
	path.Rectangle(10, 10, 80, 80)
	backend.FillPath(path, recording.NewSolidBrush(render.Red), recording.FillRuleEvenOdd)

	_ = backend.End()
}

func TestBackendSavePNG(t *testing.T) {
	backend := NewBackend()
	_ = backend.Begin(10, 10)

	rect := recording.NewRect(0, 0, 10, 10)
	backend.FillRect(rect, recording.NewSolidBrush(render.Red))

	_ = backend.End()

	// SavePNG to non-existent directory should fail
	err := backend.SavePNG("/nonexistent/path/test.png")
	if err == nil {
		t.Error("SavePNG to invalid path should return error")
	}
}

// The recording style enums are aliases of render's, not a parallel set: a
// recorded cap/join/rule reaches a context without any conversion step. These
// assignments are the check — they fail to compile if the types diverge again.
func TestBackendStyleEnumsAreRenderTypes(t *testing.T) {
	var (
		rule  render.FillRule = recording.FillRuleEvenOdd
		capit render.LineCap  = recording.LineCapSquare
		join  render.LineJoin = recording.LineJoinBevel
	)
	if rule != render.FillRuleEvenOdd || capit != render.LineCapSquare || join != render.LineJoinBevel {
		t.Errorf("enum alias mismatch: rule=%v cap=%v join=%v", rule, capit, join)
	}
}
