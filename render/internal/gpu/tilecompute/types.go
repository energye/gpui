//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package tilecompute

// Tile dimensions (matching Vello TILE_WIDTH/TILE_HEIGHT).
const (
	TileWidth  = 16
	TileHeight = 16
	tileScale  = 1.0 / 16.0
)

// LineSoup matches vello_encoding::LineSoup.
// Lines are stored in their ORIGINAL direction (NOT pre-sorted by Y).
type LineSoup struct {
	PathIx uint32
	P0     [2]float32 // Start point in pixel coordinates
	P1     [2]float32 // End point in pixel coordinates
}

// Path matches vello_encoding::Path.
type Path struct {
	BBox  [4]uint32 // Bounding box in TILE coordinates [x0, y0, x1, y1]
	Tiles uint32    // Offset into the Tile array
}

// Tile matches vello_encoding::Tile.
type Tile struct {
	Backdrop         int32
	SegmentCountOrIx uint32 // Count in path_count, inverted index after coarse
}

// SegmentCount matches vello_encoding::SegmentCount.
// Bridge between path_count (stage 1) and path_tiling (stage 2).
type SegmentCount struct {
	LineIx uint32 // Index into LineSoup array
	Counts uint32 // (seg_within_slice << 16) | seg_within_line
}

// PathSegment matches vello_encoding::PathSegment.
// Coordinates are tile-relative.
type PathSegment struct {
	Point0 [2]float32 // Tile-relative start point
	Point1 [2]float32 // Tile-relative end point
	YEdge  float32    // Y where segment touches x=0 (tile-relative), or 1e9
}

// lineDDA holds the per-line DDA parameters shared by path_count (stage 1) and
// path_tiling (stage 2).
//
// vello recomputes these parameters in both compute shaders because separate
// GPU stages cannot share memory (the recomputation in path_tiling is documented
// as "identical to path_count"). On the CPU the values are computed once per
// line and reused for every (line, tile) instance, which is bit-identical to
// recomputing them.
type lineDDA struct {
	xy0, xy1        vec2    // Endpoints ordered top-to-bottom (xy0.y <= xy1.y)
	isDown          bool    // Original line points downward (p1.y >= p0.y)
	isPositiveSlope bool    // s1.x >= s0.x in tile space
	count           uint32  // Tile-crossing segment count (robust DDA estimate)
	a, b            float32 // DDA slope/intercept in "i" space
	sign, x0, y0    float32 // DDA walk origin and direction
	pathIx          uint32  // Index into the paths array
	pathTiles       int32   // Path.Tiles base offset
	bboxi           [4]int32
	stride          int32
}

// BumpAllocators matches vello_encoding::BumpAllocators (subset).
type BumpAllocators struct {
	Lines     uint32
	SegCounts uint32
	Segments  uint32
}

// FillRule for winding number computation.
type FillRule int

const (
	FillRuleNonZero FillRule = iota
	FillRuleEvenOdd
)

// PathDef defines a single path for multi-path rendering.
// Color is in straight (non-premultiplied) RGBA format.
type PathDef struct {
	Lines    []LineSoup
	Color    [4]uint8 // RGBA (straight alpha)
	FillRule FillRule
}

// ElementType distinguishes draw, clip-begin, and clip-end scene elements.
type ElementType uint8

const (
	// ElementDraw is a solid color fill.
	ElementDraw ElementType = iota

	// ElementBeginClip pushes a clip layer with a clip path.
	ElementBeginClip

	// ElementEndClip pops the innermost clip layer.
	ElementEndClip
)

// SceneElement describes a single element in a scene with clip support.
// It replaces PathDef for scenes that use clip layers.
//
// For ElementDraw: Lines + Color + FillRule are used.
// For ElementBeginClip: Lines + BlendMode + Alpha are used.
// For ElementEndClip: no fields are needed (the match is done by clip_leaf).
type SceneElement struct {
	Type      ElementType
	Lines     []LineSoup // Path geometry (for Draw and BeginClip)
	Color     [4]uint8   // RGBA straight alpha (for Draw)
	FillRule  FillRule   // Fill rule (for Draw)
	BlendMode uint32     // Blend mode (for BeginClip; 0x8003 = simple clip)
	Alpha     float32    // Group alpha (for BeginClip; [0.0, 1.0])
}

// ClipInp holds input data for clip matching (output of draw_leaf, input to clip_leaf).
type ClipInp struct {
	// Ix is the draw object index.
	Ix uint32
	// PathIx encodes the path index for BeginClip (positive) or
	// the bitwise complement of the draw object index for EndClip (negative).
	PathIx int32
}
