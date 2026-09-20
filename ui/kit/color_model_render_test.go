package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestColorModel_PanelGeometryMatchesCases(t *testing.T) {
	cases := loadColorCases(t)
	panel := cases["panel"].(map[string]any)

	full := kit.ComputeColorModelPanel(false)
	if full.SV.W != panel["inner_w"].(float64) || full.SV.H != panel["sv_h"].(float64) {
		t.Fatalf("sv = %.1f,%.1f want 210,210", full.SV.W, full.SV.H)
	}
	if full.Hue.H != panel["strip_h"].(float64) {
		t.Fatalf("hue h = %v want 8", full.Hue.H)
	}
	if !full.HasAlpha || full.Alpha.H != panel["alpha_h"].(float64) {
		t.Fatal("alpha strip must exist at h8")
	}
	if full.Height != panel["height_withalpha"].(float64) {
		t.Fatalf("height = %v want 266", full.Height)
	}
	bare := kit.ComputeColorModelPanel(true)
	if bare.HasAlpha {
		t.Fatal("disabledAlpha must drop alpha strip")
	}
	if bare.Height != panel["height_noalpha"].(float64) {
		t.Fatalf("bare height = %v want 250", bare.Height)
	}
	if kit.ColorModelPanelW != panel["w"].(float64) {
		t.Fatalf("panel w = %v want 234", kit.ColorModelPanelW)
	}
	// Handle math: hue 0 left edge, 360 right edge, 180 center.
	hx0, _ := kit.ColorModelHandleXY(0, full)
	hx1, _ := kit.ColorModelHandleXY(360, full)
	hxm, _ := kit.ColorModelHandleXY(180, full)
	if hx0 != full.Hue.X || hx1 != full.Hue.X+full.Hue.W {
		t.Fatalf("hue ends = %v,%v", hx0, hx1)
	}
	if hxm != full.Hue.X+full.Hue.W/2 {
		t.Fatalf("hue mid = %v want center", hxm)
	}
	sx, sy := kit.ColorModelSVXY(0, 1, full)
	if sx != full.SV.X || sy != full.SV.Y {
		t.Fatalf("sv origin = %v,%v", sx, sy)
	}
}
