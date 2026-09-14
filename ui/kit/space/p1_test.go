package space_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/kit/space"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// P0 completeness: baseline alignment falls back to start for boxes without
// text baselines (no crash, tops level, geometry non-zero).
func TestSpace_AlignBaseline(t *testing.T) {
	fx := loadSpace(t)
	s := space.NewSpace(box(60, fx.ShortH), box(60, fx.TallH))
	s.SetAlign(space.SpaceAlignBaseline)
	if s.EffectiveAlign() != space.SpaceAlignBaseline {
		t.Fatalf("effective=%q want baseline", s.EffectiveAlign())
	}
	sz := s.Layout(rendering.Loose(400, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v want >0", sz)
	}
	if ns := s.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("node size=%v want >0", ns)
	}
	off := offsets(s)
	if math.Abs(off[0].Y) > fx.Tolerance || math.Abs(off[1].Y) > fx.Tolerance {
		t.Fatalf("baseline box tops %+v want 0 (start fallback)", off)
	}
}

// P0 completeness: RTL mirrors the row (and the vertical cross axis).
func TestSpace_RTLMirror(t *testing.T) {
	fx := loadSpace(t)
	a, b := box(50, 20), box(30, 20)
	s := space.NewSpace(a, b)
	s.Layout(rendering.Loose(400, 100))
	off := offsets(s)
	if math.Abs(spacingX(off, sizes(s), 0)-fx.GapSmall) > fx.Tolerance {
		t.Fatalf("ltr spacing=%v want %v", spacingX(off, sizes(s), 0), fx.GapSmall)
	}
	s.SetRTL(true)
	if !s.RTL() {
		t.Fatal("RTL flag must stick")
	}
	s.Layout(rendering.Loose(400, 100))
	roff := offsets(s)
	// Content 50+8+30=88: mirrored first starts at 38, second at 0.
	if math.Abs(roff[0].X-38) > fx.Tolerance || math.Abs(roff[1].X) > fx.Tolerance {
		t.Fatalf("rtl offsets %+v want [38 0]", roff)
	}
	if ns := s.Node().Size(); ns.Width <= 0 {
		t.Fatalf("rtl node size=%v want >0", ns)
	}
	// Vertical RTL mirrors the cross axis only.
	v := space.NewSpace(box(50, 20), box(30, 20))
	v.SetVertical(true)
	v.SetRTL(true)
	v.Layout(rendering.Loose(200, 400))
	voff := offsets(v)
	if math.Abs(voff[0].X) > fx.Tolerance || math.Abs(voff[1].X-20) > fx.Tolerance {
		t.Fatalf("vertical rtl cross %+v want [0 20]", voff)
	}
}

// P0 completeness: Space block fills the parent width (inline vs block flex).
func TestSpace_SpaceBlockExpand(t *testing.T) {
	fx := loadSpace(t)
	s := space.NewSpace(box(50, 20), box(50, 20))
	loose := s.Layout(rendering.Loose(300, 100))
	wantW := 2*50 + fx.GapSmall
	if math.Abs(loose.Width-wantW) > fx.Tolerance {
		t.Fatalf("inline width=%v want %v", loose.Width, wantW)
	}
	s.SetExpandMax(true)
	if !s.ExpandMax() {
		t.Fatal("ExpandMax flag must stick")
	}
	full := s.Layout(rendering.Loose(300, 100))
	if math.Abs(full.Width-300) > fx.Tolerance {
		t.Fatalf("block width=%v want 300", full.Width)
	}
	if ns := s.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("block node size=%v want >0", ns)
	}
	s.SetExpandMax(false)
	back := s.Layout(rendering.Loose(300, 100))
	if math.Abs(back.Width-wantW) > fx.Tolerance {
		t.Fatalf("inline again=%v want %v", back.Width, wantW)
	}
}

