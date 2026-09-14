package layout_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/layout"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type layoutFile struct {
	HeaderHeight    float64 `json:"headerHeight"`
	SiderWidth      float64 `json:"siderWidth"`
	CollapsedWidth  float64 `json:"collapsedWidth"`
	TriggerHeight   float64 `json:"triggerHeight"`
	ZeroTriggerSize float64 `json:"zeroTriggerSize"`
	FooterHeight    float64 `json:"footerHeight"`
	SiderDarkBg     string  `json:"siderDarkBg"`
	TriggerDarkBg   string  `json:"triggerDarkBg"`
}

func loadLayoutFile(t *testing.T) layoutFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "layout.json"))
	if err != nil {
		t.Fatalf("read layout.json: %v", err)
	}
	var f layoutFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse layout.json: %v", err)
	}
	return f
}

func absRect(n rendering.RenderObject) (x, y, w, h float64) {
	if n == nil {
		return 0, 0, 0, 0
	}
	sz := n.Size()
	w, h = sz.Width, sz.Height
	xx, yy := 0.0, 0.0
	for cur := n; cur != nil; cur = cur.Parent() {
		o := cur.Offset()
		xx += o.X
		yy += o.Y
	}
	return xx, yy, w, h
}

func rectsOverlap(ax, ay, aw, ah, bx, by, bw, bh float64) bool {
	return ax < bx+bw-1e-9 && bx < ax+aw-1e-9 && ay < by+bh-1e-9 && by < ay+ah-1e-9
}

func mustNoOverlap(t *testing.T, rects [][4]float64) {
	t.Helper()
	for i := 0; i < len(rects); i++ {
		if rects[i][2] <= 0 || rects[i][3] <= 0 {
			t.Fatalf("rect %d size %vx%v want >0", i, rects[i][2], rects[i][3])
		}
		for j := i + 1; j < len(rects); j++ {
			if rectsOverlap(rects[i][0], rects[i][1], rects[i][2], rects[i][3], rects[j][0], rects[j][1], rects[j][2], rects[j][3]) {
				t.Fatalf("rect %d overlaps %d", i, j)
			}
		}
	}
}

func TestLayout_PRD_LAY01(t *testing.T) {
	l := layout.NewLayout()
	if l.IsRow() {
		t.Fatal("default layout without sider should be column")
	}
	h := layout.NewHeader()
	if math.Abs(h.EffectiveHeight()-64) > 0.5 {
		t.Fatalf("header=%v want 64", h.EffectiveHeight())
	}
	s := layout.NewSider()
	if math.Abs(s.EffectiveWidth()-200) > 0.5 {
		t.Fatalf("sider=%v want 200", s.EffectiveWidth())
	}
	if math.Abs(s.EffectiveCollapsedWidth()-80) > 0.5 {
		t.Fatalf("collapsed=%v want 80", s.EffectiveCollapsedWidth())
	}
	if s.Collapsible() || s.CollapsedState() {
		t.Fatal("collapsible/collapsed defaults")
	}
	if s.Theme() != layout.SiderThemeDark || s.ReverseArrow() {
		t.Fatal("theme dark / reverse defaults")
	}
	if s.TriggerVisible() {
		t.Fatal("trigger hidden when not collapsible")
	}
	tok := theme.Default.Current()
	if math.Abs(tok.ControlHeight*2-64) > 1e-9 {
		t.Fatalf("token controlHeight*2=%v want 64", tok.ControlHeight*2)
	}
	sz := l.Layout(rendering.Tight(1200, 800))
	if math.Abs(sz.Width-1200) > 0.5 || math.Abs(sz.Height-800) > 0.5 {
		t.Fatalf("layout=%v", sz)
	}
}

func TestLayout_PRD_LAY02(t *testing.T) {
	h := layout.NewHeader()
	s := layout.NewSider()
	c := layout.NewContent()
	f := layout.NewFooter()
	inner := layout.NewLayout(s, c)
	outer := layout.NewLayout(h, inner, f)
	outer.Layout(rendering.Tight(1200, 800))
	hx, hy, hw, hh := absRect(h.Node())
	sx, sy, sw, sh := absRect(s.Node())
	cx, cy, cw, ch := absRect(c.Node())
	fx, fy, fw, fh := absRect(f.Node())
	mustNoOverlap(t, [][4]float64{{hx, hy, hw, hh}, {sx, sy, sw, sh}, {cx, cy, cw, ch}, {fx, fy, fw, fh}})
}

