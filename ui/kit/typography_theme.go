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
	"github.com/energye/gpui/ui/kit/internal/prim"
	"github.com/energye/gpui/ui/theme"
)

// ResolvedTypography is the theme-resolved look for one build: everything
// the Render segment needs, merged through the three tiers (this call's
// props > Ctx component theme > global seed). No colors or sizes live in
// the Render segment.
type ResolvedTypography struct {
	// FontSize resolves the kind ladder: Title picks FontSizeHeading{Level},
	// everything else uses the seed body size (14).
	FontSize float64
	// Color resolves kind + type: Link uses the link color, Type picks the
	// semantic color, disabled wins over both (or-rule).
	Color theme.Color
	// Decoration carries underline / line-through from the modifier props
	// (prim facade enum).
	Decoration prim.TextDecorationAlias
	// Bold marks the strong modifier (weight handled by the face path).
	Bold bool
	// MarkBg is the mark highlight background (gold[2] component constant).
	MarkBg theme.Color
	// ChipRadius is the code/keyboard corner radius (component constant 3).
	ChipRadius float64
	// ChipBg / ChipBorder dress code and keyboard chrome.
	ChipBg     theme.Color
	ChipBorder theme.Color
	// OperatorGap is the operator spacing (marginXXS = 4).
	OperatorGap float64
	// ExpandCopy / CollapseCopy are the locale-driven operator labels
	// (antd defaults 展开/收起).
	ExpandCopy   string
	CollapseCopy string
}

// ResolveTypography merges the three tiers. There is no per-component
// theme override table yet (F0 seed only), so this is props-over-seed;
// the merge point stays here so component tokens slot in without touching
// call sites.
func ResolveTypography(seed theme.Tokens, p TypographyProps, disabled bool) ResolvedTypography {
	r := ResolvedTypography{
		FontSize:     seed.FontSize,
		MarkBg:       theme.Hex("#ffe58f"), // antd gold-2, component constant
		ChipRadius:   3,                    // antd code/kbd radius, style constant
		ChipBg:       seed.ColorFillSecondary,
		ChipBorder:   seed.ColorBorderSecondary,
		OperatorGap:  4, // marginXXS
		ExpandCopy:   "展开",
		CollapseCopy: "收起",
	}
	// Size ladder: Title picks the heading token for its level (1..5,
	// clamped), everything else keeps the body size.
	if p.Kind == TypographyTitle {
		switch clampLevel(p.Level) {
		case 1:
			r.FontSize = seed.FontSizeHeading1
		case 2:
			r.FontSize = seed.FontSizeHeading2
		case 3:
			r.FontSize = seed.FontSizeHeading3
		case 4:
			r.FontSize = seed.FontSizeHeading4
		default:
			r.FontSize = seed.FontSizeHeading5
		}
	}
	// Color: disabled wins (or-rule), then kind, then type semantics.
	switch {
	case disabled:
		r.Color = seed.ColorTextDisabled
	case p.Kind == TypographyLink:
		r.Color = seed.ColorLink
	case p.Type == TypographyTypeSecondary:
		r.Color = seed.ColorTextSecondary
	case p.Type == TypographyTypeSuccess:
		r.Color = seed.ColorSuccess
	case p.Type == TypographyTypeWarning:
		r.Color = seed.ColorWarning
	case p.Type == TypographyTypeDanger:
		r.Color = seed.ColorError
	default:
		if p.Kind == TypographyTitle {
			r.Color = seed.ColorTextHeading
		} else {
			r.Color = seed.ColorText
		}
	}
	if p.Underline {
		r.Decoration |= prim.DecorationUnderline
	}
	if p.Delete {
		r.Decoration |= prim.DecorationLineThrough
	}
	r.Bold = p.Strong
	return r
}

// clampLevel pins Title level into 1..5 (antd maps unknown levels to the
// h1 default).
func clampLevel(l int) int {
	if l < 1 || l > 5 {
		return 1
	}
	return l
}

// TypographyLabelStyle projects the resolved look onto the prim TextStyle
// (the L1 facade vocabulary).
func (r ResolvedTypography) TypographyLabelStyle() prim.TextStyle {
	return prim.TextStyle{
		FontSize: r.FontSize,
		Color:    r.Color,
		HasColor: true,
	}
}
