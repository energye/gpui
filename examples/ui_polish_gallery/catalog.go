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

	bPrimary := kit.NewButton("Primary")
	bPrimary.SetType(kit.ButtonPrimary)
	bPrimary.SetFace(face)
	bDefault := kit.NewButton("Default")
	bDefault.SetType(kit.ButtonDefault)
	bDefault.SetFace(face)
	bDashed := kit.NewButton("Dashed")
	bDashed.SetType(kit.ButtonDashed)
	bDashed.SetFace(face)
	*buttons = append(*buttons, bPrimary, bDefault, bDashed)
	add("btn", "Button", "General · Button",
		sec(face, "types"),
		primitive.Row(bPrimary.Node(), bDefault.Node(), bDashed.Node()),
	)

	fb := kit.NewFloatButton("+")
	fb.SetFace(face)
	add("float_button", "FloatButton", "General · FloatButton", fb.Node())

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

	add("flex", "Flex", "Layout · Flex",
		kit.NewFlexRow(kit.NewText("row-a").Node(), kit.NewText("row-b").Node()).Node(),
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

	sp := kit.NewSpace(kit.NewTag("A").Node(), kit.NewTag("B").Node(), kit.NewTag("C").Node())
	sp.SetSize(8)
	add("space", "Space", "Layout · Space", sp.Node())

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

	sw := kit.NewSwitch()
	*tickers = append(*tickers, sw)
	add("switch", "Switch", "Data Entry · Switch", sw.Node())

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