func TestLayout_PRD_LAY03(t *testing.T) {
	h := layout.NewHeader()
	sz := h.Layout(rendering.Loose(1200, 800))
	if math.Abs(sz.Height-64) > 0.5 {
		t.Fatalf("header h=%v want 64", sz.Height)
	}
}

func TestLayout_PRD_LAY04(t *testing.T) {
	s := layout.NewSider()
	sz := s.Layout(rendering.Loose(1000, 800))
	if math.Abs(sz.Width-200) > 0.5 {
		t.Fatalf("sider w=%v want 200", sz.Width)
	}
}

func TestLayout_PRD_LAY05(t *testing.T) {
	s := layout.NewSider()
	s.SetCollapsible(true)
	var got []struct {
		collapsed bool
		typ       layout.CollapseType
	}
	s.SetOnCollapse(func(c bool, typ layout.CollapseType) {
		got = append(got, struct {
			collapsed bool
			typ       layout.CollapseType
		}{c, typ})
	})
	s.ActivateTrigger()
	if !s.CollapsedState() {
		t.Fatal("should collapse")
	}
	sz := s.Layout(rendering.Loose(1000, 800))
	if math.Abs(sz.Width-80) > 0.5 {
		t.Fatalf("collapsed w=%v want 80", sz.Width)
	}
	if len(got) != 1 || !got[0].collapsed || got[0].typ != layout.CollapseClickTrigger {
		t.Fatalf("onCollapse=%+v want 1x (true,click)", got)
	}
}

func TestLayout_PRD_LAY06(t *testing.T) {
	s := layout.NewSider()
	want := render.Hex("#001529")
	if got := s.EffectiveBackground(); got != want {
		t.Fatalf("dark bg=%+v want %+v", got, want)
	}
	tok := theme.Default.Current()
	_ = tok.ColorBgLayout
}

func TestLayout_PRD_LAY07(t *testing.T) {
	s := layout.NewSider()
	s.SetCollapsible(true)
	s.ActivateTrigger()
	if !s.CollapsedState() {
		t.Fatal("should collapse first")
	}
	s.ActivateTrigger()
	if s.CollapsedState() {
		t.Fatal("should expand")
	}
	sz := s.Layout(rendering.Loose(1000, 800))
	if math.Abs(sz.Width-200) > 0.5 {
		t.Fatalf("expanded w=%v want 200", sz.Width)
	}
}

func TestLayout_PRD_LAY08(t *testing.T) {
	s := layout.NewSider()
	s.SetBreakpoint(layout.LayoutBreakpointMD)
	s.SetViewportWidth(700)
	if !s.CollapsedState() {
		t.Fatal("viewport 700 < 768 should auto collapse")
	}
	sz := s.Layout(rendering.Loose(1000, 800))
	if math.Abs(sz.Width-80) > 0.5 {
		t.Fatalf("auto collapsed w=%v want 80", sz.Width)
	}
}

func TestLayout_PRD_LAY09(t *testing.T) {
	h := layout.NewHeader()
	c := layout.NewContent()
	f := layout.NewFooter()
	l := layout.NewLayout(h, c, f)
	l.Layout(rendering.Tight(1200, 800))
	hx, hy, hw, hh := absRect(h.Node())
	cx, cy, cw, ch := absRect(c.Node())
	fx, fy, fw, fh := absRect(f.Node())
	if math.Abs(hh-64) > 0.5 {
		t.Fatalf("header h=%v want 64", hh)
	}
	mustNoOverlap(t, [][4]float64{{hx, hy, hw, hh}, {cx, cy, cw, ch}, {fx, fy, fw, fh}})
}

func TestLayout_PRD_LAY10(t *testing.T) {
	h := layout.NewHeader()
	c := layout.NewContent()
	f := layout.NewFooter()
	l := layout.NewLayout(h, c, f)
	l.Layout(rendering.Tight(1200, 800))
	fx, fy, fw, fh := absRect(f.Node())
	_ = fx
	_ = fw
	if math.Abs(fy+fh-800) > 0.5 {
		t.Fatalf("footer bottom=%v want 800", fy+fh)
	}
}

func TestLayout_PRD_LAY11(t *testing.T) {
	h := layout.NewHeader()
	s := layout.NewSider()
	c := layout.NewContent()
	inner := layout.NewLayout(s, c)
	outer := layout.NewLayout(h, inner)
	outer.Layout(rendering.Tight(1200, 800))
	_, _, sw, _ := absRect(s.Node())
	_, _, cw, _ := absRect(c.Node())
	if math.Abs(sw-200) > 0.5 {
		t.Fatalf("sider w=%v want 200", sw)
	}
	if math.Abs(cw-(1200-200)) > 0.5 {
		t.Fatalf("content w=%v want %v", cw, 1200-200)
	}
}

