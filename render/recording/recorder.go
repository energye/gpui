package recording

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// Recorder captures drawing operations as commands.
// It mirrors the gg.Context drawing API but generates commands
// instead of rasterizing pixels. Use FinishRecording to obtain
// an immutable Recording that can be replayed to different backends.
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

	// Current path being built
	currentPath *render.Path

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
		stateStack:  make([]recorderState, 0, 8),
	}
}

// FinishRecording returns an immutable Recording containing all recorded commands.
// After calling FinishRecording, the Recorder should not be used again.
func (r *Recorder) FinishRecording() *Recording {
	return &Recording{
		width:     r.width,
		height:    r.height,
		commands:  r.commands,
		resources: r.resources,
	}
}

// Recording is an immutable container for recorded drawing commands.
// It can be replayed to any Backend implementation.
type Recording struct {
	width, height int
	commands      []Command
	resources     *ResourcePool
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
		case SetTransformCommand:
			backend.SetTransform(c.Matrix)
		case SetClipCommand:
			path := r.resources.GetPath(c.Path)
			backend.SetClip(path, c.Rule)
		case ClearClipCommand:
			backend.ClearClip()
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
		case DrawImageCommand:
			img := r.resources.GetImage(c.Image)
			backend.DrawImage(img, c.SrcRect, c.DstRect, c.Options)
		case DrawTextCommand:
			brush := r.resources.GetBrush(c.Brush)
			// Font face lookup would need additional handling
			backend.DrawText(c.Text, c.X, c.Y, nil, brush)
		case StrokeTextCommand:
			brush := r.resources.GetBrush(c.Brush)
			// StrokeText is recorded as a stroke text command.
			// Backends that support text stroking can use the stroke style;
			// others fall back to DrawText (fill) as an approximation.
			backend.DrawText(c.Text, c.X, c.Y, nil, brush)
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
