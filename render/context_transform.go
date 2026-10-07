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
	"math"

)

// DeviceScale returns the device scale factor (physical pixels per logical pixel).
// Default is 1.0. On Retina/HiDPI displays, typical values are 2.0 or 3.0.
func (c *Context) DeviceScale() float64 {
	return math.Float64frombits(c.deviceScale.Load())
}

// SetDeviceScale changes the device scale factor on an existing context.
// This reallocates the internal pixmap at the new physical resolution
// and adjusts the base transform. The logical dimensions (Width, Height)
// remain unchanged.
//
// Use this when the window moves to a display with a different scale factor.
// Scale must be > 0; values <= 0 are ignored.
// SetDeviceScale changes the device scale factor on an existing context.
// This reallocates the internal pixmap at the new physical resolution
// and adjusts the base transform. The logical dimensions (Width, Height)
// remain unchanged.
//
// Use this when the window moves to a display with a different scale factor.
// Scale must be > 0; values <= 0 are ignored.
func (c *Context) SetDeviceScale(scale float64) {
	if scale <= 0 || scale == math.Float64frombits(c.deviceScale.Load()) {
		return
	}

	oldScale := math.Float64frombits(c.deviceScale.Load())
	c.deviceScale.Store(math.Float64bits(scale))

	// Physical dimensions
	pw := int(float64(c.width) * scale)
	ph := int(float64(c.height) * scale)

	Logger().Info("SetDeviceScale",
		"old_scale", oldScale, "new_scale", scale,
		"logical_w", c.width, "logical_h", c.height,
		"physical_w", pw, "physical_h", ph,
	)

	// Reallocate pixmap at new physical resolution
	c.pixmap = NewPixmap(pw, ph)

	// Update renderer dimensions and device scale
	if sr, ok := c.renderer.(*SoftwareRenderer); ok {
		sr.Resize(pw, ph)
		sr.SetDeviceScale(float32(scale))
	}

	// Update device matrix. User matrix (c.matrix) is NOT touched —
	// it contains only user transforms and is independent of device scale.
	c.deviceMatrix = Identity()
	if scale != 1.0 {
		c.deviceMatrix = Scale(scale, scale)
	}

	// Reset clip stack (clip regions are in pixel coordinates)
	c.clipStack = nil
	c.gpuClipPath = nil
	c.ClearPath()
}

// Image returns the context's image.
// Pending GPU commands are flushed first so readback matches SavePNG semantics
// (CPU pixmap must include GPU-rendered content).
// Identity resets the user transformation matrix to the identity matrix.
// Device scale is applied separately at rendering boundaries (not in the CTM),
// so Identity() always resets to a pure identity matrix regardless of scale.
func (c *Context) Identity() {
	c.matrix = Identity()
}

// Translate applies a translation to the transformation matrix.
// Translate applies a translation to the transformation matrix.
func (c *Context) Translate(x, y float64) {

	c.matrix = c.matrix.Multiply(Translate(x, y))
}

// Scale applies a scaling transformation.
// Scale applies a scaling transformation.
func (c *Context) Scale(x, y float64) {

	c.matrix = c.matrix.Multiply(Scale(x, y))
}

// Rotate applies a rotation (angle in radians).
// Rotate applies a rotation (angle in radians).
func (c *Context) Rotate(angle float64) {

	c.matrix = c.matrix.Multiply(Rotate(angle))
}

// RotateAbout rotates around a specific point.
// RotateAbout rotates around a specific point.
func (c *Context) RotateAbout(angle, x, y float64) {
	c.Translate(x, y)
	c.Rotate(angle)
	c.Translate(-x, -y)
}

// Shear applies a shear transformation.
// Shear applies a shear transformation.
func (c *Context) Shear(x, y float64) {
	c.matrix = c.matrix.Multiply(Shear(x, y))
}

// Transform multiplies the current transformation matrix by the given matrix.
// This is similar to CanvasRenderingContext2D.transform() in web browsers.
// The transformation is applied in the order: current * m.
// Transform multiplies the current transformation matrix by the given matrix.
// This is similar to CanvasRenderingContext2D.transform() in web browsers.
// The transformation is applied in the order: current * m.
func (c *Context) Transform(m Matrix) {
	c.matrix = c.matrix.Multiply(m)
}

// SetTransform replaces the current transformation matrix with the given matrix.
// This is similar to CanvasRenderingContext2D.setTransform() in web browsers.
// Unlike Transform, this completely replaces the matrix rather than multiplying.
// SetTransform replaces the current transformation matrix with the given matrix.
// This is similar to CanvasRenderingContext2D.setTransform() in web browsers.
// Unlike Transform, this completely replaces the matrix rather than multiplying.
func (c *Context) SetTransform(m Matrix) {
	c.matrix = m
}

// GetTransform returns a copy of the current transformation matrix.
// This is similar to CanvasRenderingContext2D.getTransform() in web browsers.
// The returned matrix is a copy, so modifying it will not affect the context.
// GetTransform returns a copy of the current transformation matrix.
// This is similar to CanvasRenderingContext2D.getTransform() in web browsers.
// The returned matrix is a copy, so modifying it will not affect the context.
func (c *Context) GetTransform() Matrix {
	return c.matrix
}

// TransformPoint transforms a point by the current matrix.
// TransformPoint transforms a point by the current matrix.
func (c *Context) TransformPoint(x, y float64) (float64, float64) {
	p := c.matrix.TransformPoint(Pt(x, y))
	return p.X, p.Y
}

// InvertY inverts the Y axis (useful for coordinate system changes).
// Uses logical height so the inversion works correctly at any device scale.
// InvertY inverts the Y axis (useful for coordinate system changes).
// Uses logical height so the inversion works correctly at any device scale.
func (c *Context) InvertY() {
	c.Translate(0, float64(c.height))
	c.Scale(1, -1)
}

// totalMatrix returns the combined device + user transform matrix.
// Used at rendering boundaries where device-space coordinates are needed.
// At scale=1.0, this is identical to c.matrix (zero overhead).
// totalMatrix returns the combined device + user transform matrix.
// Used at rendering boundaries where device-space coordinates are needed.
// At scale=1.0, this is identical to c.matrix (zero overhead).
func (c *Context) totalMatrix() Matrix {
	if c.deviceMatrix.IsIdentity() {
		return c.matrix
	}
	return c.deviceMatrix.Multiply(c.matrix)
}

// deviceSpacePath returns the current path transformed to device-space.
// Path coordinates are in user-space (transformed by c.matrix only).
// The renderer operates in device-space, so we apply deviceMatrix here.
// At scale=1.0, returns the original path (zero copy).
