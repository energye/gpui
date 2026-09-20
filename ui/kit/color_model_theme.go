package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// ColorModelResolved carries picker chrome colors resolved from seed.
type ColorModelResolved struct {
	PanelBg   theme.Color
	Text      theme.Color
	TriggerBg theme.Color
	Border    theme.Color
	Selected  theme.Color
	Disabled  theme.Color
	Focus     scope.FocusRing
}

// ResolveColorModel resolves panel/trigger chrome (props > ctx theme > seed).
func ResolveColorModel(seed theme.Tokens) ColorModelResolved {
	bg := seed.ColorBgElevated
	if bg.A <= 0 {
		bg = seed.ColorBgContainer
	}
	return ColorModelResolved{
		PanelBg:   bg,
		Text:      seed.ColorText,
		TriggerBg: seed.ColorBgContainer,
		Border:    seed.ColorBorder,
		Selected:  seed.ColorPrimary,
		Disabled:  seed.ColorTextDisabled,
		Focus:     scope.ResolveFocusRing(seed),
	}
}

// Trigger swatch sizes per tier (color-picker.md §6.2.1):
// small 16, middle 24, large 32.
const (
	ColorModelSwatchSmall  = 16.0
	ColorModelSwatchMedium = 24.0
	ColorModelSwatchLarge  = 32.0
)

// ColorModelSwatchSize returns the trigger swatch edge.
func ColorModelSwatchSize(size ColorModelSize) float64 {
	switch size {
	case ColorModelSizeSmall:
		return ColorModelSwatchSmall
	case ColorModelSizeLarge:
		return ColorModelSwatchLarge
	default:
		return ColorModelSwatchMedium
	}
}

// ColorModelSwatch returns the current color for the trigger swatch.
// Cleared renders transparent (alpha 0).
func ColorModelSwatch(in *ColorModelInstance) (r, g, b, a float64) {
	if in == nil {
		return 0, 0, 0, 0
	}
	v := in.Value()
	if v.Cleared {
		return 0, 0, 0, 0
	}
	c := in.CurrentColor()
	return float64(c.R) / 255, float64(c.G) / 255, float64(c.BL) / 255, c.A
}
