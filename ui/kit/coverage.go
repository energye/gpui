package kit

// Coverage is the machine-readable mirror of docs/antd/README progress board.
//
// Status values copy the board literally so the two stay consistent:
// 未开工 → 首批中 → 首批完 → 全完. Foundation rows track W0 groundwork;
// Controls rows track one line per component: which §6.8 P0 scope applies
// and which P1 items stay open (see each docs/antd/<name>.md §6.8-§6.12).
type Status string

const (
	NotStarted Status = "未开工"
	InProgress Status = "首批中"
	P0Done     Status = "首批完"
	Done       Status = "全完"
)

// Foundation tracks one W0 groundwork row.
type Foundation struct {
	Item   string
	Status Status
	Note   string
}

// Control tracks one component row.
type Control struct {
	// Name is the kit directory name, e.g. button.
	Name string
	// Doc is the spec file, e.g. docs/antd/button.md.
	Doc string
	// Wave is W0..W5 per README cascade.
	Wave string
	Status Status
	// P0 is the implemented scope (empty while NotStarted).
	P0 string
	// P1 lists deferred items while NotStarted (pointer to spec).
	P1 string
}

// FoundationW0 mirrors README W0 地基 rows.
var FoundationW0 = []Foundation{
	{Item: "kit-scaffold", Status: P0Done, Note: "ui/kit 一控件一目录 + doc.go；§6.10 落 ui/kit/<名>/"},
	{Item: "gallery", Status: P0Done, Note: "examples/ui_polish_gallery 左标签右展示 + catalog 独立打开"},
	{Item: "coverage", Status: P0Done, Note: "ui/kit/coverage.go 与 README 看板一致"},
	{Item: "theme-seed", Status: P0Done, Note: "ui/theme 全局种子对齐 antd v6.5.1（浅色默认）"},
	{Item: "overlay-placement", Status: P0Done, Note: "ui/overlay 十二方向/翻转/箭头/外点关/焦点锁"},
}

