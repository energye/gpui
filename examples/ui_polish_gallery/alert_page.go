package main

func init() {
	// Alert P0 main path (docs/antd/alert.md §6.8): one section per P0
	// official example plus the action slot. Standalone: -tab=alert.
	Register(Page{
		Name:  "alert",
		Title: "Alert",
		Doc:   "docs/antd/alert.md",
		Sections: []Section{
			{Title: "Basic", Note: "title + type=success (ALT-08)"},
			{Title: "FourTypes", Note: "success/info/warning/error (ALT-09)"},
			{Title: "Filled", Note: "variant=filled borderless (ALT-10)"},
			{Title: "Closable", Note: "four types + closable (ALT-11)"},
			{Title: "Description", Note: "four types + description double row (ALT-12)"},
			{Title: "Icon", Note: "showIcon ± description ± closable (ALT-13)"},
			{Title: "Banner", Note: "top banner combo (ALT-14)"},
			{Title: "LoopBanner", Note: "banner + TitleNode long text (ALT-15)"},
			{Title: "Action", Note: "right slot single/double buttons + desc (ALT-15b)"},
		},
	})
}
