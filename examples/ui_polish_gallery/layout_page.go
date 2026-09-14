package main

func init() {
	// Layout P0 main path (docs/antd/layout.md section 6.8): basic /
	// top / top-side / top-side-2 / side / custom-trigger /
	// collapsible-overlay / responsive. Standalone: -tab=layout.
	Register(Page{
		Name:  "layout",
		Title: "Layout",
		Doc:   "docs/antd/layout.md",
		Sections: []Section{
			{Title: "Basic", Note: "Header Content Footer column no sider"},
			{Title: "Top", Note: "upper middle lower footer pinned bottom"},
			{Title: "TopSide", Note: "header plus sider 200 plus content remainder"},
			{Title: "TopSide2", Note: "through header full width sider under header"},
			{Title: "Side", Note: "sider plus content trigger collapses to 80"},
			{Title: "CustomTrigger", Note: "custom node trigger same collapse"},
			{Title: "CollapsibleOverlay", Note: "collapsedWidth 0 overlay content unchanged"},
			{Title: "Responsive", Note: "breakpoint md viewport 700 auto collapse"},
		},
	})
}
