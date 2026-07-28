package rendering

import "github.com/energye/gpui/render"

// Filter paint façades on PaintContext (Flutter ColorFilter / ImageFilter apply subset).
// These operate on the current surface contents (full-canvas filter), matching
// render.Apply* — not yet a retained saveLayer-isolated subtree product.
//
// Callers that need CPU/GPU filter backends must blank-import
// github.com/energye/gpui/render/filters (same requirement as raw DC.Apply*).

// ApplyGrayscale converts the current surface to grayscale via the shipped
// render color-matrix path (FF-COLOR-MATRIX / FP-COLOR-FILTER).
func ApplyGrayscale(pc *PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.ApplyGrayscale()
}

// ApplyColorMatrix applies a 4×5 row-major color matrix to the current surface
// (FF-COLOR-MATRIX). Matrix layout matches render.ApplyColorMatrix.
func ApplyColorMatrix(pc *PaintContext, matrix [20]float32) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.ApplyColorMatrix(matrix)
}

// ApplyBlur applies a uniform Gaussian blur to the current surface
// (FF-BLUR / FP-IMAGE-FILTER). radius <= 0 is a no-op.
func ApplyBlur(pc *PaintContext, radius float64) {
	if pc == nil || pc.DC == nil || radius <= 0 {
		return
	}
	pc.DC.ApplyBlur(radius)
}

// ApplyDropShadow composites a drop shadow under the current surface contents
// (FC-DRAW-SHADOW / FF filter path). offset is in logical px (Y-down);
// blurRadius is the shadow soft radius; color is 0..1 RGBA.
//
// Flushes pending GPU draws first so the filter sees painted content on the
// CPU/GPU surface (same discipline as PushBackdrop). Full-surface filter —
// not a path-bound Material elevation API.
func ApplyDropShadow(pc *PaintContext, offsetX, offsetY, blurRadius, r, g, b, a float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	if a <= 0 {
		a = 1
	}
	_ = pc.DC.FlushGPU()
	pc.DC.ApplyDropShadow(offsetX, offsetY, blurRadius, render.RGBA{R: r, G: g, B: b, A: a})
}

// PushBackdrop begins a backdrop filter layer: snapshots the current canvas,
// optionally blurs it, then subsequent draws composite over that snapshot
// until PopBackdrop (FF-BACKDROP / Flutter BackdropFilter subset).
//
// opacity is the layer composite opacity (≤0 → 1). blurRadius ≤0 skips blur.
// Returns false if pc/DC is nil. Pair with PopBackdrop.
//
// Honest limits: full-surface snapshot (not a clip-local backdrop product);
// requires render/filters import when blur > 0.
func PushBackdrop(pc *PaintContext, blurRadius, opacity float64) bool {
	if pc == nil || pc.DC == nil {
		return false
	}
	if opacity <= 0 {
		opacity = 1
	}
	if opacity > 1 {
		opacity = 1
	}
	pc.DC.PushBackdropLayer(render.BlendNormal, opacity)
	if blurRadius > 0 {
		pc.DC.ApplyBlur(blurRadius)
	}
	return true
}

// PopBackdrop ends the most recent PushBackdrop (PopLayer).
func PopBackdrop(pc *PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.PopLayer()
}
