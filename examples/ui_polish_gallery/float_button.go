//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerFloatButton() {
	// FloatButton — Ant Design demos (docs/antd/float-button.md §6.8 P0)
	// https://ant.design/components/float-button
	trackFB := func(fb *kit.FloatButton) *kit.FloatButton {
		fb.SetFace(c.face)
		if fb.AriaLabel == "" {
			fb.SetAriaLabel("float-button")
		}
		if fb.Button() != nil {
			*c.buttons = append(*c.buttons, fb.Button())
		}
		return fb
	}
	// basic / type
	fbDefault := trackFB(kit.NewFloatButton())
	fbDefault.SetType(kit.ButtonDefault)
	fbDefault.SetIcon("plus")
	fbDefault.SetAriaLabel("default")
	fbPrimary := trackFB(kit.NewFloatButton())
	fbPrimary.SetType(kit.ButtonPrimary)
	fbPrimary.SetIcon("plus")
	fbPrimary.SetAriaLabel("primary")
	// shape
	fbCircle := trackFB(kit.NewFloatButton())
	fbCircle.SetShape(kit.FloatButtonCircle)
	fbCircle.SetIcon("plus")
	fbCircle.SetAriaLabel("circle")
	fbSquare := trackFB(kit.NewFloatButton())
	fbSquare.SetShape(kit.FloatButtonSquare)
	fbSquare.SetIcon("plus")
	fbSquare.SetAriaLabel("square")
	// content
	fbContent := trackFB(kit.NewFloatButton())
	fbContent.SetIcon("info")
	fbContent.SetContent("HELP")
	fbContent.SetShape(kit.FloatButtonSquare)
	fbContent.SetAriaLabel("help")
	// tooltip
	fbTip := trackFB(kit.NewFloatButton())
	fbTip.SetIcon("info")
	fbTip.SetTooltip("HELP TOOLTIP")
	fbTip.SetAriaLabel("with-tooltip")
	// disabled / loading
	fbDis := trackFB(kit.NewFloatButton())
	fbDis.SetIcon("plus")
	fbDis.SetDisabled(true)
	fbDis.SetAriaLabel("disabled")
	fbLoad := trackFB(kit.NewFloatButton())
	fbLoad.SetIcon("plus")
	fbLoad.SetLoading(true)
	fbLoad.SetAriaLabel("loading")
	*c.tickers = append(*c.tickers, fbLoad)
	// group (always open)
	gChild1 := trackFB(kit.NewFloatButton())
	gChild1.SetIcon("plus")
	gChild1.SetAriaLabel("g1")
	gChild2 := trackFB(kit.NewFloatButton())
	gChild2.SetIcon("info")
	gChild2.SetAriaLabel("g2")
	fbGroup := kit.NewFloatButtonGroup(gChild1, gChild2)
	fbGroup.SetFace(c.face)
	// menu mode click (antd group-menu.tsx) — fixed stage so overlay list is not clipped
	m1 := trackFB(kit.NewFloatButton())
	m1.SetIcon("plus")
	m1.SetAriaLabel("m1")
	m2 := trackFB(kit.NewFloatButton())
	m2.SetIcon("info")
	m2.SetAriaLabel("m2")
	fbMenu := kit.NewFloatButtonGroup(m1, m2)
	fbMenu.SetFace(c.face)
	fbMenu.SetTrigger(kit.FloatButtonTriggerClick)
	fbMenu.SetType(kit.ButtonPrimary)
	fbMenu.SetIcon("plus")
	fbMenu.SetDefaultOpen(true)
	if tr := fbMenu.TriggerButton(); tr != nil && tr.Button() != nil {
		*c.buttons = append(*c.buttons, tr.Button())
	}
	// controlled + placement left (antd controlled / placement)
	c1 := trackFB(kit.NewFloatButton())
	c1.SetIcon("plus")
	c1.SetAriaLabel("c1")
	fbCtrl := kit.NewFloatButtonGroup(c1)
	fbCtrl.SetFace(c.face)
	fbCtrl.SetTrigger(kit.FloatButtonTriggerClick)
	fbCtrl.SetPlacement(kit.FloatButtonLeft)
	fbCtrl.SetOpen(true)
	fbCtrl.SetIcon("info")
	if tr := fbCtrl.TriggerButton(); tr != nil && tr.Button() != nil {
		*c.buttons = append(*c.buttons, tr.Button())
	}
	// layout stage (bottom-right sample)
	corner := trackFB(kit.NewFloatButton())
	corner.SetIcon("plus")
	corner.SetType(kit.ButtonPrimary)
	corner.SetAriaLabel("corner")

	// Fixed playgrounds (antd placement.tsx style): menu expands inside the stage
	// without changing demo-section height; group anchors bottom/end like inset FAB.
	fabStage := func(body core.Node, w, h float64) core.Node {
		// pin body to bottom-end
		inner := primitive.Column(primitive.Spacer(), primitive.Row(primitive.Spacer(), body))
		inner.CrossAlign = core.CrossStretch
		stage := primitive.NewDecorated(inner)
		stage.Width = w
		stage.Height = h
		stage.Padding = primitive.All(12)
		stage.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		stage.Radius = 8
		stage.StretchChild = true
		return stage
	}
	// open menu (top): 2×40 + gap16 + gap16 + trigger40 ≈ 152 → stage 180
	menuStage := fabStage(fbMenu.Node(), 200, 180)
	// left placement: need horizontal room for child + gap + trigger
	ctrlStage := fabStage(fbCtrl.Node(), 220, 120)
	cornerStage := fabStage(corner.Node(), 280, 160)

	c.items = append(c.items, ctlTab("float_button", "FloatButton"))
	c.contents["float_button"] = demoPage(c.face, "FloatButton",
		"Floating action button. Position via layout — not OS always-on-top. Defaults: type=default, shape=circle, 40×40 (docs/antd/float-button.md §6 P0).",
		demoSection(c.face, c.theme, "Basic / Type", "default vs primary (antd type.tsx).",
			spaceWrap(16, fbDefault.Node(), fbPrimary.Node())),
		demoSection(c.face, c.theme, "Shape", "circle vs square.",
			spaceWrap(16, fbCircle.Node(), fbSquare.Node())),
		demoSection(c.face, c.theme, "Content", "icon + content caption (square recommended).",
			spaceWrap(16, fbContent.Node())),
		demoSection(c.face, c.theme, "Tooltip", "hover bubble (SetTooltip).",
			spaceWrap(16, fbTip.Node())),
		demoSection(c.face, c.theme, "Disabled / Loading", "disabled swallows click; loading uses Ticker spinner.",
			spaceWrap(16, fbDis.Node(), fbLoad.Node())),
		demoSection(c.face, c.theme, "Group", "no trigger — children always visible (antd group.tsx stack).",
			fbGroup.Node()),
		demoSection(c.face, c.theme, "Menu mode", "antd group-menu.tsx: trigger=click; open shows children above + close icon. Stage height fixed so parent section does not jump.",
			menuStage),
		demoSection(c.face, c.theme, "Controlled + placement=left", "SetOpen(true); children to the left of trigger (antd placement).",
			ctrlStage),
		demoSection(c.face, c.theme, "Placement (layout)", "Bottom-right via Column/Row spacers in a stage — not OS float.",
			cornerStage),
	)
}
