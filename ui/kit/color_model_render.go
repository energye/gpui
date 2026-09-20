package kit

// Fixed color-picker geometry aligned to color-picker.md §6.2.1.
// Painting stays in prim when F0-2 lands; this file only computes rects.
//
// Panel 234 wide; SV square fills width minus 2*12 padding; hue/alpha
// strips 8 tall; handles 12 (small) / 16.
const (
	ColorModelPanelW       = 234.0
	ColorModelPanelPad     = 12.0
	ColorModelSVH          = 210.0
	ColorModelStripH       = 8.0
	ColorModelStripGap     = 8.0
	ColorModelHandleSmall  = 12.0
	ColorModelHandleNormal = 16.0
)

// ColorModelRect is one panel part rect in logical pixels.
type ColorModelRect struct {
	X, Y, W, H float64
}

// ColorModelPanelLayout carries the three interactive rects.
type ColorModelPanelLayout struct {
	SV       ColorModelRect
	Hue      ColorModelRect
	Alpha    ColorModelRect
	HasAlpha bool
	Height   float64
}

// ComputeColorModelPanel returns SV/hue/alpha rects for a 234 panel.
// disabledAlpha hides the alpha strip and collapses panel height.
func ComputeColorModelPanel(disabledAlpha bool) ColorModelPanelLayout {
	inner := ColorModelPanelW - ColorModelPanelPad*2
	sv := ColorModelRect{X: ColorModelPanelPad, Y: ColorModelPanelPad, W: inner, H: ColorModelSVH}
	y := ColorModelPanelPad + ColorModelSVH + ColorModelStripGap
	hue := ColorModelRect{X: ColorModelPanelPad, Y: y, W: inner, H: ColorModelStripH}
	y += ColorModelStripH + ColorModelStripGap
	var alpha ColorModelRect
	hasAlpha := !disabledAlpha
	if hasAlpha {
		alpha = ColorModelRect{X: ColorModelPanelPad, Y: y, W: inner, H: ColorModelStripH}
		y += ColorModelStripH + ColorModelStripGap
	}
	return ColorModelPanelLayout{
		SV:       sv,
		Hue:      hue,
		Alpha:    alpha,
		HasAlpha: hasAlpha,
		Height:   y + ColorModelPanelPad - ColorModelStripGap,
	}
}

// ColorModelHandleXY returns the hue handle center for hue 0..360.
func ColorModelHandleXY(hue float64, layout ColorModelPanelLayout) (x, y float64) {
	t := hue / 360
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return layout.Hue.X + t*layout.Hue.W, layout.Hue.Y + layout.Hue.H/2
}

// ColorModelAlphaXY returns the alpha handle center for alpha 0..1.
func ColorModelAlphaXY(alpha float64, layout ColorModelPanelLayout) (x, y float64) {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	return layout.Alpha.X + alpha*layout.Alpha.W, layout.Alpha.Y + layout.Alpha.H/2
}

// ColorModelSVXY returns the SV thumb for s/b 0..1.
func ColorModelSVXY(s, b float64, layout ColorModelPanelLayout) (x, y float64) {
	if s < 0 {
		s = 0
	}
	if s > 1 {
		s = 1
	}
	if b < 0 {
		b = 0
	}
	if b > 1 {
		b = 1
	}
	return layout.SV.X + s*layout.SV.W, layout.SV.Y + (1-b)*layout.SV.H
}
