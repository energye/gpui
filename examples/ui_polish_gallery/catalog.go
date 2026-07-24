//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

// buildCatalogPanels builds left Tabs rail:
//
//	General (gray header, not clickable)
//	-
//	Button / FloatButton / Icon / Typography (each has own content)
//	Layout (gray header)
//	-
//	Divider / Flex / ...
//
// Every selectable control has its own tab content (one file per control).
// msgHost is app-level Message/Notification portal (Ant App pattern). Must stay
// mounted under the window root — not only inside a tab panel — or toasts never show
// when the Message tab content is inactive / unmounted.
func buildCatalogPanels(face text.Face, theme *core.Theme, status *string, buttons *[]*kit.Button, tickers *[]interface{ AttachTicker(*core.Tree) }, msgHost *kit.MessageHost) (
	items []kit.MenuItem, contents map[string]core.Node, modal *kit.Modal,
) {
	c := &catalogCtx{
		face:     face,
		theme:    theme,
		status:   status,
		buttons:  buttons,
		tickers:  tickers,
		msgHost:  msgHost,
		contents: make(map[string]core.Node),
	}

	// ─── General ───────────────────────────────────────────────
	c.cat("General")
	c.registerButton()
	c.registerFloatButton()
	c.registerIcon()
	c.registerTypography()

	// ─── Layout ────────────────────────────────────────────────
	c.cat("Layout")
	c.registerDivider()
	c.registerFlex()
	c.registerGrid()
	c.registerLayout()
	c.registerSpace()
	c.registerSplitter()

	// ─── Navigation ────────────────────────────────────────────
	c.cat("Navigation")
	c.registerAnchor()
	c.registerBreadcrumb()
	c.registerDropdown()
	c.registerMenu()
	c.registerPagination()
	c.registerSteps()
	c.registerTabs()

	// ─── Data Entry ────────────────────────────────────────────
	c.cat("Data Entry")
	c.registerAutoComplete()
	c.registerCascader()
	c.registerCheckbox()
	c.registerColorPicker()
	c.registerDatePicker()
	c.registerForm()
	c.registerInput()
	c.registerInputNumber()
	c.registerMentions()
	c.registerRadio()
	c.registerRate()
	c.registerSelect()
	c.registerSlider()
	c.registerSwitch()
	c.registerTextArea()
	c.registerTimePicker()
	c.registerTransfer()
	c.registerTreeSelect()
	c.registerUpload()

	// ─── Data Display ──────────────────────────────────────────
	c.cat("Data Display")
	c.registerAvatar()
	c.registerBadge()
	c.registerCalendar()
	c.registerCard()
	c.registerCarousel()
	c.registerCollapse()
	c.registerDescriptions()
	c.registerEmpty()
	c.registerImage()
	c.registerList()
	c.registerPopover()
	c.registerQRCode()
	c.registerSegmented()
	c.registerStatistic()
	c.registerTable()
	c.registerTag()
	c.registerTimeline()
	c.registerTooltip()
	c.registerTour()
	c.registerTree()

	// ─── Feedback ──────────────────────────────────────────────
	c.cat("Feedback")
	c.registerAlert()
	c.registerDrawer()
	c.registerMessage()
	c.registerModal()
	c.registerNotification()
	c.registerPopconfirm()
	c.registerProgress()
	c.registerResult()
	c.registerSkeleton()
	c.registerSpin()
	c.registerWatermark()

	// ─── Other ─────────────────────────────────────────────────
	c.cat("Other")
	c.registerAffix()
	c.registerApp()
	c.registerConfigProvider()
	c.registerScroll()
	c.registerScrollbar()

	return c.items, c.contents, c.modal
}
