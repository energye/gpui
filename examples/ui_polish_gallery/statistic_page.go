package main

func init() {
	// Statistic P0 main path (docs/antd/statistic.md §6.8): basic / unit /
	// animated (formatter approx) / card / timer / style-class.
	// Standalone: -tab=statistic.
	Register(Page{
		Name:  "statistic",
		Title: "Statistic",
		Doc:   "docs/antd/statistic.md",
		Sections: []Section{
			{Title: "Basic", Note: "title+value+precision+loading"},
			{Title: "Unit", Note: "prefix like + suffix / 100"},
			{Title: "Animated", Note: "formatter instant final (pixel P1)"},
			{Title: "Card", Note: "content color + prefix/suffix in card"},
			{Title: "Timer", Note: "countdown/countup + format + onChange/onFinish"},
			{Title: "StyleClass", Note: "shallow styles/classNames"},
		},
	})
}
