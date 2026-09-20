package kit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func loadDateCases(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "date_engine_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	return m
}

func TestDateEngine_DefaultsMatchCases(t *testing.T) {
	cases := loadDateCases(t)
	def := cases["default_props"].(map[string]any)
	p := kit.DefaultDateEngineProps()
	if string(p.Picker) != def["picker"].(string) {
		t.Fatalf("picker = %q want date", p.Picker)
	}
	if kit.ResolveDateEngineOrder(p) != def["order"].(bool) {
		t.Fatal("order must default true")
	}
	if got := kit.ResolveDateEngineWeekStart(p, "zh-CN"); int(got) != int(def["weekstart_zh"].(float64)) {
		t.Fatalf("zh weekStart = %d want 1", got)
	}
	if got := kit.ResolveDateEngineWeekStart(p, "en-US"); int(got) != int(def["weekstart_en"].(float64)) {
		t.Fatalf("en weekStart = %d want 0", got)
	}
	if got := kit.DisplayDateEngineFormat(p); got != def["display_format"].(string) {
		t.Fatalf("display format = %q", got)
	}
}
