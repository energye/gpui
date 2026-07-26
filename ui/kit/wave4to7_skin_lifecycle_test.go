package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

func TestAllKit_SkinPaintersRegistered(t *testing.T) {
	th := skindefault.Theme()
	ids := []string{
		kit.TypeButton, kit.TypeBreadcrumb, kit.TypeInput, kit.TypeSwitch,
		kit.TypeCheckbox, kit.TypeRadio, kit.TypeTag, kit.TypeFloatButton,
		kit.TypeBadge, kit.TypeAlert, kit.TypeProgress, kit.TypeSpin,
		kit.TypeSkeleton, kit.TypeEmpty, kit.TypeResult, kit.TypeStatistic,
		kit.TypeAvatar, kit.TypeDivider, kit.TypeIcon, kit.TypeTypography,
		kit.TypeTabs, kit.TypeMenu, kit.TypeDropdown, kit.TypePagination,
		kit.TypeSteps, kit.TypeAnchor, kit.TypeSpace, kit.TypeKitFlex,
		kit.TypeScroll, kit.TypeLayout, kit.TypeSplitter,
		kit.TypeModal, kit.TypeDrawer, kit.TypeMessage, kit.TypeNotification,
		kit.TypePopover, kit.TypeTooltip, kit.TypePopconfirm, kit.TypeTour,
		kit.TypeSelect, kit.TypeInputNumber, kit.TypeMentions, kit.TypeAutoComplete,
		kit.TypeCascader, kit.TypeTreeSelect, kit.TypeDatePicker, kit.TypeTimePicker,
		kit.TypeUpload, kit.TypeForm, kit.TypeRate, kit.TypeSlider,
		kit.TypeColorPicker, kit.TypeSegmented, kit.TypeTransfer,
		kit.TypeTable, kit.TypeTree, kit.TypeList, kit.TypeDescriptions,
		kit.TypeCard, kit.TypeCalendar, kit.TypeCarousel, kit.TypeCollapse,
		kit.TypeImage, kit.TypeTimeline, kit.TypeQRCode,
		kit.TypeAffix, kit.TypeWatermark, kit.TypeBorderBeam, kit.TypeConfigProvider,
	}
	for _, id := range ids {
		if th.Painter(id) == nil {
			t.Errorf("missing skin painter for %s", id)
		}
	}
}

func TestWave4to7_EnsureBuilt_SampleNodes(t *testing.T) {
	box := primitive.NewBox()
	cases := []struct {
		name string
		node func() core.Node
	}{
		{"Modal", func() core.Node { return kit.NewModal("T").Node() }},
		{"Drawer", func() core.Node { return kit.NewDrawer("T").Node() }},
		{"Message", func() core.Node { return kit.NewMessage().Node() }},
		{"Notification", func() core.Node { return kit.NewNotification().Node() }},
		{"Popover", func() core.Node { return kit.NewPopover("t").Node() }},
		{"Tooltip", func() core.Node { return kit.NewTooltip("tip").Node() }},
		{"Popconfirm", func() core.Node { return kit.NewPopconfirm("sure?").Node() }},
		{"Tour", func() core.Node { return kit.NewTour().Node() }},
		{"Select", func() core.Node { return kit.NewSelect("pick").Node() }},
		{"InputNumber", func() core.Node { return kit.NewInputNumber().Node() }},
		{"Form", func() core.Node { return kit.NewForm().Node() }},
		{"Rate", func() core.Node { return kit.NewRate().Node() }},
		{"Slider", func() core.Node { return kit.NewSlider(0).Node() }},
		{"Table", func() core.Node { return kit.NewTable().Node() }},
		{"List", func() core.Node { return kit.NewList("a", "b").Node() }},
		{"Card", func() core.Node { return kit.NewCard("title").Node() }},
		{"Collapse", func() core.Node { return kit.NewCollapse().Node() }},
		{"Timeline", func() core.Node { return kit.NewTimeline().Node() }},
		{"QRCode", func() core.Node { return kit.NewQRCode("https://example.com").Node() }},
		{"Affix", func() core.Node { return kit.NewAffix(box).Node() }},
		{"Watermark", func() core.Node { return kit.NewWatermark(box).Node() }},
		{"Calendar", func() core.Node { return kit.NewCalendar().Node() }},
		{"Image", func() core.Node { return kit.NewImage().Node() }},
		{"ColorPicker", func() core.Node { return kit.NewColorPicker().Node() }},
		{"Transfer", func() core.Node { return kit.NewTransfer().Node() }},
		{"Segmented", func() core.Node { return kit.NewSegmented("A", "B").Node() }},
		{"Upload", func() core.Node { return kit.NewUpload().Node() }},
		{"DatePicker", func() core.Node { return kit.NewDatePicker().Node() }},
		{"TimePicker", func() core.Node { return kit.NewTimePicker().Node() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic: %v", r)
				}
			}()
			n := tc.node()
			if n == nil {
				t.Fatal("nil node")
			}
		})
	}
}
