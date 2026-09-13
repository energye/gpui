package main

import "testing"

func TestCatalog_OverviewStandalone(t *testing.T) {
	pages := Pages()
	if len(pages) < 1 {
		t.Fatal("no pages")
	}
	ov, ok := Lookup("overview")
	if !ok || len(ov.Sections) == 0 {
		t.Fatalf("overview missing %+v", ov)
	}
	if _, ok := Lookup("no-such-tab"); ok {
		t.Fatal("unknown tab should miss")
	}
}

func TestCatalog_RegisterAndSelect(t *testing.T) {
	Register(Page{Name: "button", Title: "Button", Doc: "docs/antd/button.md", Sections: []Section{{Title: "Type"}, {Title: "Size"}}})
	defer delete(registry, "button")
	if _, ok := Lookup("button"); !ok {
		t.Fatal("button should register")
	}
	s := NewScene(1200, 800, "button")
	if s.SelectedPage().Name != "button" {
		t.Fatalf("selected=%s", s.SelectedPage().Name)
	}
	if !s.Select("overview") || s.Selected != "overview" {
		t.Fatal("select overview")
	}
	if s.Select("no-such") {
		t.Fatal("unknown select should fail")
	}
}

func TestScene_NavHit(t *testing.T) {
	s := NewScene(1200, 800, "overview")
	s.Layout()
	if len(s.navBoxes) == 0 {
		t.Fatal("no nav")
	}
	o := s.navBoxes[0].Offset()
	sz := s.navBoxes[0].Size()
	name, ok := s.HitNav(o.X+sz.Width/2, o.Y+sz.Height/2)
	if !ok || name == "" {
		t.Fatal("nav hit miss")
	}
	if _, ok := s.HitNav(1190, 790); ok {
		t.Fatal("far corner should miss nav")
	}
}
