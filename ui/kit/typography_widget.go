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
	"github.com/energye/gpui/ui/rendering"
)

// BuildTypography is the sole construction entry: build → bind → attach.
// The render root is a single styled label (no operator chrome yet —
// operators land with the copy/edit legs); the WidgetNode welds the
// binding per KIT_FRAME_BINDING_PLAN 件 1.
func BuildTypography(ctx scope.Ctx, p TypographyProps) (*TypographyInstance, *scope.WidgetNode) {
	c := ctx.Normalize()
	disabled := scope.DisabledOr(c.Disabled, p.Disabled)
	res := ResolveTypography(c.Theme, p, disabled)

	label := rendering.NewRenderText(p.Value)
	if st := res.TypographyLabelStyle(); st.FontSize > 0 {
		label.SetFontSize(st.FontSize)
		label.SetColor(st.Color.R, st.Color.G, st.Color.B, st.Color.A)
	}
	label.SetDecoration(res.Decoration)
	// Ellipsis quick path: rows cap + overflow mark (binary cut lands with
	// the measure leg; single-line overflow uses the render-level mark).
	if p.Ellipsis.On && !p.Ellipsis.Middle {
		if rows := p.Ellipsis.Rows; rows > 0 {
			label.SetMaxLines(rows)
		}
		label.SetOverflow(rendering.TextOverflowEllipsis)
	}

	inst := newTypographyInstance(c, p, label)
	node := scope.Attach(inst, TypographySubscribedAspects()...)
	inst.Mount(c, node)
	return inst, node
}

// Syntax sugar for the four antd forms.

// BuildTypographyText builds inline text.
func BuildTypographyText(ctx scope.Ctx, p TypographyProps) (*TypographyInstance, *scope.WidgetNode) {
	p.Kind = TypographyText
	return BuildTypography(ctx, p)
}

// BuildTypographyTitle builds a heading; level defaults to 1.
func BuildTypographyTitle(ctx scope.Ctx, p TypographyProps) (*TypographyInstance, *scope.WidgetNode) {
	p.Kind = TypographyTitle
	if p.Level <= 0 {
		p.Level = 1
	}
	return BuildTypography(ctx, p)
}

// BuildTypographyParagraph builds a body paragraph.
func BuildTypographyParagraph(ctx scope.Ctx, p TypographyProps) (*TypographyInstance, *scope.WidgetNode) {
	p.Kind = TypographyParagraph
	return BuildTypography(ctx, p)
}

// BuildTypographyLink builds hyperlink-styled text.
func BuildTypographyLink(ctx scope.Ctx, p TypographyProps) (*TypographyInstance, *scope.WidgetNode) {
	p.Kind = TypographyLink
	return BuildTypography(ctx, p)
}
