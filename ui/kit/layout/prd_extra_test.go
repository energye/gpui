package layout_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/kit/layout"
	"github.com/energye/gpui/ui/rendering"
)

// P0 hasSider can force row direction for SSR parity; explicit flag wins
// over child inspection (§6.3 hasSider, §6.9 LAY-02/11 companion).
func TestLayout_PRD_LAY02_HasSiderForced(t *testing.T) {
	plain := layout.NewLayout(layout.NewContent())
	plain.SetHasSider(true)
	if !plain.IsRow() {
		t.Fatal("SetHasSider(true) must force row direction")
	}
	withSider := layout.NewLayout(layout.NewSider(), layout.NewContent())
	withSider.SetHasSider(false)
	if withSider.IsRow() {
		t.Fatal("explicit SetHasSider(false) must win over child sider")
	}
	if sz := plain.Layout(rendering.Tight(800, 400)); sz.Width != 800 || sz.Height != 400 {
		t.Fatalf("forced-row layout=%v", sz)
	}
	if sz := withSider.Layout(rendering.Tight(800, 400)); sz.Width != 800 || sz.Height != 400 {
		t.Fatalf("forced-column layout=%v", sz)
	}
}

// P0 light-theme sider + reverseArrow right-sider mapping (§6.9 LAY-06/LAY-11
// companions): light shell follows the container token, arrow flips.
func TestLayout_PRD_LAY06_ThemeLightReverse(t *testing.T) {
	s := layout.NewSider()
	s.SetTheme(layout.SiderThemeLight)
	s.SetReverseArrow(true)
	if s.Theme() != layout.SiderThemeLight || !s.ReverseArrow() {
		t.Fatal("theme light / reverse flags must stick")
	}
	// Default left sider: expanded points left, collapsed points right.
	// Right sider (reverseArrow): mirrored — expanded points right.
	if s.ArrowPointsLeft() {
		t.Fatal("reverse arrow on uncollapsed right sider must point right")
	}
	s.SetCollapsible(true)
	s.ActivateTrigger()
	if !s.ArrowPointsLeft() {
		t.Fatal("collapsed reverse arrow must point left")
	}
}

// P0 nested double sider + header-through mapping from the §3 sample
// (§6.9 LAY-02 companion): left+right siders keep 200 with content between.
func TestLayout_PRD_LAY11_NestedDoubleSider(t *testing.T) {
	left := layout.NewSider()
	right := layout.NewSider()
	c := layout.NewContent()
	inner := layout.NewLayout(left, c, right)
	outer := layout.NewLayout(layout.NewHeader(), inner, layout.NewFooter())
	outer.Layout(rendering.Tight(1200, 800))
	lx, ly, lw, lh := absRect(left.Node())
	cx, cy, cw, ch := absRect(c.Node())
	rx, ry, rw, rh := absRect(right.Node())
	mustNoOverlap(t, [][4]float64{{lx, ly, lw, lh}, {cx, cy, cw, ch}, {rx, ry, rw, rh}})
	if math.Abs(lw-200) > 0.5 || math.Abs(rw-200) > 0.5 {
		t.Fatalf("siders %v/%v want 200/200", lw, rw)
	}
	if math.Abs(cw-800) > 0.5 {
		t.Fatalf("content w=%v want 800", cw)
	}
}

// P0 trigger=null hides the bar but the sider still folds via SetCollapsed
// (custom-trigger demo: external node drives collapse, §6.9 LAY-14 companion).
func TestLayout_PRD_LAY14_HideTriggerFolds(t *testing.T) {
	s := layout.NewSider()
	s.SetCollapsible(true)
	s.SetHideTrigger()
	if s.TriggerVisible() || s.TriggerRole() != "" {
		t.Fatal("hidden trigger must report invisible with no role")
	}
	s.SetCollapsed(true)
	if !s.CollapsedState() {
		t.Fatal("hidden-trigger sider must still fold")
	}
	if sz := s.Layout(rendering.Loose(1000, 800)); math.Abs(sz.Width-80) > 0.5 {
		t.Fatalf("w=%v want 80", sz.Width)
	}
	s.ShowTrigger()
	if !s.TriggerVisible() {
		t.Fatal("ShowTrigger must restore the default bar")
	}
}

