//go:build linux && !nogpu

package main

import (
	"fmt"
	"strings"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTypography() {
	// Typography — docs/antd/typography.md §6.8 P0
	// https://ant.design/components/typography
	secTypoBasic := demoSection(c.face, c.theme, "Basic",
		"Title + Paragraph (antd basic.tsx).",
		primitive.Column(
			func() core.Node {
				h := kit.NewTitle("Introduction", 2)
				h.SetFace(c.face)
				return h.Node()
			}(),
			func() core.Node {
				p := kit.NewParagraph("Ant Design, a design language for background applications, is refined by Ant UED Team.")
				p.SetFace(c.face)
				p.SetMaxWidth(520)
				return p.Node()
			}(),
		))

	titleKids := make([]core.Node, 0, 5)
	for lv := 1; lv <= 5; lv++ {
		h := kit.NewTitle(fmt.Sprintf("h%d. Ant Design", lv), lv)
		h.SetFace(c.face)
		titleKids = append(titleKids, h.Node())
	}
	secTypoTitle := demoSection(c.face, c.theme, "Title",
		"level 1..5 → 38/30/24/20/16 (antd title.tsx).",
		primitive.Column(titleKids...))

	mkText := func(label string, cfg func(*kit.Typography)) core.Node {
		x := kit.NewText(label)
		x.SetFace(c.face)
		if cfg != nil {
			cfg(x)
		}
		return x.Node()
	}
	secTypoText := demoSection(c.face, c.theme, "Text & Link",
		"type / mark / code / delete / underline / strong / Link (antd text.tsx).",
		spaceWrap(12,
			mkText("default", nil),
			mkText("secondary", func(t *kit.Typography) { t.SetType(kit.TypographyTypeSecondary) }),
			mkText("success", func(t *kit.Typography) { t.SetType(kit.TypographyTypeSuccess) }),
			mkText("warning", func(t *kit.Typography) { t.SetType(kit.TypographyTypeWarning) }),
			mkText("danger", func(t *kit.Typography) { t.SetType(kit.TypographyTypeDanger) }),
			mkText("mark", func(t *kit.Typography) { t.SetMark(true) }),
			mkText("code", func(t *kit.Typography) { t.SetCode(true) }),
			mkText("delete", func(t *kit.Typography) { t.SetDelete(true) }),
			mkText("underline", func(t *kit.Typography) { t.SetUnderline(true) }),
			mkText("strong", func(t *kit.Typography) { t.SetStrong(true) }),
			func() core.Node {
				l := kit.NewLink("Ant Design")
				l.SetFace(c.face)
				return l.Node()
			}(),
		))

	editTx := kit.NewText("This is an editable text.")
	editTx.SetFace(c.face)
	editTx.SetEditable(true)
	secTypoEdit := demoSection(c.face, c.theme, "Editable",
		"Edit icon → Enter commit / Esc cancel (antd editable.tsx).",
		editTx.Node())

	copyTx := kit.NewText("This is a copyable text.")
	copyTx.SetFace(c.face)
	copyTx.SetCopyable(true)
	secTypoCopy := demoSection(c.face, c.theme, "Copyable",
		"Copy icon → Tree clipboard + onCopy (antd copyable.tsx).",
		copyTx.Node())

	ellip := kit.NewParagraph(strings.Repeat("Ant Design, a design language for background applications, is refined by Ant UED Team. ", 6))
	ellip.SetFace(c.face)
	ellip.SetEllipsis(true)
	ellip.SetEllipsisRows(3)
	ellip.SetMaxWidth(420)
	ellip.SetExpandable(true)
	ellip.SetCollapsible(true)
	secTypoEllipsis := demoSection(c.face, c.theme, "Ellipsis",
		"rows=3 + expandable/collapsible (antd ellipsis.tsx).",
		ellip.Node())

	ellipCtrl := kit.NewParagraph(strings.Repeat("Controlled expand / collapse. ", 12))
	ellipCtrl.SetFace(c.face)
	ellipCtrl.SetEllipsis(true)
	ellipCtrl.SetEllipsisRows(2)
	ellipCtrl.SetMaxWidth(420)
	ellipCtrl.SetExpandable(true)
	ellipCtrl.SetCollapsible(true)
	ellipCtrl.SetExpanded(false)
	secTypoEllipsisCtrl := demoSection(c.face, c.theme, "Controlled expand",
		"SetExpanded controlled (antd ellipsis-controlled.tsx).",
		ellipCtrl.Node())

	ellipMid := kit.NewText("https://ant.design/components/typography-cn#components-typography-demo-ellipsis-middle")
	ellipMid.SetFace(c.face)
	ellipMid.SetEllipsis(true)
	ellipMid.SetEllipsisMiddle(true)
	ellipMid.SetMaxWidth(280)
	secTypoMiddle := demoSection(c.face, c.theme, "Ellipsis middle",
		"start…end truncation (antd ellipsis-middle.tsx).",
		ellipMid.Node())

	disTx := kit.NewText("disabled text")
	disTx.SetFace(c.face)
	disTx.SetDisabled(true)
	disTx.SetCopyable(true)
	secTypoDisabled := demoSection(c.face, c.theme, "Disabled",
		"disabled color; copy/edit hidden.",
		disTx.Node())

	c.items = append(c.items, ctlTab("typography", "Typography"))
	c.contents["typography"] = demoPage(c.face, "Typography",
		"Text / Title / Paragraph / Link. P0: type, disabled, copyable, editable, ellipsis(+middle/controlled), decorations, Token 14 & title ladder (docs/antd/typography.md §6).",
		secTypoBasic, secTypoTitle, secTypoText, secTypoEdit, secTypoCopy,
		secTypoEllipsis, secTypoEllipsisCtrl, secTypoMiddle, secTypoDisabled,
	)
}
