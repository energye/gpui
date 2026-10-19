//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"github.com/energye/gpui/render/internal/raster"
	"github.com/energye/gpui/render/scene"
)

// FineRasterizer performs fine rasterization with analytic anti-aliasing.
// It calculates exact sub-pixel coverage for each tile based on the segments
// that cross it and the fill rule.
type FineRasterizer struct {
	grid     *TileGrid
	segments *SegmentList

	// backdropRows holds the per-pixel-row backdrop winding produced by
	// CoarseRasterizer.CalculateBackdrop, indexed by
	// [(pixelY-backdropRowOffset)*cols + tileX]. Only that row range is
	// materialized (see CoarseRasterizer.CalculateBackdrop).
	backdropRows []float32

	// backdropRowOffset is the canvas pixel row stored at backdropRows row 0.
	backdropRowOffset int

	fillRule scene.FillStyle

	// visitedTiles marks tiles reached through coarse entries, indexed by
	// ty*tileColumns+tx. Reused across Rasterize calls (see Rasterize), so the
	// backdrop-only emission pass can skip already-processed tiles without a
	// per-fill map or hashing.
	visitedTiles []bool

	// Viewport dimensions
	width  uint16
	height uint16

	// Tile dimensions
	tileColumns uint16
	tileRows    uint16
}

// NewFineRasterizer creates a new fine rasterizer for the given dimensions.
func NewFineRasterizer(width, height uint16) *FineRasterizer {
	return &FineRasterizer{
		grid:        NewTileGrid(),
		width:       width,
		height:      height,
		tileColumns: (width + TileWidth - 1) / TileWidth,
		tileRows:    (height + TileHeight - 1) / TileHeight,
		fillRule:    scene.FillNonZero,
	}
}

// Reset clears the rasterizer state for reuse.
func (fr *FineRasterizer) Reset() {
	fr.grid.Reset()
	fr.segments = nil
	fr.backdropRows = nil
	fr.backdropRowOffset = 0
}

// SetFillRule sets the fill rule for coverage calculation.
func (fr *FineRasterizer) SetFillRule(rule scene.FillStyle) {
	fr.fillRule = rule
	fr.grid.SetFillRule(rule)
}

// FillRule returns the current fill rule.
func (fr *FineRasterizer) FillRule() scene.FillStyle {
	return fr.fillRule
}

// Rasterize performs fine rasterization on coarse tile entries.
// It calculates analytic anti-aliased coverage for each pixel.
//
// Coverage accumulates exactly: every pixel starts from the per-pixel-row
// backdrop (winding of everything left of the tile) and each entry adds its
// local per-pixel area contribution (see accumulateSegmentWinding).
//
// After processing all coarse entries (tiles with segments), this method
// also emits tiles for backdrop-only positions — tiles that have non-zero
// backdrop winding but no segment entries. These are the interior tiles of
// filled shapes (e.g., the middle of a wide rectangle).
func (fr *FineRasterizer) Rasterize(
	coarse *CoarseRasterizer,
	segments *SegmentList,
	backdropRows []float32,
) {
	fr.segments = segments
	fr.backdropRows = backdropRows
	fr.backdropRowOffset = coarse.BackdropRowOffset()
	fr.grid.Reset()

	if segments == nil || len(coarse.Entries()) == 0 {
		// Even with no entries, backdrop may contain non-zero values
		// from a prior CalculateBackdrop() — but if entries is empty,
		// backdrop should also be nil per CalculateBackdrop contract.
		if backdropRows != nil {
			fr.emitBackdropOnlyTiles(backdropRows, coarse)
		}
		return
	}

	entries := coarse.Entries()
	lines := segments.Segments()

	// Process tiles in sorted order, grouping by location
	coarse.SortEntries()

	// Track which tiles were visited via coarse entries so we can emit
	// backdrop-only tiles afterward. Flat bitmap (cols*tileRows), reused across
	// calls; the map version cost one hash per tile lookup.
	tileCols := int(coarse.TileColumns())
	tileRows := int(coarse.TileRows())
	nTiles := tileCols * tileRows
	if cap(fr.visitedTiles) < nTiles {
		fr.visitedTiles = make([]bool, nTiles)
	} else {
		fr.visitedTiles = fr.visitedTiles[:nTiles]
		clear(fr.visitedTiles)
	}
	visited := fr.visitedTiles

	var currentX, currentY uint16 = 0xFFFF, 0xFFFF
	var tileWinding [TileSize][TileSize]float32

	for _, entry := range entries {
		if entry.X != currentX || entry.Y != currentY {
			// Finalize previous tile if any
			if currentX != 0xFFFF {
				fr.finalizeTile(currentX, currentY, &tileWinding)
			}

			currentX, currentY = entry.X, entry.Y
			visited[int(currentY)*tileCols+int(currentX)] = true
			fr.initTileWinding(&tileWinding, currentX, currentY)
		}

		// Process segment contribution to this tile
		if int(entry.LineIdx) < len(lines) {
			accumulateSegmentWinding(lines[entry.LineIdx], currentX, currentY, &tileWinding)
		}
	}

	// Finalize the last tile
	if currentX != 0xFFFF {
		fr.finalizeTile(currentX, currentY, &tileWinding)
	}

	// Emit tiles for backdrop-only positions (interior tiles with no segments).
	// These tiles have non-zero backdrop from the prefix sum but were never visited
	// because no segments cross them.
	fr.emitBackdropOnlyTilesWithVisited(backdropRows, coarse, visited)
}

