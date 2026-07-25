package kit

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Avatar defaults — prepareComponentToken / genBaseStyle.
// docs/antd/avatar.md §6.2 / §6.10
const (
	// DefaultAvatarGap is characters' side inset (antd gap default 4).
	DefaultAvatarGap = 4.0
	// DefaultAvatarIconFont is middle icon size ≈ (fontSizeLG+fontSizeXL)/2.
	DefaultAvatarIconFont = 18.0
	// DefaultAvatarIconFontLG ≈ fontSizeHeading3.
	DefaultAvatarIconFontLG = 24.0
	// DefaultAvatarCustomTextFont is fontSize used for custom-number non-icon avatars.
	DefaultAvatarCustomTextFont = 18.0
	// DefaultAvatarGroupOverlapping is antd groupOverlapping (−marginXS seed 8).
	DefaultAvatarGroupOverlapping = -8.0
	// DefaultAvatarGroupSpace is antd groupSpace (marginXXS).
	DefaultAvatarGroupSpace = 4.0
	// avatarSpinRPS is loading ring turns per second.
	avatarSpinRPS = 1.0
)

// AvatarSize is the size ladder (antd size).
type AvatarSize int

const (
	// AvatarMiddle is the default 32px controlHeight.
	AvatarMiddle AvatarSize = iota
	// AvatarLarge is 40px controlHeightLG.
	AvatarLarge
	// AvatarSmall is 24px controlHeightSM.
	AvatarSmall
	// AvatarSizeCustom uses SizePx / responsive map.
	AvatarSizeCustom
)

// AvatarShape is circle | square.
type AvatarShape int

const (
	// AvatarCircle is the default round avatar.
	AvatarCircle AvatarShape = iota
	// AvatarSquare uses borderRadius ladder.
	AvatarSquare
)

// AvatarContent is the resolved content mode (image | icon | text).
type AvatarContent int

const (
	// AvatarContentText shows children characters.
	AvatarContentText AvatarContent = iota
	// AvatarContentIcon shows icon / IconNode.
	AvatarContentIcon
	// AvatarContentImage shows src image (pixels or placeholder).
	AvatarContentImage
)

// AvatarResponsiveSize maps antd breakpoint keys to pixel sizes.
// Zero entries are skipped when resolving.
type AvatarResponsiveSize struct {
	XS, SM, MD, LG, XL, XXL float64
}

// Avatar is Ant Design Avatar (data display).
//
//	Decorated root (size×size; radius; bg)
//	  └─ content: image | icon | scaled text
//	  └─ loading spinner (optional overlay)
//
// Optional Pressable host when interactive (OnClick / SetInteractive).
// Product contract: docs/antd/avatar.md §6 (P0 DoD).
type Avatar struct {
	Root      *primitive.Decorated
	pressable *primitive.Pressable
	label     *primitive.Text
	icon      *primitive.Icon
	spinner   *primitive.Canvas
	imgNode   *primitive.PainterNode

	// Text is children string (character avatar).
	Text string
	// IconName is a registry icon key (empty = none).
	IconName string
	// IconNode is an optional custom icon node (wins over IconName when set).
	IconNode core.Node
	// Shape defaults to AvatarCircle.
	Shape AvatarShape
	// Size is the preset ladder; AvatarSizeCustom uses SizePx / Responsive.
	Size AvatarSize
	// SizePx is the custom number size when Size==AvatarSizeCustom (or SetSizePx).
	SizePx float64
	// Responsive + Breakpoint drive responsive size map (antd size={{xs:…}}).
	Responsive AvatarResponsiveSize
	Breakpoint string // "xs"|"sm"|"md"|"lg"|"xl"|"xxl"; empty → first non-zero / md

	// Gap is character side inset; 0 → DefaultAvatarGap unless gapSet.
	Gap    float64
	gapSet bool

	// Src is the image source label/URL (host decodes via SetPixels / SetImageOK).
	Src         string
	SrcSet      string
	Alt         string
	Draggable   bool
	CrossOrigin string
	// isImgExist mirrors antd state; false after NotifyImageError fallback.
	isImgExist bool
	// imageOK is host-confirmed decode success (or SetPixels).
	imageOK bool
	// Pixels optional packed RGBA for image paint.
	Pixels         []byte
	PixelW, PixelH int

	Loading   bool
	Disabled  bool
	OnError   func() bool
	OnClick   func()
	AriaLabel string
	// Interactive enables focus + keyboard activation (also set by SetOnClick).
	Interactive bool

	// Group-applied context (only when child has not set size/shape explicitly).
	groupSize     AvatarSize
	groupSizePx   float64
	groupShape    AvatarShape
	groupHasSize  bool
	groupHasShape bool
	// sizeExplicit / shapeExplicit: SetSize* / SetShape called on this instance.
	sizeExplicit  bool
	shapeExplicit bool
	// groupBorder paints groupBorderColor when true (Avatar.Group children).
	groupBorder bool

	Face  text.Face
	Theme *core.Theme
	Style Style

	// TextScale is last computed character scale (≤1); exported for tests.
	TextScale float64

	spinPhase float64
	life      tickerLifecycle
	boundTree *core.Tree
}

