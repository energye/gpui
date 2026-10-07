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
	"io"

)

// RenderPathStats returns a copy of the current routing counters.
func (c *Context) RenderPathStats() RenderPathStats {
	return c.pathStats
}

// ResetRenderPathStats clears GPU/CPU routing counters.
// ResetRenderPathStats clears GPU/CPU routing counters.
func (c *Context) ResetRenderPathStats() {
	c.pathStats = RenderPathStats{}
}

// LogLine formats counters for visualcmd stdout (parsed by STRICT tests).
// LogLine formats counters for visualcmd stdout (parsed by STRICT tests).
func (s RenderPathStats) LogLine() string {
	if s.LastCPUFallbackReason == "" {
		return fmt.Sprintf("gpu_ops=%d cpu_fallback_ops=%d frame_flushes=%d", s.GPUOps, s.CPUFallbackOps, s.FrameFlushes)
	}
	return fmt.Sprintf("gpu_ops=%d cpu_fallback_ops=%d frame_flushes=%d last_cpu_fb=%s", s.GPUOps, s.CPUFallbackOps, s.FrameFlushes, s.LastCPUFallbackReason)
}

func (c *Context) gpuPathAvailable() bool {
	return c.gpuCtxOps() != nil || Accelerator() != nil
}

// PixmapForTest exposes the live pixmap (scratch while a pass swap is
// active). Tests only.
func (c *Context) PixmapForTest() *Pixmap {
	if c == nil {
		return nil
	}
	return c.pixmap
}

// RecordCPUFallbackForTest routes through the production fallback funnel
// (marks scratch dirty when a pass swap is active). Tests only.
// RecordCPUFallbackForTest routes through the production fallback funnel
// (marks scratch dirty when a pass swap is active). Tests only.
func (c *Context) RecordCPUFallbackForTest(reason string) {
	c.recordCPUFallbackReason(reason)
}

// LastCPUFallbackReason returns the most recent fallback reason, if any.
// LastCPUFallbackReason returns the most recent fallback reason, if any.
func (c *Context) LastCPUFallbackReason() string {
	if c == nil {
		return ""
	}
	return c.pathStats.LastCPUFallbackReason
}

// Ensure Context implements io.Closer
var _ io.Closer = (*Context)(nil)

// NewContext creates a new drawing context with the given logical dimensions.
// Optional ContextOption arguments can be used for dependency injection:
//
//	// Default software rendering (uses analytic anti-aliasing)
//	dc := render.NewContext(800, 600)
//
//	// Custom GPU renderer (dependency injection)
//	dc := render.NewContext(800, 600, render.WithRenderer(gpuRenderer))
//
//	// HiDPI/Retina rendering (logical 800x600, physical 1600x1200)
//	dc := render.NewContext(800, 600, render.WithDeviceScale(2.0))
//
// When WithDeviceScale is used, the internal pixmap is allocated at physical
// resolution (width*scale x height*scale) while Width/Height return the
// logical dimensions. All drawing operations use logical coordinates.

// NewContextForPixmap creates a Context backed by an existing Pixmap.
// The Context renders directly into the provided pixmap without allocating
// a new one. Used by scene.Renderer for GPU-accelerated scene rendering.
// setForceSDF enables/disables forced SDF on the registered accelerator.
func (c *Context) setForceSDF(force bool) {
	a := Accelerator()
	if a == nil {
		return
	}
	if f, ok := a.(ForceSDFAware); ok {
		f.SetForceSDF(force)
	}
}
