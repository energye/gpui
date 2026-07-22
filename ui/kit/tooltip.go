package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Tooltip shows title near a trigger on hover.
// Call Sync() each frame (or after pointer dispatch) to open/close from hover.
type Tooltip struct {
	Root     *primitive.Flex
	shell    *primitive.Pressable
	Popup    *primitive.AnchoredPopup
	Bubble   *primitive.Decorated
	Label    *primitive.Text
	Title    string
	Face     text.Face
	Theme    *core.Theme
	Viewport core.Size
}

// NewTooltip wraps trigger with a hover tooltip.
func NewTooltip(trigger core.Node, title string) *Tooltip {
	tt := &Tooltip{Title: title}
	tt.rebuild(trigger)
	return tt
}

// Node returns the composition root.
func (tt *Tooltip) Node() core.Node {
	if tt.Root == nil {
		tt.rebuild(nil)
	}
	return tt.Root
}

// Sync opens/closes based on shell hover and updates anchor.
func (tt *Tooltip) Sync() {
	if tt.shell == nil || tt.Popup == nil {
		return
	}
	open := tt.shell.State.Hovered
	tt.Popup.UpdateAnchorFromNode(tt.shell)
	if tt.Viewport.Width > 0 {
		tt.Popup.Viewport = tt.Viewport
	}
	tt.Popup.SetOpen(open)
}

func (tt *Tooltip) theme() *core.Theme {
	if tt.Theme != nil {
		return tt.Theme
	}
	return DefaultTheme()
}

func (tt *Tooltip) rebuild(trigger core.Node) {
	th := tt.theme()
	tt.Label = primitive.NewText(tt.Title)
	tt.Label.FontSize = th.SizeOr(core.TokenFontSize, 14)
	tt.Label.Face = tt.Face
	tt.Label.Color = th.Color(core.TokenColorTextInverse)

	tt.Bubble = primitive.NewDecorated(tt.Label)
	tt.Bubble.Padding = primitive.Symmetric(8, 6) // Ant tooltip padding ≈ 6×8
	tt.Bubble.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	tt.Bubble.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.85}

	tt.Popup = primitive.NewAnchoredPopup(tt.Bubble)
	tt.Popup.Placement = primitive.PlaceTop
	tt.Popup.Gap = 6
	tt.Popup.Portal.ID = "tooltip"

	if trigger == nil {
		trigger = primitive.NewText("?")
	}
	tt.shell = primitive.NewPressable(trigger)
	tt.shell.Focusable = false

	tt.Root = primitive.Column(tt.shell, tt.Popup)
	tt.Root.CrossAlign = core.CrossStart
}

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
