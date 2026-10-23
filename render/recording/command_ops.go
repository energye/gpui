//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package recording

import (
	"image"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// This file holds the display-list commands that give a Recorder the same
// drawing surface as a live *render.Context (render.Canvas parity, S2a). Each
// command mirrors one Context method and carries plain data only, so a
// recording stays inspectable and replayable on any thread that owns the
// target context.
//
// Live-only Context methods (GPU texture handles, present, offscreen passes,
// encode) are deliberately absent: a display list cannot carry a GPU handle
// that only exists on the raster thread.

// ClipRectCommand sets an axis-aligned rectangular clip in user space.
type ClipRectCommand struct {
	Rect Rect
}

// Type implements Command.
func (ClipRectCommand) Type() CommandType { return CmdClipRect }

// SetTextDecorationCommand sets the decoration bitset used by later text
// draws recorded on this stream.
type SetTextDecorationCommand struct {
	Deco render.TextDecoration
}

// Type implements Command.
func (SetTextDecorationCommand) Type() CommandType { return CmdSetTextDecoration }

// DrawImageNineCommand is a nine-patch image draw: the four corners and the
// centre of `Center` keep their size while the edges stretch to `Dst`.
type DrawImageNineCommand struct {
	Image  ImageRef
	Center image.Rectangle
	Dst    Rect
}

// Type implements Command.
func (DrawImageNineCommand) Type() CommandType { return CmdDrawImageNine }

// DrawImageQuadCommand maps an image onto an arbitrary quad.
type DrawImageQuadCommand struct {
	Image   ImageRef
	Corners [4]render.Point
	Opts    render.QuadDrawOptions
}

// Type implements Command.
func (DrawImageQuadCommand) Type() CommandType { return CmdDrawImageQuad }

// DrawImageRoundedCommand draws an image clipped to a rounded rectangle.
type DrawImageRoundedCommand struct {
	Image  ImageRef
	X, Y   float64
	Radius float64
}

// Type implements Command.
func (DrawImageRoundedCommand) Type() CommandType { return CmdDrawImageRounded }

// DrawImageCircularCommand draws an image clipped to a circle.
type DrawImageCircularCommand struct {
	Image  ImageRef
	CX, CY float64
	Radius float64
}

// Type implements Command.
func (DrawImageCircularCommand) Type() CommandType { return CmdDrawImageCircular }

// DrawAtlasCommand is a batch of sprites sampled from one image. DrawAtlas and
// DrawAtlasEx share it: AtlasDrawOptions currently carries no fields, and the
// degradation report Context returns is a raster-time fact a recording cannot
// precompute.
type DrawAtlasCommand struct {
	Image   ImageRef
	Sprites []render.AtlasSprite
	Opts    render.AtlasDrawOptions
}

// Type implements Command.
func (DrawAtlasCommand) Type() CommandType { return CmdDrawAtlas }

// DrawDepthSpritesCommand is a sprite batch carrying per-sprite depth.
type DrawDepthSpritesCommand struct {
	Image   ImageRef
	Sprites []render.DepthSprite
	Opts    render.DepthDrawOptions
}

// Type implements Command.
func (DrawDepthSpritesCommand) Type() CommandType { return CmdDrawDepthSprites }

// DrawVerticesCommand is an immediate-mode mesh. Brush is the fill colour the
// mesh was recorded with: Context.DrawVertices falls back to the current fill
// paint when the colour array is short, so the recording has to carry it —
// replay's context would otherwise inherit whatever the previous command set.
type DrawVerticesCommand struct {
	Positions []render.Point
	Colors    []render.RGBA
	Mode      render.VertexMode
	Brush     BrushRef
}

// Type implements Command.
func (DrawVerticesCommand) Type() CommandType { return CmdDrawVertices }

// DrawGlyphsCommand is a shaped-glyph run. Colored selects the per-glyph
// colour variant (Context.DrawShapedColorGlyphs) over the single-brush run,
// whose colour comes from the recorded Brush.
type DrawGlyphsCommand struct {
	Glyphs  []text.ShapedGlyph
	Face    FontRef
	X, Y    float64
	Colored bool
	Brush   BrushRef
}

// Type implements Command.
func (DrawGlyphsCommand) Type() CommandType { return CmdDrawGlyphs }

// --------------------------------------------------------------------------
// Clipping with operations
// --------------------------------------------------------------------------

// ClipRectOpCommand intersects, subtracts or replaces the clip with an
// axis-aligned rectangle in user space. ClipRectCommand is the intersect-only
// special case; this is the SkClipOp-complete counterpart (C.06).
type ClipRectOpCommand struct {
	Rect Rect
	Op   render.ClipOp
}

// Type implements Command.
func (ClipRectOpCommand) Type() CommandType { return CmdClipRectOp }

// ClipPathOpCommand applies the current path to the clip with the given
// operation. The fill rule is the one recorded at clip time, matching
// Context.ClipPathOp, which reads paint.FillRule rather than an argument.
type ClipPathOpCommand struct {
	Path PathRef
	Rule FillRule
	Op   render.ClipOp
}

// Type implements Command.
func (ClipPathOpCommand) Type() CommandType { return CmdClipPathOp }

// --------------------------------------------------------------------------
// Text quality state
// --------------------------------------------------------------------------

// SetTextModeCommand selects the text rendering strategy for later draws. It is
// Context state that no draw command carries, so it replays in stream order —
// which is also how the live path behaves, since Context.Push does not save the
// text mode.
type SetTextModeCommand struct {
	Mode render.TextMode
}

// Type implements Command.
func (SetTextModeCommand) Type() CommandType { return CmdSetTextMode }

// SetLCDLayoutCommand sets the LCD subpixel order for ClearType text.
type SetLCDLayoutCommand struct {
	Layout render.LCDLayout
}

// Type implements Command.
func (SetLCDLayoutCommand) Type() CommandType { return CmdSetLCDLayout }

// --------------------------------------------------------------------------
// Meshes
// --------------------------------------------------------------------------

// DrawMeshCommand is an indexed (or triangle-list) mesh. Brush is the fill paint
// the mesh was recorded with, for the same reason DrawVerticesCommand carries
// one: the vertex-colour fallback is the current paint, and a replaying context
// would otherwise inherit whatever the previous command left on it.
type DrawMeshCommand struct {
	Positions []render.Point
	Colors    []render.RGBA
	Indices   []uint16
	Brush     BrushRef
}

// Type implements Command.
func (DrawMeshCommand) Type() CommandType { return CmdDrawMesh }

// --------------------------------------------------------------------------
// Layers
// --------------------------------------------------------------------------

// PushLayerCommand opens a compositing layer; the matching pop composites it
// onto the parent with the recorded blend mode and opacity.
type PushLayerCommand struct {
	Blend   render.BlendMode
	Opacity float64
}

// Type implements Command.
func (PushLayerCommand) Type() CommandType { return CmdPushLayer }

// PushLayerIsolatedCommand opens a forced true save-layer (Flutter
// Canvas.saveLayer semantics) so overlapping draws within the group composite
// among themselves before blending.
type PushLayerIsolatedCommand struct {
	Opacity float64
}

// Type implements Command.
func (PushLayerIsolatedCommand) Type() CommandType { return CmdPushLayerIsolated }

// PushBackdropLayerCommand opens a layer that starts from the parent's
// contents instead of a cleared surface.
type PushBackdropLayerCommand struct {
	Blend   render.BlendMode
	Opacity float64
}

// Type implements Command.
func (PushBackdropLayerCommand) Type() CommandType { return CmdPushBackdropLayer }

// PopLayerCommand composites the innermost layer.
type PopLayerCommand struct{}

// Type implements Command.
func (PopLayerCommand) Type() CommandType { return CmdPopLayer }

// --------------------------------------------------------------------------
// Post-effects
// --------------------------------------------------------------------------

// ApplyBlurCommand applies a blur post-effect to subsequent draws.
type ApplyBlurCommand struct {
	Radius float64
}

// Type implements Command.
func (ApplyBlurCommand) Type() CommandType { return CmdApplyBlur }

// ApplyColorMatrixCommand applies a 4x5 colour matrix post-effect.
type ApplyColorMatrixCommand struct {
	Matrix [20]float32
}

// Type implements Command.
func (ApplyColorMatrixCommand) Type() CommandType { return CmdApplyColorMatrix }

// ApplyDropShadowCommand applies a drop-shadow post-effect.
type ApplyDropShadowCommand struct {
	OffsetX, OffsetY float64
	BlurRadius       float64
	Color            render.RGBA
}

// Type implements Command.
func (ApplyDropShadowCommand) Type() CommandType { return CmdApplyDropShadow }

// ApplyGrayscaleCommand desaturates subsequent draws.
type ApplyGrayscaleCommand struct{}

// Type implements Command.
func (ApplyGrayscaleCommand) Type() CommandType { return CmdApplyGrayscale }
