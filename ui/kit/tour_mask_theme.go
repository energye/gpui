package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// TourMaskResolved carries tour chrome colors resolved from seed.
type TourMaskResolved struct {
	PanelBg      theme.Color
	PanelText    theme.Color
	PrimaryBg    theme.Color
	PrimaryText  theme.Color
	Mask         theme.Color
	HoleBorder   theme.Color
	IndicatorOn  theme.Color
	IndicatorOff theme.Color
	Focus        scope.FocusRing
}

// ResolveTourMask resolves panel/mask/indicator colors (props > ctx theme > seed).
func ResolveTourMask(seed theme.Tokens) TourMaskResolved {
	panel := seed.ColorBgElevated
	if panel.A <= 0 {
		panel = seed.ColorBgContainer
	}
	return TourMaskResolved{
		PanelBg:      panel,
		PanelText:    seed.ColorText,
		PrimaryBg:    seed.ColorPrimary,
		PrimaryText:  theme.Hex("#ffffff"),
		Mask:         seed.ColorBgMask,
		HoleBorder:   seed.ColorPrimary,
		IndicatorOn:  seed.ColorPrimary,
		IndicatorOff: seed.ColorFill,
		Focus:        scope.ResolveFocusRing(seed),
	}
}

// TourMaskPanelBg resolves the visible panel fill with shallow override.
func TourMaskPanelBg(seed theme.Tokens, in *TourMaskInstance) theme.Color {
	base := ResolveTourMask(seed)
	typ := TourMaskTypeDefault
	var overrideSet bool
	var r, g, b float64
	if in != nil {
		typ = in.EffectiveType()
		if step, ok := in.CurrentStep(); ok && step.StyleSectionSet {
			overrideSet = true
			r, g, b = step.StyleSectionR, step.StyleSectionG, step.StyleSectionB
		}
	}
	if overrideSet {
		return theme.RGBA(r, g, b, 1)
	}
	if typ == TourMaskTypePrimary {
		return base.PrimaryBg
	}
	return base.PanelBg
}

// TourMaskPanelText resolves the panel text color for the effective type.
func TourMaskPanelText(seed theme.Tokens, in *TourMaskInstance) theme.Color {
	base := ResolveTourMask(seed)
	if in != nil && in.EffectiveType() == TourMaskTypePrimary {
		return base.PrimaryText
	}
	return base.PanelText
}

// TourMaskMaskColor resolves the mask fill with shallow color override.
func TourMaskMaskColor(seed theme.Tokens, props TourMaskProps) theme.Color {
	base := ResolveTourMask(seed).Mask
	if props.MaskColorSet {
		return theme.RGBA(props.MaskColorR, props.MaskColorG, props.MaskColorB, props.MaskColorA)
	}
	return base
}
