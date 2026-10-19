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
	"sort"
)

// TileWidth is the width of a tile in pixels (matches TileSize).
const TileWidth = TileSize

// TileHeight is the height of a tile in pixels (matches TileSize).
const TileHeight = TileSize

// coarseHorizontalEpsilon bounds the Y-span below which a segment carries no
// usable winding in the 4x4 tile pipeline: winding is accumulated as d*h over
// the pixel rows a segment covers, so |dy| < 1e-6 contributes < 1/255000 of a
// row. Skipping these in the coarse pass also keeps near-horizontal slivers
// (boolean-op outputs contain thousands with 1e-7-scale slopes) from enumerating
// one entry per tile column they span, which otherwise floods the entry list.
// The vello line soup keeps such segments (tilecompute consumes the full
// contour); only the per-tile-column entry binning skips them.
const coarseHorizontalEpsilon = 1e-6

// CoarseRasterizer performs coarse rasterization of line segments into tiles.
// It bins segments into the tiles they intersect and tracks winding information.
type CoarseRasterizer struct {
	grid     *TileGrid
	segments *SegmentList
	entries  []CoarseTileEntry

	// Viewport dimensions in pixels
	width  uint16
	height uint16

	// Tile dimensions
	tileColumns uint16
	tileRows    uint16

	// backdropRowOffset is the first pixel row covered by the most recent
	// CalculateBackdrop result. Rows outside the segments' Y extent carry zero
	// winding and are not materialized (see BackdropRowOffset).
	backdropRowOffset int

	// backdropBuf and backdropMinKBuf are the per-pixel-row backdrop scratch
	// buffers, reused across fills (see CalculateBackdrop).
	backdropBuf     []float32
	backdropMinKBuf []int32
}

// CoarseTileEntry represents a tile with its associated line segment.
// This is used during the coarse rasterization phase before fine rasterization.
type CoarseTileEntry struct {
	X       uint16 // Tile X coordinate
	Y       uint16 // Tile Y coordinate
	LineIdx uint32 // Index into segment list
	Winding bool   // True if segment contributes winding at this tile
}

// NewCoarseRasterizer creates a new coarse rasterizer for the given dimensions.
func NewCoarseRasterizer(width, height uint16) *CoarseRasterizer {
	cr := &CoarseRasterizer{
		width:       width,
		height:      height,
		tileColumns: (width + TileWidth - 1) / TileWidth,
		tileRows:    (height + TileHeight - 1) / TileHeight,
		entries:     make([]CoarseTileEntry, 0, 256),
	}
	cr.grid = NewTileGrid()
	return cr
}

// Reset clears the rasterizer state for reuse.
func (cr *CoarseRasterizer) Reset() {
	cr.grid.Reset()
	cr.segments = nil
	cr.entries = cr.entries[:0]
	cr.backdropRowOffset = 0
}

// Rasterize performs coarse rasterization of segments into tiles.
// It determines which tiles each segment intersects and calculates winding.
func (cr *CoarseRasterizer) Rasterize(segments *SegmentList) {
	cr.segments = segments
	cr.entries = cr.entries[:0]

	if segments == nil || segments.Len() == 0 {
		return
	}

	if cr.width == 0 || cr.height == 0 {
		return
	}

	lines := segments.Segments()
	for lineIdx, line := range lines {
		//nolint:gosec // lineIdx is bounded by segments length which fits in uint32
		cr.rasterizeLine(line, uint32(lineIdx))
	}
}