func TestLayout_PRD_LAY12(t *testing.T) {
	h := layout.NewHeader()
	s := layout.NewSider()
	c := layout.NewContent()
	inner := layout.NewLayout(s, c)
	outer := layout.NewLayout(h, inner)
	outer.Layout(rendering.Tight(1200, 800))
	hx, hy, hw, _ := absRect(h.Node())
	sx, sy, _, _ := absRect(s.Node())
	if math.Abs(hw-1200) > 0.5 || math.Abs(hx) > 0.5 || math.Abs(hy) > 0.5 {
		t.Fatalf("header through hx=%v hw=%v", hx, hw)
	}
	if sy < hy+64-0.5 {
		t.Fatalf("sider y=%v should be under header", sy)
	}
	_ = sx
}

func TestLayout_PRD_LAY13(t *testing.T) {
	s := layout.NewSider()
	s.SetCollapsible(true)
	c := layout.NewContent()
	l := layout.NewLayout(s, c)
	l.Layout(rendering.Tight(1200, 800))
	calls := 0
	s.SetOnCollapse(func(bool, layout.CollapseType) { calls++ })
	s.ActivateTrigger()
	l.Layout(rendering.Tight(1200, 800))
	_, _, sw, _ := absRect(s.Node())
	if math.Abs(sw-80) > 0.5 {
		t.Fatalf("sider w=%v want 80", sw)
	}
	if calls != 1 {
		t.Fatalf("onCollapse=%d want 1", calls)
	}
}

func TestLayout_PRD_LAY14(t *testing.T) {
	s := layout.NewSider()
	s.SetCollapsible(true)
	custom := rendering.NewRenderColorBox(48, 48, 0.1, 0.2, 0.3, 1)
	s.SetTrigger(custom)
	if !s.TriggerVisible() || s.TriggerRole() != "button" {
		t.Fatal("custom trigger should stay visible as button")
	}
	s.ActivateTrigger()
	sz := s.Layout(rendering.Loose(1000, 800))
	if math.Abs(sz.Width-80) > 0.5 {
		t.Fatalf("custom collapsed w=%v want 80", sz.Width)
	}
	found := false
	for _, ch := range s.Node().Children() {
		if ch == rendering.RenderObject(custom) {
			found = true
		}
	}
	if !found {
		t.Fatal("custom trigger node should be in tree")
	}
}

func TestLayout_PRD_LAY15(t *testing.T) {
	s := layout.NewSider()
	s.SetCollapsible(true)
	s.SetCollapsedWidth(0)
	s.SetOverlay(true)
	s.SetDefaultCollapsed(true)
	if !s.CollapsedState() || !s.IsZeroTrigger() {
		t.Fatal("zero overlay trigger state")
	}
	if math.Abs(s.FlowWidth()) > 0.5 {
		t.Fatalf("flow=%v want 0", s.FlowWidth())
	}
	c := layout.NewContent()
	l := layout.NewLayout(s, c)
	l.Layout(rendering.Tight(1200, 800))
	_, _, cw, _ := absRect(c.Node())
	if math.Abs(cw-1200) > 0.5 {
		t.Fatalf("overlay content w=%v want 1200", cw)
	}
}

func TestLayout_PRD_LAY16(t *testing.T) {
	s := layout.NewSider()
	s.SetBreakpoint(layout.LayoutBreakpointMD)
	broken := 0
	var last bool
	s.SetOnBreakpoint(func(b bool) {
		broken++
		last = b
	})
	s.SetViewportWidth(700)
	if !s.CollapsedState() {
		t.Fatal("should auto collapse")
	}
	sz := s.Layout(rendering.Loose(1000, 800))
	if math.Abs(sz.Width-80) > 0.5 {
		t.Fatalf("w=%v want 80", sz.Width)
	}
	if broken != 1 || !last {
		t.Fatalf("onBreakpoint=%d last=%v want 1x true", broken, last)
	}
}

