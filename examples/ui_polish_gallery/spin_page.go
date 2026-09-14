package main

func init() {
	// Spin P0 main path (docs/antd/spin.md §6.8): basic / size / nested /
	// tip / delay / custom-indicator / percent / style-class.
	// Standalone: -tab=spin.
	Register(Page{
		Name:  "spin",
		Title: "Spin",
		Doc:   "docs/antd/spin.md",
		Sections: []Section{
			{Title: "Basic", Note: "default spinning indicator"},
			{Title: "Size", Note: "small 14 / medium 20 / large 32"},
			{Title: "Nested", Note: "content stays in tree, mask blocks hits"},
			{Title: "Tip", Note: "description with indicator across sizes"},
			{Title: "Delay", Note: "500ms gate against flicker"},
			{Title: "CustomIndicator", Note: "instance beats global default"},
			{Title: "Percent", Note: "numeric ring and auto climb below 100"},
			{Title: "StyleClass", Note: "shallow classNames and styles hooks"},
		},
	})
}
