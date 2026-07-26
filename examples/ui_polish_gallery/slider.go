//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerSlider() {
	// Slider — docs/antd/slider.md §6.8 P0
	// https://ant.design/components/slider
	// demos: basic / input-number / icon-slider / tip-formatter / event / mark / vertical / show-tooltip
	face, th := c.face, c.theme
	wire := func(s *kit.Slider, tag string) *kit.Slider {
		s.SetFace(face)
		if th != nil {
			s.SetTheme(th)
		}
		s.SetOnChange(func(v float64) {
			*c.status = fmt.Sprintf("slider %s → %.2g", tag, v)
		})
		s.SetOnChangeComplete(func(vals []float64) {
			*c.status = fmt.Sprintf("slider %s complete → %v", tag, vals)
		})
		return s
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
	fullWidth := func(n core.Node) core.Node {
		h := primitive.NewDecorated(n)
		h.ExpandWidth = true
		h.StretchChild = true
		return h
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewSlider(30), "basic")
	basicRange := wire(kit.NewSlider(0), "basic-range")
	basicRange.SetRange(true)
	basicRange.SetDefaultValues(20, 50)
	disSW := kit.NewSwitch()
	disSW.SetFace(face)
	if th != nil {
		disSW.Theme = th
	}
	disSW.SetOnChange(func(on bool) {
		basic.SetDisabled(on)
		basicRange.SetDisabled(on)
		*c.status = fmt.Sprintf("slider disabled → %v", on)
	})
	secBasic := demoSection(face, th, "基本",
		"defaultValue + range；Switch 切换 disabled。",
		col(
			fullWidth(basic.Node()),
			fullWidth(basicRange.Node()),
			row(label("Disabled:"), disSW.Node()),
		))

	// ---------- input-number.tsx ----------
	intSlider := wire(kit.NewSlider(1), "input-int")
	intSlider.SetMin(1)
	intSlider.SetMax(20)
	intSlider.SetControlled(true)
	intSlider.SetValue(1)
	intBox := kit.NewInputNumber()
	intBox.SetFace(face)
	if th != nil {
		intBox.SetTheme(th)
	}
	intBox.SetMin(1)
	intBox.SetMax(20)
	intBox.SetValue(1)
	intSlider.SetOnChange(func(v float64) {
		intSlider.SetValue(v)
		intBox.SetValue(v)
		*c.status = fmt.Sprintf("slider input-int → %.0f", v)
	})
	intBox.SetOnChange(func(v float64) {
		intSlider.SetValue(v)
		*c.status = fmt.Sprintf("input-number int → %.0f", v)
	})

	decSlider := wire(kit.NewSlider(0), "input-dec")
	decSlider.SetMin(0)
	decSlider.SetMax(1)
	decSlider.SetStep(0.01)
	decSlider.SetControlled(true)
	decSlider.SetValue(0)
	decBox := kit.NewInputNumber()
	decBox.SetFace(face)
	if th != nil {
		decBox.SetTheme(th)
	}
	decBox.SetMin(0)
	decBox.SetMax(1)
	decBox.SetStep(0.01)
	decBox.SetValue(0)
	decSlider.SetOnChange(func(v float64) {
		decSlider.SetValue(v)
		decBox.SetValue(v)
		*c.status = fmt.Sprintf("slider input-dec → %.2f", v)
	})
	decBox.SetOnChange(func(v float64) {
		decSlider.SetValue(v)
		*c.status = fmt.Sprintf("input-number dec → %.2f", v)
	})
	secInput := demoSection(face, th, "带输入框的滑块",
		"与 InputNumber 双向同步（整数 / 小数 step=0.01）。",
		col(
			row(fullWidth(intSlider.Node()), intBox.Node()),
			row(fullWidth(decSlider.Node()), decBox.Node()),
		))

	// ---------- icon-slider.tsx ----------
	iconSlider := wire(kit.NewSlider(0), "icon")
	iconSlider.SetMin(0)
	iconSlider.SetMax(20)
	iconSlider.SetControlled(true)
	iconSlider.SetValue(0)
	frown := kit.NewText("☹")
	frown.SetFace(face)
	frown.SetFontSize(18)
	smile := kit.NewText("☺")
	smile.SetFace(face)
	smile.SetFontSize(18)
	mid := 10.0
	applyIconChrome := func(v float64) {
		if v < mid {
			frown.SetStyle(kit.Style{Text: kit.DefaultTheme().Color(core.TokenColorPrimary)})
			smile.SetStyle(kit.Style{Text: kit.DefaultTheme().Color(core.TokenColorTextSecondary)})
		} else {
			frown.SetStyle(kit.Style{Text: kit.DefaultTheme().Color(core.TokenColorTextSecondary)})
			smile.SetStyle(kit.Style{Text: kit.DefaultTheme().Color(core.TokenColorPrimary)})
		}
	}
	applyIconChrome(0)
	iconSlider.SetOnChange(func(v float64) {
		iconSlider.SetValue(v)
		applyIconChrome(v)
		*c.status = fmt.Sprintf("slider icon → %.0f", v)
	})
	secIcon := demoSection(face, th, "带 icon 的滑块",
		"两侧表情随 value 相对 mid 高亮。",
		row(frown.Node(), fullWidth(iconSlider.Node()), smile.Node()))

	// ---------- tip-formatter.tsx ----------
	tipFmt := wire(kit.NewSlider(0), "tip-fmt")
	tipFmt.SetDefaultValue(20)
	tipFmt.SetTooltipFormatter(func(v float64) string { return fmt.Sprintf("%.0f%%", v) })
	tipNull := wire(kit.NewSlider(0), "tip-null")
	tipNull.SetDefaultValue(20)
	tipNull.SetTooltipFormatterNull(true)
	secTip := demoSection(face, th, "自定义提示",
		"tooltip.formatter → `n%`；formatter=null 关闭文案。",
		col(fullWidth(tipFmt.Node()), fullWidth(tipNull.Node())))

	// ---------- event.tsx ----------
	evSingle := wire(kit.NewSlider(30), "event")
	evRange := wire(kit.NewSlider(0), "event-range")
	evRange.SetRange(true)
	evRange.SetStep(10)
	evRange.SetDefaultValues(20, 50)
	evRange.SetOnRangeChange(func(lo, hi float64) {
		*c.status = fmt.Sprintf("slider event-range → [%.0f, %.0f]", lo, hi)
	})
	secEvent := demoSection(face, th, "事件",
		"onChange（拖动中）+ onChangeComplete（松手）。",
		col(fullWidth(evSingle.Node()), fullWidth(evRange.Node())))

	// ---------- mark.tsx ----------
	marks := []kit.SliderMark{
		{Value: 0, Label: "0°C"},
		{Value: 26, Label: "26°C"},
		{Value: 37, Label: "37°C"},
		{Value: 100, Label: "100°C"},
	}
	mkInc := wire(kit.NewSlider(37), "mark-inc")
	mkInc.SetMarks(marks...)
	mkRange := wire(kit.NewSlider(0), "mark-range")
	mkRange.SetRange(true)
	mkRange.SetMarks(marks...)
	mkRange.SetDefaultValues(26, 37)
	mkExcl := wire(kit.NewSlider(37), "mark-excl")
	mkExcl.SetMarks(marks...)
	mkExcl.SetIncluded(false)
	mkStep := wire(kit.NewSlider(37), "mark-step")
	mkStep.SetMarks(marks...)
	mkStep.SetStep(10)
	mkNull := wire(kit.NewSlider(37), "mark-null")
	mkNull.SetMarks(marks...)
	mkNull.SetStepNull(true)
	secMark := demoSection(face, th, "带标签的滑块",
		"marks + included true/false + step / step=null。",
		col(
			label("included=true"),
			fullWidth(mkInc.Node()),
			fullWidth(mkRange.Node()),
			label("included=false"),
			fullWidth(mkExcl.Node()),
			label("marks & step=10"),
			fullWidth(mkStep.Node()),
			label("step=null"),
			fullWidth(mkNull.Node()),
		))

	// ---------- vertical.tsx ----------
	vBox := func(n core.Node) core.Node {
		h := primitive.NewDecorated(n)
		h.Width = 56
		h.Height = 300
		h.StretchChild = true
		return h
	}
	v1 := wire(kit.NewSlider(30), "vert")
	v1.SetVertical(true)
	v1.SetHeight(300)
	v2 := wire(kit.NewSlider(0), "vert-range")
	v2.SetVertical(true)
	v2.SetRange(true)
	v2.SetStep(10)
	v2.SetDefaultValues(20, 50)
	v2.SetHeight(300)
	v3 := wire(kit.NewSlider(0), "vert-marks")
	v3.SetVertical(true)
	v3.SetRange(true)
	v3.SetMarks(marks...)
	v3.SetDefaultValues(26, 37)
	v3.SetHeight(300)
	secVert := demoSection(face, th, "垂直",
		"orientation=vertical；单值 / range+step / range+marks。",
		row(vBox(v1.Node()), vBox(v2.Node()), vBox(v3.Node())))

	// ---------- show-tooltip.tsx ----------
	showTip := wire(kit.NewSlider(30), "show-tip")
	showTip.SetTooltipOpen(kit.SliderTooltipAlways)
	secShowTip := demoSection(face, th, "控制 ToolTip 的显示",
		"tooltip.open=true 常显当前值。",
		fullWidth(showTip.Node()))

	// Lifecycle (#9)
	life := wire(kit.NewSlider(40), "lifecycle")
	life.SetMin(0)
	life.SetMax(100)
	life.SetStep(5)
	life.SetDots(true)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (Range/Marks) then chromeChange (Step/Dots)；ensureBuilt 懒构建。",
		fullWidth(life.Node()))

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeSlider, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok && d != nil {
			if d.Base().Key == "slider-skin-demo" {
				d.BorderWidth = 2
				d.BorderColor = render.Hex("#1677FF")
			}
			if p := baseSkin.Painter(kit.TypeSlider); p != nil {
				p(pc, d)
				return
			}
			primitive.PaintDecorated(pc, d)
			return
		}
		if p := baseSkin.Painter(kit.TypeSlider); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinS := wire(kit.NewSlider(60), "skin")
	skinHost := primitive.NewDecorated(skinS.Node())
	skinHost.ExpandWidth = true
	skinHost.StretchChild = true
	skinHost.SkinType = kit.TypeSlider
	skinHost.Base().Key = "slider-skin-demo"
	skinHost.Padding = primitive.All(4)
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"sliderHost TypeID=kit.Slider；宿主 Decorated SkinType=kit.Slider → 蓝色 2px 边框 Override。",
		skinHost)

	c.add("slider", "Slider", "Data Entry · Slider",
		demoPage(face, "Slider",
			"滑动输入器。P0: value/defaultValue/onChange/onChangeComplete、min/max/step、range、marks/included/dots、disabled、keyboard、orientation/vertical、tooltip open/formatter、官方 basic/input-number/icon/tip-formatter/event/mark/vertical/show-tooltip。",
			secBasic, secInput, secIcon, secTip, secEvent, secMark, secVert, secShowTip, secLife, secSkin))
}
