//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerRate() {
	// Rate — docs/antd/rate.md §6.8 P0
	// https://ant.design/components/rate
	// demos: basic / size / half / text / disabled / clear / character / character-function
	face, th := c.face, c.theme
	wire := func(r *kit.Rate, tag string) *kit.Rate {
		r.SetFace(face)
		if th != nil {
			r.SetTheme(th)
		}
		r.SetOnChange(func(v float64) {
			*c.status = fmt.Sprintf("rate %s → %.1f", tag, v)
		})
		return r
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}
	row := func(kids ...core.Node) core.Node {
		return spaceWrap(16, kids...)
	}
	label := func(s string) core.Node {
		t := kit.NewText(s)
		t.SetFace(face)
		t.SetStyle(kit.Style{Text: kit.DefaultTheme().Color(core.TokenColorTextSecondary)})
		return t.Node()
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewRate(), "basic")
	secBasic := demoSection(face, th, "Basic",
		"The simplest use.",
		basic.Node())

	// ---------- size.tsx ----------
	szLG := wire(kit.NewRate(), "size-lg")
	szLG.SetSize(kit.RateLarge)
	szMD := wire(kit.NewRate(), "size-md")
	szSM := wire(kit.NewRate(), "size-sm")
	szSM.SetSize(kit.RateSmall)
	secSize := demoSection(face, th, "尺寸",
		"size=large / middle / small（starSize 25 / 20 / 15）。",
		col(szLG.Node(), szMD.Node(), szSM.Node()))

	// ---------- half.tsx ----------
	half := wire(kit.NewRate(), "half")
	half.SetAllowHalf(true)
	half.SetDefaultValue(2.5)
	secHalf := demoSection(face, th, "半星",
		"allowHalf + defaultValue=2.5。",
		half.Node())

	// ---------- text.tsx ----------
	desc := []string{"terrible", "bad", "normal", "good", "wonderful"}
	textRate := wire(kit.NewRate(), "text")
	textRate.SetTooltips(desc)
	textRate.SetControlled(true)
	textRate.SetValue(3)
	textLab := kit.NewText(desc[2])
	textLab.SetFace(face)
	textRate.SetOnChange(func(v float64) {
		textRate.SetValue(v)
		if v <= 0 {
			textLab.SetValue("")
			*c.status = "rate text → 0"
			return
		}
		idx := int(v) - 1
		if idx >= 0 && idx < len(desc) {
			textLab.SetValue(desc[idx])
		}
		*c.status = fmt.Sprintf("rate text → %.0f (%s)", v, textLab.Value)
	})
	secText := demoSection(face, th, "文案展现",
		"tooltips + 旁路文案随 value 变化。",
		row(textRate.Node(), textLab.Node()))

	// ---------- disabled.tsx ----------
	dis := wire(kit.NewRate(), "disabled")
	dis.SetDefaultValue(2)
	dis.SetDisabled(true)
	secDis := demoSection(face, th, "只读",
		"disabled 只读展示。",
		dis.Node())

	// ---------- clear.tsx ----------
	clrYes := wire(kit.NewRate(), "clear-yes")
	clrYes.SetDefaultValue(3)
	clrNo := wire(kit.NewRate(), "clear-no")
	clrNo.SetDefaultValue(3)
	clrNo.SetAllowClear(false)
	secClear := demoSection(face, th, "清除",
		"allowClear true（再点清空）/ false。",
		col(
			row(clrYes.Node(), label("allowClear: true")),
			row(clrNo.Node(), label("allowClear: false")),
		))

	// ---------- character.tsx ----------
	chHeart := wire(kit.NewRate(), "char-heart")
	chHeart.SetCharacter("♥")
	chHeart.SetAllowHalf(true)
	chA := wire(kit.NewRate(), "char-A")
	chA.SetCharacter("A")
	chA.SetAllowHalf(true)
	chA.SetStyle(kit.Style{FontSize: 36}) // note: star size still from Size; glyph paints at starSize
	chHao := wire(kit.NewRate(), "char-hao")
	chHao.SetCharacter("好")
	chHao.SetAllowHalf(true)
	secChar := demoSection(face, th, "其他字符",
		"自定义 character 字符串（♥ / A / 好）+ allowHalf。",
		col(chHeart.Node(), chA.Node(), chHao.Node()))

	// ---------- character-function.tsx ----------
	chFnNum := wire(kit.NewRate(), "char-fn-num")
	chFnNum.SetDefaultValue(2)
	chFnNum.SetCharacterAt(func(index int) string {
		return fmt.Sprintf("%d", index+1)
	})
	faces := []string{"☹", "☹", "😐", "☺", "☺"}
	chFnFace := wire(kit.NewRate(), "char-fn-face")
	chFnFace.SetDefaultValue(3)
	chFnFace.SetCharacterAt(func(index int) string {
		if index >= 0 && index < len(faces) {
			return faces[index]
		}
		return "★"
	})
	secCharFn := demoSection(face, th, "自定义字符",
		"character(index) 按星渲染（数字 / 表情）。",
		col(chFnNum.Node(), chFnFace.Node()))

	c.add("rate", "Rate", "Data Entry · Rate",
		demoPage(face, "Rate",
			"评分。P0: value/defaultValue/controlled、onChange/onHoverChange、disabled、size、count、allowClear、allowHalf、character、tooltips、keyboard、官方 basic/size/half/text/disabled/clear/character。",
			secBasic, secSize, secHalf, secText, secDis, secClear, secChar, secCharFn))
}
