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

import ()

func (c *Context) takeBrushBootstrapIfAny() {
	rc := c.gpuCtxOps()
	if rc == nil {
		return
	}
	if r := rc.TakeBrushBootstrapReason(); r != "" {
		c.pathStats.BrushBootstrapOps++
		c.pathStats.LastBrushBootstrapReason = r
	}
}

// recordCPUFallbackReason increments the CPU fallback counter and records a short reason.
// Reason is best-effort diagnostic for soak/leak tools (last reason wins).
// A fallback during a scratch swap lands pixels in scratch — mark it dirty
// so Commit uploads (and Begin pre-clears) instead of skipping.
func (c *Context) SetTextMode(mode TextMode) {
	c.textMode = mode
}

// TextMode returns the current text rendering strategy.
// TextMode returns the current text rendering strategy.
func (c *Context) TextMode() TextMode {
	return c.textMode
}

// SetLCDLayout sets the LCD subpixel layout for ClearType text rendering.
// Use LCDLayoutRGB for most monitors, LCDLayoutBGR for rare BGR panels,
// or LCDLayoutNone to disable subpixel rendering (grayscale, the default).
//
// When a GPU accelerator is registered and implements LCDLayoutAware,
// the layout is propagated so the glyph mask engine rasterizes glyphs
// with 3x horizontal oversampling and the GPU uses the LCD fragment shader.
//
// The setting is per-Context. Call this before drawing text.
// setupGPUMask uploads the active alpha mask to the GPU accelerator.
// The mask remains bound until ClearMask/SetMask(nil) or a replacement
// SetMaskTexture — it must survive until FlushGPU so cover-inline R8 sampling
// (L.06) can bind the texture during RecordDraws.
//
// Returns a no-op cleanup for call-site defer compatibility.
func (c *Context) setupGPUMask() (func(), error) {
	a := Accelerator()
	if a == nil {
		return func() {}, nil
	}
	ma, ok := a.(MaskAware)
	if !ok {
		// paint.MaskCoverage still drives fillMaskedAsImage bootstrap (L.06).
		return func() {}, nil
	}
	if c.mask == nil {
		ma.ClearMaskTexture()
		return func() {}, nil
	}
	ma.SetMaskTexture(c.mask.Data(), c.mask.Width(), c.mask.Height())
	return func() {}, nil
}

// tryGPUOpRC routes GPU operations through the per-context GPURenderContext.
// sdfAccelForShape maps a shape kind to its SDF acceleration capability.
func sdfAccelForShape(kind ShapeKind) AcceleratedOp {
	switch kind {
	case ShapeCircle, ShapeEllipse, ShapeArc:
		return AccelCircleSDF
	case ShapeRect, ShapeRRect:
		return AccelRRectSDF
	default:
		return 0
	}
}

// doFill performs the fill operation respecting the current RasterizerMode.
