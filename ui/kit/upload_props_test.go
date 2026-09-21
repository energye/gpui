package kit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func loadUploadCases(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "upload_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	return m
}

func TestUpload_DefaultsMatchCases(t *testing.T) {
	cases := loadUploadCases(t)
	def := cases["default_props"].(map[string]any)
	p := kit.DefaultUploadProps()
	if string(p.Type) != def["type"].(string) {
		t.Fatalf("type = %q want select", p.Type)
	}
	if string(p.ListType) != def["list_type"].(string) {
		t.Fatalf("listType = %q want text", p.ListType)
	}
	if p.TriggerLabel != def["trigger_label"].(string) {
		t.Fatalf("trigger = %q", p.TriggerLabel)
	}
	if p.DragText != def["drag_text"].(string) {
		t.Fatalf("drag text = %q", p.DragText)
	}
	if p.DragHint != def["drag_hint"].(string) {
		t.Fatalf("drag hint = %q", p.DragHint)
	}
	if p.ShowUploadList != def["show_upload_list"].(bool) {
		t.Fatal("showUploadList must default true")
	}
	if p.Disabled != def["disabled"].(bool) {
		t.Fatal("disabled must default false")
	}
	dp := kit.DefaultUploadDraggerProps()
	if dp.Type != kit.UploadTypeDrag {
		t.Fatal("dragger sugar must set type drag")
	}
}
