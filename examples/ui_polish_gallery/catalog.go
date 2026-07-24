//go:build linux && !nogpu

package main

import (
	"fmt"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// demoDesc is multi-line secondary copy for gallery pages.
// kit.NewText defaults to single-line (EllipsisRows→1); long 说明 would clip/overflow.
// Use Paragraph + wrap budget so descriptions stay readable under CrossStretch parents.
func demoDesc(face text.Face, s string) core.Node {
	if s == "" {
		return nil
	}
	d := kit.NewParagraph(s)
	d.SetFace(face)
	// Readable body copy: Token secondary A=0.45 looks soft/uneven on CJK strokes.
	// Use near-body text color (antd description ~0.65–0.88), not pure secondary.
	d.SetStyle(kit.Style{Text: render.RGBA{R: 0, G: 0, B: 0, A: 0.65}})
	d.SetEllipsisRows(16) // soft-wrap up to 16 lines; not ellipsis unless SetEllipsis
	return d.Node()
}

// panel wraps a single control demo for its own tab content.
func panel(face text.Face, title string, kids ...core.Node) core.Node {
	col := primitive.Column(demoDesc(face, title))
	col.Gap = 12
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch
	col.Padding = primitive.All(12)
	for _, k := range kids {
		if k != nil {
			col.AddChild(k)
		}
	}
	return col
}

// demoPage is an Ant Design–style component docs page: title + optional description
// + stacked demo sections (scrolls inside Tabs body ScrollViewport).
func demoPage(face text.Face, title, desc string, sections ...core.Node) core.Node {
	lab := kit.NewText(title)
	lab.SetFace(face)
	lab.SetFontSize(20)
	col := primitive.Column(lab.Node())
	col.Gap = 24
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch
	col.Padding = primitive.All(16)
	if n := demoDesc(face, desc); n != nil {
		col.AddChild(n)
	}
	for _, s := range sections {
		if s != nil {
			col.AddChild(s)
		}
	}
	return col
}

// demoSection is one bordered playground block (title + optional desc + content).
func demoSection(face text.Face, theme *core.Theme, title, desc string, body core.Node) core.Node {
	titleT := kit.NewText(title)
	titleT.SetFace(face)
	titleT.SetFontSize(16)
	inner := primitive.Column(titleT.Node())
	inner.Gap = 12
	inner.MainAlign = core.MainStart
	inner.CrossAlign = core.CrossStretch
	if n := demoDesc(face, desc); n != nil {
		inner.AddChild(n)
	}
	if body != nil {
		inner.AddChild(body)
	}
	card := primitive.NewDecorated(inner)
	card.Padding = primitive.All(16)
	card.Radius = 8
	card.BorderWidth = 1
	if theme != nil {
		card.Background = theme.Color(core.TokenColorBgContainer)
		card.BorderColor = theme.Color(core.TokenColorBorder)
	}
	card.Hit = core.HitDefer
	return card
}

// spaceWrap is Ant Space with wrap for demo button rows.
func spaceWrap(gap float64, kids ...core.Node) core.Node {
	sp := kit.NewSpace(kids...)
	sp.SetSize(gap)
	sp.SetWrap(true)
	return sp.Node()
}

func sec(face text.Face, s string) core.Node {
	return demoDesc(face, s)
}

func catHeader(label string) kit.MenuItem {
	return kit.MenuItem{Key: "cat:" + label, Label: label, Disabled: true}
}

func catDivider() kit.MenuItem {
	return kit.MenuItem{Key: "div", Label: "-", Divider: true}
}

func ctlTab(key, label string) kit.MenuItem {
	return kit.MenuItem{Key: key, Label: label}
}

// buildCatalogPanels builds left Tabs rail:
//
//	General (gray header, not clickable)
//	-
//	Button / FloatButton / Icon / Typography (each has own content)
//	Layout (gray header)
//	-
//	Divider / Flex / ...
//
// Every selectable control has its own tab content.
// msgHost is app-level Message/Notification portal (Ant App pattern). Must stay
// mounted under the window root — not only inside a tab panel — or toasts never show
// when the Message tab content is inactive / unmounted.
func buildCatalogPanels(face text.Face, theme *core.Theme, status *string, buttons *[]*kit.Button, tickers *[]interface{ AttachTicker(*core.Tree) }, msgHost *kit.MessageHost) (
	items []kit.MenuItem, contents map[string]core.Node, modal *kit.Modal,
) {
	contents = make(map[string]core.Node)
	add := func(key, label, title string, kids ...core.Node) {
		items = append(items, ctlTab(key, label))
		contents[key] = panel(face, title, kids...)
	}

	// ─── General ───────────────────────────────────────────────
	items = append(items, catHeader("General"), catDivider())

	// Button — Ant Design docs-style multi-section page
	// https://ant.design/components/button
	trackBtn := func(b *kit.Button) *kit.Button {
		b.SetFace(face)
		*buttons = append(*buttons, b)
		return b
	}
	mkBtn := func(label string, typ kit.ButtonType) *kit.Button {
		b := kit.NewButton(label)
		b.SetType(typ)
		return trackBtn(b)
	}

	// Type
	bPrimary := mkBtn("Primary Button", kit.ButtonPrimary)
	bDefault := mkBtn("Default Button", kit.ButtonDefault)
	bDashed := mkBtn("Dashed Button", kit.ButtonDashed)
	bText := mkBtn("Text Button", kit.ButtonText)
	bLink := mkBtn("Link Button", kit.ButtonLink)
	secType := demoSection(face, theme, "Type",
		"There are primary button, default button, dashed button, text button and link button.",
		spaceWrap(8, bPrimary.Node(), bDefault.Node(), bDashed.Node(), bText.Node(), bLink.Node()))

	// Icon
	bIconSearch := mkBtn("Search", kit.ButtonPrimary)
	bIconSearch.SetIcon("search")
	bIconPlus := mkBtn("Add", kit.ButtonDefault)
	bIconPlus.SetIcon("plus")
	bIconOnly := mkBtn("Search", kit.ButtonDefault)
	bIconOnly.SetIcon("search")
	secIcon := demoSection(face, theme, "Icon",
		"Button components can contain an Icon (leading or end).",
		spaceWrap(8, bIconSearch.Node(), bIconPlus.Node(), bIconOnly.Node()))

	// Icon placement end
	bIconEnd := mkBtn("Search", kit.ButtonDefault)
	bIconEnd.SetIcon("search")
	bIconEnd.SetIconPlacement(kit.ButtonIconEnd)
	secIconEnd := demoSection(face, theme, "Icon Placement",
		"Icon at start (default) or end of the label.",
		spaceWrap(8, bIconSearch.Node(), bIconEnd.Node()))

	// Size
	bLarge := mkBtn("Large", kit.ButtonPrimary)
	bLarge.SetSize(kit.ButtonLarge)
	bMiddle := mkBtn("Middle", kit.ButtonPrimary)
	bMiddle.SetSize(kit.ButtonMiddle)
	bSmall := mkBtn("Small", kit.ButtonPrimary)
	bSmall.SetSize(kit.ButtonSmall)
	secSize := demoSection(face, theme, "Size",
		"Ant Design supports a default button size as well as a large and small size.",
		spaceWrap(8, bLarge.Node(), bMiddle.Node(), bSmall.Node()))

	// Disabled
	bDisP := mkBtn("Primary", kit.ButtonPrimary)
	bDisP.SetDisabled(true)
	bDisD := mkBtn("Default", kit.ButtonDefault)
	bDisD.SetDisabled(true)
	bDisH := mkBtn("Dashed", kit.ButtonDashed)
	bDisH.SetDisabled(true)
	bDisT := mkBtn("Text", kit.ButtonText)
	bDisT.SetDisabled(true)
	bDisL := mkBtn("Link", kit.ButtonLink)
	bDisL.SetDisabled(true)
	secDisabled := demoSection(face, theme, "Disabled",
		"To mark a button as disabled, add the disabled property to the Button.",
		spaceWrap(8, bDisP.Node(), bDisD.Node(), bDisH.Node(), bDisT.Node(), bDisL.Node()))

	// Loading
	bLoad1 := mkBtn("Loading", kit.ButtonPrimary)
	bLoad1.SetLoading(true)
	bLoad2 := mkBtn("Loading", kit.ButtonDefault)
	bLoad2.SetLoading(true)
	bLoad3 := mkBtn("Loading", kit.ButtonDashed)
	bLoad3.SetLoading(true)
	*tickers = append(*tickers, bLoad1, bLoad2, bLoad3)
	secLoading := demoSection(face, theme, "Loading",
		"A loading indicator can be added to a button by setting the loading property on the Button.",
		spaceWrap(8, bLoad1.Node(), bLoad2.Node(), bLoad3.Node()))

	// Multiple
	bM1 := mkBtn("Button 1", kit.ButtonDefault)
	bM2 := mkBtn("Button 2", kit.ButtonDefault)
	bM3 := mkBtn("Button 3", kit.ButtonDefault)
	secMultiple := demoSection(face, theme, "Multiple Buttons",
		"If you need several buttons, we recommend that you use Space to set the spacing.",
		spaceWrap(8, bM1.Node(), bM2.Node(), bM3.Node()))

	// Danger
	bDangP := mkBtn("Primary", kit.ButtonPrimary)
	bDangP.SetDanger(true)
	bDangD := mkBtn("Default", kit.ButtonDefault)
	bDangD.SetDanger(true)
	bDangH := mkBtn("Dashed", kit.ButtonDashed)
	bDangH.SetDanger(true)
	bDangT := mkBtn("Text", kit.ButtonText)
	bDangT.SetDanger(true)
	bDangL := mkBtn("Link", kit.ButtonLink)
	bDangL.SetDanger(true)
	secDanger := demoSection(face, theme, "Danger Buttons",
		"Danger buttons are used for actions with higher risk.",
		spaceWrap(8, bDangP.Node(), bDangD.Node(), bDangH.Node(), bDangT.Node(), bDangL.Node()))

	// Block
	bBlock := mkBtn("Primary Block Button", kit.ButtonPrimary)
	bBlock.SetBlock(true)
	bBlock2 := mkBtn("Default Block Button", kit.ButtonDefault)
	bBlock2.SetBlock(true)
	blockCol := primitive.Column(bBlock.Node(), bBlock2.Node())
	blockCol.Gap = 8
	blockCol.CrossAlign = core.CrossStretch
	secBlock := demoSection(face, theme, "Block Button",
		"block property will make the button fit to its parent width.",
		blockCol)

	// Ghost — Ant demos put ghost on a dark/complex surface so transparency is obvious.
	bGhostP := mkBtn("Primary", kit.ButtonPrimary)
	bGhostP.SetGhost(true)
	bGhostD := mkBtn("Default", kit.ButtonDefault)
	bGhostD.SetGhost(true)
	bGhostR := mkBtn("Danger", kit.ButtonPrimary)
	bGhostR.SetDanger(true)
	bGhostR.SetGhost(true)
	ghostRow := kit.NewSpace(bGhostP.Node(), bGhostD.Node(), bGhostR.Node())
	ghostRow.SetSize(8)
	ghostRow.SetWrap(true)
	ghostHost := primitive.NewDecorated(ghostRow.Node())
	ghostHost.Padding = primitive.All(16)
	ghostHost.Radius = 8
	// Ant docs use a dark band behind ghost buttons.
	ghostHost.Background = render.RGBA{R: 0.12, G: 0.14, B: 0.18, A: 1}
	ghostHost.BorderWidth = 0
	secGhost := demoSection(face, theme, "Ghost Button",
		"Ghost = transparent fill (use on dark / image backgrounds). Primary/Default/Danger differ by border & text color.",
		ghostHost)

	// Color & Variant — each variant looks different; Color changes the accent.
	mkVar := func(label string, v kit.ButtonVariant, c kit.ButtonColor) core.Node {
		b := mkBtn(label, kit.ButtonDefault)
		b.SetVariant(v)
		b.SetColor(c)
		return b.Node()
	}
	rowPrimary := spaceWrap(8,
		mkVar("Solid", kit.ButtonVariantSolid, kit.ButtonColorPrimary),
		mkVar("Outlined", kit.ButtonVariantOutlined, kit.ButtonColorPrimary),
		mkVar("Dashed", kit.ButtonVariantDashed, kit.ButtonColorPrimary),
		mkVar("Filled", kit.ButtonVariantFilled, kit.ButtonColorPrimary),
		mkVar("Text", kit.ButtonVariantText, kit.ButtonColorPrimary),
		mkVar("Link", kit.ButtonVariantLink, kit.ButtonColorPrimary),
	)
	rowDanger := spaceWrap(8,
		mkVar("Solid", kit.ButtonVariantSolid, kit.ButtonColorDanger),
		mkVar("Outlined", kit.ButtonVariantOutlined, kit.ButtonColorDanger),
		mkVar("Filled", kit.ButtonVariantFilled, kit.ButtonColorDanger),
		mkVar("Text", kit.ButtonVariantText, kit.ButtonColorDanger),
	)
	rowSuccess := spaceWrap(8,
		mkVar("Solid", kit.ButtonVariantSolid, kit.ButtonColorSuccess),
		mkVar("Outlined", kit.ButtonVariantOutlined, kit.ButtonColorSuccess),
		mkVar("Filled", kit.ButtonVariantFilled, kit.ButtonColorSuccess),
	)
	varCol := primitive.Column(
		sec(face, "color=primary · variant=…"),
		rowPrimary,
		sec(face, "color=danger"),
		rowDanger,
		sec(face, "color=success"),
		rowSuccess,
	)
	varCol.Gap = 10
	varCol.CrossAlign = core.CrossStart
	secVariant := demoSection(face, theme, "Color & Variant",
		"Solid=fill · Outlined=border · Dashed=dashed border · Filled=light wash · Text/Link=no chrome. Same Type「Primary」≈ Solid+primary.",
		varCol)

	items = append(items, ctlTab("btn", "Button"))
	contents["btn"] = demoPage(face, "Button",
		"To trigger an operation. Aligns Ant Design Button demos (type/size/icon/disabled/loading/danger/block/ghost/variant).",
		secType, secIcon, secIconEnd, secSize, secDisabled, secLoading, secMultiple, secDanger, secBlock, secGhost, secVariant)

	// FloatButton — Ant Design demos (docs/antd/float-button.md §6.8 P0)
	// https://ant.design/components/float-button
	trackFB := func(fb *kit.FloatButton) *kit.FloatButton {
		fb.SetFace(face)
		if fb.AriaLabel == "" {
			fb.SetAriaLabel("float-button")
		}
		if fb.Button() != nil {
			*buttons = append(*buttons, fb.Button())
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
	*tickers = append(*tickers, fbLoad)
	// group (always open)
	gChild1 := trackFB(kit.NewFloatButton())
	gChild1.SetIcon("plus")
	gChild1.SetAriaLabel("g1")
	gChild2 := trackFB(kit.NewFloatButton())
	gChild2.SetIcon("info")
	gChild2.SetAriaLabel("g2")
	fbGroup := kit.NewFloatButtonGroup(gChild1, gChild2)
	fbGroup.SetFace(face)
	// menu mode click (antd group-menu.tsx) — fixed stage so overlay list is not clipped
	m1 := trackFB(kit.NewFloatButton())
	m1.SetIcon("plus")
	m1.SetAriaLabel("m1")
	m2 := trackFB(kit.NewFloatButton())
	m2.SetIcon("info")
	m2.SetAriaLabel("m2")
	fbMenu := kit.NewFloatButtonGroup(m1, m2)
	fbMenu.SetFace(face)
	fbMenu.SetTrigger(kit.FloatButtonTriggerClick)
	fbMenu.SetType(kit.ButtonPrimary)
	fbMenu.SetIcon("plus")
	fbMenu.SetDefaultOpen(true)
	if tr := fbMenu.TriggerButton(); tr != nil && tr.Button() != nil {
		*buttons = append(*buttons, tr.Button())
	}
	// controlled + placement left (antd controlled / placement)
	c1 := trackFB(kit.NewFloatButton())
	c1.SetIcon("plus")
	c1.SetAriaLabel("c1")
	fbCtrl := kit.NewFloatButtonGroup(c1)
	fbCtrl.SetFace(face)
	fbCtrl.SetTrigger(kit.FloatButtonTriggerClick)
	fbCtrl.SetPlacement(kit.FloatButtonLeft)
	fbCtrl.SetOpen(true)
	fbCtrl.SetIcon("info")
	if tr := fbCtrl.TriggerButton(); tr != nil && tr.Button() != nil {
		*buttons = append(*buttons, tr.Button())
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

	items = append(items, ctlTab("float_button", "FloatButton"))
	contents["float_button"] = demoPage(face, "FloatButton",
		"Floating action button. Position via layout — not OS always-on-top. Defaults: type=default, shape=circle, 40×40 (docs/antd/float-button.md §6 P0).",
		demoSection(face, theme, "Basic / Type", "default vs primary (antd type.tsx).",
			spaceWrap(16, fbDefault.Node(), fbPrimary.Node())),
		demoSection(face, theme, "Shape", "circle vs square.",
			spaceWrap(16, fbCircle.Node(), fbSquare.Node())),
		demoSection(face, theme, "Content", "icon + content caption (square recommended).",
			spaceWrap(16, fbContent.Node())),
		demoSection(face, theme, "Tooltip", "hover bubble (SetTooltip).",
			spaceWrap(16, fbTip.Node())),
		demoSection(face, theme, "Disabled / Loading", "disabled swallows click; loading uses Ticker spinner.",
			spaceWrap(16, fbDis.Node(), fbLoad.Node())),
		demoSection(face, theme, "Group", "no trigger — children always visible (antd group.tsx stack).",
			fbGroup.Node()),
		demoSection(face, theme, "Menu mode", "antd group-menu.tsx: trigger=click; open shows children above + close icon. Stage height fixed so parent section does not jump.",
			menuStage),
		demoSection(face, theme, "Controlled + placement=left", "SetOpen(true); children to the left of trigger (antd placement).",
			ctrlStage),
		demoSection(face, theme, "Placement (layout)", "Bottom-right via Column/Row spacers in a stage — not OS float.",
			cornerStage),
	)

	// Icon — docs/antd/icon.md §6.8 P0
	// https://ant.design/components/icon
	trackIcon := func(ic *kit.Icon) *kit.Icon {
		return ic
	}
	trackSpinIcon := func(ic *kit.Icon) *kit.Icon {
		ic.SetSpin(true)
		*tickers = append(*tickers, ic)
		return ic
	}
	icCheck := trackIcon(kit.NewIcon("check"))
	icInfo := trackIcon(kit.NewIcon("info"))
	icSearch := trackIcon(kit.NewIcon("search"))
	icPlus := trackIcon(kit.NewIcon("plus"))
	icLoading := trackSpinIcon(kit.NewIcon("loading"))
	icRot := trackIcon(kit.NewIcon("info"))
	icRot.SetRotate(180)

	secIconBasic := demoSection(face, theme, "Basic",
		"Named registry icons; spin + rotate (antd basic.tsx).",
		spaceWrap(12,
			icCheck.Node(), icInfo.Node(), icSearch.Node(), icPlus.Node(),
			icLoading.Node(), icRot.Node()))

	icStar := trackIcon(kit.NewIcon("star"))
	icHeart := trackIcon(kit.NewIcon("heart"))
	icHeart.SetTwoToneColor(render.RGBA{R: 0.92, G: 0.18, B: 0.59, A: 1})
	icCheckTT := trackIcon(kit.NewIcon("check"))
	icCheckTT.SetTwoToneColors(
		render.RGBA{R: 0.32, G: 0.77, B: 0.1, A: 1},
		render.RGBA{R: 0.32, G: 0.77, B: 0.1, A: 0.25})
	secIconTwoTone := demoSection(face, theme, "Two-tone",
		"twoToneColor primary / primary+secondary (antd two-tone.tsx).",
		spaceWrap(12, icStar.Node(), icHeart.Node(), icCheckTT.Node()))

	icCustom := trackIcon(kit.NewIcon(""))
	icCustom.SetSize(28)
	icCustom.SetColor(render.RGBA{R: 1, G: 0.41, B: 0.71, A: 1}) // hotpink
	icCustom.SetPainter(func(pc *core.PaintContext, size float64, primary, secondary render.RGBA) {
		if pc == nil {
			return
		}
		pc.FillLocalCircle(size*0.35, size*0.38, size*0.18, primary)
		pc.FillLocalCircle(size*0.65, size*0.38, size*0.18, primary)
		pc.StrokeLocalPolyline([]float64{
			size * 0.18, size * 0.42,
			size * 0.5, size * 0.82,
			size * 0.82, size * 0.42,
		}, 2, primary)
	})
	icCustomBig := trackIcon(kit.NewIcon("info"))
	icCustomBig.SetSize(32)
	secIconCustom := demoSection(face, theme, "Custom",
		"SetPainter (antd component) + large size.",
		spaceWrap(12, icCustom.Node(), icCustomBig.Node()))

	// Offline iconfont (antd createFromIconfontCN — no CDN).
	kit.RegisterIconSource("gallery_iconfont", map[string]primitive.IconDef{
		"icon-tuichu":   {Kind: primitive.IconClose},
		"icon-facebook": {Kind: primitive.IconInfo},
		"icon-twitter":  {Kind: primitive.IconStar},
	})
	kit.RegisterIconSource("gallery_iconfont_b", map[string]primitive.IconDef{
		"icon-shoppingcart": {Kind: primitive.IconPlus},
		"icon-python":       {Kind: primitive.IconSearch},
	})
	kit.RegisterIconSource("gallery_iconfont_c", map[string]primitive.IconDef{
		"icon-shoppingcart": {Kind: primitive.IconCheck}, // later source overrides
		"icon-javascript":   {Kind: primitive.IconInfo},
	})
	fontA := kit.CreateFromIconfont(kit.IconfontOptions{Sources: []string{"gallery_iconfont"}})
	ff1 := fontA.NewIcon("icon-tuichu")
	ff2 := fontA.NewIcon("icon-facebook")
	ff2.SetColor(render.RGBA{R: 0.09, G: 0.47, B: 0.95, A: 1})
	ff3 := fontA.NewIcon("icon-twitter")
	secIconFont := demoSection(face, theme, "Iconfont (offline)",
		"CreateFromIconfont + RegisterSource — maps antd iconfont.cn without network.",
		spaceWrap(12, ff1.Node(), ff2.Node(), ff3.Node()))

	fontMulti := kit.CreateFromIconfont(kit.IconfontOptions{
		Sources: []string{"gallery_iconfont_b", "gallery_iconfont_c"},
	})
	fm1 := fontMulti.NewIcon("icon-javascript")
	fm2 := fontMulti.NewIcon("icon-shoppingcart") // from c → check
	fm3 := fontMulti.NewIcon("icon-python")
	secIconMulti := demoSection(face, theme, "Multi-source",
		"Multiple sources; later overrides same type (antd scriptUrl[]).",
		spaceWrap(12, fm1.Node(), fm2.Node(), fm3.Node()))

	icSz16 := trackIcon(kit.NewIcon("search"))
	icSz24 := trackIcon(kit.NewIcon("search"))
	icSz24.SetSize(24)
	icSz32 := trackIcon(kit.NewIcon("search"))
	icSz32.SetSize(32)
	icCol := trackIcon(kit.NewIcon("check"))
	icCol.SetColor(render.RGBA{R: 0.09, G: 0.42, B: 0.93, A: 1})
	icDis := trackIcon(kit.NewIcon("close"))
	icDis.SetDisabled(true)
	secIconStyle := demoSection(face, theme, "Size & color",
		"Default 16; SetSize; SetColor; disabled tint.",
		spaceWrap(12, icSz16.Node(), icSz24.Node(), icSz32.Node(), icCol.Node(), icDis.Node()))

	items = append(items, ctlTab("icon", "Icon"))
	contents["icon"] = demoPage(face, "Icon",
		"Semantic vector icons. P0: name, size, color, rotate, spin, twoTone, painter, offline iconfont multi-source, decorative a11y.",
		secIconBasic, secIconTwoTone, secIconCustom, secIconFont, secIconMulti, secIconStyle)

	// Typography — docs/antd/typography.md §6.8 P0
	// https://ant.design/components/typography
	secTypoBasic := demoSection(face, theme, "Basic",
		"Title + Paragraph (antd basic.tsx).",
		primitive.Column(
			func() core.Node {
				h := kit.NewTitle("Introduction", 2)
				h.SetFace(face)
				return h.Node()
			}(),
			func() core.Node {
				p := kit.NewParagraph("Ant Design, a design language for background applications, is refined by Ant UED Team.")
				p.SetFace(face)
				p.SetMaxWidth(520)
				return p.Node()
			}(),
		))

	titleKids := make([]core.Node, 0, 5)
	for lv := 1; lv <= 5; lv++ {
		h := kit.NewTitle(fmt.Sprintf("h%d. Ant Design", lv), lv)
		h.SetFace(face)
		titleKids = append(titleKids, h.Node())
	}
	secTypoTitle := demoSection(face, theme, "Title",
		"level 1..5 → 38/30/24/20/16 (antd title.tsx).",
		primitive.Column(titleKids...))

	mkText := func(label string, cfg func(*kit.Typography)) core.Node {
		x := kit.NewText(label)
		x.SetFace(face)
		if cfg != nil {
			cfg(x)
		}
		return x.Node()
	}
	secTypoText := demoSection(face, theme, "Text & Link",
		"type / mark / code / delete / underline / strong / Link (antd text.tsx).",
		spaceWrap(12,
			mkText("default", nil),
			mkText("secondary", func(t *kit.Typography) { t.SetType(kit.TypographyTypeSecondary) }),
			mkText("success", func(t *kit.Typography) { t.SetType(kit.TypographyTypeSuccess) }),
			mkText("warning", func(t *kit.Typography) { t.SetType(kit.TypographyTypeWarning) }),
			mkText("danger", func(t *kit.Typography) { t.SetType(kit.TypographyTypeDanger) }),
			mkText("mark", func(t *kit.Typography) { t.SetMark(true) }),
			mkText("code", func(t *kit.Typography) { t.SetCode(true) }),
			mkText("delete", func(t *kit.Typography) { t.SetDelete(true) }),
			mkText("underline", func(t *kit.Typography) { t.SetUnderline(true) }),
			mkText("strong", func(t *kit.Typography) { t.SetStrong(true) }),
			func() core.Node {
				l := kit.NewLink("Ant Design")
				l.SetFace(face)
				return l.Node()
			}(),
		))

	editTx := kit.NewText("This is an editable text.")
	editTx.SetFace(face)
	editTx.SetEditable(true)
	secTypoEdit := demoSection(face, theme, "Editable",
		"Edit icon → Enter commit / Esc cancel (antd editable.tsx).",
		editTx.Node())

	copyTx := kit.NewText("This is a copyable text.")
	copyTx.SetFace(face)
	copyTx.SetCopyable(true)
	secTypoCopy := demoSection(face, theme, "Copyable",
		"Copy icon → Tree clipboard + onCopy (antd copyable.tsx).",
		copyTx.Node())

	ellip := kit.NewParagraph(strings.Repeat("Ant Design, a design language for background applications, is refined by Ant UED Team. ", 6))
	ellip.SetFace(face)
	ellip.SetEllipsis(true)
	ellip.SetEllipsisRows(3)
	ellip.SetMaxWidth(420)
	ellip.SetExpandable(true)
	ellip.SetCollapsible(true)
	secTypoEllipsis := demoSection(face, theme, "Ellipsis",
		"rows=3 + expandable/collapsible (antd ellipsis.tsx).",
		ellip.Node())

	ellipCtrl := kit.NewParagraph(strings.Repeat("Controlled expand / collapse. ", 12))
	ellipCtrl.SetFace(face)
	ellipCtrl.SetEllipsis(true)
	ellipCtrl.SetEllipsisRows(2)
	ellipCtrl.SetMaxWidth(420)
	ellipCtrl.SetExpandable(true)
	ellipCtrl.SetCollapsible(true)
	ellipCtrl.SetExpanded(false)
	secTypoEllipsisCtrl := demoSection(face, theme, "Controlled expand",
		"SetExpanded controlled (antd ellipsis-controlled.tsx).",
		ellipCtrl.Node())

	ellipMid := kit.NewText("https://ant.design/components/typography-cn#components-typography-demo-ellipsis-middle")
	ellipMid.SetFace(face)
	ellipMid.SetEllipsis(true)
	ellipMid.SetEllipsisMiddle(true)
	ellipMid.SetMaxWidth(280)
	secTypoMiddle := demoSection(face, theme, "Ellipsis middle",
		"start…end truncation (antd ellipsis-middle.tsx).",
		ellipMid.Node())

	disTx := kit.NewText("disabled text")
	disTx.SetFace(face)
	disTx.SetDisabled(true)
	disTx.SetCopyable(true)
	secTypoDisabled := demoSection(face, theme, "Disabled",
		"disabled color; copy/edit hidden.",
		disTx.Node())

	items = append(items, ctlTab("typography", "Typography"))
	contents["typography"] = demoPage(face, "Typography",
		"Text / Title / Paragraph / Link. P0: type, disabled, copyable, editable, ellipsis(+middle/controlled), decorations, Token 14 & title ladder (docs/antd/typography.md §6).",
		secTypoBasic, secTypoTitle, secTypoText, secTypoEdit, secTypoCopy,
		secTypoEllipsis, secTypoEllipsisCtrl, secTypoMiddle, secTypoDisabled,
	)

	// ─── Layout ────────────────────────────────────────────────
	items = append(items, catHeader("Layout"), catDivider())

	// Divider — docs/antd/divider.md §6.8 P0
	// https://ant.design/components/divider
	para := func(s string) core.Node {
		tx := kit.NewText(s)
		tx.SetFace(face)
		tx.SetSecondary(true)
		return tx.Node()
	}
	divPlain := kit.NewDivider()
	divDashed := kit.NewDivider()
	divDashed.SetDashed(true)
	secDivHorizontal := demoSection(face, theme, "Horizontal",
		"Default solid + dashed sugar (horizontal.tsx).",
		primitive.Column(
			para("Lorem ipsum dolor sit amet, consectetur adipiscing elit."),
			divPlain.Node(),
			para("Sed nonne merninisti licere mihi ista probare."),
			divDashed.Node(),
			para("Refert tamen, quo modo."),
		))

	mkTitleDiv := func(title string, place kit.DividerTitlePlacement, plain bool) core.Node {
		d := kit.NewDivider()
		d.SetTitle(title)
		d.SetTitlePlacement(place)
		d.SetPlain(plain)
		d.SetFace(face)
		return d.Node()
	}
	secDivWithText := demoSection(face, theme, "With text",
		"titlePlacement center / start / end (with-text.tsx).",
		primitive.Column(
			para("Above center"),
			mkTitleDiv("Text", kit.DividerTitleCenter, false),
			para("Above start"),
			mkTitleDiv("Left Text", kit.DividerTitleStart, false),
			para("Above end"),
			mkTitleDiv("Right Text", kit.DividerTitleEnd, false),
		))

	mkSize := func(s kit.DividerSize, label string) core.Node {
		d := kit.NewDivider()
		d.SetSize(s)
		return primitive.Column(para(label), d.Node())
	}
	secDivSize := demoSection(face, theme, "Size",
		"Horizontal marginBlock: small=8 · medium=16 · large/unset=24 (size.tsx).",
		primitive.Column(
			mkSize(kit.DividerSizeSmall, "small"),
			mkSize(kit.DividerSizeMedium, "medium"),
			mkSize(kit.DividerSizeLarge, "large"),
		))

	secDivPlain := demoSection(face, theme, "Plain",
		"Title uses body fontSize=14 (plain.tsx).",
		primitive.Column(
			mkTitleDiv("Text", kit.DividerTitleCenter, true),
			mkTitleDiv("Left Text", kit.DividerTitleStart, true),
			mkTitleDiv("Right Text", kit.DividerTitleEnd, true),
		))

	mkVert := func() core.Node {
		d := kit.NewDivider()
		d.SetOrientation(kit.DividerVertical)
		return d.Node()
	}
	secDivVertical := demoSection(face, theme, "Vertical",
		"Inline vertical rails (vertical.tsx).",
		func() core.Node {
			mkLab := func(s string) core.Node {
				tx := kit.NewText(s)
				tx.SetFace(face)
				return tx.Node()
			}
			row := primitive.Row(
				mkLab("Text"),
				mkVert(),
				mkLab("Link"),
				mkVert(),
				mkLab("Link"),
			)
			row.CrossAlign = core.CrossCenter
			row.Gap = 4
			return row
		}())

	mkDivVar := func(v kit.DividerVariant, title string) core.Node {
		d := kit.NewDivider()
		d.SetTitle(title)
		d.SetVariant(v)
		d.SetFace(face)
		d.SetStyle(kit.Style{Border: render.Hex("#7cb305")})
		return d.Node()
	}
	secDivVariant := demoSection(face, theme, "Variant",
		"solid / dotted / dashed + Style.Border override (variant.tsx).",
		primitive.Column(
			mkDivVar(kit.DividerSolid, "Solid"),
			mkDivVar(kit.DividerDotted, "Dotted"),
			mkDivVar(kit.DividerDashed, "Dashed"),
		))

	labInline := func(s string) core.Node {
		tx := kit.NewText(s)
		tx.SetFace(face)
		return tx.Node()
	}
	secDivSemantic := demoSection(face, theme, "Structure (semantic)",
		"root / rail / content structure (classNames depth = P1).",
		func() core.Node {
			col := primitive.Column(
				mkTitleDiv("root · rail · content", kit.DividerTitleCenter, false),
				func() core.Node {
					// antd _semantic.tsx: These | are | vertical | Dividers
					row := primitive.Row(
						labInline("These"),
						mkVert(),
						labInline("are"),
						mkVert(),
						labInline("vertical"),
						mkVert(),
						labInline("Dividers"),
					)
					row.CrossAlign = core.CrossCenter
					row.Gap = 4
					return row
				}(),
			)
			col.Gap = 8
			col.CrossAlign = core.CrossStretch
			// Extra bottom inset so last line is not flush against scroll clip.
			col.Padding = primitive.EdgeInsets{Bottom: 8}
			return col
		}())

	// style-class.tsx — semantic classNames/styles 深度为 P1；示例用 Title/Placement/Size/Style 贴近官方四格
	secDivStyleClass := demoSection(face, theme, "Style / classNames",
		"antd style-class.tsx：classNames Object|Function · styles Object|Function（API 深度 P1，此处示意）。",
		func() core.Node {
			// 1) classNames Object — 仅挂标题，语义名写在文案上
			dClassObj := kit.NewDivider()
			dClassObj.SetTitle("classNames Object")
			dClassObj.SetFace(face)

			// 2) classNames Function — titlePlacement=start 时走不同分支（antd info.props.titlePlacement）
			dClassFn := kit.NewDivider()
			dClassFn.SetTitle("classNames Function")
			dClassFn.SetTitlePlacement(kit.DividerTitleStart)
			dClassFn.SetFace(face)

			// 3) styles Object — root dashed+较粗线色、content 次级色示意 italic、rail 略淡
			dStylesObj := kit.NewDivider()
			dStylesObj.SetTitle("styles Object")
			dStylesObj.SetVariant(kit.DividerDashed)
			dStylesObj.SetFace(face)
			dStylesObj.SetStyle(kit.Style{
				Border: render.Hex("#1677FF"),
				Text:   render.RGBA{R: 0, G: 0, B: 0, A: 0.45}, // content 示意
			})

			// 4) styles Function — size=small 时弱化；否则浅底+边色（antd stylesFn 分支）
			dStylesFnSmall := kit.NewDivider()
			dStylesFnSmall.SetTitle("styles Function (size=small)")
			dStylesFnSmall.SetSize(kit.DividerSizeSmall)
			dStylesFnSmall.SetFace(face)
			dStylesFnSmall.SetStyle(kit.Style{
				Text: render.RGBA{R: 0, G: 0, B: 0, A: 0.35}, // opacity≈0.6 示意
			})
			// wrap small sample with muted chrome to echo root opacity
			smallHost := primitive.NewDecorated(dStylesFnSmall.Node())
			smallHost.Padding = primitive.Symmetric(0, 0)
			smallHost.Background = render.RGBA{R: 1, G: 1, B: 1, A: 0.6}

			dStylesFnDefault := kit.NewDivider()
			dStylesFnDefault.SetTitle("styles Function (default)")
			dStylesFnDefault.SetFace(face)
			dStylesFnDefault.SetStyle(kit.Style{Border: render.Hex("#d9d9d9")})
			defHost := primitive.NewDecorated(dStylesFnDefault.Node())
			defHost.Padding = primitive.Symmetric(8, 4)
			defHost.Background = render.Hex("#fafafa")
			defHost.BorderWidth = 1
			defHost.BorderColor = render.Hex("#d9d9d9")
			defHost.Radius = 4

			note := kit.NewText("注：class 字符串 / 函数式 semantic 钩子为 P1；上列用 Placement·Size·Variant·Style 复现主视觉。")
			note.SetFace(face)
			note.SetSecondary(true)

			col := primitive.Column(
				dClassObj.Node(),
				dClassFn.Node(),
				dStylesObj.Node(),
				smallHost,
				defHost,
				note.Node(),
			)
			col.Gap = 12
			col.CrossAlign = core.CrossStretch
			col.Padding = primitive.EdgeInsets{Bottom: 8}
			return col
		}())

	items = append(items, ctlTab("divider", "Divider"))
	contents["divider"] = demoPage(face, "Divider",
		"区隔内容的分割线。P0: orientation / size / variant / title / plain / titlePlacement；style-class 示意（semantic 深度 P1）。",
		secDivHorizontal, secDivWithText, secDivSize, secDivPlain, secDivVertical, secDivVariant, secDivStyleClass, secDivSemantic,
	)

	// Flex — antd demos: basic / align / gap / wrap / combination
	// https://ant.design/components/flex · components/flex/demo/*.tsx
	//
	// Responsive rules (match antd docs, avoid overflow outside parent):
	//   - playgrounds use ExpandWidth so they shrink with the tab body
	//   - wrap Flex must see a bounded MaxWidth (ExpandWidth host)
	//   - fixed 480/520/620 widths were causing children to paint past the card
	// Playground host: fills available width, clips overflow, optional fixed height.
	playground := func(body core.Node, h float64, border render.RGBA) *primitive.Decorated {
		d := primitive.NewDecorated(body)
		d.ExpandWidth = true
		d.StretchChild = true
		d.Radius = 6
		d.Padding = primitive.All(0)
		d.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		if h > 0 {
			d.Height = h
		}
		if border.A > 0 {
			d.BorderWidth = 1
			d.BorderColor = border
		}
		return d
	}
	// antd basic.tsx bars: height 54.
	// horizontal → Flexible equal width (≈25%); vertical → fixed h=54, stretch width.
	mkBar := func(solid bool) *primitive.Decorated {
		d := primitive.NewDecorated(nil)
		d.Height = 54
		d.MinHeight = 54
		if solid {
			d.Background = render.Hex("#1677ff")
		} else {
			d.Background = render.Hex("#1677ffbf")
		}
		return d
	}
	basicBars := kit.NewFlex()
	fillBasicBars := func(vertical bool) {
		solids := []bool{false, true, false, true}
		kids := make([]core.Node, 0, 4)
		for _, solid := range solids {
			d := mkBar(solid)
			if vertical {
				kids = append(kids, d)
			} else {
				fl := primitive.NewFlexible(1, d)
				fl.FillChild = true
				kids = append(kids, fl)
			}
		}
		basicBars.SetChildren(kids...)
		basicBars.SetVertical(vertical)
		if vertical {
			basicBars.SetAlign(kit.FlexAlignStretch)
		} else {
			basicBars.SetAlign(kit.FlexAlignAuto)
		}
	}
	fillBasicBars(false)
	mkPrimaryBtn := func(label string) *kit.Button {
		b := kit.NewButton(label)
		b.SetFace(face)
		b.SetType(kit.ButtonPrimary)
		return trackBtn(b)
	}
	mkBtnTyped := func(label string, typ kit.ButtonType) *kit.Button {
		b := kit.NewButton(label)
		b.SetFace(face)
		b.SetType(typ)
		return trackBtn(b)
	}
	radioRow := func(g *kit.RadioGroup) {
		if fl, ok := g.Node().(*primitive.Flex); ok {
			fl.Axis = core.AxisHorizontal
			fl.CrossAlign = core.CrossCenter
			fl.Gap = 16
			fl.Wrap = true
			fl.MarkNeedsLayout()
		}
	}

	// ---------- basic.tsx ----------
	basicHost := playground(basicBars.Node(), 0, render.RGBA{})
	basicHost.Radius = 0

	rh := kit.NewRadio("horizontal", "horizontal")
	rv := kit.NewRadio("vertical", "vertical")
	rh.SetFace(face)
	rv.SetFace(face)
	basicRG := kit.NewRadioGroup(rh, rv)
	basicRG.Select("horizontal")
	radioRow(basicRG)
	basicRG.OnChange = func(v string) {
		fillBasicBars(v == "vertical")
		basicHost.MarkNeedsLayout()
		*status = "flex basic → " + v
	}
	basicOuter := kit.NewFlex(basicRG.Node(), basicHost)
	basicOuter.SetVertical(true)
	basicOuter.SetGapSize(kit.FlexGapMedium)
	secFlexBasic := demoSection(face, theme, "基本布局",
		"antd basic.tsx：Radio 切换 horizontal / vertical。色块高 54；水平均分宽度，垂直通栏堆叠。宿主 ExpandWidth，随窗口收缩。",
		basicOuter.Node())

	// ---------- align.tsx ----------
	alignBtns := kit.NewFlex(
		mkPrimaryBtn("Primary").Node(),
		mkPrimaryBtn("Primary").Node(),
		mkPrimaryBtn("Primary").Node(),
		mkPrimaryBtn("Primary").Node(),
	)
	alignBtns.SetJustify(kit.FlexJustifyStart)
	alignBtns.SetAlign(kit.FlexAlignStart)
	// antd align.tsx: no wrap — align-items needs a single line in the tall playground
	alignBtns.SetWrap(false)
	alignBtns.SetGap(8)
	alignPlay := playground(alignBtns.Node(), 120, render.Hex("#40a9ff"))

	applyJustify := func(v string) {
		switch v {
		case "center":
			alignBtns.SetJustify(kit.FlexJustifyCenter)
		case "flex-end":
			alignBtns.SetJustify(kit.FlexJustifyEnd)
		case "space-between":
			alignBtns.SetJustify(kit.FlexJustifySpaceBetween)
		case "space-around":
			alignBtns.SetJustify(kit.FlexJustifySpaceAround)
		case "space-evenly":
			alignBtns.SetJustify(kit.FlexJustifySpaceEvenly)
		default:
			alignBtns.SetJustify(kit.FlexJustifyStart)
		}
		alignPlay.MarkNeedsLayout()
		*status = "flex justify → " + v
	}
	applyAlign := func(v string) {
		switch v {
		case "center":
			alignBtns.SetAlign(kit.FlexAlignCenter)
		case "flex-end":
			alignBtns.SetAlign(kit.FlexAlignEnd)
		default:
			alignBtns.SetAlign(kit.FlexAlignStart)
		}
		alignPlay.MarkNeedsLayout()
		*status = "flex align → " + v
	}
	justifySeg := kit.NewSegmented(
		"flex-start", "center", "flex-end", "space-between", "space-around", "space-evenly",
	)
	justifySeg.SetFace(face)
	justifySeg.OnChange = applyJustify
	alignSeg := kit.NewSegmented("flex-start", "center", "flex-end")
	alignSeg.SetFace(face)
	alignSeg.OnChange = applyAlign
	lblJ := kit.NewText("Select justify :")
	lblJ.SetFace(face)
	lblA := kit.NewText("Select align :")
	lblA.SetFace(face)
	alignOuter := kit.NewFlex(
		lblJ.Node(), justifySeg.Node(),
		lblA.Node(), alignSeg.Node(),
		alignPlay,
	)
	alignOuter.SetVertical(true)
	alignOuter.SetGapSize(kit.FlexGapMedium)
	alignOuter.SetAlign(kit.FlexAlignStart)
	secFlexAlign := demoSection(face, theme, "对齐方式",
		"antd align.tsx：Segmented 选 justify / align。场地高度 120、边框 #40a9ff、宽度 100%（ExpandWidth），缩窗口时按钮可 wrap，不跑出父容器。",
		alignOuter.Node())

	// ---------- gap.tsx ----------
	gapRow := kit.NewFlex(
		mkBtnTyped("Primary", kit.ButtonPrimary).Node(),
		mkBtnTyped("Default", kit.ButtonDefault).Node(),
		mkBtnTyped("Dashed", kit.ButtonDashed).Node(),
		mkBtnTyped("Link", kit.ButtonLink).Node(),
	)
	gapRow.SetGapSize(kit.FlexGapSmall)
	gapRow.SetWrap(true)

	rSmall := kit.NewRadio("small", "small")
	rMed := kit.NewRadio("medium", "medium")
	rLarge := kit.NewRadio("large", "large")
	rCust := kit.NewRadio("customize", "customize")
	for _, r := range []*kit.Radio{rSmall, rMed, rLarge, rCust} {
		r.SetFace(face)
	}
	gapRG := kit.NewRadioGroup(rSmall, rMed, rLarge, rCust)
	gapRG.Select("small")
	radioRow(gapRG)
	gapSlider := kit.NewSlider(16)
	gapSlider.Min, gapSlider.Max = 0, 64
	gapSlider.Width = 0 // follow host
	// Slider host expands with content width when shown
	sliderSlot := primitive.NewSlot("flex-gap-slider", nil)

	applyGapMode := func(mode string) {
		switch mode {
		case "medium":
			gapRow.SetGapSize(kit.FlexGapMedium)
			sliderSlot.SetChild(nil)
		case "large":
			gapRow.SetGapSize(kit.FlexGapLarge)
			sliderSlot.SetChild(nil)
		case "customize":
			gapRow.SetGap(gapSlider.Value)
			// wrap slider in ExpandWidth host so it tracks tab width
			sh := primitive.NewDecorated(gapSlider.Node())
			sh.ExpandWidth = true
			sh.Height = 32
			sh.StretchChild = true
			sliderSlot.SetChild(sh)
		default:
			gapRow.SetGapSize(kit.FlexGapSmall)
			sliderSlot.SetChild(nil)
		}
		*status = "flex gap → " + mode
	}
	gapRG.OnChange = applyGapMode
	gapSlider.OnChange = func(v float64) {
		if gapRG.Value == "customize" {
			gapRow.SetGap(v)
			*status = fmt.Sprintf("flex gap custom → %.0f", v)
		}
	}
	gapOuter := kit.NewFlex(gapRG.Node(), sliderSlot, gapRow.Node())
	gapOuter.SetVertical(true)
	gapOuter.SetGapSize(kit.FlexGapMedium)
	secFlexGap := demoSection(face, theme, "设置间隙",
		"antd gap.tsx：Radio 选 small(8)/medium(16)/large(24)/customize。customize 时出现 Slider；按钮行 wrap，窄窗不溢出。",
		gapOuter.Node())

	// ---------- wrap.tsx ----------
	wrapKids := make([]core.Node, 0, 24)
	for i := 0; i < 24; i++ {
		wrapKids = append(wrapKids, mkPrimaryBtn("Button").Node())
	}
	flexWrap := kit.NewFlex(wrapKids...)
	flexWrap.SetWrap(true)
	flexWrap.SetGapSize(kit.FlexGapSmall)
	// ExpandWidth host passes bounded MaxWidth so wrap actually engages (antd 100% row).
	wrapHost := playground(flexWrap.Node(), 0, render.RGBA{R: 0, G: 0, B: 0, A: 0.06})
	wrapHost.Padding = primitive.All(8)
	wrapHost.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
	secFlexWrap := demoSection(face, theme, "自动换行",
		"antd wrap.tsx：wrap + gap=small，24 个 Primary Button。宿主宽度随内容区收缩；窗口变窄时自动换行，不横向撑破父级。",
		wrapHost)

	// ---------- combination.tsx ----------
	// antd combination.tsx (Card 620, body pad 0):
	//   Flex justify=space-between
	//     img width 273 (photo ~square → height ≈ 180–273)
	//     Flex vertical align=end justify=space-between style={{padding:32}}
	//       Title level 3
	//       Button "Get Started"
	//
	// Critical for "Get Started" at bottom-right with a large gap:
	// the right column must be TALLER than title+button. antd gets that from
	// the image height; if title wraps taller than the image, space-between
	// has zero free space and the button sticks under the title.
	const comboImgW, comboImgH = 273.0, 273.0
	flexImg := primitive.NewDecorated(nil)
	flexImg.Width = comboImgW
	flexImg.Height = comboImgH
	flexImg.MinWidth = comboImgW
	flexImg.Background = render.Hex("#1677ff")

	quote := kit.NewTitle("“antd is an enterprise-class UI design language and React UI library.”", 3)
	quote.SetFace(face)
	quote.SetEllipsisRows(3)
	// Cap title width so level-3 wraps like antd (~ right column − pad).
	quoteBox := primitive.NewDecorated(quote.Node())
	quoteBox.Width = 250

	getStarted := mkPrimaryBtn("Get Started")
	comboInner := kit.NewFlex(quoteBox, getStarted.Node())
	comboInner.SetVertical(true)
	comboInner.SetAlign(kit.FlexAlignEnd)              // items to the right
	comboInner.SetJustify(kit.FlexJustifySpaceBetween) // title top, button bottom

	// Definite height = image height so StretchChild gives the inner flex a
	// tight height and space-between creates the large vertical gap.
	innerPad := primitive.NewDecorated(comboInner.Node())
	innerPad.Padding = primitive.All(32)
	innerPad.Width = 620 - comboImgW // 347
	innerPad.Height = comboImgH
	innerPad.StretchChild = true

	comboRow := kit.NewFlex(flexImg, innerPad)
	comboRow.SetJustify(kit.FlexJustifyStart)
	comboRow.SetAlign(kit.FlexAlignStart) // both panes fixed size; no stretch fight
	comboRow.SetWrap(false)
	comboRow.SetGap(0)

	cardShell := primitive.NewDecorated(comboRow.Node())
	cardShell.Width = 620
	cardShell.Radius = theme.SizeOr(core.TokenBorderRadiusLG, 8)
	cardShell.BorderWidth = 1
	cardShell.BorderColor = theme.Color(core.TokenColorBorder)
	cardShell.Background = theme.Color(core.TokenColorBgContainer)
	cardShell.Padding = primitive.EdgeInsets{}
	secFlexCombo := demoSection(face, theme, "组合使用",
		"antd combination.tsx：左图 273×273；右栏宽 347、高与图相同、pad 32；vertical + align=end + space-between → 标题右上、Get Started 右下且中间大间距。",
		cardShell)

	items = append(items, ctlTab("flex", "Flex"))
	contents["flex"] = demoPage(face, "Flex",
		"用于对齐的弹性布局容器。示例对齐 antd 6.5 官方非 debug demo。交互改 props 后立即重排；宿主随窗口宽度收缩，子项 wrap/均分，不溢出父容器。",
		secFlexBasic, secFlexAlign, secFlexGap, secFlexWrap, secFlexCombo,
	)

	g := kit.NewGridCols(3,
		kit.NewText("1").Node(), kit.NewText("2").Node(), kit.NewText("3").Node(),
		kit.NewText("4").Node(), kit.NewText("5").Node(), kit.NewText("6").Node(),
	)
	add("grid", "Grid", "Layout · Grid 3-col", g.Node())

	lay := kit.NewLayout(
		kit.NewText("Header").Node(),
		kit.NewText("Sider").Node(),
		kit.NewText("Content").Node(),
		kit.NewText("Footer").Node(),
	)
	add("layout", "Layout", "Layout · Layout shell", lay.Node())

	// Space — size / vertical / wrap
	spH := kit.NewSpace(
		kit.NewTag("Item 1").Node(),
		kit.NewTag("Item 2").Node(),
		kit.NewTag("Item 3").Node(),
	)
	spH.SetSize(16)

	spV := kit.NewSpace(
		kit.NewTag("Top").Node(),
		kit.NewTag("Middle").Node(),
		kit.NewTag("Bottom").Node(),
	)
	spV.SetSize(8)
	spV.SetVertical(true)

	spWrap := kit.NewSpace()
	spWrap.SetSize(8)
	spWrap.SetWrap(true)
	for _, lab := range []string{"Tag", "Tag", "Tag", "Tag", "Tag", "Tag", "Tag"} {
		spWrap.Add(kit.NewTag(lab).Node())
	}
	spWrapHost := primitive.NewDecorated(spWrap.Node())
	spWrapHost.Width = 220
	spWrapHost.Padding = primitive.All(8)
	spWrapHost.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	spWrapHost.Radius = 6

	items = append(items, ctlTab("space", "Space"))
	contents["space"] = demoPage(face, "Space",
		"Ant Design Space: uniform gap, direction, wrap.",
		demoSection(face, theme, "Horizontal", "size=16 gap between children.", spH.Node()),
		demoSection(face, theme, "Vertical", "SetVertical(true).", spV.Node()),
		demoSection(face, theme, "Wrap", "SetWrap(true) in a narrow host (width 220).", spWrapHost),
	)

	split := kit.NewSplitter(
		kit.NewText("Left pane").Node(),
		kit.NewText("Right pane").Node(),
	)
	add("splitter", "Splitter", "Layout · Splitter", split.Node())

	// ─── Navigation ────────────────────────────────────────────
	items = append(items, catHeader("Navigation"), catDivider())

	anchor := kit.NewAnchor("#Intro", "#API", "#FAQ")
	anchor.SetFace(face)
	add("anchor", "Anchor", "Navigation · Anchor", anchor.Node())

	bc := kit.NewBreadcrumb("Home", "Nav", "Page")
	bc.SetFace(face)
	add("breadcrumb", "Breadcrumb", "Navigation · Breadcrumb", bc.Node())

	dd := kit.NewDropdown("Dropdown", kit.MenuItem{Key: "1", Label: "One"}, kit.MenuItem{Key: "2", Label: "Two"})
	add("dropdown", "Dropdown", "Navigation · Dropdown", dd.Node())

	menu := kit.NewMenu(
		kit.MenuItem{Key: "a", Label: "Item A"},
		kit.MenuItem{Key: "b", Label: "Item B"},
		kit.MenuItem{Key: "c", Label: "Item C"},
	)
	menu.Face = face
	add("menu", "Menu", "Navigation · Menu", menu.Node())

	pag := kit.NewPagination(5)
	add("pagination", "Pagination", "Navigation · Pagination", pag.Node())

	steps := kit.NewSteps("Login", "Order", "Done")
	steps.SetFace(face)
	steps.SetCurrent(1)
	add("steps", "Steps", "Navigation · Steps", steps.Node())

	tabsMini := kit.NewTabs(
		kit.MenuItem{Key: "t1", Label: "Tab1"},
		kit.MenuItem{Key: "t2", Label: "Tab2"},
		kit.MenuItem{Key: "t3", Label: "Tab3"},
		kit.MenuItem{Key: "t4", Label: "Tab4"},
		kit.MenuItem{Key: "t5", Label: "Tab5"},
		kit.MenuItem{Key: "t6", Label: "Tab6"},
		kit.MenuItem{Key: "t7", Label: "Tab7"},
		kit.MenuItem{Key: "t8", Label: "Tab8"},
	)
	tabsMini.Face = face
	tabsMini.SetPosition(kit.TabLeft)
	tabsMini.TabWidth = 100
	tabsMini.TabItemHeight = 36
	for _, k := range []string{"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8"} {
		tx := kit.NewText("content " + k)
		tx.SetFace(face)
		tabsMini.SetContent(k, tx.Node())
	}
	tabsHost := primitive.NewBox(tabsMini.Node())
	tabsHost.Width, tabsHost.Height = 480, 220
	add("tabs", "Tabs", "Navigation · Tabs (left rail scroll)", tabsHost)

	// ─── Data Entry ────────────────────────────────────────────
	items = append(items, catHeader("Data Entry"), catDivider())

	ac := kit.NewAutoComplete("AutoComplete", "Apple", "Banana", "Cherry")
	ac.SetFace(face)
	add("auto_complete", "AutoComplete", "Data Entry · AutoComplete", ac.Node())

	casc := kit.NewCascader(&kit.TreeNode{
		Key: "z", Title: "Zhejiang",
		Children: []*kit.TreeNode{{Key: "hz", Title: "Hangzhou"}},
	})
	add("cascader", "Cascader", "Data Entry · Cascader", casc.Node())

	cb := kit.NewCheckbox("Checkbox")
	cb.SetFace(face)
	add("checkbox", "Checkbox", "Data Entry · Checkbox", cb.Node())

	cp := kit.NewColorPicker()
	add("color_picker", "ColorPicker", "Data Entry · ColorPicker", cp.Node())

	dp := kit.NewDatePicker()
	dp.SetFace(face)
	add("date_picker", "DatePicker", "Data Entry · DatePicker", dp.Node())

	fm := core.NewFormModel()
	form := kit.NewForm(fm)
	form.AddItem(kit.NewFormItem("name", "Name", kit.NewInput("name").Node()))
	add("form", "Form", "Data Entry · Form", form.Node())

	in := kit.NewInput("Input")
	in.SetFace(face)
	in.SetFixedSize(240, 32)
	*tickers = append(*tickers, in)
	add("input", "Input", "Data Entry · Input", in.Node())

	num := kit.NewInputNumber(1)
	num.SetFace(face)
	add("input_number", "InputNumber", "Data Entry · InputNumber", num.Node())

	ment := kit.NewMentions("@user", "alice", "bob")
	ment.SetFace(face)
	add("mentions", "Mentions", "Data Entry · Mentions", ment.Node())

	ra := kit.NewRadio("x", "Radio A")
	rb := kit.NewRadio("y", "Radio B")
	ra.SetFace(face)
	rb.SetFace(face)
	rg := kit.NewRadioGroup(ra, rb)
	rg.Select("x")
	add("radio", "Radio", "Data Entry · Radio / RadioGroup", rg.Node())

	rate := kit.NewRate(3)
	rate.SetFace(face)
	add("rate", "Rate", "Data Entry · Rate", rate.Node())

	sel := kit.NewSelect("Select", kit.SelectOption{Value: "1", Label: "One"}, kit.SelectOption{Value: "2", Label: "Two"})
	sel.SetFace(face)
	add("select", "Select", "Data Entry · Select", sel.Node())

	slider := kit.NewSlider(40)
	add("slider", "Slider", "Data Entry · Slider", slider.Node())

	// Switch — docs/antd/switch.md §6.8 P0
	// https://ant.design/components/switch
	trackSw := func(s *kit.Switch) *kit.Switch {
		*tickers = append(*tickers, s)
		return s
	}
	swBasic := trackSw(kit.NewSwitch())
	swBasic.SetDefaultChecked(true)
	swBasic.SetOnChange(func(v bool) { *status = fmt.Sprintf("switch → %v", v) })
	swBasic.SetAriaLabel("basic")

	swDis := trackSw(kit.NewSwitch())
	swDis.SetDefaultChecked(true)
	swDis.SetDisabled(true)
	swDisToggle := kit.NewButton("Toggle disabled")
	swDisToggle.SetType(kit.ButtonPrimary)
	swDisToggle.SetOnClick(func() {
		swDis.SetDisabled(!swDis.Disabled)
		if swDis.Disabled {
			*status = "switch disabled"
		} else {
			*status = "switch enabled"
		}
	})
	*buttons = append(*buttons, swDisToggle)

	swText1 := trackSw(kit.NewSwitch())
	swText1.SetFace(face)
	swText1.SetCheckedChildren("On")
	swText1.SetUnCheckedChildren("Off")
	swText1.SetDefaultChecked(true)
	swText2 := trackSw(kit.NewSwitch())
	swText2.SetFace(face)
	swText2.SetCheckedChildren("1")
	swText2.SetUnCheckedChildren("0")
	swText2.SetDefaultChecked(true)

	swMed := trackSw(kit.NewSwitch())
	swMed.SetDefaultChecked(true)
	swSm := trackSw(kit.NewSwitch())
	swSm.SetSize(kit.SwitchSmall)
	swSm.SetDefaultChecked(true)

	swLoad1 := trackSw(kit.NewSwitch())
	swLoad1.SetLoading(true)
	swLoad1.SetDefaultChecked(true)
	swLoad2 := trackSw(kit.NewSwitch())
	swLoad2.SetSize(kit.SwitchSmall)
	swLoad2.SetLoading(true)

	swCtrl := trackSw(kit.NewSwitch())
	swCtrl.SetControlled(true)
	swCtrl.SetChecked(false)
	swCtrl.SetOnChange(func(v bool) {
		// Parent applies value (controlled demo).
		swCtrl.SetChecked(v)
		*status = fmt.Sprintf("controlled → %v", v)
	})
	swCtrl.SetAriaLabel("controlled")

	secSwBasic := demoSection(face, theme, "Basic",
		"The most basic usage.",
		spaceWrap(12, swBasic.Node()))
	secSwDis := demoSection(face, theme, "Disabled",
		"Disabled state of Switch.",
		spaceWrap(12, swDis.Node(), swDisToggle.Node()))
	secSwText := demoSection(face, theme, "Text & icon",
		"With text checkedChildren / unCheckedChildren (string).",
		spaceWrap(12, swText1.Node(), swText2.Node()))
	secSwSize := demoSection(face, theme, "Two sizes",
		"size=medium (default) and size=small.",
		spaceWrap(12, swMed.Node(), swSm.Node()))
	secSwLoad := demoSection(face, theme, "Loading",
		"Mark a pending state of switch.",
		spaceWrap(12, swLoad1.Node(), swLoad2.Node()))
	secSwCtrl := demoSection(face, theme, "Controlled",
		"SetControlled + SetChecked: parent owns the value.",
		spaceWrap(12, swCtrl.Node()))
	add("switch", "Switch", "Data Entry · Switch",
		demoPage(face, "Switch",
			"Switching Selector. P0: checked/value, defaultChecked, controlled, onChange/onClick, disabled, loading, size, children text.",
			secSwBasic, secSwDis, secSwText, secSwSize, secSwLoad, secSwCtrl))

	ta := kit.NewTextArea("TextArea", 3)
	add("textarea", "TextArea", "Data Entry · TextArea", ta.Node())

	tp := kit.NewTimePicker()
	tp.SetFace(face)
	add("time_picker", "TimePicker", "Data Entry · TimePicker", tp.Node())

	tr := kit.NewTransfer([]string{"A", "B", "C"})
	add("transfer", "Transfer", "Data Entry · Transfer", tr.Node())

	ts := kit.NewTreeSelect("TreeSelect", "a/b", "a/c")
	add("tree_select", "TreeSelect", "Data Entry · TreeSelect", ts.Node())

	up := kit.NewUpload("Upload")
	up.SetFace(face)
	add("upload", "Upload", "Data Entry · Upload", up.Node())

	// ─── Data Display ──────────────────────────────────────────
	items = append(items, catHeader("Data Display"), catDivider())

	av := kit.NewAvatar("UI")
	av.SetFace(face)
	add("avatar", "Avatar", "Data Display · Avatar", av.Node())

	badge := kit.NewBadge(kit.NewButton("msg").Node(), 8)
	add("badge", "Badge", "Data Display · Badge", badge.Node())

	cal := kit.NewCalendar(2026, 7)
	cal.SetFace(face)
	add("calendar", "Calendar", "Data Display · Calendar", cal.Node())

	card := kit.NewCard("Card")
	card.SetFace(face)
	card.SetContent(kit.NewText("body").Node())
	add("card", "Card", "Data Display · Card", card.Node())

	carousel := kit.NewCarousel(kit.NewText("Slide A").Node(), kit.NewText("Slide B").Node())
	add("carousel", "Carousel", "Data Display · Carousel", carousel.Node())

	collapse := kit.NewCollapse(
		kit.CollapsePanel{Key: "1", Header: "Panel 1", Content: kit.NewText("body 1").Node()},
		kit.CollapsePanel{Key: "2", Header: "Panel 2", Content: kit.NewText("body 2").Node()},
	)
	collapse.SetFace(face)
	collapse.SetActive("1")
	add("collapse", "Collapse", "Data Display · Collapse", collapse.Node())

	desc := kit.NewDescriptions([2]string{"Name", "Ada"}, [2]string{"City", "London"})
	desc.SetFace(face)
	add("descriptions", "Descriptions", "Data Display · Descriptions", desc.Node())

	empty := kit.NewEmpty("No data")
	empty.SetFace(face)
	add("empty", "Empty", "Data Display · Empty", empty.Node())

	img := kit.NewImage("Image", 120, 72)
	add("image", "Image", "Data Display · Image", img.Node())

	list := kit.NewList("Alpha", "Beta", "Gamma")
	add("list", "List", "Data Display · List", list.Node())

	pop := kit.NewPopover(kit.NewButton("Popover").Node(), kit.NewText("popover body").Node())
	add("popover", "Popover", "Data Display · Popover", pop.Node())

	qr := kit.NewQRCode("gpui")
	add("qrcode", "QRCode", "Data Display · QRCode", qr.Node())

	seg := kit.NewSegmented("Daily", "Weekly")
	seg.SetFace(face)
	add("segmented", "Segmented", "Data Display · Segmented", seg.Node())

	stat := kit.NewStatistic("Users", "1,024")
	stat.SetFace(face)
	add("statistic", "Statistic", "Data Display · Statistic", stat.Node())

	table := kit.NewTable(
		[]kit.TableColumn{{Key: "n", Title: "Name"}, {Key: "a", Title: "Age"}},
		[]map[string]string{{"n": "Ada", "a": "36"}, {"n": "Lin", "a": "28"}},
	)
	add("table", "Table", "Data Display · Table", table.Node())

	tag := kit.NewTag("Tag")
	tag.SetFace(face)
	add("tag", "Tag", "Data Display · Tag", tag.Node())

	tl := kit.NewTimeline(kit.TimelineItem{Label: "Create"}, kit.TimelineItem{Label: "Ship"})
	tl.SetFace(face)
	add("timeline", "Timeline", "Data Display · Timeline", tl.Node())

	tooltip := kit.NewTooltip(kit.NewButton("Hover me").Node(), "Tooltip")
	add("tooltip", "Tooltip", "Data Display · Tooltip", tooltip.Node())

	tour := kit.NewTour(kit.TourStep{Title: "Step1", Body: "demo"})
	add("tour", "Tour", "Data Display · Tour", tour.Node())

	tree := kit.NewTree(&kit.TreeNode{Key: "r", Title: "root", Children: []*kit.TreeNode{{Key: "c", Title: "child"}}})
	add("tree", "Tree", "Data Display · Tree", tree.Node())

	// ─── Feedback ──────────────────────────────────────────────
	items = append(items, catHeader("Feedback"), catDivider())

	al := kit.NewAlert("Alert message")
	al.SetFace(face)
	al.SetType("warning")
	add("alert", "Alert", "Feedback · Alert", al.Node())

	drawer := kit.NewDrawer("Drawer")
	drawer.Face = face
	drawer.SetContent(kit.NewText("drawer body").Node())
	openD := kit.NewButton("Open Drawer")
	openD.SetFace(face)
	openD.SetOnClick(func() { drawer.SetOpen(true); *status = "drawer open" })
	*buttons = append(*buttons, openD)
	add("drawer", "Drawer", "Feedback · Drawer", openD.Node(), drawer.Node())

	if msgHost == nil {
		msgHost = kit.NewMessageHost()
	}
	msgBtn := kit.NewButton("Info message")
	msgBtn.SetFace(face)
	msgBtn.SetOnClick(func() { msgHost.Info("hello"); *status = "message" })
	*buttons = append(*buttons, msgBtn)
	// Portal is mounted at app root (main.go); only the trigger lives in this tab.
	add("message", "Message", "Feedback · Message", msgBtn.Node())

	modal = kit.NewModal("Confirm")
	modal.SetFace(face)
	modal.SetContent(kit.NewText("Modal body").Node())
	modal.Viewport = core.Size{Width: 1024, Height: 768}
	modal.OnOk = func() { *status = "modal ok"; modal.SetOpen(false) }
	modal.OnCancel = func() { *status = "modal cancel"; modal.SetOpen(false) }
	openM := kit.NewButton("Open Modal")
	openM.SetFace(face)
	openM.SetOnClick(func() { modal.SetOpen(true); *status = "modal open" })
	*buttons = append(*buttons, openM)
	add("modal", "Modal", "Feedback · Modal", openM.Node(), modal.Node())

	// Notification uses the same app-level MessageHost (Ant notification queue).
	nBtn := kit.NewButton("Notify")
	nBtn.SetFace(face)
	nBtn.SetOnClick(func() { msgHost.Notification("Title", "body"); *status = "notify" })
	*buttons = append(*buttons, nBtn)
	add("notification", "Notification", "Feedback · Notification", nBtn.Node())

	pcTrig := kit.NewButton("Popconfirm")
	pcTrig.SetFace(face)
	pc := kit.NewPopconfirm(pcTrig.Node(), "Are you sure?")
	pc.SetFace(face)
	*buttons = append(*buttons, pcTrig)
	add("popconfirm", "Popconfirm", "Feedback · Popconfirm", pc.Node())

	prog := kit.NewProgress(60)
	prog.Width = 280
	prog.ShowInfo = true
	add("progress", "Progress", "Feedback · Progress", prog.Node())

	res := kit.NewResult("success", "Success", "All good")
	res.SetFace(face)
	add("result", "Result", "Feedback · Result", res.Node())

	sk := kit.NewSkeleton(200, 16)
	sk.SetActive(true)
	*tickers = append(*tickers, sk)
	add("skeleton", "Skeleton", "Feedback · Skeleton", sk.Node())

	spin := kit.NewSpin(nil)
	*tickers = append(*tickers, spin)
	add("spin", "Spin", "Feedback · Spin", spin.Node())

	wm := kit.NewWatermark(kit.NewText("content").Node(), "WM")
	add("watermark", "Watermark", "Feedback · Watermark", wm.Node())

	// ─── Other ─────────────────────────────────────────────────
	items = append(items, catHeader("Other"), catDivider())

	affix := kit.NewAffix(kit.NewText("Affix/Sticky").Node())
	add("affix", "Affix", "Other · Affix", affix.Node())

	// App = theme density demo
	add("app", "App", "Other · App (Theme density)",
		sec(face, "DefaultTheme + ApplyDensity"),
		kit.NewText(fmt.Sprintf("density=%s", theme.Density)).Node(),
	)

	cfg := kit.NewConfigProvider(theme, kit.NewText("ConfigProvider child").Node())
	add("config_provider", "ConfigProvider", "Other · ConfigProvider", cfg.Node())

	// ─── Scroll + Scrollbar (standalone policy control) ─────────
	// Scroll is the overflow container; Scrollbar is the chrome policy enabled on it.
	mkLines := func(prefix string, n int) core.Node {
		inner := primitive.Column()
		inner.Gap = 2
		for i := 0; i < n; i++ {
			line := kit.NewText(fmt.Sprintf("%s · line %02d — wheel / drag thumb / track click", prefix, i+1))
			line.SetFace(face)
			inner.AddChild(line.Node())
		}
		return inner
	}

	// Interactive live panel: switch visibility policy at runtime.
	live := kit.NewScroll(mkLines("live", 20))
	live.SetSize(420, 140)
	live.SetScrollbar(primitive.DefaultScrollbar()) // Auto + non-overlap (default)
	*tickers = append(*tickers, live)

	policyLab := kit.NewText("policy: Auto — 溢出显示，内容区已减去条宽")
	policyLab.SetFace(face)
	policyLab.SetSecondary(true)

	setPolicy := func(name string, vis primitive.ScrollbarVisibility, overlay bool) {
		b := primitive.DefaultScrollbar()
		b.Vertical = vis
		b.Horizontal = vis
		b.Enabled = vis != primitive.ScrollbarNever
		b.Overlay = overlay
		b.Thickness = 6
		b.HoverThickness = 10
		b.AutoHideDelay = 1.2
		live.SetScrollbar(b)
		*status = "scrollbar " + name
		switch vis {
		case primitive.ScrollbarHover:
			policyLab.SetValue("policy: Hover — 溢出+悬停/滚轮显示；内容不重叠条")
		case primitive.ScrollbarAuto:
			policyLab.SetValue("policy: Auto — 溢出即显示；内容布局减去条宽/高")
		case primitive.ScrollbarAlways:
			policyLab.SetValue("policy: Always — 始终显示 track；内容不重叠")
		case primitive.ScrollbarNever:
			policyLab.SetValue("policy: Never — 无条；内容可用全宽")
		}
	}

	btnHover := kit.NewButton("Hover")
	btnHover.SetFace(face)
	btnHover.SetOnClick(func() { setPolicy("Hover", primitive.ScrollbarHover, false) })
	btnAuto := kit.NewButton("Auto")
	btnAuto.SetFace(face)
	btnAuto.SetType(kit.ButtonPrimary)
	btnAuto.SetOnClick(func() { setPolicy("Auto", primitive.ScrollbarAuto, false) })
	btnAlways := kit.NewButton("Always")
	btnAlways.SetFace(face)
	btnAlways.SetOnClick(func() { setPolicy("Always", primitive.ScrollbarAlways, false) })
	btnNever := kit.NewButton("Never")
	btnNever.SetFace(face)
	btnNever.SetOnClick(func() { setPolicy("Never", primitive.ScrollbarNever, false) })
	btnThick := kit.NewButton("Thick 12")
	btnThick.SetFace(face)
	btnThick.SetOnClick(func() {
		live.ConfigureScrollbar(func(b *primitive.Scrollbar) {
			b.SetThickness(12).SetHoverThickness(18).SetMinThumb(28)
		})
		*status = "scrollbar thickness=12 hover=18"
	})
	btnTrack := kit.NewButton("Track on/off")
	btnTrack.SetFace(face)
	trackOn := true
	btnTrack.SetOnClick(func() {
		trackOn = !trackOn
		live.ConfigureScrollbar(func(b *primitive.Scrollbar) { b.SetShowTrack(trackOn) })
		*status = fmt.Sprintf("showTrack=%v", trackOn)
	})
	btnHoverGrow := kit.NewButton("Hover grow")
	btnHoverGrow.SetFace(face)
	btnHoverGrow.SetOnClick(func() {
		live.ConfigureScrollbar(func(b *primitive.Scrollbar) {
			b.SetExpandOnHover(true).SetThickness(6).SetHoverThickness(14)
		})
		*status = "expand on hover 6→14"
	})
	btnColor := kit.NewButton("Blue thumb")
	btnColor.SetFace(face)
	btnColor.SetOnClick(func() {
		live.ConfigureScrollbar(func(b *primitive.Scrollbar) {
			b.SetColors(
				render.RGBA{R: 0.9, G: 0.92, B: 0.96, A: 1},
				render.RGBA{R: 0.15, G: 0.45, B: 0.95, A: 0.85},
				render.RGBA{R: 0.1, G: 0.35, B: 0.9, A: 1},
			)
		})
		*status = "custom track/thumb colors"
	})
	*buttons = append(*buttons, btnHover, btnAuto, btnAlways, btnNever, btnThick, btnTrack, btnHoverGrow, btnColor)

	// Static side-by-side samples
	sample := func(vis primitive.ScrollbarVisibility, title string) core.Node {
		sc := kit.NewScroll(mkLines(title, 12))
		sc.SetSize(200, 96)
		sc.SetScrollbarVisibility(vis)
		sc.SetOverlay(false) // content never under bar
		if vis == primitive.ScrollbarHover {
			*tickers = append(*tickers, sc)
		}
		box := primitive.Column(
			sec(face, title),
			sc.Node(),
		)
		box.Gap = 6
		return box
	}
	samples := primitive.Row(
		sample(primitive.ScrollbarHover, "Hover"),
		sample(primitive.ScrollbarAuto, "Auto"),
		sample(primitive.ScrollbarAlways, "Always"),
		sample(primitive.ScrollbarNever, "Never"),
	)
	samples.Gap = 12
	samples.CrossAlign = core.CrossStart

	// Horizontal scroll demo
	hInner := primitive.Row()
	hInner.Gap = 8
	for i := 0; i < 12; i++ {
		cell := primitive.NewBox()
		cell.Width, cell.Height = 72, 48
		cell.Color = render.RGBA{R: 0.85, G: 0.9, B: 0.98, A: 1}
		lab := kit.NewText(fmt.Sprintf("H%d", i+1))
		lab.SetFace(face)
		stack := primitive.NewStack(cell, primitive.Positioned(core.AlignCenter, lab.Node()))
		hInner.AddChild(stack)
	}
	hScroll := kit.NewScroll(hInner)
	hScroll.SetSize(420, 64)
	hScroll.SetAxis(false, true)
	hBar := primitive.DefaultScrollbar()
	hBar.Vertical = primitive.ScrollbarNever
	hBar.Horizontal = primitive.ScrollbarAuto
	hBar.Overlay = false
	hScroll.SetScrollbar(hBar)

	add("scroll", "Scroll", "Other · Scroll 容器（启用滚动）",
		sec(face, "垂直溢出 · 默认 Auto 条（内容区已减去条宽）"),
		live.Node(),
		policyLab.Node(),
		sec(face, "切换 Scrollbar 策略（独立控件配置）"),
		primitive.Row(btnHover.Node(), btnAuto.Node(), btnAlways.Node(), btnNever.Node()),
		primitive.Row(btnThick.Node(), btnTrack.Node(), btnHoverGrow.Node(), btnColor.Node()),
	)

	add("scrollbar", "Scrollbar", "Other · Scrollbar 策略对照",
		sec(face, "同一溢出内容 · 四种显示策略并排"),
		samples,
		sec(face, "水平滚动 · Horizontal=Auto"),
		hScroll.Node(),
		sec(face, "配置项: Enabled / Visibility(Never·Auto·Always·Hover) / Overlay / Thickness / HoverThickness / MinThumb / AutoHideDelay / DragThumb / TrackClick / WheelStep / Colors"),
	)

	return items, contents, modal
}
