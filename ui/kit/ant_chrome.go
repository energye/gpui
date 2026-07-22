package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Ant-facing chrome helpers shared by list/menu/table rows.

// antItemHoverFill is controlItemBgHover (rgba black 0.06) composited over container.
func antItemHoverFill(th *core.Theme) render.RGBA {
	if th == nil {
		return render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	bg := th.Color(core.TokenColorBgContainer)
	h := th.Color(core.TokenColorBgTextHover)
	if h.A < 0.02 {
		h = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	return compositeOver(h, bg)
}

// antItemSelectedFill is Ant item selected background (colorPrimaryBg #E6F4FF).
func antItemSelectedFill(th *core.Theme) render.RGBA {
	if th == nil {
		return render.Hex("#E6F4FF")
	}
	c := th.Color(core.TokenColorPrimaryBg)
	if c.A < 0.05 {
		return render.Hex("#E6F4FF")
	}
	return c
}

// antItemSelectedText is primary color for selected labels.
func antItemSelectedText(th *core.Theme) render.RGBA {
	if th == nil {
		return render.Hex("#1677FF")
	}
	return th.Color(core.TokenColorPrimary)
}

// antHeaderFill is table/list header background (#FAFAFA).
func antHeaderFill(th *core.Theme) render.RGBA {
	if th == nil {
		return render.Hex("#FAFAFA")
	}
	// Slightly stronger than fillSecondary for solid header.
	return render.Hex("#FAFAFA")
}
