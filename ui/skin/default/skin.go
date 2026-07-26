// Package skindefault provides the default Ant-leaning TokenSet and Skin (L3c).
// Import as skindefault to avoid clashing with package name "default".
//
//	import skindefault "github.com/energye/gpui/ui/skin/default"
package skindefault

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Product typeIDs as string literals to avoid importing ui/kit (cycle).
// Must stay equal to kit.TypeButton / kit.TypeBreadcrumb etc.
const (
	typeButton         = "kit.Button"
	typeBreadcrumb     = "kit.Breadcrumb"
	typeInput          = "kit.Input"
	typeSwitch         = "kit.Switch"
	typeCheckbox       = "kit.Checkbox"
	typeRadio          = "kit.Radio"
	typeTag            = "kit.Tag"
	typeFloatButton    = "kit.FloatButton"
	typeBadge          = "kit.Badge"
	typeAlert          = "kit.Alert"
	typeProgress       = "kit.Progress"
	typeSpin           = "kit.Spin"
	typeSkeleton       = "kit.Skeleton"
	typeEmpty          = "kit.Empty"
	typeResult         = "kit.Result"
	typeStatistic      = "kit.Statistic"
	typeAvatar         = "kit.Avatar"
	typeDivider        = "kit.Divider"
	typeIcon           = "kit.Icon"
	typeTypography     = "kit.Typography"
	typeScroll         = "kit.Scroll"
	typeTour           = "kit.Tour"
	typeConfigProvider = "kit.ConfigProvider"
	typeBorderBeam     = "kit.BorderBeam"
	typeWatermark      = "kit.Watermark"
	typeAffix          = "kit.Affix"
	typeQRCode         = "kit.QRCode"
	typeTimeline       = "kit.Timeline"
	typeImage          = "kit.Image"
	typeCollapse       = "kit.Collapse"
	typeCarousel       = "kit.Carousel"
	typeCalendar       = "kit.Calendar"
	typeCard           = "kit.Card"
	typeDescriptions   = "kit.Descriptions"
	typeList           = "kit.List"
	typeTable          = "kit.Table"
	typeTree           = "kit.Tree"
	typeTransfer       = "kit.Transfer"
	typeSegmented      = "kit.Segmented"
	typeColorPicker    = "kit.ColorPicker"
	typeSlider         = "kit.Slider"
	typeRate           = "kit.Rate"
	typeForm           = "kit.Form"
	typeUpload         = "kit.Upload"
	typeTimePicker     = "kit.TimePicker"
	typeDatePicker     = "kit.DatePicker"
	typeTreeSelect     = "kit.TreeSelect"
	typeCascader       = "kit.Cascader"
	typeAutoComplete   = "kit.AutoComplete"
	typeMentions       = "kit.Mentions"
	typeInputNumber    = "kit.InputNumber"
	typeSelect         = "kit.Select"
	typePopconfirm     = "kit.Popconfirm"
	typeTooltip        = "kit.Tooltip"
	typePopover        = "kit.Popover"
	typeNotification   = "kit.Notification"
	typeMessage        = "kit.Message"
	typeDrawer         = "kit.Drawer"
	typeModal          = "kit.Modal"
	typeLayout         = "kit.Layout"
	typeSplitter       = "kit.Splitter"
	typeKitFlex        = "kit.Flex"
	typeSpace          = "kit.Space"
	typeAnchor         = "kit.Anchor"
	typeSteps          = "kit.Steps"
	typePagination     = "kit.Pagination"
	typeDropdown       = "kit.Dropdown"
	typeMenu           = "kit.Menu"
	typeTabs           = "kit.Tabs"
)

// Tokens returns a clone of the Ant light token table.
func Tokens() *core.TokenSet {
	return core.AntLightTokens()
}

