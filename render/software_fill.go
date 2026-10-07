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
	"unsafe"

	"github.com/energye/gpui/render/internal/raster"
)

// convertGGPathToCorePath converts a render.Path to raster.PathLike.
func convertGGPathToCorePath(p *Path) raster.PathLike {
	verbs := make([]raster.PathVerb, 0, p.NumVerbs())
	points := make([]float32, 0, len(p.Coords())*2) //nolint:mnd // preallocate for float64→float32

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			verbs = append(verbs, raster.MoveTo)
			points = append(points, float32(coords[0]), float32(coords[1]))
		case LineTo:
			verbs = append(verbs, raster.LineTo)
			points = append(points, float32(coords[0]), float32(coords[1]))
		case QuadTo:
			verbs = append(verbs, raster.QuadTo)
			points = append(points,
				float32(coords[0]), float32(coords[1]),
				float32(coords[2]), float32(coords[3]),
			)
		case CubicTo:
			verbs = append(verbs, raster.CubicTo)
			points = append(points,
				float32(coords[0]), float32(coords[1]),
				float32(coords[2]), float32(coords[3]),
				float32(coords[4]), float32(coords[5]),
			)
		case Close:
			verbs = append(verbs, raster.Close)
		}
	})

	return raster.NewScenePathAdapter(len(verbs) == 0, verbs, points)
}

const (
	// minTileArea is the minimum bounding box area (px²) for tile-based
	// rasterization. Below this, tile setup overhead exceeds scanline cost.
	// 512 = 32×16 — allows wide-but-short paths (e.g. text at 16px height)
	// while rejecting paths that are too small in both dimensions.
	minTileArea = 512

	// minSingleDimension prevents degenerate nearly-linear paths from
	// triggering tile rasterization. A 1000px × 1px line should not use tiles.
	minSingleDimension = 8

	// minElementThreshold is the absolute minimum element count for CoverageFiller.
	// Paths with fewer elements are always cheaper with scanline rasterization
	// since the per-pixel work scales linearly with edge crossings.
	minElementThreshold = 32

	// maxElementThreshold caps the adaptive threshold for tiny bounding boxes.
	// Even extremely complex paths in a small area are better handled by scanline
	// because the total pixel count is low.
	maxElementThreshold = 256
)

// adaptiveThreshold computes the element count threshold for switching from
// AnalyticFiller to CoverageFiller based on bounding box area.
// Larger bounding boxes lower the threshold because scanline cost grows with
// width (O(width * edges)) while tile-based cost grows with fill area.
// The formula 2048/sqrt(area) produces: 100x100 -> 20, 50x50 -> 29, 200x200 -> 10.
// Results are clamped to [minElementThreshold, maxElementThreshold].
// adaptiveThreshold computes the element count threshold for switching from
// AnalyticFiller to CoverageFiller based on bounding box area.
// Larger bounding boxes lower the threshold because scanline cost grows with
// width (O(width * edges)) while tile-based cost grows with fill area.
// The formula 2048/sqrt(area) produces: 100x100 -> 20, 50x50 -> 29, 200x200 -> 10.
// Results are clamped to [minElementThreshold, maxElementThreshold].
func adaptiveThreshold(bboxArea float64) int {
	if bboxArea <= 0 {
		return maxElementThreshold
	}
	threshold := int(2048.0 / math.Sqrt(bboxArea))
	if threshold < minElementThreshold {
		return minElementThreshold
	}
	if threshold > maxElementThreshold {
		return maxElementThreshold
	}
	return threshold
}

// pathBounds computes the axis-aligned bounding box of a path by iterating
// over all path elements. Returns (minX, minY, maxX, maxY).
// For an empty path, returns (0, 0, 0, 0).
// pathBounds computes the axis-aligned bounding box of a path by iterating
// over all path elements. Returns (minX, minY, maxX, maxY).
// For an empty path, returns (0, 0, 0, 0).
func pathBounds(p *Path) (minX, minY, maxX, maxY float64) {
	if p.isEmpty() {
		return 0, 0, 0, 0
	}

	minX = math.MaxFloat64
	minY = math.MaxFloat64
	maxX = -math.MaxFloat64
	maxY = -math.MaxFloat64

	expandPt := func(x, y float64) {
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo, LineTo:
			expandPt(coords[0], coords[1])
		case QuadTo:
			expandPt(coords[0], coords[1])
			expandPt(coords[2], coords[3])
		case CubicTo:
			expandPt(coords[0], coords[1])
			expandPt(coords[2], coords[3])
			expandPt(coords[4], coords[5])
		}
	})

	return minX, minY, maxX, maxY
}

