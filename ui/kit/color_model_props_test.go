package kit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func loadColorCases(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "color_model_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	return m
}

func TestColorModel_DefaultsMatchCases(t *testing.T) {
	cases := loadColorCases(t)
	def := cases["default_props"].(map[string]any)
	p := kit.DefaultColorModelProps()
	if string(p.Format) != def["format"].(string) {
		t.Fatalf("format = %q want hex", p.Format)
	}
	if string(p.Mode) != def["mode"].(string) {
		t.Fatalf("mode = %q want single", p.Mode)
	}
	if string(p.Size) != def["size"].(string) {
		t.Fatalf("size = %q want medium", p.Size)
	}
	if p.Placement != def["placement"].(string) {
		t.Fatalf("placement = %q want bottomLeft", p.Placement)
	}
	if string(p.Trigger) != def["trigger"].(string) {
		t.Fatalf("trigger = %q want click", p.Trigger)
	}
	if p.AllowClear != def["allow_clear"].(bool) {
		t.Fatal("allowClear must default false")
	}
	if p.Disabled != def["disabled"].(bool) {
		t.Fatal("disabled must default false")
	}
	if p.DisabledAlpha != def["disabled_alpha"].(bool) {
		t.Fatal("disabledAlpha must default false")
	}
}