// NewSkin builds the default map skin with painters for common primitives
// and product chrome hooks.
//
//	TypeDecorated  — generic box chrome
//	kit.Button     — Button Decorated chrome (default = PaintDecorated)
//	kit.Breadcrumb — Breadcrumb Flex root (default = paint children)
//	kit.Input      — Input Decorated chrome
//	kit.Switch     — Switch track Decorated chrome
//	kit.Checkbox   — Checkbox indicator Decorated chrome
//	kit.Radio      — Radio indicator / button chrome
//	kit.Tag         — Tag Decorated chrome
//	kit.FloatButton — FAB Decorated chrome
//	kit.Badge       — Badge host + count capsule
//	kit.Alert       — Alert Decorated chrome
//	kit.Progress    — Progress host
//
// Decorated chrome is delegated to primitive.PaintDecorated (single source of truth).
func NewSkin() *core.MapSkin {
	s := core.NewMapSkin()
	paintDeco := func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
		}
	}
	s.Set(primitive.TypeDecorated, paintDeco)
	// Button tags its Decorated with SkinType=kit.Button so product skins can
	// override only buttons without replacing all Decorated chrome.
	s.Set(typeButton, paintDeco)
	// Breadcrumb root is a Flex tagged SkinType=kit.Breadcrumb.
	s.Set(typeBreadcrumb, func(pc *core.PaintContext, n core.Node) {
		if f, ok := n.(*primitive.Flex); ok {
			f.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeInput, paintDeco)
	s.Set(typeSwitch, paintDeco)
	s.Set(typeCheckbox, paintDeco)
	s.Set(typeRadio, paintDeco)
	s.Set(typeTag, paintDeco)
	s.Set(typeFloatButton, paintDeco)
	s.Set(typeBadge, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		// badgeHost / other: walk children
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeAlert, paintDeco)
	s.Set(typeProgress, func(pc *core.PaintContext, n core.Node) {
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeSpin, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeSkeleton, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeEmpty, paintDeco)
	s.Set(typeResult, func(pc *core.PaintContext, n core.Node) {
		if f, ok := n.(*primitive.Flex); ok {
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeStatistic, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeAvatar, paintDeco)
	s.Set(typeDivider, func(pc *core.PaintContext, n core.Node) {
		if f, ok := n.(*primitive.Flex); ok {
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeIcon, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTypography, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTabs, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if f, ok := n.(*primitive.Flex); ok {
			if f.SkinType == "" {
				// ok
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeMenu, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if f, ok := n.(*primitive.Flex); ok {
			if f.SkinType == "" {
				// ok
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeDropdown, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if f, ok := n.(*primitive.Flex); ok {
			if f.SkinType == "" {
				// ok
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typePagination, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if f, ok := n.(*primitive.Flex); ok {
			if f.SkinType == "" {
				// ok
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeSteps, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if f, ok := n.(*primitive.Flex); ok {
			if f.SkinType == "" {
				// ok
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeAnchor, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if f, ok := n.(*primitive.Flex); ok {
			if f.SkinType == "" {
				// ok
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeSpace, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if f, ok := n.(*primitive.Flex); ok {
			if f.SkinType == "" {
				// ok
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeKitFlex, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if f, ok := n.(*primitive.Flex); ok {
			if f.SkinType == "" {
				// ok
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeScroll, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if f, ok := n.(*primitive.Flex); ok {
			if f.SkinType == "" {
				// ok
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeLayout, func(pc *core.PaintContext, n core.Node) {
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeSplitter, func(pc *core.PaintContext, n core.Node) {
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeModal, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeDrawer, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeMessage, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeNotification, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typePopover, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTooltip, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typePopconfirm, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTour, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeSelect, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeInputNumber, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeMentions, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeAutoComplete, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeCascader, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTreeSelect, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeDatePicker, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTimePicker, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeUpload, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeForm, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeRate, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeSlider, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeColorPicker, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeSegmented, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTransfer, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTree, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTable, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeList, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeDescriptions, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeCard, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeCalendar, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeCarousel, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeCollapse, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeImage, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeTimeline, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeQRCode, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeAffix, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeWatermark, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeBorderBeam, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	s.Set(typeConfigProvider, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok {
			primitive.PaintDecorated(pc, d)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	return s
}

// Theme returns core.DefaultTheme with default skin attached.
func Theme() *core.Theme {
	th := core.DefaultTheme()
	th.Skin = NewSkin()
	return th
}
