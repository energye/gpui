// Gallery catalog: one tab per control.
//
// A Page is one left-nav entry; Sections follow the official antd examples
// for that control (§6.8 P0 first, then P1). Pages register here; main.go
// renders them. A page must open standalone (-tab=<name>), test standalone
// and screenshot standalone; the combined window is preview only.
package main

import "sort"

// Section is one capability block on a page (one official example group).
type Section struct {
	Title string
	Note  string
}

// Page is one gallery tab.
type Page struct {
	// Name is the kit directory name, e.g. button.
	Name string
	// Title is the nav label.
	Title string
	// Doc is the spec path, e.g. docs/antd/button.md.
	Doc string
	// Sections follow official examples (§6.8 P0 first).
	Sections []Section
}

var registry = map[string]Page{}

// Register adds or replaces a page. Pages sort by Name for stable nav.
func Register(p Page) {
	if p.Name == "" {
		return
	}
	registry[p.Name] = p
}

// Lookup returns the page for standalone open (-tab=<name>).
func Lookup(name string) (Page, bool) {
	p, ok := registry[name]
	return p, ok
}

// Pages returns all registered pages sorted by Name.
func Pages() []Page {
	out := make([]Page, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ClearRegistry removes all pages (tests only).
func ClearRegistry() {
	registry = map[string]Page{}
}

func init() {
	// W0 overview tab (not a control): proves the shell, theme seed and
	// overlay placement wiring before any control lands. Control tabs
	// register from their own files as W1+ lands.
	Register(Page{
		Name:  "overview",
		Title: "Overview 地基",
		Doc:   "docs/antd/README.md",
		Sections: []Section{
			{Title: "Foundation", Note: "W0 五项：架子/总窗/覆盖/种子/浮层已就绪"},
			{Title: "Theme", Note: "Ant v6.5.1 全局种子色板与度量（见 ui/theme）"},
			{Title: "Placement", Note: "十二方向/翻转/箭头示意（见 ui/overlay）"},
		},
	})
}
