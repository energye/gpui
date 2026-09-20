package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// ModalHostResolved carries host chrome colors resolved from seed.
type ModalHostResolved struct {
	PanelBg theme.Color
	Mask    theme.Color
	Title   theme.Color
	Content theme.Color
	Focus   scope.FocusRing
}

// ResolveModalHost resolves panel/mask/text colors (props > ctx theme > seed).
func ResolveModalHost(seed theme.Tokens) ModalHostResolved {
	panel := seed.ColorBgElevated
	if panel.A <= 0 {
		panel = seed.ColorBgContainer
	}
	return ModalHostResolved{
		PanelBg: panel,
		Mask:    seed.ColorBgMask,
		Title:   seed.ColorTextHeading,
		Content: seed.ColorText,
		Focus:   scope.ResolveFocusRing(seed),
	}
}

// ModalHostOkBg resolves the OK button fill with disabled-first lookup.
func ModalHostOkBg(seed theme.Tokens, st scope.WidgetState) theme.Color {
	r := scope.DisabledFirstResolver[theme.Color]{
		Disabled: seed.ColorBgContainerDisabled,
		Rest: scope.StateResolverFunc[theme.Color](func(s scope.WidgetState) theme.Color {
			if s.Has(scope.StatePressed) {
				return seed.ColorPrimaryActive
			}
			if s.Has(scope.StateHover) {
				return seed.ColorPrimaryHover
			}
			return seed.ColorPrimary
		}),
	}
	return r.Resolve(st)
}