// shouldUseTileRasterizer returns true if the path is complex enough to
// benefit from tile-based rasterization. It uses a bounding box area check
// (not per-dimension) so that wide-but-short dense paths (e.g. text outlines
// at 16px with hundreds of path elements) can be routed to the tile rasterizer.
// shouldUseTileRasterizer returns true if the path is complex enough to
// benefit from tile-based rasterization. It uses a bounding box area check
// (not per-dimension) so that wide-but-short dense paths (e.g. text outlines
// at 16px with hundreds of path elements) can be routed to the tile rasterizer.
func shouldUseTileRasterizer(p *Path) bool {
	nElems := p.NumVerbs()
	if nElems <= 0 {
		return false
	}

	x1, y1, x2, y2 := pathBounds(p)
	bboxW := x2 - x1
	bboxH := y2 - y1

	// Area check: tile setup overhead is only worthwhile when there is
	// enough fill area. This replaces the old per-dimension check
	// (bboxMinDimension=32) which rejected wide-but-short text paths.
	bboxArea := bboxW * bboxH
	if bboxArea < minTileArea {
		return false
	}

	// Require at least half-tile in each dimension to avoid degenerate
	// nearly-linear paths (e.g. 1000px × 1px hairline).
	if bboxW < minSingleDimension || bboxH < minSingleDimension {
		return false
	}

	return nElems > adaptiveThreshold(bboxArea)
}

// fillWithCoverageFiller rasterizes the path using the tile-based CoverageFiller
// and composites the result onto the pixmap.
// fillWithCoverageFiller rasterizes the path using the tile-based CoverageFiller
// and composites the result onto the pixmap.
func (r *SoftwareRenderer) fillWithCoverageFiller(
	pixmap *Pixmap, p *Path, paint *Paint, filler CoverageFiller,
) {
	fillRule := FillRuleNonZero
	if paint.FillRule == FillRuleEvenOdd {
		fillRule = FillRuleEvenOdd
	}
	clipFn := paint.ClipCoverage
	maskFn := paint.MaskCoverage
	if color, ok := solidColorFromPaint(paint); ok {
		r.fillCoverageSolidPath(pixmap, p, filler, fillRule, color, clipFn, maskFn)
	} else {
		r.fillCoveragePaintPath(pixmap, p, filler, fillRule, paint, clipFn, maskFn)
	}
}

// fillCoverageSolidPath fills using the CoverageFiller with a solid color,
// applying optional clip and mask coverage.
// fillCoverageSolidPath fills using the CoverageFiller with a solid color,
// applying optional clip and mask coverage.
func (r *SoftwareRenderer) fillCoverageSolidPath(
	pixmap *Pixmap, p *Path, filler CoverageFiller,
	fillRule FillRule, color RGBA, clipFn func(x, y float64) byte, maskFn func(x, y int) uint8,
) {
	filler.FillCoverage(p, r.width, r.height, fillRule,
		func(x, y int, coverage uint8) {
			coverage = applyClipCoverage(clipFn, x, y, coverage)
			coverage = applyMaskCoverage(maskFn, x, y, coverage)
			if coverage == 0 {
				return
			}
			r.blendCoverageSolid(pixmap, x, y, coverage, color)
		})
}

// fillCoveragePaintPath fills using the CoverageFiller with a paint pattern,
// applying optional clip and mask coverage.
// fillCoveragePaintPath fills using the CoverageFiller with a paint pattern,
// applying optional clip and mask coverage.
func (r *SoftwareRenderer) fillCoveragePaintPath(
	pixmap *Pixmap, p *Path, filler CoverageFiller,
	fillRule FillRule, paint *Paint, clipFn func(x, y float64) byte, maskFn func(x, y int) uint8,
) {
	filler.FillCoverage(p, r.width, r.height, fillRule,
		func(x, y int, coverage uint8) {
			coverage = applyClipCoverage(clipFn, x, y, coverage)
			coverage = applyMaskCoverage(maskFn, x, y, coverage)
			if coverage == 0 {
				return
			}
			r.blendCoveragePaint(pixmap, x, y, coverage, paint)
		})
}

