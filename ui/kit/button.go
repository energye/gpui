package kit

import (
	"math"

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
//	            Spinner? · Icon? · Text(label)
//
// Hover/press chrome tracks PressableState automatically via OnStateChange.
// SyncState() remains for explicit host loops that want a manual refresh.
//
// Metrics follow Ant Design 5 defaults (middle: h=32, font=14, paddingInline=15, radius=6).
type Button struct {
	Root      *primitive.Pressable
	decorated *primitive.Decorated
	row       *primitive.Flex
	label     *primitive.Text
	icon      *primitive.Icon
	spinner   *primitive.Canvas

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
	// Style optional one-off overrides (background/font/size). See kit.Style.
	Style Style

	bgNormal, bgHover, bgPressed render.RGBA
	bdNormal, bdHover            render.RGBA
	borderW                      float64
	lastHovered, lastPressed     bool
	lastFocused                  bool
	spinPhase                    float64
	boundTree                    *core.Tree
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
		if b.Root != nil {
			b.Root.Base().Label = s
		}
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

// SetLoading toggles loading (disables press, shows spinner).
func (b *Button) SetLoading(v bool) {
	if b.Loading == v {
		return
	}
	b.Loading = v
	if b.Root != nil {
		b.Root.SetDisabled(b.Disabled || b.Loading)
	}
	// Spinner presence changes the child list — rebuild chrome.
	b.rebuild()
	if b.boundTree != nil {
		if v {
			b.boundTree.AddTicker(b)
		} else {
			b.boundTree.RemoveTicker(b)
		}
	}
}

// AttachTicker registers the loading spinner for demand-frame ANIMATING.
func (b *Button) AttachTicker(t *core.Tree) {
	if b == nil || t == nil {
		return
	}
	b.boundTree = t
	t.BindTicker(b, b.Loading)
}

// Tick advances the loading spinner. Implements core.Ticker when Loading.
func (b *Button) Tick(dt float64) bool {
	if b == nil || !b.Loading {
		return false
	}
	b.spinPhase += dt * 1.4
	if b.spinPhase > 1 {
		b.spinPhase -= 1
	}
	if b.spinner != nil {
		b.spinner.MarkNeedsPaint()
	} else if b.Root != nil {
		b.Root.MarkNeedsPaint()
	}
	return b.Loading
}

// SetDanger toggles danger styling.
func (b *Button) SetDanger(v bool) {
	b.Danger = v
	b.applyChrome()
}

// SetBlock makes the button expand to the parent max width when bounded
// (Ant block). Parent Column should use CrossStretch for full content width.
func (b *Button) SetBlock(v bool) {
	if b.Block == v {
		return
	}
	b.Block = v
	b.rebuild()
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
	b.Style.Face = face
	if b.label != nil {
		b.label.Face = face
	}
}

// SetStyle applies visual overrides (bg/text/font/size) and refreshes chrome.
func (b *Button) SetStyle(st Style) {
	b.Style = st
	if st.Face != nil {
		b.Face = st.Face
	}
	if st.FontSize > 0 || st.Height > 0 || st.hasRadius() {
		b.rebuild()
		return
	}
	if b.label != nil {
		if st.Face != nil {
			b.label.Face = st.Face
		}
		if st.FontSize > 0 {
			b.label.FontSize = st.FontSize
		}
	}
	b.applyChrome()
}

// SetBackground overrides idle fill color.
func (b *Button) SetBackground(c render.RGBA) {
	b.Style.Background = c
	b.applyChrome()
}

// SetTextColor overrides label color.
func (b *Button) SetTextColor(c render.RGBA) {
	b.Style.Text = c
	b.applyChrome()
}

// SetFontSize overrides label size (rebuilds metrics-sensitive layout).
func (b *Button) SetFontSize(px float64) {
	b.Style.FontSize = px
	b.rebuild()
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
// Prefer automatic OnStateChange; this remains for explicit host loops.
func (b *Button) SyncState() {
	if b.Root == nil || b.decorated == nil {
		return
	}
	h, p, f := b.Root.State.Hovered, b.Root.State.Pressed, b.Root.State.Focused
	if h == b.lastHovered && p == b.lastPressed && f == b.lastFocused {
		return
	}
	b.lastHovered, b.lastPressed, b.lastFocused = h, p, f
	b.applyStateChrome()
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
	var n core.Node
	if b.Root != nil {
		n = b.Root
	}
	return themeOf(b.Theme, n)
}

func (b *Button) rebuild() {
	th := b.theme()
	padH, padV, height, fontSize, radius, gap := b.metrics(th)

	b.label = primitive.NewText(b.Label)
	b.label.FontSize = fontSize
	b.label.Face = b.Face

	if b.row == nil {
		b.row = primitive.Row()
	} else {
		b.row.ClearChildren()
	}
	b.row.Gap = gap
	b.row.CrossAlign = core.CrossCenter
	b.row.MainAlign = core.MainCenter

	// Loading spinner (Ant: leading icon-sized ring).
	spinSize := fontSize
	if spinSize < 12 {
		spinSize = 12
	}
	if b.Loading {
		b.spinner = primitive.NewCanvas(spinSize, spinSize, b.paintSpinner)
		b.row.AddChild(b.spinner)
	} else {
		b.spinner = nil
	}

	if b.IconName != "" && !b.Loading {
		b.icon = primitive.NewIcon(b.IconName)
		b.icon.Size = fontSize + 2
		b.row.AddChild(b.icon)
	} else {
		b.icon = nil
	}
	b.row.AddChild(b.label)

	if b.decorated == nil {
		b.decorated = primitive.NewDecorated(b.row)
	} else {
		b.decorated.ClearChildren()
		b.decorated.AddChild(b.row)
	}
	b.decorated.Padding = primitive.Symmetric(padH, padV)
	b.decorated.Radius = radius
	// Force exact Ant control height; center label/icon in chrome.
	b.decorated.MinHeight = height
	b.decorated.Height = height
	b.decorated.SetCenterContent(true)
	if b.Block {
		// Full parent max width (Ant block); StretchChild centers label row.
		b.decorated.ExpandWidth = true
		b.decorated.StretchChild = true
		b.decorated.MinWidth = 0
	} else {
		b.decorated.ExpandWidth = false
		b.decorated.StretchChild = false
		b.decorated.MinWidth = 0
	}

	if b.Root == nil {
		b.Root = primitive.NewPressable(b.decorated)
	} else {
		// Keep mounted Pressable; ensure decorated is its child.
		b.Root.ClearChildren()
		b.Root.AddChild(b.decorated)
	}
	b.Root.Focusable = true
	b.Root.ShowFocusRing = true
	b.Root.FocusRingRadius = radius
	b.Root.FocusRingOutset = 1.5 // Ant-tight
	b.Root.Click = b.fireClick
	b.Root.OnStateChange = b.SyncState
	b.Root.SetDisabled(b.Disabled || b.Loading)
	b.Root.Base().Role = "button"
	b.Root.Base().Label = b.Label
	b.Root.SetThemeHook(func(*core.Theme) { b.rebuild() })

	b.lastHovered, b.lastPressed, b.lastFocused = false, false, false
	b.applyChrome()
	b.Root.MarkNeedsLayout()
	b.Root.MarkNeedsPaint()
}

func (b *Button) paintSpinner(pc *core.PaintContext, sz core.Size) {
	if pc == nil || !b.Loading {
		return
	}
	th := b.theme()
	col := th.Color(core.TokenColorPrimary)
	if b.Type == ButtonPrimary {
		col = th.Color(core.TokenColorTextInverse)
	}
	if b.Disabled {
		col = th.Color(core.TokenColorDisabledText)
	}
	track := render.RGBA{R: col.R, G: col.G, B: col.B, A: col.A * 0.35}
	if track.A < 0.1 {
		track.A = 0.2
	}
	stroke := 2.0
	if sz.Width < 14 {
		stroke = 1.5
	}
	cx, cy := sz.Width/2, sz.Height/2
	r := sz.Width/2 - stroke
	if r < 1 {
		r = 1
	}
	// Local helpers (canvas Origin already applied by PaintContext).
	pc.StrokeLocalCircle(cx, cy, r, stroke, track)
	start := -math.Pi/2 + b.spinPhase*2*math.Pi
	end := start + 2*math.Pi*0.7
	steps := 40
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		a := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(a), cy+r*math.Sin(a))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

func (b *Button) metrics(th *core.Theme) (padH, padV, height, fontSize, radius, gap float64) {
	fontSize = th.SizeOr(core.TokenFontSize, 14)
	radius = th.SizeOr(core.TokenBorderRadius, 6)
	gap = th.SizeOr(core.TokenMarginXS, 4) + 4 // Ant icon gap ~8
	padV = 0
	switch b.Size {
	case ButtonSmall:
		height = th.SizeOr(core.TokenControlHeightSM, 24)
		fontSize = th.SizeOr(core.TokenFontSizeSM, 12)
		padH = th.SizeOr(core.TokenButtonPaddingInlineSM, 7)
		radius = th.SizeOr(core.TokenBorderRadiusSM, 4)
		gap = th.SizeOr(core.TokenMarginXS, 4)
	case ButtonLarge:
		height = th.SizeOr(core.TokenControlHeightLG, 40)
		fontSize = th.SizeOr(core.TokenFontSizeLG, 16)
		padH = th.SizeOr(core.TokenButtonPaddingInlineLG, 15)
		radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	default:
		height = th.SizeOr(core.TokenControlHeight, 32)
		padH = th.SizeOr(core.TokenButtonPaddingInline, 15)
	}
	// Style overrides
	if b.Style.FontSize > 0 {
		fontSize = b.Style.FontSize
	}
	if b.Style.Height > 0 {
		height = b.Style.Height
	}
	if b.Style.hasRadius() {
		radius = b.Style.Radius
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
	b.decorated.Height = height
	if b.Root != nil {
		b.Root.FocusRingRadius = radius
		b.Root.FocusRingOutset = 1.5
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
	// Ant default button hover: colorBgTextHover (rgba black 0.06).
	hoverFill := th.Color(core.TokenColorBgTextHover)
	if hoverFill.A < 0.02 {
		hoverFill = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	// Composite hover over white so solid paint is correct without blend.
	hoverFill = compositeOver(hoverFill, bg)
	pressFill := th.Color(core.TokenColorBgTextActive)
	if pressFill.A < 0.02 {
		pressFill = render.RGBA{R: 0, G: 0, B: 0, A: 0.15}
	}
	pressFill = compositeOver(pressFill, bg)
	borderHover := th.Color(core.TokenColorPrimaryHover)
	if borderHover.A < 0.5 {
		borderHover = primaryHover
	}

	var bgN, bgH, bgP, fg, bd, bdH render.RGBA
	bw := th.SizeOr(core.TokenLineWidth, 1)
	var dash []float64

	switch b.Type {
	case ButtonPrimary:
		bgN, bgH, bgP = primary, primaryHover, primaryActive
		fg = textInv
		if b.Danger {
			bgN, bgH, bgP = errorC, render.Hex("#FF7875"), render.Hex("#D9363E")
		}
		bd, bdH = bgN, bgH
		bw = 0
	case ButtonDashed:
		bgN, bgH, bgP = bg, hoverFill, pressFill
		fg, bd, bdH = textCol, border, borderHover
		dash = []float64{3, 2}
		if b.Danger {
			fg, bd, bdH = errorC, errorC, render.Hex("#FF7875")
		}
	case ButtonText:
		bgN = render.RGBA{}
		bgH, bgP = hoverFill, pressFill
		fg = textCol
		if b.Danger {
			fg = errorC
		}
		bd, bdH, bw = render.RGBA{}, render.RGBA{}, 0
	case ButtonLink:
		bgN = render.RGBA{}
		// Link: hover lightens primary text, subtle bg optional.
		bgH, bgP = render.RGBA{}, render.RGBA{}
		fg = primary
		if b.Danger {
			fg = errorC
		}
		bd, bdH, bw = render.RGBA{}, render.RGBA{}, 0
	default: // ButtonDefault
		bgN, bgH, bgP = bg, hoverFill, pressFill
		fg, bd, bdH = textCol, border, borderHover
		if b.Danger {
			fg, bd, bdH = errorC, errorC, render.Hex("#FF7875")
		}
	}

	if b.Disabled {
		if b.Type == ButtonPrimary {
			// Disabled primary: muted solid (Ant ~35% opacity).
			p := primary
			if b.Danger {
				p = errorC
			}
			bgN = render.RGBA{R: p.R, G: p.G, B: p.B, A: 0.35}
			bgH, bgP = bgN, bgN
			fg = textInv
			bd, bdH = bgN, bgN
			bw = 0
			dash = nil
		} else {
			bgN, bgH, bgP = disabledBg, disabledBg, disabledBg
			fg, bd, bdH = disabledText, border, border
		}
	} else if b.Loading {
		// Ant loading: keep type colors solid; clicks blocked via Root.SetDisabled.
		if b.Type == ButtonPrimary {
			p := primary
			if b.Danger {
				p = errorC
			}
			// Slight dim (~12% white) without washing out.
			bgN = render.RGBA{R: p.R*0.88 + 0.12, G: p.G*0.88 + 0.12, B: p.B*0.88 + 0.12, A: 1}
			bgH, bgP = bgN, bgN
			fg = textInv
			bd, bdH = bgN, bgN
			bw = 0
			dash = nil
		}
		// non-primary: keep type chrome from switch above
	}

	// Style overrides (one-off colors)
	if b.Style.hasBG() {
		bgN = b.Style.Background
		if !b.Style.hasBGHover() {
			bgH = bgN
		}
		if !b.Style.hasBGActive() {
			bgP = bgN
		}
	}
	if b.Style.hasBGHover() {
		bgH = b.Style.BackgroundHover
	}
	if b.Style.hasBGActive() {
		bgP = b.Style.BackgroundActive
	}
	if b.Style.hasBorder() {
		bd, bdH = b.Style.Border, b.Style.Border
	}
	if b.Style.hasText() {
		fg = b.Style.Text
	}
	if b.Style.Width > 0 && b.decorated != nil {
		b.decorated.MinWidth = b.Style.Width
		b.decorated.Width = b.Style.Width
	}

	b.bgNormal, b.bgHover, b.bgPressed = bgN, bgH, bgP
	b.bdNormal, b.bdHover, b.borderW = bd, bdH, bw

	b.decorated.BorderDash = dash
	b.label.Color = fg
	if b.icon != nil {
		b.icon.Color = fg
	}
	// Pressable stays transparent; Decorated owns chrome.
	b.Root.Color = render.RGBA{}
	b.Root.ColorHovered = render.RGBA{}
	b.Root.ColorPressed = render.RGBA{}
	b.lastHovered, b.lastPressed, b.lastFocused = false, false, false
	b.applyStateChrome()
}

// applyStateChrome paints the current hover/press/focus into Decorated.
func (b *Button) applyStateChrome() {
	if b.decorated == nil {
		return
	}
	h, p, f := false, false, false
	if b.Root != nil {
		h, p = b.Root.State.Hovered, b.Root.State.Pressed
		f = b.Root.State.Focused
	}
	bg := b.bgNormal
	bd := b.bdNormal
	switch {
	case b.Disabled || b.Loading:
		bg = b.bgNormal
		bd = b.bdNormal
	case p:
		bg = b.bgPressed
		bd = b.bdHover
	case h:
		bg = b.bgHover
		bd = b.bdHover
	}
	dash := b.decorated.BorderDash
	if f && !b.Disabled && !b.Loading && b.Type != ButtonDashed {
		th := b.theme()
		bd = th.Color(core.TokenColorPrimary)
	}
	// Link type: text color shifts on hover (lighter primary).
	if b.Type == ButtonLink && !b.Disabled && !b.Loading && b.label != nil {
		th := b.theme()
		if h || p {
			if b.Danger {
				b.label.Color = render.Hex("#FF7875")
			} else {
				b.label.Color = th.Color(core.TokenColorPrimaryHover)
			}
		} else {
			if b.Danger {
				b.label.Color = th.Color(core.TokenColorError)
			} else {
				b.label.Color = th.Color(core.TokenColorPrimary)
			}
		}
	}
	b.decorated.Background = bg
	b.decorated.BorderColor = bd
	b.decorated.BorderWidth = b.borderW
	// Dashed stays dashed in all states including focus (user: dashed focus = dashed).
	if b.Type == ButtonDashed && !b.Disabled && !b.Loading {
		b.decorated.BorderDash = []float64{3, 2}
		if f || h || p {
			// Focus/hover: still dashed but primary-colored.
			th := b.theme()
			b.decorated.BorderColor = th.Color(core.TokenColorPrimary)
		}
	} else {
		b.decorated.BorderDash = nil
	}
	_ = dash
	b.decorated.MarkNeedsPaint()
}

// compositeOver blends src (with alpha) over dst solid.
func compositeOver(src, dst render.RGBA) render.RGBA {
	a := src.A
	if a <= 0 {
		return dst
	}
	if a >= 1 {
		return src
	}
	ia := 1 - a
	return render.RGBA{
		R: src.R*a + dst.R*ia,
		G: src.G*a + dst.G*ia,
		B: src.B*a + dst.B*ia,
		A: 1,
	}
}
