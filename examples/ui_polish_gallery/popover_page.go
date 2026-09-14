package main

func init() {
	// Popover P0 main path (docs/antd/popover.md §6.8): 7 examples —
	// basic/trigger/placement/arrow/shift/control/hover-click.
	// Standalone: -tab=popover.
	Register(Page{
		Name:  "popover",
		Title: "Popover",
		Doc:   "docs/antd/popover.md",
		Sections: []Section{
			{Title: "Basic", Note: "title+content hover (POP-08)"},
			{Title: "Trigger", Note: "hover/focus/click modes (POP-09)"},
			{Title: "Placement", Note: "12-way placement (POP-10)"},
			{Title: "Arrow", Note: "arrow on/off/pointAtCenter (POP-11)"},
			{Title: "Shift", Note: "autoAdjustOverflow viewport flip/shift (POP-12)"},
			{Title: "Control", Note: "controlled open + inner close (POP-13)"},
			{Title: "HoverClick", Note: "hover+click mixed modes (POP-14)"},
		},
	})
}
