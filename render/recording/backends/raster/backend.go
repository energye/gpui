//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package raster provides a raster backend for the recording system.
// It renders recordings to pixel images using render.Context.
//
// The raster backend serves multiple purposes:
//   - Architecture validation for the recording system
//   - Reference implementation for other backends
//   - Pixel-accurate comparison testing
//   - Caching/buffering of drawing operations
//
// # Supported Features
//
//   - Solid color fills and strokes
//   - Path operations (fill, stroke, clip)
//   - Transform matrix
//   - Stroke styling (width, cap, join, dash patterns)
//   - State management (Save/Restore)
//   - PNG output
//
// # Limitations
//
// Gradient brushes (linear, radial, sweep) are correctly translated to gg
// brush types, but the underlying render.SoftwareRenderer currently only supports
// solid colors. Gradients will render as black until the gg library implements
// gradient support in its software renderer.
//
// # Example
//
//	// Import to register the backend
//	import _ "github.com/energye/gpui/render/recording/backends/raster"
//
//	// Create via registry
//	backend, _ := recording.NewBackend("raster")
//
//	// Or create directly
//	backend := raster.NewBackend()
//
//	// Playback recording
//	rec.Playback(backend)
//
//	// Get output
//	backend.SavePNG("output.png")
//	img := backend.Image()
package raster

import (
	"image"
	"image/png"
	"io"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
)

func init() {
	recording.Register("raster", func() recording.Backend {
		return NewBackend()
	})
}

// Backend renders recordings to a pixel image using render.Context.
// It implements recording.Backend, recording.WriterBackend,
// recording.FileBackend, and recording.PixmapBackend interfaces.
type Backend struct {
	ctx    *render.Context
	width  int
	height int
}

// Ensure Backend implements all required interfaces.
var (
	_ recording.Backend       = (*Backend)(nil)
	_ recording.WriterBackend = (*Backend)(nil)
	_ recording.FileBackend   = (*Backend)(nil)
	_ recording.PixmapBackend = (*Backend)(nil)
)

// NewBackend creates a new raster backend.
// The backend must be initialized with Begin before use.
func NewBackend() *Backend {
	return &Backend{}
}

// Begin initializes the backend for rendering at the given dimensions.
// This must be called before any drawing operations.
func (b *Backend) Begin(width, height int) error {
	b.width = width
	b.height = height
	b.ctx = render.NewContext(width, height)
	return nil
}

// End finalizes the rendering.
// After End is called, output methods (WriteTo, SaveToFile) can be used.
func (b *Backend) End() error {
	return nil
}

// Save saves the current graphics state onto a stack.
func (b *Backend) Save() {
	b.ctx.Push()
}

// Restore restores the graphics state from the stack.
func (b *Backend) Restore() {
	b.ctx.Pop()
}

// SetTransform sets the current transformation matrix.
func (b *Backend) SetTransform(m recording.Matrix) {
	// Convert recording.Matrix to render.Matrix
	b.ctx.SetTransform(render.Matrix{
		A: m.A, B: m.B, C: m.C,
		D: m.D, E: m.E, F: m.F,
	})
}

// SetClip sets the clipping region to the given path.
func (b *Backend) SetClip(path *render.Path, rule recording.FillRule) {
	if path == nil {
		return
	}

	// Recording paths are already in world coordinates; clip through the
	// context's real clip stack instead of merely leaving the path loaded.
	b.setPath(path)
	b.ctx.SetFillRule(convertFillRule(rule))
	b.ctx.Clip()
}

// ClearClip removes any clipping region.
func (b *Backend) ClearClip() {
	b.ctx.ResetClip()
}

// FillPath fills the given path with the brush.
func (b *Backend) FillPath(path *render.Path, brush recording.Brush, rule recording.FillRule) {
	if path == nil {
		return
	}

	b.applyBrush(brush, true)
	b.ctx.SetFillRule(convertFillRule(rule))
	b.setPath(path)
	_ = b.ctx.Fill()
}

// StrokePath strokes the given path with the brush and stroke style.
func (b *Backend) StrokePath(path *render.Path, brush recording.Brush, stroke recording.Stroke) {
	if path == nil {
		return
	}

	b.applyBrush(brush, false)
	b.applyStroke(stroke)
	b.setPath(path)
	_ = b.ctx.Stroke()
}