// emitBackdropOnlyTiles fills tiles that have non-zero backdrop but no coarse entries.
// This is the fallback path when there are no coarse entries at all.
func (fr *FineRasterizer) emitBackdropOnlyTiles(backdropRows []float32, coarse *CoarseRasterizer) {
	fr.emitBackdropOnlyTilesWithVisited(backdropRows, coarse, nil)
}

// emitBackdropOnlyTilesWithVisited fills tiles that have non-zero backdrop but
// were not visited during coarse entry processing. These are the interior tiles
// of filled shapes — they have no segments crossing them but should be filled
// because the winding number (from edges to the left) is non-zero. The fill is
// done per PIXEL row so partial rows at a shape's start/end stay exact.
func (fr *FineRasterizer) emitBackdropOnlyTilesWithVisited(
	backdropRows []float32,
	coarse *CoarseRasterizer,
	visited []bool,
) {
	if backdropRows == nil {
		return
	}

	cols := int(coarse.TileColumns())
	rows := int(coarse.TileRows())

	var rowValues [TileSize]float32
	for ty := 0; ty < rows; ty++ {
		rowBase := ty * cols
		for tx := 0; tx < cols; tx++ {
			if visited != nil && visited[rowBase+tx] {
				continue // Already processed via coarse entries
			}

			hasBackdrop := false
			for y := 0; y < TileSize; y++ {
				idx := (ty*TileSize+y-fr.backdropRowOffset)*cols + tx
				var v float32
				if idx >= 0 && idx < len(backdropRows) {
					v = backdropRows[idx]
				}
				rowValues[y] = v
				if v != 0 {
					hasBackdrop = true
				}
			}
			if !hasBackdrop {
				continue
			}

			// This tile has non-zero backdrop and no segments — fill it.
			//nolint:gosec // tx, ty bounded by viewport tile dimensions
			tile := fr.grid.GetOrCreate(int32(tx), int32(ty))
			fr.fillTileFromBackdropRows(tile, &rowValues)
		}
	}
}

// initTileWinding initializes the per-pixel winding accumulator of a tile from
// the per-pixel-row backdrop.
func (fr *FineRasterizer) initTileWinding(
	tileWinding *[TileSize][TileSize]float32,
	tileX, tileY uint16,
) {
	cols := int(fr.tileColumns)
	for y := 0; y < TileSize; y++ {
		idx := (int(tileY)*TileSize+y-fr.backdropRowOffset)*cols + int(tileX)
		var v float32
		if fr.backdropRows != nil && idx >= 0 && idx < len(fr.backdropRows) {
			v = fr.backdropRows[idx]
		}
		for x := 0; x < TileSize; x++ {
			tileWinding[y][x] = v
		}
	}
}

