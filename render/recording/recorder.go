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

// Recorder captures drawing operations as commands.
// It mirrors the render.Context drawing API but generates commands
// instead of rasterizing pixels. Use FinishRecording to obtain
// an immutable Recording that can be replayed to different backends.
//
// # Coordinate space
//
// Geometry is stored in user space: the recorded transforms travel in the
// command stream as relative concats (ConcatCommand) and each backend applies
// them onto its own CTM. This matches render.Context, whose path verbs are
// baked by the user matrix and whose device scale is folded in only at the
// rasterization boundary (totalMatrix). A recording therefore replays crisp at
// any device scale and survives layer offsets without being re-recorded.
//
// Example:
//
//	rec := recording.NewRecorder(800, 600)
//	rec.SetRGB(1, 0, 0)
//	rec.DrawCircle(100, 100, 50)
//	rec.Fill()
//	recording := rec.FinishRecording()
//
// The Recorder is not safe for concurrent use.
type Recorder struct {
	width, height int
	commands      []Command
	resources     *ResourcePool

	// Current path being built, stored in user space relative to pathBasis
	currentPath *render.Path
	pathBasis   Matrix

	// Current state
	fillBrush   Brush
	strokeBrush Brush
	lineWidth   float64
	lineCap     LineCap
	lineJoin    LineJoin
	miterLimit  float64
	dashPattern []float64
	dashOffset  float64
	fillRule    FillRule
	antiAlias   bool
	transform   Matrix

	// Current font
	fontFace   text.Face
	fontFamily string
	fontSize   float64

	// State stack
	stateStack []recorderState
}

// recorderState stores the graphics state for Save/Restore.
type recorderState struct {
	fillBrush   Brush
	strokeBrush Brush
	lineWidth   float64
	lineCap     LineCap
	lineJoin    LineJoin
	miterLimit  float64
	dashPattern []float64
	dashOffset  float64
	fillRule    FillRule
	antiAlias   bool
	transform   Matrix
	fontFace    text.Face
	fontFamily  string
	fontSize    float64
}

// NewRecorder creates a new Recorder for the given dimensions.
// The Recorder starts with default state: black fill/stroke, 1px line width,
// butt caps, miter joins, non-zero fill rule, and identity transform.
func NewRecorder(width, height int) *Recorder {
	defaultBrush := NewSolidBrush(render.Black)
	return &Recorder{
		width:       width,
		height:      height,
		commands:    make([]Command, 0, 256),
		resources:   NewResourcePool(),
		currentPath: render.NewPath(),
		fillBrush:   defaultBrush,
		strokeBrush: defaultBrush,
		lineWidth:   1.0,
		lineCap:     LineCapButt,
		lineJoin:    LineJoinMiter,
		miterLimit:  4.0,
		fillRule:    FillRuleNonZero,
		antiAlias:   true,
		transform:   Identity(),
		pathBasis:   Identity(),
		stateStack:  make([]recorderState, 0, 8),
	}
}

// FinishRecording returns an immutable Recording containing all recorded
// commands. The content footprint, the can-it-paint flag and the replay facts a
// retained compositor needs (length, image sampling) are resolved here, on the
// recording thread (see bounds.go), so a consumer never has to measure shared
// font faces or read live commands to size, admit or budget a surface.
// After calling FinishRecording, the Recorder should not be used again.
func (r *Recorder) FinishRecording() *Recording {
	s := summarize(r.width, r.height, r.commands, r.resources)
	return &Recording{
		width:      r.width,
		height:     r.height,
		commands:   r.commands,
		resources:  r.resources,
		bounds:     s.bounds,
		visible:    s.visible,
		paintOps:   s.paintOps,
		cpuScratch: s.cpuScratch,
	}
}

// Recording is an immutable container for recorded drawing commands.
// It can be replayed to any Backend implementation.
type Recording struct {
	width, height int
	commands      []Command
	resources     *ResourcePool
	bounds        image.Rectangle
	visible       bool
	paintOps      int
	cpuScratch    bool
}

