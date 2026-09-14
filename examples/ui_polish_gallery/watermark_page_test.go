package main

import "testing"

func TestCatalog_WatermarkPage(t *testing.T) {
	p, ok := Lookup("watermark")
	if !ok {
		t.Fatal("watermark page missing (standalone -tab=watermark breaks)")
	}
	if p.Doc != "docs/antd/watermark.md" {
		t.Fatalf("watermark Doc=%s want docs/antd/watermark.md", p.Doc)
	}
	if len(p.Sections) != 5 {
		t.Fatalf("watermark sections=%d want 5 (basic/multi-line/image/custom/modal-drawer)", len(p.Sections))
	}
	s := NewScene(1200, 800, "watermark")
	if s.SelectedPage().Name != "watermark" {
		t.Fatal("watermark page cannot open standalone")
	}
}
