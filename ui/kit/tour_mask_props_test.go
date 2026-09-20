package kit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func loadTourCases(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "tour_mask_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	return m
}

func TestTourMask_DefaultsMatchCases(t *testing.T) {
	cases := loadTourCases(t)
	def := cases["default_props"].(map[string]any)
	p := kit.DefaultTourMaskProps()
	if p.Open != false {
		t.Fatal("open must default false")
	}
	if p.Placement != kit.TourMaskPlacement(def["placement"].(string)) {
		t.Fatalf("placement = %q want bottom", p.Placement)
	}
	if string(p.Type) != def["type"].(string) {
		t.Fatalf("type = %q want default", p.Type)
	}
	if p.Mask != true {
		t.Fatal("mask must default true")
	}
	if p.Gap.OffsetX != def["gap_offset"].(float64) || p.Gap.Radius != def["gap_radius"].(float64) {
		t.Fatalf("gap = %+v want offset 6 radius 2", p.Gap)
	}
	if p.ZIndex != int(def["z"].(float64)) {
		t.Fatalf("z = %d want 1001", p.ZIndex)
	}
	if p.Keyboard != true || p.Arrow != true || p.CloseIcon != true {
		t.Fatal("keyboard/arrow/closeIcon must default true")
	}
	prev, next := kit.ResolveTourMaskStepButtons("", "", false, "zh-CN")
	btn := cases["buttons"].(map[string]any)
	if prev != btn["prev_zh"].(string) || next != btn["next_zh"].(string) {
		t.Fatalf("zh buttons = %q,%q", prev, next)
	}
	_, fin := kit.ResolveTourMaskStepButtons("", "", true, "en-US")
	if fin != btn["finish_en"].(string) {
		t.Fatalf("finish en = %q", fin)
	}
}
