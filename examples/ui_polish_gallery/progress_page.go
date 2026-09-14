package main

func init() {
	// Progress P0 main path (docs/antd/progress.md §6.8): line / circle /
	// line-mini / circle-micro / circle-mini / dynamic / format / dashboard.
	// Standalone: -tab=progress.
	Register(Page{
		Name:  "progress",
		Title: "Progress",
		Doc:   "docs/antd/progress.md",
		Sections: []Section{
			{Title: "Line", Note: "30/50 active/70 exception/100/50 hideInfo"},
			{Title: "Circle", Note: "75/70 exception/100"},
			{Title: "LineMini", Note: "size=small line height 6"},
			{Title: "CircleMicro", Note: "small size + strokeWidth + format"},
			{Title: "CircleMini", Note: "size=80 three rings"},
			{Title: "Dynamic", Note: "SetPercent ±10 no root rebuild"},
			{Title: "Format", Note: "format return shows in info"},
			{Title: "Dashboard", Note: "type=dashboard + gapDegree"},
		},
	})
}
