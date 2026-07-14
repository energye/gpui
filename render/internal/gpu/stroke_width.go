package gpu

import "github.com/energye/gpui/render"

func effectiveStrokeWidth(paint *render.Paint) float64 {
	width := paint.EffectiveLineWidth()
	transformScale := paint.TransformScale
	if transformScale <= 0 {
		transformScale = 1.0
	}
	width *= transformScale
	if width < 1.0 {
		return 1.0
	}
	return width
}
