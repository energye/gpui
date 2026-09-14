package layout_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/layout"
	"github.com/energye/gpui/ui/rendering"
)

// P1 fixed header / fixed sider (fixed.tsx / fixed-sider.tsx): the official
// demos rely on sticky positioning plus a host scroll-lock container, which
// the kit has no scroll-lock host for yet (§6.7 P1, §6.8 P1). Skipped with
// reason, not silently dropped.
func TestLayout_PRD_LAY23_FixedChromeNA(t *testing.T) {
	h := layout.NewHeader()
	c := layout.NewContent()
	l := layout.NewLayout(h, c)
	if sz := l.Layout(rendering.Tight(1200, 800)); sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v want non-zero", sz)
	}
	t.Skip("P1 fixed header/sider needs sticky + host scroll-lock container (fixed.tsx/fixed-sider.tsx); kit has no scroll-lock host yet, staged per §6.8 P1")
}

// P1 semantic classNames/styles depth: only AriaLabel naming ships (§6.8
// P1). Naming must stay layout-stable; the record depth is staged.
func TestLayout_PRD_LAY23_SemanticDepthNA(t *testing.T) {
	s := layout.NewSider()
	before := s.Layout(rendering.Loose(1000, 800))
	s.SetAriaLabel("layout-sider-semantic")
	after := s.Layout(rendering.Loose(1000, 800))
	if before != after {
		t.Fatalf("aria naming must not move layout: %v vs %v", before, after)
	}
	t.Skip("P1 semantic classNames/styles record depth is staged; only AriaLabel naming ships (layout-stable, checked above)")
}

// P1 ConfigProvider global Layout defaults: explicit props already win over
// compiled defaults; a dedicated provider section is staged (§6.8 P1).
func TestLayout_PRD_LAY23_GlobalDefaultsNA(t *testing.T) {
	s := layout.NewSider()
	s.SetWidth(240)
	if s.EffectiveWidth() != 240 {
		t.Fatalf("explicit width=%v must win", s.EffectiveWidth())
	}
	s.SetWidth(0)
	if s.EffectiveWidth() != 200 {
		t.Fatalf("cleared width=%v want default 200", s.EffectiveWidth())
	}
	t.Skip("P1 ConfigProvider global Layout defaults are staged; explicit > default precedence checked above")
}

// L4 human-eye side-by-side with ant.design needs a reviewer; documented
// Skip per §6.9 LAY-22 (not a code gap).
func TestLayout_PRD_LAY22_HumanEyeNA(t *testing.T) {
	t.Skip("L4 LAY-22 needs human side-by-side sign-off against ant.design; no automated assertion, staged")
}

// Debug demos and ant.design pixel-hash parity are explicitly out of scope
// (§6.1 L4 / §6.8 P1); the repo golden is the L3 source.
func TestLayout_PRD_LAY23_DebugPixelHashNA(t *testing.T) {
	t.Skip("P1 debug demo and ant.design pixel-hash parity are not built; repo golden is the L3 source")
}