// rasterizeLine determines which tiles a single line segment intersects.
// This implements the coarse binning phase from vello.
func (cr *CoarseRasterizer) rasterizeLine(line LineSegment, lineIdx uint32) {
	// Segments are Y-normalized (Y0 <= Y1); skip near-horizontal ones before the
	// tile math (see coarseHorizontalEpsilon).
	if line.Y1-line.Y0 < coarseHorizontalEpsilon {
		return
	}

	// Convert to tile coordinates
	p0x := line.X0 / float32(TileWidth)
	p0y := line.Y0 / float32(TileHeight)
	p1x := line.X1 / float32(TileWidth)
	p1y := line.Y1 / float32(TileHeight)

	// Determine left/right bounds
	lineLeftX := minf32(p0x, p1x)
	lineRightX := maxf32(p0x, p1x)

	// Cull lines fully to the right of viewport
	if lineLeftX > float32(cr.tileColumns) {
		return
	}

	// Determine top/bottom (lines are guaranteed monotonic Y: Y0 <= Y1)
	lineTopY := p0y
	lineTopX := p0x
	lineBottomY := p1y
	lineBottomX := p1x

	// Clamp to viewport rows
	yTopTiles := clampU16(int32(lineTopY), 0, int32(cr.tileRows))
	yBottomTiles := clampU16(int32(lineBottomY+0.999999), 0, int32(cr.tileRows))

	// Skip horizontal lines or lines fully outside viewport
	if yTopTiles >= yBottomTiles {
		return
	}

	// Get tile coordinates for endpoints
	p0TileX := int32(lineTopX)
	p0TileY := int32(lineTopY)
	p1TileX := int32(lineBottomX)
	p1TileY := int32(lineBottomY)

	// Check if both endpoints are in the same tile
	sameX := p0TileX == p1TileX
	sameY := p0TileY == p1TileY

	if sameX && sameY {
		// Line fully contained in single tile
		x := clampU16(int32(lineLeftX), 0, int32(cr.tileColumns-1))
		// Set winding if line crosses tile top edge
		winding := p0TileY >= int32(yTopTiles)
		cr.addEntry(x, yTopTiles, lineIdx, winding)
		return
	}

	// Handle vertical lines specially
	if lineLeftX == lineRightX {
		cr.rasterizeVerticalLine(lineLeftX, lineTopY, lineBottomY, lineIdx)
		return
	}

	// General sloped line
	cr.rasterizeSlopedLine(lineIdx, lineTopX, lineTopY, lineBottomX, lineBottomY, yTopTiles, yBottomTiles)
}

// addEntry adds a coarse tile entry.
func (cr *CoarseRasterizer) addEntry(x, y uint16, lineIdx uint32, winding bool) {
	cr.entries = append(cr.entries, CoarseTileEntry{
		X:       x,
		Y:       y,
		LineIdx: lineIdx,
		Winding: winding,
	})
}

// rasterizeVerticalLine handles vertical line segments.
func (cr *CoarseRasterizer) rasterizeVerticalLine(x float32, topY, bottomY float32, lineIdx uint32) {
	xTile := clampU16(int32(x), 0, int32(cr.tileColumns-1))

	yTopTiles := clampU16(int32(topY), 0, int32(cr.tileRows))
	yBottomTiles := clampU16(int32(bottomY+0.999999), 0, int32(cr.tileRows))

	if yTopTiles >= yBottomTiles {
		return
	}

	// First tile - check if line starts above tile top
	isStartCulled := topY < 0
	if !isStartCulled {
		winding := float32(yTopTiles) >= topY
		cr.addEntry(xTile, yTopTiles, lineIdx, winding)
	}

	// Middle tiles - line crosses top and bottom
	yStart := yTopTiles
	if !isStartCulled {
		yStart++
	}
	yEndIdx := clampU16(int32(bottomY), 0, int32(cr.tileRows))

	for y := yStart; y < yEndIdx; y++ {
		cr.addEntry(xTile, y, lineIdx, true) // Winding always true for middle tiles
	}

	// Last tile if line doesn't end exactly on tile boundary
	bottomFloor := float32(int32(bottomY))
	if bottomY != bottomFloor && yEndIdx < cr.tileRows {
		cr.addEntry(xTile, yEndIdx, lineIdx, true)
	}
}