// applyClipCoverage multiplies pixel coverage by the clip mask coverage.
// Returns 0 if the pixel is fully clipped. When clipFn is nil, returns the
// original coverage unchanged.
// applyClipCoverage multiplies pixel coverage by the clip mask coverage.
// Returns 0 if the pixel is fully clipped. When clipFn is nil, returns the
// original coverage unchanged.
func applyClipCoverage(clipFn func(x, y float64) byte, px, py int, coverage uint8) uint8 {
	if clipFn == nil {
		return coverage
	}
	cc := clipFn(float64(px)+0.5, float64(py)+0.5)
	if cc == 0 {
		return 0
	}
	if cc == 255 {
		return coverage
	}
	return uint8(uint16(coverage) * uint16(cc) / 255)
}

// applyMaskCoverage multiplies pixel coverage by the alpha mask coverage.
// Returns 0 if the pixel is fully masked out. When maskFn is nil, returns the
// original coverage unchanged. Uses int coords because masks are pixel-aligned.
// applyMaskCoverage multiplies pixel coverage by the alpha mask coverage.
// Returns 0 if the pixel is fully masked out. When maskFn is nil, returns the
// original coverage unchanged. Uses int coords because masks are pixel-aligned.
func applyMaskCoverage(maskFn func(x, y int) uint8, px, py int, coverage uint8) uint8 {
	if maskFn == nil {
		return coverage
	}
	mc := maskFn(px, py)
	if mc == 0 {
		return 0
	}
	if mc == 255 {
		return coverage
	}
	return uint8(uint16(coverage) * uint16(mc) / 255)
}

