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
//	            Spinner? · Icon? · Text(label) · Icon?   (icon start or end)
//
// Hover/press chrome tracks PressableState automatically via OnStateChange.
// SyncState() remains for explicit host loops that want a manual refresh.
//
// Metrics follow docs/antd/button.md §6.2 (middle: h=32, font=14, paddingInline=15, radius=6).
// Product contract: docs/antd/button.md §6 (P0 DoD).
type Button struct {
	Root      *primitive.Pressable
	decorated *primitive.Decorated
	row       *primitive.Flex
	label     *primitive.Text
	icon      *primitive.Icon
	spinner   *primitive.Canvas

	Type     ButtonType
	Size     ButtonSize
	Shape    ButtonShape // default | circle | round (§6.10)
	Danger   bool
	Disabled bool
	Loading  bool
	Block    bool
	// Ghost: transparent fill (Ant ghost).
	Ghost bool
	// Variant / Color: Ant 5.21+; VariantAuto derives from Type.
	// When Variant != Auto, it takes precedence over Type (§6.3 / B-S / BTN-19).
	Variant ButtonVariant
	Color   ButtonColor
	// IconPlacement: start (default) or end.
	IconPlacement ButtonIconPlacement
	Label         string
	IconName      string
	// AriaLabel: accessible name; required for icon-only (BTN-23 / §6.6).
	AriaLabel string
	OnClick   func()
	Face      text.Face
	Theme     *core.Theme
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
// Defaults (docs/antd/button.md §6.10): Type=default, Size=middle, Shape=default,
// Variant=auto, Color=default, IconPlacement=start, all flags false.
func NewButton(label string) *Button {
	b := &Button{
		Label:         label,
		Type:          ButtonDefault,
		Size:          ButtonMiddle,
		Shape:         ButtonShapeDefault,
		Variant:       ButtonVariantAuto,
		Color:         ButtonColorDefault,
		IconPlacement: ButtonIconStart,
	}
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
	prevEmpty := b.Label == ""
	b.Label = s
	// Child list / circle padding depends on whether label is empty.
	if prevEmpty != (s == "") || b.Shape == ButtonShapeCircle {
		b.rebuild()
		return
	}
	if b.label != nil {
		b.label.SetValue(s)
		b.applyA11yName()
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

// SetGhost toggles Ant ghost styling (transparent fill).
func (b *Button) SetGhost(v bool) {
	if b.Ghost == v {
		return
	}
	b.Ghost = v
	b.applyChrome()
}

// SetVariant sets Ant 5.21+ visual variant (Auto → derive from Type).
func (b *Button) SetVariant(v ButtonVariant) {
	if b.Variant == v {
		return
	}
	b.Variant = v
	b.applyChrome()
}

// SetColor sets Ant 5.21+ semantic color for variants (Default → Type/Danger).
func (b *Button) SetColor(c ButtonColor) {
	if b.Color == c {
		return
	}
	b.Color = c
	b.applyChrome()
}

// SetIcon sets an optional icon name (empty clears). Placement via SetIconPlacement.
func (b *Button) SetIcon(name string) {
	b.IconName = name
	b.rebuild()
}

// SetIconPlacement places the icon at start (default) or end of the label.
func (b *Button) SetIconPlacement(p ButtonIconPlacement) {
	if b.IconPlacement == p {
		return
	}
	b.IconPlacement = p
	b.rebuild()
}

// SetShape sets Ant shape: default rectangle, circle (w=h), or round (capsule).
func (b *Button) SetShape(s ButtonShape) {
	if b.Shape == s {
		return
	}
	b.Shape = s
	b.rebuild()
}

// SetAriaLabel sets the accessible name. Required when Label is empty (icon-only).
func (b *Button) SetAriaLabel(name string) {
	b.AriaLabel = name
	if b.Root != nil {
		b.applyA11yName()
	}
}

// SetOnClick sets the click handler.
func (b *Button) SetOnClick(fn func()) {
	b.OnClick = fn
	if b.Root != nil {
		b.Root.Click = b.fireClick
	}
}

// applyA11yName sets Pressable role label from Label or AriaLabel (§6.6).
func (b *Button) applyA11yName() {
	if b.Root == nil {
		return
	}
	name := b.Label
	if name == "" {
		name = b.AriaLabel
	}
	b.Root.Base().Label = name
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
	h, p := b.Root.State.Hovered, b.Root.State.Pressed
	f := b.Root.State.Focused && b.Root.State.FocusVisible
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

	// Shape: circle → square box w=h, full round; round → capsule radius h/2.
	// docs/antd/button.md §6.2 / BTN-16 / BTN-17
	switch b.Shape {
	case ButtonShapeCircle:
		radius = height / 2
	case ButtonShapeRound:
		radius = height / 2
	}

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
	} else {
		b.icon = nil
	}
	// Icon start (default): icon · label; end: label · icon. Spinner always leading when loading.
	// Skip empty label when icon-only so gap does not offset the icon (FloatButton FAB / circle).
	hasLabel := b.Label != ""
	if b.icon != nil && b.IconPlacement != ButtonIconEnd {
		b.row.AddChild(b.icon)
	}
	if hasLabel || b.icon == nil {
		// Always keep a label node for color updates when text-only; icon-only omits empty text.
		b.row.AddChild(b.label)
	}
	if b.icon != nil && b.IconPlacement == ButtonIconEnd {
		b.row.AddChild(b.icon)
	}

	if b.decorated == nil {
		b.decorated = primitive.NewDecorated(b.row)
	} else {
		b.decorated.ClearChildren()
		b.decorated.AddChild(b.row)
	}
	// Circle icon-only: no horizontal padding so w≈h; content centered.
	if b.Shape == ButtonShapeCircle && !hasLabel {
		b.decorated.Padding = primitive.Symmetric(0, 0)
	} else {
		b.decorated.Padding = primitive.Symmetric(padH, padV)
	}
	b.decorated.Radius = radius
	// Force exact Ant control height; center label/icon in chrome.
	b.decorated.MinHeight = height
	b.decorated.Height = height
	b.decorated.SetCenterContent(true)
	if b.Shape == ButtonShapeCircle {
		// Square outer box (BTN-16).
		b.decorated.MinWidth = height
		b.decorated.Width = height
		b.decorated.ExpandWidth = false
		b.decorated.StretchChild = false
	} else if b.Block {
		// Full parent max width (Ant block); StretchChild centers label row.
		b.decorated.ExpandWidth = true
		b.decorated.StretchChild = true
		b.decorated.MinWidth = 0
		b.decorated.Width = 0
	} else {
		b.decorated.ExpandWidth = false
		b.decorated.StretchChild = false
		b.decorated.MinWidth = 0
		b.decorated.Width = 0
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
	b.Root.FocusRingOutset = 1.5 // Ant-tight §6.2
	b.Root.Click = b.fireClick
	b.Root.OnStateChange = b.SyncState
	b.Root.SetDisabled(b.Disabled || b.Loading)
	b.Root.Base().Role = "button"
	b.applyA11yName()
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
	// Inverse spinner on solid non-ghost fills.
	v := b.Variant
	if v == ButtonVariantAuto && b.Type == ButtonPrimary {
		v = ButtonVariantSolid
	}
	if v == ButtonVariantSolid && !b.Ghost {
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
	switch b.Shape {
	case ButtonShapeCircle, ButtonShapeRound:
		radius = height / 2
	}
	b.decorated.Radius = radius
	b.decorated.MinHeight = height
	b.decorated.Height = height
	if b.Shape == ButtonShapeCircle {
		b.decorated.MinWidth = height
		b.decorated.Width = height
	}
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
	successC := th.Color(core.TokenColorSuccess)
	warningC := th.Color(core.TokenColorWarning)
	primaryBg := th.Color(core.TokenColorPrimaryBg)
	hoverFill := th.Color(core.TokenColorBgTextHover)
	if hoverFill.A < 0.02 {
		hoverFill = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
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

	// Resolve Ant 5.21+ color + variant (Auto → classic Type).
	variant, accent, accentHover, accentActive := b.resolveColorVariant(
		primary, primaryHover, primaryActive, errorC, successC, warningC, textCol, border,
	)

	var bgN, bgH, bgP, fg, bd, bdH render.RGBA
	bw := th.SizeOr(core.TokenLineWidth, 1)
	var dash []float64

	switch variant {
	case ButtonVariantSolid:
		bgN, bgH, bgP = accent, accentHover, accentActive
		fg = textInv
		bd, bdH = bgN, bgH
		bw = 0
	case ButtonVariantDashed:
		bgN, bgH, bgP = bg, hoverFill, pressFill
		if b.Color == ButtonColorDefault && !b.Danger && b.Type != ButtonPrimary {
			// Classic Type=Dashed: black text + gray dashed border.
			fg, bd, bdH = textCol, border, borderHover
		} else {
			// color=primary/danger dashed: accent stroke + text.
			fg, bd, bdH = accent, accent, accentHover
		}
		dash = []float64{3, 2}
	case ButtonVariantFilled:
		// Light wash of accent (primaryBg for primary; tint otherwise).
		fill := primaryBg
		if b.Color == ButtonColorDanger || (b.Color == ButtonColorDefault && b.Danger) {
			fill = render.RGBA{R: errorC.R, G: errorC.G, B: errorC.B, A: 0.1}
			fill = compositeOver(fill, bg)
		} else if b.Color == ButtonColorSuccess {
			fill = render.RGBA{R: successC.R, G: successC.G, B: successC.B, A: 0.1}
			fill = compositeOver(fill, bg)
		} else if b.Color == ButtonColorWarning {
			fill = render.RGBA{R: warningC.R, G: warningC.G, B: warningC.B, A: 0.12}
			fill = compositeOver(fill, bg)
		} else if b.Color != ButtonColorPrimary && b.Color != ButtonColorDefault {
			fill = primaryBg
		} else if b.Type != ButtonPrimary && b.Color == ButtonColorDefault && !b.Danger {
			// default filled ≈ subtle gray
			fill = hoverFill
		}
		bgN, bgH, bgP = fill, compositeOver(hoverFill, fill), compositeOver(pressFill, fill)
		fg = accent
		bd, bdH, bw = render.RGBA{}, render.RGBA{}, 0
	case ButtonVariantText:
		bgN = render.RGBA{}
		bgH, bgP = hoverFill, pressFill
		fg = accent
		bd, bdH, bw = render.RGBA{}, render.RGBA{}, 0
	case ButtonVariantLink:
		bgN, bgH, bgP = render.RGBA{}, render.RGBA{}, render.RGBA{}
		fg = accent
		bd, bdH, bw = render.RGBA{}, render.RGBA{}, 0
	default: // outlined
		bgN, bgH, bgP = bg, hoverFill, pressFill
		// Primary/Danger/... outlined: accent stroke + text.
		// Default color outlined: text token + gray border (Ant default button).
		if b.Color == ButtonColorDefault && !b.Danger && b.Type != ButtonPrimary {
			fg, bd, bdH = textCol, border, borderHover
		} else {
			fg, bd, bdH = accent, accent, accentHover
		}
	}

	// Ghost: transparent idle fill; keep border/text of the variant (Ant ghost).
	if b.Ghost && !b.Disabled {
		switch variant {
		case ButtonVariantSolid:
			// solid+ghost → outlined accent on transparent
			bgN = render.RGBA{}
			bgH = render.RGBA{R: accent.R, G: accent.G, B: accent.B, A: 0.1}
			bgH = compositeOver(bgH, bg)
			bgP = render.RGBA{R: accent.R, G: accent.G, B: accent.B, A: 0.2}
			bgP = compositeOver(bgP, bg)
			fg = accent
			bd, bdH = accent, accentHover
			bw = th.SizeOr(core.TokenLineWidth, 1)
			dash = nil
		case ButtonVariantText, ButtonVariantLink:
			// already transparent
		default:
			bgN = render.RGBA{}
			// keep hover washes
		}
	}

	if b.Disabled {
		if variant == ButtonVariantSolid && !b.Ghost {
			p := accent
			bgN = render.RGBA{R: p.R, G: p.G, B: p.B, A: 0.35}
			bgH, bgP = bgN, bgN
			fg = textInv
			bd, bdH = bgN, bgN
			bw = 0
			dash = nil
		} else {
			bgN, bgH, bgP = disabledBg, disabledBg, disabledBg
			fg, bd, bdH = disabledText, border, border
			if b.Ghost {
				bgN, bgH, bgP = render.RGBA{}, render.RGBA{}, render.RGBA{}
			}
		}
	} else if b.Loading {
		if variant == ButtonVariantSolid && !b.Ghost {
			p := accent
			bgN = render.RGBA{R: p.R*0.88 + 0.12, G: p.G*0.88 + 0.12, B: p.B*0.88 + 0.12, A: 1}
			bgH, bgP = bgN, bgN
			fg = textInv
			bd, bdH = bgN, bgN
			bw = 0
			dash = nil
		}
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

// resolveColorVariant maps Type/Danger/Color/Variant to a concrete variant and accent colors.
func (b *Button) resolveColorVariant(
	primary, primaryHover, primaryActive, errorC, successC, warningC, textCol, border render.RGBA,
) (variant ButtonVariant, accent, accentHover, accentActive render.RGBA) {
	// Variant from Type when Auto.
	variant = b.Variant
	if variant == ButtonVariantAuto {
		switch b.Type {
		case ButtonPrimary:
			variant = ButtonVariantSolid
		case ButtonDashed:
			variant = ButtonVariantDashed
		case ButtonText:
			variant = ButtonVariantText
		case ButtonLink:
			variant = ButtonVariantLink
		default:
			variant = ButtonVariantOutlined
		}
	}
	// Accent from Color, then Danger, then Type primary.
	switch b.Color {
	case ButtonColorPrimary:
		accent, accentHover, accentActive = primary, primaryHover, primaryActive
	case ButtonColorDanger:
		accent, accentHover, accentActive = errorC, render.Hex("#FF7875"), render.Hex("#D9363E")
	case ButtonColorSuccess:
		accent, accentHover, accentActive = successC, render.Hex("#73D13D"), render.Hex("#389E0D")
	case ButtonColorWarning:
		accent, accentHover, accentActive = warningC, render.Hex("#FFC53D"), render.Hex("#D48806")
	default:
		if b.Danger {
			accent, accentHover, accentActive = errorC, render.Hex("#FF7875"), render.Hex("#D9363E")
		} else if b.Type == ButtonPrimary || variant == ButtonVariantSolid || variant == ButtonVariantLink {
			accent, accentHover, accentActive = primary, primaryHover, primaryActive
		} else {
			// Default color + non-primary type: neutral text; outlined/dashed use border token
			// when accent equals textCol (see variant branches: accent.A check uses border for bd).
			accent, accentHover, accentActive = textCol, primaryHover, primaryActive
		}
	}
	return variant, accent, accentHover, accentActive
}

// applyStateChrome paints the current hover/press/focus into Decorated.
func (b *Button) applyStateChrome() {
	if b.decorated == nil {
		return
	}
	h, p, f := false, false, false
	if b.Root != nil {
		h, p = b.Root.State.Hovered, b.Root.State.Pressed
		// Ant :focus-visible — only keyboard focus changes chrome outline.
		f = b.Root.State.Focused && b.Root.State.FocusVisible
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
	isDashed := b.Type == ButtonDashed || b.Variant == ButtonVariantDashed
	if f && !b.Disabled && !b.Loading && !isDashed {
		th := b.theme()
		bd = th.Color(core.TokenColorPrimary)
	}
	// Link (type or variant): text color shifts on hover (lighter primary).
	isLink := b.Type == ButtonLink || b.Variant == ButtonVariantLink
	if isLink && !b.Disabled && !b.Loading && b.label != nil {
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
	// Dashed (Type or Variant) keeps dash pattern in all states.
	if isDashed && !b.Disabled && !b.Loading {
		b.decorated.BorderDash = []float64{3, 2}
		if f || h || p {
			// Focus/hover: still dashed but primary-colored (classic Type=Dashed).
			// Color+Variant dashed keeps accent from applyChrome (bd already set).
			if b.Variant == ButtonVariantAuto || b.Variant == 0 {
				th := b.theme()
				b.decorated.BorderColor = th.Color(core.TokenColorPrimary)
			}
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
