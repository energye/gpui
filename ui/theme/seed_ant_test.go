package theme_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/theme"
)

func loadSeed(t *testing.T) map[string]any {
	t.Helper()
	p := filepath.Join("testdata", "seed_default.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse seed json: %v", err)
	}
	return m
}

func num(m map[string]any, k string) float64 {
	v, ok := m[k].(float64)
	if !ok {
		return math.NaN()
	}
	return v
}

// TestSeed_DefaultMatchesAnt checks the global seed against testdata/seed_default.json
// (antd v6.5.1). L2 assertions read these numbers; component tokens land per-kit.
func TestSeed_DefaultMatchesAnt(t *testing.T) {
	want := loadSeed(t)
	got := theme.DefaultTokens()

	feq := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

	checkNum := func(name string, g, w float64) {
		t.Helper()
		if !feq(g, w) {
			t.Fatalf("%s = %v want %v", name, g, w)
		}
	}

	checkNum("fontSize", got.FontSize, num(want, "fontSize"))
	checkNum("fontSizeSM", got.FontSizeSM, num(want, "fontSizeSM"))
	checkNum("fontSizeLG", got.FontSizeLG, num(want, "fontSizeLG"))
	checkNum("fontSizeXL", got.FontSizeXL, num(want, "fontSizeXL"))
	checkNum("lineHeight", got.LineHeight, num(want, "lineHeight"))
	checkNum("fontHeight", got.FontHeight, num(want, "fontHeight"))
	checkNum("size", got.Size, num(want, "size"))
	checkNum("sizeXS", got.SizeXS, num(want, "sizeXS"))
	checkNum("sizeXXL", got.SizeXXL, num(want, "sizeXXL"))
	checkNum("controlHeight", got.ControlHeight, num(want, "controlHeight"))
	checkNum("controlHeightSM", got.ControlHeightSM, num(want, "controlHeightSM"))
	checkNum("controlHeightLG", got.ControlHeightLG, num(want, "controlHeightLG"))
	checkNum("radius", got.Radius, num(want, "borderRadius"))
	checkNum("radiusLG", got.RadiusLG, num(want, "borderRadiusLG"))
	checkNum("radiusSM", got.RadiusSM, num(want, "borderRadiusSM"))
	checkNum("padding", got.Padding, num(want, "padding"))
	checkNum("margin", got.Margin, num(want, "margin"))
	checkNum("marginXXL", got.MarginXXL, num(want, "marginXXL"))
	checkNum("lineWidthFocus", got.LineWidthFocus, num(want, "lineWidthFocus"))
	checkNum("zIndexPopupBase", got.ZIndexPopupBase, num(want, "zIndexPopupBase"))
	checkNum("sizePopupArrow", got.SizePopupArrow, num(want, "sizePopupArrow"))

	// Spot-check colors via hex round-trip (alpha kept).
	if got.ColorPrimary != theme.Hex("#1677ff") {
		t.Fatalf("primary = %+v", got.ColorPrimary)
	}
	if got.ColorBgMask.A != 0.45 {
		t.Fatalf("mask alpha = %v", got.ColorBgMask.A)
	}
	if got.ColorText.A != 0.88 || got.ColorTextSecondary.A != 0.65 {
		t.Fatalf("text alpha = %v / %v", got.ColorText.A, got.ColorTextSecondary.A)
	}
	if got.Surface != theme.Hex("#ffffff") {
		t.Fatalf("surface should be Ant container white, got %+v", got.Surface)
	}
	if got.MotionDurationFast != "0.1s" || got.BoxShadow == "" {
		t.Fatalf("motion/shadow missing")
	}
}
