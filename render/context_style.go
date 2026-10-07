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
	"image/color"
)

func (c *Context) SetPipelineMode(mode PipelineMode) {
	c.pipelineMode = mode
	if rc := c.gpuCtxOps(); rc != nil {
		rc.SetPipelineMode(mode)
	} else if a := Accelerator(); a != nil {
		if pma, ok := a.(PipelineModeAware); ok {
			pma.SetPipelineMode(mode)
		}
	}
}

// PipelineMode returns the current pipeline mode.
// PipelineMode returns the current pipeline mode.
func (c *Context) PipelineMode() PipelineMode {
	return c.pipelineMode
}

// SetRasterizerMode sets the rasterization strategy for this context.
// RasterizerAuto (default) uses intelligent auto-selection based on path
// complexity, bounding box area, and shape type.
// Other modes force a specific algorithm, bypassing auto-selection.
//
// The mode is per-Context — different contexts can use different strategies.
// SetRasterizerMode sets the rasterization strategy for this context.
// RasterizerAuto (default) uses intelligent auto-selection based on path
// complexity, bounding box area, and shape type.
// Other modes force a specific algorithm, bypassing auto-selection.
//
// The mode is per-Context — different contexts can use different strategies.
func (c *Context) SetRasterizerMode(mode RasterizerMode) {
	c.rasterizerMode = mode
}

// RasterizerMode returns the current rasterizer mode.
// RasterizerMode returns the current rasterizer mode.
func (c *Context) RasterizerMode() RasterizerMode {
	return c.rasterizerMode
}

// SetAntiAlias enables or disables anti-aliasing for geometry rendering.
//
// When enabled (default), shapes are rendered with smooth edges using analytic
// anti-aliasing. When disabled, shapes are rendered with binary
// coverage (fully inside or fully outside) producing crisp, aliased edges.
//
// This is useful for pixel art, retro-style graphics, technical drawings,
// and any use case where sub-pixel blending is undesirable.
//
// Text anti-aliasing is controlled independently via SetTextMode.
// The anti-aliasing state participates in Push/Pop.
// SetAntiAlias enables or disables anti-aliasing for geometry rendering.
//
// When enabled (default), shapes are rendered with smooth edges using analytic
// anti-aliasing. When disabled, shapes are rendered with binary
// coverage (fully inside or fully outside) producing crisp, aliased edges.
//
// This is useful for pixel art, retro-style graphics, technical drawings,
// and any use case where sub-pixel blending is undesirable.
//
// Text anti-aliasing is controlled independently via SetTextMode.
// The anti-aliasing state participates in Push/Pop.
func (c *Context) SetAntiAlias(enabled bool) {
	c.antiAlias = enabled
}

// AntiAlias returns whether anti-aliasing is enabled for geometry rendering.
// AntiAlias returns whether anti-aliasing is enabled for geometry rendering.
func (c *Context) AntiAlias() bool {
	return c.antiAlias
}

// SetTextMode sets the text rendering strategy.
// See TextMode constants for available strategies.
//
// The mode is per-Context — different contexts can use different strategies.

// SetEffectSurface marks this Context as a continuous effect offscreen (glow,
// blur tile, saveLayer-like RT). Forces 1x GPU samples so every-frame flushes
// skip 4x MSAA allocate/resolve — Skia-class for small filtered RTs.
// Main/window contexts should leave this false for geometry AA quality.
// SetEffectSurface marks this Context as a continuous effect offscreen (glow,
// blur tile, saveLayer-like RT). Forces 1x GPU samples so every-frame flushes
// skip 4x MSAA allocate/resolve — Skia-class for small filtered RTs.
// Main/window contexts should leave this false for geometry AA quality.
func (c *Context) SetEffectSurface(enabled bool) {
	if c == nil {
		return
	}
	c.effectSurface = enabled
	c.ensureGPUCtx()
	if rc := c.gpuCtxOps(); rc != nil {
		type sc1 interface{ SetPreferSampleCount1(bool) }
		if s, ok := rc.(sc1); ok {
			s.SetPreferSampleCount1(enabled)
		}
	}
}

// acquireEffectPublishView allocates a pooled TextureBinding RT for F14 effect
// FlushGPU publish. Caller must pass release to attachFilterGPUResult (or call it).
// SetLCDLayout sets the LCD subpixel layout for ClearType text rendering.
// Use LCDLayoutRGB for most monitors, LCDLayoutBGR for rare BGR panels,
// or LCDLayoutNone to disable subpixel rendering (grayscale, the default).
//
// When a GPU accelerator is registered and implements LCDLayoutAware,
// the layout is propagated so the glyph mask engine rasterizes glyphs
// with 3x horizontal oversampling and the GPU uses the LCD fragment shader.
//
// The setting is per-Context. Call this before drawing text.
func (c *Context) SetLCDLayout(layout LCDLayout) {
	c.lcdLayout = layout
	a := Accelerator()
	if a == nil {
		return
	}
	if la, ok := a.(LCDLayoutAware); ok {
		la.SetLCDLayout(layout)
	}
}

