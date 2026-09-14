package main

import "testing"

func TestCatalog_SplitterPage(t *testing.T) {
	p, ok := Lookup("splitter")
	if !ok {
		t.Fatal("splitter page missing (standalone -tab=splitter breaks)")
	}
	if p.Doc != "docs/antd/splitter.md" {
		t.Fatalf("doc=%q want docs/antd/splitter.md", p.Doc)
	}
	// §6.8 P0 is 8 official examples: basic/controlled/vertical/
	// collapsible/collapsibleIcon/multiple/group/lazy.
	if len(p.Sections) != 8 {
		t.Fatalf("splitter sections=%d want 8 (basic/controlled/vertical/collapsible/icon/multiple/group/lazy)", len(p.Sections))
	}
	s := NewScene(1200, 800, "splitter")
	if s.SelectedPage().Name != "splitter" {
		t.Fatalf("selected=%s want splitter", s.SelectedPage().Name)
	}
}