// Fill implements Renderer.Fill using analytic anti-aliasing.
// For complex paths, it auto-selects the registered CoverageFiller (tile-based
// rasterizer) when available, using an adaptive threshold based on both path
// element count and bounding box area.
//
// When rasterizerMode is set (via Context.SetRasterizerMode), the forced
// algorithm is used instead of auto-selection.
// Fill implements Renderer.Fill using analytic anti-aliasing.
// For complex paths, it auto-selects the registered CoverageFiller (tile-based
// rasterizer) when available, using an adaptive threshold based on both path
// element count and bounding box area.
//
// When rasterizerMode is set (via Context.SetRasterizerMode), the forced
// algorithm is used instead of auto-selection.
func (r *SoftwareRenderer) Fill(pixmap *Pixmap, p *Path, paint *Paint) error {
	// Non-AA path: completely separate code path.
	// Integer scanline, binary coverage, no CoverageFiller/AnalyticFiller.
	if !r.antiAlias {
		return r.fillNoAA(pixmap, p, paint)
	}

	// Force mode: specific algorithm without auto-selection.
	switch r.rasterizerMode {
	case RasterizerAnalytic:
		// Skip CoverageFiller entirely → always use AnalyticFiller below.

	case RasterizerSparseStrips:
		if filler := r.forcedFiller(RasterizerSparseStrips); filler != nil {
			r.fillWithCoverageFiller(pixmap, p, paint, filler)
			return nil
		}

	case RasterizerTileCompute:
		if filler := r.forcedFiller(RasterizerTileCompute); filler != nil {
			r.fillWithCoverageFiller(pixmap, p, paint, filler)
			return nil
		}

	default: // RasterizerAuto
		// Auto-selection: use CoverageFiller for complex paths.
		if filler := GetCoverageFiller(); filler != nil && shouldUseTileRasterizer(p) {
			r.fillWithCoverageFiller(pixmap, p, paint, filler)
			return nil
		}
	}

	// AnalyticFiller path (scanline) — simple paths, forced analytic, or no filler
	r.edgeBuilder.Reset()
	r.analyticFiller.Reset()

	// Clip paths to canvas bounds to prevent FDot6→FDot16 integer overflow.
	// At aaShift=4, coordinates > 2048px overflow int32 in FDot16, causing
	// silent wrap-around that places edges at wrong positions (RAST-010).
	// Small margin for AA bleed — coordinates at canvas+2px are still well
	// within safe range.
	clipMargin := float32(2)
	clipRect := raster.Rect{
		MinX: -clipMargin,
		MinY: -clipMargin,
		MaxX: float32(pixmap.Width()) + clipMargin,
		MaxY: float32(pixmap.Height()) + clipMargin,
	}
	r.edgeBuilder.SetClipRect(&clipRect)

	// Flatten curves to line segments for the AnalyticFiller.
	// Forward differencing (QuadraticEdge/CubicEdge) can produce zero-height
	// segments after FDot6 rounding, silently losing winding contribution.
	// Pre-flattening with adaptive subdivision (0.1px tolerance) eliminates
	// this class of errors. This is the standard approach in tiny-skia and
	// the analytic AA scanline rasterizer.
	r.edgeBuilder.SetFlattenCurves(true)
	defer r.edgeBuilder.SetFlattenCurves(false)

	// Build edges from the path directly from float64 coords (zero-alloc).
	// PathVerb values match between gg and raster packages (both 0-4 iota).
	// render.PathVerb is byte, so []PathVerb has identical memory layout to []byte.
	verbs := p.Verbs()
	if len(verbs) > 0 {
		verbBytes := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(verbs))), len(verbs))
		r.edgeBuilder.BuildFromPathF64(verbBytes, p.Coords())
	}

	// If no edges, nothing to fill
	if r.edgeBuilder.IsEmpty() {
		return nil
	}

	// Convert fill rule
	coreFillRule := raster.FillRuleNonZero
	if paint.FillRule == FillRuleEvenOdd {
		coreFillRule = raster.FillRuleEvenOdd
	}

	if color, ok := solidColorFromPaint(paint); ok {
		// Fast path: solid color
		clipFn := paint.ClipCoverage
		maskFn := paint.MaskCoverage
		r.analyticFiller.Fill(r.edgeBuilder, coreFillRule, func(y int, runs *raster.AlphaRuns) {
			r.blendAlphaRunsFromCoreRuns(pixmap, y, runs, color, clipFn, maskFn, paint.BlendMode)
		})
	} else {
		// Pattern/gradient path: per-pixel color sampling
		clipFn := paint.ClipCoverage
		maskFn := paint.MaskCoverage
		r.analyticFiller.Fill(r.edgeBuilder, coreFillRule, func(y int, runs *raster.AlphaRuns) {
			r.blendAlphaRunsFromCoreRunsPaint(pixmap, y, runs, paint, clipFn, maskFn)
		})
	}

	return nil
}

// fillNoAA renders a filled path without anti-aliasing.
// Uses a dedicated NoAAFiller that produces solid horizontal spans with
// binary coverage (0 or 255). This is a completely separate code path
// from the AA rasterizer.
// fillNoAA renders a filled path without anti-aliasing.
// Uses a dedicated NoAAFiller that produces solid horizontal spans with
// binary coverage (0 or 255). This is a completely separate code path
// from the AA rasterizer.
func (r *SoftwareRenderer) fillNoAA(pixmap *Pixmap, p *Path, paint *Paint) error {
	// Lazy-init the no-AA edge builder and filler.
	if r.noAAEdgeBuilder == nil {
		r.noAAEdgeBuilder = raster.NewEdgeBuilder(0) // aaShift=0: no sub-pixel
		if ds := math.Float32frombits(r.deviceScale.Load()); ds > 1.0 {
			r.noAAEdgeBuilder.SetFlattenTolerance(0.1 / ds)
		}
	}
	if r.noAAFiller == nil {
		r.noAAFiller = raster.NewNoAAFiller(r.width, r.height)
	}

	r.noAAEdgeBuilder.Reset()

	clipMargin := float32(2)
	clipRect := raster.Rect{
		MinX: -clipMargin,
		MinY: -clipMargin,
		MaxX: float32(pixmap.Width()) + clipMargin,
		MaxY: float32(pixmap.Height()) + clipMargin,
	}
	r.noAAEdgeBuilder.SetClipRect(&clipRect)

	r.noAAEdgeBuilder.SetFlattenCurves(true)
	defer r.noAAEdgeBuilder.SetFlattenCurves(false)

	verbs := p.Verbs()
	if len(verbs) > 0 {
		verbBytes := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(verbs))), len(verbs))
		r.noAAEdgeBuilder.BuildFromPathF64(verbBytes, p.Coords())
	}

	if r.noAAEdgeBuilder.IsEmpty() {
		return nil
	}

	coreFillRule := raster.FillRuleNonZero
	if paint.FillRule == FillRuleEvenOdd {
		coreFillRule = raster.FillRuleEvenOdd
	}

	clipFn := paint.ClipCoverage
	maskFn := paint.MaskCoverage

	if color, ok := solidColorFromPaint(paint); ok {
		r.noAAFiller.Fill(r.noAAEdgeBuilder, coreFillRule, func(y, left, spanWidth int) {
			r.blitNoAASolidSpan(pixmap, y, left, spanWidth, color, clipFn, maskFn)
		})
	} else {
		r.noAAFiller.Fill(r.noAAEdgeBuilder, coreFillRule, func(y, left, spanWidth int) {
			r.blitNoAAPaintSpan(pixmap, y, left, spanWidth, paint, clipFn, maskFn)
		})
	}

	return nil
}

