package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

// TestAntCoverageNoLaterOrPrimitive enforces UI_UI_Base_ALL.md catalog completeness.
func TestAntCoverageNoLaterOrPrimitive(t *testing.T) {
	entries := kit.AntCoverage()
	if len(entries) < 50 {
		t.Fatalf("coverage entries=%d want full Ant overview set", len(entries))
	}
	for _, e := range entries {
		switch e.Status {
		case kit.CovLater, kit.CovPrimitive:
			t.Errorf("%s still %s via %s", e.Ant, e.Status, e.Via)
		case kit.CovReady, kit.CovPartial:
			if e.Via == "" {
				t.Errorf("%s missing Via", e.Ant)
			}
		default:
			t.Errorf("%s unknown status %q", e.Ant, e.Status)
		}
	}
}

// TestCatalogConstructorsLayout smoke-layouts every major kit constructor.
func TestCatalogConstructorsLayout(t *testing.T) {
	loose := core.Loose(400, 300)
	must := func(name string, n core.Node) {
		t.Helper()
		if n == nil {
			t.Fatalf("%s nil node", name)
		}
		sz := n.Layout(loose)
		if sz.Width < 0 || sz.Height < 0 {
			t.Fatalf("%s bad size %v", name, sz)
		}
	}

	must("Button", kit.NewButton("ok").Node())
	must("FloatButton", kit.NewFloatButton().Node())
	must("Icon", kit.NewIcon("check").Node())
	must("Text", kit.NewText("t").Node())
	must("Title", kit.NewTitle("T", 2).Node())
	must("Paragraph", kit.NewParagraph("p").Node())
	must("Link", kit.NewLink("l").Node())
	must("Typography", kit.NewTypography("ty").Node())
	must("Space", kit.NewSpace(kit.NewText("a").Node()).Node())
	must("Divider", kit.NewDivider().Node())
	must("Flex", kit.NewFlex(kit.NewText("a").Node()).Node())
	must("Grid", func() core.Node {
		a, b := kit.NewCol(kit.NewText("1").Node()), kit.NewCol(kit.NewText("2").Node())
		a.SetSpan(12)
		b.SetSpan(12)
		return kit.NewRow(a.Node(), b.Node()).Node()
	}())
	must("Layout", kit.NewLayout(kit.NewHeader(kit.NewText("h").Node()).Node(), kit.NewContent(kit.NewText("c").Node()).Node()).Node())
	must("Splitter", kit.NewSplitterNodes(kit.NewText("a").Node(), kit.NewText("b").Node()).Node())
	must("Breadcrumb", kit.NewBreadcrumb("a", "b").Node())
	must("Steps", kit.NewSteps(kit.StepTitles("a", "b")...).Node())
	must("Anchor", kit.NewAnchor(kit.AnchorItem{Key: "a", Href: "#a", Title: "A"}).Node())
	must("Pagination", func() core.Node { p := kit.NewPagination(); p.SetTotal(30); return p.Node() }())
	must("Menu", kit.NewMenu(kit.MenuItem{Key: "a", Label: "A"}).Node())
	must("Tabs", kit.NewTabs(kit.TabItem{Key: "a", Label: "A"}).Node())
	must("Dropdown", kit.NewDropdown("d", kit.MenuItem{Key: "1", Label: "1"}).Node())
	must("Input", kit.NewInput("x").Node())
	must("TextArea", kit.NewTextArea("x", 2).Node())
	must("InputNumber", kit.NewInputNumberValue(1).Node())
	must("Checkbox", kit.NewCheckbox("c").Node())
	must("Switch", kit.NewSwitch().Node())
	must("Select", kit.NewSelect("s", kit.SelectOption{Value: "1", Label: "1"}).Node())
	must("Rate", kit.NewRate().Node())
	must("Segmented", kit.NewSegmented("a", "b").Node())
	must("Slider", kit.NewSlider(10).Node())
	must("AutoComplete", kit.NewAutoComplete("a", "x").Node())
	must("Mentions", kit.NewMentions("@", "u").Node())
	must("Calendar", func() core.Node { c := kit.NewCalendar(); c.SetDefaultValue(kit.DateOf(2026, 7, 1)); return c.Node() }())
	must("DatePicker", kit.NewDatePicker().Node())
	must("TimePicker", kit.NewTimePicker().Node())
	must("ColorPicker", kit.NewColorPicker().Node())
	must("Upload", kit.NewUpload("up").Node())
	must("TreeSelect", kit.NewTreeSelect("t", kit.TreeSelectNode{Value: "a/b", Title: "a/b"}).Node())
	must("Form", kit.NewForm().Node())
	must("Tag", kit.NewTag("t").Node())
	must("Avatar", kit.NewAvatar("A").Node())
	must("Badge", func() core.Node {
		b := kit.NewBadge()
		b.SetChild(kit.NewText("x").Node())
		b.SetCount(1)
		return b.Node()
	}())
	must("Card", kit.NewCard("c").Node())
	must("Empty", kit.NewEmpty().Node())
	must("List", kit.NewList("a").Node())
	must("Table", kit.NewTableWith([]kit.TableColumn{{Key: "a", Title: "A"}}, nil).Node())
	must("Tree", kit.NewTree(&kit.TreeNode{Key: "r", Title: "r"}).Node())
	must("Statistic", func() core.Node {
		s := kit.NewStatistic()
		s.SetTitle("t")
		s.SetValue(1)
		return s.Node()
	}())
	must("Descriptions", kit.NewDescriptions(kit.DescriptionsItem{Label: "a", Children: "b"}).Node())
	must("Timeline", kit.NewTimeline(kit.TimelineItem{Content: "x"}).Node())
	must("Collapse", kit.NewCollapse(kit.CollapsePanel{Key: "1", Header: "h"}).Node())
	must("Carousel", kit.NewCarousel(kit.NewText("1").Node()).Node())
	must("Image", kit.NewImageSized("i", 40, 40).Node())
	must("QRCode", kit.NewQRCode("q").Node())
	must("Tooltip", kit.NewTooltip("tip").Node())
	pop := kit.NewPopover("t")
	pop.SetContent("b")
	must("Popover", pop.Node())
	must("Tour", kit.NewTour(kit.TourStep{Title: "t", Body: "b"}).Node())
	must("Watermark", kit.NewWatermark(kit.NewText("c").Node(), "w").Node())
	must("Alert", kit.NewAlert("a").Node())
	must("Progress", kit.NewProgress(50).Node())
	must("Spin", kit.NewSpin(nil).Node())
	must("Skeleton", kit.NewSkeleton(40, 10).Node())
	res := kit.NewResult()
	res.SetTitle("t")
	res.SetSubTitle("s")
	must("Result", res.Node())
	must("Modal", kit.NewModal("m").Node())
	must("Drawer", kit.NewDrawer("d").Node())
	must("MessageHost", kit.NewMessageHost().Node())
	must("Notification", kit.NewNotification().Node())
	must("Popconfirm", kit.NewPopconfirm("sure?").Node())
	must("Scroll", kit.NewScroll(kit.NewText("s").Node()).Node())
	must("Affix", kit.NewAffix(kit.NewText("a").Node()).Node())
	must("ConfigProvider", kit.NewConfigProvider(kit.DefaultTheme(), kit.NewText("c").Node()).Node())
	must("Transfer", func() core.Node {
		tr := kit.NewTransfer()
		tr.SetDataSource(kit.TransferItemsFromTitles("a"))
		return tr.Node()
	}())
	must("Cascader", kit.NewCascader("", kit.CascaderOption{Value: "r", Label: "r"}).Node())
}