// accumulateSegmentWinding adds the exact per-pixel winding contribution of a
// segment to a tile's winding accumulator.
//
// A segment acts as a crossing of the left-to-right ray of every pixel row it
// overlaps, contributing its delta d (line.Winding: +1 upward, -1 downward)
// scaled by the covered height h of that row. Pixels right of the segment
// receive d*h once the row passes the segment's x extent, so the contribution
// splits into:
//
//   - a local term: the exact trapezoid area A of the segment inside each
//     pixel, added only for tiles the segment actually crosses in that row;
//   - the complementary (1-A) share, carried by the precomputed backdrop of
//     the tiles further right (CoarseRasterizer.CalculateBackdrop).
//
// The area uses the antiderivative of clamp(z,0,1) (see clampIntegral), which
// is exact for a straight monotonic segment because x is linear in y.
func accumulateSegmentWinding(
	line LineSegment,
	tileX, tileY uint16,
	tileWinding *[TileSize][TileSize]float32,
) {
	d := float32(line.Winding)
	if d == 0 || line.Y1 <= line.Y0 {
		return // horizontal/degenerate segments carry no winding
	}

	tileLeft := float32(tileX) * float32(TileWidth)
	tileTop := float32(tileY) * float32(TileHeight)
	tileRight := tileLeft + float32(TileWidth)
	slope := (line.X1 - line.X0) / (line.Y1 - line.Y0)

	for row := 0; row < TileHeight; row++ {
		yTop := line.Y0
		if py := tileTop + float32(row); yTop < py {
			yTop = py
		}
		yBot := line.Y1
		if py := tileTop + float32(row) + 1; yBot > py {
			yBot = py
		}
		h := yBot - yTop
		if h <= 0 {
			continue // segment does not cross this pixel row
		}

		xTop := line.X0 + (yTop-line.Y0)*slope
		xBot := line.X0 + (yBot-line.Y0)*slope

		// Only tiles the segment actually crosses in this pixel row take local
		// area; tiles further right are covered by the backdrop.
		if minf32(xTop, xBot) > tileRight || maxf32(xTop, xBot) < tileLeft {
			continue
		}

		for col := 0; col < TileWidth; col++ {
			// Distance from the segment to the right edge of this pixel, at the
			// top and the bottom of the pixel row.
			edgeX := tileLeft + float32(col) + 1
			zTop := edgeX - xTop
			zBot := edgeX - xBot

			var avg float32
			switch {
			case zTop <= 0 && zBot <= 0:
				continue // pixel entirely left of the segment
			case zTop >= 1 && zBot >= 1:
				avg = 1 // pixel entirely right of the segment
			case zTop == zBot:
				avg = clampf32(zTop, 0, 1)
			default:
				avg = (clampIntegral(zTop) - clampIntegral(zBot)) / (zTop - zBot)
			}

			tileWinding[row][col] += d * h * avg
		}
	}
}

// clampIntegral is the antiderivative of clamp(z, 0, 1), used to average the
// clamped coverage over an interval exactly:
//
//	∫ clamp(t,0,1) dt = 0 (z<=0), z²/2 (0<=z<=1), z-1/2 (z>=1)
func clampIntegral(z float32) float32 {
	switch {
	case z <= 0:
		return 0
	case z >= 1:
		return z - 0.5
	default:
		return z * z * 0.5
	}
}

// finalizeTile converts winding values to coverage and stores in grid.
func (fr *FineRasterizer) finalizeTile(
	tileX, tileY uint16,
	tileWinding *[TileSize][TileSize]float32,
) {
	tile := fr.grid.GetOrCreate(int32(tileX), int32(tileY))

	// Convert winding to coverage based on fill rule
	for y := 0; y < TileSize; y++ {
		for x := 0; x < TileSize; x++ {
			coverage := windingCoverage(fr.fillRule, tileWinding[y][x])
			alpha := uint8(coverage*255.0 + 0.5)
			tile.SetCoverage(x, y, alpha)
		}
	}
}

// windingCoverage converts a winding accumulator value to coverage in [0, 1]
// according to the fill rule.
//
// Non-zero: coverage is |winding| (winding accumulates fractional areas, so a
// partially covered pixel naturally lands in (0,1)).
// Even-odd: coverage is |winding - 2*round(winding/2)|, i.e. the distance to
// the nearest even winding count.
func windingCoverage(rule scene.FillStyle, winding float32) float32 {
	var coverage float32

	switch rule {
	case scene.FillEvenOdd:
		absWinding := absf32(winding)
		im1 := float32(int32(absWinding*0.5 + 0.5))
		coverage = absf32(absWinding - 2.0*im1)
	default:
		coverage = absf32(winding)
	}

	if coverage > 1.0 {
		coverage = 1.0
	}
	return coverage
}

// Grid returns the tile grid with computed coverage.
func (fr *FineRasterizer) Grid() *TileGrid {
	return fr.grid
}

