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

import ()

// SetDither enables ordered dithering of soft fills after GPU resolve (P.09).
// When enabled, subsequent Image()/FlushGPU results for this context receive a
// Bayer 4x4 ordered dither on the pixmap (reduces gradient banding).
func (c *Context) SetDither(enabled bool) {
	if c == nil {
		return
	}
	c.dither = enabled
}

// Dither reports whether ordered dithering is enabled.
// Dither reports whether ordered dithering is enabled.
func (c *Context) Dither() bool {
	if c == nil {
		return false
	}
	return c.dither
}

func (c *Context) applyDitherIfEnabled() {
	if c == nil || !c.dither || c.pixmap == nil {
		return
	}
	applyBayerDither4(c.pixmap)
	c.pixmap.NotifyPixelsChanged()
}

// Bayer 4x4 thresholds in 0..15, scaled into low-bit noise for 8-bit channels.
var bayer4 = [4][4]int{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

func applyBayerDither4(pm *Pixmap) {
	if pm == nil {
		return
	}
	w, h := pm.Width(), pm.Height()
	data := pm.Data()
	stride := pm.Width() * 4
	if len(data) < h*stride {
		return
	}
	for y := 0; y < h; y++ {
		row := y * stride
		for x := 0; x < w; x++ {
			thr := bayer4[y&3][x&3] // 0..15
			// Bias each channel by ±2 based on threshold vs low bits.
			off := row + x*4
			for c := 0; c < 3; c++ {
				v := int(data[off+c])
				// ordered: if (v & 15) > thr then bump else leave; mild banding break
				if (v & 15) > thr {
					if v < 253 {
						v += 2
					}
				} else if v > 2 {
					v -= 1
				}
				if v < 0 {
					v = 0
				}
				if v > 255 {
					v = 255
				}
				data[off+c] = byte(v)
			}
		}
	}
}

// DrawImageQuad draws an image into a free-form destination quad (T.04 non-affine subset).
// corners are user-space points in order: top-left, top-right, bottom-right, bottom-left.
// GPU path uses QueueImageDraw with arbitrary corner mapping (perspective-like trapezoids).
// CPU path uses the same TL-TR-BL + TR-BR-BL split for pixel parity.
// Compat wrapper: defaults Bilinear + opaque + Normal, degenerate quads no-op.
