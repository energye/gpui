package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Popover shows content near a trigger on click.
type Popover struct {
	Root     *primitive.Flex
	shell    *primitive.Pressable
	Popup    *primitive.AnchoredPopup
	Panel    *primitive.Decorated
	Content  core.Node
	Theme    *core.Theme
	Viewport core.Size
	Open     bool
}

// NewPopover wraps trigger with click-to-open content.
func NewPopover(trigger core.Node, content core.Node) *Popover {
	p := &Popover{Content: content}
	p.rebuild(trigger)
	return p
}

// Node returns the root.
func (p *Popover) Node() core.Node {
	if p.Root == nil {
		p.rebuild(nil)
	}
	return p.Root
}

// SetOpen forces open state (same measure + Viewport path as click).
func (p *Popover) SetOpen(open bool) {
	p.Open = open
	if p.Popup == nil {
		return
	}
	if open {
		// Measure panel before open so AnchoredPopup reposition has size.
		if p.Panel != nil {
			_ = p.Panel.Layout(core.Loose(400, 400))
		}
		if p.shell != nil {
			p.Popup.UpdateAnchorFromNode(p.shell)
		}
		if p.Viewport.Width > 0 {
			p.Popup.Viewport = p.Viewport
		}
	}
	p.Popup.SetOpen(open)
}

// Sync updates anchor while open.
// Deprecated: prefer Tree.Layout + AnchoredPopup.RefreshOpenGeometry (automatic).
// Kept for one-shot forced reposition after external layout changes.
func (p *Popover) Sync() {
	if p.Popup == nil || p.shell == nil {
		return
	}
	if p.Viewport.Width > 0 {
		p.Popup.Viewport = p.Viewport
	}
	if p.Open {
		p.Popup.UpdateAnchorFromNode(p.shell)
		p.Popup.SetOpen(true)
	}
}

func (p *Popover) theme() *core.Theme {
	var n core.Node
	if p.Root != nil {
		n = p.Root
	}
	return themeOf(p.Theme, n)
}

func (p *Popover) rebuild(trigger core.Node) {
	th := p.theme()
	if p.Content == nil {
		p.Content = primitive.NewText("…")
	}
	p.Panel = primitive.NewDecorated(p.Content)
	p.Panel.Padding = primitive.All(12)
	p.Panel.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	p.Panel.Background = th.Color(core.TokenColorBgContainer)
	p.Panel.BorderWidth = 1
	p.Panel.BorderColor = th.Color(core.TokenColorBorder)

	p.Popup = primitive.NewAnchoredPopup(p.Panel)
	p.Popup.Placement = primitive.PlaceBottomStart
	p.Popup.Gap = 8
	p.Popup.Portal.ID = "" // auto id per instance (avoid clobbering other popovers)
	p.Popup.DismissOnOutside = true
	p.Popup.OnDismiss = func() {
		p.Open = false
	}

	if trigger == nil {
		trigger = primitive.NewText("Popover")
	}
	p.shell = primitive.NewPressable(trigger)
	p.shell.Focusable = true
	p.shell.Click = func() {
		p.Open = !p.Open
		// Measure panel content before open so AnchoredPopup reposition has size.
		if p.Panel != nil {
			_ = p.Panel.Layout(core.Loose(400, 400))
		}
		p.Popup.UpdateAnchorFromNode(p.shell)
		if p.Viewport.Width > 0 {
			p.Popup.Viewport = p.Viewport
		}
		p.Popup.SetOpen(p.Open)
		if t := p.shell.Tree(); t != nil {
			t.MarkDirty()
		}
	}

	p.Root = primitive.Column(p.shell, p.Popup)
	p.Root.CrossAlign = core.CrossStart
}
