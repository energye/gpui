//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package hal

// Mip level math, shared by the VRAM ledger and the render image cache so the
// two never diverge (P3.6 batch 2, D12: both sides count the same bytes).
//
//口径对齐 Impeller `Size::MipLevels()` and this repo's CPU chain: the count is
// 1 + floor(log2(max(w,h))) — max, not min — and every level halves both
// dimensions with a floor of 1 pixel, so a 70x8 picture ends at 1x1 rather than
// running off the end.

// MipLevelCount returns how many levels a w-by-h texture holds. A non-positive
// side holds none (0), which callers treat as "no chain at all".
func MipLevelCount(w, h uint32) uint32 {
	if w == 0 || h == 0 {
		return 0
	}
	m := w
	if h > m {
		m = h
	}
	n := uint32(1)
	for m > 1 {
		m >>= 1
		n++
	}
	return n
}

// MipLevelSize returns one level's dimensions. Level 0 is the full size; deeper
// levels halve both axes and never go below 1 pixel.
func MipLevelSize(w, h uint32, level uint32) (uint32, uint32) {
	if level == 0 {
		return w, h
	}
	// Shift beyond the width's bit count is undefined behaviour in Go, so clamp.
	shift := level
	if shift > 31 {
		shift = 32
	}
	lw, lh := uint64(w), uint64(h)
	if shift <= 31 {
		lw >>= shift
		lh >>= shift
	} else {
		lw, lh = 0, 0
	}
	if lw == 0 {
		lw = 1
	}
	if lh == 0 {
		lh = 1
	}
	return uint32(lw), uint32(lh)
}

// MipChainTexels counts texels across `levels` levels of a w-by-h texture,
// including level 0. levels <= 1 returns the single-level area.
func MipChainTexels(w, h uint32, levels uint32) uint64 {
	if w == 0 || h == 0 || levels == 0 {
		return 0
	}
	var total uint64
	for l := uint32(0); l < levels; l++ {
		lw, lh := MipLevelSize(w, h, l)
		total += uint64(lw) * uint64(lh)
		if lw == 1 && lh == 1 {
			// Every deeper level is 1x1 as well; add the remaining ones in one
			// step instead of looping up to a huge `levels`.
			if l+1 < levels {
				total += uint64(levels-l-1)
			}
			return total
		}
	}
	return total
}