// P0 completeness: Compact orientation wins over the vertical sugar.
func TestSpace_CompactOrientationPriority(t *testing.T) {
	c := space.NewSpaceCompact(box(60, 32), box(60, 32))
	c.SetOrientation(space.SpaceVertical)
	c.SetVertical(false)
	if !c.IsVertical() {
		t.Fatal("compact orientation must win over vertical=false")
	}
	c.SetOrientation(space.SpaceHorizontal)
	c.SetVertical(true)
	if c.IsVertical() {
		t.Fatal("compact explicit horizontal must win over vertical=true")
	}
	sz := c.Layout(rendering.Loose(400, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("compact layout=%v want >0", sz)
	}
}

// P0 completeness: Addon heights per size档, standalone radius kept.
func TestSpace_AddonSizes(t *testing.T) {
	fx := loadSpace(t)
	a := space.NewSpaceAddon(box(40, 20))
	if math.Abs(a.ControlHeight()-32) > fx.Tolerance {
		t.Fatalf("middle height=%v want 32", a.ControlHeight())
	}
	a.SetSize(space.SpaceSizeSmall)
	if math.Abs(a.ControlHeight()-24) > fx.Tolerance {
		t.Fatalf("small height=%v want 24", a.ControlHeight())
	}
	a.SetSize(space.SpaceSizeLarge)
	if math.Abs(a.ControlHeight()-40) > fx.Tolerance {
		t.Fatalf("large height=%v want 40", a.ControlHeight())
	}
	// Standalone (never in Compact) keeps the theme radius.
	if r := a.EffectiveRadius(); r <= 0 {
		t.Fatalf("standalone radius=%v want kept", r)
	}
	// Single-child helpers.
	b := space.NewSpaceAddon()
	b.SetChild(box(20, 10))
	if b.ChildCount() != 1 {
		t.Fatalf("SetChild count=%d want 1", b.ChildCount())
	}
	b.SetChildren(box(10, 10), box(10, 10))
	if b.ChildCount() != 2 {
		t.Fatalf("SetChildren count=%d want 2", b.ChildCount())
	}
	sz := b.Layout(rendering.Loose(200, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("addon layout=%v want >0", sz)
	}
	_ = theme.Default.Current()
}

// P0 completeness: separator factory runs once per gap (no node mounts twice).
func TestSpace_SeparatorFactoryOnce(t *testing.T) {
	calls := 0
	s := space.NewSpace(box(40, 20), box(40, 20), box(40, 20))
	s.SetSeparator(func() rendering.RenderObject {
		calls++
		return rendering.NewRenderColorBox(8, 16, 0.6, 0.6, 0.6, 1)
	})
	if calls != 2 {
		t.Fatalf("factory calls=%d want 2 (N-1)", calls)
	}
	if s.SeparatorCount() != 2 {
		t.Fatalf("separators=%d want 2", s.SeparatorCount())
	}
	sz := s.Layout(rendering.Loose(400, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v want >0", sz)
	}
}

// SPC-24 P1: vertical compact page (Compact ability already proven
// horizontally; vertical stacking shares the same overlap rule).
func TestSpace_PRD_SPC24_VerticalCompact(t *testing.T) {
	fx := loadSpace(t)
	c := space.NewSpaceCompact(box(60, 32), box(60, 32))
	c.SetVertical(true)
	if !c.IsVertical() {
		t.Fatal("vertical compact must flip axis")
	}
	c.Layout(rendering.Loose(200, 400))
	kids := c.Children()
	if len(kids) != 2 {
		t.Fatalf("children=%d want 2", len(kids))
	}
	shared := (kids[0].Offset().Y + 32) - kids[1].Offset().Y
	if math.Abs(shared-1) > fx.Tolerance {
		t.Fatalf("vertical shared=%v want 1", shared)
	}
	if ns := c.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("node size=%v want >0", ns)
	}
	// Block still fills the parent width on the vertical axis.
	c.SetBlock(true)
	sz := c.Layout(rendering.Tight(200, 200))
	if math.Abs(sz.Width-200) > fx.Tolerance {
		t.Fatalf("block width=%v want 200", sz.Width)
	}
}

// SPC-24 P1 semantic classNames/styles depth: structure mount stays P0,
// functional Record strings stay staged.
func TestSpace_PRD_SPC24_SemanticDepth(t *testing.T) {
	s := space.NewSpace(box(40, 20), box(40, 20))
	sz := s.Layout(rendering.Loose(300, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("mount layout=%v", sz)
	}
	t.Skip("P1 staged: functional classNames/styles Record strings map browser CSS and have no desktop equivalent; structure mount stays P0")
}

// SPC-24 P1 ConfigProvider global Space defaults: per-instance theme stays P0.
func TestSpace_PRD_SPC24_ConfigProvider(t *testing.T) {
	fx := loadSpace(t)
	s := space.NewSpace(box(20, 20))
	s.SetSize(space.SpaceSizeMiddle)
	if math.Abs(s.ResolvedGap()-fx.GapMiddle) > fx.Tolerance {
		t.Fatalf("middle=%v want %v", s.ResolvedGap(), fx.GapMiddle)
	}
	t.Skip("P1 staged: ConfigProvider global Space defaults need cross-package provider wiring; per-instance SetTheme/SetProvider stays P0")
}

// SPC-24 P1 animation / virtual list pixel depth: Space is instant layout.
func TestSpace_PRD_SPC24_AnimationVirtualNA(t *testing.T) {
	s := space.NewSpace(box(20, 20), box(20, 20))
	sz := s.Layout(rendering.Loose(200, 100))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
	t.Skip("P1 staged: animation-pixel motion and complex virtual-list windowing have no Space desktop mapping; P0 stays instant")
}

// SPC-24 P1 style-class.tsx + _semantic.tsx: semantic depth page.
func TestSpace_PRD_SPC24_CustomSemanticNA(t *testing.T) {
	t.Skip("P1 not counted now: style-class.tsx custom semantic page and _semantic.tsx need the staged classNames/styles hook above")
}

// SPC-24 P1 debug examples + ant.design pixel-hash parity are out of scope.
func TestSpace_PRD_SPC24_DebugHashNA(t *testing.T) {
	t.Skip("P1 not counted: compact-debug/compact-nested/debug/gap-in-line/component-token previews plus browser pixel-hash identity are explicitly out of scope (§6.1 L4, §6.8)")
}

// SPC-23 L4 human-eye side-by-side needs a reviewer: documented Skip.
func TestSpace_PRD_SPC23_HumanEyeNA(t *testing.T) {
	t.Skip("L4 SPC-23 needs human side-by-side sign-off against ant.design; no automated assertion, staged")
}
