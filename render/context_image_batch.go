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

// ImageBatchStats is the image layer's batching census, accumulated across
// frames until a reader resets it (P3.7 batch 6). It exists because the
// per-frame batch totals cannot answer the question a real window has to ask:
// "did N consecutive same-perspective quads collapse into one draw, or did
// something between them break the run?" — that is PerspDraws==1 with
// MaxPerspRun>=N versus PerspDraws==N with MaxPerspRun==1.
//
// Cumulative-since-reset by design, same convention as ProjectionStats.
type ImageBatchStats struct {
	// Draws counts image draws issued; Quads counts the image commands those
	// draws covered (a merged run covers several commands in one draw).
	Draws int
	Quads int
	// MaxRun is the longest single draw measured in queued commands, across
	// every arm; 1 means nothing ever merged.
	MaxRun int
	// Projective arm (P3.7 batch 1a merged perspective neighbours).
	PerspDraws  int
	MaxPerspRun int
	// Quad-material arm (P3.7 batch 3a: one composed pipeline per effect).
	MaterialDraws  int
	MaxMaterialRun int
}

// imageBatchReader is the optional GPU-side source of the census; accelerators
// without image batching simply do not answer and the read returns zeros.
type imageBatchReader interface {
	ImageBatchStats() ImageBatchStats
	ResetImageBatchStats() ImageBatchStats
}

// ImageBatchStats returns this context's image batching census since the last
// reset. CPU-only contexts (no GPU session) report the zero value.
func (c *Context) ImageBatchStats() ImageBatchStats {
	if c == nil {
		return ImageBatchStats{}
	}
	if r, ok := c.gpuCtxOps().(imageBatchReader); ok {
		return r.ImageBatchStats()
	}
	return ImageBatchStats{}
}

// ResetImageBatchStats zeroes the census and returns the value it replaced,
// so a window can measure one segment in isolation.
func (c *Context) ResetImageBatchStats() ImageBatchStats {
	if c == nil {
		return ImageBatchStats{}
	}
	if r, ok := c.gpuCtxOps().(imageBatchReader); ok {
		return r.ResetImageBatchStats()
	}
	return ImageBatchStats{}
}
