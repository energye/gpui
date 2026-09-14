package main

func init() {
	// Button P0 main path (docs/antd/button.md §6.8): 11 examples —
	// type/size/disabled/loading/icon/icon-placement/multiple/ghost/
	// danger/block/color-variant. Standalone: -tab=button.
	Register(Page{
		Name:  "button",
		Title: "Button",
		Doc:   "docs/antd/button.md",
		Sections: []Section{
			{Title: "Type", Note: "primary default dashed text link syntax sugar"},
			{Title: "Size", Note: "small 24 / middle 32 / large 40 heights"},
			{Title: "Disabled", Note: "disabled swallows click, flat chrome"},
			{Title: "Loading", Note: "spinner + no repeat submit"},
			{Title: "Icon", Note: "icon with text mix"},
			{Title: "IconPlacement", Note: "start/end order, RTL mirrored"},
			{Title: "Multiple", Note: "horizontal spacing row"},
			{Title: "Ghost", Note: "transparent fill on dark backdrop"},
			{Title: "Danger", Note: "error colorway solid/outlined"},
			{Title: "Block", Note: "width fills parent, height keeps size"},
			{Title: "ColorVariant", Note: "solid/outlined x primary/default/danger"},
		},
	})
}
