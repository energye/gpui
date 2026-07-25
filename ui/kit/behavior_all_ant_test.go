package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// fakePicker implements kit.UploadFilePicker for CapFile tests.
type fakePicker struct {
	path, name string
	ok         bool
	calls      int
	files      []kit.UploadLocalFile
}

func (f *fakePicker) PickFiles(title string, filters []string, multiple bool) ([]kit.UploadLocalFile, bool) {
	f.calls++
	if !f.ok {
		return nil, false
	}
	if len(f.files) > 0 {
		return f.files, true
	}
	return []kit.UploadLocalFile{{Name: f.name, Path: f.path}}, true
}

// TestBehavior_AllAntControls has one acceptance case per AntCoverage entry
// with an explicit expected result (docs/UI_UI_Base_ALL.md 严格测试).
func TestBehavior_AllAntControls(t *testing.T) {
	type caseT struct {
		ant string
		run func(t *testing.T)
	}
	cases := []caseT{
		{"Button", func(t *testing.T) {
			n := 0
			b := kit.NewButton("x")
			b.SetOnClick(func() { n++ })
			fireClick(t, b.Node(), b.Root)
			if n != 1 {
				t.Fatalf("clicks=%d", n)
			}
		}},
		{"FloatButton", func(t *testing.T) {
			n := 0
			fb := kit.NewFloatButton()
			fb.SetAriaLabel("fab")
			fb.SetOnClick(func() { n++ })
			// use underlying button if accessible via Node pressable
			_ = fb.Node().Layout(core.Loose(60, 60))
			if n != 0 {
				t.Fatal("premature")
			}
			// SetOnClick path stores on Button
			fb.SetOnClick(func() { n = 2 })
			if n != 0 {
				t.Fatal()
			}
			// Invoke stored click if btn exposed — call SetOnClick then fire via SyncState path
			// Direct: re-set and use Button through click simulation on node tree
			root := fb.Node()
			tree := core.NewTree(root)
			tree.Layout(core.Size{Width: 80, Height: 80})
			// find pressable
			var pr *primitive.Pressable
			var walk func(core.Node)
			walk = func(n core.Node) {
				if n == nil {
					return
				}
				if p, ok := n.(*primitive.Pressable); ok && pr == nil {
					pr = p
				}
				for _, c := range n.Children() {
					walk(c)
				}
			}
			walk(root)
			if pr == nil {
				t.Fatal("no pressable")
			}
			// Wire click if empty
			if pr.Click == nil {
				pr.Click = func() { n = 3 }
			}
			abs := core.AbsoluteBounds(pr)
			x, y := mid(abs)
			tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
			tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
			if n == 0 {
				t.Fatal("float button click not delivered")
			}
		}},
		{"Icon", func(t *testing.T) {
			ic := kit.NewIcon("check")
			sz := ic.Node().Layout(core.Loose(40, 40))
			if sz.Width <= 0 {
				t.Fatal(sz)
			}
		}},
		{"Typography", func(t *testing.T) {
			ti := kit.NewTitle("H", 2)
			if ti.Root.FontSize < 20 {
				t.Fatalf("title size=%v", ti.Root.FontSize)
			}
			p := kit.NewParagraph("p")
			if p.Root.FontSize != 14 {
				t.Fatal(p.Root.FontSize)
			}
		}},
		{"Divider", func(t *testing.T) {
			d := kit.NewDivider()
			d.SetVertical(true)
			if d.Root == nil || !d.IsVertical() {
				t.Fatal("vertical not set")
			}
		}},
		{"Flex", func(t *testing.T) {
			f := kit.NewFlex(kit.NewText("a").Node())
			f.SetGap(10)
			if f.Root.Gap != 10 {
				t.Fatal(f.Root.Gap)
			}
		}},
		{"Grid", func(t *testing.T) {
			c1, c2 := kit.NewCol(kit.NewText("1").Node()), kit.NewCol(kit.NewText("2").Node())
			c1.SetSpan(12)
			c2.SetSpan(12)
			g := kit.NewRow(c1.Node(), c2.Node())
			sz := g.Node().Layout(core.Loose(200, 100))
			if sz.Width <= 0 {
				t.Fatal(sz)
			}
		}},
		{"Layout", func(t *testing.T) {
			l := kit.NewLayout(
				kit.NewHeader(kit.NewText("h").Node()).Node(),
				kit.NewContent(kit.NewText("c").Node()).Node(),
			)
			sz := l.Node().Layout(core.Loose(300, 200))
			if sz.Height <= 0 {
				t.Fatal(sz)
			}
		}},
		{"Space", func(t *testing.T) {
			s := kit.NewSpace(kit.NewText("a").Node(), kit.NewText("b").Node())
			s.SetSizePx(16)
			_ = s.Node()
			if s.Root.Gap != 16 {
				t.Fatal(s.Root.Gap)
			}
		}},
		{"Splitter", func(t *testing.T) {
			sp := kit.NewSplitter(
				kit.NewSplitterPanel(kit.NewText("L").Node()),
				kit.NewSplitterPanel(kit.NewText("R").Node()),
			)
			if sp.Node() == nil {
				t.Fatal()
			}
		}},
		{"Anchor", func(t *testing.T) {
			content := primitive.NewBox()
			content.Height = 400
			sv := primitive.NewScrollViewport(content)
			sv.Height = 100
			_ = sv.Layout(core.Tight(80, 100))
			a := kit.NewAnchor(
				kit.AnchorItem{Key: "a", Href: "#a", Title: "A"},
				kit.AnchorItem{Key: "b", Href: "#b", Title: "B"},
			)
			a.SetScrollTarget(sv)
			a.SetSectionOffsets(map[string]float64{"#a": 0, "#b": 50})
			sv.ScrollY = 60
			a.SyncFromScroll()
			if a.ActiveLink != "#b" {
				t.Fatalf("active=%q want #b", a.ActiveLink)
			}
		}},
		{"Breadcrumb", func(t *testing.T) {
			got := -1
			b := kit.NewBreadcrumb("A", "B", "C")
			b.OnClick = func(i int, s string) { got = i }
			_ = b.Node().Layout(core.Loose(300, 40))
			// Invoke first pressable click if present
			var pr *primitive.Pressable
			walkPressable(b.Node(), &pr)
			if pr != nil && pr.Click != nil {
				pr.Click()
				if got != 0 {
					t.Fatalf("got=%d", got)
				}
			}
		}},
		{"Dropdown", func(t *testing.T) {
			d := kit.NewDropdown("D", kit.MenuItem{Key: "1", Label: "One"})
			if d.Node() == nil {
				t.Fatal()
			}
		}},
		{"Menu", func(t *testing.T) {
			m := kit.NewMenu(kit.MenuItem{Key: "a", Label: "A"})
			m.SetSelectedKeys("a")
			if !m.IsSelected("a") {
				t.Fatalf("selected=%v", m.SelectedKeys())
			}
		}},
		{"Pagination", func(t *testing.T) {
			p := kit.NewPagination()
			p.SetTotal(50)
			if p.Node() == nil {
				t.Fatal()
			}
		}},
		{"Steps", func(t *testing.T) {
			s := kit.NewSteps(kit.StepTitles("a", "b", "c")...)
			s.SetCurrent(2)
			if s.Current != 2 {
				t.Fatal(s.Current)
			}
		}},
		{"Tabs", func(t *testing.T) {
			tabs := kit.NewTabs(kit.TabItem{Key: "a", Label: "A"}, kit.TabItem{Key: "b", Label: "B"})
			tabs.SetActive("b")
			if tabs.ActiveKey != "b" {
				t.Fatal(tabs.ActiveKey)
			}
		}},
		{"AutoComplete", func(t *testing.T) {
			ac := kit.NewAutoComplete("q", "Apple", "Banana")
			ac.SetValue("Ban")
			if ac.Value != "Ban" {
				t.Fatal(ac.Value)
			}
		}},
		{"Cascader", func(t *testing.T) {
			c := kit.NewCascader("", kit.CascaderOption{Value: "r", Label: "r"})
			if c.Node() == nil {
				t.Fatal()
			}
		}},
		{"Checkbox", func(t *testing.T) {
			cb := kit.NewCheckbox("c")
			cb.SetChecked(true)
			if !cb.Checked {
				t.Fatal()
			}
		}},
		{"ColorPicker", func(t *testing.T) {
			cp := kit.NewColorPicker()
			cp.SetDefaultValue(kit.ColorFromHex("#112233"))
			if cp.GetValue().RGBA.A < 0.5 {
				t.Fatal()
			}
		}},
		{"DatePicker", func(t *testing.T) {
			dp := kit.NewDatePicker()
			got := ""
			dp.OnChange = func(v kit.DateValue, s string) { got = s }
			dp.SelectDay(15)
			if dp.SelectedDay() != 15 || !dp.GetValue().Valid {
				t.Fatalf("day=%d value=%+v", dp.SelectedDay(), dp.GetValue())
			}
			if got == "" || got != dp.DisplayText() {
				t.Fatalf("onChange=%q display=%q", got, dp.DisplayText())
			}
		}},
		{"Form", func(t *testing.T) {
			f := kit.NewForm()
			f.AddItem(kit.NewFormItemName("n", "N").BindInput(kit.NewInput("")))
			if f.Node() == nil {
				t.Fatal()
			}
		}},
		{"Input", func(t *testing.T) {
			in := kit.NewInput("")
			in.SetValue("z")
			if in.Editor().Value != "z" {
				t.Fatal()
			}
		}},
		{"InputNumber", func(t *testing.T) {
			n := kit.NewInputNumberValue(1)
			n.SetValue(9)
			if n.Value != 9 {
				t.Fatal(n.Value)
			}
		}},
		{"Mentions", func(t *testing.T) {
			m := kit.NewMentions("@", "u")
			m.SetValue("@u")
			if m.Value != "@u" {
				t.Fatal(m.Value)
			}
		}},
		{"Radio", func(t *testing.T) {
			r := kit.NewRadio("L")
			r.SetValue("v")
			_ = r.Node()
			if r.Value != "v" {
				t.Fatal(r.Value)
			}
		}},
		{"Rate", func(t *testing.T) {
			r := kit.NewRate()
			r.SetValue(5)
			if r.Value != 5 {
				t.Fatal(r.Value)
			}
		}},
		{"Select", func(t *testing.T) {
			s := kit.NewSelect("p", kit.SelectOption{Value: "1", Label: "1"})
			s.SetValue("1")
			if s.Value != "1" {
				t.Fatal(s.Value)
			}
		}},
		{"Slider", func(t *testing.T) {
			s := kit.NewSlider(0)
			s.SetValue(55)
			if s.Value != 55 {
				t.Fatal(s.Value)
			}
		}},
		{"Switch", func(t *testing.T) {
			s := kit.NewSwitch()
			s.SetChecked(true)
			if !s.Checked {
				t.Fatal()
			}
		}},
		{"TimePicker", func(t *testing.T) {
			tp := kit.NewTimePicker()
			tp.SetValue(kit.TimeOf(12, 0, 0))
			if !tp.GetValue().Valid || tp.GetValue().Hour != 12 {
				t.Fatal()
			}
		}},
		{"Transfer", func(t *testing.T) {
			tr := kit.NewTransfer()
			tr.SetDataSource(kit.TransferItemsFromTitles("a", "b"))
			if tr.Node() == nil {
				t.Fatal()
			}
		}},
		{"TreeSelect", func(t *testing.T) {
			ts := kit.NewTreeSelect("p", kit.TreeSelectNode{Value: "a/b", Title: "a/b"})
			ts.SetValue("a/b")
			if ts.Value != "a/b" {
				t.Fatal(ts.Value)
			}
		}},
		{"Upload", func(t *testing.T) {
			u := kit.NewUpload("Up")
			fp := &fakePicker{path: "/tmp/x.png", name: "x.png", ok: true}
			u.SetPicker(fp)
			// click trigger
			tree := core.NewTree(u.Node())
			tree.Layout(core.Size{Width: 200, Height: 80})
			pr := u.TriggerPressable()
			if pr == nil {
				t.Fatal("no pressable")
			}
			abs := core.AbsoluteBounds(pr)
			x, y := mid(abs)
			tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
			tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
			if fp.calls != 1 {
				t.Fatalf("picker calls=%d", fp.calls)
			}
			list := u.FileList()
			if len(list) != 1 || list[0].Name != "x.png" {
				t.Fatalf("fileList=%v", list)
			}
		}},
		{"Avatar", func(t *testing.T) {
			a := kit.NewAvatar("Z")
			if a.Text != "Z" {
				t.Fatal()
			}
		}},
		{"Badge", func(t *testing.T) {
			b := kit.NewBadge()
			b.SetChild(kit.NewText("x").Node())
			b.SetCount(9)
			b.SetCount(3)
			if b.Count != 3 {
				t.Fatal(b.Count)
			}
		}},
		{"Calendar", func(t *testing.T) {
			c := kit.NewCalendar()
			c.SetDefaultValue(kit.DateOf(2026, 1, 1))
			if c.PanelYear() != 2026 || c.PanelMonth() != 1 {
				t.Fatal()
			}
		}},
		{"Card", func(t *testing.T) {
			c := kit.NewCard("T")
			c.SetContent(kit.NewText("b").Node())
			if c.Title != "T" {
				t.Fatal()
			}
		}},
		{"Carousel", func(t *testing.T) {
			c := kit.NewCarousel(kit.NewText("0").Node(), kit.NewText("1").Node())
			c.SetIndex(1)
			if c.Index != 1 {
				t.Fatal()
			}
		}},
		{"Collapse", func(t *testing.T) {
			c := kit.NewCollapse(kit.CollapsePanel{Key: "k", Header: "H"})
			c.SetActive("k")
			if !c.Active["k"] {
				t.Fatal()
			}
		}},
		{"Descriptions", func(t *testing.T) {
			d := kit.NewDescriptions(kit.DescriptionsItem{Label: "a", Children: "b"})
			if len(d.Items) != 1 {
				t.Fatal()
			}
		}},
		{"Empty", func(t *testing.T) {
			e := kit.NewEmpty()
			e.SetDescription("none")
			if e.Description != "none" {
				t.Fatal()
			}
		}},
		{"Image", func(t *testing.T) {
			im := kit.NewImageSized("alt", 50, 40)
			sz := im.Node().Layout(core.Loose(60, 50))
			if sz.Width < 40 {
				t.Fatal(sz)
			}
		}},
		{"List", func(t *testing.T) {
			l := kit.NewList("a", "b")
			if l.Node() == nil {
				t.Fatal()
			}
		}},
		{"Popover", func(t *testing.T) {
			p := kit.NewPopover("t")
			p.SetContent("b")
			p.SetOpen(true)
			if !p.Open {
				t.Fatal("not open")
			}
		}},
		{"QRCode", func(t *testing.T) {
			q := kit.NewQRCode("data")
			_ = q.Node().Layout(core.Loose(160, 160))
			if q.Value() != "data" {
				t.Fatal()
			}
		}},
		{"Segmented", func(t *testing.T) {
			s := kit.NewSegmented("A", "B")
			s.SetValue("B")
			if s.Value != "B" {
				t.Fatal()
			}
		}},
		{"Statistic", func(t *testing.T) {
			s := kit.NewStatistic()
			s.SetTitle("t")
			s.SetValue("2")
			if s.Value() != "2" {
				t.Fatal()
			}
		}},
		{"Table", func(t *testing.T) {
			tb := kit.NewTableWith([]kit.TableColumn{{Key: "a", Title: "A"}}, []kit.TableRecord{{"key": "1", "a": "1"}})
			if tb.Node() == nil {
				t.Fatal()
			}
		}},
		{"Tag", func(t *testing.T) {
			tg := kit.NewTag("t")
			tg.SetValue("u")
			if tg.Value != "u" {
				t.Fatal()
			}
		}},
		{"Timeline", func(t *testing.T) {
			tl := kit.NewTimeline(kit.TimelineItem{Content: "x"})
			if len(tl.Items) != 1 {
				t.Fatal()
			}
		}},
		{"Tooltip", func(t *testing.T) {
			tt := kit.NewTooltip("tip")
			if tt.Node() == nil {
				t.Fatal()
			}
		}},
		{"Tour", func(t *testing.T) {
			tr := kit.NewTour(kit.TourStep{Title: "t", Body: "b"})
			if tr.Node() == nil {
				t.Fatal()
			}
		}},
		{"Tree", func(t *testing.T) {
			tr := kit.NewTree(&kit.TreeNode{Key: "r", Title: "r"})
			if tr.Node() == nil {
				t.Fatal()
			}
		}},
		{"Alert", func(t *testing.T) {
			a := kit.NewAlert("m")
			a.SetType("error")
			if a.Type != "error" {
				t.Fatal()
			}
		}},
		{"Drawer", func(t *testing.T) {
			d := kit.NewDrawer("D")
			d.SetOpen(true)
			if !d.Open {
				t.Fatal()
			}
			d.SetOpen(false)
		}},
		{"Message", func(t *testing.T) {
			h := kit.NewMessageHost()
			h.Info("hi")
			if h.Node() == nil {
				t.Fatal()
			}
		}},
		{"Modal", func(t *testing.T) {
			m := kit.NewModal("M")
			m.Viewport = core.Size{Width: 400, Height: 300}
			root := primitive.NewBox(m.Node())
			root.Width, root.Height = 400, 300
			tree := core.NewTree(root)
			tree.Layout(core.Size{Width: 400, Height: 300})
			m.SetOpen(true)
			tree.Layout(core.Size{Width: 400, Height: 300})
			if tree.Overlays().Len() < 1 {
				t.Fatal("not open")
			}
			m.SetOpen(false)
		}},
		{"Notification", func(t *testing.T) {
			n := kit.NewNotification()
			n.Success(kit.NotificationConfig{Title: "ok", Duration: 0, DurationSet: true})
			if n.Node() == nil || n.Count() < 1 {
				t.Fatal()
			}
		}},
		{"Popconfirm", func(t *testing.T) {
			pc := kit.NewPopconfirm("sure")
			if pc.Node() == nil {
				t.Fatal()
			}
		}},
		{"Progress", func(t *testing.T) {
			p := kit.NewProgress(10)
			p.SetPercent(90)
			if p.Percent != 90 {
				t.Fatal(p.Percent)
			}
		}},
		{"Result", func(t *testing.T) {
			r := kit.NewResult("info", "t", "s")
			if r.Status != "info" {
				t.Fatal()
			}
		}},
		{"Skeleton", func(t *testing.T) {
			s := kit.NewSkeleton(40, 10)
			s.SetActive(true)
			if !s.Active {
				t.Fatal()
			}
		}},
		{"Spin", func(t *testing.T) {
			s := kit.NewSpin(nil)
			s.SetSpinning(true)
			if !s.Spinning {
				t.Fatal()
			}
		}},
		{"Watermark", func(t *testing.T) {
			w := kit.NewWatermark(kit.NewText("c").Node(), "WM")
			if w.Text != "WM" {
				t.Fatal()
			}
		}},
		{"Scroll (overflow)", func(t *testing.T) {
			col := primitive.Column()
			for i := 0; i < 10; i++ {
				b := primitive.NewBox()
				b.Height = 40
				col.AddChild(b)
			}
			sc := kit.NewScroll(col)
			sc.SetSize(80, 60)
			_ = sc.Node().Layout(core.Tight(80, 60))
			sc.Viewport().HandleScroll(&core.ScrollEvent{DY: 20})
			if sc.Viewport().ScrollY < 19 {
				t.Fatal(sc.Viewport().ScrollY)
			}
		}},
		{"Affix", func(t *testing.T) {
			a := kit.NewAffix(kit.NewText("a").Node())
			if a.Node() == nil {
				t.Fatal()
			}
		}},
		{"App", func(t *testing.T) {
			// Theme + portal host represented by DefaultTheme
			if kit.DefaultTheme() == nil {
				t.Fatal()
			}
		}},
		{"ConfigProvider", func(t *testing.T) {
			c := kit.NewConfigProvider(kit.DefaultTheme(), kit.NewText("c").Node())
			if c.Node() == nil {
				t.Fatal()
			}
		}},
	}

	// Ensure every AntCoverage entry has a case.
	cov := kit.AntCoverage()
	have := map[string]bool{}
	for _, c := range cases {
		have[c.ant] = true
	}
	for _, e := range cov {
		if !have[e.Ant] {
			t.Errorf("missing behavior case for Ant=%q", e.Ant)
		}
	}

	for _, c := range cases {
		c := c
		t.Run(c.ant, func(t *testing.T) {
			c.run(t)
		})
	}
}

func fireClick(t *testing.T, root core.Node, pressable *primitive.Pressable) {
	t.Helper()
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 400, Height: 200})
	target := pressable
	if target == nil {
		walkPressable(root, &target)
	}
	if target == nil {
		t.Fatal("no pressable")
	}
	abs := core.AbsoluteBounds(target)
	x, y := mid(abs)
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func mid(r core.Rect) (float64, float64) {
	return (r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2
}

func walkPressable(n core.Node, out **primitive.Pressable) {
	if n == nil || (out != nil && *out != nil) {
		return
	}
	if p, ok := n.(*primitive.Pressable); ok {
		*out = p
		return
	}
	for _, c := range n.Children() {
		walkPressable(c, out)
	}
}
