// Package kit is the product control surface (Ant Design tokens + composition).
//
// Controls compose ui/primitive only — no OS, no GPU device ownership.
// Prefer shared Style / theme tokens; keep chrome in Decorated + Pressable;
// avoid per-control layout hacks (hit must match paint Offset).
//
// # Developer guide (required reading)
//
//	docs/UI_KIT_DEV_GUIDE.md     — contracts for kit work on the stabilized foundation
//	docs/UI_KIT_ANT_V5_SPEC.md   — Ant Design v5 alignment goals, waves, L1–L4 acceptance
//	docs/UI_FOUNDATION_P0.md     — what the foundation delivered (Present, overlays, theme, …)
//	docs/LAYOUT_FOUNDATION.md    — hit == layout == paint
//
// Rules of thumb:
//   - Align Ant via behavior + tokens + in-repo golden (not browser pixel hash).
//   - DefaultXxx + SetXxx for chrome metrics (docs/UI_KIT_DEV_GUIDE.md §0.1) — Ant real defaults; override at use site.
//   - Demos: every P0 capability goes in examples/ui_polish_gallery under the matching
//     control page/tab (see docs/antd/README.md «ui_polish_gallery» + each control §6.12).
//     Scope = that control's docs/antd/<name>.md §6.8 P0 (not full §1–§3 demo matrix).
//     P1 may skip gallery; note in coverage.go. Util (no UI) is exempt.
//   - No ContinuousRender for product controls (use Tree.AddTicker).
//   - No per-frame Sync() requirement for popups (Tree.Layout refreshes anchors).
//   - Theme via themeOf / ConfigProvider / Tree.SetTheme + SetThemeHook(rebuild).
//   - True-window present: app.OwnedPresenter or exboot.RunUIDemand only.
//
// # File layout
//
//   - One exported control per file (button.go, modal.go, …).
//   - Co-located helpers for that control stay in the same file
//     (e.g. FormItem in form.go, MenuItem in menu.go, sliderHost in slider.go).
//   - Category-shared helpers live in:
//     general_common.go, layout_common.go, navigation_common.go,
//     entry_common.go, display_common.go, feedback_common.go, other_common.go.
//   - Cross-cutting Ant chrome tokens: ant_chrome.go
//   - Theme resolve: theme_resolve.go (themeOf)
//   - Theme entry: DefaultTheme via ui/skin/default.
//   - Coverage table: coverage.go (AntCoverage) — source of truth for Ready/Notes.
//
// DefaultTheme returns Ant-leaning tokens via ui/skin/default.
package kit

import (
	"github.com/energye/gpui/ui/core"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// Type IDs for plugin/skin hooks (Theme.Skin painters / Decorated.SkinType).
const (
	TypeButton         = "kit.Button"
	TypeBreadcrumb     = "kit.Breadcrumb"
	TypeInput          = "kit.Input"
	TypeSwitch         = "kit.Switch"
	TypeCheckbox       = "kit.Checkbox"
	TypeRadio          = "kit.Radio"
	TypeTag            = "kit.Tag"
	TypeFloatButton    = "kit.FloatButton"
	TypeBadge          = "kit.Badge"
	TypeAlert          = "kit.Alert"
	TypeProgress       = "kit.Progress"
	TypeSpin           = "kit.Spin"
	TypeSkeleton       = "kit.Skeleton"
	TypeEmpty          = "kit.Empty"
	TypeResult         = "kit.Result"
	TypeStatistic      = "kit.Statistic"
	TypeAvatar         = "kit.Avatar"
	TypeDivider        = "kit.Divider"
	TypeIcon           = "kit.Icon"
	TypeText           = "kit.Text"
	TypeTypography     = "kit.Typography"
	TypeTabs           = "kit.Tabs"
	TypeMenu           = "kit.Menu"
	TypeDropdown       = "kit.Dropdown"
	TypePagination     = "kit.Pagination"
	TypeSteps          = "kit.Steps"
	TypeAnchor         = "kit.Anchor"
	TypeSpace          = "kit.Space"
	TypeKitFlex        = "kit.Flex"
	TypeScroll         = "kit.Scroll"
	TypeLayout         = "kit.Layout"
	TypeHeader         = "kit.Header"
	TypeFooter         = "kit.Footer"
	TypeContent        = "kit.Content"
	TypeSider          = "kit.Sider"
	TypeSplitter       = "kit.Splitter"
	TypeSplitterPanel  = "kit.SplitterPanel"
	TypeSplitterBar    = "kit.SplitterBar"
	TypeModal          = "kit.Modal"
	TypeDrawer         = "kit.Drawer"
	TypeMessage        = "kit.Message"
	TypeNotification   = "kit.Notification"
	TypePopover        = "kit.Popover"
	TypeTooltip        = "kit.Tooltip"
	TypePopconfirm     = "kit.Popconfirm"
	TypeTour           = "kit.Tour"
	TypeSelect         = "kit.Select"
	TypeInputNumber    = "kit.InputNumber"
	TypeMentions       = "kit.Mentions"
	TypeAutoComplete   = "kit.AutoComplete"
	TypeCascader       = "kit.Cascader"
	TypeTreeSelect     = "kit.TreeSelect"
	TypeDatePicker     = "kit.DatePicker"
	TypeTimePicker     = "kit.TimePicker"
	TypeUpload         = "kit.Upload"
	TypeForm           = "kit.Form"
	TypeRate           = "kit.Rate"
	TypeSlider         = "kit.Slider"
	TypeColorPicker    = "kit.ColorPicker"
	TypeSegmented      = "kit.Segmented"
	TypeTransfer       = "kit.Transfer"
	TypeTable          = "kit.Table"
	TypeTree           = "kit.Tree"
	TypeList           = "kit.List"
	TypeDescriptions   = "kit.Descriptions"
	TypeCard           = "kit.Card"
	TypeCalendar       = "kit.Calendar"
	TypeCarousel       = "kit.Carousel"
	TypeCollapse       = "kit.Collapse"
	TypeImage          = "kit.Image"
	TypeTimeline       = "kit.Timeline"
	TypeQRCode         = "kit.QRCode"
	TypeAffix          = "kit.Affix"
	TypeWatermark      = "kit.Watermark"
	TypeBorderBeam     = "kit.BorderBeam"
	TypeConfigProvider = "kit.ConfigProvider"
)

// DefaultTheme returns the default product theme (Ant-leaning tokens + skin).
func DefaultTheme() *core.Theme {
	return skindefault.Theme()
}