// Controls mirrors README W0-W5 + Util rows (72 specs, 71 in kit).
var Controls = []Control{
	{Name: "icon", Doc: "docs/antd/icon.md", Wave: "W0", Status: P0Done, P0: "P0行为/度量/L3对勾截图/画廊页已对齐§6（ICO-01…19绿）", P1: "远程scriptUrl/extraCommonProps/全量SVG/动画像素/Config全局/可聚焦Icon（见 docs/antd/icon.md §6.8 P1）"},
	{Name: "alert", Doc: "docs/antd/alert.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（ALT-01…19绿，ALT-20截图）", P1: "见 docs/antd/alert.md §6.8 P1"},
	{Name: "border-beam", Doc: "docs/antd/border-beam.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（BB-01…15绿）", P1: "见 docs/antd/border-beam.md §6.8 P1"},
	{Name: "button", Doc: "docs/antd/button.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（BTN-01…20/23/24绿）", P1: "见 docs/antd/button.md §6.8 P1"},
	{Name: "divider", Doc: "docs/antd/divider.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（DIV-01…23绿）", P1: "见 docs/antd/divider.md §6.8 P1"},
	{Name: "flex", Doc: "docs/antd/flex.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（FLX-01…18绿）", P1: "见 docs/antd/flex.md §6.8 P1"},
	{Name: "float-button", Doc: "docs/antd/float-button.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（FB-01…26绿）", P1: "见 docs/antd/float-button.md §6.8 P1"},
	{Name: "grid", Doc: "docs/antd/grid.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（GRD-01…19绿）", P1: "见 docs/antd/grid.md §6.8 P1"},
	{Name: "layout", Doc: "docs/antd/layout.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（LAY-01…21绿）", P1: "见 docs/antd/layout.md §6.8 P1"},
	{Name: "masonry", Doc: "docs/antd/masonry.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（MAS-01…16绿）", P1: "见 docs/antd/masonry.md §6.8 P1"},
	{Name: "progress", Doc: "docs/antd/progress.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（PRG-01…22绿）", P1: "见 docs/antd/progress.md §6.8 P1"},
	{Name: "skeleton", Doc: "docs/antd/skeleton.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（SKL-01…18绿）", P1: "见 docs/antd/skeleton.md §6.8 P1"},
	{Name: "space", Doc: "docs/antd/space.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（SPC-01…21绿）", P1: "见 docs/antd/space.md §6.8 P1"},
	{Name: "spin", Doc: "docs/antd/spin.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（SPN-01…18绿）", P1: "见 docs/antd/spin.md §6.8 P1"},
	{Name: "splitter", Doc: "docs/antd/splitter.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（SPL-01…21绿）", P1: "见 docs/antd/splitter.md §6.8 P1"},
	{Name: "statistic", Doc: "docs/antd/statistic.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（STA-01…16绿）", P1: "见 docs/antd/statistic.md §6.8 P1"},
	{Name: "tag", Doc: "docs/antd/tag.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（TAG-01…19绿）", P1: "见 docs/antd/tag.md §6.8 P1"},
	{Name: "timeline", Doc: "docs/antd/timeline.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（TL-01…17绿）", P1: "见 docs/antd/timeline.md §6.8 P1"},
	{Name: "typography", Doc: "docs/antd/typography.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（TYP-01…25绿）", P1: "见 docs/antd/typography.md §6.8 P1"},
	{Name: "watermark", Doc: "docs/antd/watermark.md", Wave: "W1", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（WM-01…14绿）", P1: "见 docs/antd/watermark.md §6.8 P1"},
	{Name: "affix", Doc: "docs/antd/affix.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/affix.md §6.8 P1"},
	{Name: "avatar", Doc: "docs/antd/avatar.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/avatar.md §6.8 P1"},
	{Name: "badge", Doc: "docs/antd/badge.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/badge.md §6.8 P1"},
	{Name: "card", Doc: "docs/antd/card.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/card.md §6.8 P1"},
	{Name: "carousel", Doc: "docs/antd/carousel.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/carousel.md §6.8 P1"},
	{Name: "collapse", Doc: "docs/antd/collapse.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/collapse.md §6.8 P1"},
	{Name: "descriptions", Doc: "docs/antd/descriptions.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/descriptions.md §6.8 P1"},
	{Name: "empty", Doc: "docs/antd/empty.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/empty.md §6.8 P1"},
	{Name: "menu", Doc: "docs/antd/menu.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/menu.md §6.8 P1"},
	{Name: "tooltip", Doc: "docs/antd/tooltip.md", Wave: "W2", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（TIP-01…21绿）", P1: "见 docs/antd/tooltip.md §6.8 P1"},
	{Name: "popover", Doc: "docs/antd/popover.md", Wave: "W2", Status: P0Done, P0: "P0行为/度量/L3截图/画廊页已对齐§6（POP-01…19中P0绿）", P1: "见 docs/antd/popover.md §6.8 P1"},
	{Name: "qr-code", Doc: "docs/antd/qr-code.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/qr-code.md §6.8 P1"},
	{Name: "radio", Doc: "docs/antd/radio.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/radio.md §6.8 P1"},
	{Name: "rate", Doc: "docs/antd/rate.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/rate.md §6.8 P1"},
	{Name: "result", Doc: "docs/antd/result.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/result.md §6.8 P1"},
	{Name: "segmented", Doc: "docs/antd/segmented.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/segmented.md §6.8 P1"},
	{Name: "slider", Doc: "docs/antd/slider.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/slider.md §6.8 P1"},
	{Name: "steps", Doc: "docs/antd/steps.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/steps.md §6.8 P1"},
	{Name: "switch", Doc: "docs/antd/switch.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/switch.md §6.8 P1"},
	{Name: "tabs", Doc: "docs/antd/tabs.md", Wave: "W2", Status: NotStarted, P1: "见 docs/antd/tabs.md §6.8 P1"},
	{Name: "input", Doc: "docs/antd/input.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/input.md §6.8 P1"},
	{Name: "select", Doc: "docs/antd/select.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/select.md §6.8 P1"},
	{Name: "tree", Doc: "docs/antd/tree.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/tree.md §6.8 P1"},
	{Name: "time-picker", Doc: "docs/antd/time-picker.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/time-picker.md §6.8 P1"},
	{Name: "auto-complete", Doc: "docs/antd/auto-complete.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/auto-complete.md §6.8 P1"},
	{Name: "mentions", Doc: "docs/antd/mentions.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/mentions.md §6.8 P1"},
	{Name: "input-number", Doc: "docs/antd/input-number.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/input-number.md §6.8 P1"},
	{Name: "color-picker", Doc: "docs/antd/color-picker.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/color-picker.md §6.8 P1"},
	{Name: "image", Doc: "docs/antd/image.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/image.md §6.8 P1"},
	{Name: "message", Doc: "docs/antd/message.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/message.md §6.8 P1"},
	{Name: "notification", Doc: "docs/antd/notification.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/notification.md §6.8 P1"},
	{Name: "tour", Doc: "docs/antd/tour.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/tour.md §6.8 P1"},
	{Name: "anchor", Doc: "docs/antd/anchor.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/anchor.md §6.8 P1"},
	{Name: "dropdown", Doc: "docs/antd/dropdown.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/dropdown.md §6.8 P1"},
	{Name: "popconfirm", Doc: "docs/antd/popconfirm.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/popconfirm.md §6.8 P1"},
	{Name: "upload", Doc: "docs/antd/upload.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/upload.md §6.8 P1"},
	{Name: "drawer", Doc: "docs/antd/drawer.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/drawer.md §6.8 P1"},
	{Name: "modal", Doc: "docs/antd/modal.md", Wave: "W3", Status: NotStarted, P1: "见 docs/antd/modal.md §6.8 P1"},
	{Name: "checkbox", Doc: "docs/antd/checkbox.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/checkbox.md §6.8 P1"},
	{Name: "pagination", Doc: "docs/antd/pagination.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/pagination.md §6.8 P1"},
	{Name: "calendar", Doc: "docs/antd/calendar.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/calendar.md §6.8 P1"},
	{Name: "breadcrumb", Doc: "docs/antd/breadcrumb.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/breadcrumb.md §6.8 P1"},
	{Name: "list", Doc: "docs/antd/list.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/list.md §6.8 P1"},
	{Name: "table", Doc: "docs/antd/table.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/table.md §6.8 P1"},
	{Name: "transfer", Doc: "docs/antd/transfer.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/transfer.md §6.8 P1"},
	{Name: "cascader", Doc: "docs/antd/cascader.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/cascader.md §6.8 P1"},
	{Name: "date-picker", Doc: "docs/antd/date-picker.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/date-picker.md §6.8 P1"},
	{Name: "tree-select", Doc: "docs/antd/tree-select.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/tree-select.md §6.8 P1"},
	{Name: "form", Doc: "docs/antd/form.md", Wave: "W4", Status: NotStarted, P1: "见 docs/antd/form.md §6.8 P1"},
	{Name: "app", Doc: "docs/antd/app.md", Wave: "W5", Status: NotStarted, P1: "见 docs/antd/app.md §6.8 P1"},
	{Name: "config-provider", Doc: "docs/antd/config-provider.md", Wave: "W5", Status: NotStarted, P1: "见 docs/antd/config-provider.md §6.8 P1"},
	{Name: "util", Doc: "docs/antd/util.md", Wave: "any", Status: NotStarted, P0: "无 UI，不进 kit", P1: "按 §6 做类型对照"},
}

// ByName returns the control row, or nil.
func ByName(name string) *Control {
	for i := range Controls {
		if Controls[i].Name == name {
			return &Controls[i]
		}
	}
	return nil
}
