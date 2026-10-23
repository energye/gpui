//----------------------------------------
//
// Copyright © yanghy.All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"github.com/energye/gpui/render"
)

// These helpers submit a node's own field-derived content (a background fill,
// an image) to whatever display-list canvas the caller is recording onto — the
// live context on a FullPaint frame, a recorder on a retained frame. Both get
// the same calls in the same order, so a retained recording and a live paint of
// one node cannot diverge by construction.
//
// Degenerate geometry is skipped rather than issued: a zero-sized fill has no
// pixel for the rasterizer to touch, and recording the command anyway would
// charge it to the frame's op budget.
//
// Colour channels are normalized to [0,1] here, as the retired UI-side
// recorder did. Nothing else does: Context.SetRGBA stores the values it is
// given, so an out-of-range field would reach the rasterizer unclamped and the
// two paths would disagree.

// clamp01 normalizes one colour channel.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// fillRectOn fills an axis-aligned rectangle with a solid colour.
func fillRectOn(dc render.Canvas, x, y, w, h, r, g, b, a float64) {
	if dc == nil || w <= 0 || h <= 0 {
		return
	}
	dc.SetRGBA(clamp01(r), clamp01(g), clamp01(b), clamp01(a))
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

// drawImageOn submits an image, scaled into (dw,dh) when both are positive and
// at native size otherwise. A missing or disposed buffer draws nothing.
func drawImageOn(dc render.Canvas, img *render.ImageBuf, x, y, dw, dh float64) {
	if dc == nil || img == nil || img.Disposed() {
		return
	}
	if dw > 0 && dh > 0 {
		dc.DrawImageEx(img, render.DrawImageOptions{
			X: x, Y: y, DstWidth: dw, DstHeight: dh,
		})
		return
	}
	dc.DrawImage(img, x, y)
}