// RasterizeCurves processes curve edges directly without pre-flattening.
// This is the retained mode rendering path for scene graph rendering.
//
// Unlike Rasterize() which works with pre-flattened LineSegments,
// this method accepts curve edges from EdgeBuilder and steps through
// their segments during tile processing using forward differencing.
//
// Parameters:
//   - curveBins: Map of tile coordinates (as uint64 key) to curve edge bins (from BinCurveEdges)
//
// The output is stored in the FineRasterizer's TileGrid.
func (fr *FineRasterizer) RasterizeCurves(curveBins map[uint64]*CurveTileBin) {
	fr.grid.Reset()

	if len(curveBins) == 0 {
		return
	}

	// Process each tile with curve edges
	for key, bin := range curveBins {
		// Decode tile coordinates from key
		// Key format: Y in upper 32 bits, X in lower 32 bits
		tileX := int32(key & 0xFFFFFFFF) //nolint:gosec // Safe: extracting lower 32 bits
		tileY := int32(key >> 32)        //nolint:gosec // Safe: extracting upper 32 bits

		// Skip tiles outside viewport
		if tileX < 0 || tileX >= int32(fr.tileColumns) ||
			tileY < 0 || tileY >= int32(fr.tileRows) {
			continue
		}

		//nolint:gosec // tileX, tileY validated above
		fr.processTileWithCurves(uint16(tileX), uint16(tileY), bin.Edges, bin.Backdrop)
	}
}

// processTileWithCurves computes coverage for a single tile using curve edges.
// This is the core of curve-aware fine rasterization.
func (fr *FineRasterizer) processTileWithCurves(
	tileX, tileY uint16,
	edges []raster.CurveEdgeVariant,
	backdrop int32,
) {
	if len(edges) == 0 {
		// No edges - fill with backdrop if non-zero
		if backdrop != 0 {
			tile := fr.grid.GetOrCreate(int32(tileX), int32(tileY))
			fr.fillTileWithBackdrop(tile, backdrop)
		}
		return
	}

	// Initialize the winding accumulator from the tile backdrop
	var tileWinding [TileSize][TileSize]float32

	backdropF := float32(backdrop)
	for y := 0; y < TileSize; y++ {
		for x := 0; x < TileSize; x++ {
			tileWinding[y][x] = backdropF
		}
	}

	// Process each edge
	for i := range edges {
		edge := &edges[i]
		fr.processEdgeForTile(edge, tileX, tileY, &tileWinding)
	}

	// Convert winding to coverage and store in grid
	fr.finalizeTileFromCurves(tileX, tileY, &tileWinding)
}

// processEdgeForTile processes a single edge (line or curve) for a tile.
func (fr *FineRasterizer) processEdgeForTile(
	edge *raster.CurveEdgeVariant,
	tileX, tileY uint16,
	tileWinding *[TileSize][TileSize]float32,
) {
	switch edge.Type {
	case raster.EdgeTypeLine:
		if edge.Line != nil {
			fr.processLineEdgeForTile(edge.Line, tileX, tileY, tileWinding)
		}

	case raster.EdgeTypeQuadratic:
		if edge.Quadratic != nil {
			fr.processQuadraticEdgeForTile(edge.Quadratic, tileX, tileY, tileWinding)
		}

	case raster.EdgeTypeCubic:
		if edge.Cubic != nil {
			fr.processCubicEdgeForTile(edge.Cubic, tileX, tileY, tileWinding)
		}
	}
}

// processLineEdgeForTile processes a simple line edge for a tile.
func (fr *FineRasterizer) processLineEdgeForTile(
	line *raster.LineEdge,
	tileX, tileY uint16,
	tileWinding *[TileSize][TileSize]float32,
) {
	// Convert raster.LineEdge to LineSegment for accumulateSegmentWinding
	segment := fr.lineEdgeToSegment(line, tileX, tileY)
	if segment.Y0 != segment.Y1 { // Skip horizontal segments
		accumulateSegmentWinding(segment, tileX, tileY, tileWinding)
	}
}

