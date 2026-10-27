package recording

import (
	"image"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// stubBackend is a do-nothing Backend that test doubles embed so they only
// write the methods they actually observe. The production interpreter is
// CanvasBackend; this exists so adding an operation to the Backend interface
// touches one stub here instead of every mock in the tree.
type stubBackend struct{}

var _ Backend = (*stubBackend)(nil)

func (stubBackend) Begin(_, _ int) error { return nil }
func (stubBackend) End() error           { return nil }
func (stubBackend) Save()                {}
func (stubBackend) Restore()             {}
func (stubBackend) Concat(_ Matrix)      {}
func (stubBackend) SetClip(_ *render.Path, _ FillRule) {
}
func (stubBackend) ClearClip()                          {}
func (stubBackend) ClipRect(_ Rect)                     {}
func (stubBackend) ClipRoundRect(_, _, _, _, _ float64) {}
func (stubBackend) ClipRectOp(_ Rect, _ render.ClipOp)  {}
func (stubBackend) ClipPathOp(_ *render.Path, _ FillRule, _ render.ClipOp) {
}
func (stubBackend) FillPath(_ *render.Path, _ Brush, _ FillRule, _ render.BlendMode) {
}
func (stubBackend) StrokePath(_ *render.Path, _ Brush, _ Stroke, _ render.BlendMode) {}
func (stubBackend) FillRect(_ Rect, _ Brush, _ render.BlendMode)                     {}
func (stubBackend) StrokeRect(_ Rect, _ Brush, _ Stroke, _ render.BlendMode)         {}
func (stubBackend) DrawImage(_ *render.ImageBuf, _ render.DrawImageOptions) {
}
func (stubBackend) DrawImageNine(_ *render.ImageBuf, _ image.Rectangle, _ Rect) {}
func (stubBackend) DrawImageQuad(_ *render.ImageBuf, _ [4]render.Point, _ render.QuadDrawOptions) {
}
func (stubBackend) DrawImageRounded(_ *render.ImageBuf, _, _, _ float64)  {}
func (stubBackend) DrawImageCircular(_ *render.ImageBuf, _, _, _ float64) {}
func (stubBackend) DrawAtlas(_ *render.ImageBuf, _ []render.AtlasSprite, _ render.AtlasDrawOptions) {
}
func (stubBackend) DrawDepthSprites(_ *render.ImageBuf, _ []render.DepthSprite, _ render.DepthDrawOptions) {
}
func (stubBackend) DrawVertices(_ []render.Point, _ []render.RGBA, _ render.VertexMode, _ Brush) {
}
func (stubBackend) DrawMesh(_ render.Mesh, _ Brush) {
}
func (stubBackend) DrawGlyphs(_ []text.ShapedGlyph, _ text.Face, _, _ float64, _ bool, _ Brush) {
}
func (stubBackend) DrawText(_ string, _, _ float64, _ text.Face, _ Brush) {}
func (stubBackend) SetTextDecoration(_ render.TextDecoration)             {}
func (stubBackend) SetTextMode(_ render.TextMode)                         {}
func (stubBackend) SetLCDLayout(_ render.LCDLayout)                       {}
func (stubBackend) SetAntiAlias(_ bool)                                   {}
func (stubBackend) ApplyBlur(_ float64)                                   {}
func (stubBackend) ApplyColorMatrix(_ [20]float32)                        {}
func (stubBackend) ApplyDropShadow(_, _, _ float64, _ render.RGBA)        {}
func (stubBackend) ApplyGrayscale()                                       {}
func (stubBackend) PushLayer(_ render.BlendMode, _ float64)               {}
func (stubBackend) PushLayerIsolated(_ float64)                           {}
func (stubBackend) PushBackdropLayer(_ render.BlendMode, _ float64)       {}
func (stubBackend) PopLayer()                                             {}
