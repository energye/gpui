package main

import "testing"

func TestCatalog_IconPage(t *testing.T) {
	p, ok := Lookup("icon")
	if !ok {
		t.Fatal("icon page missing (standalone -tab=icon breaks)")
	}
	if len(p.Sections) != 5 {
		t.Fatalf("icon sections=%d want 5 (basic/two-tone/custom/iconfont/multi-source)", len(p.Sections))
	}
}