// Width returns the logical width of the context.
// This is the coordinate space used by drawing operations.
// For the physical pixel dimensions, use PixelWidth.
// SetColor sets the current drawing color.
//
// New code prefers Solid* + SetFillBrush.
func (c *Context) SetColor(col color.Color) {
	c.paint.solidColor = FromColor(col)
	c.paint.isSolid = true
	c.paint.Brush = nil
	c.paint.Pattern = nil
}

// SetRGB sets the current color using RGB values (0-1).
// SetRGB sets the current color using RGB values (0-1).
func (c *Context) SetRGB(r, g, b float64) {
	c.paint.solidColor = RGBA{R: r, G: g, B: b, A: 1}
	c.paint.isSolid = true
	c.paint.Brush = nil
	c.paint.Pattern = nil
}

// SetRGBA sets the current color using RGBA values (0-1).
// SetRGBA sets the current color using RGBA values (0-1).
func (c *Context) SetRGBA(r, g, b, a float64) {
	c.paint.solidColor = RGBA{R: r, G: g, B: b, A: a}
	c.paint.isSolid = true
	c.paint.Brush = nil
	c.paint.Pattern = nil
}

// SetHexColor sets the current color using a hex string.
// SetHexColor sets the current color using a hex string.
func (c *Context) SetHexColor(hex string) {
	c.paint.solidColor = Hex(hex)
	c.paint.isSolid = true
	c.paint.Brush = nil
	c.paint.Pattern = nil
}

// SetFillBrush sets the brush used for fill operations.
// This is the preferred way to set fill styling in new code.
//
// Example:
//
//	ctx.SetFillBrush(render.Solid(render.Red))
//	ctx.SetFillBrush(render.SolidHex("#FF5733"))
//	ctx.SetFillBrush(render.HorizontalGradient(render.Red, render.Blue, 0, 100))
//
// SetFillBrush sets the brush used for fill operations.
// This is the preferred way to set fill styling in new code.
//
// Example:
//
//	ctx.SetFillBrush(render.Solid(render.Red))
//	ctx.SetFillBrush(render.SolidHex("#FF5733"))
//	ctx.SetFillBrush(render.HorizontalGradient(render.Red, render.Blue, 0, 100))
func (c *Context) SetFillBrush(b Brush) {
	c.paint.SetBrush(b)
}

// SetStrokeBrush sets the brush used for stroke operations.
// Note: In the current implementation, fill and stroke share the same brush.
// This method is provided for API symmetry and future extensibility.
//
// Example:
//
//	ctx.SetStrokeBrush(render.Solid(render.Black))
//	ctx.SetStrokeBrush(render.SolidRGB(0.5, 0.5, 0.5))
//
// SetStrokeBrush sets the brush used for stroke operations.
// Note: In the current implementation, fill and stroke share the same brush.
// This method is provided for API symmetry and future extensibility.
//
// Example:
//
//	ctx.SetStrokeBrush(render.Solid(render.Black))
//	ctx.SetStrokeBrush(render.SolidRGB(0.5, 0.5, 0.5))
func (c *Context) SetStrokeBrush(b Brush) {
	c.paint.SetBrush(b)
}

// FillBrush returns the current fill brush.
// FillBrush returns the current fill brush.
func (c *Context) FillBrush() Brush {
	return c.paint.GetBrush()
}

// StrokeBrush returns the current stroke brush.
// Note: In the current implementation, fill and stroke share the same brush.
// StrokeBrush returns the current stroke brush.
// Note: In the current implementation, fill and stroke share the same brush.
func (c *Context) StrokeBrush() Brush {
	return c.paint.GetBrush()
}

// SetLineWidth sets the line width for stroking.
// Also syncs paint.Stroke.Width when a Stroke struct is active (EffectiveLineWidth).
// SetLineWidth sets the line width for stroking.
// Also syncs paint.Stroke.Width when a Stroke struct is active (EffectiveLineWidth).
func (c *Context) SetLineWidth(width float64) {
	c.paint.LineWidth = width
	if c.paint.Stroke != nil {
		c.paint.Stroke.Width = width
	}
}

// SetLineCap sets the line cap style.
// Also syncs paint.Stroke.Cap when a Stroke struct is active (EffectiveLineCap);
// otherwise SetLineCap is ignored after any SetDash/SetStroke and ends stay Butt.
// SetLineCap sets the line cap style.
// Also syncs paint.Stroke.Cap when a Stroke struct is active (EffectiveLineCap);
// otherwise SetLineCap is ignored after any SetDash/SetStroke and ends stay Butt.
func (c *Context) SetLineCap(lineCap LineCap) {
	c.paint.LineCap = lineCap
	if c.paint.Stroke != nil {
		c.paint.Stroke.Cap = lineCap
	}
}

