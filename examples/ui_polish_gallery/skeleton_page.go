package main

func init() {
	// Skeleton P0 main path (docs/antd/skeleton.md §6.8): basic / complex /
	// active / element / children / list / style-class / semantic.
	// Standalone: -tab=skeleton.
	Register(Page{
		Name:  "skeleton",
		Title: "Skeleton",
		Doc:   "docs/antd/skeleton.md",
		Sections: []Section{
			{Title: "Basic", Note: "title + 3 rows, last 61%"},
			{Title: "Complex", Note: "avatar + title + 2 rows"},
			{Title: "Active", Note: "1.4s shimmer, reduced-motion stops"},
			{Title: "Element", Note: "avatar/button/input/image/node sizes"},
			{Title: "Children", Note: "loading false shows content"},
			{Title: "List", Note: "active avatar rows=4 repeated"},
			{Title: "StyleClass", Note: "shallow classNames/styles hooks"},
			{Title: "Semantic", Note: "root/header/section/avatar/title/paragraph"},
		},
	})
}
