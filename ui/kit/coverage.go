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
		{Ant: "Button", Status: CovReady, Via: "kit.Button", Since: "M1"},
		{Ant: "FloatButton", Status: CovLater, Via: "primitive Pressable+Stack", Since: ""},
		{Ant: "Icon", Status: CovReady, Via: "kit.Icon / primitive.Icon", Since: "M1"},
		{Ant: "Typography", Status: CovPartial, Via: "kit.Text", Since: "M1", Notes: "no copyable/ellipsis multi yet"},
		// Layout
		{Ant: "Divider", Status: CovPrimitive, Via: "primitive.Divider", Since: "M1"},
		{Ant: "Flex", Status: CovPrimitive, Via: "primitive.Flex", Since: "M0"},
		{Ant: "Grid", Status: CovPrimitive, Via: "primitive.Grid", Since: "M4"},
		{Ant: "Layout", Status: CovPartial, Via: "Flex/Box composition", Since: "M0"},
		{Ant: "Space", Status: CovPrimitive, Via: "Flex+gap", Since: "M0"},
		{Ant: "Splitter", Status: CovPrimitive, Via: "primitive.SplitPane", Since: "M4"},
		// Navigation
		{Ant: "Anchor", Status: CovLater, Via: "ScrollSpy", Since: ""},
		{Ant: "Breadcrumb", Status: CovLater, Via: "Flex+Pressable", Since: ""},
		{Ant: "Dropdown", Status: CovReady, Via: "kit.Dropdown", Since: "M4"},
		{Ant: "Menu", Status: CovPartial, Via: "kit.Menu", Since: "M3", Notes: "flat; nested later"},
		{Ant: "Pagination", Status: CovReady, Via: "kit.Pagination", Since: "M4"},
		{Ant: "Steps", Status: CovLater, Via: "Flex+Decorated", Since: ""},
		{Ant: "Tabs", Status: CovPartial, Via: "kit.Tabs", Since: "M3"},
		// Data Entry
		{Ant: "AutoComplete", Status: CovLater, Via: "Input+popup list", Since: ""},
		{Ant: "Cascader", Status: CovPartial, Via: "kit.Cascader", Since: "M4"},
		{Ant: "Checkbox", Status: CovReady, Via: "kit.Checkbox", Since: "M2"},
		{Ant: "ColorPicker", Status: CovLater, Via: "Canvas+popup", Since: ""},
		{Ant: "DatePicker", Status: CovLater, Via: "Grid calendar", Since: ""},
		{Ant: "Form", Status: CovPartial, Via: "kit.Form + FormModel", Since: "M3"},
		{Ant: "Input", Status: CovReady, Via: "kit.Input", Since: "M2"},
		{Ant: "InputNumber", Status: CovLater, Via: "EditableText+step", Since: ""},
		{Ant: "Mentions", Status: CovLater, Via: "Editable+popup", Since: ""},
		{Ant: "Radio", Status: CovReady, Via: "kit.Radio/RadioGroup", Since: "M2"},
		{Ant: "Rate", Status: CovLater, Via: "Flex+Pressable icons", Since: ""},
		{Ant: "Select", Status: CovPartial, Via: "kit.Select", Since: "M3"},
		{Ant: "Slider", Status: CovLater, Via: "Draggable", Since: ""},
		{Ant: "Switch", Status: CovReady, Via: "kit.Switch", Since: "M2"},
		{Ant: "TimePicker", Status: CovLater, Via: "—", Since: ""},
		{Ant: "Transfer", Status: CovPartial, Via: "kit.Transfer", Since: "M4"},
		{Ant: "TreeSelect", Status: CovLater, Via: "Select+Tree", Since: ""},
		{Ant: "Upload", Status: CovLater, Via: "FileHost Cap", Since: ""},
		// Data Display
		{Ant: "Avatar", Status: CovLater, Via: "Box+clip", Since: ""},
		{Ant: "Badge", Status: CovLater, Via: "Stack", Since: ""},
		{Ant: "Calendar", Status: CovLater, Via: "Grid", Since: ""},
		{Ant: "Card", Status: CovPrimitive, Via: "Decorated+Flex", Since: "M1"},
		{Ant: "Carousel", Status: CovLater, Via: "Clip+offset", Since: ""},
		{Ant: "Collapse", Status: CovLater, Via: "Motion height", Since: "M5 motion ready"},
		{Ant: "Descriptions", Status: CovLater, Via: "Grid", Since: ""},
		{Ant: "Empty", Status: CovLater, Via: "Flex+Text", Since: ""},
		{Ant: "Image", Status: CovLater, Via: "C-Image", Since: ""},
		{Ant: "List", Status: CovPartial, Via: "kit.List", Since: "M4"},
		{Ant: "Popover", Status: CovPartial, Via: "kit.Popover", Since: "M2"},
		{Ant: "QRCode", Status: CovLater, Via: "Canvas", Since: "M5 canvas ready"},
		{Ant: "Segmented", Status: CovLater, Via: "Flex+Selection", Since: ""},
		{Ant: "Statistic", Status: CovLater, Via: "Text", Since: ""},
		{Ant: "Table", Status: CovPartial, Via: "kit.Table", Since: "M4", Notes: "virtual rows; sticky head"},
		{Ant: "Tag", Status: CovLater, Via: "Decorated+Text", Since: ""},
		{Ant: "Timeline", Status: CovLater, Via: "Flex", Since: ""},
		{Ant: "Tooltip", Status: CovPartial, Via: "kit.Tooltip", Since: "M2"},
		{Ant: "Tour", Status: CovPartial, Via: "kit.Tour", Since: "M5"},
		{Ant: "Tree", Status: CovPartial, Via: "kit.Tree", Since: "M4"},
		// Feedback
		{Ant: "Alert", Status: CovLater, Via: "Decorated+Text", Since: ""},
		{Ant: "Drawer", Status: CovPartial, Via: "kit.Drawer", Since: "M3"},
		{Ant: "Message", Status: CovPartial, Via: "kit.MessageHost", Since: "M3"},
		{Ant: "Modal", Status: CovPartial, Via: "kit.Modal", Since: "M3"},
		{Ant: "Notification", Status: CovPartial, Via: "MessageHost queue", Since: "M3"},
		{Ant: "Popconfirm", Status: CovLater, Via: "Popover+Buttons", Since: ""},
		{Ant: "Progress", Status: CovReady, Via: "kit.Progress / ProgressRing", Since: "M5"},
		{Ant: "Result", Status: CovLater, Via: "Flex", Since: ""},
		{Ant: "Skeleton", Status: CovReady, Via: "kit.Skeleton", Since: "M5"},
		{Ant: "Spin", Status: CovReady, Via: "kit.Spin", Since: "M5"},
		{Ant: "Watermark", Status: CovLater, Via: "Canvas overlay", Since: ""},
		// Other
		{Ant: "Affix", Status: CovPrimitive, Via: "Sticky", Since: "M4"},
		{Ant: "App", Status: CovPartial, Via: "Theme+PortalHost", Since: "M1"},
		{Ant: "ConfigProvider", Status: CovPartial, Via: "Theme/Token/Density", Since: "M1/M5"},
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
