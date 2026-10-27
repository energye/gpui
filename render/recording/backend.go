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
	"io"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// Backend is the interface that all export backends must implement.
// Backends receive high-level drawing commands and translate them to
// their output format (raster pixels, PDF content streams, SVG elements, etc.).
//
// A Backend manages its own state stack for Save/Restore operations and
// handles coordinate transformations as needed (e.g., PDF Y-flip).
//
// Backends are created via the registry using NewBackend(name) and
// registered via Register() in their init() functions.
//
// # Implementation Contract
//
// Each backend must:
//  1. Register in init() using recording.Register()
//  2. Handle all Backend methods (even if no-op for some)
//  3. Manage own state stack for Save/Restore
//  4. Translate coordinates if needed (e.g., PDF Y-flip)
//
// # Coordinate contract
//
// All geometry handed to a Backend — paths, rects, image destinations and text
// positions — is in USER space. The active CTM is whatever the replayed
// ConcatCommand stream composed on top of the matrix the backend already had
// when playback began (Save/Restore nest it). Backends draw through that
// matrix instead of resetting it: recordings carry no baked device coordinates,
// so one recording replays correctly at any device scale and under any layer
// offset.
//
// # Example Backend Registration
//
//	func init() {
//	    recording.Register("pdf", func() recording.Backend {
//	        return NewPDFBackend()
//	    })
//	}
type Backend interface {
	// Lifecycle methods

	// Begin initializes the backend for rendering at the given dimensions.
	// This must be called before any drawing operations.
	// Returns an error if initialization fails.
	Begin(width, height int) error

	// End finalizes the rendering and prepares the output.
	// After End is called, output methods (WriteTo, SaveToFile) can be used.
	// Returns an error if finalization fails.
	End() error

	// State management methods

	// Save saves the current graphics state (transform, clip) onto a stack.
	// The state can be restored with Restore.
	Save()

	// Restore restores the graphics state from the stack.
	// If the stack is empty, this is a no-op.
	Restore()

	// Transform methods

	// Concat multiplies the given matrix into the current transformation
	// matrix. Transform ops in a recording are relative: the backend composes
	// onto the CTM it already has, so a playback context pre-set with a layer
	// offset and device scale renders the recording in the right place.
	Concat(m Matrix)

	// Clipping methods

	// SetClip sets the clipping region to the given path.
	// Only pixels inside the path (according to the fill rule) will be drawn.
	// The clip is intersected with any existing clip.
	SetClip(path *render.Path, rule FillRule)

	// ClearClip removes any clipping region.
	ClearClip()

	// ClipRect intersects the clip region with an axis-aligned rectangle given
	// in user space. It is the cheaper counterpart of SetClip for rects.
	ClipRect(rect Rect)

	// ClipRectOp combines the clip region with an axis-aligned rectangle in user
	// space using op — intersect, difference or replace (C.06). ClipRect is the
	// intersect-only special case.
	ClipRectOp(rect Rect, op render.ClipOp)

	// ClipPathOp combines the clip region with the path using op and the fill
	// rule, the operation-complete counterpart of SetClip.
	ClipPathOp(path *render.Path, rule FillRule, op render.ClipOp)

	// ClipRoundRect intersects the clip region with a rounded rectangle in user
	// space. Playback must take the same call the paint walk made: re-expressing
	// it as a path changes the rasterizer's route (a CPU clip mask instead of
	// the rounded-corner coverage the live canvas uses), which is both slower and
	// a fidelity difference between live paint and replay.
	ClipRoundRect(x, y, w, h, radius float64)

	// Drawing methods

	// FillPath fills the given path with the brush color/pattern.
	// The rule determines how to handle self-intersecting paths. blend is the
	// Porter-Duff mode the recording captured for this draw; backends apply it
	// for this call only.
	FillPath(path *render.Path, brush Brush, rule FillRule, blend render.BlendMode)

	// StrokePath strokes the given path with the brush and stroke style.
	StrokePath(path *render.Path, brush Brush, stroke Stroke, blend render.BlendMode)

	// FillRect is an optimized method for filling axis-aligned rectangles.
	// Backends may implement this more efficiently than FillPath with a rect path.
	FillRect(rect Rect, brush Brush, blend render.BlendMode)

	// StrokeRect strokes an axis-aligned rectangle kept in user space, the
	// stroking counterpart of FillRect.
	StrokeRect(rect Rect, brush Brush, stroke Stroke, blend render.BlendMode)

	// DrawImage draws an image buffer through the recorded options. Backends
	// receive the same render.DrawImageOptions the caller recorded (destination,
	// source sub-rect, interpolation, opacity, blend mode), so replay reaches the
	// rasterizer without losing fields the display list never stored.
	DrawImage(img *render.ImageBuf, opts render.DrawImageOptions)

	// DrawImageNine stretches the edges of img between the centre patch bounds
	// to fill dst, keeping the corners at native size.
	DrawImageNine(img *render.ImageBuf, center image.Rectangle, dst Rect)

	// DrawImageQuad maps img onto an arbitrary quad.
	DrawImageQuad(img *render.ImageBuf, corners [4]render.Point, opts render.QuadDrawOptions)

	// DrawImageRounded draws img clipped to a rounded rectangle.
	DrawImageRounded(img *render.ImageBuf, x, y, radius float64)

	// DrawImageCircular draws img clipped to a circle.
	DrawImageCircular(img *render.ImageBuf, cx, cy, radius float64)

	// DrawAtlas draws a batch of sprites sampled from one image.
	DrawAtlas(img *render.ImageBuf, sprites []render.AtlasSprite, opts render.AtlasDrawOptions)

	// DrawDepthSprites draws a sprite batch carrying per-sprite depth.
	DrawDepthSprites(img *render.ImageBuf, sprites []render.DepthSprite, opts render.DepthDrawOptions)

	// DrawVertices draws an immediate-mode vertex mesh. brush is the fill paint
	// recorded with the mesh — the fallback colour when a mesh carries fewer
	// colours than vertices.
	DrawVertices(positions []render.Point, colors []render.RGBA, mode render.VertexMode, brush Brush)

	// DrawMesh draws an indexed (or triangle-list) coloured mesh. brush is the
	// fill paint recorded with it, the fallback when the mesh carries no per
	// vertex colours.
	DrawMesh(mesh render.Mesh, brush Brush)

	// DrawGlyphs draws a shaped-glyph run at the baseline origin. colored selects
	// the per-glyph colour variant over the single-brush run, which uses brush.
	DrawGlyphs(glyphs []text.ShapedGlyph, face text.Face, x, y float64, colored bool, brush Brush)

	// DrawText draws text at the given position with the specified font face and brush.
	// The position (x, y) is the baseline origin of the text.
	DrawText(s string, x, y float64, face text.Face, brush Brush)

	// Decoration and post-effect state

	// SetTextDecoration sets the decoration bitset for text drawn afterwards.
	SetTextDecoration(deco render.TextDecoration)

	// SetTextMode selects the text rendering strategy for text drawn afterwards.
	SetTextMode(mode render.TextMode)

	// SetLCDLayout sets the LCD subpixel order for ClearType text.
	SetLCDLayout(layout render.LCDLayout)

	// SetAntiAlias enables or disables anti-aliasing for the geometry and clips
	// drawn afterwards.
	SetAntiAlias(enabled bool)

	// ApplyBlur attaches a blur post-effect to subsequent draws.
	ApplyBlur(radius float64)

	// ApplyColorMatrix attaches a 4x5 colour matrix post-effect.
	ApplyColorMatrix(matrix [20]float32)

	// ApplyDropShadow attaches a drop-shadow post-effect.
	ApplyDropShadow(offsetX, offsetY, blurRadius float64, color render.RGBA)

	// ApplyGrayscale desaturates subsequent draws.
	ApplyGrayscale()

	// Compositing layers

	// PushLayer opens a layer composited onto the parent with blend and opacity.
	PushLayer(blend render.BlendMode, opacity float64)

	// PushLayerIsolated opens a forced true save-layer.
	PushLayerIsolated(opacity float64)

	// PushBackdropLayer opens a layer seeded from the parent's contents.
	PushBackdropLayer(blend render.BlendMode, opacity float64)

	// PopLayer composites the innermost layer.
	PopLayer()
}

