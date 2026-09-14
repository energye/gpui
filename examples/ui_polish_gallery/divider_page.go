package main

func init() {
	// Divider P0 main path (docs/antd/divider.md §6.8): horizontal /
	// with-text / size / plain / vertical / variant, style-class示意.
	// Standalone: -tab=divider.
	Register(Page{
		Name:  "divider",
		Title: "Divider",
		Doc:   "docs/antd/divider.md",
		Sections: []Section{
			{Title: "Horizontal", Note: "horizontal solid + dashed"},
			{Title: "WithText", Note: "center start end title rails"},
			{Title: "Size", Note: "small medium large marginBlock 8/16/24"},
			{Title: "Plain", Note: "plain body-size title"},
			{Title: "Vertical", Note: "inline 0.9em rail"},
			{Title: "Variant", Note: "solid dashed dotted"},
			{Title: "StyleClass", Note: "semantic structure mount only (depth P1)"},
		},
	})
}
