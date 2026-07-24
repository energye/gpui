//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// panel wraps a single control demo for its own tab content.
func panel(face text.Face, title string, kids ...core.Node) core.Node {
	lab := kit.NewText(title)
	lab.SetFace(face)
	lab.SetSecondary(true)
	col := primitive.Column(lab.Node())
	col.Gap = 12
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStart
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
	lab.Root.FontSize = 20
	col := primitive.Column(lab.Node())
	col.Gap = 24
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch
	col.Padding = primitive.All(16)
	if desc != "" {
		d := kit.NewText(desc)
		d.SetFace(face)
		d.SetSecondary(true)
		col.AddChild(d.Node())
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
	titleT.Root.FontSize = 16
	inner := primitive.Column(titleT.Node())
	inner.Gap = 12
	inner.MainAlign = core.MainStart
	inner.CrossAlign = core.CrossStretch
	if desc != "" {
		d := kit.NewText(desc)
		d.SetFace(face)
		d.SetSecondary(true)
		inner.AddChild(d.Node())
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
	t := kit.NewText(s)
	t.SetFace(face)
	t.SetSecondary(true)
	return t.Node()
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
	// menu mode click
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
	// controlled + placement left
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
	stageInner := primitive.Column(primitive.Spacer(), primitive.Row(primitive.Spacer(), corner.Node()))
	stageInner.CrossAlign = core.CrossStretch
	stage := primitive.NewDecorated(stageInner)
	stage.Width = 280
	stage.Height = 160
	stage.Padding = primitive.All(12)
	stage.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
	stage.Radius = 8
	stage.StretchChild = true

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
		demoSection(face, theme, "Group", "no trigger — children always visible.",
			fbGroup.Node()),
		demoSection(face, theme, "Menu mode", "trigger=click; open shows children + close icon.",
			fbMenu.Node()),
		demoSection(face, theme, "Controlled + placement=left", "SetOpen(true); children to the left of trigger.",
			fbCtrl.Node()),
		demoSection(face, theme, "Placement (layout)", "Bottom-right via Column/Row spacers in a stage — not OS float.",
			stage),
	)

	ic := kit.NewIcon("star")
	add("icon", "Icon", "General · Icon", ic.Node())

	ty := kit.NewTitle("Title H3", 3)
	ty.SetFace(face)
	pg := kit.NewParagraph("Typography paragraph — Ant General.")
	pg.SetFace(face)
	tx := kit.NewText("Text")
	tx.SetFace(face)
	add("typography", "Typography", "General · Typography (Text / Title / Paragraph)",
		tx.Node(), ty.Node(), pg.Node(),
	)

	// ─── Layout ────────────────────────────────────────────────
	items = append(items, catHeader("Layout"), catDivider())

	div := kit.NewDivider()
	add("divider", "Divider", "Layout · Divider", div.Node())

	// Flex — gap / wrap / grow / shrink (Ant Flex demos)
	box := func(label string, w, h float64, col render.RGBA) core.Node {
		tx := kit.NewText(label)
		tx.SetFace(face)
		inner := primitive.NewDecorated(tx.Node())
		inner.Padding = primitive.Symmetric(8, 4)
		inner.Background = col
		inner.Radius = 4
		host := primitive.NewBox(inner)
		if w > 0 {
			host.Width = w
		}
		if h > 0 {
			host.Height = h
		}
		return host
	}
	blue := render.RGBA{R: 0.09, G: 0.47, B: 1, A: 0.25}
	green := render.RGBA{R: 0.32, G: 0.77, B: 0.10, A: 0.25}
	orange := render.RGBA{R: 0.98, G: 0.68, B: 0.08, A: 0.35}
	gray := render.RGBA{R: 0, G: 0, B: 0, A: 0.06}

	flexGap := kit.NewFlexRow(
		box("A", 64, 32, blue),
		box("B", 64, 32, green),
		box("C", 64, 32, orange),
	)
	flexGap.SetGap(16)

	// Wrap: many fixed boxes in a bounded width host
	wrapKids := make([]core.Node, 0, 8)
	for i, lab := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
		_ = i
		wrapKids = append(wrapKids, box(lab, 72, 28, blue))
	}
	flexWrap := kit.NewFlexRow(wrapKids...)
	flexWrap.SetGap(8)
	flexWrap.SetWrap(true)
	wrapHost := primitive.NewDecorated(flexWrap.Node())
	wrapHost.Width = 280
	wrapHost.Padding = primitive.All(8)
	wrapHost.Background = gray
	wrapHost.Radius = 6

	// Grow: two Flexible share remaining width
	left := primitive.NewFlexible(1, box("grow=1", 0, 32, blue))
	left.FillChild = true
	right := primitive.NewFlexible(2, box("grow=2", 0, 32, green))
	right.FillChild = true
	growRow := primitive.Row(box("fixed", 56, 32, orange), left, right)
	growRow.Gap = 8
	growHost := primitive.NewDecorated(growRow)
	growHost.Width = 400
	growHost.Padding = primitive.All(8)
	growHost.Background = gray
	growHost.Radius = 6
	growHost.StretchChild = true

	// Shrink demos.
	// IMPORTANT: fixed-width playgrounds must NOT sit under CrossStretch alone, or
	// parent tight MinWidth forces the host to full page width while children still
	// layout at host.Width — looks like "blocks never change". Wrap with CrossStart.
	alignStart := func(n core.Node) core.Node {
		r := primitive.Row(n)
		r.CrossAlign = core.CrossStart
		return r
	}

	// Prefer width via Decorated MinWidth so measure is large before shrink.
	wide := func(label string, shrink float64, minW float64, col render.RGBA) core.Node {
		tx := kit.NewText(label)
		tx.SetFace(face)
		d := primitive.NewDecorated(tx.Node())
		d.Padding = primitive.Symmetric(12, 6)
		d.Background = col
		d.Radius = 4
		d.MinWidth = minW
		f := primitive.NewFlexible(0, d)
		f.Shrink = shrink
		f.FillChild = true
		return f
	}

	// Fixed playground: host width 240, two blocks want 160 each → shrink to ~108.
	shrinkRow := primitive.Row(wide("shrink=1 · A", 1, 160, blue), wide("shrink=1 · B", 1, 160, green))
	shrinkRow.Gap = 8
	shrinkHost := primitive.NewDecorated(shrinkRow)
	shrinkHost.Width = 240
	shrinkHost.Padding = primitive.All(8)
	shrinkHost.Background = gray
	shrinkHost.Radius = 6
	shrinkHost.StretchChild = true

	// shrink=0 vs shrink=1 in fixed 240 host
	shrink0Row := primitive.Row(wide("shrink=0", 0, 160, orange), wide("shrink=1", 1, 160, blue))
	shrink0Row.Gap = 8
	shrink0Host := primitive.NewDecorated(shrink0Row)
	shrink0Host.Width = 240
	shrink0Host.Padding = primitive.All(8)
	shrink0Host.Background = gray
	shrink0Host.Radius = 6
	shrink0Host.StretchChild = true

	// Responsive: host expands to content width (no fixed Width). Blocks prefer 220 each.
	// Narrow the window / content column — when max width < 220+8+220, both shrink.
	respRow := primitive.Row(wide("want 280 · shrink=1", 1, 280, blue), wide("want 280 · shrink=1", 1, 280, green))
	respRow.Gap = 8
	respHost := primitive.NewDecorated(respRow)
	respHost.ExpandWidth = true // follow parent max width
	respHost.Padding = primitive.All(8)
	respHost.Background = gray
	respHost.Radius = 6
	respHost.StretchChild = true

	items = append(items, ctlTab("flex", "Flex"))
	contents["flex"] = demoPage(face, "Flex",
		"Ant Design Flex: gap, wrap, grow, shrink. Blue/green blocks are the flex items that shrink — gray is only the host.",
		demoSection(face, theme, "Basic / Gap", "Horizontal row with gap.", flexGap.Node()),
		demoSection(face, theme, "Wrap", "Children wrap when main axis is bounded (width 280).", alignStart(wrapHost)),
		demoSection(face, theme, "Grow", "Flexible grow factors share free space (1 : 2).", alignStart(growHost)),
		demoSection(face, theme, "Shrink (fixed host 240)",
			"Gray host is fixed 240px wide. Two blocks want 160 each → FlexShrink packs them to ~108. The colored blocks shrink, not the gray chrome.",
			alignStart(shrinkHost)),
		demoSection(face, theme, "Shrink = 0",
			"Left shrink=0 keeps ~160; right shrink=1 absorbs the deficit.",
			alignStart(shrink0Host)),
		demoSection(face, theme, "Shrink with window",
			"Gray host fills the content width. Each block wants 280px. Narrow the window — when content area < ~568px the colored blocks compress (not the gray alone).",
			respHost),
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
