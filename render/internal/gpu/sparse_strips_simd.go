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
	"github.com/energye/gpui/render/internal/wide"
	"github.com/energye/gpui/render/scene"
)

// SIMD-optimized operations for sparse strips rasterization.
// Uses wide package types for auto-vectorization by Go compiler.

// TileWinding16 holds winding values for a 4×4 tile in SIMD-friendly layout.
// All 16 pixels stored contiguously for vectorized operations.
type TileWinding16 [16]float32

// TileCoverage16 holds coverage values for a 4×4 tile.
type TileCoverage16 [16]uint8

// ProcessSegmentSIMD processes a line segment's contribution to a tile.
//
// The flat 16-value layout is only a storage detail: the winding accumulation
// itself is delegated to accumulateSegmentWinding so this entry point shares
// the exact result of the scalar rasterizer.
func ProcessSegmentSIMD(
	line LineSegment,
	tileX, tileY uint16,
	tileWinding *TileWinding16,
) {
	// Unpack to the [row][col] accumulator layout
	var winding [TileSize][TileSize]float32
	for y := 0; y < TileSize; y++ {
		for x := 0; x < TileSize; x++ {
			winding[y][x] = tileWinding[y*TileSize+x]
		}
	}

	accumulateSegmentWinding(line, tileX, tileY, &winding)

	// Pack back
	for y := 0; y < TileSize; y++ {
		for x := 0; x < TileSize; x++ {
			tileWinding[y*TileSize+x] = winding[y][x]
		}
	}
}

// FinalizeTileSIMD converts winding values to coverage using SIMD.
// Supports both NonZero and EvenOdd fill rules.
func FinalizeTileSIMD(
	tileWinding *TileWinding16,
	coverage *TileCoverage16,
	fillRule scene.FillStyle,
) {
	switch fillRule {
	case scene.FillNonZero:
		finalizeTileNonZeroSIMD(tileWinding, coverage)
	case scene.FillEvenOdd:
		finalizeTileEvenOddSIMD(tileWinding, coverage)
	default:
		finalizeTileNonZeroSIMD(tileWinding, coverage)
	}
}

// finalizeTileNonZeroSIMD converts winding to coverage using NonZero rule.
func finalizeTileNonZeroSIMD(tileWinding *TileWinding16, coverage *TileCoverage16) {
	// Process 8 values at a time using F32x8
	for base := 0; base < 16; base += 8 {
		// Load winding values
		var winding wide.F32x8
		for i := 0; i < 8; i++ {
			winding[i] = tileWinding[base+i]
		}

		// Absolute value
		var absWinding wide.F32x8
		for i := 0; i < 8; i++ {
			if winding[i] < 0 {
				absWinding[i] = -winding[i]
			} else {
				absWinding[i] = winding[i]
			}
		}

		// Clamp to [0, 1] and convert to uint8
		clamped := absWinding.Clamp(0, 1)
		for i := 0; i < 8; i++ {
			coverage[base+i] = uint8(clamped[i]*255.0 + 0.5)
		}
	}
}

// finalizeTileEvenOddSIMD converts winding to coverage using EvenOdd rule.
func finalizeTileEvenOddSIMD(tileWinding *TileWinding16, coverage *TileCoverage16) {
	for base := 0; base < 16; base += 8 {
		var winding wide.F32x8
		for i := 0; i < 8; i++ {
			winding[i] = tileWinding[base+i]
		}

		// EvenOdd: |winding - 2*round(winding/2)|
		var cov wide.F32x8
		for i := 0; i < 8; i++ {
			absW := absf32(winding[i])
			im1 := float32(int32(absW*0.5 + 0.5))
			c := absf32(absW - 2.0*im1)
			if c > 1.0 {
				c = 1.0
			}
			cov[i] = c
		}

		for i := 0; i < 8; i++ {
			coverage[base+i] = uint8(cov[i]*255.0 + 0.5)
		}
	}
}

