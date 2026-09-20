package prim_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/internal/prim"
	"github.com/energye/gpui/ui/rendering"
)

// TestPrim_LayoutEdge covers overflow, unbounded wrap, RTL mirroring,
// IndexedStack max sizing, CustomMeasure fallback and virtual binding.
func TestPrim_LayoutEdge(t *testing.T) {
	t.Run("overflow_keeps_outer_allows_spill", func(t *testing.T) {
		items := []prim.FlexSpec{prim.FixedSpec(200, 40), prim.FixedSpec(200, 40)}
		got := prim.FlexLayout(true, 300, 100, 8, prim.DirLTR, prim.CrossStart, items)
		if got.Outer.Width != 300 {
			t.Fatalf("bounded row must fill max, got %.2f", got.Outer.Width)
		}
		if got.Boxes[1].X != 208 {
			t.Fatalf("second box x = %.2f want 208 (spill allowed)", got.Boxes[1].X)
		}
	})

	t.Run("unbounded_wraps_content", func(t *testing.T) {
		items := []prim.FlexSpec{prim.FixedSpec(120, 30), prim.FixedSpec(90, 50)}
		got := prim.FlexLayout(true, 1e9, 200, 10, prim.DirLTR, prim.CrossStart, items)
		if got.Outer.Width != 220 {
			t.Fatalf("unbounded row must wrap, got %.2f", got.Outer.Width)
		}
	})

	t.Run("rtl_mirrors_row", func(t *testing.T) {
		items := []prim.FlexSpec{prim.FixedSpec(100, 40), prim.FixedSpec(80, 40)}
		ltr := prim.FlexLayout(true, 300, 100, 8, prim.DirLTR, prim.CrossStart, items)
		rtl := prim.FlexLayout(true, 300, 100, 8, prim.DirRTL, prim.CrossStart, items)
		if ltr.Boxes[0].X == rtl.Boxes[0].X {
			t.Fatal("RTL must mirror box order")
		}
		if rtl.Boxes[0].X != 88 {
			t.Fatalf("rtl first box x = %.2f want 88", rtl.Boxes[0].X)
		}
	})

	t.Run("limited_caps_unbounded", func(t *testing.T) {
		parent := rendering.Constraints{MaxWidth: rendering.Unbounded, MaxHeight: rendering.Unbounded}
		lim := prim.Limits{MinW: -1, MaxW: 120, MinH: -1, MaxH: -1}
		got := prim.ConstrainSize(lim, parent, rendering.Size{Width: 400, Height: 30})
		if got.Width != 120 {
			t.Fatalf("limited width = %.2f want 120", got.Width)
		}
	})

	t.Run("offstage_keeps_size_no_hit", func(t *testing.T) {
		o := prim.Offstage{ChildSize: rendering.Size{Width: 90, Height: 40}, Hidden: true}
		if o.Size() != (rendering.Size{Width: 90, Height: 40}) {
			t.Fatalf("offstage must keep layout size, got %+v", o.Size())
		}
		if o.PaintEnabled() || o.HitEnabled() {
			t.Fatal("hidden offstage must skip paint and hit")
		}
		shown := prim.Offstage{ChildSize: rendering.Size{Width: 90, Height: 40}}
		if !shown.PaintEnabled() || !shown.HitEnabled() {
			t.Fatal("shown offstage must paint and hit")
		}
	})

	t.Run("indexed_sizes_to_largest", func(t *testing.T) {
		parent := rendering.Constraints{MaxWidth: 400, MaxHeight: 300}
		got := prim.IndexedOuter(parent, []rendering.Size{{Width: 100, Height: 40}, {Width: 160, Height: 90}})
		if got.Width != 160 || got.Height != 90 {
			t.Fatalf("indexed outer = %+v want 160x90", got)
		}
	})

	t.Run("custom_fallback_measures_max", func(t *testing.T) {
		a := rendering.NewRenderColorBox(60, 20, 1, 0, 0, 1)
		b := rendering.NewRenderColorBox(90, 10, 0, 1, 0, 1)
		parent := rendering.Constraints{MaxWidth: 400, MaxHeight: 300}
		got := prim.CustomMeasure(parent, []rendering.RenderObject{a, b}, nil)
		if got.Width != 90 || got.Height != 20 {
			t.Fatalf("custom fallback = %+v want 90x20", got)
		}
	})

	t.Run("builder_picks_by_width", func(t *testing.T) {
		var picked string
		fn := prim.BuilderFunc(func(c rendering.Constraints) rendering.RenderObject {
			if c.MaxWidth < 600 {
				picked = "narrow"
				return rendering.NewRenderColorBox(100, 40, 1, 0, 0, 1)
			}
			picked = "wide"
			return rendering.NewRenderColorBox(200, 40, 0, 0, 1, 1)
		})
		narrow := fn(rendering.Constraints{MaxWidth: 400, MaxHeight: 300})
		if picked != "narrow" || narrow.Size().Width != 0 {
			_ = narrow.Layout(rendering.Constraints{MaxWidth: 400, MaxHeight: 300})
		}
		if picked != "narrow" {
			t.Fatalf("narrow breakpoint picked %q", picked)
		}
		wide := fn(rendering.Constraints{MaxWidth: 800, MaxHeight: 300})
		_ = wide.Layout(rendering.Constraints{MaxWidth: 800, MaxHeight: 300})
		if picked != "wide" {
			t.Fatalf("wide breakpoint picked %q", picked)
		}
	})

	t.Run("virtual_only_mounts_visible", func(t *testing.T) {
		vb := prim.NewViewportBox(1000, 40, func(i int) rendering.RenderObject {
			return rendering.NewRenderColorBox(200, 40, 0.2, 0.3, 0.4, 1)
		})
		vb.Viewport.FixedWidth, vb.Viewport.FixedHeight = 200, 200
		vb.Viewport.Layout(rendering.Tight(200, 200))
		if vb.BindCount() <= 0 || vb.BindCount() >= 1000 {
			t.Fatalf("bind_count = %d want 0 < n << 1000", vb.BindCount())
		}
	})
}
