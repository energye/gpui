package main

func init() {
	// Typography P0 main path (docs/antd/typography.md §6.8): basic /
	// title / text-link / editable / copyable / ellipsis /
	// ellipsis-controlled / ellipsis-middle. Standalone: -tab=typography.
	Register(Page{
		Name:  "typography",
		Title: "Typography",
		Doc:   "docs/antd/typography.md",
		Sections: []Section{
			{Title: "Basic", Note: "Text Title Paragraph Link one line each"},
			{Title: "Title", Note: "level 1..5 size 38/30/24/20/16"},
			{Title: "TextLink", Note: "type secondary success warning danger + link click"},
			{Title: "Editable", Note: "enter submits onChange, esc cancels"},
			{Title: "Copyable", Note: "copy to clipboard + onCopy once"},
			{Title: "Ellipsis", Note: "narrow rows truncated with mark"},
			{Title: "EllipsisControlled", Note: "SetExpanded toggles full + onExpand"},
			{Title: "EllipsisMiddle", Note: "head + mark + fixed tail suffix"},
		},
	})
}
