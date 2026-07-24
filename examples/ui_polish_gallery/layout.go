//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerLayout() {
	// Layout — docs/antd/layout.md §6.8 P0
	// https://ant.design/components/layout
	// demos: basic / top / top-side / top-side-2 / side / custom-trigger /
	//        collapsible-overlay / responsive
	// P1 deferred: fixed / fixed-sider

	face, th := c.face, c.theme

	// Colored region label (antd basic.tsx style blocks).
	region := func(label string, bg render.RGBA, h float64) core.Node {
		tx := kit.NewText(label)
		tx.SetFace(face)
		tx.SetStyle(kit.Style{Text: render.RGBA{R: 1, G: 1, B: 1, A: 1}})
		d := primitive.NewDecorated(tx.Node())
		d.Background = bg
		d.ExpandWidth = true
		d.StretchChild = true
		d.SetCenterContent(true)
		if h > 0 {
			d.Height = h
			d.MinHeight = h
		} else {
			d.MinHeight = 48
		}
		return d
	}
	hdrBlue := render.Hex("#4096ff")
	cntBlue := render.Hex("#0958d9")
	sidBlue := render.Hex("#1677ff")
	ftrBlue := render.Hex("#4096ff")

	playground := func(body core.Node, h float64) core.Node {
		d := primitive.NewDecorated(body)
		d.ExpandWidth = true
		d.StretchChild = true
		d.Radius = 8
		d.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		if h > 0 {
			d.Height = h
			d.MinHeight = h
		}
		return d
	}

	// ---------- basic.tsx (4 variants) ----------
	// 1) H-C-F
	basic1 := kit.NewLayout(
		func() core.Node {
			h := kit.NewHeader(region("Header", hdrBlue, 0))
			h.SetBackground(hdrBlue)
			h.SetPaddingInsets(primitive.EdgeInsets{})
			return h.Node()
		}(),
		func() core.Node {
			ct := kit.NewContent(region("Content", cntBlue, 80))
			ct.SetBackground(cntBlue)
			return ct.Node()
		}(),
		func() core.Node {
			f := kit.NewFooter(region("Footer", ftrBlue, 0))
			f.SetBackground(ftrBlue)
			f.SetPaddingInsets(primitive.Symmetric(0, 8))
			return f.Node()
		}(),
	)
	// 2) H-(S|C)-F
	basic2Sider := kit.NewSider(region("Sider", sidBlue, 80))
	basic2Sider.SetWidth(100)
	basic2Sider.SetBackground(sidBlue)
	basic2 := kit.NewLayout(
		func() core.Node {
			h := kit.NewHeader(region("Header", hdrBlue, 0))
			h.SetBackground(hdrBlue)
			h.SetPaddingInsets(primitive.EdgeInsets{})
			return h.Node()
		}(),
		kit.NewLayout(
			basic2Sider.Node(),
			func() core.Node {
				ct := kit.NewContent(region("Content", cntBlue, 80))
				ct.SetBackground(cntBlue)
				return ct.Node()
			}(),
		).Node(),
		func() core.Node {
			f := kit.NewFooter(region("Footer", ftrBlue, 0))
			f.SetBackground(ftrBlue)
			f.SetPaddingInsets(primitive.Symmetric(0, 8))
			return f.Node()
		}(),
	)
	// 3) H-(C|S)-F
	basic3Sider := kit.NewSider(region("Sider", sidBlue, 80))
	basic3Sider.SetWidth(100)
	basic3Sider.SetBackground(sidBlue)
	basic3 := kit.NewLayout(
		func() core.Node {
			h := kit.NewHeader(region("Header", hdrBlue, 0))
			h.SetBackground(hdrBlue)
			h.SetPaddingInsets(primitive.EdgeInsets{})
			return h.Node()
		}(),
		kit.NewLayout(
			func() core.Node {
				ct := kit.NewContent(region("Content", cntBlue, 80))
				ct.SetBackground(cntBlue)
				return ct.Node()
			}(),
			basic3Sider.Node(),
		).Node(),
		func() core.Node {
			f := kit.NewFooter(region("Footer", ftrBlue, 0))
			f.SetBackground(ftrBlue)
			f.SetPaddingInsets(primitive.Symmetric(0, 8))
			return f.Node()
		}(),
	)
	// 4) S|(H-C-F)
	basic4Sider := kit.NewSider(region("Sider", sidBlue, 0))
	basic4Sider.SetWidth(100)
	basic4Sider.SetBackground(sidBlue)
	basic4 := kit.NewLayout(
		basic4Sider.Node(),
		kit.NewLayout(
			func() core.Node {
				h := kit.NewHeader(region("Header", hdrBlue, 0))
				h.SetBackground(hdrBlue)
				h.SetPaddingInsets(primitive.EdgeInsets{})
				return h.Node()
			}(),
			func() core.Node {
				ct := kit.NewContent(region("Content", cntBlue, 48))
				ct.SetBackground(cntBlue)
				return ct.Node()
			}(),
			func() core.Node {
				f := kit.NewFooter(region("Footer", ftrBlue, 0))
				f.SetBackground(ftrBlue)
				f.SetPaddingInsets(primitive.Symmetric(0, 8))
				return f.Node()
			}(),
		).Node(),
	)
	basicRow := kit.NewFlex(
		playground(basic1.Node(), 200),
		playground(basic2.Node(), 200),
		playground(basic3.Node(), 200),
		playground(basic4.Node(), 200),
	)
	basicRow.SetGapSize(kit.FlexGapSmall)
	basicRow.SetWrap(true)
	// equal width via Flexible hosts already inside playground ExpandWidth
	secBasic := demoSection(face, th, "基本结构",
		"antd basic.tsx：四种经典嵌套（H-C-F / H-S-C-F / H-C-S-F / S-HCF）。",
		basicRow.Node())

	// ---------- top.tsx ----------
	top := kit.NewLayout(
		kit.NewHeader(region("Header · nav", DefaultDark, 0)).Node(),
		func() core.Node {
			ct := kit.NewContent(region("Content", render.Hex("#ffffff"), 120))
			ct.SetBackground(render.Hex("#ffffff"))
			ct.SetPaddingInsets(primitive.All(16))
			ct.SetMinHeight(120)
			// recolor text for light bg
			tx := kit.NewText("Content")
			tx.SetFace(face)
			ct.SetChildren(tx.Node())
			return ct.Node()
		}(),
		kit.NewFooter(func() core.Node {
			tx := kit.NewText("Ant Design © Created by Ant UED")
			tx.SetFace(face)
			return tx.Node()
		}()).Node(),
	)
	secTop := demoSection(face, th, "上中下布局",
		"antd top.tsx：Header + Content + Footer。",
		playground(top.Node(), 260))

	// ---------- top-side.tsx ----------
	topSideInner := kit.NewLayout(
		func() core.Node {
			s := kit.NewSider(region("Sider menu", render.Hex("#ffffff"), 160))
			s.SetSiderTheme(kit.SiderThemeLight)
			s.SetWidth(160)
			tx := kit.NewText("subnav")
			tx.SetFace(face)
			s.SetChildren(tx.Node())
			return s.Node()
		}(),
		func() core.Node {
			ct := kit.NewContent()
			tx := kit.NewText("Content")
			tx.SetFace(face)
			ct.SetChildren(tx.Node())
			ct.SetMinHeight(160)
			ct.SetPaddingInsets(primitive.All(16))
			return ct.Node()
		}(),
	)
	topSide := kit.NewLayout(
		kit.NewHeader(region("Header", DefaultDark, 0)).Node(),
		topSideInner.Node(),
		kit.NewFooter(func() core.Node {
			tx := kit.NewText("Footer")
			tx.SetFace(face)
			return tx.Node()
		}()).Node(),
	)
	secTopSide := demoSection(face, th, "顶部-侧边布局",
		"antd top-side.tsx：顶栏 + 内嵌浅色 Sider + Content。",
		playground(topSide.Node(), 300))

	// ---------- top-side-2.tsx ----------
	topSide2 := kit.NewLayout(
		kit.NewHeader(region("Header", DefaultDark, 0)).Node(),
		kit.NewLayout(
			func() core.Node {
				s := kit.NewSider()
				s.SetSiderTheme(kit.SiderThemeLight)
				s.SetWidth(160)
				tx := kit.NewText("Sider")
				tx.SetFace(face)
				s.SetChildren(tx.Node())
				return s.Node()
			}(),
			func() core.Node {
				inner := kit.NewLayout(
					func() core.Node {
						ct := kit.NewContent()
						tx := kit.NewText("Content")
						tx.SetFace(face)
						ct.SetChildren(tx.Node())
						ct.SetBackground(render.Hex("#ffffff"))
						ct.SetMinHeight(160)
						ct.SetPaddingInsets(primitive.All(16))
						return ct.Node()
					}(),
				)
				return inner.Node()
			}(),
		).Node(),
	)
	secTopSide2 := demoSection(face, th, "顶部-侧边布局-通栏",
		"antd top-side-2.tsx：顶栏通栏，侧栏贴左。",
		playground(topSide2.Node(), 280))

	// ---------- side.tsx (collapsible) ----------
	sideSider := kit.NewSider()
	sideSider.SetCollapsible(true)
	sideSider.SetChildren(func() core.Node {
		tx := kit.NewText("nav 1\nnav 2\nnav 3")
		tx.SetFace(face)
		tx.SetStyle(kit.Style{Text: render.RGBA{R: 1, G: 1, B: 1, A: 0.85}})
		return tx.Node()
	}())
	sideState := kit.NewText("collapsed=false")
	sideState.SetFace(face)
	sideSider.SetOnCollapse(func(collapsed bool, typ kit.CollapseType) {
		sideState.SetValue(fmt.Sprintf("collapsed=%v (%v)", collapsed, typ))
	})
	side := kit.NewLayout(
		sideSider.Node(),
		kit.NewLayout(
			func() core.Node {
				h := kit.NewHeader()
				h.SetBackground(render.Hex("#ffffff"))
				h.SetPaddingInsets(primitive.EdgeInsets{Left: 16})
				tx := kit.NewText("Header")
				tx.SetFace(face)
				h.SetChildren(tx.Node())
				return h.Node()
			}(),
			func() core.Node {
				ct := kit.NewContent()
				ct.SetBackground(render.Hex("#ffffff"))
				ct.SetMinHeight(160)
				ct.SetPaddingInsets(primitive.All(16))
				ct.SetChildren(sideState.Node())
				return ct.Node()
			}(),
			kit.NewFooter(func() core.Node {
				tx := kit.NewText("Footer")
				tx.SetFace(face)
				return tx.Node()
			}()).Node(),
		).Node(),
	)
	secSide := demoSection(face, th, "侧边布局",
		"antd side.tsx：可折叠 Sider（底 trigger）+ 主栏。",
		playground(side.Node(), 320))

	// ---------- custom-trigger.tsx ----------
	ctSider := kit.NewSider()
	ctSider.SetCollapsible(true)
	ctSider.SetHideTrigger()
	ctSider.SetChildren(func() core.Node {
		tx := kit.NewText("Menu")
		tx.SetFace(face)
		tx.SetStyle(kit.Style{Text: render.RGBA{R: 1, G: 1, B: 1, A: 0.85}})
		return tx.Node()
	}())
	ctCollapsed := false
	ctBtn := kit.NewButton("Toggle")
	ctBtn.SetFace(face)
	ctBtn.SetType(kit.ButtonText)
	ctLabel := kit.NewText("collapsed=false")
	ctLabel.SetFace(face)
	ctBtn.SetOnClick(func() {
		ctCollapsed = !ctCollapsed
		ctSider.SetCollapsed(ctCollapsed)
		ctLabel.SetValue(fmt.Sprintf("collapsed=%v", ctCollapsed))
	})
	c.trackBtn(ctBtn)
	custom := kit.NewLayout(
		ctSider.Node(),
		kit.NewLayout(
			func() core.Node {
				h := kit.NewHeader()
				h.SetBackground(render.Hex("#ffffff"))
				h.SetPaddingInsets(primitive.EdgeInsets{})
				h.SetChildren(ctBtn.Node())
				return h.Node()
			}(),
			func() core.Node {
				ct := kit.NewContent()
				ct.SetBackground(render.Hex("#ffffff"))
				ct.SetMinHeight(140)
				ct.SetPaddingInsets(primitive.All(16))
				ct.SetChildren(ctLabel.Node())
				return ct.Node()
			}(),
		).Node(),
	)
	secCustom := demoSection(face, th, "自定义触发器",
		"antd custom-trigger.tsx：trigger=null，Header 按钮受控折叠。",
		playground(custom.Node(), 280))

	// ---------- collapsible-overlay.tsx ----------
	ovSider := kit.NewSider()
	ovSider.SetCollapsible(true)
	ovSider.SetCollapsedWidth(0)
	ovSider.SetOverlay(true)
	ovSider.SetChildren(func() core.Node {
		tx := kit.NewText("Overlay\nnav")
		tx.SetFace(face)
		tx.SetStyle(kit.Style{Text: render.RGBA{R: 1, G: 1, B: 1, A: 0.85}})
		return tx.Node()
	}())
	ovLabel := kit.NewText("Content keeps full width when sider overlays.")
	ovLabel.SetFace(face)
	ovSider.SetOnCollapse(func(collapsed bool, _ kit.CollapseType) {
		if collapsed {
			ovLabel.SetValue("Sider collapsed (width 0); content full width.")
		} else {
			ovLabel.SetValue("Content keeps full width when sider overlays.")
		}
	})
	overlay := kit.NewLayout(
		ovSider.Node(),
		kit.NewLayout(
			func() core.Node {
				h := kit.NewHeader()
				h.SetBackground(render.Hex("#ffffff"))
				h.SetPaddingInsets(primitive.EdgeInsets{})
				return h.Node()
			}(),
			func() core.Node {
				ct := kit.NewContent()
				ct.SetBackground(render.Hex("#ffffff"))
				ct.SetMinHeight(140)
				ct.SetPaddingInsets(primitive.All(16))
				ct.SetChildren(ovLabel.Node())
				return ct.Node()
			}(),
			kit.NewFooter(func() core.Node {
				tx := kit.NewText("Footer")
				tx.SetFace(face)
				return tx.Node()
			}()).Node(),
		).Node(),
	)
	secOverlay := demoSection(face, th, "折叠覆盖布局",
		"antd collapsible-overlay.tsx：collapsedWidth=0 + Overlay，内容不被挤压。",
		playground(overlay.Node(), 300))

	// ---------- responsive.tsx ----------
	rpSider := kit.NewSider()
	rpSider.SetBreakpoint(kit.LayoutBreakpointLG)
	rpSider.SetCollapsedWidth(0)
	// Gallery viewport ~ content width; inject a mid size for demo readability.
	rpSider.SetViewportWidth(1200)
	rpLabel := kit.NewText("breakpoint=lg · viewport=1200 (not broken)")
	rpLabel.SetFace(face)
	rpSider.SetOnBreakpoint(func(broken bool) {
		rpLabel.SetValue(fmt.Sprintf("onBreakpoint broken=%v", broken))
	})
	rpSider.SetOnCollapse(func(collapsed bool, typ kit.CollapseType) {
		rpLabel.SetValue(fmt.Sprintf("collapsed=%v type=%v", collapsed, typ))
	})
	rpSider.SetChildren(func() core.Node {
		tx := kit.NewText("nav")
		tx.SetFace(face)
		tx.SetStyle(kit.Style{Text: render.RGBA{R: 1, G: 1, B: 1, A: 0.85}})
		return tx.Node()
	}())
	// Buttons to simulate viewport change
	rpNarrow := kit.NewButton("Viewport 800")
	rpNarrow.SetFace(face)
	rpNarrow.SetOnClick(func() {
		rpSider.SetViewportWidth(800)
	})
	c.trackBtn(rpNarrow)
	rpWide := kit.NewButton("Viewport 1200")
	rpWide.SetFace(face)
	rpWide.SetOnClick(func() {
		rpSider.SetViewportWidth(1200)
	})
	c.trackBtn(rpWide)
	rpBtns := kit.NewFlex(rpNarrow.Node(), rpWide.Node(), rpLabel.Node())
	rpBtns.SetGapSize(kit.FlexGapSmall)
	rpBtns.SetAlign(kit.FlexAlignCenter)
	responsive := kit.NewLayout(
		rpSider.Node(),
		kit.NewLayout(
			func() core.Node {
				h := kit.NewHeader()
				h.SetBackground(render.Hex("#ffffff"))
				return h.Node()
			}(),
			func() core.Node {
				ct := kit.NewContent()
				ct.SetBackground(render.Hex("#ffffff"))
				ct.SetMinHeight(140)
				ct.SetPaddingInsets(primitive.All(16))
				ct.SetChildren(rpBtns.Node())
				return ct.Node()
			}(),
			kit.NewFooter(func() core.Node {
				tx := kit.NewText("Footer")
				tx.SetFace(face)
				return tx.Node()
			}()).Node(),
		).Node(),
	)
	secResponsive := demoSection(face, th, "响应式布局",
		"antd responsive.tsx：breakpoint=lg + collapsedWidth=0；按钮模拟视口。",
		playground(responsive.Node(), 300))

	c.add("layout", "Layout", "Layout · 页面级布局",
		secBasic, secTop, secTopSide, secTopSide2,
		secSide, secCustom, secOverlay, secResponsive,
	)
}

// DefaultDark is antd Layout header/sider chrome (#001529).
var DefaultDark = render.Hex("#001529")
