package main

func init() {
	// FloatButton P0 main path (docs/antd/float-button.md §6.8): basic /
	// type / shape / content / tooltip / group / group-menu / controlled /
	// placement. Standalone: -tab=float-button.
	Register(Page{
		Name:  "float-button",
		Title: "FloatButton 悬浮按钮",
		Doc:   "docs/antd/float-button.md",
		Sections: []Section{
			{Title: "Basic 基本", Note: "默认单钮，点击 1 次，边长 40"},
			{Title: "Type 类型", Note: "default + primary 并存"},
			{Title: "Shape 形状", Note: "circle r=20 + square r=8"},
			{Title: "Content 描述", Note: "content 文字钮，字号 12"},
			{Title: "Tooltip 气泡卡片", Note: "悬停气泡文案，不抢主点击"},
			{Title: "Group 浮动按钮组", Note: "无 trigger 子钮常显，纵向间距 16"},
			{Title: "GroupMenu 菜单模式", Note: "trigger click/hover + 组外点击收起"},
			{Title: "Controlled 受控模式", Note: "SetOpen(true/false) 驱动显隐"},
			{Title: "Placement 弹出方向", Note: "top/left/right/bottom 四向行为"},
		},
	})
}
