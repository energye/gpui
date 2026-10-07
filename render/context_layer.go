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
	"image"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// Layer represents a drawing layer with blend mode and opacity.
// Layers allow isolating drawing operations and compositing them with
// different blend modes and opacity values, similar to layers in Photoshop
// or SVG group opacity.
type Layer struct {
	pixmap    *Pixmap
	blendMode BlendMode
	opacity   float64
	mask      *Mask // optional alpha mask, applied on PopLayer (nil = no mask)
	// damage is the union of draw bounds on this layer in pixmap pixel space.
	// PopLayer composites only this rect (plus AA pad) instead of the full
	// surface — full 800x600 CPU blend was ~50ms/frame on Intel iGPU.
	// Empty means "unknown / full surface" (safe fallback).
	damage image.Rectangle
	// fullComposite forces a full-surface blend (backdrop snapshots, masks).
	fullComposite bool

	// opacityGroup (F1): Normal/Copy layers without mask skip isolation RT and
	// multiply paint alpha into subsequent draws instead. Semantically equal for
	// single SourceOver fills; multi-draw isolation still uses a real layer RT
	// when blend is advanced or a mask is set.
	opacityGroup bool

	// GPU layer RT (P0-1 / L.01–L.02): draw into an offscreen texture when
	// available, then composite with DrawGPUTexture* on Pop — no GPU→CPU
	// readback of the layer surface.
	gpuView    gpucontext.TextureView
	gpuRelease func()
	gpuW, gpuH int
	// cpuDrew is true when any CPU path wrote into layer.pixmap while the
	// layer was active (fallback, SetPixel, advanced-blend CPU layers, etc.).
	cpuDrew bool
}

// layerStack manages the layer hierarchy for the context.
type layerStack struct {
	layers []*Layer
	// pool reuses full-surface layer Pixmaps. intImage.Pool is ImageBuf-only.
	pool *pixmapPool
}
