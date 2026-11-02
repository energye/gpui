//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package image provides image buffer management for gogpu/gg.
package image

import (
	"math"

	"github.com/energye/gpui/gpu/hal"
)

// MipmapMode is the single truth for how a minified image samples its mip
// chain. It lives here because both the CPU image path and the GPU sampler
// wiring consume it; render re-exports it and engine/tex aliases render's
// export, so the repo keeps one enum (P3.6 batch 2, D11).
//
// MipmapNone is the zero value on purpose: leaving a field unset keeps
// today's behaviour bit-identical (single-level texture, level 0 only), so no
// caller silently starts paying chain-generation cost.
type MipmapMode int

const (
	// MipmapNone draws from a single-level texture: no chain is built and the
	// sampler never selects above level 0.
	MipmapNone MipmapMode = iota
	// MipmapNearest selects the closest level, no blending between levels.
	MipmapNearest
	// MipmapLinear blends adjacent levels (trilinear with a linear minifier).
	MipmapLinear
)

// String returns the stable log name of m.
func (m MipmapMode) String() string {
	switch m {
	case MipmapNearest:
		return "nearest"
	case MipmapLinear:
		return "linear"
	default:
		return "none"
	}
}

// UsesChain reports whether this mode needs a prefilled mip chain at all.
func (m MipmapMode) UsesChain() bool { return m != MipmapNone }

// MipmapChain holds pre-computed downscaled versions of an image.
//
// A mipmap chain consists of multiple levels, where each level is half the
// size of the previous level (both width and height). Level 0 is the original
// full-resolution image. The chain continues until the smallest dimension
// reaches 1 pixel.
//
// Mipmaps are used for efficient texture filtering at different scales,
// reducing aliasing artifacts when rendering scaled images.
type MipmapChain struct {
	levels []*ImageBuf // Level 0 = original size
}

// GenerateMipmaps creates a mipmap chain from the source image.
//
// Uses a box filter (2x2 average) to downsample each level. The process
// continues until the smallest dimension reaches 1 pixel. The source image
// becomes level 0 and is not copied.
//
// Returns nil if src is nil or empty.
func GenerateMipmaps(src *ImageBuf) *MipmapChain {
	if src == nil || src.IsEmpty() {
		return nil
	}

	// Level count comes from hal, so the CPU chain, the texture creation and the
	// VRAM ledger sum the same levels (P3.6 batch 2).
	numLevels := int(hal.MipLevelCount(uint32(src.Width()), uint32(src.Height())))

	chain := &MipmapChain{
		levels: make([]*ImageBuf, numLevels),
	}

	// Level 0 is the original image (no copy)
	chain.levels[0] = src

	// Generate downsampled levels
	for i := 1; i < numLevels; i++ {
		chain.levels[i] = downsample(chain.levels[i-1])
	}

	return chain
}

// downsample lives in mipmap_downsample.go (row-wise, byte-identical to the
// original per-pixel GetRGBA/SetRGBA version — see TestDownsampleByteIdentical).

// Level returns the mipmap at the specified level.
// Level 0 is the original image. Returns nil if level is out of range.
func (m *MipmapChain) Level(n int) *ImageBuf {
	if m == nil || n < 0 || n >= len(m.levels) {
		return nil
	}
	return m.levels[n]
}

// NumLevels returns the total number of mipmap levels in the chain.
// Returns 0 if the chain is nil.
func (m *MipmapChain) NumLevels() int {
	if m == nil {
		return 0
	}
	return len(m.levels)
}

// LevelForScale returns the appropriate mipmap level for a given scale factor.
//
// The scale parameter represents the ratio of displayed size to original size:
//   - scale = 1.0: original size (level 0)
//   - scale = 0.5: half size (level 1)
//   - scale = 0.25: quarter size (level 2)
//   - etc.
//
// The level is calculated as: level = floor(-log2(scale))
// The result is clamped to [0, NumLevels-1].
//
// Returns 0 if scale is >= 1.0 or if the chain is nil.
func (m *MipmapChain) LevelForScale(scale float64) *ImageBuf {
	if m == nil || len(m.levels) == 0 {
		return nil
	}

	if scale >= 1.0 {
		return m.levels[0]
	}

	// Calculate appropriate level: level = floor(-log2(scale))
	level := int(math.Floor(-math.Log2(scale)))

	// Clamp to valid range
	if level < 0 {
		level = 0
	}
	if level >= len(m.levels) {
		level = len(m.levels) - 1
	}

	return m.levels[level]
}

// Release returns all mipmap buffers to the pool except level 0.
//
// Level 0 is the original image and is not returned to the pool since
// it was provided by the caller. After calling Release, the chain should
// not be used.
func (m *MipmapChain) Release() {
	if m == nil {
		return
	}

	// Return all levels except 0 (which is the original image)
	for i := 1; i < len(m.levels); i++ {
		if m.levels[i] != nil {
			PutToDefault(m.levels[i])
			m.levels[i] = nil
		}
	}
}
