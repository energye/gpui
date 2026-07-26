//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerSpin() {
	// Spin — docs/antd/spin.md §6.8 P0
	// https://ant.design/components/spin
	// demos: basic / size / nested / tip / delay / custom-indicator / percent / style-class
	//
	// P1 not shown: fullscreen, semantic classNames/styles depth, ConfigProvider,
	// pixel-perfect 4-dot keyframes, debug/_semantic, 官网逐像素.

	face, th := c.face, c.theme

	wire := func(s *kit.Spin) *kit.Spin {
		if th != nil {
			s.SetTheme(th)
		}
		c.trackTicker(s)
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
		f := primitive.Row(kids...)
		f.Gap = 16
		f.CrossAlign = core.CrossCenter
		f.Wrap = true
		return f
	}
	// Nested placeholder content (tip.tsx contentStyle-ish).
	padBox := func() core.Node {
		b := primitive.NewDecorated(nil)
		b.Width, b.Height = 120, 80
		b.Padding = primitive.All(0)
		b.Radius = 4
		b.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.05}
		b.Hit = core.HitDefer
		return b
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewSpin(nil))
	secBasic := demoSection(face, th, "基本用法",
		"basic.tsx：独立 Spin，默认 spinning + medium 4-dot。",
		basic.Node())

	// ---------- size.tsx ----------
	szSM := wire(kit.NewSpin(nil))
	szSM.SetSize(kit.SpinSizeSmall)
	szMD := wire(kit.NewSpin(nil))
	szLG := wire(kit.NewSpin(nil))
	szLG.SetSize(kit.SpinSizeLarge)
	secSize := demoSection(face, th, "各种大小",
		"size.tsx：small / medium / large（DotSize 14 / 20 / 32）。",
		row(szSM.Node(), szMD.Node(), szLG.Node()))

	// ---------- nested.tsx ----------
	nestedSpin := wire(kit.NewSpin(nil))
	nestedBody := primitive.NewDecorated(nil)
	nestedBody.Width = 320
	nestedBody.MinHeight = 72
	nestedBody.Padding = primitive.All(12)
	nestedBody.Radius = 6
	nestedBody.Background = render.RGBA{R: 0.9, G: 0.95, B: 1, A: 1}
	if th != nil {
		nestedBody.Background = th.Color(core.TokenColorPrimaryBg)
		nestedBody.BorderColor = th.Color(core.TokenColorPrimaryBorder)
		nestedBody.BorderWidth = 1
	}
	title := kit.NewText("Alert message title")
	title.SetFace(face)
	desc := kit.NewText("Further details about the context of this alert.")
	desc.SetFace(face)
	desc.SetStyle(kit.Style{Text: render.RGBA{R: 0, G: 0, B: 0, A: 0.65}})
	alertCol := primitive.Column(title.Node(), desc.Node())
	alertCol.Gap = 4
	nestedBody.AddChild(alertCol)
	nestedSpin.SetContent(nestedBody)
	nestedSpin.SetSpinning(false)

	sw := kit.NewSwitch()
	if th != nil {
		sw.Theme = th
	}
	sw.SetOnChange(func(on bool) {
		nestedSpin.SetSpinning(on)
		if c.status != nil {
			if on {
				*c.status = "Spin nested: loading"
			} else {
				*c.status = "Spin nested: idle"
			}
		}
	})
	loadLab := kit.NewText("Loading state：")
	loadLab.SetFace(face)
	swRow := primitive.Row(loadLab.Node(), sw.Node())
	swRow.Gap = 8
	swRow.CrossAlign = core.CrossCenter
	secNested := demoSection(face, th, "卡片加载中",
		"nested.tsx：Switch 切换 spinning；children 始终在树，spinning 时遮罩挡点。",
		col(nestedSpin.Node(), swRow))

	// ---------- tip.tsx (description) ----------
	tipSM := wire(kit.NewSpin(padBox()))
	tipSM.SetSize(kit.SpinSizeSmall)
	tipSM.SetDescription("Loading")
	tipMD := wire(kit.NewSpin(padBox()))
	tipMD.SetDescription("Loading")
	tipLG := wire(kit.NewSpin(padBox()))
	tipLG.SetSize(kit.SpinSizeLarge)
	tipLG.SetDescription("Loading")
	tipAlert := wire(kit.NewSpin(nil))
	tipBody := primitive.NewDecorated(nil)
	tipBody.Width = 360
	tipBody.MinHeight = 64
	tipBody.Padding = primitive.All(12)
	tipBody.Radius = 6
	if th != nil {
		tipBody.Background = th.Color(core.TokenColorPrimaryBg)
		tipBody.BorderColor = th.Color(core.TokenColorPrimaryBorder)
		tipBody.BorderWidth = 1
	}
	t1 := kit.NewText("Alert message title")
	t1.SetFace(face)
	t2 := kit.NewText("Further details about the context of this alert.")
	t2.SetFace(face)
	tc := primitive.Column(t1.Node(), t2.Node())
	tc.Gap = 4
	tipBody.AddChild(tc)
	tipAlert.SetContent(tipBody)
	tipAlert.SetDescription("Loading...")
	secTip := demoSection(face, th, "自定义描述文案",
		"tip.tsx：description 与指示同显（small/medium/large + Alert）。",
		col(row(tipSM.Node(), tipMD.Node(), tipLG.Node()), tipAlert.Node()))

	// ---------- delayAndDebounce.tsx ----------
	delaySpin := wire(kit.NewSpin(nil))
	delayBody := primitive.NewDecorated(nil)
	delayBody.Width = 320
	delayBody.MinHeight = 72
	delayBody.Padding = primitive.All(12)
	delayBody.Radius = 6
	if th != nil {
		delayBody.Background = th.Color(core.TokenColorPrimaryBg)
		delayBody.BorderWidth = 1
		delayBody.BorderColor = th.Color(core.TokenColorPrimaryBorder)
	}
	d1 := kit.NewText("Alert message title")
	d1.SetFace(face)
	d2 := kit.NewText("Further details about the context of this alert.")
	d2.SetFace(face)
	dc := primitive.Column(d1.Node(), d2.Node())
	dc.Gap = 4
	delayBody.AddChild(dc)
	delaySpin.SetContent(delayBody)
	delaySpin.SetDelay(500)
	delaySpin.SetSpinning(false)

	delaySW := kit.NewSwitch()
	if th != nil {
		delaySW.Theme = th
	}
	delaySW.SetOnChange(func(on bool) {
		delaySpin.SetSpinning(on)
		if c.status != nil {
			if on {
				*c.status = "Spin delay: waiting/showing"
			} else {
				*c.status = "Spin delay: idle"
			}
		}
	})
	delayLab := kit.NewText("Loading state：")
	delayLab.SetFace(face)
	delayRow := primitive.Row(delayLab.Node(), delaySW.Node())
	delayRow.Gap = 8
	delayRow.CrossAlign = core.CrossCenter
	secDelay := demoSection(face, th, "延迟",
		"delayAndDebounce.tsx：delay=500ms，开启后半秒内不闪指示。",
		col(delaySpin.Node(), delayRow))

	// ---------- custom-indicator.tsx ----------
	mkIcon := func(px float64) core.Node {
		ic := kit.NewIcon("loading")
		ic.SetSize(px)
		ic.SetSpin(true)
		if th != nil {
			ic.SetColor(th.Color(core.TokenColorPrimary))
		}
		c.trackTicker(ic)
		return ic.Node()
	}
	ciSM := wire(kit.NewSpin(nil))
	ciSM.SetSize(kit.SpinSizeSmall)
	ciSM.SetIndicator(mkIcon(14))
	ciMD := wire(kit.NewSpin(nil))
	ciMD.SetIndicator(mkIcon(20))
	ciLG := wire(kit.NewSpin(nil))
	ciLG.SetSize(kit.SpinSizeLarge)
	ciLG.SetIndicator(mkIcon(32))
	ciXL := wire(kit.NewSpin(nil))
	ciXL.SetIndicator(mkIcon(48))
	secCustom := demoSection(face, th, "自定义指示符",
		"custom-indicator.tsx：Icon loading + spin 作为 indicator。",
		row(ciSM.Node(), ciMD.Node(), ciLG.Node(), ciXL.Node()))

	// ---------- percent.tsx ----------
	pctAuto := false
	pctVal := 0.0
	pctSM := wire(kit.NewSpin(nil))
	pctSM.SetSize(kit.SpinSizeSmall)
	pctSM.SetPercent(0)
	pctMD := wire(kit.NewSpin(nil))
	pctMD.SetPercent(0)
	pctLG := wire(kit.NewSpin(nil))
	pctLG.SetSize(kit.SpinSizeLarge)
	pctLG.SetPercent(0)

	// Drive percent via a small ticker wrapper.
	pctDriver := &spinPercentDriver{
		spins: []*kit.Spin{pctSM, pctMD, pctLG},
		auto:  &pctAuto,
		val:   &pctVal,
	}
	c.trackTicker(pctDriver)

	autoSW := kit.NewSwitch()
	if th != nil {
		autoSW.Theme = th
	}
	autoSW.SetOnChange(func(on bool) {
		pctAuto = on
		pctVal = -50
		for _, sp := range []*kit.Spin{pctSM, pctMD, pctLG} {
			if on {
				sp.SetPercentAuto()
			} else {
				sp.SetPercent(pctVal)
			}
		}
	})
	secPercent := demoSection(face, th, "进度",
		"percent.tsx：数值进度环 + Auto 模拟爬升（usePercent 桶）。",
		row(autoSW.Node(), pctSM.Node(), pctMD.Node(), pctLG.Node()))

	// ---------- style-class.tsx ----------
	sc1 := wire(kit.NewSpin(nil))
	sc1.SetStyles(kit.SpinStyles{
		Indicator: kit.Style{Text: render.Hex("#00d4ff")},
	})
	sc1.SetClassNames(kit.SpinClassNames{Root: "demo-spin-root"})
	sc2 := wire(kit.NewSpin(nil))
	sc2.SetSize(kit.SpinSizeSmall)
	sc2.SetStyles(kit.SpinStyles{
		Indicator: kit.Style{Text: render.Hex("#722ed1")},
	})
	secStyle := demoSection(face, th, "自定义语义结构的样式和类",
		"style-class.tsx：浅层 styles.indicator 色 + classNames.root（深度/函数式 P1）。",
		row(sc1.Node(), sc2.Node()))

	// Lifecycle (#9)
	life := wire(kit.NewSpin(nil))
	life.SetSize(kit.SpinSizeLarge)
	life.SetTip("building…")
	life.SetSpinning(true)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (Size/Tip) then chrome (Spinning)；ensureBuilt 懒构建 host。",
		life.Node())

	// Skin (#6) — host embeds RepaintBoundary: TypeID registered; do not intercept host Paint.
	skinS := wire(kit.NewSpin(nil))
	skinS.SetSize(kit.SpinSizeMedium)
	skinS.SetTip("TypeID=kit.Spin")
	skinS.SetTheme(th)
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"spinHost TypeID=kit.Spin 已注册；host 内嵌 RepaintBoundary，Paint 不走 Skin 拦截（与 Icon 同）。",
		skinS.Node())

	c.addPage("spin", "Spin", demoPage(face,
		"Spin 加载中",
		"用于页面和区块的加载中状态。P0 对齐 docs/antd/spin.md §6；fullscreen 等为 P1。Also #9 lifecycle + #6 Skin.",
		secBasic, secSize, secNested, secTip, secDelay, secCustom, secPercent, secStyle, secLife, secSkin,
	))
}

// spinPercentDriver advances percent demo values (mirrors percent.tsx timer).
type spinPercentDriver struct {
	spins []*kit.Spin
	auto  *bool
	val   *float64
	acc   float64
}

func (d *spinPercentDriver) AttachTicker(t *core.Tree) {
	if t != nil {
		t.BindTicker(d, true)
	}
}

func (d *spinPercentDriver) Tick(dt float64) bool {
	if d == nil || d.auto == nil || d.val == nil {
		return false
	}
	if *d.auto {
		// auto mode handled inside each Spin ticker
		return true
	}
	d.acc += dt
	if d.acc < 0.1 {
		return true
	}
	d.acc = 0
	*d.val += 5
	if *d.val > 150 {
		*d.val = -50
	}
	for _, sp := range d.spins {
		if sp != nil {
			sp.SetPercent(*d.val)
		}
	}
	return true
}