// P0 custom trigger node life-cycle plus hover/zero chrome (§6.5 trigger
// states, §6.9 LAY-14/LAY-15 companions).
func TestLayout_PRD_LAY15_CustomTriggerChrome(t *testing.T) {
	s := layout.NewSider()
	s.SetCollapsible(true)
	custom := rendering.NewRenderColorBox(48, 48, 1, 0.75, 0.2, 1)
	s.SetTrigger(custom)
	if !s.TriggerVisible() || s.TriggerRole() != "button" {
		t.Fatal("custom trigger must stay a visible button")
	}
	s.SetTriggerHovered(true)
	if !s.TriggerHovered() {
		t.Fatal("hover feedback must stick")
	}
	if !s.HitTrigger(100, s.Node().Size().Height) {
		// Node has no size pre-layout; fall back to geometric probe.
		s.Layout(rendering.Loose(1000, 800))
	}
	sz := s.Layout(rendering.Loose(1000, 800))
	if !s.HitTrigger(sz.Width-1, sz.Height-1) {
		t.Fatal("trigger hit must cover the bottom bar")
	}
	if s.HitTrigger(-1, -1) {
		t.Fatal("outside point must miss the trigger")
	}
}

// L3 responsive callback pairing (onCollapse responsive + breakpoint flip,
// §6.9 LAY-16 companion): shrinking then growing the viewport fires twice
// and leaves the persisted broken flag consistent.
func TestLayout_PRD_LAY16_BreakpointBack(t *testing.T) {
	s := layout.NewSider()
	s.SetBreakpoint(layout.LayoutBreakpointMD)
	var collapses []layout.CollapseType
	s.SetOnCollapse(func(_ bool, typ layout.CollapseType) { collapses = append(collapses, typ) })
	s.SetViewportWidth(700)
	if !s.CollapsedState() {
		t.Fatal("700 < 768 must collapse")
	}
	s.SetViewportWidth(1200)
	if s.CollapsedState() {
		t.Fatal("1200 must expand again")
	}
	if len(collapses) != 2 || collapses[0] != layout.CollapseResponsive || collapses[1] != layout.CollapseResponsive {
		t.Fatalf("collapses=%v want 2x responsive", collapses)
	}
}

// Layout matrix companion (acceptance gate piece 2): Exact/Min-Max loosen
// the same four-region tree without shrinking the sider or header.
func TestLayout_PRD_LAY02_LayoutMatrix(t *testing.T) {
	h := layout.NewHeader()
	s := layout.NewSider()
	c := layout.NewContent()
	f := layout.NewFooter()
	inner := layout.NewLayout(s, c)
	outer := layout.NewLayout(h, inner, f)
	exact := outer.Layout(rendering.Tight(1200, 800))
	if exact.Width != 1200 || exact.Height != 800 {
		t.Fatalf("exact=%v want 1200x800", exact)
	}
	mm := outer.Layout(rendering.Constraints{MinWidth: 900, MaxWidth: 1200, MinHeight: 600, MaxHeight: 800})
	if mm.Width < 900 || mm.Width > 1200 || mm.Height < 600 || mm.Height > 800 {
		t.Fatalf("minmax=%v want 900..1200 x 600..800", mm)
	}
	if math.Abs(h.Node().Size().Height-64) > 0.5 {
		t.Fatalf("header h=%v want 64", h.Node().Size().Height)
	}
	if math.Abs(s.Node().Size().Width-200) > 0.5 {
		t.Fatalf("sider w=%v want 200", s.Node().Size().Width)
	}
}
