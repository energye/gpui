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