// FillRect fills a rectangle with the brush.
// The rect coordinates are in world space (already transformed during recording).
func (b *Backend) FillRect(rect recording.Rect, brush recording.Brush) {
	b.applyBrush(brush, true)
	// Reset transform since rect is already in world coordinates
	b.ctx.Identity()
	b.ctx.DrawRectangle(rect.MinX, rect.MinY, rect.Width(), rect.Height())
	_ = b.ctx.Fill()
}

// DrawImage draws an image from source to destination rectangle.
func (b *Backend) DrawImage(img image.Image, src, dst recording.Rect, opts recording.ImageOptions) {
	if img == nil || dst.IsEmpty() {
		return
	}

	srcW := src.Width()
	srcH := src.Height()
	if srcW <= 0 || srcH <= 0 {
		bounds := img.Bounds()
		src = recording.NewRect(0, 0, float64(bounds.Dx()), float64(bounds.Dy()))
		srcW = src.Width()
		srcH = src.Height()
	}
	if srcW <= 0 || srcH <= 0 {
		return
	}

	// Recording destinations are already in world coordinates.
	b.ctx.Identity()

	bounds := img.Bounds()
	// ImageBufFromImage normalizes the image to a 0,0 origin, while the
	// recording's src rect is image-relative. Do not add Bounds().Min here;
	// a sub-image would otherwise sample from the wrong pixels.
	srcRect := image.Rect(
		int(src.MinX),
		int(src.MinY),
		int(src.MaxX),
		int(src.MaxY),
	).Intersect(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	if srcRect.Empty() {
		return
	}

	interp := render.InterpBilinear
	if opts.Interpolation == recording.InterpolationNearest {
		interp = render.InterpNearest
	}
	opacity := opts.Alpha
	if opacity <= 0 {
		// DrawImageEx treats zero as "unspecified" and uses 1.0; the
		// recording contract treats zero as fully transparent.
		return
	}
	if opacity > 1 {
		opacity = 1
	}

	b.ctx.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
		X:             dst.MinX,
		Y:             dst.MinY,
		DstWidth:      dst.Width(),
		DstHeight:     dst.Height(),
		SrcRect:       &srcRect,
		Interpolation: interp,
		Opacity:       opacity,
	})
}

// DrawText draws text at the given position with the specified font face and brush.
func (b *Backend) DrawText(s string, x, y float64, face text.Face, brush recording.Brush) {
	if s == "" || face == nil {
		return
	}

	// Recording coordinates already include the recorded transform.
	b.ctx.Push()
	defer b.ctx.Pop()
	b.ctx.Identity()
	b.applyBrush(brush, true)
	b.ctx.SetFont(face)
	b.ctx.DrawString(s, x, y)
}

// WriteTo writes the rendered content as PNG to the given writer.
func (b *Backend) WriteTo(w io.Writer) (int64, error) {
	cw := &countingWriter{w: w}
	err := png.Encode(cw, b.ctx.Image())
	return cw.n, err
}

// SaveToFile saves the rendered content as PNG to a file.
func (b *Backend) SaveToFile(path string) error {
	return b.ctx.SavePNG(path)
}

// Pixmap returns the rendered pixmap.
func (b *Backend) Pixmap() *render.Pixmap {
	// render.Context doesn't expose Pixmap directly
	// We need to convert from image
	img := b.ctx.Image()
	return render.FromImage(img)
}

// Image returns the rendered image.
func (b *Backend) Image() image.Image {
	return b.ctx.Image()
}

// SavePNG is a convenience method to save the image as PNG.
func (b *Backend) SavePNG(path string) error {
	return b.ctx.SavePNG(path)
}

// Width returns the backend width.
func (b *Backend) Width() int {
	return b.width
}

// Height returns the backend height.
func (b *Backend) Height() int {
	return b.height
}

// setPath sets the path on the context with identity transform.
// The path is assumed to be already in world coordinates.
func (b *Backend) setPath(path *render.Path) {
	b.ctx.Identity()
	b.ctx.ClearPath()
	b.setPathFromElements(path)
}

