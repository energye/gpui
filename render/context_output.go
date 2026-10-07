//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"math"

)

// Width returns the logical width of the context.
// This is the coordinate space used by drawing operations.
// For the physical pixel dimensions, use PixelWidth.
func (c *Context) Width() int {
	return c.width
}

// Height returns the logical height of the context.
// This is the coordinate space used by drawing operations.
// For the physical pixel dimensions, use PixelHeight.
// Height returns the logical height of the context.
// This is the coordinate space used by drawing operations.
// For the physical pixel dimensions, use PixelHeight.
func (c *Context) Height() int {
	return c.height
}

// PixelWidth returns the physical pixel width of the internal pixmap.
// This equals Width() * DeviceScale(), rounded to int.
// On non-HiDPI displays (scale=1.0), this equals Width().
// PixelWidth returns the physical pixel width of the internal pixmap.
// This equals Width() * DeviceScale(), rounded to int.
// On non-HiDPI displays (scale=1.0), this equals Width().
func (c *Context) PixelWidth() int {
	return int(float64(c.width) * math.Float64frombits(c.deviceScale.Load()))
}

// PixelHeight returns the physical pixel height of the internal pixmap.
// This equals Height() * DeviceScale(), rounded to int.
// On non-HiDPI displays (scale=1.0), this equals Height().
// PixelHeight returns the physical pixel height of the internal pixmap.
// This equals Height() * DeviceScale(), rounded to int.
// On non-HiDPI displays (scale=1.0), this equals Height().
func (c *Context) PixelHeight() int {
	return int(float64(c.height) * math.Float64frombits(c.deviceScale.Load()))
}

// DeviceScale returns the device scale factor (physical pixels per logical pixel).
// Default is 1.0. On Retina/HiDPI displays, typical values are 2.0 or 3.0.
// SavePNG saves the context to a PNG file.
func (c *Context) SavePNG(path string) error {
	_ = c.FlushGPU() // Flush pending GPU shapes before reading pixels.
	_ = c.syncViewFlushIntoPixmap()
	_ = c.materializeFilterGPU()
	return c.pixmap.SavePNG(path)
}

// Clear resets the entire context to transparent (zero alpha).
// To fill with a specific background color, use [ClearWithColor].
// EncodePNG writes the image as PNG to the given writer.
// This is useful for streaming, network output, or custom storage.
func (c *Context) EncodePNG(w io.Writer) error {
	return png.Encode(w, c.Image())
}

// EncodeJPEG writes the image as JPEG with the given quality (1-100).
// EncodeJPEG writes the image as JPEG with the given quality (1-100).
func (c *Context) EncodeJPEG(w io.Writer, quality int) error {
	return jpeg.Encode(w, c.Image(), &jpeg.Options{Quality: quality})
}

// Resize changes the context logical dimensions, reusing internal buffers where possible.
// If the dimensions haven't changed, this is a no-op.
// Returns an error if width or height is <= 0.
//
// The width and height are logical dimensions. The internal pixmap is
// allocated at physical resolution (width*deviceScale x height*deviceScale).
//
// After Resize:
//   - The pixmap is reallocated only if dimensions changed
//   - The clip region is reset to the full rectangle
//   - The transformation matrix is preserved (Push/Pop stack is preserved)
//   - The current path is cleared
//
// This method is useful for UI frameworks that need to resize the canvas
// when the window size changes, without creating a new Context.
// Resize changes the context logical dimensions, reusing internal buffers where possible.
// If the dimensions haven't changed, this is a no-op.
// Returns an error if width or height is <= 0.
//
// The width and height are logical dimensions. The internal pixmap is
// allocated at physical resolution (width*deviceScale x height*deviceScale).
//
// After Resize:
//   - The pixmap is reallocated only if dimensions changed
//   - The clip region is reset to the full rectangle
//   - The transformation matrix is preserved (Push/Pop stack is preserved)
//   - The current path is cleared
//
// This method is useful for UI frameworks that need to resize the canvas
// when the window size changes, without creating a new Context.
func (c *Context) Resize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid dimensions: width=%d, height=%d (both must be > 0)", width, height)
	}

	// No-op if dimensions haven't changed
	if c.width == width && c.height == height {
		return nil
	}

	// Update logical dimensions
	c.width = width
	c.height = height

	// Physical dimensions
	pw := int(float64(width) * math.Float64frombits(c.deviceScale.Load()))
	ph := int(float64(height) * math.Float64frombits(c.deviceScale.Load()))

	// Reallocate pixmap at physical resolution
	c.pixmap = NewPixmap(pw, ph)

	// Single-slot replacement: the context only
	// ever requests layers at the current window size, so stale sizes held by
	// the layer pool are dropped at once instead of being retained forever.
	if c.layerStack != nil {
		c.layerStack.pool.EvictExcept(width, height)
	}

	// Resize renderer if it supports resizing
	if sr, ok := c.renderer.(*SoftwareRenderer); ok {
		sr.Resize(pw, ph)
	}

	// Reset clip stack to full rectangle
	c.clipStack = nil
	c.gpuClipPath = nil

	// Clear any existing path
	c.ClearPath()

	return nil
}

// ResizeTarget returns the underlying pixmap for resize operations.
// This is primarily used by renderers and advanced users who need
// direct access to the target buffer during resize operations.
// ResizeTarget returns the underlying pixmap for resize operations.
// This is primarily used by renderers and advanced users who need
// direct access to the target buffer during resize operations.
func (c *Context) ResizeTarget() *Pixmap {
	return c.pixmap
}

// FlushGPU flushes any pending GPU accelerator operations to the pixel buffer.
// Call this before reading pixel data (e.g., SavePNG, Image) when using a
// batch-capable GPU accelerator. For immediate-mode accelerators this is a no-op.

// SetSharedEncoder sets a shared command encoder for single-command-buffer
// frames (ADR-017, Flutter Impeller pattern). When set, FlushGPU/FlushGPUWithView
// record render passes into this encoder instead of creating their own and
// submitting. The caller is responsible for encoder.Finish() + queue.Submit().
//
// Pass a zero-value CommandEncoder (IsNil() == true) to restore normal
// per-context submit behavior.