// rasterizeSlopedLine handles general sloped line segments.
func (cr *CoarseRasterizer) rasterizeSlopedLine(
	lineIdx uint32,
	lineTopX, lineTopY, lineBottomX, lineBottomY float32,
	yTopTiles, _ uint16, // yBottomTiles - reserved for future clipping
) {
	dx := lineBottomX - lineTopX
	dy := lineBottomY - lineTopY
	xSlope := dx / dy

	// Determine winding direction based on slope direction
	dxDir := lineBottomX >= lineTopX

	lineLeftX := minf32(lineTopX, lineBottomX)
	lineRightX := maxf32(lineTopX, lineBottomX)

	isStartCulled := lineTopY < 0

	// Process first row (if not culled)
	if !isStartCulled {
		y := float32(yTopTiles)
		rowBottomY := minf32(y+1.0, lineBottomY)
		winding := y >= lineTopY
		cr.processRow(lineIdx, lineTopY, y, rowBottomY, xSlope, lineTopX, lineLeftX, lineRightX, yTopTiles, winding, dxDir)
	}

	// Process middle rows
	yStartMiddle := yTopTiles
	if !isStartCulled {
		yStartMiddle++
	}
	yEndMiddle := clampU16(int32(lineBottomY), 0, int32(cr.tileRows))

	for y := yStartMiddle; y < yEndMiddle; y++ {
		yf := float32(y)
		rowBottomY := minf32(yf+1.0, lineBottomY)
		cr.processRow(lineIdx, lineTopY, yf, rowBottomY, xSlope, lineTopX, lineLeftX, lineRightX, y, true, dxDir)
	}

	// Process last row if line doesn't end on row boundary
	bottomFloor := float32(int32(lineBottomY))
	if lineBottomY != bottomFloor && yEndMiddle < cr.tileRows {
		if isStartCulled || yEndMiddle != yTopTiles {
			yf := float32(yEndMiddle)
			cr.processRow(lineIdx, lineTopY, yf, lineBottomY, xSlope, lineTopX, lineLeftX, lineRightX, yEndMiddle, true, dxDir)
		}
	}
}

// processRow processes a single row of tiles for a sloped line.
func (cr *CoarseRasterizer) processRow(
	lineIdx uint32,
	lineTopY float32, // Original line top Y for calculating X positions
	rowTopY, rowBottomY float32,
	xSlope, lineTopX float32,
	lineLeftX, lineRightX float32,
	yIdx uint16,
	winding bool,
	dxDir bool,
) {
	// Calculate X range for this row using line equation
	// x = lineTopX + (y - lineTopY) * xSlope
	rowTopX := lineTopX + (rowTopY-lineTopY)*xSlope
	rowBottomX := lineTopX + (rowBottomY-lineTopY)*xSlope

	// Clamp to line bounds
	rowLeftX := maxf32(minf32(rowTopX, rowBottomX), lineLeftX)
	rowRightX := minf32(maxf32(rowTopX, rowBottomX), lineRightX)

	xStart := clampU16(int32(rowLeftX), 0, int32(cr.tileColumns-1))
	xEnd := clampU16(int32(rowRightX), 0, int32(cr.tileColumns-1))

	if xStart > xEnd {
		return
	}

	// Single tile case
	if xStart == xEnd {
		cr.addEntry(xStart, yIdx, lineIdx, winding)
		return
	}

	// Multiple tiles
	// First tile gets winding based on direction
	if dxDir {
		// Going right: left tile gets winding
		cr.addEntry(xStart, yIdx, lineIdx, winding)
	} else {
		// Going left: right tile gets winding
		cr.addEntry(xStart, yIdx, lineIdx, false)
	}

	// Middle tiles (no winding)
	for x := xStart + 1; x < xEnd; x++ {
		cr.addEntry(x, yIdx, lineIdx, false)
	}

	// Last tile
	if dxDir {
		cr.addEntry(xEnd, yIdx, lineIdx, false)
	} else {
		cr.addEntry(xEnd, yIdx, lineIdx, winding)
	}
}