// blitNoAASolidSpan blits a solid-color span with optional clip and mask.
// blitNoAASolidSpan blits a solid-color span with optional clip and mask.
func (r *SoftwareRenderer) blitNoAASolidSpan(
	pixmap *Pixmap, y, left, spanWidth int, color RGBA,
	clipFn func(float64, float64) byte, maskFn func(int, int) uint8,
) {
	for x := left; x < left+spanWidth; x++ {
		cov := noaaPixelCoverage(x, y, clipFn, maskFn)
		if cov == 0 {
			continue
		}
		r.blendCoverageSolid(pixmap, x, y, cov, color)
	}
}

// blitNoAAPaintSpan blits a paint-sampled span with optional clip and mask.
// blitNoAAPaintSpan blits a paint-sampled span with optional clip and mask.
func (r *SoftwareRenderer) blitNoAAPaintSpan(
	pixmap *Pixmap, y, left, spanWidth int, paint *Paint,
	clipFn func(float64, float64) byte, maskFn func(int, int) uint8,
) {
	for x := left; x < left+spanWidth; x++ {
		cov := noaaPixelCoverage(x, y, clipFn, maskFn)
		if cov == 0 {
			continue
		}
		c := paint.ColorAt(float64(x)+0.5, float64(y)+0.5)
		r.blendCoverageSolid(pixmap, x, y, cov, c)
	}
}

// noaaPixelCoverage computes per-pixel coverage from clip and mask functions.
// Returns 0 if the pixel is fully clipped/masked, 255 if no clip/mask is active.
// noaaPixelCoverage computes per-pixel coverage from clip and mask functions.
// Returns 0 if the pixel is fully clipped/masked, 255 if no clip/mask is active.
func noaaPixelCoverage(x, y int, clipFn func(float64, float64) byte, maskFn func(int, int) uint8) byte {
	cov := byte(255)
	if clipFn != nil {
		clipCov := clipFn(float64(x)+0.5, float64(y)+0.5)
		if clipCov == 0 {
			return 0
		}
		cov = clipCov
	}
	if maskFn != nil {
		mc := maskFn(x, y)
		if mc == 0 {
			return 0
		}
		if cov != 255 {
			cov = uint8(uint16(cov) * uint16(mc) / 255)
		} else {
			cov = mc
		}
	}
	return cov
}

