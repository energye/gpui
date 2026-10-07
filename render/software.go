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
	"sync/atomic"

	"github.com/energye/gpui/render/internal/raster"
)

// SoftwareRenderer is a CPU-based scanline rasterizer using analytic anti-aliasing.
//
// Analytic AA computes the exact area of the shape within each pixel using
// trapezoidal integration. This provides higher quality anti-aliasing than
// supersampling approaches, with no extra memory overhead.
type SoftwareRenderer struct {
	// Analytic AA components
	edgeBuilder    *raster.EdgeBuilder
	analyticFiller *raster.AnalyticFiller

	// Dimensions (physical pixels)
	width, height int

	// HiDPI device scale factor (1.0 = no scaling).
	// Used to adjust curve flattening tolerance for sharper rendering on Retina.
	// R1-3: atomic (math.Float32bits) — rewritten on the raster thread
	// (SetDeviceScale) while draw paths on either thread read it.
	deviceScale atomic.Uint32

	// rasterizerMode is set by Context before calling Fill/Stroke
	// to support forced algorithm selection (RasterizerSparseStrips, etc.).
	// Reset to RasterizerAuto after each call.
	rasterizerMode RasterizerMode

	// antiAlias is set by Context before calling Fill/Stroke.
	// When false, the NoAAFiller (integer scanline, binary coverage) is used
	// instead of AnalyticFiller/CoverageFiller. Reset to true after each call.
	antiAlias bool

	// noAAFiller is the non-anti-aliased filler (lazy-initialized).
	noAAFiller *raster.NoAAFiller

	// noAAEdgeBuilder is a separate EdgeBuilder with aaShift=0 for non-AA.
	// Non-AA does not need sub-pixel edge coordinate shifting.
	noAAEdgeBuilder *raster.EdgeBuilder

	// scratchStrokePath reuses path allocation across Stroke calls.
	scratchStrokePath *Path
}
