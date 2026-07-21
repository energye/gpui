package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Button is a product-level control composed from Pressable + Decorated + Flex + Text/Icon.
//
//	Pressable
//	  └─ Decorated
//	       └─ Flex(Row)
//	            Icon? · Text(label)
//
// Call SyncState() once per frame (or after pointer events) so hover/press
// chrome tracks PressableState.
type Button struct {
	Root      *primitive.Pressable
	decorated *primitive.Decorated
	row       *primitive.Flex
	label     *primitive.Text
	icon      *primitive.Icon

	Type     ButtonType
	Size     ButtonSize
	Danger   bool
	Disabled bool
	Loading  bool
	Block    bool
	Label    string
	IconName string
	OnClick  func()
	Face     text.Face
	Theme    *core.Theme

	bgNormal, bgHover, bgPressed render.RGBA
	bdNormal                     render.RGBA
	borderW                      float64
	lastHovered, lastPressed     bool
}

// NewButton creates a Button with the given label.
func NewButton(label string) *Button {
	b := &Button{Label: label, Type: ButtonDefault, Size: ButtonMiddle}
	b.rebuild()
	return b
}

// Node returns the root core.Node for tree attachment.
func (b *Button) Node() core.Node {
	if b.Root == nil {
		b.rebuild()
	}
	return b.Root
}

// ChromeNode returns the Decorated chrome (for visual tests / composition).
func (b *Button) ChromeNode() core.Node {
	if b.decorated == nil {
		b.rebuild()
	}
	return b.decorated
}

// SetLabel updates the button label.
func (b *Button) SetLabel(s string) {
	b.Label = s
	if b.label != nil {
		b.label.SetValue(s)
		return
	}
	b.rebuild()
}

// SetType updates visual type and recolors.
func (b *Button) SetType(t ButtonType) {
	b.Type = t
	b.applyChrome()
}

// SetSize updates control size and rebuilds metrics.
func (b *Button) SetSize(s ButtonSize) {
	b.Size = s
	b.rebuild()
}

// SetDisabled toggles disabled.
func (b *Button) SetDisabled(d bool) {
	b.Disabled = d
	if b.Root != nil {
		b.Root.SetDisabled(d || b.Loading)
	}
	b.applyChrome()
}

// SetLoading toggles loading (disables press).
func (b *Button) SetLoading(v bool) {
	b.Loading = v
	if b.Root != nil {
		b.Root.SetDisabled(b.Disabled || b.Loading)
	}
	b.applyChrome()
}

// SetDanger toggles danger styling.
func (b *Button) SetDanger(v bool) {
	b.Danger = v
	b.applyChrome()
}

// SetIcon sets an optional leading icon name (empty clears).
func (b *Button) SetIcon(name string) {
	b.IconName = name
	b.rebuild()
}

// SetOnClick sets the click handler.
func (b *Button) SetOnClick(fn func()) {
	b.OnClick = fn
	if b.Root != nil {
		b.Root.Click = b.fireClick
	}
}

// SetFace sets the label font face.
func (b *Button) SetFace(face text.Face) {
	b.Face = face
	if b.label != nil {
		b.label.Face = face
	}
}

// SetFixedSize forces outer chrome to a fixed size (0 clears width/height force).
// Used by visual scenarios for stable 120×40 chrome blocks.
func (b *Button) SetFixedSize(w, h float64) {
	if b.decorated == nil {
		b.rebuild()
	}
	b.decorated.Width = w
	b.decorated.Height = h
	if h > 0 {
		b.decorated.MinHeight = h
	}
	if w > 0 {
		b.decorated.MinWidth = w
	}
	b.decorated.MarkNeedsLayout()
}

// SyncState copies Pressable hover/press into Decorated background.
// Call after DispatchPointer / each frame.
func (b *Button) SyncState() {
	if b.Root == nil || b.decorated == nil {
		return
	}
	h, p := b.Root.State.Hovered, b.Root.State.Pressed
	if h == b.lastHovered && p == b.lastPressed {
		return
	}
	b.lastHovered, b.lastPressed = h, p
	bg := b.bgNormal
	switch {
	case b.Disabled || b.Loading:
		bg = b.bgNormal
	case p:
		bg = b.bgPressed
	case h:
		bg = b.bgHover
	}
	b.decorated.Background = bg
	b.decorated.MarkNeedsPaint()
}

func (b *Button) fireClick() {
	if b.Disabled || b.Loading {
		return
	}
	if b.OnClick != nil {
		b.OnClick()
	}
}

func (b *Button) theme() *core.Theme {
	if b.Theme != nil {
		return b.Theme
	}
	return DefaultTheme()
}

func (b *Button) rebuild() {
	th := b.theme()
	padH, padV, height, fontSize, radius, gap := b.metrics(th)

	b.label = primitive.NewText(b.Label)
	b.label.FontSize = fontSize
	b.label.Face = b.Face

	b.row = primitive.Row()
	b.row.Gap = gap
	b.row.CrossAlign = core.CrossCenter
	b.row.MainAlign = core.MainCenter

	if b.IconName != "" {
		b.icon = primitive.NewIcon(b.IconName)
		b.icon.Size = fontSize + 2
		b.row.AddChild(b.icon)
	} else {
		b.icon = nil
	}
	b.row.AddChild(b.label)

	b.decorated = primitive.NewDecorated(b.row)
	b.decorated.Padding = primitive.Symmetric(padH, padV)
	b.decorated.Radius = radius
	b.decorated.MinHeight = height
	if b.Block {
		// Expand horizontally when parent gives a max width.
		b.decorated.MinWidth = th.SizeOr(core.TokenControlHeight, 32) * 4
	}

	b.Root = primitive.NewPressable(b.decorated)
	b.Root.Focusable = true
	b.Root.FocusRingRadius = radius
	b.Root.Click = b.fireClick
	b.Root.SetDisabled(b.Disabled || b.Loading)
	b.Root.Base().Role = "button"
	b.Root.Base().Label = b.Label

	b.lastHovered, b.lastPressed = false, false
	b.applyChrome()
}

