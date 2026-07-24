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
	must("Space", kit.NewSpace(kit.NewText("a").Node()).Node())
	must("Divider", kit.NewDivider().Node())
	must("Flex", kit.NewFlexRow(kit.NewText("a").Node()).Node())
	must("Grid", kit.NewGridCols(2, kit.NewText("1").Node(), kit.NewText("2").Node()).Node())
	must("Layout", kit.NewLayout(kit.NewText("h").Node(), nil, kit.NewText("c").Node(), nil).Node())
	must("Splitter", kit.NewSplitter(kit.NewText("a").Node(), kit.NewText("b").Node()).Node())
	must("Breadcrumb", kit.NewBreadcrumb("a", "b").Node())
	must("Steps", kit.NewSteps("a", "b").Node())
	must("Anchor", kit.NewAnchor("#a").Node())
	must("Pagination", kit.NewPagination(3).Node())
	must("Menu", kit.NewMenu(kit.MenuItem{Key: "a", Label: "A"}).Node())
	must("Tabs", kit.NewTabs(kit.MenuItem{Key: "a", Label: "A"}).Node())
	must("Dropdown", kit.NewDropdown("d", kit.MenuItem{Key: "1", Label: "1"}).Node())
	must("Input", kit.NewInput("x").Node())
	must("TextArea", kit.NewTextArea("x", 2).Node())
	must("InputNumber", kit.NewInputNumber(1).Node())
	must("Checkbox", kit.NewCheckbox("c").Node())
	must("Switch", kit.NewSwitch().Node())
	must("Select", kit.NewSelect("s", kit.SelectOption{Value: "1", Label: "1"}).Node())
	must("Rate", kit.NewRate(2).Node())
	must("Segmented", kit.NewSegmented("a", "b").Node())
	must("Slider", kit.NewSlider(10).Node())
	must("AutoComplete", kit.NewAutoComplete("a", "x").Node())
	must("Mentions", kit.NewMentions("@", "u").Node())
	must("Calendar", kit.NewCalendar(2026, 7).Node())
	must("DatePicker", kit.NewDatePicker().Node())
	must("TimePicker", kit.NewTimePicker().Node())
	must("ColorPicker", kit.NewColorPicker().Node())
	must("Upload", kit.NewUpload("up").Node())
	must("TreeSelect", kit.NewTreeSelect("t", "a/b").Node())
	must("Form", kit.NewForm(core.NewFormModel()).Node())
	must("Tag", kit.NewTag("t").Node())
	must("Avatar", kit.NewAvatar("A").Node())
	must("Badge", kit.NewBadge(kit.NewText("x").Node(), 1).Node())
	must("Card", kit.NewCard("c").Node())
	must("Empty", kit.NewEmpty("").Node())
	must("List", kit.NewList("a").Node())
	must("Table", kit.NewTable([]kit.TableColumn{{Key: "a", Title: "A"}}, nil).Node())
	must("Tree", kit.NewTree(&kit.TreeNode{Key: "r", Title: "r"}).Node())
	must("Statistic", kit.NewStatistic("t", "1").Node())
	must("Descriptions", kit.NewDescriptions([2]string{"a", "b"}).Node())
	must("Timeline", kit.NewTimeline(kit.TimelineItem{Label: "x"}).Node())
	must("Collapse", kit.NewCollapse(kit.CollapsePanel{Key: "1", Header: "h"}).Node())
	must("Carousel", kit.NewCarousel(kit.NewText("1").Node()).Node())
	must("Image", kit.NewImage("i", 40, 40).Node())
	must("QRCode", kit.NewQRCode("q").Node())
	must("Tooltip", kit.NewTooltip(kit.NewText("t").Node(), "tip").Node())
	must("Popover", kit.NewPopover(kit.NewText("t").Node(), kit.NewText("b").Node()).Node())
	must("Tour", kit.NewTour(kit.TourStep{Title: "t", Body: "b"}).Node())
	must("Watermark", kit.NewWatermark(kit.NewText("c").Node(), "w").Node())
	must("Alert", kit.NewAlert("a").Node())
	must("Progress", kit.NewProgress(50).Node())
	must("Spin", kit.NewSpin(nil).Node())
	must("Skeleton", kit.NewSkeleton(40, 10).Node())
	must("Result", kit.NewResult("info", "t", "s").Node())
	must("Modal", kit.NewModal("m").Node())
	must("Drawer", kit.NewDrawer("d").Node())
	must("MessageHost", kit.NewMessageHost().Node())
	must("Popconfirm", kit.NewPopconfirm(kit.NewText("t").Node(), "sure?").Node())
	must("Scroll", kit.NewScroll(kit.NewText("s").Node()).Node())
	must("Affix", kit.NewAffix(kit.NewText("a").Node()).Node())
	must("ConfigProvider", kit.NewConfigProvider(kit.DefaultTheme(), kit.NewText("c").Node()).Node())
	must("Transfer", kit.NewTransfer([]string{"a"}).Node())
	must("Cascader", kit.NewCascader(&kit.TreeNode{Key: "r", Title: "r"}).Node())
}
