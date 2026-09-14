package divider_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/divider"
	"github.com/energye/gpui/ui/rendering"
)

// P1 per docs/antd/divider.md §6.8: staged items keep a single case each.
// Runnable parts assert here; browser-only depth Skips with reasons.

// DIV-26 semantic classNames/styles depth: P0 mounts structure (DIV-21),
// functional Record/class strings stay P1.
func TestDivider_PRD_DIV26_SemanticDepth(t *testing.T) {
	d := divider.NewDividerWithTitle("Text")
	hook := rendering.NewRenderColorBox(24, 8, 0.2, 0.4, 0.8, 1)
	d.SetTitleNode(hook)
	sz := d.Layout(rendering.Loose(260, 80))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("semantic mount layout=%v", sz)
	}
	t.Skip("P1 staged: functional classNames/styles Record strings map browser CSS and have no desktop equivalent; structure mount stays P0 (DIV-21)")
}

// P1 orientationMargin absolute px + styles.content.margin matrix:
// P0 covers default ratio 0.05 + custom ratio; absolute strings stay P1.
func TestDivider_P1_OrientationMarginAbsolute(t *testing.T) {
	d := divider.NewDividerWithTitle("Text")
	d.SetTitlePlacement(divider.Start)
	d.SetOrientationMargin(0.2)
	if d.OrientationMarginRatio() != 0.2 {
		t.Fatalf("ratio=%v", d.OrientationMarginRatio())
	}
	t.Skip("P1 staged: absolute px / \"20\" / \"2em\" / \"10%\" string parsing plus styles.content.margin matrix is browser CSS; Go API §6.10 only exposes ratio float64")
}

// P1 vertical optical top -0.06em: P0 approximates 0 per §6.7.
func TestDivider_P1_VerticalOptical(t *testing.T) {
	d := divider.NewDivider()
	d.SetVertical(true)
	sz := d.Layout(rendering.Loose(100, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("vertical layout=%v", sz)
	}
	t.Skip("P1 staged: vertical -0.06em optical offset (~0.84px at 14pt) sits below AA threshold; P0 keeps 0 per §6.7")
}

// P1 dashed/dotted browser pixel纹样: P0 keeps visible distinction.
func TestDivider_P1_DashPixelPattern(t *testing.T) {
	a := divider.NewDivider()
	a.SetVariant(divider.Dashed)
	b := divider.NewDivider()
	b.SetVariant(divider.Dotted)
	if a.EffectiveVariant() == b.EffectiveVariant() {
		t.Fatal("dashed/dotted must differ")
	}
	t.Skip("P1 staged: byte-identical browser dash/dot pattern hash is explicitly not required (§6.7); P0 only needs visible distinction (showcase asserts dash gaps vs solid)")
}

// P1 ConfigProvider global Divider defaults: per-instance theme stays P0.
func TestDivider_P1_ConfigProvider(t *testing.T) {
	d := divider.NewDivider()
	if d.LineWidth() <= 0 || d.LineColor().A <= 0 {
		t.Fatal("per-instance theme probe")
	}
	t.Skip("P1 staged: ConfigProvider global Divider defaults need cross-package provider wiring; per-instance SetTheme/SetProvider stays P0")
}

// P1 debug example +官网 hash: not counted per §6.8.
func TestDivider_P1_DebugHash(t *testing.T) {
	t.Skip("P1 not counted: component-token debug preview plus browser pixel-hash identity are explicitly out of scope (§6.1 L4, §6.8)")
}