// GPUTextureBackend is the optional capability of replaying a device-owned GPU
// texture view (DrawGPUTextureCommand). It is separate from Backend because it
// only makes sense on targets that reach the device the view belongs to: the
// live context and CanvasBackend do, export backends (image, PDF, SVG) do not
// and skip the command.
type GPUTextureBackend interface {
	DrawGPUTexture(view render.TextureView, x, y float64, width, height int)
}

// MaskBackend is the optional capability of replaying device-space mask
// post-processes (SetMask/ClearMask/InvertMask/ApplyMask/PushMaskLayer).
//
// It is separate from Backend because a mask plane lives in target device
// pixels: only a target that can reproduce the recording's device space may
// execute it (the live context and CanvasBackend do; export backends cannot).
// Playback skips mask commands on backends that do not implement this
// interface and counts the skip in MaskSkips.
//
// The mask passed to SetMask/ApplyMask/PushMaskLayer is a fresh snapshot owned
// by the backend; implementations may retain or mutate it without affecting
// the recording or a later playback.
type MaskBackend interface {
	SetMask(mask *render.Mask)
	ClearMask()
	InvertMask()
	ApplyMask(mask *render.Mask)
	PushMaskLayer(mask *render.Mask)
}

// WriterBackend extends Backend with the ability to write output to an io.Writer.
// This is useful for streaming output or writing to network connections.
type WriterBackend interface {
	Backend

	// WriteTo writes the rendered content to the given writer.
	// This should only be called after End().
	// Returns the number of bytes written and any error.
	WriteTo(w io.Writer) (int64, error)
}

// FileBackend extends Backend with the ability to save output directly to a file.
// This is a convenience interface for backends that can optimize file output.
type FileBackend interface {
	Backend

	// SaveToFile saves the rendered content to a file at the given path.
	// This should only be called after End().
	// Returns an error if the file cannot be written.
	SaveToFile(path string) error
}

// PixmapBackend extends Backend with access to a rasterized pixmap.
// This is implemented by the raster backend and allows direct pixel access.
type PixmapBackend interface {
	Backend

	// Pixmap returns the rendered pixmap.
	// This should only be called after End().
	// Returns nil if no pixmap is available.
	Pixmap() *render.Pixmap
}
