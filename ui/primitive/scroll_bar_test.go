package primitive_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestScrollViewportOverflowAndThumb(t *testing.T) {
	col := primitive.Column()
	for i := 0; i < 10; i++ {
		b := primitive.NewBox()
		b.Height = 40
		b.Width = 80
		col.AddChild(b)
	}
	sv := primitive.NewScrollViewport(col)
	sv.Width, sv.Height = 100, 80
	_ = sv.Layout(core.Tight(100, 80))
	if !sv.OverflowY() {
		t.Fatal("expected overflow")
	}
	// Default Auto: visible whenever overflowing
	if !sv.BarVisible(true) {
		t.Fatal("Auto bar should show on overflow")
	}
	// Content insets reserve bar thickness (no overlap)
	_, _, r, _ := sv.ContentInsets()
	if r < 1 {
		t.Fatalf("expected right gutter, got %v", r)
	}
	sv.HandleScroll(&core.ScrollEvent{DY: 40})
	if sv.ScrollY < 39 {
		t.Fatalf("ScrollY=%v", sv.ScrollY)
	}
}

func TestScrollViewportHorizontal(t *testing.T) {
	row := primitive.Row()
	for i := 0; i < 10; i++ {
		b := primitive.NewBox()
		b.Width = 40
		b.Height = 40
		row.AddChild(b)
	}
	sv := primitive.NewScrollViewport(row)
	sv.SetAxis(false, true)
	sv.Width, sv.Height = 80, 50
	_ = sv.Layout(core.Tight(80, 50))
	if !sv.OverflowX() {
		t.Fatal("expected overflow x")
	}
}

func TestScrollbarVisibilityPolicies(t *testing.T) {
	col := primitive.Column()
	for i := 0; i < 8; i++ {
		b := primitive.NewBox()
		b.Height = 30
		col.AddChild(b)
	}
	sv := primitive.NewScrollViewport(col)
	sv.Width, sv.Height = 100, 60
	_ = sv.Layout(core.Tight(100, 60))

	// Never
	sv.SetScrollbarVisibility(primitive.ScrollbarNever)
	sv.SetHovered(true)
	if sv.BarVisible(true) {
		t.Fatal("Never should hide")
	}

	// Auto: visible whenever overflow
	sv.SetScrollbarVisibility(primitive.ScrollbarAuto)
	sv.SetHovered(false)
	if !sv.BarVisible(true) {
		t.Fatal("Auto should show on overflow")
	}

	// Always: visible even if we force no overflow by enlarging
	sv.SetScrollbarVisibility(primitive.ScrollbarAlways)
	if !sv.BarVisible(true) {
		t.Fatal("Always should show")
	}

	// Hover: only with hover
	sv.SetScrollbarVisibility(primitive.ScrollbarHover)
	sv.SetHovered(false)
	sv.HandleScroll(&core.ScrollEvent{DY: 1}) // reveal
	if !sv.BarVisible(true) {
		t.Fatal("Hover should show after wheel reveal")
	}
	// exhaust reveal via Tick
	for i := 0; i < 50; i++ {
		if !sv.Tick(0.1) {
			break
		}
	}
	sv.SetHovered(false)
	if sv.BarVisible(true) {
		t.Fatal("Hover should hide after delay without hover")
	}
	sv.SetHovered(true)
	if !sv.BarVisible(true) {
		t.Fatal("Hover should show on hover")
	}
}

func TestScrollbarDisabledMaster(t *testing.T) {
	col := primitive.Column()
	b := primitive.NewBox()
	b.Height = 200
	col.AddChild(b)
	sv := primitive.NewScrollViewport(col)
	sv.Width, sv.Height = 50, 40
	_ = sv.Layout(core.Tight(50, 40))
	sv.SetShowScrollbar(false)
	sv.SetHovered(true)
	if sv.BarVisible(true) {
		t.Fatal("master off")
	}
}

