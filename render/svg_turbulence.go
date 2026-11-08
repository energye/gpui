//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

// SVGTurbulenceParams is the SVG feTurbulence attribute set (P3.8 4b-3,
// resvg turbulence.rs semantics). The noise samples at buffer pixel +
// Offset (the filter region origin: this engine's filter buffers are
// user-space 1:1, the (x+offset)/canvas-scale of resvg with the scale
// folded away). Output is premultiplied RGBA8.
type SVGTurbulenceParams struct {
	OffsetX, OffsetY              float32
	BaseFrequencyX, BaseFrequencyY float32
	NumOctaves                    int
	Seed                          int32
	StitchTiles                   bool
	FractalNoise                  bool // type="fractalNoise"; default turbulence
	LinearRGB                     bool // color-interpolation-filters (spec default true)
}

// SVG edge modes for feConvolveMatrix (resvg EdgeMode order).
const (
	SVGEdgeModeNone      = 0
	SVGEdgeModeDuplicate = 1
	SVGEdgeModeWrap      = 2
)

// SVGConvolveMatrixParams is the SVG feConvolveMatrix attribute set (P3.8
// 4b-3, resvg convolve_matrix.rs semantics). Kernel is row-major with
// exactly Columns*Rows entries (validated by the svg layer — invalid
// parameter sets produce a transparent dummy primitive, usvg stance).
type SVGConvolveMatrixParams struct {
	Columns, Rows int       // orderX / orderY
	Kernel        []float32 // row-major, len == Columns*Rows
	Divisor       float32   // non-zero (validated by the svg layer)
	Bias          float32
	TargetX       int // kernel center, defaults floor(order/2)
	TargetY       int
	EdgeMode      int // SVGEdgeModeNone / Duplicate / Wrap
	PreserveAlpha bool
	LinearRGB     bool // color-interpolation-filters (spec default true)
}

// ApplySVGTurbulence generates SVG feTurbulence fractal noise over the
// content (P3.8 4b-3). The source content is ignored — the primitive is a
// generator. CPU-only for now: the pixel-exact reference is resvg
// turbulence.rs and the conformance harness needs deterministic offline
// results; a GPU variant is a registered follow-up batch (same stance as
// the lighting pair and the convolve matrix).
func (c *Context) ApplySVGTurbulence(p SVGTurbulenceParams) {
	if c == nil || c.pixmap == nil {
		return
	}
	_ = c.FlushGPU()
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if turbulenceApply != nil {
			turbulenceApply(src, dst, p)
		}
	})
	c.markLayerFullComposite()
}

// ApplySVGConvolveMatrix runs SVG feConvolveMatrix over the content (P3.8
// 4b-3): the kernel matrix convolves the premul buffer (straight-alpha when
// PreserveAlpha), RGB clamped to the new alpha. CPU-only (same stance as
// ApplySVGTurbulence).
func (c *Context) ApplySVGConvolveMatrix(p SVGConvolveMatrixParams) {
	if c == nil || c.pixmap == nil {
		return
	}
	_ = c.FlushGPU()
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if convolveMatrixApply != nil {
			convolveMatrixApply(src, dst, p)
		}
	})
	c.markLayerFullComposite()
}