// Grid returns the tile grid after rasterization.
func (cr *CoarseRasterizer) Grid() *TileGrid {
	return cr.grid
}

// Entries returns the coarse tile entries.
func (cr *CoarseRasterizer) Entries() []CoarseTileEntry {
	return cr.entries
}

// Segments returns the segment list.
func (cr *CoarseRasterizer) Segments() *SegmentList {
	return cr.segments
}

// SortEntries sorts the entries for efficient rendering.
// Tiles are sorted by Y, then X, then line index.
func (cr *CoarseRasterizer) SortEntries() {
	sort.Slice(cr.entries, func(i, j int) bool {
		ei, ej := cr.entries[i], cr.entries[j]
		if ei.Y != ej.Y {
			return ei.Y < ej.Y
		}
		if ei.X != ej.X {
			return ei.X < ej.X
		}
		return ei.LineIdx < ej.LineIdx
	})
}

// TileColumns returns the number of tile columns.
func (cr *CoarseRasterizer) TileColumns() uint16 {
	return cr.tileColumns
}

// TileRows returns the number of tile rows.
func (cr *CoarseRasterizer) TileRows() uint16 {
	return cr.tileRows
}

// CalculateBackdrop calculates the backdrop winding for fine rasterization.
//
// The result is indexed by [pixelY * tileColumns + tileX]: one value per PIXEL
// row (not per tile row), holding the accumulated winding of all segments
// strictly to the LEFT of tileX at pixel row pixelY. This is vello's backdrop
// ("winding state before this tile's own segments"), refined to pixel-row
// granularity so that shapes whose edges start or end mid-tile stay exact on
// the rows they do not cover.
//
// For every pixel row a segment overlaps, the segment crosses the
// left-to-right ray of that row with its delta d (Winding: +1 upward, -1
// downward) times the covered height h. All tiles fully right of the segment
// accumulate d*h, so the delta is added at the first tile right of the
// segment's maximum X for that row and prefix-summed to the right edge.
// Tiles the segment actually crosses get the exact local area from the fine
// rasterizer instead and are never double counted here.
func (cr *CoarseRasterizer) CalculateBackdrop() []float32 {
	if cr.segments == nil || len(cr.entries) == 0 {
		cr.backdropRowOffset = 0
		return nil
	}

	cols := int(cr.tileColumns)
	pixelRows := int(cr.tileRows) * TileHeight

	// Rows outside the segments' Y extent are identically zero: every
	// contribution is bounded by the Y span of the segment that produces it.
	// Crop the per-pixel-row prefix sum to that range so shapes that do not
	// span the canvas pay only for their own rows.
	rowStart, rowEnd := cr.backdropRowRange(pixelRows)
	if rowEnd <= rowStart {
		cr.backdropRowOffset = 0
		return nil
	}
	cr.backdropRowOffset = rowStart
	numRows := rowEnd - rowStart

	// Reuse the scratch buffers across fills (rasterizers are pooled and each
	// fill consumes the returned slice before the next call). Zeroing before the
	// prefix sum keeps rows that receive no delta at zero.
	need := cols * numRows
	if cap(cr.backdropBuf) < need {
		cr.backdropBuf = make([]float32, need)
	} else {
		cr.backdropBuf = cr.backdropBuf[:need]
	}
	backdrop := cr.backdropBuf
	clear(backdrop)

	// Per-pixel-row leftmost tile that received a delta; the prefix sum only
	// needs to start there.
	minK := cr.backdropMinKBuf
	if cap(minK) < numRows {
		minK = make([]int32, numRows)
	}
	minK = minK[:numRows]
	cr.backdropMinKBuf = minK
	for i := range minK {
		minK[i] = int32(cols)
	}

	width := float32(cr.width)
	lines := cr.segments.Segments()

	for idx := range lines {
		line := &lines[idx]
		d := float32(line.Winding)
		if d == 0 || line.Y1 <= line.Y0 {
			continue // horizontal/degenerate segments carry no winding
		}

		slope := (line.X1 - line.X0) / (line.Y1 - line.Y0)

		yStart := int32(line.Y0)
		yEnd := int32(line.Y1)
		if line.Y0 < 0 {
			yStart = 0
		}
		if line.Y0 < float32(rowStart) {
			yStart = int32(rowStart)
		}
		if yEnd >= int32(rowEnd) {
			yEnd = int32(rowEnd) - 1
		}
		if yStart > yEnd {
			continue
		}

		for py := yStart; py <= yEnd; py++ {
			yTop := line.Y0
			if yf := float32(py); yTop < yf {
				yTop = yf
			}
			yBot := line.Y1
			if yf := float32(py) + 1; yBot > yf {
				yBot = yf
			}
			h := yBot - yTop
			if h <= 0 {
				continue
			}

			xTop := line.X0 + (yTop-line.Y0)*slope
			xBot := line.X0 + (yBot-line.Y0)*slope
			xhi := xTop
			if xBot > xhi {
				xhi = xBot
			}

			if xhi >= width {
				continue // segment is right of the viewport on this row
			}

			// First tile right of the segment's maximum X for this pixel row.
			k := int32(0)
			if xhi >= 0 {
				k = int32(xhi/float32(TileWidth)) + 1
			}
			if k >= int32(cols) {
				continue
			}

			row := int(py) - rowStart
			backdrop[row*cols+int(k)] += d * h
			if k < minK[row] {
				minK[row] = k
			}
		}
	}

	// Prefix sum per pixel row: every tile right of a segment inherits its delta.
	for row := 0; row < numRows; row++ {
		k := minK[row]
		if k >= int32(cols) {
			continue
		}
		base := row * cols
		var sum float32
		for tx := int(k); tx < cols; tx++ {
			sum += backdrop[base+tx]
			backdrop[base+tx] = sum
		}
	}

	return backdrop
}

