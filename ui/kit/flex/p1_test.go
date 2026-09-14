package flex_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/kit/flex"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// P1 ConfigProvider global default staging: explicit props win over the
// provider, provider wins over the compiled default. Theme.Provider is the
// P1 staging point (dedicated ConfigProvider flex section is later).
func TestFlex_PRD_FLX21_GlobalDefaults(t *testing.T) {
	fx := loadFlex(t)
	p := theme.NewProvider(theme.DefaultTokens())
	f := flex.NewFlex(box(50, 20), box(50, 20))
	f.SetProvider(p)
	f.SetGapSize(flex.FlexGapMedium)
	if math.Abs(f.ResolvedGap()-p.Current().Padding) > fx.Tolerance {
		t.Fatalf("provider medium=%v want %v", f.ResolvedGap(), p.Current().Padding)
	}
	f.SetGap(8)
	if math.Abs(f.ResolvedGap()-8) > fx.Tolerance {
		t.Fatalf("explicit gap=%v must win over provider", f.ResolvedGap())
	}
	f.SetGapSize(flex.FlexGapMedium)
	f.SetProvider(nil)
	if math.Abs(f.ResolvedGap()-fx.GapMedium) > fx.Tolerance {
		t.Fatalf("cleared provider gap=%v want default %v", f.ResolvedGap(), fx.GapMedium)
	}
	if sz := f.Layout(rendering.Loose(400, 100)); sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v want non-zero", sz)
	}
}

// P1 flex CSS shorthand (container acting as a flex item: flex-grow /
// shrink / basis) needs a parent item protocol; staged via
// primitive.Flexible, no API in this package yet.
func TestFlex_PRD_FLX21_FlexShorthandNA(t *testing.T) {
	t.Skip("P1 flex shorthand (flex: grow/shrink/basis as item) is staged; item side lands on primitive.Flexible, no gpui API yet")
}

// P1 component (custom element type) is browser-only React API with no
// desktop mapping; the container node type stays flexNode.
func TestFlex_PRD_FLX21_ComponentNA(t *testing.T) {
	t.Skip("P1 component is browser-only (React.ComponentType div override); no gpui desktop mapping, staged")
}

// P1 non-bool wrap (wrap-reverse / nowrap strings) is staged; only bool
// single/multi-line is implemented.
func TestFlex_PRD_FLX21_WrapReverseNA(t *testing.T) {
	t.Skip("P1 wrap-reverse and flex-wrap string set are staged; SetWrap(bool) covers nowrap/wrap only")
}

// P1 semantic classNames/styles depth is staged; AriaLabel naming is the
// only P0 hook and it stays paint-only.
func TestFlex_PRD_FLX21_SemanticNA(t *testing.T) {
	f := flex.NewFlex(box(20, 20))
	before := f.Layout(rendering.Loose(200, 100))
	f.SetAriaLabel("flex-semantic")
	after := f.Layout(rendering.Loose(200, 100))
	if before != after {
		t.Fatalf("aria naming must not move layout: %v vs %v", before, after)
	}
	t.Skip("P1 semantic classNames/styles depth is staged; only AriaLabel naming ships (layout-stable, checked above)")
}

// P1 pixel-level motion and complex virtual lists are out of scope for a
// pure layout box; motion follows the host tick, no flex-owned animation.
func TestFlex_PRD_FLX21_MotionVirtualNA(t *testing.T) {
	t.Skip("P1 motion pixels / virtualized flex children are staged; flex owns no animation loop, host tick applies")
}

// P1 debug demos and ant.design pixel-hash parity are explicitly out of
// scope (§6.1 L4 / §6.8 P1); the repo golden is the L3 source.
func TestFlex_PRD_FLX21_DebugPixelHashNA(t *testing.T) {
	t.Skip("P1 debug demo and ant.design pixel-hash parity are not built; repo golden is the L3 source")
}

// L4 human-eye side-by-side sign-off needs a reviewer looking at
// ant.design; documented Skip (not a code gap).
func TestFlex_PRD_FLX20_HumanEyeNA(t *testing.T) {
	t.Skip("L4 FLX-20 needs human side-by-side sign-off against ant.design; no automated assertion, staged")
}
