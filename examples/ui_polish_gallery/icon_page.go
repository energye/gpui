package main

func init() {
	// Icon P0 main path (docs/antd/icon.md §6.8): basic / two-tone /
	// custom / iconfont / multi-source. Standalone: -tab=icon.
	Register(Page{
		Name:  "icon",
		Title: "Icon",
		Doc:   "docs/antd/icon.md",
		Sections: []Section{
			{Title: "Basic", Note: "check close home setting smile sync loading star search plus minus edit delete arrows file-text circles"},
			{Title: "Two-tone", Note: "smile heart check-circle primary + secondary"},
			{Title: "Custom", Note: "SetPainter custom glyph"},
			{Title: "Iconfont", Note: "CreateFromIconfont offline register"},
			{Title: "Multi-source", Note: "later source overrides same name"},
		},
	})
}
