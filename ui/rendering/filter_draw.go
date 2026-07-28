package rendering

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
