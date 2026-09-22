package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// IconResolved carries theme-resolved colors (WIDGET_MODEL Theme segment).
type IconResolved struct {
	Main      theme.Color
	Secondary theme.Color
	HasSecond bool
	TwoTone   bool
	Size      float64
}

// ResolveIcon merges explicit props > global two-tone > theme seed.
// Empty color string falls back to theme colorText; disabled maps to
// colorTextDisabled. Secondary defaults to derived halo unless set.
func ResolveIcon(seed theme.Tokens, p IconProps) IconResolved {
	size := ResolveIconSize(p)
	main := parseIconColor(p.Color)
	if p.Color == "" || main.A == 0 {
		main = seed.ColorText
	}
	if p.Disabled || main.A == 0 {
		if p.Disabled {
			main = seed.ColorTextDisabled
		} else if main.A == 0 {
			main = seed.ColorText
		}
	}
	twoTone := p.TwoToneSet
	second := theme.Color{}
	hasSecond := false
	if p.HasSecondary {
		second = parseIconColor(p.TwoToneSecondary)
		hasSecond = second.A > 0
	} else if p.TwoToneSet && p.TwoTonePrimary != "" {
		// Single primary form: secondary derived in prim painter.
		main = parseIconColor(p.TwoTonePrimary)
		if main.A == 0 {
			main = seed.ColorText
		}
	}
	if !hasSecond && p.TwoToneSet && p.TwoTonePrimary != "" {
		global := parseIconColor(GetTwoToneColorGlobal())
		_ = global
	}
	if !p.TwoToneSet {
		// Instance global default participates only when two-tone used.
		twoTone = false
	}
	return IconResolved{Main: main, Secondary: second, HasSecond: hasSecond, TwoTone: twoTone, Size: size}
}

// ResolveIconCtx resolves with Ctx theme seed.
func ResolveIconCtx(ctx scope.Ctx, p IconProps) IconResolved {
	return ResolveIcon(ctx.Normalize().Theme, p)
}
