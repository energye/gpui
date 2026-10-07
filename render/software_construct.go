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

	"github.com/energye/gpui/render/internal/raster"
)

// NewSoftwareRenderer creates a new software renderer with analytic anti-aliasing.
func NewSoftwareRenderer(width, height int) *SoftwareRenderer {
	eb := raster.NewEdgeBuilder(2) // 4x AA, max coord 8191px
	r := &SoftwareRenderer{
		edgeBuilder:    eb,
		analyticFiller: raster.NewAnalyticFiller(width, height),
		width:          width,
		height:         height,
		antiAlias:      true,
	}
	// Atomic has no literal form: store the documented 1.0 default.
	r.deviceScale.Store(math.Float32bits(1.0))
	return r
}

// Resize updates the renderer dimensions (physical pixels).
// This should be called when the context is resized.
// Resize updates the renderer dimensions (physical pixels).
// This should be called when the context is resized.
func (r *SoftwareRenderer) Resize(width, height int) {
	r.width = width
	r.height = height
	eb := raster.NewEdgeBuilder(2) // 4x AA, max coord 8191px
	if ds := math.Float32frombits(r.deviceScale.Load()); ds > 1.0 {
		eb.SetFlattenTolerance(0.1 / ds)
	}
	r.edgeBuilder = eb
	r.analyticFiller = raster.NewAnalyticFiller(width, height)
	// Reset lazy no-AA resources so they pick up new dimensions.
	r.noAAFiller = nil
	r.noAAEdgeBuilder = nil
}

// SetAntiAlias enables or disables anti-aliasing for subsequent Fill/Stroke calls.
// When disabled, the NoAAFiller (integer scanline, binary coverage) is used
// instead of the AnalyticFiller or CoverageFiller.
//
// This method is intended for use by the scene renderer which needs to
// propagate per-draw AA state decoded from TagSetAntiAlias commands.
// SetAntiAlias enables or disables anti-aliasing for subsequent Fill/Stroke calls.
// When disabled, the NoAAFiller (integer scanline, binary coverage) is used
// instead of the AnalyticFiller or CoverageFiller.
//
// This method is intended for use by the scene renderer which needs to
// propagate per-draw AA state decoded from TagSetAntiAlias commands.
func (r *SoftwareRenderer) SetAntiAlias(enabled bool) {
	r.antiAlias = enabled
}

// SetDeviceScale sets the HiDPI device scale factor for the renderer.
// When scale > 1.0, curve flattening tolerance is reduced for finer
// subdivision on HiDPI displays (femtovg pattern: tol = baseTol / scale).
// This produces smoother curves at physical pixel resolution.
// SetDeviceScale sets the HiDPI device scale factor for the renderer.
// When scale > 1.0, curve flattening tolerance is reduced for finer
// subdivision on HiDPI displays (femtovg pattern: tol = baseTol / scale).
// This produces smoother curves at physical pixel resolution.
func (r *SoftwareRenderer) SetDeviceScale(scale float32) {
	if scale <= 0 {
		scale = 1.0
	}
	r.deviceScale.Store(math.Float32bits(scale))
	if scale > 1.0 {
		r.edgeBuilder.SetFlattenTolerance(0.1 / scale)
	}
}

// convertGGPathToCorePath converts a render.Path to raster.PathLike.
