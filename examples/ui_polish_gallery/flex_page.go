package main

func init() {
	// Flex P0 main path (docs/antd/flex.md §6.8): basic / align / gap /
	// wrap / combination. Standalone: -tab=flex.
	Register(Page{
		Name:  "flex",
		Title: "Flex",
		Doc:   "docs/antd/flex.md",
		Sections: []Section{
			{Title: "Basic", Note: "horizontal two items gap small"},
			{Title: "Align", Note: "justify + align center variants"},
			{Title: "Gap", Note: "small 8 / middle 16 / large 24"},
			{Title: "Wrap", Note: "wrap true narrow container"},
			{Title: "Combination", Note: "vertical + justify + align"},
		},
	})
}
