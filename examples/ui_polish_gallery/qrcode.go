//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerQRCode() {
	// QRCode — docs/antd/qr-code.md §6.8 P0
	// https://ant.design/components/qr-code
	// demos: base / icon / status / customStatusRender / type / customSize /
	//        customColor / errorlevel / Popover(borderless)
	//
	// P1 not shown: download export, boostLevel, HTTP icon decode,
	// string[] value, ConfigProvider, style-class depth.

	face, th := c.face, c.theme
	status := c.status

	wire := func(q *kit.QRCode) *kit.QRCode {
		q.SetFace(face)
		if th != nil {
			q.SetTheme(th)
		}
		c.trackTicker(q)
		return q
	}

	// ---------- base.tsx ----------
	baseVal := "https://ant.design/"
	baseQR := wire(kit.NewQRCode(baseVal))
	baseIn := kit.NewInput("value")
	baseIn.SetFace(face)
	if th != nil {
		baseIn.SetTheme(th)
	}
	baseIn.SetValue(baseVal)
	baseIn.SetOnChange(func(v string) {
		if v == "" {
			v = "-"
		}
		baseQR.SetValue(v)
		if status != nil {
			*status = "QRCode · value=" + v
		}
	})
	baseCol := primitive.Column(baseQR.Node(), baseIn.Node())
	baseCol.Gap = 12
	baseCol.CrossAlign = core.CrossCenter
	secBase := demoSection(face, th, "基本使用",
		"base.tsx：value 生成矩阵；Input 改 value。",
		baseCol)

	// ---------- icon.tsx ----------
	iconQR := wire(kit.NewQRCode("https://ant.design/"))
	iconQR.SetErrorLevel(kit.QRErrorLevelH)
	// host/P0: IconNode stand-in (HTTP decode P1)
	logo := primitive.NewDecorated()
	logo.Width, logo.Height = 40, 40
	logo.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	logo.Radius = 4
	logo.CenterContent = true
	logoLab := kit.NewText("AD")
	logoLab.SetFace(face)
	logo.AddChild(logoLab.Node())
	iconQR.SetIconNode(logo)
	secIcon := demoSection(face, th, "带 Icon",
		"icon.tsx：errorLevel=H + 中心 IconNode（URL 解码 P1）。",
		iconQR.Node())

	// ---------- status.tsx ----------
	stLoad := wire(kit.NewQRCode("https://ant.design"))
	stLoad.SetStatus(kit.QRStatusLoading)
	stExp := wire(kit.NewQRCode("https://ant.design"))
	stExp.SetStatus(kit.QRStatusExpired)
	stExp.SetOnRefresh(func() {
		if status != nil {
			*status = "QRCode · refresh"
		}
		stExp.SetStatus(kit.QRStatusActive)
	})
	stScan := wire(kit.NewQRCode("https://ant.design"))
	stScan.SetStatus(kit.QRStatusScanned)
	stRow := primitive.Row(stLoad.Node(), stExp.Node(), stScan.Node())
	stRow.Gap = 16
	stRow.CrossAlign = core.CrossCenter
	secStatus := demoSection(face, th, "不同的状态",
		"status.tsx：loading / expired(+Refresh) / scanned。",
		stRow)

	// ---------- customStatusRender.tsx ----------
	customBody := func(info kit.QRStatusRenderInfo) core.Node {
		switch info.Status {
		case kit.QRStatusExpired:
			col := primitive.Column()
			col.Gap = 4
			col.CrossAlign = core.CrossCenter
			t := kit.NewText("✕ " + info.Expired)
			t.SetFace(face)
			col.AddChild(t.Node())
			if info.OnRefresh != nil {
				b := c.trackBtn(kit.NewButton(info.Refresh))
				b.SetType(kit.ButtonLink)
				b.SetOnClick(info.OnRefresh)
				col.AddChild(b.Node())
			}
			return col
		case kit.QRStatusLoading:
			t := kit.NewText("Loading...")
			t.SetFace(face)
			return t.Node()
		case kit.QRStatusScanned:
			t := kit.NewText("✓ " + info.Scanned)
			t.SetFace(face)
			return t.Node()
		default:
			return nil
		}
	}
	crLoad := wire(kit.NewQRCode("https://ant.design"))
	crLoad.SetStatusRender(customBody)
	crLoad.SetStatus(kit.QRStatusLoading)
	crExp := wire(kit.NewQRCode("https://ant.design"))
	crExp.SetOnRefresh(func() {
		if status != nil {
			*status = "QRCode · custom refresh"
		}
	})
	crExp.SetStatusRender(customBody)
	crExp.SetStatus(kit.QRStatusExpired)
	crScan := wire(kit.NewQRCode("https://ant.design"))
	crScan.SetStatusRender(customBody)
	crScan.SetStatus(kit.QRStatusScanned)
	crRow := primitive.Row(crLoad.Node(), crExp.Node(), crScan.Node())
	crRow.Gap = 16
	secCustomStatus := demoSection(face, th, "自定义状态渲染器",
		"customStatusRender.tsx：StatusRender 覆盖默认 cover 内容。",
		crRow)

	// ---------- type.tsx ----------
	tyCanvas := wire(kit.NewQRCode("https://ant.design/"))
	tyCanvas.SetType(kit.QRCodeTypeCanvas)
	tySVG := wire(kit.NewQRCode("https://ant.design/"))
	tySVG.SetType(kit.QRCodeTypeSVG)
	tyRow := primitive.Row(tyCanvas.Node(), tySVG.Node())
	tyRow.Gap = 16
	secType := demoSection(face, th, "自定义渲染类型",
		"type.tsx：canvas / svg 标签（桌面同为模块绘制）。",
		tyRow)

	// ---------- customSize.tsx ----------
	sizeLogo := primitive.NewDecorated()
	sizeLogo.Width, sizeLogo.Height = 40, 40
	sizeLogo.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	sizeLogo.Radius = 4
	sizeLogo.CenterContent = true
	sizeLogoLab := kit.NewText("AD")
	sizeLogoLab.SetFace(face)
	sizeLogo.AddChild(sizeLogoLab.Node())

	sizeQR := wire(kit.NewQRCode("https://ant.design/"))
	sizeQR.SetErrorLevel(kit.QRErrorLevelH)
	sizeQR.SetSize(160)
	sizeQR.SetIconSize(40)
	sizeQR.SetIconNode(sizeLogo)
	decBtn := c.trackBtn(kit.NewButton("Smaller"))
	incBtn := c.trackBtn(kit.NewButton("Larger"))
	decBtn.SetOnClick(func() {
		sz := sizeQR.Size() - 10
		if sz < 48 {
			sz = 48
		}
		sizeQR.SetSize(sz)
		sizeQR.SetIconSize(sz / 4)
		if status != nil {
			*status = "QRCode · size"
		}
	})
	incBtn.SetOnClick(func() {
		sz := sizeQR.Size() + 10
		if sz > 300 {
			sz = 300
		}
		sizeQR.SetSize(sz)
		sizeQR.SetIconSize(sz / 4)
		if status != nil {
			*status = "QRCode · size"
		}
	})
	sizeCtrl := primitive.Row(decBtn.Node(), incBtn.Node())
	sizeCtrl.Gap = 8
	sizeCol := primitive.Column(sizeCtrl, sizeQR.Node())
	sizeCol.Gap = 12
	secSize := demoSection(face, th, "自定义尺寸",
		"customSize.tsx：size 48–300；iconSize=size/4。",
		sizeCol)

	// ---------- customColor.tsx ----------
	col1 := wire(kit.NewQRCode("https://ant.design/"))
	if th != nil {
		col1.SetColor(th.Color(core.TokenColorSuccess))
	} else {
		col1.SetColor(render.Hex("#52C41A"))
	}
	col2 := wire(kit.NewQRCode("https://ant.design/"))
	if th != nil {
		col2.SetColor(th.Color(core.TokenColorPrimary))
		col2.SetBgColor(th.Color(core.TokenColorBgLayout))
	} else {
		col2.SetColor(render.Hex("#1677FF"))
		col2.SetBgColor(render.Hex("#F5F5F5"))
	}
	colRow := primitive.Row(col1.Node(), col2.Node())
	colRow.Gap = 16
	secColor := demoSection(face, th, "自定义颜色",
		"customColor.tsx：color / bgColor。",
		colRow)

	// ---------- errorlevel.tsx ----------
	lvQR := wire(kit.NewQRCode("https://gw.alipayobjects.com/zos/rmsportal/KDpgvguMpGfqaHPjicRK.svg"))
	lvQR.SetErrorLevel(kit.QRErrorLevelL)
	mkLv := func(label string, lv kit.QRErrorLevel) *kit.Button {
		b := c.trackBtn(kit.NewButton(label))
		b.SetOnClick(func() {
			lvQR.SetErrorLevel(lv)
			if status != nil {
				*status = "QRCode · errorLevel=" + label
			}
		})
		return b
	}
	lvRow := primitive.Row(
		mkLv("L", kit.QRErrorLevelL).Node(),
		mkLv("M", kit.QRErrorLevelM).Node(),
		mkLv("Q", kit.QRErrorLevelQ).Node(),
		mkLv("H", kit.QRErrorLevelH).Node(),
	)
	lvRow.Gap = 8
	lvCol := primitive.Column(lvQR.Node(), lvRow)
	lvCol.Gap = 12
	secLevel := demoSection(face, th, "纠错比例",
		"errorlevel.tsx：L / M / Q / H。",
		lvCol)

	// ---------- Popover.tsx (borderless) ----------
	popQR := wire(kit.NewQRCode("https://ant.design"))
	popQR.SetBordered(false)
	pop := kit.NewPopover("Hover me")
	pop.SetFace(face)
	if th != nil {
		pop.Theme = th
	}
	pop.SetContentNode(popQR.Node())
	secPop := demoSection(face, th, "高级用法 · Popover",
		"Popover.tsx：bordered=false 的 QR 作为 Popover content。",
		pop.Node())

	page := demoPage(face, "QRCode 二维码",
		"将文本编码为二维码。P0：value/size/color/bgColor/bordered/errorLevel/marginSize/icon/status/onRefresh/statusRender/type。\n"+
			"P1：下载导出、boostLevel、HTTP icon、string[] value、ConfigProvider、styles 函数形态。",
		secBase, secIcon, secStatus, secCustomStatus, secType, secSize, secColor, secLevel, secPop)
	c.addPage("qrcode", "QRCode", page)
}
