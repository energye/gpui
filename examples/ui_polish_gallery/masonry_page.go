package main

func init() {
	// Masonry P0 main path (docs/antd/masonry.md §6.8): basic / responsive /
	// image / dynamic. Standalone: -tab=masonry.
	Register(Page{
		Name:  "masonry",
		Title: "Masonry",
		Doc:   "docs/antd/masonry.md",
		Sections: []Section{
			{Title: "Basic", Note: "six items columns 3 gutter 0"},
			{Title: "Responsive", Note: "viewport 500 to 800 breakpoint switch"},
			{Title: "Image", Note: "uneven heights shortest column first"},
			{Title: "Dynamic", Note: "append one item onLayoutChange fires"},
		},
	})
}