// Bounds returns the recording's content footprint in the recording's own
// coordinate space — that is, where the geometry lands when playback starts
// from the identity CTM. It is the layer-local, scale-agnostic space the
// recorder was drawn in: the raster thread's layer offset and device scale
// compose onto it at playback, so a consumer sizes and damages a surface by
// scaling this rect, never by re-measuring the commands.
//
// The rect is a superset by construction: clips are not intersected, zero-alpha
// paint still counts as geometry, and a recording carrying a blur or drop
// shadow reports the whole canvas, because those filters act on the surface
// rather than on the command that follows. Empty means nothing was recorded;
// a nil recording is empty.
func (r *Recording) Bounds() image.Rectangle {
	if r == nil {
		return image.Rectangle{}
	}
	return r.bounds
}

// HasVisiblePaint reports whether replaying this recording can put a pixel on
// the surface. Command count is not the question: a recording of empty paths,
// zero-alpha fills and brush-less geometry is legal but invisible, and feeding
// one to a texture allocator would spend a surface to show nothing.
// A nil recording paints nothing.
func (r *Recording) HasVisiblePaint() bool {
	if r == nil {
		return false
	}
	return r.visible
}

// PaintOpCount returns the display-list length: the number of commands a
// replay issues work for. State, clip and layer commands are not counted, so
// this is what a per-frame op budget should charge a recording for.
// A nil recording carries no work.
func (r *Recording) PaintOpCount() int {
	if r == nil {
		return 0
	}
	return r.paintOps
}

// NeedsCPUPassScratch reports whether a replay can put pixels on the surface
// through the CPU: it draws from an image buffer, and an image path without a
// GPU resource samples on the CPU. A rasterizer recording the replay into a GPU
// texture must swap in a CPU scratch pixmap for the pass or those pixels miss
// the texture; a recording with no image commands is provably GPU-only and can
// skip the swap at zero behavior change.
// A nil recording needs no scratch.
func (r *Recording) NeedsCPUPassScratch() bool {
	if r == nil {
		return false
	}
	return r.cpuScratch
}

// ImageBufs returns the image buffers the recording draws from, in pool order.
// A retained packet must hold these for as long as it may replay them: the
// recording stores buffers by pointer (no pixel copy), so disposing one while a
// recording still references it leaves a disposed image at playback.
// The result is read-only. A nil recording draws from no buffers.
func (r *Recording) ImageBufs() []*render.ImageBuf {
	if r == nil {
		return nil
	}
	return r.resources.Images()
}

// Width returns the width of the recording canvas.
func (r *Recording) Width() int {
	return r.width
}

// Height returns the height of the recording canvas.
func (r *Recording) Height() int {
	return r.height
}

// Commands returns the recorded commands.
func (r *Recording) Commands() []Command {
	return r.commands
}

// Resources returns the resource pool.
func (r *Recording) Resources() *ResourcePool {
	return r.resources
}

