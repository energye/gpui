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
// All drawing semantics live in recording.ContextBackend — this package owns
// only the image-sized context and the PNG/pixmap output, so the raster and
// the live-context playback paths cannot drift apart.
//
// # Limitations
//
// Gradient brushes (linear, radial, sweep) are translated to render brush
// types, but the software renderer currently only supports solid colors.
// Gradients render as black until render adds software gradient support; on a
// GPU-backed context they work.
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
)

func init() {
	recording.Register("raster", func() recording.Backend {
		return NewBackend()
	})
}

// Backend renders recordings to a pixel image. It owns a render.Context sized
// to the recording and delegates every drawing operation to
// recording.ContextBackend.
type Backend struct {
	*recording.ContextBackend
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

// Begin initializes an offscreen context at the recording's dimensions and
// prepares the delegated playback onto it.
func (b *Backend) Begin(width, height int) error {
	b.width = width
	b.height = height
	b.ctx = render.NewContext(width, height)
	b.ContextBackend = recording.NewContextBackend(b.ctx)
	return b.ContextBackend.Begin(width, height)
}

// End finalizes the rendering.
// After End is called, output methods (WriteTo, SaveToFile) can be used.
func (b *Backend) End() error {
	return b.ContextBackend.End()
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
	return render.FromImage(b.ctx.Image())
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
