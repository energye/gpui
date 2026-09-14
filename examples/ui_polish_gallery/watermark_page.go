package main

// Watermark P0 main path (docs/antd/watermark.md §6.8): basic / multi-line /
// image / custom / modal-drawer portal. Standalone: -tab=watermark.
func init() {
	Register(Page{
		Name:  "watermark",
		Title: "Watermark",
		Doc:   "docs/antd/watermark.md",
		Sections: []Section{
			{Title: "Basic", Note: "content=Ant Design single line"},
			{Title: "Multi-line", Note: "two rows with per-line fontSize"},
			{Title: "Image", Note: "width/height/image host pixels"},
			{Title: "Custom", Note: "font/gap/offset/rotate/zIndex"},
			{Title: "ModalDrawer", Note: "inherit=true Wrap to popup content"},
		},
	})
}
