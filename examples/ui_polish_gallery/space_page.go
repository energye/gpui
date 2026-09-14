package main

func init() {
	// Space P0 main path (docs/antd/space.md §6.8): basic / vertical /
	// size / align / wrap / separator / compact / compact-buttons.
	// Standalone: -tab=space.
	Register(Page{
		Name:  "space",
		Title: "Space",
		Doc:   "docs/antd/space.md",
		Sections: []Section{
			{Title: "Basic", Note: "base.tsx three children size small gap 8"},
			{Title: "Vertical", Note: "vertical.tsx vertical stack gap 8"},
			{Title: "Size", Note: "size.tsx small 8 / middle 16 / large 24"},
			{Title: "Align", Note: "align.tsx unequal heights align center"},
			{Title: "Wrap", Note: "wrap.tsx wrap true narrow container"},
			{Title: "Separator", Note: "separator.tsx one node per gap aria-hidden"},
			{Title: "Compact", Note: "compact.tsx shared border overlap lineWidth"},
			{Title: "CompactButtons", Note: "compact-buttons.tsx middle cell radius cleared"},
		},
	})
}
