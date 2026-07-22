package visualtest_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	"github.com/energye/gpui/ui/visualtest"
)

func frame(child core.Node, w, h float64) *primitive.Box {
	b := primitive.NewBox(child)
	b.Width, b.Height = w, h
	b.Padding = primitive.All(8)
	b.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	return b
}

func captureAssert(t *testing.T, name string, root core.Node, w, h int) {
	t.Helper()
	img := visualtest.CaptureTree(w, h, root, kit.DefaultTheme())
	if img == nil {
		t.Fatalf("%s: nil image", name)
	}
	// Prefer AssertScenario when goldens exist; first run UPDATE_VISUAL=1.
	visualtest.AssertScenario(t, name, img, visualtest.DefaultCompare)
}

func TestVisual_TagAlertCard(t *testing.T) {
	tag := kit.NewTag("Tag")
	al := kit.NewAlert("Alert")
	card := kit.NewCard("Card")
	card.SetContent(kit.NewText("body").Node())
	col := primitive.Column(tag.Node(), al.Node(), card.Node())
	col.Gap = 8
	captureAssert(t, "catalog_tag_alert_card", frame(col, 280, 200), 280, 200)
}

func TestVisual_SpaceAvatarBadge(t *testing.T) {
	sp := kit.NewSpace(kit.NewAvatar("A").Node(), kit.NewBadge(kit.NewButton("").Node(), 3).Node())
	captureAssert(t, "catalog_space_avatar_badge", frame(sp.Node(), 200, 80), 200, 80)
}

func TestVisual_StepsBreadcrumb(t *testing.T) {
	st := kit.NewSteps("A", "B", "C")
	st.SetCurrent(1)
	bc := kit.NewBreadcrumb("Home", "List")
	col := primitive.Column(st.Node(), bc.Node())
	col.Gap = 12
	captureAssert(t, "catalog_steps_breadcrumb", frame(col, 360, 100), 360, 100)
}

func TestVisual_SegmentedRateSlider(t *testing.T) {
	seg := kit.NewSegmented("A", "B")
	rate := kit.NewRate(3)
	sl := kit.NewSlider(50)
	col := primitive.Column(seg.Node(), rate.Node(), sl.Node())
	col.Gap = 12
	captureAssert(t, "catalog_seg_rate_slider", frame(col, 280, 120), 280, 120)
}

func TestVisual_ScrollOverflow(t *testing.T) {
	inner := primitive.Column()
	for i := 0; i < 8; i++ {
		b := primitive.NewBox()
		b.Width, b.Height = 100, 30
		b.Color = render.RGBA{R: 0.9, G: 0.9, B: 0.95, A: 1}
		inner.AddChild(b)
	}
	sc := kit.NewScroll(inner)
	sc.SetSize(120, 80)
	captureAssert(t, "catalog_scroll", frame(sc.Node(), 140, 100), 140, 100)
}

func TestVisual_EmptyResultProgress(t *testing.T) {
	em := kit.NewEmpty("none")
	res := kit.NewResult("info", "Info", "sub")
	prog := kit.NewProgress(70)
	col := primitive.Column(em.Node(), res.Node(), prog.Node())
	col.Gap = 8
	captureAssert(t, "catalog_empty_result_progress", frame(col, 320, 280), 320, 280)
}

func TestVisual_CalendarImageQR(t *testing.T) {
	cal := kit.NewCalendar(2026, 7)
	img := kit.NewImage("img", 80, 48)
	qr := kit.NewQRCode("x")
	col := primitive.Column(cal.Node(), img.Node(), qr.Node())
	col.Gap = 8
	captureAssert(t, "catalog_cal_img_qr", frame(col, 320, 360), 320, 360)
}
