package main

func init() {
	// BorderBeam P0 main path (docs/antd/border-beam.md §6.8):
	// basic / hover / custom-container / customized-color /
	// duration / size / line-width. Standalone: -tab=border-beam.
	Register(Page{
		Name:  "border-beam",
		Title: "BorderBeam",
		Doc:   "docs/antd/border-beam.md",
		Sections: []Section{
			{Title: "Basic", Note: "basic.tsx default beam on container"},
			{Title: "Hover", Note: "hover.tsx showOnHover beam"},
			{Title: "CustomContainer", Note: "custom-container.tsx radius 8 host"},
			{Title: "CustomizedColor", Note: "customized-color.tsx multi-stop gradient"},
			{Title: "Duration", Note: "duration.tsx 3s 6s 12s档"},
			{Title: "Size", Note: "size.tsx beam length 100 56 160"},
			{Title: "LineWidth", Note: "line-width.tsx lineWidth 2"},
		},
	})
}
