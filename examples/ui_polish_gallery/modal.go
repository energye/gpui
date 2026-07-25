//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// registerModal rebuilds the Modal gallery page to cover docs/antd/modal.md §6.8 P0
// official demos: basic / async / footer / mask / loading / footer-render / hooks / locale.
func (c *catalogCtx) registerModal() {
	basic := c.newGalleryModal("Basic Modal", simpleModalContent("Some contents...", 3))
	basic.OnOk = func() { basic.SetOpen(false) }

	asyncM := c.newGalleryModal("Title", kit.NewText("Content of the modal").Node())
	asyncM.OnOk = func() {
		asyncM.SetContent(kit.NewText("The modal will be closed after two seconds").Node())
		asyncM.SetConfirmLoading(true)
	}

	footerCustom := c.newGalleryModal("Title", simpleModalContent("Some contents...", 5))
	ret := c.trackBtn(kit.NewButton("Return"))
	ret.SetOnClick(func() { footerCustom.SetOpen(false) })
	sub := c.trackBtn(kit.NewButton("Submit"))
	sub.SetType(kit.ButtonPrimary)
	sub.SetOnClick(func() {
		sub.SetLoading(true)
		*c.status = "modal footer submit loading"
	})
	search := c.trackBtn(kit.NewButton("Search on Google"))
	search.SetType(kit.ButtonPrimary)
	footRow := primitive.Row(ret.Node(), sub.Node(), search.Node())
	footRow.Gap = 8
	footRow.MainAlign = core.MainEnd
	footerCustom.SetFooter(footRow)

	maskHost := kit.NewModalHost()
	maskHost.Face = c.face
	maskHost.Theme = c.theme

	loadingM := c.newGalleryModal("Loading Modal", simpleModalContent("Some contents...", 3))
	loadingM.SetDestroyOnHidden(true)
	reload := c.trackBtn(kit.NewButton("Reload"))
	reload.SetType(kit.ButtonPrimary)
	reload.SetOnClick(func() { loadingM.SetLoading(true) })
	loadingM.SetFooter(reload.Node())
	c.trackTicker(loadingM)

	footerRender := c.newGalleryModal("Title", simpleModalContent("Some contents...", 5))
	footerRender.SetFooterRender(func(ok, cancel core.Node) core.Node {
		custom := c.trackBtn(kit.NewButton("Custom Button"))
		row := primitive.Row(custom.Node(), cancel, ok)
		row.Gap = 8
		row.MainAlign = core.MainEnd
		return row
	})
	footerRender.OnOk = func() { footerRender.SetOpen(false) }

	hooksHost := kit.NewModalHost()
	hooksHost.Face = c.face
	hooksHost.Theme = c.theme

	locale := c.newGalleryModal("Modal", simpleModalContent("Bla bla ...", 3))
	locale.SetOkText("确认")
	locale.SetCancelText("取消")
	locale.OnOk = func() { locale.SetOpen(false) }

	localeHost := kit.NewModalHost()
	localeHost.Face = c.face
	localeHost.Theme = c.theme

	// Keep one modal on c.modal for main.go Viewport/Sync compatibility.
	c.modal = basic

	modals := []core.Node{
		basic.Node(),
		asyncM.Node(),
		footerCustom.Node(),
		loadingM.Node(),
		footerRender.Node(),
		locale.Node(),
		// hosts + no-mask modal are mounted inside their demo rows
	}

	page := demoPage(c.face, "Modal", "Feedback / Modal",
		demoSection(c.face, c.theme, "Basic", "Controlled open · title · default footer OK/Cancel.",
			modalOpenRow(c, basic, "Open Modal")),
		demoSection(c.face, c.theme, "Asynchronously close", "confirmLoading on OK; finish manually.",
			modalAsyncRow(c, asyncM)),
		demoSection(c.face, c.theme, "Customized footer", "SetFooter with Return / Submit / Search.",
			modalOpenRow(c, footerCustom, "Open Modal with customized footer")),
		demoSection(c.face, c.theme, "Mask", "ModalHost.Confirm with default dimmed mask; plus no-mask modal.",
			modalMaskRow(c, maskHost)),
		demoSection(c.face, c.theme, "Loading", "loading skeleton body; Reload re-enters loading.",
			modalLoadingRow(c, loadingM)),
		demoSection(c.face, c.theme, "Custom footer render function", "SetFooterRender(ok, cancel) composes Custom + Cancel + OK.",
			modalOpenRow(c, footerRender, "Open Modal")),
		demoSection(c.face, c.theme, "Hooks modal", "ModalHost ≡ useModal contextHolder (confirm/info/…).",
			modalHooksRow(c, hooksHost)),
		demoSection(c.face, c.theme, "Internationalization", "okText/cancelText 确认/取消.",
			modalLocaleRow(c, locale, localeHost)),
	)
	if col, ok := page.(*primitive.Flex); ok {
		for _, n := range modals {
			col.AddChild(n)
		}
	}
	c.addPage("modal", "Modal", page)
}

