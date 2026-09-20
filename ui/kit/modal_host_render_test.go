package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestModalHost_PanelGeometryMatchesCases(t *testing.T) {
	cases := loadHostCases(t)
	top := cases["panel_top"].(map[string]any)
	cfg := kit.DefaultModalHostConfirmConfig()
	r := kit.ComputeModalHostPanel(1200, 800, cfg, 1000)
	if r.W != top["want_w"].(float64) || r.X != top["want_x"].(float64) || r.Y != top["want_y"].(float64) {
		t.Fatalf("panel = %.1f,%.1f,%.1f want 392,100,416", r.X, r.Y, r.W)
	}
	cc := cases["centered_y"].(map[string]any)
	cfg.Centered = true
	rc := kit.ComputeModalHostPanel(1200, 800, cfg, 1010)
	if rc.Y != cc["want_y"].(float64) {
		t.Fatalf("centered y = %.1f want 290", rc.Y)
	}
	if kit.ZIndexForDepth(1000, 2) != 1020 {
		t.Fatal("depth 2 must be 1020")
	}
	ok, cancel := kit.ResolveModalHostConfirmText(kit.DefaultModalHostConfirmConfig(), "zh-CN")
	dt := cases["default_text"].(map[string]any)
	if ok != dt["ok_zh"].(string) || cancel != dt["cancel_zh"].(string) {
		t.Fatalf("zh copy = %q,%q", ok, cancel)
	}
}
