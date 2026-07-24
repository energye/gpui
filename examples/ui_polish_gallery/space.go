//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerSpace() {
	// Space — antd demos §6.8 P0:
	// base / vertical / size / align / wrap / separator / compact / compact-buttons
	// https://ant.design/components/space · components/space/demo/*.tsx
	//
	// P1 not shown: compact-button-vertical, style-class / semantic, debug.

	mkBtn := func(label string, typ kit.ButtonType) *kit.Button {
		b := kit.NewButton(label)
		b.SetFace(c.face)
		b.SetType(typ)
		return c.trackBtn(b)
	}
	mkTag := func(lab string) core.Node {
		t := kit.NewTag(lab)
		return t.Node()
	}
	playground := func(body core.Node) *primitive.Decorated {
		d := primitive.NewDecorated(body)
		d.ExpandWidth = true
		d.Padding = primitive.All(8)
		d.Radius = 6
		d.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		return d
	}
	// flexPct wraps a control so Compact block children take a share of parent
	// width (antd style={{width:'20%'}} etc.). remainingGrow leaves empty tail
	// so items do not stretch to 100% when antd only used part of the row.
	flexPct := func(n core.Node, grow float64) core.Node {
		if d, ok := n.(*primitive.Decorated); ok {
			d.ExpandWidth = true
		}
		fl := primitive.NewFlexible(grow, n)
		fl.FillChild = true
		return fl
	}

	// ---------- base.tsx ----------
	baseSp := kit.NewSpace(
		kit.NewText("Space").Node(),
		mkBtn("Button", kit.ButtonPrimary).Node(),
		mkBtn("Click to Upload", kit.ButtonDefault).Node(),
		mkBtn("Confirm", kit.ButtonDefault).Node(),
	)
	secBase := demoSection(c.face, c.theme, "基本用法",
		"antd base.tsx：默认 size=small（gap 8）、水平、交叉轴 center。",
		playground(baseSp.Node()))

	// ---------- vertical.tsx ----------
	mkCard := func() core.Node {
		card := kit.NewCard("Card")
		card.SetContent(kit.NewText("Card content").Node())
		return card.Node()
	}
	vertSp := kit.NewSpace(mkCard(), mkCard(), mkCard())
	vertSp.SetOrientation(kit.SpaceVertical)
	vertSp.SetSize(kit.SpaceSizeMiddle)
	// antd vertical.tsx: style={{ display: 'flex' }} → block-level column
	vertSp.SetExpandMax(true)
	secVert := demoSection(c.face, c.theme, "垂直间距",
		"antd vertical.tsx：orientation=vertical + size=middle（gap 16）；display:flex 通栏。",
		playground(vertSp.Node()))

	// ---------- size.tsx ----------
	// antd: Radio small|medium|large|customize；customize 时出现 Slider。
	// SPC-12 / size.tsx
	sizeRow := kit.NewSpace(
		mkBtn("Primary", kit.ButtonPrimary).Node(),
		mkBtn("Default", kit.ButtonDefault).Node(),
		mkBtn("Dashed", kit.ButtonDashed).Node(),
		mkBtn("Link", kit.ButtonLink).Node(),
	)
	sizeRow.SetSize(kit.SpaceSizeSmall)
	sizeHost := playground(sizeRow.Node())

	rs := kit.NewRadio("small", "small")
	rm := kit.NewRadio("medium", "medium")
	rl := kit.NewRadio("large", "large")
	rc := kit.NewRadio("customize", "customize")
	for _, r := range []*kit.Radio{rs, rm, rl, rc} {
		r.SetFace(c.face)
	}
	sizeRG := kit.NewRadioGroup(rs, rm, rl, rc)
	sizeRG.Select("small")
	if fl, ok := sizeRG.Node().(*primitive.Flex); ok {
		fl.Axis = core.AxisHorizontal
		fl.CrossAlign = core.CrossCenter
		fl.Gap = 16
		fl.Wrap = true
	}

	sizeSlider := kit.NewSlider(16)
	sizeSlider.Min, sizeSlider.Max = 0, 64
	sizeSlider.SetWidth(0) // fill ExpandWidth host
	sliderSlot := primitive.NewSlot("space-size-slider", nil)

	applySizeMode := func(mode string) {
		switch mode {
		case "medium":
			sizeRow.SetSize(kit.SpaceSizeMiddle)
			sliderSlot.SetChild(nil)
		case "large":
			sizeRow.SetSize(kit.SpaceSizeLarge)
			sliderSlot.SetChild(nil)
		case "customize":
			sizeRow.SetSizePx(sizeSlider.Value)
			sh := primitive.NewDecorated(sizeSlider.Node())
			sh.ExpandWidth = true
			sh.Height = 32
			sh.StretchChild = true
			sliderSlot.SetChild(sh)
		default: // small
			sizeRow.SetSize(kit.SpaceSizeSmall)
			sliderSlot.SetChild(nil)
		}
		sizeHost.MarkNeedsLayout()
		*c.status = "space size → " + mode
	}
	sizeRG.OnChange = applySizeMode
	sizeSlider.OnChange = func(v float64) {
		if sizeRG.Value == "customize" {
			sizeRow.SetSizePx(v)
			sizeHost.MarkNeedsLayout()
			*c.status = fmt.Sprintf("space size custom → %.0f", v)
		}
	}

	sizeOuter := kit.NewFlex(sizeRG.Node(), sliderSlot, sizeHost)
	sizeOuter.SetVertical(true)
	sizeOuter.SetGapSize(kit.FlexGapMedium)
	secSize := demoSection(c.face, c.theme, "间距大小",
		"antd size.tsx：Radio 切换 small(8)/medium(16)/large(24)/customize；customize 时出现 Slider（0–64）。",
		sizeOuter.Node())

	// ---------- align.tsx ----------
	// antd mockBox: padding paddingXL×padding (32×16), text "Block", gray fill.
	// spaceAlignBox: blue border + small pad. SPC-13 / SPC-S7.
	mkAlignBox := func(align kit.SpaceAlign, label string) core.Node {
		blockLab := kit.NewText("Block")
		blockLab.SetFace(c.face)
		mock := primitive.NewDecorated(blockLab.Node())
		// antd: padding: paddingXL padding → 32 vertical, 16 horizontal
		mock.Padding = primitive.EdgeInsets{Left: 16, Right: 16, Top: 32, Bottom: 32}
		mock.Background = render.RGBA{R: 150 / 255.0, G: 150 / 255.0, B: 150 / 255.0, A: 0.2}
		mock.Radius = 0

		labelTx := kit.NewText(label)
		labelTx.SetFace(c.face)
		sp := kit.NewSpace(
			labelTx.Node(),
			mkBtn("Primary", kit.ButtonPrimary).Node(),
			mock,
		)
		sp.SetAlign(align)

		// antd spaceAlignBox: margin XXS, padding XXS, blue border
		box := primitive.NewDecorated(sp.Node())
		box.Padding = primitive.All(4) // paddingXXS ≈ 4
		box.BorderWidth = 1
		box.BorderColor = render.Hex("#1677ff")
		box.Radius = 0
		return box
	}
	alignRow := kit.NewFlex(
		mkAlignBox(kit.SpaceAlignCenter, "center"),
		mkAlignBox(kit.SpaceAlignStart, "start"),
		mkAlignBox(kit.SpaceAlignEnd, "end"),
		mkAlignBox(kit.SpaceAlignBaseline, "baseline"),
	)
	alignRow.SetWrap(true)
	alignRow.SetGap(8) // marginXXS rhythm between boxes
	alignRow.SetAlign(kit.FlexAlignStart)
	secAlign := demoSection(c.face, c.theme, "对齐",
		"antd align.tsx：center/start/end/baseline；Block 为 padding 32×16 灰底文本，与 Primary 高差体现交叉轴对齐。",
		playground(alignRow.Node()))

	// ---------- wrap.tsx ----------
	wrapKids := make([]core.Node, 0, 20)
	for i := 0; i < 20; i++ {
		wrapKids = append(wrapKids, mkBtn("Button", kit.ButtonDefault).Node())
	}
	wrapSp := kit.NewSpace(wrapKids...)
	wrapSp.SetSizeXY(8, 16)
	wrapSp.SetWrap(true)
	secWrap := demoSection(c.face, c.theme, "自动换行",
		"antd wrap.tsx：size={[8,16]} + wrap；窄宿主下多行排布。",
		playground(wrapSp.Node()))

	// ---------- separator.tsx ----------
	link1 := kit.NewLink("Link")
	link1.SetFace(c.face)
	link2 := kit.NewLink("Link")
	link2.SetFace(c.face)
	link3 := kit.NewLink("Link")
	link3.SetFace(c.face)
	sepSp := kit.NewSpace(link1.Node(), link2.Node(), link3.Node())
	sepSp.SetSeparator(func() core.Node {
		d := kit.NewDivider()
		d.SetVertical(true)
		return d.Node()
	})
	secSep := demoSection(c.face, c.theme, "分隔符",
		"antd separator.tsx：子项之间插入垂直 Divider。",
		playground(sepSp.Node()))

	// ---------- compact.tsx (simplified main path) ----------
	// antd: Compact block + children width % / calc(100%-btn) so row ≤ parent.
	// SPC-16: 禁止子项固有宽之和撑破父容器。
	in1 := kit.NewInput("0571")
	in1.SetFace(c.face)
	in1.SetValue("0571")
	in2 := kit.NewInput("26888888")
	in2.SetFace(c.face)
	in2.SetValue("26888888")
	// 20% + 30% + empty 50% (antd phone row)
	cpPhone := kit.NewSpaceCompact(
		flexPct(in1.Node(), 2),
		flexPct(in2.Node(), 3),
		primitive.NewFlexible(5, nil), // empty tail
	)
	cpPhone.SetBlock(true)

	inURL := kit.NewInput("https://ant.design")
	inURL.SetFace(c.face)
	inURL.SetValue("https://ant.design")
	subBtn := mkBtn("Submit", kit.ButtonPrimary)
	// calc(100% - button) ≈ Flexible input + fixed button
	cpURL := kit.NewSpaceCompact(
		flexPct(inURL.Node(), 1),
		subBtn.Node(),
	)
	cpURL.SetBlock(true)

	inMoney := kit.NewInput("amount")
	inMoney.SetFace(c.face)
	moneyLab := kit.NewText("$")
	moneyLab.SetFace(c.face)
	addon := kit.NewSpaceAddon(moneyLab.Node())
	// Register via AddAddon so Compact size/theme push applies (general path).
	cpAddon := kit.NewSpaceCompact()
	cpAddon.SetBlock(true)
	cpAddon.Add(flexPct(inMoney.Node(), 1))
	cpAddon.AddAddon(addon)

	compactCol := kit.NewSpace(cpPhone.Node(), cpURL.Node(), cpAddon.Node())
	compactCol.SetOrientation(kit.SpaceVertical)
	compactCol.SetSize(kit.SpaceSizeMiddle)
	// antd vertical stack + Compact block: Space must be block-level so children
	// see finite MaxWidth (equivalent to style={{display:'flex'}} / full row).
	compactCol.SetExpandMax(true)
	compactHost := playground(compactCol.Node())
	secCompact := demoSection(c.face, c.theme, "紧凑布局组合",
		"antd compact.tsx 主路径：Space.Compact block；子项 Flexible 比例宽（≈20%/30%、calc(100%-btn)），不超出父容器。",
		compactHost)

	// ---------- compact-buttons.tsx ----------
	cpIcons := kit.NewSpaceCompact()
	cpIcons.SetButtons(
		mkBtn("Like", kit.ButtonDefault),
		mkBtn("Comment", kit.ButtonDefault),
		mkBtn("Star", kit.ButtonDefault),
		mkBtn("Heart", kit.ButtonDefault),
		mkBtn("Share", kit.ButtonDefault),
	)
	cpIcons.SetBlock(true)

	cpPrimary := kit.NewSpaceCompact()
	cpPrimary.SetButtons(
		mkBtn("Button 1", kit.ButtonPrimary),
		mkBtn("Button 2", kit.ButtonPrimary),
		mkBtn("Button 3", kit.ButtonPrimary),
		mkBtn("Button 4", kit.ButtonPrimary),
	)
	cpPrimary.SetBlock(true)

	cpMixed := kit.NewSpaceCompact()
	cpMixed.SetButtons(
		mkBtn("Button 1", kit.ButtonDefault),
		mkBtn("Button 2", kit.ButtonDefault),
		mkBtn("Button 3", kit.ButtonDefault),
		mkBtn("Button 4", kit.ButtonPrimary),
	)
	cpMixed.SetBlock(true)

	compactBtns := kit.NewSpace(cpIcons.Node(), cpPrimary.Node(), cpMixed.Node())
	compactBtns.SetOrientation(kit.SpaceVertical)
	compactBtns.SetSize(kit.SpaceSizeMiddle)
	compactBtns.SetExpandMax(true)
	secCompactBtns := demoSection(c.face, c.theme, "Button 紧凑布局",
		"antd compact-buttons.tsx：图标组 / 主色组 / 混合组；中间项圆角清零 + 叠边。",
		playground(compactBtns.Node()))

	// tag row kept for quick visual of default small gap
	_ = kit.NewSpace(mkTag("Tag"), mkTag("Tag"), mkTag("Tag"), mkTag("Tag"))

	c.items = append(c.items, ctlTab("space", "Space"))
	c.contents["space"] = demoPage(c.face, "Space",
		"Ant Design Space · docs/antd/space.md §6 P0：size / orientation / align / wrap / separator / Compact。",
		secBase,
		secVert,
		secSize,
		secAlign,
		secWrap,
		secSep,
		secCompact,
		secCompactBtns,
	)
}
