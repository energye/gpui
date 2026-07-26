//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTooltip() {
	// Tooltip — Ant Design demos (docs/antd/tooltip.md §6.8 P0)
	// https://ant.design/components/tooltip
	// P0: basic / smooth-transition / placement / arrow / shift / colorful / disabled / wrap-custom
	// P1 (not shown): unique ConfigProvider, semantic classNames/styles, zoom motion, debug

	face, th := c.face, c.theme
	status := c.status

	track := func(tt *kit.Tooltip) *kit.Tooltip {
		tt.SetFace(face)
		if th != nil {
			tt.SetTheme(th)
		}
		// Gallery: zero enter delay so hover feels responsive.
		tt.SetMouseEnterDelay(0)
		tt.SetMouseLeaveDelay(0.05)
		c.trackTicker(tt)
		return tt
	}

	// ── basic.tsx ─────────────────────────────────────────────
	basic := track(kit.NewTooltip("prompt text"))
	basic.SetTriggerNode(kit.NewText("Tooltip will show on mouse enter.").Node())
	secBasic := demoSection(face, th, "基本",
		"最简单的用法，鼠标移入时显示提示。",
		basic.Node())

	// ── smooth-transition.tsx (without unique — P1) ───────────
	mkSmoothBtn := func(pl kit.TooltipPlacement) core.Node {
		btn := c.trackBtn(kit.NewButton("Button"))
		btn.SetType(kit.ButtonPrimary)
		tt := track(kit.NewTooltip("Hello, Ant Design!"))
		tt.SetTriggerNode(btn.Node())
		tt.SetPlacement(pl)
		return tt.Node()
	}
	smoothCol := primitive.Column(
		spaceWrap(8, mkSmoothBtn(kit.TooltipTop), mkSmoothBtn(kit.TooltipTop)),
		spaceWrap(8, mkSmoothBtn(kit.TooltipBottom), mkSmoothBtn(kit.TooltipBottom)),
	)
	smoothCol.Gap = 8
	secSmooth := demoSection(face, th, "平滑过渡",
		"多 Tip 独立展示（ConfigProvider tooltip.unique 为 P1，本页不启用）。",
		smoothCol)

	// ── placement.tsx ─────────────────────────────────────────
	mkPlace := func(label string, pl kit.TooltipPlacement) core.Node {
		tt := track(kit.NewTooltip("prompt text"))
		tt.SetTriggerLabel(label)
		tt.SetPlacement(pl)
		return tt.Node()
	}
	secPlaceBody := primitive.Column(
		spaceWrap(8,
			mkPlace("TL", kit.TooltipTopLeft),
			mkPlace("Top", kit.TooltipTop),
			mkPlace("TR", kit.TooltipTopRight),
		),
		spaceWrap(8,
			mkPlace("LT", kit.TooltipLeftTop),
			mkPlace("Left", kit.TooltipLeft),
			mkPlace("LB", kit.TooltipLeftBottom),
			mkPlace("RT", kit.TooltipRightTop),
			mkPlace("Right", kit.TooltipRight),
			mkPlace("RB", kit.TooltipRightBottom),
		),
		spaceWrap(8,
			mkPlace("BL", kit.TooltipBottomLeft),
			mkPlace("Bottom", kit.TooltipBottom),
			mkPlace("BR", kit.TooltipBottomRight),
		),
	)
	secPlaceBody.Gap = 12
	secPlace := demoSection(face, th, "位置",
		"位置有 12 个方向。",
		secPlaceBody)

	// ── arrow.tsx ─────────────────────────────────────────────
	mkArrow := func(label string, pl kit.TooltipPlacement, show, center bool) core.Node {
		tt := track(kit.NewTooltip("prompt text"))
		tt.SetTriggerLabel(label)
		tt.SetPlacement(pl)
		tt.SetArrowConfig(show, center)
		return tt.Node()
	}
	secArrowBody := primitive.Column(
		kit.NewText("Show").Node(),
		spaceWrap(8,
			mkArrow("TL", kit.TooltipTopLeft, true, false),
			mkArrow("Top", kit.TooltipTop, true, false),
			mkArrow("TR", kit.TooltipTopRight, true, false),
		),
		kit.NewText("Hide").Node(),
		spaceWrap(8,
			mkArrow("Hide Top", kit.TooltipTop, false, false),
			mkArrow("Hide Bottom", kit.TooltipBottom, false, false),
		),
		kit.NewText("Center (pointAtCenter)").Node(),
		spaceWrap(8,
			mkArrow("TL", kit.TooltipTopLeft, true, true),
			mkArrow("Top", kit.TooltipTop, true, true),
			mkArrow("TR", kit.TooltipTopRight, true, true),
		),
	)
	secArrowBody.Gap = 12
	secArrow := demoSection(face, th, "箭头展示",
		"支持显示、隐藏与 pointAtCenter。",
		secArrowBody)

	// ── shift.tsx ─────────────────────────────────────────────
	shift := track(kit.NewTooltip("Thanks for using antd. Have a nice day !"))
	btnShift := c.trackBtn(kit.NewButton("Scroll The Window"))
	btnShift.SetType(kit.ButtonPrimary)
	shift.SetTriggerNode(btnShift.Node())
	shift.SetOpen(true)
	secShift := demoSection(face, th, "贴边偏移",
		"气泡被遮挡时自动调整位置（autoAdjustOverflow）。",
		shift.Node())

	// ── colorful.tsx ──────────────────────────────────────────
	presets := []string{"pink", "red", "yellow", "orange", "cyan", "green", "blue", "purple", "geekblue", "magenta", "volcano", "gold", "lime"}
	customs := []string{"#f50", "#2db7f5", "#87d068", "#108ee9"}
	presetKids := make([]core.Node, 0, len(presets))
	for _, p := range presets {
		col := p
		tt := track(kit.NewTooltip("prompt text"))
		tt.SetTriggerLabel(col)
		tt.SetColor(col)
		presetKids = append(presetKids, tt.Node())
	}
	customKids := make([]core.Node, 0, len(customs))
	for _, p := range customs {
		col := p
		tt := track(kit.NewTooltip("prompt text"))
		tt.SetTriggerLabel(col)
		tt.SetColor(col)
		customKids = append(customKids, tt.Node())
	}
	colorBody := primitive.Column(
		demoSection(face, th, "Presets", "", spaceWrap(8, presetKids...)),
		demoSection(face, th, "Custom", "", spaceWrap(8, customKids...)),
	)
	colorBody.Gap = 8
	secColor := demoSection(face, th, "多彩文字提示",
		"推荐使用预设色彩；也可自定义色值。",
		colorBody)

	// ── disabled.tsx ──────────────────────────────────────────
	disabledOn := true
	disHost := primitive.Column()
	disHost.CrossAlign = core.CrossStart
	var rebuildDis func()
	rebuildDis = func() {
		disHost.ClearChildren()
		title := ""
		lab := "Enable"
		if !disabledOn {
			title = "prompt text"
			lab = "Disable"
		}
		tt := track(kit.NewTooltip(title))
		btn := c.trackBtn(kit.NewButton(lab))
		btn.SetOnClick(func() {
			disabledOn = !disabledOn
			rebuildDis()
			if status != nil {
				if disabledOn {
					*status = "tooltip disabled (empty title)"
				} else {
					*status = "tooltip enabled"
				}
			}
		})
		tt.SetTriggerNode(btn.Node())
		disHost.AddChild(tt.Node())
		disHost.MarkNeedsLayout()
		disHost.MarkNeedsPaint()
	}
	rebuildDis()
	secDis := demoSection(face, th, "禁用",
		"通过空 title 禁用提示（antd 官方语义）。",
		disHost)

	// ── wrap-custom-component.tsx ──────────────────────────────
	wrap := track(kit.NewTooltip("prompt text"))
	wrap.SetTriggerNode(kit.NewText("This text is inside a component with the necessary events exposed.").Node())
	secWrap := demoSection(face, th, "自定义子组件",
		"包装自定义触发节点（须可接收指针事件）。",
		wrap.Node())

	// Lifecycle (#9)
	life := track(kit.NewTooltip("Lifecycle (#9)"))
	life.SetTriggerLabel("Lifecycle")
	life.SetPlacement(kit.TooltipTop)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Title → TriggerLabel → Placement；ensureBuilt 懒构建 Wrap。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeTooltip, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok && d != nil {
			if d.Base().Key == "tooltip-skin-demo" {
				d.BorderWidth = 2
				d.BorderColor = render.Hex("#1677FF")
			}
			if p := baseSkin.Painter(kit.TypeTooltip); p != nil {
				p(pc, d)
				return
			}
			primitive.PaintDecorated(pc, d)
			return
		}
		if p := baseSkin.Painter(kit.TypeTooltip); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinT := track(kit.NewTooltip("panel.SkinType=kit.Tooltip"))
	skinT.SetTriggerLabel("Skin tooltip")
	skinNode := skinT.Node()
	if panel := skinT.Panel(); panel != nil {
		panel.Base().Key = "tooltip-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"panel.SkinType=kit.Tooltip 已注册；Key=tooltip-skin-demo → 蓝边框 Override。",
		skinNode)

	c.addPage("tooltip", "Tooltip", demoPage(face, "Tooltip",
		"简单的文字提示气泡框。P0 对齐 docs/antd/tooltip.md §6。",
		secBasic, secSmooth, secPlace, secArrow, secShift, secColor, secDis, secWrap, secLife, secSkin,
	))
}
