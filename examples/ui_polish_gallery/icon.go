//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerIcon() {
	// Icon — docs/antd/icon.md §6.8 P0
	// https://ant.design/components/icon
	trackIcon := func(ic *kit.Icon) *kit.Icon {
		return ic
	}
	trackSpinIcon := func(ic *kit.Icon) *kit.Icon {
		ic.SetSpin(true)
		*c.tickers = append(*c.tickers, ic)
		return ic
	}
	icCheck := trackIcon(kit.NewIcon("check"))
	icInfo := trackIcon(kit.NewIcon("info"))
	icSearch := trackIcon(kit.NewIcon("search"))
	icPlus := trackIcon(kit.NewIcon("plus"))
	icLoading := trackSpinIcon(kit.NewIcon("loading"))
	icRot := trackIcon(kit.NewIcon("info"))
	icRot.SetRotate(180)

	secIconBasic := demoSection(c.face, c.theme, "Basic",
		"Named registry icons; spin + rotate (antd basic.tsx).",
		spaceWrap(12,
			icCheck.Node(), icInfo.Node(), icSearch.Node(), icPlus.Node(),
			icLoading.Node(), icRot.Node()))

	icStar := trackIcon(kit.NewIcon("star"))
	icHeart := trackIcon(kit.NewIcon("heart"))
	icHeart.SetTwoToneColor(render.RGBA{R: 0.92, G: 0.18, B: 0.59, A: 1})
	icCheckTT := trackIcon(kit.NewIcon("check"))
	icCheckTT.SetTwoToneColors(
		render.RGBA{R: 0.32, G: 0.77, B: 0.1, A: 1},
		render.RGBA{R: 0.32, G: 0.77, B: 0.1, A: 0.25})
	secIconTwoTone := demoSection(c.face, c.theme, "Two-tone",
		"twoToneColor primary / primary+secondary (antd two-tone.tsx).",
		spaceWrap(12, icStar.Node(), icHeart.Node(), icCheckTT.Node()))

	icCustom := trackIcon(kit.NewIcon(""))
	icCustom.SetSize(28)
	icCustom.SetColor(render.RGBA{R: 1, G: 0.41, B: 0.71, A: 1}) // hotpink
	icCustom.SetPainter(func(pc *core.PaintContext, size float64, primary, secondary render.RGBA) {
		if pc == nil {
			return
		}
		pc.FillLocalCircle(size*0.35, size*0.38, size*0.18, primary)
		pc.FillLocalCircle(size*0.65, size*0.38, size*0.18, primary)
		pc.StrokeLocalPolyline([]float64{
			size * 0.18, size * 0.42,
			size * 0.5, size * 0.82,
			size * 0.82, size * 0.42,
		}, 2, primary)
	})
	icCustomBig := trackIcon(kit.NewIcon("info"))
	icCustomBig.SetSize(32)
	secIconCustom := demoSection(c.face, c.theme, "Custom",
		"SetPainter (antd component) + large size.",
		spaceWrap(12, icCustom.Node(), icCustomBig.Node()))

	// Offline iconfont (antd createFromIconfontCN — no CDN).
	kit.RegisterIconSource("gallery_iconfont", map[string]primitive.IconDef{
		"icon-tuichu":   {Kind: primitive.IconClose},
		"icon-facebook": {Kind: primitive.IconInfo},
		"icon-twitter":  {Kind: primitive.IconStar},
	})
	kit.RegisterIconSource("gallery_iconfont_b", map[string]primitive.IconDef{
		"icon-shoppingcart": {Kind: primitive.IconPlus},
		"icon-python":       {Kind: primitive.IconSearch},
	})
	kit.RegisterIconSource("gallery_iconfont_c", map[string]primitive.IconDef{
		"icon-shoppingcart": {Kind: primitive.IconCheck}, // later source overrides
		"icon-javascript":   {Kind: primitive.IconInfo},
	})
	fontA := kit.CreateFromIconfont(kit.IconfontOptions{Sources: []string{"gallery_iconfont"}})
	ff1 := fontA.NewIcon("icon-tuichu")
	ff2 := fontA.NewIcon("icon-facebook")
	ff2.SetColor(render.RGBA{R: 0.09, G: 0.47, B: 0.95, A: 1})
	ff3 := fontA.NewIcon("icon-twitter")
	secIconFont := demoSection(c.face, c.theme, "Iconfont (offline)",
		"CreateFromIconfont + RegisterSource — maps antd iconfont.cn without network.",
		spaceWrap(12, ff1.Node(), ff2.Node(), ff3.Node()))

	fontMulti := kit.CreateFromIconfont(kit.IconfontOptions{
		Sources: []string{"gallery_iconfont_b", "gallery_iconfont_c"},
	})
	fm1 := fontMulti.NewIcon("icon-javascript")
	fm2 := fontMulti.NewIcon("icon-shoppingcart") // from c → check
	fm3 := fontMulti.NewIcon("icon-python")
	secIconMulti := demoSection(c.face, c.theme, "Multi-source",
		"Multiple sources; later overrides same type (antd scriptUrl[]).",
		spaceWrap(12, fm1.Node(), fm2.Node(), fm3.Node()))

	icSz16 := trackIcon(kit.NewIcon("search"))
	icSz24 := trackIcon(kit.NewIcon("search"))
	icSz24.SetSize(24)
	icSz32 := trackIcon(kit.NewIcon("search"))
	icSz32.SetSize(32)
	icCol := trackIcon(kit.NewIcon("check"))
	icCol.SetColor(render.RGBA{R: 0.09, G: 0.42, B: 0.93, A: 1})
	icDis := trackIcon(kit.NewIcon("close"))
	icDis.SetDisabled(true)
	secIconStyle := demoSection(c.face, c.theme, "Size & color",
		"Default 16; SetSize; SetColor; disabled tint.",
		spaceWrap(12, icSz16.Node(), icSz24.Node(), icSz32.Node(), icCol.Node(), icDis.Node()))

	c.items = append(c.items, ctlTab("icon", "Icon"))
	c.contents["icon"] = demoPage(c.face, "Icon",
		"Semantic vector icons. P0: name, size, color, rotate, spin, twoTone, painter, offline iconfont multi-source, decorative a11y.",
		secIconBasic, secIconTwoTone, secIconCustom, secIconFont, secIconMulti, secIconStyle)
}