// processQuadraticEdgeForTile processes a quadratic curve edge for a tile.
// It steps through the curve segments using forward differencing.
func (fr *FineRasterizer) processQuadraticEdgeForTile(
	quad *raster.QuadraticEdge,
	tileX, tileY uint16,
	tileWinding *[TileSize][TileSize]float32,
) {
	// Process all segments from this quadratic curve
	for quad.CurveCount() > 0 {
		// Current line segment from the curve
		line := quad.Line()
		segment := fr.lineEdgeToSegment(line, tileX, tileY)
		if segment.Y0 != segment.Y1 {
			accumulateSegmentWinding(segment, tileX, tileY, tileWinding)
		}

		// Advance to next segment using forward differencing
		if !quad.Update() {
			break
		}
	}

	// Process the final segment
	line := quad.Line()
	segment := fr.lineEdgeToSegment(line, tileX, tileY)
	if segment.Y0 != segment.Y1 {
		accumulateSegmentWinding(segment, tileX, tileY, tileWinding)
	}
}

// processCubicEdgeForTile processes a cubic curve edge for a tile.
// It steps through the curve segments using forward differencing.
func (fr *FineRasterizer) processCubicEdgeForTile(
	cubic *raster.CubicEdge,
	tileX, tileY uint16,
	tileWinding *[TileSize][TileSize]float32,
) {
	// Cubic uses negative count, active while < 0
	for cubic.CurveCount() < 0 {
		// Current line segment from the curve
		line := cubic.Line()
		segment := fr.lineEdgeToSegment(line, tileX, tileY)
		if segment.Y0 != segment.Y1 {
			accumulateSegmentWinding(segment, tileX, tileY, tileWinding)
		}

		// Advance to next segment using forward differencing
		if !cubic.Update() {
			break
		}
	}

	// Process the final segment
	line := cubic.Line()
	segment := fr.lineEdgeToSegment(line, tileX, tileY)
	if segment.Y0 != segment.Y1 {
		accumulateSegmentWinding(segment, tileX, tileY, tileWinding)
	}
}

// lineEdgeToSegment converts a raster.LineEdge to a LineSegment for
// accumulateSegmentWinding.
//
// raster.LineEdge follows the Skia raster convention (+1 downward, -1 upward),
// while LineSegment stores the vello line-soup crossing delta (+1 upward,
// -1 downward), so the sign is flipped in the conversion.
func (fr *FineRasterizer) lineEdgeToSegment(line *raster.LineEdge, _, _ uint16) LineSegment {
	// Convert raster.FDot16 coordinates to float32 pixel coordinates
	x0 := raster.FDot16ToFloat32(line.X)
	y0 := float32(line.FirstY)
	y1 := float32(line.LastY + 1)

	// Calculate X at the end using slope
	dy := y1 - y0
	dx := raster.FDot16ToFloat32(line.DX)
	x1 := x0 + dx*dy

	return LineSegment{
		X0:      x0,
		Y0:      y0,
		X1:      x1,
		Y1:      y1,
		Winding: -line.Winding,
	}
}

// fillTileWithBackdrop fills a tile with solid coverage based on backdrop winding.
func (fr *FineRasterizer) fillTileWithBackdrop(tile *Tile, backdrop int32) {
	coverage := windingCoverage(fr.fillRule, float32(backdrop))
	if coverage > 0 {
		alpha := uint8(coverage*255.0 + 0.5)
		tile.FillSolid(alpha)
	}
}

// fillTileFromBackdropRows fills a tile from per-pixel-row backdrop values.
// Each pixel row keeps its own coverage so shapes that start or end mid-tile
// stay exact on the rows they do not cover.
func (fr *FineRasterizer) fillTileFromBackdropRows(tile *Tile, rowValues *[TileSize]float32) {
	for y := 0; y < TileSize; y++ {
		coverage := windingCoverage(fr.fillRule, rowValues[y])
		if coverage <= 0 {
			continue
		}
		alpha := uint8(coverage*255.0 + 0.5)
		for x := 0; x < TileSize; x++ {
			tile.SetCoverage(x, y, alpha)
		}
	}
}

// finalizeTileFromCurves converts winding values to coverage and stores in grid.
func (fr *FineRasterizer) finalizeTileFromCurves(
	tileX, tileY uint16,
	tileWinding *[TileSize][TileSize]float32,
) {
	tile := fr.grid.GetOrCreate(int32(tileX), int32(tileY))

	// Convert winding to coverage based on fill rule
	for y := 0; y < TileSize; y++ {
		for x := 0; x < TileSize; x++ {
			coverage := windingCoverage(fr.fillRule, tileWinding[y][x])
			alpha := uint8(coverage*255.0 + 0.5)
			tile.SetCoverage(x, y, alpha)
		}
	}
}

