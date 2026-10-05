//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

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
// colorTextDisabled. Two-tone defaults follow the official IconBase:
// primary '#333', secondary '#E6E6E6'; setTwoToneColor(primary) switches
// to primary + generate(primary)[0] (derived in prim painter).
func ResolveIcon(seed theme.Tokens, p IconProps) IconResolved {
	size := ResolveIconSize(p)
	twoToneImplied := p.TwoToneSet || p.Variant == "twotone"
	main := parseIconColor(p.Color)
	hasExplicitColor := p.Color != "" && main.A > 0
	if !hasExplicitColor {
		main = theme.Color{}
	}
	if p.TwoToneSet && p.TwoTonePrimary != "" {
		// Explicit twoToneColor primary wins over style color.
		if m := parseIconColor(p.TwoTonePrimary); m.A > 0 {
			main = m
			hasExplicitColor = true
		}
	}
	if main.A <= 0 {
		if twoToneImplied {
			gPrimary, _, _ := getTwoToneGlobals()
			if g := parseIconColor(gPrimary); g.A > 0 {
				main = g
			} else {
				main = parseIconColor("#333")
			}
		} else {
			main = seed.ColorText
		}
	}
	if p.Disabled || main.A == 0 {
		if p.Disabled {
			main = seed.ColorTextDisabled
		} else if main.A == 0 {
			main = seed.ColorText
		}
	}
	twoTone := p.TwoToneSet || p.Variant == "twotone"
	second := theme.Color{}
	hasSecond := false
	if p.HasSecondary {
		second = parseIconColor(p.TwoToneSecondary)
		hasSecond = second.A > 0
	}
	if twoTone && !hasSecond && !p.TwoToneSet {
		// No explicit pair: use global secondary when set, else official
		// default '#E6E6E6' (never derive from near-black ColorText).
		_, gSecond, gSet := getTwoToneGlobals()
		if gSecond != "" {
			if s := parseIconColor(gSecond); s.A > 0 {
				second, hasSecond = s, true
			}
		}
		if !hasSecond && !gSet {
			second, hasSecond = parseIconColor("#E6E6E6"), true
		}
		// When global was explicitly set to a single primary, leave
		// HasSecond false so prim derives generate(primary)[0].
	}
	return IconResolved{Main: main, Secondary: second, HasSecond: hasSecond, TwoTone: twoTone, Size: size}
}

// ResolveIconCtx resolves with Ctx theme seed.
func ResolveIconCtx(ctx scope.Ctx, p IconProps) IconResolved {
	return ResolveIcon(ctx.Normalize().Theme, p)
}
