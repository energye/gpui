//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerDivider() {
	// Divider — docs/antd/divider.md §6.8 P0
	// https://ant.design/components/divider
	para := func(s string) core.Node {
		tx := kit.NewText(s)
		tx.SetFace(c.face)
		tx.SetSecondary(true)
		return tx.Node()
	}
	divPlain := kit.NewDivider()
	divDashed := kit.NewDivider()
	divDashed.SetDashed(true)
	secDivHorizontal := demoSection(c.face, c.theme, "Horizontal",
		"Default solid + dashed sugar (horizontal.tsx).",
		primitive.Column(
			para("Lorem ipsum dolor sit amet, consectetur adipiscing elit."),
			divPlain.Node(),
			para("Sed nonne merninisti licere mihi ista probare."),
			divDashed.Node(),
			para("Refert tamen, quo modo."),
		))

	mkTitleDiv := func(title string, place kit.DividerTitlePlacement, plain bool) core.Node {
		d := kit.NewDivider()
		d.SetTitle(title)
		d.SetTitlePlacement(place)
		d.SetPlain(plain)
		d.SetFace(c.face)
		return d.Node()
	}
	secDivWithText := demoSection(c.face, c.theme, "With text",
		"titlePlacement center / start / end (with-text.tsx).",
		primitive.Column(
			para("Above center"),
			mkTitleDiv("Text", kit.DividerTitleCenter, false),
			para("Above start"),
			mkTitleDiv("Left Text", kit.DividerTitleStart, false),
			para("Above end"),
			mkTitleDiv("Right Text", kit.DividerTitleEnd, false),
		))

	mkSize := func(s kit.DividerSize, label string) core.Node {
		d := kit.NewDivider()
		d.SetSize(s)
		return primitive.Column(para(label), d.Node())
	}
	secDivSize := demoSection(c.face, c.theme, "Size",
		"Horizontal marginBlock: small=8 · medium=16 · large/unset=24 (size.tsx).",
		primitive.Column(
			mkSize(kit.DividerSizeSmall, "small"),
			mkSize(kit.DividerSizeMedium, "medium"),
			mkSize(kit.DividerSizeLarge, "large"),
		))

	secDivPlain := demoSection(c.face, c.theme, "Plain",
		"Title uses body fontSize=14 (plain.tsx).",
		primitive.Column(
			mkTitleDiv("Text", kit.DividerTitleCenter, true),
			mkTitleDiv("Left Text", kit.DividerTitleStart, true),
			mkTitleDiv("Right Text", kit.DividerTitleEnd, true),
		))

	mkVert := func() core.Node {
		d := kit.NewDivider()
		d.SetOrientation(kit.DividerVertical)
		return d.Node()
	}
	secDivVertical := demoSection(c.face, c.theme, "Vertical",
		"Inline vertical rails (vertical.tsx).",
		func() core.Node {
			mkLab := func(s string) core.Node {
				tx := kit.NewText(s)
				tx.SetFace(c.face)
				return tx.Node()
			}
			row := primitive.Row(
				mkLab("Text"),
				mkVert(),
				mkLab("Link"),
				mkVert(),
				mkLab("Link"),
			)
			row.CrossAlign = core.CrossCenter
			row.Gap = 4
			return row
		}())

	mkDivVar := func(v kit.DividerVariant, title string) core.Node {
		d := kit.NewDivider()
		d.SetTitle(title)
		d.SetVariant(v)
		d.SetFace(c.face)
		d.SetStyle(kit.Style{Border: render.Hex("#7cb305")})
		return d.Node()
	}
	secDivVariant := demoSection(c.face, c.theme, "Variant",
		"solid / dotted / dashed + Style.Border override (variant.tsx).",
		primitive.Column(
			mkDivVar(kit.DividerSolid, "Solid"),
			mkDivVar(kit.DividerDotted, "Dotted"),
			mkDivVar(kit.DividerDashed, "Dashed"),
		))

	labInline := func(s string) core.Node {
		tx := kit.NewText(s)
		tx.SetFace(c.face)
		return tx.Node()
	}
	secDivSemantic := demoSection(c.face, c.theme, "Structure (semantic)",
		"root / rail / content structure (classNames depth = P1).",
		func() core.Node {
			col := primitive.Column(
				mkTitleDiv("root · rail · content", kit.DividerTitleCenter, false),
				func() core.Node {
					// antd _semantic.tsx: These | are | vertical | Dividers
					row := primitive.Row(
						labInline("These"),
						mkVert(),
						labInline("are"),
						mkVert(),
						labInline("vertical"),
						mkVert(),
						labInline("Dividers"),
					)
					row.CrossAlign = core.CrossCenter
					row.Gap = 4
					return row
				}(),
			)
			col.Gap = 8
			col.CrossAlign = core.CrossStretch
			// Extra bottom inset so last line is not flush against scroll clip.
			col.Padding = primitive.EdgeInsets{Bottom: 8}
			return col
		}())

	// style-class.tsx — semantic classNames/styles 深度为 P1；示例用 Title/Placement/Size/Style 贴近官方四格
	secDivStyleClass := demoSection(c.face, c.theme, "Style / classNames",
		"antd style-class.tsx：classNames Object|Function · styles Object|Function（API 深度 P1，此处示意）。",
		func() core.Node {
			// 1) classNames Object — 仅挂标题，语义名写在文案上
			dClassObj := kit.NewDivider()
			dClassObj.SetTitle("classNames Object")
			dClassObj.SetFace(c.face)

			// 2) classNames Function — titlePlacement=start 时走不同分支（antd info.props.titlePlacement）
			dClassFn := kit.NewDivider()
			dClassFn.SetTitle("classNames Function")
			dClassFn.SetTitlePlacement(kit.DividerTitleStart)
			dClassFn.SetFace(c.face)

			// 3) styles Object — root dashed+较粗线色、content 次级色示意 italic、rail 略淡
			dStylesObj := kit.NewDivider()
			dStylesObj.SetTitle("styles Object")
			dStylesObj.SetVariant(kit.DividerDashed)
			dStylesObj.SetFace(c.face)
			dStylesObj.SetStyle(kit.Style{
				Border: render.Hex("#1677FF"),
				Text:   render.RGBA{R: 0, G: 0, B: 0, A: 0.45}, // content 示意
			})

			// 4) styles Function — size=small 时弱化；否则浅底+边色（antd stylesFn 分支）
			dStylesFnSmall := kit.NewDivider()
			dStylesFnSmall.SetTitle("styles Function (size=small)")
			dStylesFnSmall.SetSize(kit.DividerSizeSmall)
			dStylesFnSmall.SetFace(c.face)
			dStylesFnSmall.SetStyle(kit.Style{
				Text: render.RGBA{R: 0, G: 0, B: 0, A: 0.35}, // opacity≈0.6 示意
			})
			// wrap small sample with muted chrome to echo root opacity
			smallHost := primitive.NewDecorated(dStylesFnSmall.Node())
			smallHost.Padding = primitive.Symmetric(0, 0)
			smallHost.Background = render.RGBA{R: 1, G: 1, B: 1, A: 0.6}

			dStylesFnDefault := kit.NewDivider()
			dStylesFnDefault.SetTitle("styles Function (default)")
			dStylesFnDefault.SetFace(c.face)
			dStylesFnDefault.SetStyle(kit.Style{Border: render.Hex("#d9d9d9")})
			defHost := primitive.NewDecorated(dStylesFnDefault.Node())
			defHost.Padding = primitive.Symmetric(8, 4)
			defHost.Background = render.Hex("#fafafa")
			defHost.BorderWidth = 1
			defHost.BorderColor = render.Hex("#d9d9d9")
			defHost.Radius = 4

			note := kit.NewText("注：class 字符串 / 函数式 semantic 钩子为 P1；上列用 Placement·Size·Variant·Style 复现主视觉。")
			note.SetFace(c.face)
			note.SetSecondary(true)

			col := primitive.Column(
				dClassObj.Node(),
				dClassFn.Node(),
				dStylesObj.Node(),
				smallHost,
				defHost,
				note.Node(),
			)
			col.Gap = 12
			col.CrossAlign = core.CrossStretch
			col.Padding = primitive.EdgeInsets{Bottom: 8}
			return col
		}())

	c.items = append(c.items, ctlTab("divider", "Divider"))
	c.contents["divider"] = demoPage(c.face, "Divider",
		"区隔内容的分割线。P0: orientation / size / variant / title / plain / titlePlacement；style-class 示意（semantic 深度 P1）。",
		secDivHorizontal, secDivWithText, secDivSize, secDivPlain, secDivVertical, secDivVariant, secDivStyleClass, secDivSemantic,
	)
}