// Playback replays the recording to the given backend.
func (r *Recording) Playback(backend Backend) error {
	// Initialize backend
	if err := backend.Begin(r.width, r.height); err != nil {
		return err
	}

	// Replay each command
	for _, cmd := range r.commands {
		switch c := cmd.(type) {
		case SaveCommand:
			backend.Save()
		case RestoreCommand:
			backend.Restore()
		case ConcatCommand:
			backend.Concat(c.Matrix)
		case SetClipCommand:
			path := r.resources.GetPath(c.Path)
			backend.SetClip(path, c.Rule)
		case ClearClipCommand:
			backend.ClearClip()
		case ClipRectCommand:
			backend.ClipRect(c.Rect)
		case ClipRectOpCommand:
			backend.ClipRectOp(c.Rect, c.Op)
		case ClipPathOpCommand:
			backend.ClipPathOp(r.resources.GetPath(c.Path), c.Rule, c.Op)
		case ClipRoundRectCommand:
			// Convert RRect clip to a path for backends that don't
			// natively support rounded rectangle clipping.
			rrPath := render.BuildPath().RoundRect(c.X, c.Y, c.W, c.H, c.Radius).Build()
			backend.SetClip(rrPath, FillRuleNonZero)
		case FillPathCommand:
			path := r.resources.GetPath(c.Path)
			brush := r.resources.GetBrush(c.Brush)
			backend.FillPath(path, brush, c.Rule)
		case StrokePathCommand:
			path := r.resources.GetPath(c.Path)
			brush := r.resources.GetBrush(c.Brush)
			backend.StrokePath(path, brush, c.Stroke)
		case FillRectCommand:
			brush := r.resources.GetBrush(c.Brush)
			backend.FillRect(c.Rect, brush)
		case StrokeRectCommand:
			brush := r.resources.GetBrush(c.Brush)
			backend.StrokeRect(c.Rect, brush, c.Stroke)
		case DrawImageCommand:
			backend.DrawImage(r.resources.GetImage(c.Image), c.Opts)
		case DrawImageNineCommand:
			backend.DrawImageNine(r.resources.GetImage(c.Image), c.Center, c.Dst)
		case DrawImageQuadCommand:
			backend.DrawImageQuad(r.resources.GetImage(c.Image), c.Corners, c.Opts)
		case DrawImageRoundedCommand:
			backend.DrawImageRounded(r.resources.GetImage(c.Image), c.X, c.Y, c.Radius)
		case DrawImageCircularCommand:
			backend.DrawImageCircular(r.resources.GetImage(c.Image), c.CX, c.CY, c.Radius)
		case DrawAtlasCommand:
			backend.DrawAtlas(r.resources.GetImage(c.Image), c.Sprites, c.Opts)
		case DrawDepthSpritesCommand:
			backend.DrawDepthSprites(r.resources.GetImage(c.Image), c.Sprites, c.Opts)
		case DrawVerticesCommand:
			backend.DrawVertices(c.Positions, c.Colors, c.Mode, r.resources.GetBrush(c.Brush))
		case DrawMeshCommand:
			backend.DrawMesh(render.Mesh{
				Positions: c.Positions,
				Colors:    c.Colors,
				Indices:   c.Indices,
			}, r.resources.GetBrush(c.Brush))
		case DrawGlyphsCommand:
			backend.DrawGlyphs(c.Glyphs, r.faceFor(c.Face), c.X, c.Y, c.Colored, r.resources.GetBrush(c.Brush))
		case SetTextDecorationCommand:
			backend.SetTextDecoration(c.Deco)
		case SetTextModeCommand:
			backend.SetTextMode(c.Mode)
		case SetLCDLayoutCommand:
			backend.SetLCDLayout(c.Layout)
		case SetAntiAliasCommand:
			backend.SetAntiAlias(c.Enabled)
		case PushLayerCommand:
			backend.PushLayer(c.Blend, c.Opacity)
		case PushLayerIsolatedCommand:
			backend.PushLayerIsolated(c.Opacity)
		case PushBackdropLayerCommand:
			backend.PushBackdropLayer(c.Blend, c.Opacity)
		case PopLayerCommand:
			backend.PopLayer()
		case ApplyBlurCommand:
			backend.ApplyBlur(c.Radius)
		case ApplyColorMatrixCommand:
			backend.ApplyColorMatrix(c.Matrix)
		case ApplyDropShadowCommand:
			backend.ApplyDropShadow(c.OffsetX, c.OffsetY, c.BlurRadius, c.Color)
		case ApplyGrayscaleCommand:
			backend.ApplyGrayscale()
		case DrawTextCommand:
			brush := r.resources.GetBrush(c.Brush)
			backend.DrawText(c.Text, c.X, c.Y, r.faceFor(c.Font), brush)
		case StrokeTextCommand:
			brush := r.resources.GetBrush(c.Brush)
			// StrokeText is recorded as a stroke text command.
			// Backends that support text stroking can use the stroke style;
			// others fall back to DrawText (fill) as an approximation.
			backend.DrawText(c.Text, c.X, c.Y, r.faceFor(c.Font), brush)
		// Style commands are handled by the backend's internal state
		// during the actual drawing operations
		case SetFillStyleCommand, SetStrokeStyleCommand,
			SetLineWidthCommand, SetLineCapCommand, SetLineJoinCommand,
			SetMiterLimitCommand, SetDashCommand, SetFillRuleCommand:
			// These are state commands that were recorded but the actual
			// style is captured in the drawing commands themselves
		}
	}

	return backend.End()
}

// faceFor resolves a recorded font reference. InvalidRef means the recording
// was made without SetFont; backends then receive nil and may no-op.
func (r *Recording) faceFor(ref FontRef) text.Face {
	if !ref.IsValid() {
		return nil
	}
	return r.resources.GetFont(ref)
}

// --------------------------------------------------------------------------
// Dimensions
// --------------------------------------------------------------------------

// Width returns the width of the recording canvas.
func (r *Recorder) Width() int {
	return r.width
}

// Height returns the height of the recording canvas.
func (r *Recorder) Height() int {
	return r.height
}