func (b *Button) metrics(th *core.Theme) (padH, padV, height, fontSize, radius, gap float64) {
	fontSize = th.SizeOr(core.TokenFontSize, 14)
	radius = th.SizeOr(core.TokenBorderRadius, 6)
	gap = th.SizeOr(core.TokenMarginXS, 4)
	// Horizontal padding tracks Ant middle (~15) via padding token − 1.
	padBase := th.SizeOr(core.TokenPadding, 16)
	switch b.Size {
	case ButtonSmall:
		height = th.SizeOr(core.TokenControlHeightSM, 24)
		fontSize = th.SizeOr(core.TokenFontSizeSM, 12)
		padH = th.SizeOr(core.TokenPaddingSM, 8) - 1 // ~7
		padV = th.SizeOr(core.TokenPaddingXS, 4) / 2 // ~2
		radius = th.SizeOr(core.TokenBorderRadiusSM, 4)
	case ButtonLarge:
		height = th.SizeOr(core.TokenControlHeightLG, 40)
		fontSize = th.SizeOr(core.TokenFontSizeLG, 16)
		padH = padBase - 1                           // ~15
		padV = th.SizeOr(core.TokenPaddingSM, 8) - 2 // ~6
		radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	default:
		height = th.SizeOr(core.TokenControlHeight, 32)
		padH = padBase - 1                       // ~15
		padV = th.SizeOr(core.TokenPaddingXS, 4) // ~4
	}
	return
}

func (b *Button) applyChrome() {
	if b.decorated == nil || b.label == nil {
		return
	}
	th := b.theme()
	_, _, height, _, radius, _ := b.metrics(th)
	b.decorated.Radius = radius
	b.decorated.MinHeight = height
	if b.Root != nil {
		b.Root.FocusRingRadius = radius
	}

	primary := th.Color(core.TokenColorPrimary)
	primaryHover := th.Color(core.TokenColorPrimaryHover)
	primaryActive := th.Color(core.TokenColorPrimaryActive)
	textCol := th.Color(core.TokenColorText)
	textInv := th.Color(core.TokenColorTextInverse)
	border := th.Color(core.TokenColorBorder)
	bg := th.Color(core.TokenColorBgContainer)
	disabledBg := th.Color(core.TokenColorDisabledBg)
	disabledText := th.Color(core.TokenColorDisabledText)
	errorC := th.Color(core.TokenColorError)
	// Stronger hover than raw fillSecondary (0.06) — layout bg reads as soft gray.
	hoverFill := th.Color(core.TokenColorBgLayout)
	if hoverFill.A < 0.5 {
		hoverFill = render.Hex("#F5F5F5")
	}
	pressFill := th.Color(core.TokenColorBorderSecondary)
	if pressFill.A < 0.5 {
		pressFill = render.Hex("#F0F0F0")
	}

	var bgN, bgH, bgP, fg, bd render.RGBA
	bw := th.SizeOr(core.TokenLineWidth, 1)

	switch b.Type {
	case ButtonPrimary:
		bgN, bgH, bgP = primary, primaryHover, primaryActive
		fg = textInv
		if b.Danger {
			bgN, bgH, bgP = errorC, render.Hex("#FF7875"), render.Hex("#D9363E")
		}
		bd = bgN
		bw = 0
	case ButtonDashed:
		bgN, bgH, bgP = bg, hoverFill, pressFill
		fg, bd = textCol, border
		if b.Danger {
			fg, bd = errorC, errorC
		}
	case ButtonText:
		bgN = render.RGBA{}
		bgH, bgP = hoverFill, pressFill
		fg = textCol
		if b.Danger {
			fg = errorC
		}
		bd, bw = render.RGBA{}, 0
	case ButtonLink:
		bgN = render.RGBA{}
		bgH, bgP = hoverFill, pressFill
		fg = primary
		if b.Danger {
			fg = errorC
		}
		bd, bw = render.RGBA{}, 0
	default: // ButtonDefault
		bgN, bgH, bgP = bg, hoverFill, pressFill
		fg, bd = textCol, border
		if b.Danger {
			fg, bd = errorC, errorC
		}
	}

	if b.Disabled || b.Loading {
		bgN, bgH, bgP = disabledBg, disabledBg, disabledBg
		fg, bd = disabledText, border
		if b.Type == ButtonPrimary {
			// Keep a muted solid primary-ish block for disabled primary.
			p := primary
			bgN = render.RGBA{R: p.R, G: p.G, B: p.B, A: 0.35}
			bgH, bgP = bgN, bgN
			fg = textInv
			bd = bgN
			bw = 0
		}
	}

	b.bgNormal, b.bgHover, b.bgPressed = bgN, bgH, bgP
	b.bdNormal, b.borderW = bd, bw

	b.decorated.Background = bgN
	b.decorated.BorderColor = bd
	b.decorated.BorderWidth = bw
	b.label.Color = fg
	if b.icon != nil {
		b.icon.Color = fg
	}
	// Pressable stays transparent; Decorated owns chrome.
	b.Root.Color = render.RGBA{}
	b.Root.ColorHovered = render.RGBA{}
	b.Root.ColorPressed = render.RGBA{}
	b.lastHovered, b.lastPressed = false, false
	b.decorated.MarkNeedsPaint()
}