// BackdropRowOffset returns the first pixel row covered by the slice returned
// from the most recent CalculateBackdrop call. Rows below it are zero; callers
// indexing the backdrop must subtract the offset (row 0 of the slice is pixel
// row backdropRowOffset on the canvas).
func (cr *CoarseRasterizer) BackdropRowOffset() int {
	return cr.backdropRowOffset
}

// backdropRowRange returns the pixel-row range [rowStart, rowEnd) that can hold
// non-zero backdrop winding: the segments' Y extent, clamped to the viewport.
// Degenerate bounds (no segments, NaN, inverted) fall back to the full range.
func (cr *CoarseRasterizer) backdropRowRange(pixelRows int) (int, int) {
	_, minY, _, maxY := cr.segments.Bounds()
	if !(minY <= maxY) { // NaN or inverted bounds
		return 0, pixelRows
	}
	fRows := float32(pixelRows)

	rowStart := 0
	if minY > 0 {
		if minY > fRows {
			minY = fRows
		}
		rowStart = int(minY) // truncation is floor for positive values
	}

	rowEnd := pixelRows
	if maxY < fRows {
		rowEnd = int(maxY)
		if float32(rowEnd) < maxY {
			rowEnd++ // partial bottom row
		}
	}
	if rowEnd < rowStart {
		rowEnd = rowStart
	}
	return rowStart, rowEnd
}

// clampU16 clamps value to [minVal, maxVal] range and returns as uint16.
//
//nolint:gosec,unparam // Integer overflow is acceptable; minVal kept for API flexibility
func clampU16(val, minVal, maxVal int32) uint16 {
	if val < minVal {
		return uint16(minVal)
	}
	if val > maxVal {
		return uint16(maxVal)
	}
	return uint16(val)
}

// CoarseTileIterator provides iteration over coarse tiles in sorted order.
type CoarseTileIterator struct {
	rasterizer *CoarseRasterizer
	index      int
	sorted     bool
}

