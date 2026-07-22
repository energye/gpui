//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// panel builds a scrollable Ant-category column for the gallery.
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
	// Many controls → body scroll via Tabs bodyScroll.
	return col
}

func sec(face text.Face, s string) core.Node {
	t := kit.NewText(s)
	t.SetFace(face)
	t.SetSecondary(true)
	return t.Node()
}

// buildCatalogPanels returns (keys, labels, panels) for left Tabs categories.
func buildCatalogPanels(face text.Face, theme *core.Theme, status *string, buttons *[]*kit.Button, tickers *[]interface{ AttachTicker(*core.Tree) }) (
	items []kit.MenuItem, contents map[string]core.Node, modal *kit.Modal,
) {
	contents = make(map[string]core.Node)

	// --- General ---
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
	fb := kit.NewFloatButton("+")
	fb.SetFace(face)
	ty := kit.NewTitle("Title H3", 3)
	ty.SetFace(face)
	pg := kit.NewParagraph("Typography paragraph — Ant General.")
	pg.SetFace(face)
	ic := kit.NewIcon("star")
	contents["general"] = panel(face, "General · Button / FloatButton / Icon / Typography",
		sec(face, "Button"),
		primitive.Row(bPrimary.Node(), bDefault.Node(), bDashed.Node()),
		sec(face, "FloatButton"),
		fb.Node(),
		sec(face, "Icon"),
		ic.Node(),
		sec(face, "Typography"),
		ty.Node(),
		pg.Node(),
	)

	// --- Layout ---
	sp := kit.NewSpace(kit.NewTag("A").Node(), kit.NewTag("B").Node(), kit.NewTag("C").Node())
	sp.SetSize(8)
	div := kit.NewDivider()
	g := kit.NewGridCols(3,
		kit.NewText("1").Node(), kit.NewText("2").Node(), kit.NewText("3").Node(),
		kit.NewText("4").Node(), kit.NewText("5").Node(), kit.NewText("6").Node(),
	)
	split := kit.NewSplitter(
		kit.NewText("Left pane").Node(),
		kit.NewText("Right pane").Node(),
	)
	lay := kit.NewLayout(
		kit.NewText("Header").Node(),
		kit.NewText("Sider").Node(),
		kit.NewText("Content").Node(),
		kit.NewText("Footer").Node(),
	)
	contents["layout"] = panel(face, "Layout · Divider / Flex / Grid / Layout / Space / Splitter",
		sec(face, "Space"), sp.Node(),
		sec(face, "Divider"), div.Node(),
		sec(face, "Flex"), kit.NewFlexRow(kit.NewText("row-a").Node(), kit.NewText("row-b").Node()).Node(),
		sec(face, "Grid 3-col"), g.Node(),
		sec(face, "Splitter"), split.Node(),
		sec(face, "Layout shell"), lay.Node(),
	)

	// --- Navigation ---
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
	// fixed height host so left rail scrolls
	tabsHost := primitive.NewBox(tabsMini.Node())
	tabsHost.Width, tabsHost.Height = 480, 220
	menu := kit.NewMenu(
		kit.MenuItem{Key: "a", Label: "Item A"},
		kit.MenuItem{Key: "b", Label: "Item B"},
		kit.MenuItem{Key: "c", Label: "Item C"},
	)
	menu.Face = face
	pag := kit.NewPagination(5)
	dd := kit.NewDropdown("Dropdown", kit.MenuItem{Key: "1", Label: "One"}, kit.MenuItem{Key: "2", Label: "Two"})
	bc := kit.NewBreadcrumb("Home", "Nav", "Page")
	bc.SetFace(face)
	steps := kit.NewSteps("Login", "Order", "Done")
	steps.SetFace(face)
	steps.SetCurrent(1)
	anchor := kit.NewAnchor("#Intro", "#API", "#FAQ")
	anchor.SetFace(face)
	contents["nav"] = panel(face, "Navigation · Anchor / Breadcrumb / Dropdown / Menu / Pagination / Steps / Tabs(scroll)",
		sec(face, "Breadcrumb"), bc.Node(),
		sec(face, "Steps"), steps.Node(),
		sec(face, "Anchor"), anchor.Node(),
		sec(face, "Menu"), menu.Node(),
		sec(face, "Pagination"), pag.Node(),
		sec(face, "Dropdown"), dd.Node(),
		sec(face, "Tabs left rail scroll (many items)"), tabsHost,
	)

	// --- Data Entry ---
	in := kit.NewInput("Input")
	in.SetFace(face)
	in.SetFixedSize(240, 32)
	*tickers = append(*tickers, in)
	ta := kit.NewTextArea("TextArea", 3)
	num := kit.NewInputNumber(1)
	num.SetFace(face)
	sw := kit.NewSwitch()
	*tickers = append(*tickers, sw)
	cb := kit.NewCheckbox("Checkbox")
	cb.SetFace(face)
	ra := kit.NewRadio("x", "Radio A")
	rb := kit.NewRadio("y", "Radio B")
	ra.SetFace(face)
	rb.SetFace(face)
	rg := kit.NewRadioGroup(ra, rb)
	rg.Select("x")
	sel := kit.NewSelect("Select", kit.SelectOption{Value: "1", Label: "One"}, kit.SelectOption{Value: "2", Label: "Two"})
	sel.SetFace(face)
	rate := kit.NewRate(3)
	rate.SetFace(face)
	seg := kit.NewSegmented("Daily", "Weekly")
	seg.SetFace(face)
	slider := kit.NewSlider(40)
	ac := kit.NewAutoComplete("AutoComplete", "Apple", "Banana", "Cherry")
	ac.SetFace(face)
	ment := kit.NewMentions("@user", "alice", "bob")
	ment.SetFace(face)
	dp := kit.NewDatePicker()
	dp.SetFace(face)
	tp := kit.NewTimePicker()
	tp.SetFace(face)
	cp := kit.NewColorPicker()
	up := kit.NewUpload("Upload")
	up.SetFace(face)
	ts := kit.NewTreeSelect("TreeSelect", "a/b", "a/c")
	// Form
	fm := core.NewFormModel()
	form := kit.NewForm(fm)
	fi := kit.NewFormItem("name", "Name", kit.NewInput("name").Node())
	form.AddItem(fi)
	contents["entry"] = panel(face, "Data Entry",
		sec(face, "Input / TextArea / InputNumber"),
		in.Node(), ta.Node(), num.Node(),
		sec(face, "Checkbox / Radio / Switch"),
		cb.Node(), rg.Node(), sw.Node(),
		sec(face, "Select / Rate / Segmented / Slider"),
		sel.Node(), rate.Node(), seg.Node(), slider.Node(),
		sec(face, "AutoComplete / Mentions / Date / Time / Color / Upload / TreeSelect / Form"),
		ac.Node(), ment.Node(), dp.Node(), tp.Node(), cp.Node(), up.Node(), ts.Node(), form.Node(),
	)

	// --- Data Display ---
	tag := kit.NewTag("Tag")
	tag.SetFace(face)
	av := kit.NewAvatar("UI")
	av.SetFace(face)
	badge := kit.NewBadge(kit.NewButton("msg").Node(), 8)
	card := kit.NewCard("Card")
	card.SetFace(face)
	card.SetContent(kit.NewText("body").Node())
	empty := kit.NewEmpty("No data")
	empty.SetFace(face)
	list := kit.NewList("Alpha", "Beta", "Gamma")
	table := kit.NewTable(
		[]kit.TableColumn{{Key: "n", Title: "Name"}, {Key: "a", Title: "Age"}},
		[]map[string]string{{"n": "Ada", "a": "36"}, {"n": "Lin", "a": "28"}},
	)
	tree := kit.NewTree(&kit.TreeNode{Key: "r", Title: "root", Children: []*kit.TreeNode{{Key: "c", Title: "child"}}})
	stat := kit.NewStatistic("Users", "1,024")
	stat.SetFace(face)
	desc := kit.NewDescriptions([2]string{"Name", "Ada"}, [2]string{"City", "London"})
	desc.SetFace(face)
	tl := kit.NewTimeline(kit.TimelineItem{Label: "Create"}, kit.TimelineItem{Label: "Ship"})
	tl.SetFace(face)
	collapse := kit.NewCollapse(
		kit.CollapsePanel{Key: "1", Header: "Panel 1", Content: kit.NewText("body 1").Node()},
		kit.CollapsePanel{Key: "2", Header: "Panel 2", Content: kit.NewText("body 2").Node()},
	)
	collapse.SetFace(face)
	collapse.SetActive("1")
	carousel := kit.NewCarousel(kit.NewText("Slide A").Node(), kit.NewText("Slide B").Node())
	img := kit.NewImage("Image", 120, 72)
	qr := kit.NewQRCode("gpui")
	tooltip := kit.NewTooltip(kit.NewButton("Hover me").Node(), "Tooltip")
	pop := kit.NewPopover(kit.NewButton("Popover").Node(), kit.NewText("popover body").Node())
	tour := kit.NewTour(kit.TourStep{Title: "Step1", Body: "demo"})
	wm := kit.NewWatermark(kit.NewText("content").Node(), "WM")
	contents["display"] = panel(face, "Data Display",
		sec(face, "Tag / Avatar / Badge / Card / Empty"),
		kit.NewSpace(tag.Node(), av.Node(), badge.Node()).Node(),
		card.Node(), empty.Node(),
		sec(face, "List / Table / Tree / Statistic / Descriptions"),
		list.Node(), table.Node(), tree.Node(), stat.Node(), desc.Node(),
		sec(face, "Timeline / Collapse / Carousel / Image / QR / Tooltip / Popover / Watermark"),
		tl.Node(), collapse.Node(), carousel.Node(), img.Node(), qr.Node(),
		tooltip.Node(), pop.Node(), tour.Node(), wm.Node(),
	)

	// --- Feedback ---
	al := kit.NewAlert("Alert message")
	al.SetFace(face)
	al.SetType("warning")
	prog := kit.NewProgress(60)
	spin := kit.NewSpin(nil)
	*tickers = append(*tickers, spin)
	sk := kit.NewSkeleton(200, 16)
	sk.SetActive(true)
	*tickers = append(*tickers, sk)
	res := kit.NewResult("success", "Success", "All good")
	res.SetFace(face)
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
	drawer := kit.NewDrawer("Drawer")
	drawer.Face = face
	drawer.SetContent(kit.NewText("drawer body").Node())
	openD := kit.NewButton("Open Drawer")
	openD.SetFace(face)
	openD.SetOnClick(func() { drawer.SetOpen(true); *status = "drawer open" })
	*buttons = append(*buttons, openD)
	msg := kit.NewMessageHost()
	pc := kit.NewPopconfirm(kit.NewButton("Popconfirm").Node(), "Are you sure?")
	contents["feedback"] = panel(face, "Feedback",
		sec(face, "Alert / Progress / Spin / Skeleton / Result"),
		al.Node(), prog.Node(), spin.Node(), sk.Node(), res.Node(),
		sec(face, "Modal / Drawer / Message / Popconfirm"),
		openM.Node(), modal.Node(), openD.Node(), drawer.Node(), msg.Node(), pc.Node(),
	)

	// --- Other ---
	scrollInner := primitive.Column()
	for i := 0; i < 15; i++ {
		tx := kit.NewText(fmt.Sprintf("scroll line %d", i+1))
		tx.SetFace(face)
		scrollInner.AddChild(tx.Node())
	}
	sc := kit.NewScroll(scrollInner)
	sc.SetSize(280, 120)
	affix := kit.NewAffix(kit.NewText("Affix/Sticky").Node())
	cfg := kit.NewConfigProvider(theme, kit.NewText("ConfigProvider child").Node())
	contents["other"] = panel(face, "Other · Affix / App(theme) / ConfigProvider / Scroll",
		sec(face, "Scroll (overflow + bar)"), sc.Node(),
		sec(face, "Affix"), affix.Node(),
		sec(face, "ConfigProvider"), cfg.Node(),
	)

	// Long rail to force tab-bar scrollbar
	items = []kit.MenuItem{
		{Key: "general", Label: "General"},
		{Key: "layout", Label: "Layout"},
		{Key: "nav", Label: "Navigation"},
		{Key: "entry", Label: "Data Entry"},
		{Key: "display", Label: "Data Display"},
		{Key: "feedback", Label: "Feedback"},
		{Key: "other", Label: "Other"},
	}
	return items, contents, modal
}
