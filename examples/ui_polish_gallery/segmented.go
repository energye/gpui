//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerSegmented() {
	// Segmented — docs/antd/segmented.md §6.8 P0
	// https://ant.design/components/segmented
	// demos: basic / vertical / block / shape / disabled / controlled / custom / dynamic
	face, th := c.face, c.theme
	wire := func(s *kit.Segmented, tag string) *kit.Segmented {
		s.SetFace(face)
		if th != nil {
			s.SetTheme(th)
		}
		s.SetOnChange(func(v string) {
			*c.status = fmt.Sprintf("segmented %s → %s", tag, v)
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
	blockHost := func(child core.Node, w float64) core.Node {
		d := primitive.NewDecorated(child)
		d.Width = w
		d.StretchChild = true
		d.ExpandWidth = true
		return d
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewSegmented("Daily", "Weekly", "Monthly", "Quarterly", "Yearly"), "basic")
	secBasic := demoSection(face, th, "Basic",
		"The simplest use.",
		basic.Node())

	// ---------- vertical.tsx ----------
	vert := wire(kit.NewSegmentedOptions(
		kit.SegmentedOption{Value: "List", Icon: "check", AriaLabel: "List"},
		kit.SegmentedOption{Value: "Kanban", Icon: "user", AriaLabel: "Kanban"},
	), "vertical")
	vert.SetOrientation(kit.SegmentedVertical)
	secVert := demoSection(face, th, "Vertical",
		"orientation=vertical（icon-only 选项）。",
		vert.Node())

	// ---------- block.tsx ----------
	blk := wire(kit.NewSegmentedOptions(
		kit.SegmentedOption{Label: "123", Value: "123"},
		kit.SegmentedOption{Label: "456", Value: "456"},
		kit.SegmentedOption{Label: "longtext-longtext-longtext-longtext", Value: "long"},
	), "block")
	blk.SetBlock(true)
	secBlock := demoSection(face, th, "Block",
		"block 撑满父宽，item 均分。",
		blockHost(blk.Node(), 480))

	// ---------- shape.tsx ----------
	sizeCtl := wire(kit.NewSegmented("small", "medium", "large"), "shape-size")
	sizeCtl.SetValue("medium")
	round := wire(kit.NewSegmentedOptions(
		kit.SegmentedOption{Value: "light", Icon: "star", AriaLabel: "light"},
		kit.SegmentedOption{Value: "dark", Icon: "heart", AriaLabel: "dark"},
	), "shape-round")
	round.SetShape(kit.SegmentedShapeRound)
	round.SetSize(kit.SegmentedMiddle)
	sizeCtl.SetOnChange(func(v string) {
		*c.status = fmt.Sprintf("segmented shape-size → %s", v)
		switch v {
		case "small":
			round.SetSize(kit.SegmentedSmall)
		case "large":
			round.SetSize(kit.SegmentedLarge)
		default:
			round.SetSize(kit.SegmentedMiddle)
		}
	})
	secShape := demoSection(face, th, "Shape",
		"shape=round 胶囊 + size 联动。",
		col(sizeCtl.Node(), round.Node()))

	// ---------- disabled.tsx ----------
	disAll := wire(kit.NewSegmented("Map", "Transit", "Satellite"), "dis-all")
	disAll.SetDisabled(true)
	disMix := wire(kit.NewSegmentedOptions(
		kit.SegmentedOption{Label: "Daily", Value: "Daily"},
		kit.SegmentedOption{Label: "Weekly", Value: "Weekly", Disabled: true},
		kit.SegmentedOption{Label: "Monthly", Value: "Monthly"},
		kit.SegmentedOption{Label: "Quarterly", Value: "Quarterly", Disabled: true},
		kit.SegmentedOption{Label: "Yearly", Value: "Yearly"},
	), "dis-mix")
	secDis := demoSection(face, th, "Disabled",
		"整体 disabled + 单项 disabled。",
		col(disAll.Node(), disMix.Node()))

	// ---------- controlled.tsx ----------
	ctrl := wire(kit.NewSegmented("Map", "Transit", "Satellite"), "controlled")
	ctrl.SetControlled(true)
	ctrl.SetValue("Map")
	ctrl.SetOnChange(func(v string) {
		ctrl.SetValue(v)
		*c.status = fmt.Sprintf("segmented controlled → %s", v)
	})
	secCtrl := demoSection(face, th, "Controlled",
		"value + onChange 受控同步。",
		ctrl.Node())

	// ---------- custom.tsx ----------
	mkSeason := func(title, sub string) core.Node {
		t1 := primitive.NewText(title)
		t1.FontSize = 14
		t1.Face = face
		t2 := primitive.NewText(sub)
		t2.FontSize = 12
		t2.Face = face
		t2.Color = kit.DefaultTheme().Color(core.TokenColorTextSecondary)
		colN := primitive.Column(t1, t2)
		colN.Gap = 2
		colN.CrossAlign = core.CrossCenter
		colN.Padding = primitive.All(4)
		return colN
	}
	custom := wire(kit.NewSegmentedOptions(
		kit.SegmentedOption{Value: "spring", LabelNode: mkSeason("Spring", "Jan-Mar")},
		kit.SegmentedOption{Value: "summer", LabelNode: mkSeason("Summer", "Apr-Jun")},
		kit.SegmentedOption{Value: "autumn", LabelNode: mkSeason("Autumn", "Jul-Sept")},
		kit.SegmentedOption{Value: "winter", LabelNode: mkSeason("Winter", "Oct-Dec")},
	), "custom")
	secCustom := demoSection(face, th, "Custom render",
		"option.LabelNode 自定义内容（季节卡片）。",
		custom.Node())

	// ---------- dynamic.tsx ----------
	dyn := wire(kit.NewSegmented("Daily", "Weekly", "Monthly"), "dynamic")
	moreLoaded := false
	loadBtn := c.trackBtn(kit.NewButton("Load more options"))
	loadBtn.SetType(kit.ButtonPrimary)
	loadBtn.SetOnClick(func() {
		if moreLoaded {
			return
		}
		dyn.SetOptionsStrings("Daily", "Weekly", "Monthly", "Quarterly", "Yearly")
		moreLoaded = true
		loadBtn.SetDisabled(true)
		*c.status = "segmented dynamic → loaded Quarterly/Yearly"
	})
	secDyn := demoSection(face, th, "Dynamic",
		"SetOptions 动态追加选项。",
		col(dyn.Node(), loadBtn.Node()))

	// Lifecycle (#9)
	life := wire(kit.NewSegmented("One", "Two", "Three"), "lifecycle")
	life.SetSize(kit.SegmentedLarge)
	life.SetShape(kit.SegmentedShapeRound)
	life.SetValue("Two")
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (Size/Shape) then chromeChange (Value)；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeSegmented, func(pc *core.PaintContext, n core.Node) {
		d, ok := n.(*primitive.Decorated)
		if !ok || d == nil {
			return
		}
		if d.Base().Key == "segmented-skin-demo" {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#1677FF")
		}
		if p := baseSkin.Painter(kit.TypeSegmented); p != nil {
			p(pc, d)
			return
		}
		primitive.PaintDecorated(pc, d)
	})
	skinSeg := wire(kit.NewSegmented("Skin", "Painter", "Demo"), "skin")
	if dec, ok := skinSeg.ChromeNode().(*primitive.Decorated); ok {
		dec.Base().Key = "segmented-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root.SkinType=kit.Segmented。Key=segmented-skin-demo → 蓝色 2px 边框 Override。",
		skinSeg.Node())

	c.addPage("segmented", "Segmented",
		demoPage(face, "Segmented 分段控制器",
			"用于展示多个选项并允许用户选择其中单个选项。P0：value/defaultValue/onChange、options、disabled、size、block、orientation/vertical、shape、icon/LabelNode、动态 SetOptions。\n"+
				"P1：name、tooltip 完整、三种大小/图标/with-name 完整页、thumb 滑动动画、semantic classNames/styles。",
			secBasic, secVert, secBlock, secShape, secDis, secCtrl, secCustom, secDyn, secLife, secSkin))
}