// NewIterator creates an iterator for the coarse tile entries.
func (cr *CoarseRasterizer) NewIterator() *CoarseTileIterator {
	if !cr.isSorted() {
		cr.SortEntries()
	}
	return &CoarseTileIterator{
		rasterizer: cr,
		index:      0,
		sorted:     true,
	}
}

// isSorted returns true if entries are sorted (simple heuristic).
func (cr *CoarseRasterizer) isSorted() bool {
	if len(cr.entries) < 2 {
		return true
	}
	// Check a few entries as a heuristic
	for i := 1; i < len(cr.entries) && i < 10; i++ {
		ei, ej := cr.entries[i-1], cr.entries[i]
		if ei.Y > ej.Y || (ei.Y == ej.Y && ei.X > ej.X) {
			return false
		}
	}
	return true
}

// Next returns the next tile entry or nil if done.
func (it *CoarseTileIterator) Next() *CoarseTileEntry {
	if it.index >= len(it.rasterizer.entries) {
		return nil
	}
	entry := &it.rasterizer.entries[it.index]
	it.index++
	return entry
}

// HasNext returns true if there are more entries.
func (it *CoarseTileIterator) HasNext() bool {
	return it.index < len(it.rasterizer.entries)
}

// Reset resets the iterator to the beginning.
func (it *CoarseTileIterator) Reset() {
	it.index = 0
}

// EntriesAtLocation returns all entries at the given tile location.
func (cr *CoarseRasterizer) EntriesAtLocation(x, y uint16) []CoarseTileEntry {
	var result []CoarseTileEntry
	for _, e := range cr.entries {
		if e.X == x && e.Y == y {
			result = append(result, e)
		}
	}
	return result
}

// CurveTileBin holds curve edges binned to a specific tile.
type CurveTileBin struct {
	Edges    []raster.CurveEdgeVariant
	Backdrop int32
}

// BinCurveEdges bins curve edges from an EdgeBuilder to tiles based on their Y bounds.
// Unlike RasterizeLineSegments, this preserves curve information for analytic rendering.
//
// Returns a map from tile coordinates (as uint64 key) to the edges that intersect each tile.
// This is used by FineRasterizer.RasterizeCurves for retained mode rendering.
func (cr *CoarseRasterizer) BinCurveEdges(eb *raster.EdgeBuilder) map[uint64]*CurveTileBin {
	if eb == nil || eb.IsEmpty() {
		return nil
	}

	result := make(map[uint64]*CurveTileBin)

	// Process all edges from the EdgeBuilder
	for edge := range eb.AllEdges() {
		cr.binSingleCurveEdge(edge, result)
	}

	// Calculate backdrop for each tile
	cr.calculateCurveBackdrops(result)

	return result
}

// binSingleCurveEdge bins a single curve edge to all tiles it intersects.
func (cr *CoarseRasterizer) binSingleCurveEdge(edge raster.CurveEdgeVariant, bins map[uint64]*CurveTileBin) {
	line := edge.AsLine()
	if line == nil {
		return
	}

	// Get Y range in tiles
	yTop := line.FirstY >> TileShift
	yBot := (line.LastY + TileSize) >> TileShift

	// Clamp to viewport
	if yTop < 0 {
		yTop = 0
	}
	if yBot > int32(cr.tileRows) {
		yBot = int32(cr.tileRows)
	}

	// Get X range from the line's fixed-point X coordinate
	// For curves, we need conservative X bounds
	xMin, xMax := cr.getCurveXBounds(edge)

	xTileMin := xMin >> TileShift
	xTileMax := (xMax + TileSize - 1) >> TileShift

	// Clamp to viewport
	if xTileMin < 0 {
		xTileMin = 0
	}
	if xTileMax > int32(cr.tileColumns) {
		xTileMax = int32(cr.tileColumns)
	}

	// Bin to all intersecting tiles
	for ty := yTop; ty < yBot; ty++ {
		for tx := xTileMin; tx <= xTileMax; tx++ {
			coord := TileCoord{X: tx, Y: ty}
			key := coord.Key()
			bin := bins[key]
			if bin == nil {
				bin = &CurveTileBin{
					Edges: make([]raster.CurveEdgeVariant, 0, 4),
				}
				bins[key] = bin
			}
			// Clone the edge for this tile (each tile processes independently)
			bin.Edges = append(bin.Edges, cr.cloneCurveEdge(edge))
		}
	}
}