// blendCoverageSolid blends a single pixel with solid color and coverage.
// Uses premultiplied source-over compositing.
// blendCoverageSolid blends a single pixel with solid color and coverage.
// Uses premultiplied source-over compositing.
func (r *SoftwareRenderer) blendCoverageSolid(pixmap *Pixmap, x, y int, coverage uint8, color RGBA) {
	if x < 0 || x >= pixmap.Width() || y < 0 || y >= pixmap.Height() {
		return
	}

	if coverage == 255 && color.A == 1.0 {
		pixmap.SetPixel(x, y, color)
		return
	}

	srcAlpha := color.A * float64(coverage) / 255.0
	invSrcAlpha := 1.0 - srcAlpha

	srcR := color.R * srcAlpha
	srcG := color.G * srcAlpha
	srcB := color.B * srcAlpha

	dstR, dstG, dstB, dstA := pixmap.getPremul(x, y)

	pixmap.setPremul(x, y,
		srcR+dstR*invSrcAlpha,
		srcG+dstG*invSrcAlpha,
		srcB+dstB*invSrcAlpha,
		srcAlpha+dstA*invSrcAlpha,
	)
}

// blendCoveragePaint blends a single pixel with paint-sampled color and coverage.
// Uses premultiplied source-over compositing.
// blendCoveragePaint blends a single pixel with paint-sampled color and coverage.
// Uses premultiplied source-over compositing.
func (r *SoftwareRenderer) blendCoveragePaint(pixmap *Pixmap, x, y int, coverage uint8, paint *Paint) {
	if x < 0 || x >= pixmap.Width() || y < 0 || y >= pixmap.Height() {
		return
	}

	color := paint.ColorAt(float64(x)+0.5, float64(y)+0.5)

	if coverage == 255 && color.A == 1.0 {
		pixmap.SetPixel(x, y, color)
		return
	}

	srcAlpha := color.A * float64(coverage) / 255.0
	invSrcAlpha := 1.0 - srcAlpha

	srcR := color.R * srcAlpha
	srcG := color.G * srcAlpha
	srcB := color.B * srcAlpha

	dstR, dstG, dstB, dstA := pixmap.getPremul(x, y)

	pixmap.setPremul(x, y,
		srcR+dstR*invSrcAlpha,
		srcG+dstG*invSrcAlpha,
		srcB+dstB*invSrcAlpha,
		srcAlpha+dstA*invSrcAlpha,
	)
}

// forcedFiller returns the CoverageFiller for a forced rasterizer mode.
// If the registered filler implements ForceableFiller, the specific sub-filler
// (SparseStrips or TileCompute) is returned. Otherwise, the filler is used as-is.
// forcedFiller returns the CoverageFiller for a forced rasterizer mode.
// If the registered filler implements ForceableFiller, the specific sub-filler
// (SparseStrips or TileCompute) is returned. Otherwise, the filler is used as-is.
func (r *SoftwareRenderer) forcedFiller(mode RasterizerMode) CoverageFiller {
	filler := GetCoverageFiller()
	if filler == nil {
		return nil
	}
	ff, ok := filler.(ForceableFiller)
	if !ok {
		return filler
	}
	switch mode {
	case RasterizerSparseStrips:
		return ff.SparseFiller()
	case RasterizerTileCompute:
		return ff.ComputeFiller()
	default:
		return filler
	}
}

// solidColorFromPaint returns the solid color if paint is solid.
// Returns (color, true) for solid paints, (zero, false) for patterns/gradients.
// solidColorFromPaint returns the solid color if paint is solid.
// Returns (color, true) for solid paints, (zero, false) for patterns/gradients.
func solidColorFromPaint(paint *Paint) (RGBA, bool) {
	// Fast path: inline solid color (zero allocation, no interface dispatch).
	if paint.isSolid {
		return paint.solidColor, true
	}
	// Check Brush first (takes precedence)
	if paint.Brush != nil {
		if sb, ok := paint.Brush.(SolidBrush); ok {
			return sb.Color, true
		}
		return RGBA{}, false
	}
	// Fall back to Pattern
	if sp, ok := paint.Pattern.(*SolidPattern); ok {
		return sp.Color, true
	}
	return RGBA{}, false
}

