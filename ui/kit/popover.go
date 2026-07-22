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

// SetOpen forces open state.
func (p *Popover) SetOpen(open bool) {
	p.Open = open
	if p.Popup != nil {
		p.Popup.UpdateAnchorFromNode(p.shell)
		p.Popup.SetOpen(open)
	}
}

// Sync updates anchor while open.
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
	if p.Theme != nil {
		return p.Theme
	}
	return DefaultTheme()
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
	p.Popup.Portal.ID = "popover"

	if trigger == nil {
		trigger = primitive.NewText("Popover")
	}
	p.shell = primitive.NewPressable(trigger)
	p.shell.Focusable = true
	p.shell.Click = func() {
		p.Open = !p.Open
		p.Popup.UpdateAnchorFromNode(p.shell)
		p.Popup.SetOpen(p.Open)
	}

	p.Root = primitive.Column(p.shell, p.Popup)
	p.Root.CrossAlign = core.CrossStart
}