// SetLineJoin sets the line join style.
// Also syncs paint.Stroke.Join when a Stroke struct is active (EffectiveLineJoin).
// SetLineJoin sets the line join style.
// Also syncs paint.Stroke.Join when a Stroke struct is active (EffectiveLineJoin).
func (c *Context) SetLineJoin(join LineJoin) {
	c.paint.LineJoin = join
	if c.paint.Stroke != nil {
		c.paint.Stroke.Join = join
	}
}

// SetFillRule sets the fill rule.
// SetFillRule sets the fill rule.
func (c *Context) SetFillRule(rule FillRule) {
	c.paint.FillRule = rule
}

// SetMiterLimit sets the miter limit for line joins.
// SetMiterLimit sets the miter limit for line joins.
func (c *Context) SetMiterLimit(limit float64) {
	c.paint.MiterLimit = limit
}

// SetStroke sets the complete stroke style.
// This is the preferred way to configure stroke properties.
//
// Example:
//
//	ctx.SetStroke(render.DefaultStroke().WithWidth(2).WithCap(render.LineCapRound))
//	ctx.SetStroke(render.DashedStroke(5, 3))
//
// SetStroke sets the complete stroke style.
// This is the preferred way to configure stroke properties.
//
// Example:
//
//	ctx.SetStroke(render.DefaultStroke().WithWidth(2).WithCap(render.LineCapRound))
//	ctx.SetStroke(render.DashedStroke(5, 3))
func (c *Context) SetStroke(stroke Stroke) {
	c.paint.SetStroke(stroke)
}

// GetStroke returns the current stroke style.
// GetStroke returns the current stroke style.
func (c *Context) GetStroke() Stroke {
	return c.paint.GetStroke()
}

// SetDash sets the dash pattern for stroking.
// Pass alternating dash and gap lengths.
// Passing no arguments clears the dash pattern (returns to solid lines).
//
// Example:
//
//	ctx.SetDash(5, 3) // 5 units dash, 3 units gap
//	ctx.SetDash(10, 5, 2, 5) // complex pattern
//	ctx.SetDash() // clear dash (solid line)
//
// SetDash sets the dash pattern for stroking.
// Pass alternating dash and gap lengths.
// Passing no arguments clears the dash pattern (returns to solid lines).
//
// Example:
//
//	ctx.SetDash(5, 3) // 5 units dash, 3 units gap
//	ctx.SetDash(10, 5, 2, 5) // complex pattern
//	ctx.SetDash() // clear dash (solid line)
func (c *Context) SetDash(lengths ...float64) {
	if len(lengths) == 0 {
		c.ClearDash()
		return
	}

	dash := NewDash(lengths...)
	if dash == nil {
		c.ClearDash()
		return
	}

	// Ensure we have a Stroke to set the dash on
	if c.paint.Stroke == nil {
		stroke := c.paint.GetStroke()
		c.paint.Stroke = &stroke
	}
	c.paint.Stroke.Dash = dash
}

// SetDashOffset sets the starting offset into the dash pattern.
// This has no effect if no dash pattern is set.
// SetDashOffset sets the starting offset into the dash pattern.
// This has no effect if no dash pattern is set.
func (c *Context) SetDashOffset(offset float64) {
	if c.paint.Stroke == nil {
		// Create stroke from legacy fields if needed
		stroke := c.paint.GetStroke()
		c.paint.Stroke = &stroke
	}
	if c.paint.Stroke.Dash != nil {
		c.paint.Stroke.Dash = c.paint.Stroke.Dash.WithOffset(offset)
	}
}

// ClearDash removes the dash pattern, returning to solid lines.
// IsDashed returns true if the current stroke uses a dash pattern.
func (c *Context) IsDashed() bool {
	return c.paint.IsDashed()
}

// MoveTo starts a new subpath at the given point.
// currentColor returns the current drawing color from the paint.
// If the paint is a solid color, returns that color.
// Otherwise returns black as a fallback.
func (c *Context) currentColor() color.Color {
	if c.paint.isSolid {
		return c.paint.solidColor.Color()
	}
	if c.paint.Brush != nil {
		if sb, ok := c.paint.Brush.(SolidBrush); ok {
			return sb.Color.Color()
		}
	}
	if p, ok := c.paint.Pattern.(*SolidPattern); ok {
		return p.Color.Color()
	}
	return color.Black
}

// GetCurrentPoint returns the current point of the path.
// Returns (0, 0, false) if there is no current point.