// NewAvatar creates an Avatar with optional character children.
func NewAvatar(text string) *Avatar {
	a := &Avatar{
		Text:      text,
		Size:      AvatarMiddle,
		Shape:     AvatarCircle,
		Draggable: true,
		TextScale: 1,
	}
	a.rebuild()
	return a
}

// NewAvatarIcon creates an icon Avatar.
func NewAvatarIcon(iconName string) *Avatar {
	a := NewAvatar("")
	a.SetIcon(iconName)
	return a
}

// Node returns the mount root (Pressable when interactive, else Decorated).
func (a *Avatar) Node() core.Node {
	if a == nil {
		return nil
	}
	if a.Root == nil {
		a.rebuild()
	}
	if a.Interactive && a.pressable != nil {
		return a.pressable
	}
	return a.Root
}

// ChromeNode returns the Decorated chrome (size / radius / bg).
func (a *Avatar) ChromeNode() core.Node {
	if a == nil {
		return nil
	}
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// ContentMode resolves image | icon | text (antd priority).
func (a *Avatar) ContentMode() AvatarContent {
	if a == nil {
		return AvatarContentText
	}
	if a.Src != "" && a.isImgExist {
		return AvatarContentImage
	}
	if a.IconNode != nil || a.IconName != "" {
		return AvatarContentIcon
	}
	return AvatarContentText
}

// ResolvedSize returns the pixel edge length after size / responsive resolution.
func (a *Avatar) ResolvedSize() float64 {
	if a == nil {
		return 32
	}
	return a.resolveSize(a.theme())
}

// SetText updates character children.
func (a *Avatar) SetText(s string) {
	if a == nil {
		return
	}
	a.Text = s
	a.rebuild()
}

// SetIcon sets a named icon (empty clears). Wins over text when no image.
func (a *Avatar) SetIcon(name string) {
	if a == nil {
		return
	}
	a.IconName = name
	if name != "" {
		a.IconNode = nil
	}
	a.rebuild()
}

// SetIconNode sets a custom icon node (wins over IconName).
func (a *Avatar) SetIconNode(n core.Node) {
	if a == nil {
		return
	}
	a.IconNode = n
	a.rebuild()
}

// SetShape sets circle | square.
func (a *Avatar) SetShape(sh AvatarShape) {
	if a == nil {
		return
	}
	a.Shape = sh
	a.shapeExplicit = true
	a.rebuild()
}

// SetSize sets a preset size ladder entry.
func (a *Avatar) SetSize(sz AvatarSize) {
	if a == nil {
		return
	}
	a.Size = sz
	if sz != AvatarSizeCustom {
		a.SizePx = 0
	}
	a.sizeExplicit = true
	a.rebuild()
}

// SetSizePx sets a custom number size (antd size={n}).
func (a *Avatar) SetSizePx(px float64) {
	if a == nil {
		return
	}
	if px <= 0 {
		px = 32
	}
	a.Size = AvatarSizeCustom
	a.SizePx = px
	a.sizeExplicit = true
	// Clear responsive so custom wins.
	a.Responsive = AvatarResponsiveSize{}
	a.rebuild()
}

// SetResponsiveSize installs antd size={{ xs, sm, … }} map.
func (a *Avatar) SetResponsiveSize(m AvatarResponsiveSize) {
	if a == nil {
		return
	}
	a.Responsive = m
	a.Size = AvatarSizeCustom
	a.sizeExplicit = true
	a.rebuild()
}

// SetBreakpoint sets the active responsive key ("xs"…"xxl").
func (a *Avatar) SetBreakpoint(bp string) {
	if a == nil {
		return
	}
	a.Breakpoint = strings.ToLower(strings.TrimSpace(bp))
	a.rebuild()
}

// SetGap sets character side inset (antd gap; default 4).
func (a *Avatar) SetGap(px float64) {
	if a == nil {
		return
	}
	a.Gap = px
	a.gapSet = true
	a.rebuild()
}

// SetSrc sets the image source and resets image-exist to true (antd src change).
func (a *Avatar) SetSrc(src string) {
	if a == nil {
		return
	}
	a.Src = src
	a.isImgExist = src != ""
	a.imageOK = false
	if src == "" {
		a.Pixels = nil
		a.PixelW, a.PixelH = 0, 0
	}
	a.rebuild()
}

// SetSrcSet stores srcSet (P1 decode).
func (a *Avatar) SetSrcSet(s string) {
	if a == nil {
		return
	}
	a.SrcSet = s
}

// SetPixels installs RGBA image data and marks image OK.
func (a *Avatar) SetPixels(w, h int, rgba []byte) {
	if a == nil {
		return
	}
	a.PixelW, a.PixelH = w, h
	a.Pixels = rgba
	if w > 0 && h > 0 && len(rgba) >= w*h*4 {
		a.imageOK = true
		a.isImgExist = true
		if a.Src == "" {
			a.Src = "pixels"
		}
	}
	a.rebuild()
}

// SetImageOK marks host decode success/failure without firing onError.
// true → image mode; false → isImgExist false (fallback) without callback.
func (a *Avatar) SetImageOK(ok bool) {
	if a == nil {
		return
	}
	a.imageOK = ok
	a.isImgExist = ok && a.Src != ""
	if ok && a.Src == "" {
		a.Src = "ok"
		a.isImgExist = true
	}
	a.rebuild()
}

// NotifyImageError simulates img onError (antd handleImgLoadError).
func (a *Avatar) NotifyImageError() {
	if a == nil {
		return
	}
	keep := false
	if a.OnError != nil {
		// antd: return false prevents default fallback.
		keep = a.OnError() == false
	}
	if !keep {
		a.isImgExist = false
		a.imageOK = false
	}
	a.rebuild()
}

// SetAlt sets image alt text.
func (a *Avatar) SetAlt(s string) {
	if a == nil {
		return
	}
	a.Alt = s
	a.applyA11yName()
}

// SetDraggable stores the draggable flag (desktop marker).
func (a *Avatar) SetDraggable(v bool) {
	if a == nil {
		return
	}
	a.Draggable = v
}

// SetCrossOrigin stores CORS (P1).
func (a *Avatar) SetCrossOrigin(s string) {
	if a == nil {
		return
	}
	a.CrossOrigin = s
}

// SetOnError sets the image error callback.
func (a *Avatar) SetOnError(fn func() bool) {
	if a == nil {
		return
	}
	a.OnError = fn
}

// SetOnClick enables interactive mode and click handler.
func (a *Avatar) SetOnClick(fn func()) {
	if a == nil {
		return
	}
	a.OnClick = fn
	if fn != nil {
		a.Interactive = true
	}
	a.rebuild()
}

// SetInteractive toggles focusable / keyboard activation.
func (a *Avatar) SetInteractive(v bool) {
	if a == nil {
		return
	}
	a.Interactive = v
	a.rebuild()
}

// SetLoading toggles loading spinner (Ticker).
func (a *Avatar) SetLoading(v bool) {
	if a == nil {
		return
	}
	if a.Loading == v {
		return
	}
	a.Loading = v
	a.rebuild()
	a.life.setActive(v)
}

// SetDisabled toggles disabled chrome.
func (a *Avatar) SetDisabled(v bool) {
	if a == nil {
		return
	}
	a.Disabled = v
	if a.pressable != nil {
		a.pressable.SetDisabled(v)
	}
	a.rebuild()
}

// SetTheme sets the theme override.
func (a *Avatar) SetTheme(th *core.Theme) {
	if a == nil {
		return
	}
	a.Theme = th
	a.rebuild()
}

// SetStyle applies Style overrides (Background / Text / …).
func (a *Avatar) SetStyle(st Style) {
	if a == nil {
		return
	}
	a.Style = st
	if st.Face != nil {
		a.Face = st.Face
	}
	a.rebuild()
}

// SetFace sets the font face for character avatars.
func (a *Avatar) SetFace(face text.Face) {
	if a == nil {
		return
	}
	a.Face = face
	if a.label != nil {
		a.label.Face = face
		a.label.MarkNeedsLayout()
		a.label.MarkNeedsPaint()
	}
	// Recompute scale with new face metrics.
	a.rebuild()
}

// SetAriaLabel sets the accessible name.
func (a *Avatar) SetAriaLabel(name string) {
	if a == nil {
		return
	}
	a.AriaLabel = name
	a.applyA11yName()
}

// AttachTicker binds loading spin to the tree.
func (a *Avatar) AttachTicker(t *core.Tree) {
	if a == nil || t == nil {
		return
	}
	a.boundTree = t
	a.life.attach(t, a, a.Loading)
}

// Tick advances the loading spinner.
func (a *Avatar) Tick(dt float64) bool {
	if a == nil {
		return false
	}
	if !a.life.stillMounted(a.boundTree) {
		return false
	}
	if !a.Loading {
		return false
	}
	a.spinPhase += dt * avatarSpinRPS
	if a.spinPhase > 1 {
		a.spinPhase = math.Mod(a.spinPhase, 1)
	}
	if a.spinner != nil {
		a.spinner.MarkNeedsPaint()
	} else if a.Root != nil {
		a.Root.MarkNeedsPaint()
	}
	return true
}

// applyGroupContext is used by AvatarGroup to push size/shape/border.
func (a *Avatar) applyGroupContext(size AvatarSize, sizePx float64, shape AvatarShape, border bool) {
	if a == nil {
		return
	}
	a.groupHasSize = true
	a.groupSize = size
	a.groupSizePx = sizePx
	a.groupHasShape = true
	a.groupShape = shape
	a.groupBorder = border
	a.rebuild()
}

func (a *Avatar) theme() *core.Theme {
	var n core.Node
	if a.Root != nil {
		n = a.Root
	}
	return themeOf(a.Theme, n)
}

func (a *Avatar) gap() float64 {
	if a.gapSet {
		return a.Gap
	}
	if a.Gap > 0 {
		return a.Gap
	}
	return DefaultAvatarGap
}

func (a *Avatar) resolveShape() AvatarShape {
	if a.shapeExplicit {
		return a.Shape
	}
	if a.groupHasShape {
		return a.groupShape
	}
	return a.Shape
}

func (a *Avatar) resolveSize(th *core.Theme) float64 {
	// Responsive map wins when any entry is set (explicit on this instance).
	if px := a.resolveResponsive(); px > 0 {
		return px
	}
	// Explicit size on the avatar wins over group context (antd customSize ?? ctx).
	if !a.sizeExplicit && a.groupHasSize {
		switch a.groupSize {
		case AvatarSmall:
			return th.SizeOr(core.TokenControlHeightSM, 24)
		case AvatarLarge:
			return th.SizeOr(core.TokenControlHeightLG, 40)
		case AvatarSizeCustom:
			if a.groupSizePx > 0 {
				return a.groupSizePx
			}
		default:
			return th.SizeOr(core.TokenControlHeight, 32)
		}
	}
	switch a.Size {
	case AvatarSmall:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	case AvatarLarge:
		return th.SizeOr(core.TokenControlHeightLG, 40)
	case AvatarSizeCustom:
		if a.SizePx > 0 {
			return a.SizePx
		}
		return th.SizeOr(core.TokenControlHeight, 32)
	default:
		return th.SizeOr(core.TokenControlHeight, 32)
	}
}

func (a *Avatar) resolveResponsive() float64 {
	m := a.Responsive
	if m.XS == 0 && m.SM == 0 && m.MD == 0 && m.LG == 0 && m.XL == 0 && m.XXL == 0 {
		return 0
	}
	// antd responsiveArray: xxl xl lg md sm xs — first matching screen.
	// Host injects Breakpoint; empty → prefer md then largest set.
	order := []struct {
		key string
		v   float64
	}{
		{"xxl", m.XXL},
		{"xl", m.XL},
		{"lg", m.LG},
		{"md", m.MD},
		{"sm", m.SM},
		{"xs", m.XS},
	}
	bp := a.Breakpoint
	if bp != "" {
		for _, e := range order {
			if e.key == bp && e.v > 0 {
				return e.v
			}
		}
	}
	// Fallback: first non-zero in antd priority order (largest first for gallery demos).
	for _, e := range order {
		if e.v > 0 {
			return e.v
		}
	}
	return 0
}

func (a *Avatar) squareRadius(th *core.Theme, edge float64) float64 {
	// Prefer ladder matching size preset; custom → borderRadius by edge.
	sz := a.Size
	if !a.sizeExplicit && a.groupHasSize {
		sz = a.groupSize
	}
	switch sz {
	case AvatarSmall:
		return th.SizeOr(core.TokenBorderRadiusSM, 4)
	case AvatarLarge:
		return th.SizeOr(core.TokenBorderRadiusLG, 8)
	case AvatarSizeCustom:
		// Approximate: small-ish → SM, large-ish → LG, else default.
		if edge <= 24 {
			return th.SizeOr(core.TokenBorderRadiusSM, 4)
		}
		if edge >= 40 {
			return th.SizeOr(core.TokenBorderRadiusLG, 8)
		}
		return th.SizeOr(core.TokenBorderRadius, 6)
	default:
		return th.SizeOr(core.TokenBorderRadius, 6)
	}
}

func (a *Avatar) textFontSize(th *core.Theme, edge float64) float64 {
	// antd: preset sizes all use fontSize (14); custom number uses 18 for text.
	if a.Style.FontSize > 0 {
		return a.Style.FontSize
	}
	if a.isCustomNumberSize() {
		return DefaultAvatarCustomTextFont
	}
	return th.SizeOr(core.TokenFontSize, 14)
}

func (a *Avatar) iconFontSize(th *core.Theme, edge float64) float64 {
	// custom number: size/2 (antd sizeStyle).
	if a.isCustomNumberSize() {
		return edge / 2
	}
	sz := a.Size
	if !a.sizeExplicit && a.groupHasSize {
		sz = a.groupSize
	}
	switch sz {
	case AvatarSmall:
		return th.SizeOr(core.TokenFontSize, 14)
	case AvatarLarge:
		return DefaultAvatarIconFontLG
	default:
		// middle: round((fontSizeLG+fontSizeXL)/2) ≈ 18
		lg := th.SizeOr(core.TokenFontSizeLG, 16)
		// fontSizeXL not in theme; antd ≈ 20
		return math.Round((lg + 20) / 2)
	}
}

// isCustomNumberSize reports antd number size (not small/middle/large preset).
func (a *Avatar) isCustomNumberSize() bool {
	if a == nil {
		return false
	}
	if a.resolveResponsive() > 0 {
		return true
	}
	if a.sizeExplicit {
		return a.Size == AvatarSizeCustom
	}
	if a.groupHasSize {
		return a.groupSize == AvatarSizeCustom
	}
	return a.Size == AvatarSizeCustom
}

func (a *Avatar) computeTextScale(edge, font, gap float64) float64 {
	if a.Text == "" {
		return 1
	}
	avail := edge - 2*gap
	if avail <= 0 {
		return 1
	}
	var w float64
	if a.Face != nil {
		fs := a.Face.Size()
		if fs <= 0 {
			fs = font
		}
		w = a.Face.Advance(a.Text) * (font / fs)
	} else {
		// Headless approx: 0.55em per rune (antd scale path needs face in product).
		w = float64(utf8.RuneCountInString(a.Text)) * font * 0.55
	}
	if w <= 0 || w <= avail {
		return 1
	}
	return avail / w
}

func (a *Avatar) bgColor(th *core.Theme, mode AvatarContent) render.RGBA {
	if a.Disabled {
		return th.Color(core.TokenColorDisabledBg)
	}
	if mode == AvatarContentImage {
		return render.RGBA{} // transparent
	}
	if a.Style.hasBG() {
		return a.Style.Background
	}
	// avatarBg = colorTextPlaceholder ≈ colorTextQuaternary
	c := th.Color(core.TokenColorTextQuaternary)
	if c.A < 0.05 {
		c = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	}
	return c
}

func (a *Avatar) fgColor(th *core.Theme) render.RGBA {
	if a.Disabled {
		return th.Color(core.TokenColorDisabledText)
	}
	if a.Style.hasText() {
		return a.Style.Text
	}
	// avatarColor = colorTextLightSolid
	c := th.Color(core.TokenColorTextInverse)
	if c.A < 0.5 {
		c = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	return c
}

func (a *Avatar) applyA11yName() {
	if a == nil {
		return
	}
	name := a.AriaLabel
	if name == "" {
		name = a.Alt
	}
	if name == "" {
		name = a.Text
	}
	if a.pressable != nil {
		a.pressable.Base().Label = name
		a.pressable.Base().Role = "button"
	}
	if a.Root != nil && name != "" {
		a.Root.Base().Label = name
	}
}

func (a *Avatar) rebuild() {
	if a == nil {
		return
	}
	th := a.theme()
	edge := a.resolveSize(th)
	if edge <= 0 {
		edge = 32
	}
	shape := a.resolveShape()
	mode := a.ContentMode()
	fg := a.fgColor(th)
	bg := a.bgColor(th, mode)
	gap := a.gap()
	lineW := th.SizeOr(core.TokenLineWidth, 1)

	var radius float64
	if shape == AvatarSquare {
		radius = a.squareRadius(th, edge)
	} else {
		radius = edge / 2
	}

	// Build content node.
	var content core.Node
	switch mode {
	case AvatarContentImage:
		content = a.buildImage(edge, th)
	case AvatarContentIcon:
		content = a.buildIcon(edge, th, fg)
	default:
		content = a.buildText(edge, th, fg, gap)
	}

	// Loading overlay via Stack when loading.
	var body core.Node = content
	if a.Loading {
		spinSz := edge * 0.45
		if spinSz < 10 {
			spinSz = 10
		}
		a.spinner = primitive.NewCanvas(spinSz, spinSz, a.paintSpinner)
		stack := primitive.NewStack()
		if content != nil {
			stack.AddChild(content)
		}
		stack.AddChild(primitive.Positioned(core.AlignCenter, a.spinner))
		body = stack
	} else {
		a.spinner = nil
	}

	// Clip content to circle/square bounds.
	clip := primitive.NewClip(body)
	clip.Width, clip.Height = edge, edge

	if a.Root == nil {
		a.Root = primitive.NewDecorated(clip)
	} else {
		a.Root.ClearChildren()
		a.Root.AddChild(clip)
	}
	a.Root.Width, a.Root.Height = edge, edge
	a.Root.MinWidth, a.Root.MinHeight = edge, edge
	a.Root.Radius = radius
	a.Root.Background = bg
	a.Root.BorderWidth = lineW
	if a.groupBorder {
		// groupBorderColor ≈ colorBgContainer
		bc := th.Color(core.TokenColorBgContainer)
		if bc.A < 0.5 {
			bc = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		a.Root.BorderColor = bc
	} else {
		a.Root.BorderColor = render.RGBA{} // transparent
	}
	a.Root.SetCenterContent(true)
	a.Root.StretchChild = true
	a.Root.Hit = core.HitDefer
	a.Root.SetThemeHook(func(*core.Theme) { a.rebuild() })

	if a.Interactive {
		if a.pressable == nil {
			a.pressable = primitive.NewPressable(a.Root)
		} else {
			a.pressable.ClearChildren()
			a.pressable.AddChild(a.Root)
		}
		a.pressable.Focusable = true
		a.pressable.ShowFocusRing = true
		a.pressable.FocusRingRadius = radius
		a.pressable.FocusRingOutset = 1.5
		a.pressable.SetDisabled(a.Disabled)
		a.pressable.Base().Role = "button"
		a.pressable.Click = func() {
			if a.Disabled || a.Loading {
				return
			}
			if a.OnClick != nil {
				a.OnClick()
			}
		}
	} else {
		a.pressable = nil
	}

	a.applyA11yName()
	a.Root.MarkNeedsLayout()
	a.Root.MarkNeedsPaint()
	if a.Loading {
		a.life.setActive(true)
	}
}

func (a *Avatar) buildText(edge float64, th *core.Theme, fg render.RGBA, gap float64) core.Node {
	font := a.textFontSize(th, edge)
	scale := a.computeTextScale(edge, font, gap)
	a.TextScale = scale
	eff := font * scale
	if eff < 1 {
		eff = 1
	}
	a.label = primitive.NewText(a.Text)
	a.label.FontSize = eff
	a.label.Face = a.Face
	a.label.Color = fg
	return a.label
}

func (a *Avatar) buildIcon(edge float64, th *core.Theme, fg render.RGBA) core.Node {
	a.TextScale = 1
	if a.IconNode != nil {
		return a.IconNode
	}
	isz := a.iconFontSize(th, edge)
	if a.IconName == "" {
		// Empty icon slot — keep layout stable.
		b := primitive.NewBox()
		b.Width, b.Height = isz, isz
		return b
	}
	a.icon = primitive.NewIcon(a.IconName)
	a.icon.Size = isz
	a.icon.Color = fg
	return a.icon
}

func (a *Avatar) buildImage(edge float64, th *core.Theme) core.Node {
	a.TextScale = 1
	pw, ph := a.PixelW, a.PixelH
	pix := a.Pixels
	alt := a.Alt
	if alt == "" {
		alt = a.Src
	}
	pn := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		if len(pix) >= pw*ph*4 && pw > 0 && ph > 0 {
			gx, gy := pw, ph
			if gx > 32 {
				gx = 32
			}
			if gy > 32 {
				gy = 32
			}
			cw, ch := sz.Width/float64(gx), sz.Height/float64(gy)
			for y := 0; y < gy; y++ {
				for x := 0; x < gx; x++ {
					sx := x * pw / gx
					sy := y * ph / gy
					i := (sy*pw + sx) * 4
					if i+3 >= len(pix) {
						continue
					}
					c := render.RGBA{
						R: float64(pix[i]) / 255,
						G: float64(pix[i+1]) / 255,
						B: float64(pix[i+2]) / 255,
						A: float64(pix[i+3]) / 255,
					}
					pc.FillLocalRect(float64(x)*cw, float64(y)*ch, cw+0.5, ch+0.5, c)
				}
			}
			return
		}
		// Placeholder bands when host has not injected pixels yet.
		base := render.RGBA{R: 0.90, G: 0.91, B: 0.93, A: 1}
		top := render.RGBA{R: 0.78, G: 0.84, B: 0.92, A: 1}
		pc.FillLocalRect(0, 0, sz.Width, sz.Height, base)
		pc.FillLocalRect(0, 0, sz.Width, sz.Height*0.45, top)
	})
	pn.Width, pn.Height = edge, edge
	a.imgNode = pn
	return pn
}

func (a *Avatar) paintSpinner(pc *core.PaintContext, sz core.Size) {
	if pc == nil || !a.Loading {
		return
	}
	th := a.theme()
	col := a.fgColor(th)
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
	pc.StrokeLocalCircle(cx, cy, r, stroke, track)
	start := -math.Pi/2 + a.spinPhase*2*math.Pi
	end := start + 2*math.Pi*0.7
	steps := 40
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		ang := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(ang), cy+r*math.Sin(ang))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

// ---------------------------------------------------------------------------
// Avatar.Group
// ---------------------------------------------------------------------------

// AvatarGroup is Ant Design Avatar.Group.
//
//	Row (inline-flex; Gap = groupOverlapping)
//	  └─ Avatar… (+ overflow "+N" when max.count)
type AvatarGroup struct {
	Root *primitive.Flex

	// Children avatars (product owns pointers).
	Children []*Avatar
	// MaxCount is max.count; 0 = unlimited.
	MaxCount int
	// MaxStyle styles the overflow "+N" avatar.
	MaxStyle Style
	// Size / SizePx / Shape are group context (antd AvatarContext).
	Size   AvatarSize
	SizePx float64
	Shape  AvatarShape

	Theme *core.Theme
	Face  text.Face

	// OverflowText is the last overflow label (e.g. "+1"); for tests.
	OverflowText string
	// VisibleCount is how many avatars are shown including overflow chip.
	VisibleCount int
}

// NewAvatarGroup creates a group with optional children.
func NewAvatarGroup(children ...*Avatar) *AvatarGroup {
	g := &AvatarGroup{
		Children: children,
		Size:     AvatarMiddle,
		Shape:    AvatarCircle,
	}
	g.rebuild()
	return g
}

// Node returns the group root.
func (g *AvatarGroup) Node() core.Node {
	if g == nil {
		return nil
	}
	if g.Root == nil {
		g.rebuild()
	}
	return g.Root
}

// SetChildren replaces group members.
func (g *AvatarGroup) SetChildren(children ...*Avatar) {
	if g == nil {
		return
	}
	g.Children = children
	g.rebuild()
}

// Add appends an avatar.
func (g *AvatarGroup) Add(a *Avatar) {
	if g == nil || a == nil {
		return
	}
	g.Children = append(g.Children, a)
	g.rebuild()
}

// SetMaxCount sets max.count (0 = no limit).
func (g *AvatarGroup) SetMaxCount(n int) {
	if g == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	g.MaxCount = n
	g.rebuild()
}

// SetMaxStyle styles the overflow avatar.
func (g *AvatarGroup) SetMaxStyle(st Style) {
	if g == nil {
		return
	}
	g.MaxStyle = st
	g.rebuild()
}

// SetSize sets group size context.
func (g *AvatarGroup) SetSize(sz AvatarSize) {
	if g == nil {
		return
	}
	g.Size = sz
	if sz != AvatarSizeCustom {
		g.SizePx = 0
	}
	g.rebuild()
}

// SetSizePx sets custom group size.
func (g *AvatarGroup) SetSizePx(px float64) {
	if g == nil {
		return
	}
	g.Size = AvatarSizeCustom
	g.SizePx = px
	g.rebuild()
}

// SetShape sets group shape context.
func (g *AvatarGroup) SetShape(sh AvatarShape) {
	if g == nil {
		return
	}
	g.Shape = sh
	g.rebuild()
}

// SetFace propagates face to children / overflow.
func (g *AvatarGroup) SetFace(face text.Face) {
	if g == nil {
		return
	}
	g.Face = face
	g.rebuild()
}

// SetTheme sets theme override.
func (g *AvatarGroup) SetTheme(th *core.Theme) {
	if g == nil {
		return
	}
	g.Theme = th
	g.rebuild()
}

func (g *AvatarGroup) theme() *core.Theme {
	var n core.Node
	if g.Root != nil {
		n = g.Root
	}
	return themeOf(g.Theme, n)
}

func (g *AvatarGroup) rebuild() {
	if g == nil {
		return
	}
	th := g.theme()
	_ = th

	kids := g.Children
	n := len(kids)
	show := kids
	var overflow *Avatar
	g.OverflowText = ""
	if g.MaxCount > 0 && n > g.MaxCount {
		show = kids[:g.MaxCount]
		hidden := n - g.MaxCount
		g.OverflowText = fmt.Sprintf("+%d", hidden)
		overflow = NewAvatar(g.OverflowText)
		if g.Face != nil {
			overflow.SetFace(g.Face)
		}
		if g.Theme != nil {
			overflow.Theme = g.Theme
		}
		if g.MaxStyle.hasBG() || g.MaxStyle.hasText() || g.MaxStyle.FontSize > 0 {
			overflow.SetStyle(g.MaxStyle)
		}
	}

	// Apply group context to visible + overflow.
	for _, a := range show {
		if a == nil {
			continue
		}
		if g.Face != nil && a.Face == nil {
			a.Face = g.Face
		}
		a.applyGroupContext(g.Size, g.SizePx, g.Shape, true)
	}
	if overflow != nil {
		overflow.applyGroupContext(g.Size, g.SizePx, g.Shape, true)
	}

	nodes := make([]core.Node, 0, len(show)+1)
	for _, a := range show {
		if a != nil {
			nodes = append(nodes, a.Node())
		}
	}
	if overflow != nil {
		nodes = append(nodes, overflow.Node())
	}
	g.VisibleCount = len(nodes)

	if g.Root == nil {
		g.Root = primitive.Row()
	} else {
		g.Root.ClearChildren()
	}
	for _, n := range nodes {
		g.Root.AddChild(n)
	}
	// antd: marginInlineStart = groupOverlapping on not-first-child → negative Gap.
	g.Root.Gap = DefaultAvatarGroupOverlapping
	g.Root.CrossAlign = core.CrossCenter
	g.Root.MainAlign = core.MainStart
	g.Root.Hit = core.HitDefer
	g.Root.MarkNeedsLayout()
	g.Root.MarkNeedsPaint()
}
