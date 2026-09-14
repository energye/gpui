package main

func init() {
	// Grid P0 main path (docs/antd/grid.md §6.8): basic / gutter /
	// offset / sort / flex / flex-align / flex-order / flex-stretch.
	// Standalone: -tab=grid.
	Register(Page{
		Name:  "grid",
		Title: "Grid 栅格",
		Doc:   "docs/antd/grid.md",
		Sections: []Section{
			{Title: "Basic", Note: "span 12+12 @1200 gutter 0"},
			{Title: "Gutter", Note: "gutter 16 half padding 8"},
			{Title: "Offset", Note: "offset 6 = 300 @1200"},
			{Title: "Sort", Note: "push/pull pair swaps visuals"},
			{Title: "Flex", Note: "justify center groups columns"},
			{Title: "FlexAlign", Note: "align middle centers short col"},
			{Title: "FlexOrder", Note: "order larger goes later"},
			{Title: "FlexStretch", Note: "flex fill takes remainder"},
		},
	})
}