func (c *catalogCtx) newGalleryModal(title string, body core.Node) *kit.Modal {
	m := kit.NewModal(title)
	m.SetFace(c.face)
	m.SetTheme(c.theme)
	m.SetContent(body)
	m.OnCancel = func() {
		*c.status = title + " cancel"
	}
	m.OnOpenChange = func(open bool) {
		if open {
			*c.status = title + " open"
		}
	}
	return m
}

func modalOpenRow(c *catalogCtx, m *kit.Modal, label string) core.Node {
	open := c.trackBtn(kit.NewButton(label))
	open.SetType(kit.ButtonPrimary)
	open.SetOnClick(func() { m.SetOpen(true) })
	return spaceWrap(8, open.Node())
}

func modalAsyncRow(c *catalogCtx, m *kit.Modal) core.Node {
	open := c.trackBtn(kit.NewButton("Open Modal with async logic"))
	open.SetType(kit.ButtonPrimary)
	open.SetOnClick(func() {
		m.SetConfirmLoading(false)
		m.SetContent(kit.NewText("Content of the modal").Node())
		m.SetOpen(true)
	})
	done := c.trackBtn(kit.NewButton("Finish async"))
	done.SetOnClick(func() {
		m.SetConfirmLoading(false)
		m.SetOpen(false)
		*c.status = "modal async closed"
	})
	return spaceWrap(8, open.Node(), done.Node())
}

func modalMaskRow(c *catalogCtx, host *kit.ModalHost) core.Node {
	dimmed := c.trackBtn(kit.NewButton("Dimmed mask"))
	dimmed.SetOnClick(func() {
		host.Confirm(kit.ModalConfirmConfig{Title: "Title", Content: "Some contents..."})
		*c.status = "modal mask dimmed"
	})
	none := c.trackBtn(kit.NewButton("No mask"))
	noMask := c.newGalleryModal("Title", kit.NewText("Some contents...").Node())
	noMask.SetMask(false)
	noMask.OnOk = func() { noMask.SetOpen(false) }
	none.SetOnClick(func() { noMask.SetOpen(true) })
	blur := c.trackBtn(kit.NewButton("blur (P1 approx)"))
	blur.SetOnClick(func() {
		host.Confirm(kit.ModalConfirmConfig{Title: "Title", Content: "Some contents..."})
		*c.status = "modal mask blur≈dimmed (P1)"
	})
	row := primitive.Row(blur.Node(), dimmed.Node(), none.Node(), noMask.Node(), host.Node())
	row.Gap = 8
	return row
}

func modalLoadingRow(c *catalogCtx, m *kit.Modal) core.Node {
	open := c.trackBtn(kit.NewButton("Open Modal"))
	open.SetType(kit.ButtonPrimary)
	open.SetOnClick(func() {
		m.SetLoading(true)
		m.SetOpen(true)
	})
	done := c.trackBtn(kit.NewButton("Finish Loading"))
	done.SetOnClick(func() { m.SetLoading(false) })
	return spaceWrap(8, open.Node(), done.Node())
}

func modalHooksRow(c *catalogCtx, host *kit.ModalHost) core.Node {
	cfg := kit.ModalConfirmConfig{Title: "Use Hook!", Content: "Reachable: Light!"}
	confirm := c.trackBtn(kit.NewButton("Confirm"))
	confirm.SetOnClick(func() {
		host.Confirm(cfg)
		*c.status = "hooks confirm"
	})
	warn := c.trackBtn(kit.NewButton("Warning"))
	warn.SetOnClick(func() {
		host.Warning(cfg)
		*c.status = "hooks warning"
	})
	info := c.trackBtn(kit.NewButton("Info"))
	info.SetOnClick(func() {
		host.Info(cfg)
		*c.status = "hooks info"
	})
	err := c.trackBtn(kit.NewButton("Error"))
	err.SetOnClick(func() {
		host.Error(cfg)
		*c.status = "hooks error"
	})
	row := primitive.Row(confirm.Node(), warn.Node(), info.Node(), err.Node(), host.Node())
	row.Gap = 8
	return row
}

func modalLocaleRow(c *catalogCtx, m *kit.Modal, host *kit.ModalHost) core.Node {
	open := c.trackBtn(kit.NewButton("Modal"))
	open.SetType(kit.ButtonPrimary)
	open.SetOnClick(func() { m.SetOpen(true) })
	confirm := c.trackBtn(kit.NewButton("Confirm"))
	confirm.SetOnClick(func() {
		host.Confirm(kit.ModalConfirmConfig{
			Title:      "Confirm",
			Content:    "Bla bla ...",
			OkText:     "确认",
			CancelText: "取消",
		})
		*c.status = "locale confirm"
	})
	return spaceWrap(8, open.Node(), confirm.Node(), host.Node())
}

func simpleModalContent(value string, count int) core.Node {
	col := primitive.Column()
	col.Gap = 8
	for i := 0; i < count; i++ {
		col.AddChild(kit.NewText(value).Node())
	}
	return col
}