func TestLayout_PRD_LAY17(t *testing.T) {
	f := loadLayoutFile(t)
	h := layout.NewHeader()
	s := layout.NewSider()
	s.SetCollapsible(true)
	if math.Abs(h.EffectiveHeight()-f.HeaderHeight) > 0.5 {
		t.Fatalf("header=%v want %v", h.EffectiveHeight(), f.HeaderHeight)
	}
	if math.Abs(s.EffectiveWidth()-f.SiderWidth) > 0.5 {
		t.Fatalf("sider=%v", s.EffectiveWidth())
	}
	if math.Abs(s.EffectiveCollapsedWidth()-f.CollapsedWidth) > 0.5 {
		t.Fatalf("collapsed=%v", s.EffectiveCollapsedWidth())
	}
	if math.Abs(s.EffectiveTriggerHeight()-f.TriggerHeight) > 0.5 {
		t.Fatalf("trigger=%v want %v", s.EffectiveTriggerHeight(), f.TriggerHeight)
	}
	tok := theme.Default.Current()
	wantTrigger := tok.ControlHeightLG + tok.MarginXXS*2
	if math.Abs(s.EffectiveTriggerHeight()-wantTrigger) > 0.5 {
		t.Fatalf("trigger token=%v want %v", s.EffectiveTriggerHeight(), wantTrigger)
	}
	hs := h.Layout(rendering.Loose(1200, 800))
	ss := s.Layout(rendering.Loose(1200, 800))
	if math.Abs(hs.Height-64) > 0.5 || math.Abs(ss.Width-200) > 0.5 {
		t.Fatalf("layout %v %v", hs, ss)
	}
}

func TestLayout_PRD_LAY18(t *testing.T) {
	s := layout.NewSider()
	dark := render.Hex("#001529")
	if got := s.EffectiveBackground(); got != dark {
		t.Fatalf("dark=%+v want %+v", got, dark)
	}
	s.SetTheme(layout.SiderThemeLight)
	tok := theme.Default.Current()
	wantLight := render.RGBA{R: tok.ColorBgContainer.R, G: tok.ColorBgContainer.G, B: tok.ColorBgContainer.B, A: tok.ColorBgContainer.A}
	if got := s.EffectiveBackground(); got != wantLight {
		t.Fatalf("light=%+v want %+v", got, wantLight)
	}
	l := layout.NewLayout()
	body := l.EffectiveBackground()
	if body == (render.RGBA{R: 0.09, G: 0.47, B: 1, A: 1}) {
		t.Fatal("body must not be brand blue")
	}
	f := loadLayoutFile(t)
	if f.SiderDarkBg != "#001529" || f.TriggerDarkBg != "#002140" {
		t.Fatalf("file colors %+v", f)
	}
	_ = l.Layout(rendering.Tight(400, 300))
}

func TestLayout_PRD_LAY19(t *testing.T) {
	// Disabled is N/A for layout containers: tree stays stable, no ghost state.
	h := layout.NewHeader()
	s := layout.NewSider()
	c := layout.NewContent()
	f := layout.NewFooter()
	inner := layout.NewLayout(s, c)
	outer := layout.NewLayout(h, inner, f)
	outer.Layout(rendering.Tight(1200, 800))
	if h.Focusable() || c.Focusable() || f.Focusable() || outer.Focusable() {
		t.Fatal("bands must not take focus")
	}
	dc := render.NewContext(120, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	outer.Node().Paint(rendering.NewPaintContext(dc, 1))
}

func TestLayout_PRD_LAY20(t *testing.T) {
	s := layout.NewSider()
	s.SetCollapsible(true)
	if !s.TriggerFocusable() || s.TriggerRole() != "button" {
		t.Fatal("trigger should be focusable button")
	}
	if s.TriggerAriaLabel() == "" {
		t.Fatal("trigger needs accessible name")
	}
	s.FocusTrigger()
	if !s.TriggerHasFocus() {
		t.Fatal("focused")
	}
	// Manager path: tab order plus Enter/Space activation.
	mgr := focus.NewManager()
	mgr.Register(s.TriggerFocusNode())
	if !s.TriggerFocusNode().RequestFocus() {
		t.Fatal("request focus")
	}
	if !s.TriggerFocusNode().HasFocus() {
		t.Fatal("has focus")
	}
	before := s.CollapsedState()
	if !s.HandleTriggerKey(focus.KeySpace) {
		t.Fatal("space should activate")
	}
	if s.CollapsedState() == before {
		t.Fatal("space must toggle")
	}
	if !s.HandleTriggerKey(focus.KeyEnter) {
		t.Fatal("enter should activate")
	}
	// Focus ring paints without crashing.
	s.FocusTrigger()
	s.Layout(rendering.Loose(400, 300))
	dc := render.NewContext(120, 120)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(10, 10))
}
