package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestScrollKitAPI(t *testing.T) {
	inner := primitive.Column()
	for i := 0; i < 10; i++ {
		b := primitive.NewBox()
		b.Height = 40
		b.Width = 100
		inner.AddChild(b)
	}
	sc := kit.NewScroll(inner)
	sc.SetSize(120, 80)
	_ = sc.Node().Layout(core.Tight(120, 80))
	if !sc.Viewport().OverflowY() {
		t.Fatal("want overflow")
	}
	sc.Viewport().HandleScroll(&core.ScrollEvent{DY: 30})
	if sc.Viewport().ScrollY < 29 {
		t.Fatalf("scrollY=%v", sc.Viewport().ScrollY)
	}
}

func TestSpaceGap(t *testing.T) {
	a := kit.NewText("A").Node()
	b := kit.NewText("B").Node()
	sp := kit.NewSpace(a, b)
	sp.SetSizePx(12)
	_ = sp.Node().Layout(core.Loose(400, 100))
	if sp.Root.Gap != 12 {
		t.Fatalf("gap=%v", sp.Root.Gap)
	}
}

func TestTagAlertCardEmpty(t *testing.T) {
	tag := kit.NewTag("hello")
	_ = tag.Node().Layout(core.Loose(200, 40))
	if tag.Root.Size().Width < 10 {
		t.Fatal("tag size")
	}
	al := kit.NewAlert("note")
	al.SetType("success")
	_ = al.Node().Layout(core.Loose(400, 80))
	card := kit.NewCard("Title")
	card.SetContent(kit.NewText("body").Node())
	_ = card.Node().Layout(core.Loose(300, 200))
	em := kit.NewEmpty("nothing")
	_ = em.Node().Layout(core.Loose(200, 200))
}

func TestInputNumberRateSegmented(t *testing.T) {
	n := kit.NewInputNumber(3)
	n.SetValue(5)
	if n.Value != 5 {
		t.Fatalf("value=%v", n.Value)
	}
	r := kit.NewRate(2)
	r.SetValue(4)
	if r.Value != 4 {
		t.Fatalf("rate=%d", r.Value)
	}
	s := kit.NewSegmented("Daily", "Weekly", "Monthly")
	s.SetValue("Weekly")
	if s.Value != "Weekly" {
		t.Fatalf("seg=%q", s.Value)
	}
	_ = s.Node().Layout(core.Loose(400, 40))
}

func TestStepsBreadcrumbBadgeAvatar(t *testing.T) {
	st := kit.NewSteps("A", "B", "C")
	st.SetCurrent(1)
	_ = st.Node().Layout(core.Loose(500, 40))
	bc := kit.NewBreadcrumb("Home", "List", "Detail")
	_ = bc.Node().Layout(core.Loose(400, 30))
	av := kit.NewAvatar("U")
	_ = av.Node().Layout(core.Loose(40, 40))
	if av.Root.Size().Width != 32 {
		t.Fatalf("avatar w=%v", av.Root.Size().Width)
	}
	btn := kit.NewButton("msg")
	badge := kit.NewBadge(btn.Node(), 5)
	_ = badge.Node().Layout(core.Loose(100, 40))
}

func TestDividerKit(t *testing.T) {
	d := kit.NewDivider()
	_ = d.Node().Layout(core.Loose(200, 10))
	d.SetVertical(true)
}

func TestCollapseTimelineSliderResult(t *testing.T) {
	c := kit.NewCollapse(
		kit.CollapsePanel{Key: "1", Header: "H1", Content: kit.NewText("c1").Node()},
		kit.CollapsePanel{Key: "2", Header: "H2", Content: kit.NewText("c2").Node()},
	)
	c.SetActive("1")
	_ = c.Node().Layout(core.Loose(400, 300))
	tl := kit.NewTimeline(
		kit.TimelineItem{Label: "Created"},
		kit.TimelineItem{Label: "Shipped"},
	)
	_ = tl.Node().Layout(core.Loose(300, 200))
	sl := kit.NewSlider(40)
	sl.SetValue(70)
	if sl.Value != 70 {
		t.Fatalf("slider=%v", sl.Value)
	}
	_ = sl.Node().Layout(core.Loose(220, 20))
	res := kit.NewResult("success", "Done", "ok")
	_ = res.Node().Layout(core.Loose(400, 200))
	st := kit.NewStatistic("Users", "1,280")
	_ = st.Node().Layout(core.Loose(120, 80))
	ds := kit.NewDescriptions([2]string{"Name", "Ada"}, [2]string{"Role", "Admin"})
	_ = ds.Node().Layout(core.Loose(300, 80))
	fb := kit.NewFloatButton()
	fb.SetAriaLabel("fab")
	_ = fb.Node().Layout(core.Loose(60, 60))
}

func TestCatalogRestAPIs(t *testing.T) {
	ac := kit.NewAutoComplete("type", "Apple", "Apricot", "Banana")
	ac.SetValue("Ap")
	_ = ac.Node().Layout(core.Loose(240, 200))
	cal := kit.NewCalendar(2026, 7)
	_ = cal.Node().Layout(core.Loose(300, 280))
	dp := kit.NewDatePicker()
	_ = dp.Node().Layout(core.Loose(300, 320))
	tp := kit.NewTimePicker()
	_ = tp.Node().Layout(core.Loose(300, 40))
	car := kit.NewCarousel(kit.NewText("1").Node(), kit.NewText("2").Node())
	car.SetIndex(1)
	_ = car.Node().Layout(core.Loose(200, 100))
	img := kit.NewImage("photo", 100, 60)
	_ = img.Node().Layout(core.Loose(100, 60))
	qr := kit.NewQRCode("hello")
	_ = qr.Node().Layout(core.Loose(128, 128))
	wm := kit.NewWatermark(kit.NewText("content").Node(), "CONFIDENTIAL")
	_ = wm.Node().Layout(core.Loose(200, 100))
	cp := kit.NewColorPicker()
	_ = cp.Node().Layout(core.Loose(200, 40))
	ts := kit.NewTreeSelect("path", "a/b", "a/c")
	_ = ts.Node().Layout(core.Loose(200, 40))
	up := kit.NewUpload("Upload")
	_ = up.Node().Layout(core.Loose(100, 40))
	an := kit.NewAnchor("#Intro", "#API")
	_ = an.Node().Layout(core.Loose(120, 80))
	af := kit.NewAffix(kit.NewText("sticky").Node())
	_ = af.Node().Layout(core.Loose(100, 40))
	cfg := kit.NewConfigProvider(kit.DefaultTheme(), kit.NewText("child").Node())
	if cfg.Node() == nil {
		t.Fatal("config provider")
	}
	m := kit.NewMentions("mention", "alice", "bob")
	_ = m.Node().Layout(core.Loose(200, 120))
}
