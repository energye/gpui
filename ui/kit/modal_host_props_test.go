package kit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func loadHostCases(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "modal_host_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	return m
}

func TestModalHost_DefaultsMatchCases(t *testing.T) {
	cases := loadHostCases(t)
	def := cases["default_props"].(map[string]any)
	p := kit.DefaultModalHostProps()
	if float64(p.ZIndexBase) != def["z_base"].(float64) {
		t.Fatalf("zbase = %d want 1000", p.ZIndexBase)
	}
	if p.ScrollLock != true {
		t.Fatal("scrolllock must default true")
	}
	m := kit.DefaultModalHostMask()
	dm := cases["default_mask"].(map[string]any)
	if m.Enabled != dm["enabled"].(bool) || m.Closable != dm["closable"].(bool) {
		t.Fatalf("default mask = %+v", m)
	}
	c := kit.DefaultModalHostConfirmConfig()
	w := cases["widths"].(map[string]any)
	if c.Width != w["confirm"].(float64) {
		t.Fatalf("confirm width = %v want 416", c.Width)
	}
}
