//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerDescriptions() {
	// Descriptions — docs/antd/descriptions.md §6.8 P0
	// https://ant.design/components/descriptions
	// demos: basic / border / size / responsive / vertical / vertical-border / style-class / block
	//
	// P1 not shown: semantic classNames/styles depth, ConfigProvider 全局, debug/官网逐像素.

	face, th := c.face, c.theme

	wire := func(d *kit.Descriptions) *kit.Descriptions {
		d.SetFace(face)
		if th != nil {
			d.SetTheme(th)
		}
		return d
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewDescriptions(
		kit.DescriptionsItem{Key: "1", Label: "UserName", Children: "Zhou Maomao"},
		kit.DescriptionsItem{Key: "2", Label: "Telephone", Children: "1810000000"},
		kit.DescriptionsItem{Key: "3", Label: "Live", Children: "Hangzhou, Zhejiang"},
		kit.DescriptionsItem{Key: "4", Label: "Remark", Children: "empty"},
		kit.DescriptionsItem{Key: "5", Label: "Address", Children: "No. 18, Wantang Road, Xihu District, Hangzhou, Zhejiang, China"},
	))
	basic.SetTitle("User Info")
	secBasic := demoSection(face, th, "基本",
		"basic.tsx：title + items，默认 column=3 / size=large / 非边框。",
		basic.Node())

	// ---------- border.tsx ----------
	badge := kit.NewBadge()
	badge.SetStatus(kit.BadgeStatusProcessing)
	badge.SetText("Running")
	if th != nil {
		badge.SetTheme(th)
	}
	badge.SetFace(face)

	border := wire(kit.NewDescriptions(
		kit.DescriptionsItem{Key: "1", Label: "Product", Children: "Cloud Database"},
		kit.DescriptionsItem{Key: "2", Label: "Billing Mode", Children: "Prepaid"},
		kit.DescriptionsItem{Key: "3", Label: "Automatic Renewal", Children: "YES"},
		kit.DescriptionsItem{Key: "4", Label: "Order time", Children: "2018-04-24 18:00:00"},
		kit.DescriptionsItem{Key: "5", Label: "Usage Time", Children: "2019-04-24 18:00:00", Span: 2},
		kit.DescriptionsItem{Key: "6", Label: "Status", ChildrenNode: badge.Node(), Span: 3},
		kit.DescriptionsItem{Key: "7", Label: "Negotiated Amount", Children: "$80.00"},
		kit.DescriptionsItem{Key: "8", Label: "Discount", Children: "$20.00"},
		kit.DescriptionsItem{Key: "9", Label: "Official Receipts", Children: "$60.00"},
		kit.DescriptionsItem{Key: "10", Label: "Config Info", Children: "Data disk type: MongoDB\nDatabase version: 3.4\nPackage: dds.mongo.mid\nStorage space: 10 GB\nReplication factor: 3\nRegion: East China 1"},
	))
	border.SetTitle("User Info")
	border.SetBordered(true)
	secBorder := demoSection(face, th, "带边框的",
		"border.tsx：bordered + span=2/3 + Badge status。",
		border.Node())

	// ---------- size.tsx ----------
	sizeItemsBordered := []kit.DescriptionsItem{
		{Key: "1", Label: "Product", Children: "Cloud Database"},
		{Key: "2", Label: "Billing", Children: "Prepaid"},
		{Key: "3", Label: "Time", Children: "18:00:00"},
		{Key: "4", Label: "Amount", Children: "$80.00"},
		{Key: "5", Label: "Discount", Children: "$20.00"},
		{Key: "6", Label: "Official", Children: "$60.00"},
		{Key: "7", Label: "Config Info", Children: "Data disk type: MongoDB\nDatabase version: 3.4\nPackage: dds.mongo.mid"},
	}
	sizeItemsPlain := sizeItemsBordered[:6]
	mkSize := func(sz kit.DescriptionsSize, bordered bool, items []kit.DescriptionsItem) core.Node {
		d := wire(kit.NewDescriptions(items...))
		d.SetTitle("Custom Size")
		d.SetSize(sz)
		d.SetBordered(bordered)
		edit := kit.NewButton("Edit")
		edit.SetType(kit.ButtonPrimary)
		edit.SetFace(face)
		if th != nil {
			edit.Theme = th
		}
		d.SetExtra(edit.Node())
		return d.Node()
	}
	// Default showcase: large bordered + large plain (interactive size switch is L1-covered in tests)
	sizeCol := primitive.Column(
		mkSize(kit.DescriptionsLarge, true, sizeItemsBordered),
		mkSize(kit.DescriptionsMiddle, true, sizeItemsBordered),
		mkSize(kit.DescriptionsSmall, true, sizeItemsBordered),
		mkSize(kit.DescriptionsLarge, false, sizeItemsPlain),
		mkSize(kit.DescriptionsMiddle, false, sizeItemsPlain),
		mkSize(kit.DescriptionsSmall, false, sizeItemsPlain),
	)
	sizeCol.Gap = 24
	sizeCol.CrossAlign = core.CrossStretch
	secSize := demoSection(face, th, "自定义尺寸",
		"size.tsx：large / medium / small × bordered|plain + extra Edit。",
		sizeCol)

	// ---------- responsive.tsx ----------
	resp := wire(kit.NewDescriptions(
		kit.DescriptionsItem{Label: "Product", Children: "Cloud Database"},
		kit.DescriptionsItem{Label: "Billing", Children: "Prepaid"},
		kit.DescriptionsItem{Label: "Time", Children: "18:00:00"},
		kit.DescriptionsItem{Label: "Amount", Children: "$80.00"},
		kit.DescriptionsItem{Label: "Discount", Children: "$20.00", SpanMap: map[string]int{"xl": 2, "xxl": 2}},
		kit.DescriptionsItem{Label: "Official", Children: "$60.00", SpanMap: map[string]int{"xl": 2, "xxl": 2}},
		kit.DescriptionsItem{Label: "Config Info", Children: "Data disk type: MongoDB\nDatabase version: 3.4\nPackage: dds.mongo.mid",
			SpanMap: map[string]int{"xs": 1, "sm": 2, "md": 3, "lg": 3, "xl": 2, "xxl": 2}},
		kit.DescriptionsItem{Label: "Hardware Info", Children: "CPU: 6 Core 3.5 GHz\nStorage space: 10 GB\nReplication factor: 3\nRegion: East China 1",
			SpanMap: map[string]int{"xs": 1, "sm": 2, "md": 3, "lg": 3, "xl": 2, "xxl": 2}},
	))
	resp.SetTitle("Responsive Descriptions")
	resp.SetBordered(true)
	resp.SetColumnMap(map[string]int{"xs": 1, "sm": 2, "md": 3, "lg": 3, "xl": 4, "xxl": 4})
	// Gallery assumes a typical desktop width; host can call SetViewportWidth on resize.
	resp.SetViewportWidth(1200)
	secResp := demoSection(face, th, "响应式",
		"responsive.tsx：column map + item SpanMap；ViewportWidth=1200 → column=4。",
		resp.Node())

	// ---------- vertical.tsx ----------
	vert := wire(kit.NewDescriptions(
		kit.DescriptionsItem{Key: "1", Label: "UserName", Children: "Zhou Maomao"},
		kit.DescriptionsItem{Key: "2", Label: "Telephone", Children: "1810000000"},
		kit.DescriptionsItem{Key: "3", Label: "Live", Children: "Hangzhou, Zhejiang"},
		kit.DescriptionsItem{Key: "4", Label: "Address", Span: 2, Children: "No. 18, Wantang Road, Xihu District, Hangzhou, Zhejiang, China"},
		kit.DescriptionsItem{Key: "5", Label: "Remark", Children: "empty"},
	))
	vert.SetTitle("User Info")
	vert.SetLayout(kit.DescriptionsVertical)
	secVert := demoSection(face, th, "垂直",
		"vertical.tsx：layout=vertical。",
		vert.Node())

	// ---------- vertical-border.tsx ----------
	badge2 := kit.NewBadge()
	badge2.SetStatus(kit.BadgeStatusProcessing)
	badge2.SetText("Running")
	badge2.SetFace(face)
	if th != nil {
		badge2.SetTheme(th)
	}
	vertB := wire(kit.NewDescriptions(
		kit.DescriptionsItem{Key: "1", Label: "Product", Children: "Cloud Database"},
		kit.DescriptionsItem{Key: "2", Label: "Billing Mode", Children: "Prepaid"},
		kit.DescriptionsItem{Key: "3", Label: "Automatic Renewal", Children: "YES"},
		kit.DescriptionsItem{Key: "4", Label: "Order time", Children: "2018-04-24 18:00:00"},
		kit.DescriptionsItem{Key: "5", Label: "Usage Time", Span: 2, Children: "2019-04-24 18:00:00"},
		kit.DescriptionsItem{Key: "6", Label: "Status", Span: 3, ChildrenNode: badge2.Node()},
		kit.DescriptionsItem{Key: "7", Label: "Negotiated Amount", Children: "$80.00"},
		kit.DescriptionsItem{Key: "8", Label: "Discount", Children: "$20.00"},
		kit.DescriptionsItem{Key: "9", Label: "Official Receipts", Children: "$60.00"},
		kit.DescriptionsItem{Key: "10", Label: "Config Info", Children: "Data disk type: MongoDB\nDatabase version: 3.4\nPackage: dds.mongo.mid\nStorage space: 10 GB\nReplication factor: 3\nRegion: East China 1"},
	))
	vertB.SetTitle("User Info")
	vertB.SetLayout(kit.DescriptionsVertical)
	vertB.SetBordered(true)
	secVertB := demoSection(face, th, "垂直带边框的",
		"vertical-border.tsx：layout=vertical + bordered。",
		vertB.Node())

	// ---------- style-class.tsx ----------
	styleItems := []kit.DescriptionsItem{
		{Key: "1", Label: "Product", Children: "Cloud Database"},
		{Key: "2", Label: "Billing Mode", Children: "Prepaid"},
		{Key: "3", Label: "Automatic Renewal", Children: "YES"},
	}
	scSmall := wire(kit.NewDescriptions(styleItems...))
	scSmall.SetTitle("User Info")
	scSmall.SetBordered(true)
	scSmall.SetSize(kit.DescriptionsSmall)
	scSmall.SetLabelStyle(kit.Style{Text: render.Hex("#000000")})

	scLarge := wire(kit.NewDescriptions(styleItems...))
	scLarge.SetTitle("User Info")
	scLarge.SetBordered(true)
	scLarge.SetSize(kit.DescriptionsLarge)
	scLarge.SetLabelStyle(kit.Style{Text: render.Hex("#A294F9")})
	scLarge.SetStyle(kit.Style{
		Border:      render.Hex("#CDC1FF"),
		Radius:      8,
		ForceRadius: true,
	})
	scCol := primitive.Column(scSmall.Node(), scLarge.Node())
	scCol.Gap = 16
	scCol.CrossAlign = core.CrossStretch
	secStyle := demoSection(face, th, "自定义语义结构的样式和类",
		"style-class.tsx：浅 LabelStyle / root Style（P1 深度 classNames/styles 函数形态）。",
		scCol)

	// ---------- block.tsx ----------
	block := wire(kit.NewDescriptions(
		kit.DescriptionsItem{Label: "UserName", Children: "Zhou Maomao"},
		kit.DescriptionsItem{Label: "Live", Children: "Hangzhou, Zhejiang", SpanFilled: true},
		kit.DescriptionsItem{Label: "Remark", Children: "empty", SpanFilled: true},
		kit.DescriptionsItem{Label: "Address", Span: 1, Children: "No. 18, Wantang Road, Xihu District, Hangzhou, Zhejiang, China"},
	))
	block.SetTitle("User Info")
	block.SetBordered(true)
	secBlock := demoSection(face, th, "整行",
		"block.tsx：span=filled 铺满当前行剩余列。",
		block.Node())

	// Lifecycle (#9)
	life := wire(kit.NewDescriptions(
		kit.DescriptionsItem{Label: "Field A", Children: "value a"},
		kit.DescriptionsItem{Label: "Field B", Children: "value b"},
	))
	life.SetTitle("Lifecycle")
	life.SetBordered(true)
	life.SetSize(kit.DescriptionsSmall)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Items/Title → Bordered → Size；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeDescriptions, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok && d != nil {
			if d.Base().Key == "descriptions-skin-demo" {
				d.BorderWidth = 2
				d.BorderColor = render.Hex("#1677FF")
			}
			if p := baseSkin.Painter(kit.TypeDescriptions); p != nil {
				p(pc, d)
				return
			}
			primitive.PaintDecorated(pc, d)
			return
		}
		if p := baseSkin.Painter(kit.TypeDescriptions); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinD := wire(kit.NewDescriptions(
		kit.DescriptionsItem{Label: "Skin", Children: "TypeID=kit.Descriptions"},
	))
	skinD.SetTitle("Skin")
	skinNode := skinD.Node()
	if d, ok := skinD.ChromeNode().(*primitive.Decorated); ok && d != nil {
		d.Base().Key = "descriptions-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root.SkinType=kit.Descriptions 已注册。Key=descriptions-skin-demo → 蓝边框 Override。",
		skinNode)

	page := primitive.Column(
		secBasic, secBorder, secSize, secResp,
		secVert, secVertB, secStyle, secBlock, secLife, secSkin,
	)
	page.Gap = 24
	page.CrossAlign = core.CrossStretch
	page.MainAlign = core.MainStart

	c.addPage("descriptions", "Descriptions", page)
}
