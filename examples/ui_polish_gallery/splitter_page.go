package main

func init() {
	// Splitter P0 main path (docs/antd/splitter.md §6.8): the 8 P0 official
	// examples. Standalone: -tab=splitter.
	Register(Page{
		Name:  "splitter",
		Title: "Splitter",
		Doc:   "docs/antd/splitter.md",
		Sections: []Section{
			{Title: "Basic", Note: "size.tsx: two panels 50/50, drag resizes"},
			{Title: "Controlled", Note: "control.tsx: controlled size with onResize writeback"},
			{Title: "Vertical", Note: "vertical.tsx: stacked panels, horizontal bar"},
			{Title: "Collapsible", Note: "collapsible.tsx: fold one panel to zero"},
			{Title: "CollapsibleIcon", Note: "collapsibleIcon.tsx: both-side fold buttons"},
			{Title: "Multiple", Note: "multiple.tsx: three panels, middle bar"},
			{Title: "Group", Note: "group.tsx: nested splitters, independent bars"},
			{Title: "Lazy", Note: "lazy.tsx: preview line mid-drag, commit on release"},
		},
	})
}