// setPathFromElements walks path verbs/coords and adds them to the context.
func (b *Backend) setPathFromElements(path *render.Path) {
	path.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			b.ctx.MoveTo(coords[0], coords[1])
		case render.LineTo:
			b.ctx.LineTo(coords[0], coords[1])
		case render.QuadTo:
			b.ctx.QuadraticTo(coords[0], coords[1], coords[2], coords[3])
		case render.CubicTo:
			b.ctx.CubicTo(coords[0], coords[1], coords[2], coords[3], coords[4], coords[5])
		case render.Close:
			b.ctx.ClosePath()
		}
	})
}

// applyBrush applies the recording brush to the context.
func (b *Backend) applyBrush(brush recording.Brush, fill bool) {
	if brush == nil {
		return
	}

	switch br := brush.(type) {
	case recording.SolidBrush:
		ggBrush := render.Solid(br.Color)
		if fill {
			b.ctx.SetFillBrush(ggBrush)
		} else {
			b.ctx.SetStrokeBrush(ggBrush)
		}

	case *recording.LinearGradientBrush:
		grad := render.NewLinearGradientBrush(br.Start.X, br.Start.Y, br.End.X, br.End.Y)
		for _, stop := range br.Stops {
			grad.AddColorStop(stop.Offset, stop.Color)
		}
		grad.SetExtend(render.ExtendMode(br.Extend))
		if fill {
			b.ctx.SetFillBrush(grad)
		} else {
			b.ctx.SetStrokeBrush(grad)
		}

	case *recording.RadialGradientBrush:
		grad := render.NewRadialGradientBrush(br.Center.X, br.Center.Y, br.StartRadius, br.EndRadius)
		grad.SetFocus(br.Focus.X, br.Focus.Y)
		for _, stop := range br.Stops {
			grad.AddColorStop(stop.Offset, stop.Color)
		}
		grad.SetExtend(render.ExtendMode(br.Extend))
		if fill {
			b.ctx.SetFillBrush(grad)
		} else {
			b.ctx.SetStrokeBrush(grad)
		}

	case *recording.SweepGradientBrush:
		grad := render.NewSweepGradientBrush(br.Center.X, br.Center.Y, br.StartAngle)
		grad.SetEndAngle(br.EndAngle)
		for _, stop := range br.Stops {
			grad.AddColorStop(stop.Offset, stop.Color)
		}
		grad.SetExtend(render.ExtendMode(br.Extend))
		if fill {
			b.ctx.SetFillBrush(grad)
		} else {
			b.ctx.SetStrokeBrush(grad)
		}

	default:
		// Fallback to black
		if fill {
			b.ctx.SetFillBrush(render.Solid(render.Black))
		} else {
			b.ctx.SetStrokeBrush(render.Solid(render.Black))
		}
	}
}

// applyStroke applies the stroke settings to the context.
func (b *Backend) applyStroke(stroke recording.Stroke) {
	b.ctx.SetLineWidth(stroke.Width)
	b.ctx.SetLineCap(convertLineCap(stroke.Cap))
	b.ctx.SetLineJoin(convertLineJoin(stroke.Join))
	b.ctx.SetMiterLimit(stroke.MiterLimit)

	if len(stroke.DashPattern) > 0 {
		b.ctx.SetDash(stroke.DashPattern...)
		b.ctx.SetDashOffset(stroke.DashOffset)
	} else {
		b.ctx.ClearDash()
	}
}

// convertFillRule converts recording.FillRule to render.FillRule.
func convertFillRule(rule recording.FillRule) render.FillRule {
	switch rule {
	case recording.FillRuleEvenOdd:
		return render.FillRuleEvenOdd
	default:
		return render.FillRuleNonZero
	}
}

// convertLineCap converts recording.LineCap to render.LineCap.
func convertLineCap(lineCap recording.LineCap) render.LineCap {
	switch lineCap {
	case recording.LineCapRound:
		return render.LineCapRound
	case recording.LineCapSquare:
		return render.LineCapSquare
	default:
		return render.LineCapButt
	}
}

// convertLineJoin converts recording.LineJoin to render.LineJoin.
func convertLineJoin(join recording.LineJoin) render.LineJoin {
	switch join {
	case recording.LineJoinRound:
		return render.LineJoinRound
	case recording.LineJoinBevel:
		return render.LineJoinBevel
	default:
		return render.LineJoinMiter
	}
}

// countingWriter wraps an io.Writer and counts bytes written.
type countingWriter struct {
	w io.Writer
	n int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}
