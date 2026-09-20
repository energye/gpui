package scope

import (
	"github.com/energye/gpui/ui/theme"
)

// ResolvePriority names the three-level token merge order. Larger
// values win: an explicit call-site value beats the Ctx component
// theme, which beats the global seed default.
type ResolvePriority int

const (
	// PrioritySeed is the global seed default (lowest).
	PrioritySeed ResolvePriority = iota
	// PriorityTheme is the Ctx component theme.
	PriorityTheme
	// PriorityProps is the explicit call-site value (highest).
	PriorityProps
)

// Merged resolves one token through the three levels. Explicit wins:
// a non-empty props value beats the theme value, which beats the
// seed default. Empty means "not set at this level".
func Merged(props, ctheme, seed string) (value string, from ResolvePriority) {
	switch {
	case props != "":
		return props, PriorityProps
	case ctheme != "":
		return ctheme, PriorityTheme
	default:
		return seed, PrioritySeed
	}
}

// MergedColor resolves one color token through the three levels.
// Set flags mark which levels actually carry a value; an unset
// level never wins over a set lower level.
func MergedColor(props theme.Color, propsSet bool, ctheme theme.Color, cthemeSet bool, seed theme.Color) theme.Color {
	switch {
	case propsSet:
		return props
	case cthemeSet:
		return ctheme
	default:
		return seed
	}
}

// ResolveColor resolves one color through the standard chain:
// explicit call-site color, else the component theme entry, else
// the seed default passed in. It is the value-level spelling of
// MergedColor for callers that already picked their seed.
func ResolveColor(props *theme.Color, ctheme *theme.Color, seed theme.Color) theme.Color {
	if props != nil {
		return *props
	}
	if ctheme != nil {
		return *ctheme
	}
	return seed
}

// ResolveFloat resolves one numeric token through the standard
// chain. Nil means "not set at this level".
func ResolveFloat(props *float64, ctheme *float64, seed float64) float64 {
	if props != nil {
		return *props
	}
	if ctheme != nil {
		return *ctheme
	}
	return seed
}

// DisabledOr reports the effective disabled state. The subtree flag
// and the per-widget flag combine with OR: either one disables the
// widget, and a per-widget false never clears a subtree disable.
func DisabledOr(subtree, widget bool) bool {
	return subtree || widget
}

// FocusRing is the library-wide keyboard focus outline: 3px wide in
// the primary border color, offset by 1, shown for keyboard focus
// only and never while disabled.
type FocusRing struct {
	// Width is the outline width in logical pixels.
	Width float64
	// Color is the outline color (seed primary border).
	Color theme.Color
	// Offset is the outline offset in logical pixels.
	Offset float64
}

// ResolveFocusRing derives the focus ring from the seed tokens:
// width 3, primary border color, offset 1.
func ResolveFocusRing(seed theme.Tokens) FocusRing {
	return FocusRing{
		Width:  seed.LineWidthFocus,
		Color:  seed.ColorPrimaryBorder,
		Offset: 1,
	}
}

// ShowFocusRing reports whether the ring paints for the given state:
// keyboard-focused and not disabled. Pointer clicks never set the
// focused bit, so this also encodes the focus-visible rule.
func ShowFocusRing(s WidgetState) bool {
	return s.Has(StateFocused) && !s.Has(StateDisabled)
}
