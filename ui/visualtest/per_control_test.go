package visualtest_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	"github.com/energye/gpui/ui/visualtest"
)

type ctlSpec struct {
	id   string
	w, h int
	root func() core.Node
}

func padRoot(n core.Node, w, h float64) core.Node {
	// Outer white canvas + thin border so text-only controls still produce
	// non-blank goldens without requiring a system font face.
	inner := primitive.NewDecorated(n)
	inner.Padding = primitive.All(6)
	inner.BorderWidth = 1
	inner.BorderColor = render.RGBA{R: 0.85, G: 0.85, B: 0.88, A: 1}
	inner.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	inner.Radius = 2
	b := primitive.NewBox(inner)
	b.Width, b.Height = w, h
	b.Padding = primitive.All(8)
	b.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	return b
}

// One AssertScenario golden per kit control (docs/UI_UI_Base_ALL.md §测试生图).
func perControlSpecs() []ctlSpec {
	return []ctlSpec{
		{"ctl_button", 140, 48, func() core.Node {
			b := kit.NewButton("")
			b.SetType(kit.ButtonPrimary)
			b.SetFixedSize(100, 32)
			return padRoot(b.Node(), 140, 48)
		}},
		{"ctl_float_button", 80, 80, func() core.Node {
			fb := kit.NewFloatButton()
			fb.SetAriaLabel("fab")
			return padRoot(fb.Node(), 80, 80)
		}},
		{"ctl_icon", 48, 48, func() core.Node { return padRoot(kit.NewIcon("check").Node(), 48, 48) }},
		{"ctl_text", 120, 40, func() core.Node { return padRoot(kit.NewText("Text").Node(), 120, 40) }},
		{"ctl_title", 200, 56, func() core.Node { return padRoot(kit.NewTitle("Title", 3).Node(), 200, 56) }},
		{"ctl_paragraph", 200, 48, func() core.Node { return padRoot(kit.NewParagraph("Para").Node(), 200, 48) }},
		{"ctl_space", 160, 48, func() core.Node {
			return padRoot(kit.NewSpace(kit.NewTag("A").Node(), kit.NewTag("B").Node()).Node(), 160, 48)
		}},
		{"ctl_divider", 160, 24, func() core.Node { return padRoot(kit.NewDivider().Node(), 160, 24) }},
		{"ctl_flex", 160, 40, func() core.Node {
			return padRoot(kit.NewFlexRow(kit.NewText("a").Node(), kit.NewText("b").Node()).Node(), 160, 40)
		}},
		{"ctl_grid", 180, 80, func() core.Node {
			return padRoot(kit.NewGridCols(2,
				kit.NewText("1").Node(), kit.NewText("2").Node(),
				kit.NewText("3").Node(), kit.NewText("4").Node(),
			).Node(), 180, 80)
		}},
		{"ctl_tag", 100, 40, func() core.Node { return padRoot(kit.NewTag("Tag").Node(), 100, 40) }},
		{"ctl_alert", 280, 64, func() core.Node {
			a := kit.NewAlert("Alert")
			a.SetType("info")
			return padRoot(a.Node(), 280, 64)
		}},
		{"ctl_card", 200, 100, func() core.Node {
			c := kit.NewCard("Card")
			c.SetContent(kit.NewText("body").Node())
			return padRoot(c.Node(), 200, 100)
		}},
		{"ctl_empty", 160, 100, func() core.Node { return padRoot(kit.NewEmpty("none").Node(), 160, 100) }},
		{"ctl_avatar", 56, 56, func() core.Node { return padRoot(kit.NewAvatar("AB").Node(), 56, 56) }},
		{"ctl_badge", 100, 48, func() core.Node {
			return padRoot(kit.NewBadge(kit.NewButton("").Node(), 5).Node(), 100, 48)
		}},
		{"ctl_input", 220, 48, func() core.Node {
			in := kit.NewInput("")
			in.SetFixedSize(200, 32)
			return padRoot(in.Node(), 220, 48)
		}},
		{"ctl_input_number", 180, 48, func() core.Node { return padRoot(kit.NewInputNumber(3).Node(), 180, 48) }},
		{"ctl_checkbox", 120, 40, func() core.Node { return padRoot(kit.NewCheckbox("CB").Node(), 120, 40) }},
		{"ctl_radio", 120, 40, func() core.Node { return padRoot(kit.NewRadio("v", "R").Node(), 120, 40) }},
		{"ctl_switch", 80, 40, func() core.Node { return padRoot(kit.NewSwitch().Node(), 80, 40) }},
		{"ctl_select", 260, 48, func() core.Node {
			return padRoot(kit.NewSelect("pick", kit.SelectOption{Value: "1", Label: "One"}).Node(), 260, 48)
		}},
		{"ctl_rate", 160, 40, func() core.Node { return padRoot(kit.NewRate(3).Node(), 160, 40) }},
		{"ctl_segmented", 200, 48, func() core.Node { return padRoot(kit.NewSegmented("A", "B").Node(), 200, 48) }},
		{"ctl_slider", 220, 40, func() core.Node { return padRoot(kit.NewSlider(50).Node(), 220, 40) }},
		{"ctl_steps", 320, 48, func() core.Node {
			s := kit.NewSteps("A", "B", "C")
			s.SetCurrent(1)
			return padRoot(s.Node(), 320, 48)
		}},
		{"ctl_breadcrumb", 240, 40, func() core.Node {
			return padRoot(kit.NewBreadcrumb("Home", "List").Node(), 240, 40)
		}},
		{"ctl_progress", 220, 32, func() core.Node { return padRoot(kit.NewProgress(65).Node(), 220, 32) }},
		{"ctl_spin", 80, 80, func() core.Node { return padRoot(kit.NewSpin(nil).Node(), 80, 80) }},
		{"ctl_skeleton", 200, 40, func() core.Node { return padRoot(kit.NewSkeleton(160, 16).Node(), 200, 40) }},
		{"ctl_result", 240, 160, func() core.Node {
			return padRoot(kit.NewResult("success", "OK", "sub").Node(), 240, 160)
		}},
		{"ctl_scroll", 140, 100, func() core.Node {
			col := primitive.Column()
			for i := 0; i < 6; i++ {
				b := primitive.NewBox()
				b.Width, b.Height = 100, 28
				b.Color = render.RGBA{R: 0.9, G: 0.9, B: 0.95, A: 1}
				col.AddChild(b)
			}
			sc := kit.NewScroll(col)
			sc.SetSize(120, 80)
			sc.SetScrollbarVisibility(primitive.ScrollbarAuto) // visual: bars always while overflow
			return padRoot(sc.Node(), 140, 100)
		}},
		{"ctl_calendar", 280, 280, func() core.Node { return padRoot(kit.NewCalendar(2026, 7).Node(), 280, 280) }},
		{"ctl_image", 140, 100, func() core.Node { return padRoot(kit.NewImage("img", 100, 60).Node(), 140, 100) }},
		{"ctl_qrcode", 150, 150, func() core.Node { return padRoot(kit.NewQRCode("gpui").Node(), 150, 150) }},
		{"ctl_statistic", 140, 80, func() core.Node {
			return padRoot(kit.NewStatistic("N", "128").Node(), 140, 80)
		}},
		{"ctl_timeline", 200, 120, func() core.Node {
			return padRoot(kit.NewTimeline(
				kit.TimelineItem{Label: "A"}, kit.TimelineItem{Label: "B"},
			).Node(), 200, 120)
		}},
		{"ctl_descriptions", 220, 80, func() core.Node {
			return padRoot(kit.NewDescriptions([2]string{"K", "V"}).Node(), 220, 80)
		}},
		{"ctl_list", 160, 100, func() core.Node { return padRoot(kit.NewList("a", "b", "c").Node(), 160, 100) }},
		{"ctl_pagination", 200, 40, func() core.Node { return padRoot(kit.NewPagination(5).Node(), 200, 40) }},
		{"ctl_menu", 160, 120, func() core.Node {
			return padRoot(kit.NewMenu(
				kit.MenuItem{Key: "a", Label: "A"}, kit.MenuItem{Key: "b", Label: "B"},
			).Node(), 160, 120)
		}},
		{"ctl_anchor", 120, 80, func() core.Node { return padRoot(kit.NewAnchor("#a", "#b").Node(), 120, 80) }},
		{"ctl_color_picker", 200, 48, func() core.Node { return padRoot(kit.NewColorPicker().Node(), 200, 48) }},
		{"ctl_tabs", 320, 160, func() core.Node {
			tabs := kit.NewTabs(kit.MenuItem{Key: "a", Label: "A"}, kit.MenuItem{Key: "b", Label: "B"})
			tabs.SetContent("a", kit.NewText("pa").Node())
			tabs.SetContent("b", kit.NewText("pb").Node())
			box := primitive.NewBox(tabs.Node())
			box.Width, box.Height = 300, 140
			return padRoot(box, 320, 160)
		}},
		{"ctl_collapse", 240, 120, func() core.Node {
			c := kit.NewCollapse(kit.CollapsePanel{Key: "1", Header: "H", Content: kit.NewText("c").Node()})
			c.SetActive("1")
			return padRoot(c.Node(), 240, 120)
		}},
		{"ctl_carousel", 200, 100, func() core.Node {
			return padRoot(kit.NewCarousel(kit.NewText("1").Node(), kit.NewText("2").Node()).Node(), 200, 100)
		}},
		{"ctl_layout", 280, 160, func() core.Node {
			return padRoot(kit.NewLayout(
				kit.NewText("H").Node(), kit.NewText("S").Node(),
				kit.NewText("C").Node(), kit.NewText("F").Node(),
			).Node(), 280, 160)
		}},
		{"ctl_watermark", 180, 80, func() core.Node {
			return padRoot(kit.NewWatermark(kit.NewText("body").Node(), "WM").Node(), 180, 80)
		}},
		{"ctl_upload", 120, 48, func() core.Node { return padRoot(kit.NewUpload("Upload").Node(), 120, 48) }},
		{"ctl_tree_select", 220, 48, func() core.Node {
			return padRoot(kit.NewTreeSelect("path", "a/b").Node(), 220, 48)
		}},
		{"ctl_auto_complete", 220, 120, func() core.Node {
			return padRoot(kit.NewAutoComplete("ac", "Apple", "Banana").Node(), 220, 120)
		}},
		{"ctl_modal_host", 80, 40, func() core.Node { return padRoot(kit.NewModal("M").Node(), 80, 40) }},
		{"ctl_drawer_host", 80, 40, func() core.Node { return padRoot(kit.NewDrawer("D").Node(), 80, 40) }},
		{"ctl_message_host", 80, 40, func() core.Node { return padRoot(kit.NewMessageHost().Node(), 80, 40) }},
		{"ctl_affix", 120, 40, func() core.Node {
			return padRoot(kit.NewAffix(kit.NewText("affix").Node()).Node(), 120, 40)
		}},
		{"ctl_config_provider", 120, 40, func() core.Node {
			return padRoot(kit.NewConfigProvider(kit.DefaultTheme(), kit.NewText("cfg").Node()).Node(), 120, 40)
		}},
		{"ctl_splitter", 200, 80, func() core.Node {
			return padRoot(kit.NewSplitter(kit.NewText("L").Node(), kit.NewText("R").Node()).Node(), 200, 80)
		}},
		{"ctl_table", 240, 100, func() core.Node {
			return padRoot(kit.NewTable(
				[]kit.TableColumn{{Key: "n", Title: "N"}},
				[]map[string]string{{"n": "a"}},
			).Node(), 240, 100)
		}},
		{"ctl_tree", 160, 80, func() core.Node {
			return padRoot(kit.NewTree(&kit.TreeNode{Key: "r", Title: "root"}).Node(), 160, 80)
		}},
		{"ctl_transfer", 280, 120, func() core.Node {
			return padRoot(kit.NewTransfer([]string{"a", "b"}).Node(), 280, 120)
		}},
		{"ctl_cascader", 200, 48, func() core.Node {
			return padRoot(kit.NewCascader(&kit.TreeNode{Key: "r", Title: "r"}).Node(), 200, 48)
		}},
		{"ctl_date_picker", 300, 320, func() core.Node { return padRoot(kit.NewDatePicker().Node(), 300, 320) }},
		{"ctl_time_picker", 280, 48, func() core.Node { return padRoot(kit.NewTimePicker().Node(), 280, 48) }},
		{"ctl_mentions", 220, 120, func() core.Node {
			return padRoot(kit.NewMentions("@", "alice").Node(), 220, 120)
		}},
		{"ctl_dropdown", 160, 48, func() core.Node {
			return padRoot(kit.NewDropdown("DD", kit.MenuItem{Key: "1", Label: "One"}).Node(), 160, 48)
		}},
		{"ctl_form", 240, 80, func() core.Node {
			f := kit.NewForm(core.NewFormModel())
			f.AddItem(kit.NewFormItem("n", "Name", kit.NewInput("").Node()))
			return padRoot(f.Node(), 240, 80)
		}},
		{"ctl_textarea", 220, 80, func() core.Node { return padRoot(kit.NewTextArea("ta", 3).Node(), 220, 80) }},
		{"ctl_popconfirm", 140, 48, func() core.Node {
			return padRoot(kit.NewPopconfirm(kit.NewButton("?").Node(), "sure?").Node(), 140, 48)
		}},
		{"ctl_tooltip", 140, 48, func() core.Node {
			return padRoot(kit.NewTooltip(kit.NewButton("tip").Node(), "hello").Node(), 140, 48)
		}},
		{"ctl_popover", 140, 48, func() core.Node {
			return padRoot(kit.NewPopover(kit.NewButton("pop").Node(), kit.NewText("body").Node()).Node(), 140, 48)
		}},
		{"ctl_tour", 100, 40, func() core.Node {
			return padRoot(kit.NewTour(kit.TourStep{Title: "t", Body: "b"}).Node(), 100, 40)
		}},
	}
}

func TestVisual_PerControl(t *testing.T) {
	specs := perControlSpecs()
	if len(specs) < 40 {
		t.Fatalf("per-control visual specs=%d want ≥40", len(specs))
	}
	for _, sp := range specs {
		sp := sp
		t.Run(sp.id, func(t *testing.T) {
			img := visualtest.CaptureTree(sp.w, sp.h, sp.root(), kit.DefaultTheme())
			if img == nil {
				t.Fatal("nil image")
			}
			if !imageHasInk(img) {
				t.Fatalf("%s: blank (no non-white pixels)", sp.id)
			}
			visualtest.AssertScenario(t, sp.id, img, visualtest.DefaultCompare)
		})
	}
}

func imageHasInk(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a > 0 && (r>>8 < 250 || g>>8 < 250 || bl>>8 < 250) {
				return true
			}
		}
	}
	return false
}
