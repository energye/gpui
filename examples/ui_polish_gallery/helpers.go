//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// catalogCtx is shared state for per-component gallery demos.
// Each control lives in its own file (button.go, flex.go, …) and registers
// through this context so demos stay isolated and unmixed.
type catalogCtx struct {
	face     text.Face
	theme    *core.Theme
	status   *string
	buttons  *[]*kit.Button
	tickers  *[]interface{ AttachTicker(*core.Tree) }
	msgHost  *kit.MessageHost
	items    []kit.MenuItem
	contents map[string]core.Node
	modal    *kit.Modal
}

func (c *catalogCtx) trackBtn(b *kit.Button) *kit.Button {
	b.SetFace(c.face)
	*c.buttons = append(*c.buttons, b)
	return b
}

func (c *catalogCtx) trackTicker(t interface{ AttachTicker(*core.Tree) }) {
	*c.tickers = append(*c.tickers, t)
}

// add registers a simple tab with panel(title + kids).
func (c *catalogCtx) add(key, label, title string, kids ...core.Node) {
	c.items = append(c.items, ctlTab(key, label))
	c.contents[key] = panel(c.face, title, kids...)
}

// addPage registers a tab whose content is a full demoPage (or any node).
func (c *catalogCtx) addPage(key, label string, page core.Node) {
	c.items = append(c.items, ctlTab(key, label))
	c.contents[key] = page
}

func (c *catalogCtx) cat(name string) {
	c.items = append(c.items, catHeader(name), catDivider())
}

// demoDesc is multi-line secondary copy for gallery pages.
// kit.NewText defaults to single-line (EllipsisRows→1); long 说明 would clip/overflow.
// Use Paragraph + wrap budget so descriptions stay readable under CrossStretch parents.
func demoDesc(face text.Face, s string) core.Node {
	if s == "" {
		return nil
	}
	d := kit.NewParagraph(s)
	d.SetFace(face)
	// Readable body copy: Token secondary A=0.45 looks soft/uneven on CJK strokes.
	// Use near-body text color (antd description ~0.65–0.88), not pure secondary.
	d.SetStyle(kit.Style{Text: render.RGBA{R: 0, G: 0, B: 0, A: 0.65}})
	d.SetEllipsisRows(16) // soft-wrap up to 16 lines; not ellipsis unless SetEllipsis
	return d.Node()
}

// panel wraps a single control demo for its own tab content.
func panel(face text.Face, title string, kids ...core.Node) core.Node {
	col := primitive.Column(demoDesc(face, title))
	col.Gap = 12
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch
	col.Padding = primitive.All(12)
	for _, k := range kids {
		if k != nil {
			col.AddChild(k)
		}
	}
	return col
}

// demoPage is an Ant Design–style component docs page: title + optional description
// + stacked demo sections (scrolls inside Tabs body ScrollViewport).
func demoPage(face text.Face, title, desc string, sections ...core.Node) core.Node {
	lab := kit.NewText(title)
	lab.SetFace(face)
	lab.SetFontSize(20)
	col := primitive.Column(lab.Node())
	col.Gap = 24
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch
	col.Padding = primitive.All(16)
	if n := demoDesc(face, desc); n != nil {
		col.AddChild(n)
	}
	for _, s := range sections {
		if s != nil {
			col.AddChild(s)
		}
	}
	return col
}

// demoSection is one bordered playground block (title + optional desc + content).
func demoSection(face text.Face, theme *core.Theme, title, desc string, body core.Node) core.Node {
	titleT := kit.NewText(title)
	titleT.SetFace(face)
	titleT.SetFontSize(16)
	inner := primitive.Column(titleT.Node())
	inner.Gap = 12
	inner.MainAlign = core.MainStart
	inner.CrossAlign = core.CrossStretch
	if n := demoDesc(face, desc); n != nil {
		inner.AddChild(n)
	}
	if body != nil {
		inner.AddChild(body)
	}
	card := primitive.NewDecorated(inner)
	card.Padding = primitive.All(16)
	card.Radius = 8
	card.BorderWidth = 1
	if theme != nil {
		card.Background = theme.Color(core.TokenColorBgContainer)
		card.BorderColor = theme.Color(core.TokenColorBorder)
	}
	card.Hit = core.HitDefer
	return card
}

// spaceWrap is Ant Space with wrap for demo button rows.
func spaceWrap(gap float64, kids ...core.Node) core.Node {
	sp := kit.NewSpace(kids...)
	sp.SetSize(gap)
	sp.SetWrap(true)
	return sp.Node()
}

func sec(face text.Face, s string) core.Node {
	return demoDesc(face, s)
}

func catHeader(label string) kit.MenuItem {
	return kit.MenuItem{Key: "cat:" + label, Label: label, Disabled: true}
}

func catDivider() kit.MenuItem {
	return kit.MenuItem{Key: "div", Label: "-", Divider: true}
}

func ctlTab(key, label string) kit.MenuItem {
	return kit.MenuItem{Key: key, Label: label}
}

// buildCatalogPanels builds left Tabs rail:
//
//	General (gray header, not clickable)
//	-
//	Button / FloatButton / Icon / Typography (each has own content)
//	Layout (gray header)
//	-
//	Divider / Flex / ...
//
// Every selectable control has its own tab content.
// msgHost is app-level Message/Notification portal (Ant App pattern). Must stay
// mounted under the window root — not only inside a tab panel — or toasts never show
// when the Message tab content is inactive / unmounted.
