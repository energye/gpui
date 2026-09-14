package main

func init() {
	// Tooltip P0 main path (docs/antd/tooltip.md §6.8): one section per P0
	// official example — basic / smooth-transition-degraded / placement /
	// arrow / shift / colorful / disabled / wrap-custom-component.
	// Standalone: -tab=tooltip. Live nodes mount via widgets.go (gallery
	// owner); TooltipPageDemos documents the per-section representatives.
	Register(Page{
		Name:  "tooltip",
		Title: "Tooltip",
		Doc:   "docs/antd/tooltip.md",
		Sections: []Section{
			{Title: "Basic", Note: "title + hover trigger (TIP-10)"},
			{Title: "Trigger", Note: "hover/focus/click modes (TIP-02/21)"},
			{Title: "Placement", Note: "12-way placement (TIP-12)"},
			{Title: "Arrow", Note: "show/hide/pointAtCenter (TIP-13)"},
			{Title: "Shift", Note: "autoAdjustOverflow edge flip/shift (TIP-14)"},
			{Title: "Colorful", Note: "presets + custom hex (TIP-15)"},
			{Title: "Disabled", Note: "empty title stays shut (TIP-16)"},
			{Title: "Custom", Note: "custom trigger + title slots (TIP-17)"},
		},
	})
}

// TooltipPageDemos names the live representative per section for the gallery
// owner to mount in widgets.go (one node per section, no placeholders).
var TooltipPageDemos = []string{
	"Basic", "Trigger", "Placement", "Arrow",
	"Shift", "Colorful", "Disabled", "Custom",
}
