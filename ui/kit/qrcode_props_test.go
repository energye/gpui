package kit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func loadQRCodeCases(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "qrcode_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	return m
}

func TestQRCode_DefaultsMatchCases(t *testing.T) {
	cases := loadQRCodeCases(t)
	def := cases["default_props"].(map[string]any)
	p := kit.DefaultQRCodeProps()
	if string(p.Type) != def["type"].(string) {
		t.Fatalf("type = %q want canvas", p.Type)
	}
	if kit.ResolveQRCodeSize(p) != def["size"].(float64) {
		t.Fatalf("size = %v want 160", kit.ResolveQRCodeSize(p))
	}
	iw, ih := kit.ResolveQRCodeIconSize(p)
	if iw != def["icon_size"].(float64) || ih != def["icon_size"].(float64) {
		t.Fatalf("icon = %v,%v want 40,40", iw, ih)
	}
	if string(p.ErrorLevel) != def["error_level"].(string) {
		t.Fatalf("errorLevel = %q want M", p.ErrorLevel)
	}
	if string(p.Status) != def["status"].(string) {
		t.Fatalf("status = %q want active", p.Status)
	}
	if p.Bordered != def["bordered"].(bool) {
		t.Fatal("bordered must default true")
	}
	if p.MarginSize != int(def["margin"].(float64)) {
		t.Fatal("marginSize must default 0")
	}
	exp, ref, sc := kit.ResolveQRCodeLocale("en-US")
	loc := cases["locale"].(map[string]any)
	if exp != loc["expired_en"].(string) || ref != loc["refresh_en"].(string) || sc != loc["scanned_en"].(string) {
		t.Fatalf("en locale = %q,%q,%q", exp, ref, sc)
	}
	if exp, _, _ := kit.ResolveQRCodeLocale("zh-CN"); exp != loc["expired_zh"].(string) {
		t.Fatalf("zh expired = %q", exp)
	}
}