// clampf32 clamps value to [minVal, maxVal] range.
func clampf32(val, minVal, maxVal float32) float32 {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

// RenderToBuffer renders the tile grid to a pixel buffer.
// The buffer is in RGBA format with the given stride (bytes per row).
func (fr *FineRasterizer) RenderToBuffer(
	buffer []uint8,
	width, height int,
	stride int,
	color [4]uint8,
) {
	fr.grid.ForEach(func(tile *Tile) {
		baseX := int(tile.PixelX())
		baseY := int(tile.PixelY())

		for py := 0; py < TileSize; py++ {
			pixelY := baseY + py
			if pixelY < 0 || pixelY >= height {
				continue
			}

			rowOffset := pixelY * stride

			for px := 0; px < TileSize; px++ {
				pixelX := baseX + px
				if pixelX < 0 || pixelX >= width {
					continue
				}

				coverage := tile.GetCoverage(px, py)
				if coverage == 0 {
					continue
				}

				idx := rowOffset + pixelX*4

				// Premultiplied alpha blending
				srcA := uint16(coverage) * uint16(color[3]) / 255
				invA := 255 - srcA

				if idx+3 < len(buffer) {
					//nolint:gosec // Result is bounded 0-255
					buffer[idx+0] = uint8((uint16(color[0])*srcA + uint16(buffer[idx+0])*invA) / 255)
					//nolint:gosec // Result is bounded 0-255
					buffer[idx+1] = uint8((uint16(color[1])*srcA + uint16(buffer[idx+1])*invA) / 255)
					//nolint:gosec // Result is bounded 0-255
					buffer[idx+2] = uint8((uint16(color[2])*srcA + uint16(buffer[idx+2])*invA) / 255)
					//nolint:gosec // Result is bounded 0-255
					buffer[idx+3] = uint8(srcA + uint16(buffer[idx+3])*invA/255)
				}
			}
		}
	})
}

// SparseStrip represents a sparse strip for efficient rendering.
// A strip is a horizontal run of adjacent tiles with the same Y coordinate.
// This is different from the legacy Strip type in strips.go.
type SparseStrip struct {
	X        uint16 // X coordinate in pixels
	Y        uint16 // Y coordinate in pixels
	AlphaIdx uint32 // Index into alpha buffer
}

// StripRenderer renders tiles as sparse strips.
type StripRenderer struct {
	strips    []SparseStrip
	alphas    []uint8
	fillRule  scene.FillStyle
	aliasMode bool // If true, use hard edges instead of anti-aliasing
}

// NewStripRenderer creates a new strip renderer.
func NewStripRenderer() *StripRenderer {
	return &StripRenderer{
		strips:   make([]SparseStrip, 0, 256),
		alphas:   make([]uint8, 0, 4096),
		fillRule: scene.FillNonZero,
	}
}

// Reset clears the renderer state for reuse.
func (sr *StripRenderer) Reset() {
	sr.strips = sr.strips[:0]
	sr.alphas = sr.alphas[:0]
}

// SetFillRule sets the fill rule.
func (sr *StripRenderer) SetFillRule(rule scene.FillStyle) {
	sr.fillRule = rule
}

// SetAliasMode enables or disables aliased (non-anti-aliased) rendering.
func (sr *StripRenderer) SetAliasMode(enabled bool) {
	sr.aliasMode = enabled
}

// Strips returns the strips.
func (sr *StripRenderer) Strips() []SparseStrip {
	return sr.strips
}

// Alphas returns the alpha buffer.
func (sr *StripRenderer) Alphas() []uint8 {
	return sr.alphas
}

// RenderTiles converts tiles to strips for efficient rendering.
//
// Tiles are visited in row-major order. A tile joins the open strip when it is
// horizontally adjacent to the previous tile on the same row; otherwise the
// open strip is closed with a row-end sentinel and a new strip starts. Every
// tile with non-zero coverage is emitted exactly once and the alpha buffers of
// adjacent tiles concatenate into one strip: tiles crossed by segments
// accumulate local coverage, backdrop-only tiles (interior of shapes) seed
// from the per-pixel-row backdrop.
func (sr *StripRenderer) RenderTiles(
	coarse *CoarseRasterizer,
	segments *SegmentList,
	backdropRows []float32,
) {
	sr.Reset()

	if segments == nil || len(coarse.Entries()) == 0 {
		return
	}

	entries := coarse.Entries()
	lines := segments.Segments()
	coarse.SortEntries()

	cols := int(coarse.TileColumns())
	rows := int(coarse.TileRows())
	rowOffset := coarse.BackdropRowOffset()

	var tileWinding [TileSize][TileSize]float32
	var lastX, lastY uint16
	entryIdx := 0
	stripOpen := false

	closeStrip := func() {
		sr.emitSentinel(lastY)
		stripOpen = false
	}

	for ty := 0; ty < rows; ty++ {
		for tx := 0; tx < cols; tx++ {
			// Seed the winding accumulator from the per-pixel-row backdrop.
			seeded := false
			for y := 0; y < TileSize; y++ {
				idx := (ty*TileSize+y-rowOffset)*cols + tx
				var v float32
				if backdropRows != nil && idx >= 0 && idx < len(backdropRows) {
					v = backdropRows[idx]
				}
				if v != 0 {
					seeded = true
				}
				for x := 0; x < TileSize; x++ {
					tileWinding[y][x] = v
				}
			}

			// Local contributions from all segments binned into this tile.
			hasEntries := false
			for entryIdx < len(entries) {
				entry := entries[entryIdx]
				if int(entry.Y) > ty {
					break
				}
				if int(entry.Y) < ty {
					entryIdx++ // stale entry left behind by the sweep
					continue
				}
				if int(entry.X) > tx {
					break
				}
				entryIdx++
				if int(entry.X) == tx {
					hasEntries = true
					if int(entry.LineIdx) < len(lines) {
						//nolint:gosec // tx, ty bounded by viewport tile dimensions
						accumulateSegmentWinding(lines[entry.LineIdx], uint16(tx), uint16(ty), &tileWinding)
					}
				}
			}

			if !hasEntries && !seeded {
				continue
			}

			alphas, covered := sr.tileToAlphas(&tileWinding)
			if !covered {
				continue
			}

			if !stripOpen || ty != int(lastY) || tx != int(lastX)+1 {
				if stripOpen {
					closeStrip()
				}
				//nolint:gosec // tx, ty bounded by viewport tile dimensions
				sr.startStrip(uint16(tx), uint16(ty))
				stripOpen = true
			}
			sr.alphas = append(sr.alphas, alphas[:]...)
			//nolint:gosec // tx, ty bounded by viewport tile dimensions
			lastX, lastY = uint16(tx), uint16(ty)
		}
		if stripOpen {
			closeStrip()
		}
	}
}

// startStrip begins a new strip at the given tile location.
func (sr *StripRenderer) startStrip(tileX, tileY uint16) {
	sr.strips = append(sr.strips, SparseStrip{
		X:        tileX * TileWidth,
		Y:        tileY * TileHeight,
		AlphaIdx: uint32(len(sr.alphas)), //nolint:gosec // alphas length bounded by viewport
	})
}

// emitSentinel emits a sentinel strip marking the end of the strip that shares
// the given tile row.
func (sr *StripRenderer) emitSentinel(tileY uint16) {
	sr.strips = append(sr.strips, SparseStrip{
		X:        0xFFFF,
		Y:        tileY * TileHeight,
		AlphaIdx: uint32(len(sr.alphas)), //nolint:gosec // alphas length bounded by viewport
	})
}

// tileToAlphas converts a tile's winding accumulator to 8-bit alpha values in
// column-major order (column x holds rows 0..3, matching the strip renderer's
// layout) and reports whether any pixel is covered.
func (sr *StripRenderer) tileToAlphas(tileWinding *[TileSize][TileSize]float32) ([TileSize * TileSize]uint8, bool) {
	var alphas [TileSize * TileSize]uint8
	covered := false

	for x := 0; x < TileWidth; x++ {
		for y := 0; y < TileHeight; y++ {
			coverage := windingCoverage(sr.fillRule, tileWinding[y][x])

			// Apply aliasing threshold if enabled
			if sr.aliasMode {
				if coverage >= 0.5 {
					coverage = 1.0
				} else {
					coverage = 0.0
				}
			}

			alpha := uint8(coverage*255.0 + 0.5)
			alphas[x*TileHeight+y] = alpha
			if alpha != 0 {
				covered = true
			}
		}
	}

	return alphas, covered
}
