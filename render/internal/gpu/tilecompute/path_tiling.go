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

// Stage 2: Segment clipping to tile boundaries + yEdge computation.
//
// The DDA parameters come from path_count via ddas (see lineDDA) instead of
// being recomputed for every (line, tile) instance.
func pathTilingMain(
	bump *BumpAllocators,
	segCounts []SegmentCount,
	tiles []Tile,
	segments []PathSegment,
	ddas []lineDDA,
) {
	for segIx := uint32(0); segIx < bump.SegCounts; segIx++ {
		segCount := segCounts[segIx]
		dd := &ddas[segCount.LineIx]
		counts := segCount.Counts
		segWithinSlice := counts >> 16
		segWithinLine := counts & 0xffff

		// DDA parameters computed once per line by path_count.
		xy0, xy1 := dd.xy0, dd.xy1
		isDown := dd.isDown
		count := dd.count
		a := dd.a
		b := dd.b
		isPositiveSlope := dd.isPositiveSlope
		sign := dd.sign
		x0 := dd.x0
		y0 := dd.y0

		z := floor32(a*float32(segWithinLine) + b)
		x := int32(x0) + int32(sign*z)
		y := int32(y0 + float32(segWithinLine) - z)

		bboxi := dd.bboxi
		stride := dd.stride
		tileIx := dd.pathTiles + (y-bboxi[1])*stride + x - bboxi[0]
		tile := tiles[tileIx]
		segStart := ^tile.SegmentCountOrIx
		if int32(segStart) < 0 {
			continue
		}

		tileXY := newVec2(float32(x)*float32(TileWidth), float32(y)*float32(TileHeight))
		tileXY1 := tileXY.add(newVec2(float32(TileWidth), float32(TileHeight)))

		if segWithinLine > 0 {
			zPrev := floor32(a*float32(segWithinLine-1) + b)
			if z == zPrev {
				// Top edge is clipped — entered from top
				xt := xy0.x + (xy1.x-xy0.x)*(tileXY.y-xy0.y)/(xy1.y-xy0.y)
				xt = clamp32(xt, tileXY.x+1e-3, tileXY1.x)
				xy0 = newVec2(xt, tileXY.y)
			} else {
				// Side edge is clipped — entered from left (pos slope) or right (neg slope)
				var xClip float32
				if isPositiveSlope {
					xClip = tileXY.x
				} else {
					xClip = tileXY1.x
				}
				yt := xy0.y + (xy1.y-xy0.y)*(xClip-xy0.x)/(xy1.x-xy0.x)
				yt = clamp32(yt, tileXY.y+1e-3, tileXY1.y)
				xy0 = newVec2(xClip, yt)
			}
		}

		if segWithinLine < count-1 {
			zNext := floor32(a*float32(segWithinLine+1) + b)
			if z == zNext {
				// Bottom edge is clipped
				xt := xy0.x + (xy1.x-xy0.x)*(tileXY1.y-xy0.y)/(xy1.y-xy0.y)
				xt = clamp32(xt, tileXY.x+1e-3, tileXY1.x)
				xy1 = newVec2(xt, tileXY1.y)
			} else {
				// Side edge is clipped
				var xClip float32
				if isPositiveSlope {
					xClip = tileXY1.x
				} else {
					xClip = tileXY.x
				}
				yt := xy0.y + (xy1.y-xy0.y)*(xClip-xy0.x)/(xy1.x-xy0.x)
				yt = clamp32(yt, tileXY.y+1e-3, tileXY1.y)
				xy1 = newVec2(xClip, yt)
			}
		}

		yEdge := float32(1e9)
		p0out := xy0.sub(tileXY) // Convert to tile-relative
		p1out := xy1.sub(tileXY)
		const epsilon float32 = 1e-6

		if p0out.x == 0.0 {
			if p1out.x == 0.0 {
				// Both on left edge
				p0out.x = epsilon
				if p0out.y == 0.0 {
					// Entire tile
					p1out.x = epsilon
					p1out.y = float32(TileHeight)
				} else {
					// Make segment disappear
					p1out.x = 2.0 * epsilon
					p1out.y = p0out.y
				}
			} else if p0out.y == 0.0 {
				// p0 at top-left corner
				p0out.x = epsilon
			} else {
				// p0 on left edge (not corner)
				yEdge = p0out.y
			}
		} else if p1out.x == 0.0 {
			if p1out.y == 0.0 {
				// p1 at top-left corner
				p1out.x = epsilon
			} else {
				// p1 on left edge (not corner)
				yEdge = p1out.y
			}
		}

		// Pixel boundary nudging (lines 145-150)
		if p0out.x == floor32(p0out.x) && p0out.x != 0.0 {
			p0out.x -= epsilon
		}
		if p1out.x == floor32(p1out.x) && p1out.x != 0.0 {
			p1out.x -= epsilon
		}

		// Restore original direction (line 151-153)
		if !isDown {
			p0out, p1out = p1out, p0out
		}

		segment := PathSegment{
			Point0: p0out.toArray(),
			Point1: p1out.toArray(),
			YEdge:  yEdge,
		}
		segments[segStart+segWithinSlice] = segment
	}
}
