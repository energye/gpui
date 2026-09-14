package main

func init() {
	// Tag P0 main path (docs/antd/tag.md §6.8): basic / colorful /
	// control / checkable / animation / icon / status / draggable.
	// Standalone: -tab=tag.
	Register(Page{
		Name:  "tag",
		Title: "Tag",
		Doc:   "docs/antd/tag.md",
		Sections: []Section{
			{Title: "Basic", Note: "default + closable close/prevent"},
			{Title: "Colorful", Note: "presets x filled/solid/outlined + hex"},
			{Title: "Control", Note: "dynamic add/remove tags"},
			{Title: "Checkable", Note: "checkable + group single/multi"},
			{Title: "Animation", Note: "instant add/remove (pixel P1)"},
			{Title: "Icon", Note: "tag/checkable leading icon"},
			{Title: "Status", Note: "success/processing/error/warning x variants"},
			{Title: "Draggable", Note: "group reorder combo"},
		},
	})
}