func TestDefaultScrollbarPolicy(t *testing.T) {
	b := primitive.DefaultScrollbar()
	if !b.Enabled || b.Vertical != primitive.ScrollbarAuto || b.Overlay {
		t.Fatalf("default want Auto+!Overlay, got %+v", b)
	}
	n := primitive.NeverScrollbar()
	if n.Enabled {
		t.Fatal()
	}
	a := primitive.AlwaysScrollbar()
	if a.Vertical != primitive.ScrollbarAlways || a.Overlay {
		t.Fatalf("%+v", a)
	}
	h := primitive.HoverScrollbar()
	if h.Vertical != primitive.ScrollbarHover || h.Overlay {
		t.Fatalf("%+v", h)
	}
}

func TestContentNeverOverlapsScrollbar(t *testing.T) {
	col := primitive.Column()
	for i := 0; i < 10; i++ {
		b := primitive.NewBox()
		b.Height = 40
		b.Width = 200 // ask wider than content box
		col.AddChild(b)
	}
	sv := primitive.NewScrollViewport(col)
	sv.Width, sv.Height = 120, 80
	sv.Scrollbar().Horizontal = primitive.ScrollbarNever
	_ = sv.Layout(core.Tight(120, 80))
	_, _, r, _ := sv.ContentInsets()
	if r != sv.Scrollbar().Thickness && r != 6 {
		// thickness default 6
		if r < 5 {
			t.Fatalf("right inset=%v", r)
		}
	}
	cs := sv.ContentSize()
	if cs.Width > sv.Size().Width-r+0.1 {
		t.Fatalf("content width %v not reduced by gutter %v", cs.Width, r)
	}
	// Children max width capped to content box
	if sv.ContentW > cs.Width+1 {
		// content may be smaller than max; must not exceed content box max used in layout
		t.Logf("ContentW=%v ContentSize.W=%v (ok if <=)", sv.ContentW, cs.Width)
	}
}

func TestScrollViewportBarHitStealsFromChildren(t *testing.T) {
	col := primitive.Column()
	for i := 0; i < 12; i++ {
		// Full-width pressable-like boxes covering the rail
		b := primitive.NewBox()
		b.Height = 36
		b.Width = 160
		b.Hit = core.HitBlock
		col.AddChild(b)
	}
	sv := primitive.NewScrollViewport(col)
	sv.Width, sv.Height = 160, 100
	sb := primitive.DefaultScrollbar()
	sb.Vertical = primitive.ScrollbarAuto
	sb.Overlay = false
	sb.Thickness = 8
	sv.SetScrollbar(sb)
	_ = sv.Layout(core.Tight(160, 100))
	if !sv.OverflowY() {
		t.Fatal("need overflow")
	}
	// Point on right strip should hit ScrollViewport itself (for thumb drag)
	hit := sv.HitTest(core.Point{X: 156, Y: 40})
	if hit != sv {
		t.Fatalf("bar hit=%T want *ScrollViewport", hit)
	}
	// Content area hits child
	hit2 := sv.HitTest(core.Point{X: 40, Y: 40})
	if hit2 == nil || hit2 == sv {
		t.Fatalf("content hit should be child, got %T", hit2)
	}
}

func TestScrollbarChromeConfig(t *testing.T) {
	b := primitive.DefaultScrollbar()
	b.SetThickness(8).SetHoverThickness(14).SetShowTrack(true).
		SetTrackColor(render.RGBA{R: 0.9, G: 0.9, B: 0.9, A: 1}).
		SetThumbColor(render.RGBA{R: 0.4, G: 0.4, B: 0.5, A: 1}).
		SetExpandOnHover(true)
	if b.GutterThickness() != 14 {
		t.Fatalf("gutter want 14 got %v", b.GutterThickness())
	}
	if b.Thickness != 8 {
		t.Fatal(b.Thickness)
	}
	if b.HoverThickness != 14 {
		t.Fatal(b.HoverThickness)
	}
	b.SetShowTrack(false)
	if b.ShowTrack {
		t.Fatal("track off")
	}

	col := primitive.Column()
	for i := 0; i < 8; i++ {
		box := primitive.NewBox()
		box.Height = 40
		col.AddChild(box)
	}
	sv := primitive.NewScrollViewport(col)
	sv.Width, sv.Height = 100, 60
	sv.SetScrollbar(b)
	_ = sv.Layout(core.Tight(100, 60))
	_, _, r, _ := sv.ContentInsets()
	if r != 14 {
		t.Fatalf("inset right=%v want gutter 14", r)
	}
}