// BlendTileSIMD performs alpha blending for a 4×4 tile using SIMD.
// Uses U16x16 for processing all 16 pixels at once.
func BlendTileSIMD(
	buffer []uint8,
	bufferStride int,
	baseX, baseY int,
	coverage *TileCoverage16,
	color [4]uint8,
) {
	// Load color as U16x16 (splatted)
	colorR := wide.SplatU16(uint16(color[0]))
	colorG := wide.SplatU16(uint16(color[1]))
	colorB := wide.SplatU16(uint16(color[2]))
	colorA := wide.SplatU16(uint16(color[3]))

	// Load coverage values
	var coverageVec wide.U16x16
	for i := 0; i < 16; i++ {
		coverageVec[i] = uint16(coverage[i])
	}

	// Calculate source alpha: (coverage * colorA) / 255
	srcA := coverageVec.MulDiv255(colorA)
	invA := srcA.Inv()

	// Load destination pixels
	var dstR, dstG, dstB, dstA wide.U16x16
	for py := 0; py < TileSize; py++ {
		pixelY := baseY + py
		if pixelY < 0 {
			continue
		}
		rowOffset := pixelY * bufferStride
		for px := 0; px < TileSize; px++ {
			pixelX := baseX + px
			if pixelX < 0 {
				continue
			}
			idx := rowOffset + pixelX*4
			if idx+3 >= len(buffer) {
				continue
			}
			i := py*TileSize + px
			dstR[i] = uint16(buffer[idx+0])
			dstG[i] = uint16(buffer[idx+1])
			dstB[i] = uint16(buffer[idx+2])
			dstA[i] = uint16(buffer[idx+3])
		}
	}

	// Blend: out = (src * srcA + dst * invA) / 255
	outR := colorR.MulDiv255(srcA).Add(dstR.MulDiv255(invA))
	outG := colorG.MulDiv255(srcA).Add(dstG.MulDiv255(invA))
	outB := colorB.MulDiv255(srcA).Add(dstB.MulDiv255(invA))
	outA := srcA.Add(dstA.MulDiv255(invA))

	// Store result
	for py := 0; py < TileSize; py++ {
		pixelY := baseY + py
		if pixelY < 0 {
			continue
		}
		rowOffset := pixelY * bufferStride
		for px := 0; px < TileSize; px++ {
			pixelX := baseX + px
			if pixelX < 0 {
				continue
			}
			idx := rowOffset + pixelX*4
			if idx+3 >= len(buffer) {
				continue
			}
			i := py*TileSize + px
			if coverageVec[i] == 0 {
				continue // Skip fully transparent
			}
			//nolint:gosec // Values are guaranteed to be in [0, 255] range
			buffer[idx+0] = uint8(outR[i])
			//nolint:gosec // Values are guaranteed to be in [0, 255] range
			buffer[idx+1] = uint8(outG[i])
			//nolint:gosec // Values are guaranteed to be in [0, 255] range
			buffer[idx+2] = uint8(outB[i])
			//nolint:gosec // Values are guaranteed to be in [0, 255] range
			buffer[idx+3] = uint8(outA[i])
		}
	}
}

// RenderToBufferSIMD renders the tile grid using SIMD-optimized blending.
func (fr *FineRasterizer) RenderToBufferSIMD(
	buffer []uint8,
	width, height int,
	stride int,
	color [4]uint8,
) {
	fr.grid.ForEach(func(tile *Tile) {
		baseX := int(tile.PixelX())
		baseY := int(tile.PixelY())

		// Skip tiles fully outside viewport
		if baseX >= width || baseY >= height {
			return
		}
		if baseX+TileSize <= 0 || baseY+TileSize <= 0 {
			return
		}

		// Convert tile coverage to TileCoverage16
		var coverage TileCoverage16
		for py := 0; py < TileSize; py++ {
			for px := 0; px < TileSize; px++ {
				coverage[py*TileSize+px] = tile.GetCoverage(px, py)
			}
		}

		// Use SIMD blending
		BlendTileSIMD(buffer, stride, baseX, baseY, &coverage, color)
	})
}

// InitTileWindingSIMD initializes tile winding from backdrop using SIMD.
func InitTileWindingSIMD(tileWinding *TileWinding16, backdrop float32) {
	// Splat backdrop to all 16 positions
	for i := 0; i < 16; i++ {
		tileWinding[i] = backdrop
	}
}

// ProcessMultipleSegmentsSIMD processes multiple segments for a single tile.
// This batches segment processing to maximize SIMD utilization.
func ProcessMultipleSegmentsSIMD(
	lines []LineSegment,
	lineIndices []uint32,
	tileX, tileY uint16,
	backdrop float32,
	fillRule scene.FillStyle,
) TileCoverage16 {
	var tileWinding TileWinding16
	InitTileWindingSIMD(&tileWinding, backdrop)

	// Process all segments
	for _, idx := range lineIndices {
		if int(idx) < len(lines) {
			ProcessSegmentSIMD(lines[idx], tileX, tileY, &tileWinding)
		}
	}

	// Finalize to coverage
	var coverage TileCoverage16
	FinalizeTileSIMD(&tileWinding, &coverage, fillRule)
	return coverage
}
