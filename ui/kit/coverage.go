package kit

// CoverageStatus for Ant-facing kit capability.
type CoverageStatus string

const (
	// CovReady — kit API exists and is Headless-tested.
	CovReady CoverageStatus = "ready"
	// CovPartial — base available; advanced props deferred.
	CovPartial CoverageStatus = "partial"
	// CovPrimitive — build via primitive only (no kit wrapper yet).
	CovPrimitive CoverageStatus = "primitive"
	// CovLater — planned M4+ / post-M6.
	CovLater CoverageStatus = "later"
)

// CoverageEntry maps one Ant Design component to framework support.
type CoverageEntry struct {
	Ant    string
	Status CoverageStatus
	// Via is kit type or primitive composition hint.
	Via string
	// Milestone when first landed.
	Since string
	Notes string
}

// AntCoverage is the §5.7对照 table (living; update as kit grows).
// Counts are intentional snapshots for M6 DoD.
func AntCoverage() []CoverageEntry {
	return []CoverageEntry{
		// General
		{Ant: "Button", Status: CovReady, Via: "kit.Button", Since: "M1", Notes: "P0 对齐 docs/antd/button.md §6（type/size/shape/variant/color/danger/ghost/block/loading/iconPlacement/a11y）；P1: preset 全色板、loading.delay、autoInsertSpace、wave、href/htmlType"},
		{Ant: "FloatButton", Status: CovReady, Via: "kit.FloatButton / FloatButtonGroup", Since: "Base-ALL", Notes: "P0 对齐 docs/antd/float-button.md §6（type default|primary、shape circle|square、icon/content、tooltip、disabled、loading Ticker、onClick、Group trigger click|hover、open/onOpenChange 受控、placement 四向、closeIcon、a11y AriaLabel、Token 40×40 / square r=8）；P1: badge、BackTop、draggable、href/target/htmlType、semantic classNames/styles、ConfigProvider 全局、展开入场动画像素级、官网逐像素"},
		{Ant: "Icon", Status: CovReady, Via: "kit.Icon / primitive.Icon", Since: "M1", Notes: "P0 对齐 docs/antd/icon.md §6（name 注册表、size 默认16、color Token、rotate、spin Ticker+ReduceMotion、twoTone 实例/全局、SetPainter、CreateFromIconfont+RegisterSource 离线多源、装饰默认 a11y/AriaLabel、hit==layout==paint）；P1: 远程 iconfont.cn scriptUrl、extraCommonProps/真 DOM classNames、全量 SVG Path/上千官方字形、spin 动画像素级/官网哈希、ConfigProvider 全局 Icon 默认"},
		{Ant: "Typography", Status: CovReady, Via: "kit.Typography / NewText / NewTitle / NewParagraph / NewLink", Since: "Base-ALL", Notes: "P0 对齐 docs/antd/typography.md §6（Text/Title/Paragraph/Link、level 1..5 字号 38/30/24/20/16、type secondary|success|warning|danger、disabled、copyable+onCopy+icon、editable Enter/Esc+onChange/onStart/onEnd/onCancel、ellipsis rows/expandable/collapsible/受控 expanded/middle/suffix、actions.placement、strong/code/mark/delete/underline/italic/keyboard、onClick、a11y Label、正文 14 Token、复制成功 Ticker）；P1: semantic classNames/styles、copyable.format/tooltips/tabIndex 深度、editable.autoSize/maxLength/enterIcon/triggerType 全矩阵、ellipsis.tooltip 像素级、复制成功动画像素级、ConfigProvider 全局、后缀/表格示例、官网逐像素"},
		// Layout
		{Ant: "Divider", Status: CovReady, Via: "kit.Divider", Since: "Base-ALL", Notes: "P0 对齐 docs/antd/divider.md §6（orientation/vertical、size small|medium|large|unset margin 8/16/24、variant solid|dashed|dotted + dashed 糖、Title/plain/titlePlacement start|end|center、orientationMargin 默认 0.05、role=separator、Token colorSplit + lineWidth、hit==layout==paint）；P1: semantic classNames/styles 深度、orientationMargin 绝对 px / styles.content.margin 全矩阵、垂直 -0.06em 光学、dashed/dotted 官网纹样逐像素、ConfigProvider 全局"},
		{Ant: "Flex", Status: CovReady, Via: "kit.Flex", Since: "Base-ALL", Notes: "P0 对齐 docs/antd/flex.md §6（orientation/vertical、gap small|medium|large|px 8/16/24、justify、align 默认水平 start/垂直 stretch、wrap、官方 basic/align/gap/wrap/combination、Token、AriaLabel、hit==layout==paint）；P1: flex CSS 简写、component、wrap-reverse、semantic classNames/styles、ConfigProvider 全局 flex 默认、官网逐像素"},
		{Ant: "Grid", Status: CovReady, Via: "kit.Grid", Since: "Base-ALL"},
		{Ant: "Layout", Status: CovReady, Via: "kit.Layout", Since: "Base-ALL"},
		{Ant: "Space", Status: CovReady, Via: "kit.Space", Since: "Base-ALL", Notes: "wrap ✅"},
		{Ant: "Splitter", Status: CovReady, Via: "kit.Splitter", Since: "Base-ALL"},
		// Navigation
		{Ant: "Anchor", Status: CovReady, Via: "kit.Anchor", Since: "Base-ALL", Notes: "ScrollTarget+SyncFromScroll spy"},
		{Ant: "Breadcrumb", Status: CovReady, Via: "kit.Breadcrumb", Since: "Base-ALL"},
		{Ant: "Dropdown", Status: CovReady, Via: "kit.Dropdown", Since: "M4"},
		{Ant: "Menu", Status: CovReady, Via: "kit.Menu", Since: "M3", Notes: "flat; nested later"},
		{Ant: "Pagination", Status: CovReady, Via: "kit.Pagination", Since: "M4"},
		{Ant: "Steps", Status: CovReady, Via: "kit.Steps", Since: "Base-ALL"},
		{Ant: "Tabs", Status: CovReady, Via: "kit.Tabs + Scroll", Since: "M3", Notes: "bar/body ScrollViewport"},
		// Data Entry
		{Ant: "AutoComplete", Status: CovReady, Via: "kit.AutoComplete", Since: "Base-ALL"},
		{Ant: "Cascader", Status: CovReady, Via: "kit.Cascader", Since: "M4"},
		{Ant: "Checkbox", Status: CovReady, Via: "kit.Checkbox", Since: "M2"},
		{Ant: "ColorPicker", Status: CovReady, Via: "kit.ColorPicker", Since: "Base-ALL", Notes: "swatches"},
		{Ant: "DatePicker", Status: CovReady, Via: "kit.DatePicker", Since: "Base-ALL", Notes: "SelectDay+Value; range later"},
		{Ant: "Form", Status: CovReady, Via: "kit.Form + FormModel", Since: "M3"},
		{Ant: "Input", Status: CovReady, Via: "kit.Input", Since: "M2"},
		{Ant: "InputNumber", Status: CovReady, Via: "kit.InputNumber", Since: "Base-ALL"},
		{Ant: "Mentions", Status: CovReady, Via: "kit.Mentions", Since: "Base-ALL"},
		{Ant: "Radio", Status: CovReady, Via: "kit.Radio/RadioGroup", Since: "M2"},
		{Ant: "Rate", Status: CovReady, Via: "kit.Rate", Since: "Base-ALL"},
		{Ant: "Select", Status: CovReady, Via: "kit.Select", Since: "M3"},
		{Ant: "Slider", Status: CovReady, Via: "kit.Slider", Since: "Base-ALL"},
		{Ant: "Switch", Status: CovReady, Via: "kit.Switch", Since: "M2", Notes: "P0 对齐 docs/antd/switch.md §6（checked/value、defaultChecked/defaultValue、controlled、onChange/onClick、disabled、loading、size medium|small、checkedChildren/unCheckedChildren 字符串、a11y role=switch、Token 44×22/28×16、thumb FloatAnim+loading Ticker）；P1: semantic classNames/styles、复杂 ReactNode 内文、Wave/像素级 handle 拉伸、官网逐像素"},
		{Ant: "TimePicker", Status: CovReady, Via: "kit.TimePicker", Since: "Base-ALL"},
		{Ant: "Transfer", Status: CovReady, Via: "kit.Transfer", Since: "M4"},
		{Ant: "TreeSelect", Status: CovReady, Via: "kit.TreeSelect", Since: "Base-ALL"},
		{Ant: "Upload", Status: CovReady, Via: "kit.Upload", Since: "Base-ALL", Notes: "Picker/CapFile inject; host dialog later"},
		// Data Display
		{Ant: "Avatar", Status: CovReady, Via: "kit.Avatar", Since: "Base-ALL"},
		{Ant: "Badge", Status: CovReady, Via: "kit.Badge", Since: "Base-ALL"},
		{Ant: "Calendar", Status: CovReady, Via: "kit.Calendar", Since: "Base-ALL"},
		{Ant: "Card", Status: CovReady, Via: "kit.Card", Since: "Base-ALL"},
		{Ant: "Carousel", Status: CovReady, Via: "kit.Carousel", Since: "Base-ALL"},
		{Ant: "Collapse", Status: CovReady, Via: "kit.Collapse", Since: "Base-ALL"},
		{Ant: "Descriptions", Status: CovReady, Via: "kit.Descriptions", Since: "Base-ALL"},
		{Ant: "Empty", Status: CovReady, Via: "kit.Empty", Since: "Base-ALL"},
		{Ant: "Image", Status: CovReady, Via: "kit.Image", Since: "Base-ALL", Notes: "Src+SetPixels sample; GPU texture later"},
		{Ant: "List", Status: CovReady, Via: "kit.List", Since: "M4"},
		{Ant: "Popover", Status: CovReady, Via: "kit.Popover", Since: "M2"},
		{Ant: "QRCode", Status: CovReady, Via: "kit.QRCode", Since: "Base-ALL", Notes: "deterministic modules; codec later"},
		{Ant: "Segmented", Status: CovReady, Via: "kit.Segmented", Since: "Base-ALL"},
		{Ant: "Statistic", Status: CovReady, Via: "kit.Statistic", Since: "Base-ALL"},
		{Ant: "Table", Status: CovReady, Via: "kit.Table", Since: "M4", Notes: "virtual rows; fixed header (Column, not in-scroll sticky)"},
		{Ant: "Tag", Status: CovReady, Via: "kit.Tag", Since: "Base-ALL"},
		{Ant: "Timeline", Status: CovReady, Via: "kit.Timeline", Since: "Base-ALL"},
		{Ant: "Tooltip", Status: CovReady, Via: "kit.Tooltip", Since: "M2"},
		{Ant: "Tour", Status: CovReady, Via: "kit.Tour", Since: "M5"},
		{Ant: "Tree", Status: CovReady, Via: "kit.Tree", Since: "M4"},
		// Feedback
		{Ant: "Alert", Status: CovReady, Via: "kit.Alert", Since: "Base-ALL"},
		{Ant: "Drawer", Status: CovReady, Via: "kit.Drawer", Since: "M3"},
		{Ant: "Message", Status: CovReady, Via: "kit.MessageHost", Since: "M3"},
		{Ant: "Modal", Status: CovReady, Via: "kit.Modal", Since: "M3"},
		{Ant: "Notification", Status: CovReady, Via: "MessageHost queue", Since: "M3"},
		{Ant: "Popconfirm", Status: CovReady, Via: "kit.Popconfirm", Since: "Base-ALL"},
		{Ant: "Progress", Status: CovReady, Via: "kit.Progress / ProgressRing", Since: "M5"},
		{Ant: "Result", Status: CovReady, Via: "kit.Result", Since: "Base-ALL"},
		{Ant: "Skeleton", Status: CovReady, Via: "kit.Skeleton", Since: "M5"},
		{Ant: "Spin", Status: CovReady, Via: "kit.Spin", Since: "M5"},
		{Ant: "Watermark", Status: CovReady, Via: "kit.Watermark", Since: "Base-ALL"},
		// Other
		{Ant: "Affix", Status: CovReady, Via: "kit.Affix / Sticky", Since: "Base-ALL"},
		// Overflow
		{Ant: "Scroll (overflow)", Status: CovReady, Via: "kit.Scroll / ScrollViewport", Since: "Base-ALL"},
		{Ant: "App", Status: CovReady, Via: "Theme+PortalHost", Since: "M1"},
		{Ant: "ConfigProvider", Status: CovReady, Via: "kit.ConfigProvider", Since: "Base-ALL"},
	}
}

// CoverageSummary counts statuses.
func CoverageSummary(entries []CoverageEntry) map[CoverageStatus]int {
	m := map[CoverageStatus]int{}
	for _, e := range entries {
		m[e.Status]++
	}
	return m
}