// getCurveXBounds returns conservative X bounds for a curve edge in pixels.
func (cr *CoarseRasterizer) getCurveXBounds(edge raster.CurveEdgeVariant) (xMin, xMax int32) {
	line := edge.AsLine()
	if line == nil {
		return 0, 0
	}

	// Convert raster.FDot16 X to pixel coordinates
	xStart := raster.FDot16FloorToInt(line.X)

	// For curves, estimate the X range based on edge type
	// The DX gives us slope, so we can estimate X at the end
	dy := line.LastY - line.FirstY + 1
	xAtEnd := raster.FDot16FloorToInt(line.X + line.DX*dy)

	if xAtEnd < xStart {
		xMin = xAtEnd
		xMax = xStart
	} else {
		xMin = xStart
		xMax = xAtEnd
	}

	// Add margin for curve deviation
	switch edge.Type {
	case raster.EdgeTypeQuadratic, raster.EdgeTypeCubic:
		// Curves can deviate from the line segment
		// Add conservative margin based on bounding box
		margin := int32(2 * TileSize)
		xMin -= margin
		xMax += margin
	}

	return xMin, xMax
}

// cloneCurveEdge creates a copy of a curve edge for independent processing.
func (cr *CoarseRasterizer) cloneCurveEdge(edge raster.CurveEdgeVariant) raster.CurveEdgeVariant {
	switch edge.Type {
	case raster.EdgeTypeLine:
		if edge.Line != nil {
			lineCopy := *edge.Line
			return raster.CurveEdgeVariant{
				Type: raster.EdgeTypeLine,
				Line: &lineCopy,
			}
		}
	case raster.EdgeTypeQuadratic:
		if edge.Quadratic != nil {
			quadCopy := *edge.Quadratic
			return raster.CurveEdgeVariant{
				Type:      raster.EdgeTypeQuadratic,
				Quadratic: &quadCopy,
			}
		}
	case raster.EdgeTypeCubic:
		if edge.Cubic != nil {
			cubicCopy := *edge.Cubic
			return raster.CurveEdgeVariant{
				Type:  raster.EdgeTypeCubic,
				Cubic: &cubicCopy,
			}
		}
	}
	return edge
}

// calculateCurveBackdrops calculates backdrop winding for curve tiles.
// Backdrop is the accumulated winding from edges passing to the left of each tile.
func (cr *CoarseRasterizer) calculateCurveBackdrops(bins map[uint64]*CurveTileBin) {
	if len(bins) == 0 {
		return
	}

	// Collect all tile Y values and sort
	yValues := make(map[int32]bool)
	for key := range bins {
		// Decode Y from key (upper 32 bits)
		//nolint:gosec // Safe: Y coordinate fits in int32 (from TileCoord.Key)
		tileY := int32(key >> 32)
		yValues[tileY] = true
	}

	// For each row, accumulate winding from left to right
	for ty := range yValues {
		rowWinding := int32(0)
		// Process tiles in X order for this row
		for tx := int32(0); tx < int32(cr.tileColumns); tx++ {
			coord := TileCoord{X: tx, Y: ty}
			key := coord.Key()
			bin := bins[key]
			if bin != nil {
				bin.Backdrop = rowWinding
				// Update winding based on edges that cross this tile
				for _, edge := range bin.Edges {
					line := edge.AsLine()
					if line != nil {
						// Count edges that contribute winding at this tile's left edge
						tileLeftX := tx << TileShift
						edgeX := raster.FDot16FloorToInt(line.X)
						if edgeX <= tileLeftX {
							rowWinding += int32(line.Winding)
						}
					}
				}
			}
		}
	}
}
