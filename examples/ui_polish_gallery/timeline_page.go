package main

func init() {
	// Timeline P0 main path (docs/antd/timeline.md §6.8): basic / variant /
	// pending / alternate / horizontal / custom / end / title.
	// Standalone: -tab=timeline.
	Register(Page{
		Name:  "timeline",
		Title: "Timeline",
		Doc:   "docs/antd/timeline.md",
		Sections: []Section{
			{Title: "Basic", Note: "basic.tsx 4 items content"},
			{Title: "Variant", Note: "variant.tsx outlined/filled dots"},
			{Title: "Pending", Note: "pending.tsx reverse + item.loading last"},
			{Title: "Alternate", Note: "alternate.tsx mode alternate"},
			{Title: "Horizontal", Note: "horizontal.tsx orientation horizontal"},
			{Title: "Custom", Note: "custom.tsx icon custom point"},
			{Title: "End", Note: "end.tsx mode end"},
			{Title: "Title", Note: "title.tsx title + content"},
		},
	})
}
