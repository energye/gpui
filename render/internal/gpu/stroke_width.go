//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import "github.com/energye/gpui/render"

func effectiveStrokeWidth(paint *render.Paint) float64 {
	width := paint.EffectiveLineWidth()
	transformScale := paint.TransformScale
	if transformScale <= 0 {
		transformScale = 1.0
	}
	width *= transformScale
	if width <= 0 {
		// Hairline (Skia/Flutter: stroke width 0) is one device pixel.
		// Positive sub-pixel widths stay sub-pixel so AA coverage matches.
		return 1.0
	}
	return width
}