// blendAlphaRunsFromCoreRuns blends alpha values from raster.AlphaRuns to the pixmap.
// Uses source-over compositing for proper alpha blending.
// When clipFn is non-nil, each pixel's alpha is multiplied by the clip coverage.
// When maskFn is non-nil, each pixel's alpha is multiplied by the mask coverage.
// blendAlphaRunsFromCoreRuns blends alpha values from raster.AlphaRuns to the pixmap.
// Uses source-over compositing for proper alpha blending.
// When clipFn is non-nil, each pixel's alpha is multiplied by the clip coverage.
// When maskFn is non-nil, each pixel's alpha is multiplied by the mask coverage.
func (r *SoftwareRenderer) blendAlphaRunsFromCoreRuns(pixmap *Pixmap, y int, runs *raster.AlphaRuns, color RGBA, clipFn func(x, y float64) byte, maskFn func(x, y int) uint8, blendMode BlendMode) {
	if y < 0 || y >= pixmap.Height() {
		return
	}

	fy := float64(y) + 0.5
	useAdvanced := blendMode != BlendNormal

	for x, alpha := range runs.Iter() {
		if alpha == 0 {
			continue
		}
		if x < 0 || x >= pixmap.Width() {
			continue
		}

		// Apply clip coverage if active.
		if clipFn != nil {
			cc := clipFn(float64(x)+0.5, fy)
			if cc == 0 {
				continue
			}
			alpha = uint8(uint16(alpha) * uint16(cc) / 255)
			if alpha == 0 {
				continue
			}
		}

		// Apply mask coverage if active.
		alpha = applyMaskCoverage(maskFn, x, y, alpha)
		if alpha == 0 {
			continue
		}

		if useAdvanced {
			compositeAdvanced(pixmap, x, y, color, alpha, blendMode)
			continue
		}

		// Full coverage - just set the pixel
		if alpha == 255 && color.A == 1.0 {
			pixmap.SetPixel(x, y, color)
			continue
		}

		// Partial coverage - premultiplied source-over compositing
		srcAlpha := color.A * float64(alpha) / 255.0
		invSrcAlpha := 1.0 - srcAlpha

		srcR := color.R * srcAlpha
		srcG := color.G * srcAlpha
		srcB := color.B * srcAlpha

		dstR, dstG, dstB, dstA := pixmap.getPremul(x, y)

		pixmap.setPremul(x, y,
			srcR+dstR*invSrcAlpha,
			srcG+dstG*invSrcAlpha,
			srcB+dstB*invSrcAlpha,
			srcAlpha+dstA*invSrcAlpha,
		)
	}
}

func (r *SoftwareRenderer) blendAlphaRunsFromCoreRunsPaint(pixmap *Pixmap, y int, runs *raster.AlphaRuns, paint *Paint, clipFn func(x, y float64) byte, maskFn func(x, y int) uint8) {
	if y < 0 || y >= pixmap.Height() {
		return
	}

	fy := float64(y) + 0.5

	for x, alpha := range runs.Iter() {
		if alpha == 0 {
			continue
		}
		if x < 0 || x >= pixmap.Width() {
			continue
		}

		fx := float64(x) + 0.5

		// Apply clip coverage if active.
		if clipFn != nil {
			cc := clipFn(fx, fy)
			if cc == 0 {
				continue
			}
			alpha = uint8(uint16(alpha) * uint16(cc) / 255)
			if alpha == 0 {
				continue
			}
		}

		// Apply mask coverage if active.
		alpha = applyMaskCoverage(maskFn, x, y, alpha)
		if alpha == 0 {
			continue
		}

		// Sample color from paint at pixel center
		color := paint.ColorAt(fx, fy)

		if paint.BlendMode != BlendNormal {
			compositeAdvanced(pixmap, x, y, color, alpha, paint.BlendMode)
			continue
		}

		if alpha == 255 && color.A == 1.0 {
			pixmap.SetPixel(x, y, color)
			continue
		}

		srcAlpha := color.A * float64(alpha) / 255.0
		invSrcAlpha := 1.0 - srcAlpha

		srcR := color.R * srcAlpha
		srcG := color.G * srcAlpha
		srcB := color.B * srcAlpha

		dstR, dstG, dstB, dstA := pixmap.getPremul(x, y)

		pixmap.setPremul(x, y,
			srcR+dstR*invSrcAlpha,
			srcG+dstG*invSrcAlpha,
			srcB+dstB*invSrcAlpha,
			srcAlpha+dstA*invSrcAlpha,
		)
	}
}

// Stroke implements Renderer.Stroke with anti-aliasing support.
// Strokes are expanded to fill paths and rendered with the Fill method,
// which provides analytic anti-aliased results.
