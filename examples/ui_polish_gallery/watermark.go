//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerWatermark() {
	// Watermark — docs/antd/watermark.md §6.8 P0
	// https://ant.design/components/watermark
	// demos: basic / multi-line / image / custom / portal (Modal & Drawer)
	//
	// P1 not shown: MutationObserver anti-delete, true HTTP image decode,
	// alternate clip pixel-perfect, ConfigProvider global, 官网逐像素.

	face, th := c.face, c.theme
	status := c.status

	wire := func(w *kit.Watermark) *kit.Watermark {
		w.SetFace(face)
		if th != nil {
			w.SetTheme(th)
		}
		return w
	}
	area := func(h float64) core.Node {
		b := primitive.NewDecorated(nil)
		b.Width = 480
		b.Height = h
		if th != nil {
			b.Background = th.Color(core.TokenColorBgContainer)
			b.BorderColor = th.Color(core.TokenColorBorderSecondary)
			b.BorderWidth = 1
		} else {
			b.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
			b.BorderColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.1}
			b.BorderWidth = 1
		}
		b.Radius = 6
		b.Hit = core.HitBlock
		return b
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		return f
	}

	// ---------- basic.tsx ----------
	basicBody := area(220)
	basic := wire(kit.NewWatermark(basicBody))
	basic.SetContent("Ant Design")
	secBasic := demoSection(face, th, "基本",
		"basic.tsx：content=\"Ant Design\"，默认 rotate=-22 / gap=100。",
		basic.Node())

	// ---------- multi-line.tsx ----------
	mlBody := area(220)
	ml := wire(kit.NewWatermark(mlBody))
	ml.SetContentLines(
		kit.WatermarkContentLine{Text: "Ant Design"},
		kit.WatermarkContentLine{
			Text:    "Happy Working",
			Font:    kit.WatermarkFont{FontSize: 12},
			HasFont: true,
		},
	)
	secML := demoSection(face, th, "多行水印",
		"multi-line.tsx：content 数组 + 行级 fontSize=12。",
		ml.Node())

	// ---------- image.tsx ----------
	imgBody := area(220)
	imgWM := wire(kit.NewWatermark(imgBody))
	imgWM.SetWidth(130)
	imgWM.SetHeight(30)
	imgWM.SetImage("gallery-wm-logo")
	// Host-decoded stand-in pixels (no HTTP in kit).
	const iw, ih = 32, 16
	pix := make([]byte, iw*ih*4)
	for y := 0; y < ih; y++ {
		for x := 0; x < iw; x++ {
			i := (y*iw + x) * 4
			// soft brand-ish glyph block (not solid primary fill as default text ink)
			pix[i+0] = 22
			pix[i+1] = 119
			pix[i+2] = 255
			pix[i+3] = 90
			if x > 4 && x < 28 && y > 3 && y < 12 {
				pix[i+3] = 140
			}
		}
	}
	imgWM.SetImagePixels(iw, ih, pix)
	// content fallback if image cleared
	imgWM.SetContent("Ant Design")
	secImg := demoSection(face, th, "图片水印",
		"image.tsx：width=130 height=30 + SetImagePixels（host 解码映射）。",
		imgWM.Node())

	// ---------- custom.tsx ----------
	para1 := kit.NewParagraph("The light-speed iteration of the digital world makes products more complex. However, human consciousness and attention resources are limited.")
	para1.SetFace(face)
	para1.SetEllipsisRows(6)
	para2 := kit.NewParagraph("Natural user cognition: about 80% of external information is obtained through visual channels.")
	para2.SetFace(face)
	para2.SetEllipsisRows(4)
	customInner := primitive.Column(para1.Node(), para2.Node())
	customInner.Gap = 8
	customInner.Padding = primitive.All(16)
	customBox := primitive.NewDecorated(customInner)
	customBox.Width = 420
	customBox.MinHeight = 200
	if th != nil {
		customBox.Background = th.Color(core.TokenColorBgContainer)
		customBox.BorderColor = th.Color(core.TokenColorBorderSecondary)
		customBox.BorderWidth = 1
	}
	customBox.Radius = 6

	custom := wire(kit.NewWatermark(customBox))
	custom.SetContent("Ant Design")
	custom.SetFontColor(render.RGBA{R: 0, G: 0, B: 0, A: 0.15})
	custom.SetFontSize(16)
	custom.SetZIndex(11)
	custom.SetRotate(-22)
	custom.SetGap(100, 100)

	// Live knobs (subset of custom.tsx form)
	rotLab := kit.NewText("Rotate")
	rotLab.SetFace(face)
	rotSl := kit.NewSlider(-22)
	if th != nil {
		rotSl.SetTheme(th)
	}
	rotSl.SetMin(-180)
	rotSl.SetMax(180)
	rotSl.SetOnChange(func(v float64) {
		custom.SetRotate(v)
		if status != nil {
			*status = "Watermark rotate"
		}
	})
	fsLab := kit.NewText("FontSize")
	fsLab.SetFace(face)
	fsSl := kit.NewSlider(16)
	if th != nil {
		fsSl.SetTheme(th)
	}
	fsSl.SetMin(1)
	fsSl.SetMax(40)
	fsSl.SetOnChange(func(v float64) {
		custom.SetFontSize(v)
	})
	gapLab := kit.NewText("Gap")
	gapLab.SetFace(face)
	gapSl := kit.NewSlider(100)
	if th != nil {
		gapSl.SetTheme(th)
	}
	gapSl.SetMin(20)
	gapSl.SetMax(200)
	gapSl.SetOnChange(func(v float64) {
		custom.SetGap(v, v)
	})

	formCol := primitive.Column(
		rotLab.Node(), rotSl.Node(),
		fsLab.Node(), fsSl.Node(),
		gapLab.Node(), gapSl.Node(),
	)
	formCol.Gap = 6
	formHost := primitive.NewDecorated(formCol)
	formHost.Width = 200
	formRow := primitive.Row(custom.Node(), formHost)
	formRow.Gap = 16
	formRow.CrossAlign = core.CrossStart
	secCustom := demoSection(face, th, "自定义配置",
		"custom.tsx：content + font/rotate/gap 可调（Slider 子集）。",
		formRow)

	// ---------- portal.tsx Modal & Drawer ----------
	modal := kit.NewModal("Modal")
	if th != nil {
		modal.SetTheme(th)
	}
	modal.SetDestroyOnHidden(true)
	drawer := kit.NewDrawer("Drawer")
	if th != nil {
		drawer.SetTheme(th)
	}
	drawer.SetDestroyOnHidden(true)
	drawer2 := kit.NewDrawer("Drawer (no inherit)")
	if th != nil {
		drawer2.SetTheme(th)
	}
	drawer2.SetDestroyOnHidden(true)

	mkPlaceholder := func() core.Node {
		b := primitive.NewDecorated(nil)
		b.Height = 180
		b.Width = 320
		b.Background = render.RGBA{R: 0.6, G: 0.6, B: 0.6, A: 0.2}
		b.CenterContent = true
		lab := kit.NewText("A mock height")
		lab.SetFace(face)
		b.AddChild(lab.Node())
		return b
	}

	// Shared mark config for inherit simulation
	portalCfg := wire(kit.NewWatermark(nil))
	portalCfg.SetContent("Ant Design")

	btnModal := c.trackBtn(kit.NewButton("Show in Modal"))
	btnModal.SetType(kit.ButtonPrimary)
	btnModal.SetOnClick(func() {
		// inherit=true → Wrap modal content with same mark config
		body := portalCfg.Wrap(mkPlaceholder())
		if face != nil {
			body.SetFace(face)
		}
		if th != nil {
			body.SetTheme(th)
		}
		modal.SetContent(body.Node())
		modal.SetOpen(true)
		if status != nil {
			*status = "Watermark · Modal (inherit)"
		}
	})
	btnDrawer := c.trackBtn(kit.NewButton("Show in Drawer"))
	btnDrawer.SetType(kit.ButtonPrimary)
	btnDrawer.SetOnClick(func() {
		body := portalCfg.Wrap(mkPlaceholder())
		if face != nil {
			body.SetFace(face)
		}
		drawer.SetContent(body.Node())
		drawer.SetOpen(true)
		if status != nil {
			*status = "Watermark · Drawer (inherit)"
		}
	})
	btnNo := c.trackBtn(kit.NewButton("Not Show in Drawer"))
	btnNo.SetType(kit.ButtonPrimary)
	btnNo.SetOnClick(func() {
		// inherit=false → plain content, no watermark
		drawer2.SetContent(mkPlaceholder())
		drawer2.SetOpen(true)
		if status != nil {
			*status = "Watermark · Drawer (inherit=false)"
		}
	})

	btnRow := primitive.Row(btnModal.Node(), btnDrawer.Node(), btnNo.Node())
	btnRow.Gap = 12
	btnRow.Wrap = true

	// Keep portals in tree
	portals := primitive.Column(modal.Node(), drawer.Node(), drawer2.Node())
	portals.Gap = 0

	secPortal := demoSection(face, th, "Modal 与 Drawer",
		"portal.tsx：inherit 时 Wrap 到 Modal/Drawer 内容；第三钮模拟 inherit=false。",
		col(btnRow, portals))

	// Lifecycle (#9)
	lifeBody := area(160)
	life := wire(kit.NewWatermark(lifeBody))
	life.SetContent("Lifecycle")
	life.SetRotate(-15)
	life.SetGap(80, 80)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Child/Content → Rotate → Gap；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6) — Root 是 watermarkHost（TypeID=kit.Watermark）；Override 证明挂接点存在。
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeWatermark, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeWatermark); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinBody := area(160)
	skinW := wire(kit.NewWatermark(skinBody))
	skinW.SetContent("TypeID=kit.Watermark")
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root TypeID=kit.Watermark 已注册；host 自带 clip Paint，默认委托子节点绘制。",
		skinW.Node())

	c.addPage("watermark", "Watermark",
		demoPage(face, "Watermark 水印",
			"给页面的某个区域加上水印。P0 对齐 docs/antd/watermark.md §6（basic / multi-line / image / custom / portal）。Also #9 lifecycle + #6 Skin.",
			secBasic, secML, secImg, secCustom, secPortal, secLife, secSkin))
}
