package kit

import (
	"fmt"
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Badge component tokens — prepareComponentToken / prepareToken.
// docs/antd/badge.md §6.2
const (
	// DefaultBadgeOverflowCount is antd overflowCount default.
	DefaultBadgeOverflowCount = 99
	// DefaultBadgeIndicatorHeight is medium count height (round(14×1.5714)−2).
	DefaultBadgeIndicatorHeight = 20.0
	// DefaultBadgeIndicatorHeightSM is small count height (= fontSize).
	DefaultBadgeIndicatorHeightSM = 14.0
	// DefaultBadgeDotSize is red-dot size (fontSizeSM/2).
	DefaultBadgeDotSize = 6.0
	// DefaultBadgeStatusSize is status-dot size (fontSizeSM/2).
	DefaultBadgeStatusSize = 6.0
	// DefaultBadgeTextFontSize is count text size (fontSizeSM).
	DefaultBadgeTextFontSize = 12.0
	// DefaultBadgePaddingInline is multi-digit horizontal padding (antd paddingXS=8).
	DefaultBadgePaddingInline = 8.0
	// badgeProcessingRPS is processing pulse cycles per second (≈1.2s period).
	badgeProcessingRPS = 1.0 / 1.2
)

// BadgeSize is count indicator size (antd size medium|small).
type BadgeSize int

const (
	// BadgeMedium is the default count height (20).
	BadgeMedium BadgeSize = iota
	// BadgeSmall is the compact count height (14).
	BadgeSmall
)

// BadgeStatus is the standalone / overlay status-dot mode.
type BadgeStatus int

const (
	// BadgeStatusNone means no status-dot mode.
	BadgeStatusNone BadgeStatus = iota
	// BadgeStatusSuccess is green.
	BadgeStatusSuccess
	// BadgeStatusProcessing is primary + Ticker pulse.
	BadgeStatusProcessing
	// BadgeStatusDefault is neutral.
	BadgeStatusDefault
	// BadgeStatusError is error red.
	BadgeStatusError
	// BadgeStatusWarning is warning orange.
	BadgeStatusWarning
)

// RibbonPlacement is Badge.Ribbon placement (start|end).
type RibbonPlacement int

const (
	// RibbonEnd is the default (logical end / top-right in LTR).
	RibbonEnd RibbonPlacement = iota
	// RibbonStart is logical start (top-left in LTR).
	RibbonStart
)

// Badge is Ant Design Badge (data display).
//
//	Stack host
//	  ├─ child? (wrapper)
//	  └─ indicator (count | dot | status-dot) half-out top-right + offset
//	or status row (standalone status + text)
//
// Product contract: docs/antd/badge.md §6 (P0 DoD).
type Badge struct {
	host      *badgeHost
	pressable *primitive.Pressable

	// indicator is the last built mark chrome (count/dot/status); exported via IndicatorNode.
	indicator core.Node
	// markLab is the count text node when numeric.
	markLab *primitive.Text
	// processRing is the processing pulse canvas (status=processing).
	processRing *primitive.Canvas

	child     core.Node
	Count     int
	countSet  bool // true after SetCount; distinguishes null vs 0 (antd)
	CountNode core.Node
	ShowZero  bool
	// OverflowCount defaults to 99; ≤0 falls back to 99.
	OverflowCount int
	Dot           bool
	Size          BadgeSize
	OffsetX       float64
	OffsetY       float64
	offsetSet     bool
	Status        BadgeStatus
	Text          string
	// ColorHex custom color string (empty = theme default).
	ColorHex  string
	ColorRGBA render.RGBA
	colorSet  bool
	// Title is hover / a11y name; titleNone means explicit null/false.
	Title     string
	titleSet  bool
	titleNone bool

	Disabled  bool
	OnClick   func()
	AriaLabel string

	Face  text.Face
	Theme *core.Theme
	Style Style

	// lastMarkOff is the laid-out indicator top-left (for tests).
	lastMarkOff core.Point
	// lastMarkSz is the laid-out indicator size.
	lastMarkSz core.Size
	// halfOut is true when indicator uses translate(50%,-50%) placement.
	halfOut bool

	pulsePhase float64
	life       tickerLifecycle
	boundTree  *core.Tree
}

// NewBadge creates a Badge with antd defaults (overflow=99, size=medium).
func NewBadge() *Badge {
	b := &Badge{
		OverflowCount: DefaultBadgeOverflowCount,
		Size:          BadgeMedium,
	}
	b.rebuild()
	return b
}

// Node returns the mount root (Pressable when clickable, else host).
func (b *Badge) Node() core.Node {
	if b == nil {
		return nil
	}
	if b.host == nil {
		b.rebuild()
	}
	if b.OnClick != nil && b.pressable != nil {
		return b.pressable
	}
	return b.host
}

// ChromeNode returns the layout host (Stack-like).
func (b *Badge) ChromeNode() core.Node {
	if b == nil {
		return nil
	}
	if b.host == nil {
		b.rebuild()
	}
	return b.host
}

// IndicatorNode returns the count/dot/status mark node (may be nil when hidden).
func (b *Badge) IndicatorNode() core.Node {
	if b == nil {
		return nil
	}
	return b.indicator
}

// DisplayCount returns the resolved count label ("5", "99+", "0") or "" when not numeric.
func (b *Badge) DisplayCount() string {
	if b == nil || b.Dot || b.CountNode != nil {
		return ""
	}
	if !b.countRenderable() {
		return ""
	}
	return b.formatCount()
}

// Visible reports whether any indicator / status chrome is shown.
func (b *Badge) Visible() bool {
	if b == nil {
		return false
	}
	return b.resolveMode() != badgeModeHidden
}

// IsDot reports visible red-dot mode (antd showAsDot).
func (b *Badge) IsDot() bool {
	if b == nil {
		return false
	}
	return b.showAsDot() && b.Visible()
}

// IndicatorHeight returns the count capsule height for the current size (0 if not count).
func (b *Badge) IndicatorHeight() float64 {
	if b == nil {
		return 0
	}
	if b.Dot || b.CountNode != nil {
		if b.Dot {
			return DefaultBadgeDotSize
		}
		return 0
	}
	if !b.countRenderable() && b.resolveMode() != badgeModeStatus {
		return 0
	}
	if b.Size == BadgeSmall {
		return DefaultBadgeIndicatorHeightSM
	}
	return DefaultBadgeIndicatorHeight
}

// StatusColor returns the resolved status/dot/count fill color.
func (b *Badge) StatusColor() render.RGBA {
	if b == nil {
		return render.RGBA{}
	}
	return b.resolveIndicatorColor(b.theme())
}

// ResolvedTitle returns title after antd fallback rules.
func (b *Badge) ResolvedTitle() string {
	if b == nil {
		return ""
	}
	if b.titleNone {
		return ""
	}
	if b.titleSet {
		return b.Title
	}
	if s := b.DisplayCount(); s != "" {
		return s
	}
	if b.Text != "" {
		return b.Text
	}
	return b.AriaLabel
}

// MarkOffset returns the last laid-out indicator top-left relative to the host.
func (b *Badge) MarkOffset() core.Point {
	if b == nil {
		return core.Point{}
	}
	return b.lastMarkOff
}

// MarkSize returns the last laid-out indicator size.
func (b *Badge) MarkSize() core.Size {
	if b == nil {
		return core.Size{}
	}
	return b.lastMarkSz
}

// SetChild sets the wrapped children (nil = standalone / not-a-wrapper).
func (b *Badge) SetChild(n core.Node) {
	if b == nil {
		return
	}
	b.child = n
	b.rebuild()
}

// SetCount sets the numeric count (marks count as explicitly set).
func (b *Badge) SetCount(n int) {
	if b == nil {
		return
	}
	b.Count = n
	b.countSet = true
	b.rebuild()
}

// SetCountNode sets a custom count node (wins over numeric label).
func (b *Badge) SetCountNode(n core.Node) {
	if b == nil {
		return
	}
	b.CountNode = n
	if n != nil {
		b.countSet = true
	}
	b.rebuild()
}

// SetShowZero toggles showing zero counts.
func (b *Badge) SetShowZero(v bool) {
	if b == nil {
		return
	}
	b.ShowZero = v
	b.rebuild()
}

// SetOverflowCount sets the overflow threshold (≤0 → 99).
func (b *Badge) SetOverflowCount(n int) {
	if b == nil {
		return
	}
	if n <= 0 {
		n = DefaultBadgeOverflowCount
	}
	b.OverflowCount = n
	b.rebuild()
}

// SetDot toggles red-dot mode.
func (b *Badge) SetDot(dot bool) {
	if b == nil {
		return
	}
	b.Dot = dot
	b.rebuild()
}

// SetOffset sets antd offset [x, y] from the default half-out anchor.
func (b *Badge) SetOffset(x, y float64) {
	if b == nil {
		return
	}
	b.OffsetX, b.OffsetY = x, y
	b.offsetSet = true
	b.rebuild()
}

// SetSize sets medium | small (count height).
func (b *Badge) SetSize(sz BadgeSize) {
	if b == nil {
		return
	}
	b.Size = sz
	b.rebuild()
}

// SetStatus sets the status-dot mode.
func (b *Badge) SetStatus(st BadgeStatus) {
	if b == nil {
		return
	}
	b.Status = st
	b.rebuild()
	b.life.setActive(st == BadgeStatusProcessing)
}

// SetText sets status-dot side text.
func (b *Badge) SetText(s string) {
	if b == nil {
		return
	}
	b.Text = s
	b.rebuild()
}

// SetColor sets a custom hex color (empty clears).
func (b *Badge) SetColor(hex string) {
	if b == nil {
		return
	}
	hex = strings.TrimSpace(hex)
	b.ColorHex = hex
	if hex == "" {
		b.colorSet = false
		b.ColorRGBA = render.RGBA{}
	} else {
		b.ColorRGBA = render.Hex(hex)
		b.colorSet = b.ColorRGBA.A > 0
	}
	b.rebuild()
}

// SetColorRGBA sets a custom solid color.
func (b *Badge) SetColorRGBA(c render.RGBA) {
	if b == nil {
		return
	}
	b.ColorRGBA = c
	b.colorSet = c.A > 0
	if !b.colorSet {
		b.ColorHex = ""
	}
	b.rebuild()
}

// SetTitle sets the hover / a11y title string.
func (b *Badge) SetTitle(s string) {
	if b == nil {
		return
	}
	b.Title = s
	b.titleSet = true
	b.titleNone = false
	b.applyA11y()
}

// SetTitleNone clears title and disables count fallback (antd title=null|false).
func (b *Badge) SetTitleNone() {
	if b == nil {
		return
	}
	b.Title = ""
	b.titleSet = true
	b.titleNone = true
	b.applyA11y()
}

// SetOnClick enables clickable mode (link demo).
func (b *Badge) SetOnClick(fn func()) {
	if b == nil {
		return
	}
	b.OnClick = fn
	b.rebuild()
}

// SetDisabled dims chrome and blocks clicks.
func (b *Badge) SetDisabled(v bool) {
	if b == nil {
		return
	}
	b.Disabled = v
	if b.pressable != nil {
		b.pressable.SetDisabled(v)
	}
	b.rebuild()
}

// SetTheme sets the theme override.
func (b *Badge) SetTheme(th *core.Theme) {
	if b == nil {
		return
	}
	b.Theme = th
	b.rebuild()
}

// SetStyle applies Style overrides (Background → count fill).
func (b *Badge) SetStyle(st Style) {
	if b == nil {
		return
	}
	b.Style = st
	if st.Face != nil {
		b.Face = st.Face
	}
	b.rebuild()
}

// SetFace sets the count text face.
func (b *Badge) SetFace(face text.Face) {
	if b == nil {
		return
	}
	b.Face = face
	if b.markLab != nil {
		b.markLab.Face = face
		b.markLab.MarkNeedsLayout()
		b.markLab.MarkNeedsPaint()
	}
	b.rebuild()
}

// SetAriaLabel sets the accessible name.
func (b *Badge) SetAriaLabel(name string) {
	if b == nil {
		return
	}
	b.AriaLabel = name
	b.applyA11y()
}

// AttachTicker binds processing pulse to the tree.
func (b *Badge) AttachTicker(t *core.Tree) {
	if b == nil || t == nil {
		return
	}
	b.boundTree = t
	b.life.attach(t, b, b.Status == BadgeStatusProcessing)
}

// Tick advances the processing pulse ring.
func (b *Badge) Tick(dt float64) bool {
	if b == nil {
		return false
	}
	if !b.life.stillMounted(b.boundTree) {
		return false
	}
	if b.Status != BadgeStatusProcessing {
		return false
	}
	b.pulsePhase += dt * badgeProcessingRPS
	if b.pulsePhase > 1 {
		b.pulsePhase = math.Mod(b.pulsePhase, 1)
	}
	if b.processRing != nil {
		b.processRing.MarkNeedsPaint()
	} else if b.host != nil {
		b.host.MarkNeedsPaint()
	}
	return true
}

// ── internals ──────────────────────────────────────────────────────────────

type badgeMode int

const (
	badgeModeHidden badgeMode = iota
	badgeModeWrapper
	badgeModeStandaloneCount
	badgeModeStatus
)

func (b *Badge) theme() *core.Theme {
	var n core.Node
	if b.host != nil {
		n = b.host
	}
	return themeOf(b.Theme, n)
}

func (b *Badge) overflow() int {
	if b == nil || b.OverflowCount <= 0 {
		return DefaultBadgeOverflowCount
	}
	return b.OverflowCount
}

func (b *Badge) countRenderable() bool {
	if b == nil {
		return false
	}
	if b.CountNode != nil {
		return true
	}
	if !b.countSet {
		return false
	}
	if b.Count > 0 {
		return true
	}
	return b.ShowZero // explicit 0
}

func (b *Badge) formatCount() string {
	ov := b.overflow()
	if b.Count > ov {
		return fmt.Sprintf("%d+", ov)
	}
	return fmt.Sprintf("%d", b.Count)
}

func (b *Badge) hasStatusOrColor() bool {
	if b == nil {
		return false
	}
	return b.Status != BadgeStatusNone || b.colorSet || b.Style.hasBG()
}

// isZeroCount mirrors antd isZero for numeric count (null count is not zero).
func (b *Badge) isZeroCount() bool {
	if b == nil || b.CountNode != nil || !b.countSet {
		return false
	}
	return b.Count == 0
}

// showAsDot mirrors antd: dot && !isZero.
func (b *Badge) showAsDot() bool {
	return b != nil && b.Dot && !b.isZeroCount()
}

// resolveMode mirrors antd Badge display branches.
func (b *Badge) resolveMode() badgeMode {
	if b == nil {
		return badgeModeHidden
	}
	if b.showAsDot() || b.countRenderable() {
		if b.child != nil {
			return badgeModeWrapper
		}
		return badgeModeStandaloneCount
	}
	// count ignored: null, or zero without showZero
	ignoreCount := !b.countSet || (b.isZeroCount() && !b.ShowZero)
	hasStatus := b.hasStatusOrColor() && ignoreCount
	if !hasStatus {
		return badgeModeHidden
	}
	// antd early status branch: !children && hasStatus && (text || hasStatusValue || !ignoreCount)
	hasStatusValue := b.Status != BadgeStatusNone || !b.isZeroCount()
	showStatusText := b.Text != ""
	if b.child == nil {
		if showStatusText || hasStatusValue || !ignoreCount {
			return badgeModeStatus
		}
		return badgeModeHidden
	}
	// status/color overlay on children
	return badgeModeWrapper
}

func (b *Badge) resolveIndicatorColor(th *core.Theme) render.RGBA {
	if b.Style.hasBG() {
		return b.Style.Background
	}
	if b.colorSet {
		return b.ColorRGBA
	}
	switch b.Status {
	case BadgeStatusSuccess:
		return th.Color(core.TokenColorSuccess)
	case BadgeStatusError:
		return th.Color(core.TokenColorError)
	case BadgeStatusWarning:
		return th.Color(core.TokenColorWarning)
	case BadgeStatusProcessing:
		c := th.Color(core.TokenColorPrimary)
		if c.A < 0.1 {
			c = render.Hex("#1677FF")
		}
		return c
	case BadgeStatusDefault:
		c := th.Color(core.TokenColorTextQuaternary)
		if c.A < 0.05 {
			c = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		}
		return c
	default:
		c := th.Color(core.TokenColorError)
		if c.A < 0.1 {
			c = render.Hex("#FF4D4F")
		}
		return c
	}
}

func (b *Badge) applyA11y() {
	if b == nil {
		return
	}
	name := b.AriaLabel
	if name == "" {
		name = b.ResolvedTitle()
	}
	_ = name // reserved for host a11y bridge; Pressable uses Aria when wired
	if b.pressable != nil {
		// Pressable has no AriaLabel field in all versions; keep field for tests/host.
	}
}

func (b *Badge) rebuild() {
	if b == nil {
		return
	}
	th := b.theme()
	if b.host == nil {
		b.host = &badgeHost{b: b}
		b.host.Init(b.host)
		b.host.Hit = core.HitDefer
	}
	b.host.ClearChildren()
	b.indicator = nil
	b.markLab = nil
	b.processRing = nil
	b.halfOut = false

	mode := b.resolveMode()
	switch mode {
	case badgeModeWrapper:
		if b.child != nil {
			b.host.AddChild(b.child)
		}
		if mark := b.buildMark(th, true); mark != nil {
			b.indicator = mark
			b.halfOut = true
			b.host.AddChild(mark)
		}
	case badgeModeStandaloneCount:
		if mark := b.buildMark(th, false); mark != nil {
			b.indicator = mark
			b.host.AddChild(mark)
		}
	case badgeModeStatus:
		row := b.buildStatusRow(th)
		if row != nil {
			b.indicator = row
			b.host.AddChild(row)
		}
	case badgeModeHidden:
		// empty host; still layout to zero/min
	}

	// Clickable shell
	if b.OnClick != nil {
		if b.pressable == nil {
			b.pressable = primitive.NewPressable(b.host)
			b.pressable.EnableRipple = false
		} else {
			b.pressable.ClearChildren()
			b.pressable.AddChild(b.host)
		}
		b.pressable.Click = func() {
			if b.Disabled || b.OnClick == nil {
				return
			}
			b.OnClick()
		}
		b.pressable.SetDisabled(b.Disabled)
		b.pressable.ShowFocusRing = true
		b.pressable.Focusable = !b.Disabled
	} else {
		b.pressable = nil
	}
	b.applyA11y()
	b.host.MarkNeedsLayout()
	b.host.MarkNeedsPaint()
}

func (b *Badge) buildMark(th *core.Theme, withBorder bool) core.Node {
	showDot := b.showAsDot()

	// status-dot overlay when no count/dot but status/color with children
	if !showDot && !b.countRenderable() && b.hasStatusOrColor() {
		return b.buildStatusDot(th)
	}

	col := b.resolveIndicatorColor(th)
	if b.Disabled {
		col = dimColor(col, th)
	}

	if showDot {
		d := primitive.NewBox()
		d.Width, d.Height = DefaultBadgeDotSize, DefaultBadgeDotSize
		d.Color = col
		if withBorder {
			dec := primitive.NewDecorated(nil)
			lw := th.SizeOr(core.TokenLineWidth, 1)
			dec.BorderWidth = lw
			dec.BorderColor = th.Color(core.TokenColorBgContainer)
			dec.Radius = DefaultBadgeDotSize
			dec.Background = col
			dec.Width = DefaultBadgeDotSize
			dec.Height = DefaultBadgeDotSize
			return dec
		}
		return d
	}

	// Custom count node
	if b.CountNode != nil {
		return b.CountNode
	}

	// Numeric count capsule
	txt := b.formatCount()
	lab := primitive.NewText(txt)
	lab.FontSize = DefaultBadgeTextFontSize
	lab.Face = b.Face
	lab.Color = th.Color(core.TokenColorTextInverse)
	if lab.Color.A < 0.5 {
		lab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	if b.Style.hasText() {
		lab.Color = b.Style.Text
	}
	if b.Disabled {
		lab.Color = th.Color(core.TokenColorDisabledText)
	}
	b.markLab = lab

	h := DefaultBadgeIndicatorHeight
	if b.Size == BadgeSmall {
		h = DefaultBadgeIndicatorHeightSM
	}
	padX := DefaultBadgePaddingInline / 2
	// single digit: tighter (antd minWidth = height, padding for multi)
	if len(txt) <= 1 {
		padX = 0
	}
	dec := primitive.NewDecorated(lab)
	dec.Padding = primitive.Symmetric(padX, 0)
	dec.Radius = h / 2
	dec.Background = col
	// Force min size via min box under decorated — use MinWidth/Height if available
	dec.MinWidth = h
	dec.MinHeight = h
	if withBorder {
		lw := th.SizeOr(core.TokenLineWidth, 1)
		dec.BorderWidth = lw
		dec.BorderColor = th.Color(core.TokenColorBgContainer)
	}
	return dec
}

func (b *Badge) buildStatusDot(th *core.Theme) core.Node {
	col := b.resolveIndicatorColor(th)
	if b.Disabled {
		col = dimColor(col, th)
	}
	sz := DefaultBadgeStatusSize
	dot := primitive.NewBox()
	dot.Width, dot.Height = sz, sz
	dot.Color = col

	if b.Status != BadgeStatusProcessing {
		return dot
	}

	// processing: stack pulse ring under/over the solid dot
	ringSz := sz * 2.5
	b.processRing = primitive.NewCanvas(ringSz, ringSz, b.paintProcessing)
	stack := primitive.NewStack(
		primitive.Positioned(core.AlignCenter, b.processRing),
		primitive.Positioned(core.AlignCenter, dot),
	)
	stack.Fit = false
	// size to ring
	wrap := primitive.NewBox(stack)
	wrap.Width, wrap.Height = ringSz, ringSz
	wrap.Color = render.RGBA{}
	return wrap
}

func (b *Badge) buildStatusRow(th *core.Theme) core.Node {
	dot := b.buildStatusDot(th)
	if b.Text == "" {
		return dot
	}
	lab := primitive.NewText(b.Text)
	lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
	lab.Face = b.Face
	lab.Color = th.Color(core.TokenColorText)
	if b.Disabled {
		lab.Color = th.Color(core.TokenColorDisabledText)
	}
	gap := th.SizeOr(core.TokenMarginXS, 4)
	row := primitive.Row(dot, lab)
	row.Gap = gap
	row.CrossAlign = core.CrossCenter
	return row
}

func (b *Badge) paintProcessing(pc *core.PaintContext, sz core.Size) {
	if b == nil || pc == nil || pc.DC == nil {
		return
	}
	th := b.theme()
	col := b.resolveIndicatorColor(th)
	// pulse: scale 1→2, alpha 0.6→0
	t := b.pulsePhase
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = math.Mod(t, 1)
	}
	scale := 1 + t
	alpha := col.A * (1 - t) * 0.65
	if alpha < 0 {
		alpha = 0
	}
	cx := pc.Origin.X + sz.Width/2
	cy := pc.Origin.Y + sz.Height/2
	base := DefaultBadgeStatusSize / 2
	r := base * scale
	dc := pc.DC
	dc.SetLineWidth(th.SizeOr(core.TokenLineWidth, 1))
	dc.SetRGBA(col.R, col.G, col.B, alpha)
	dc.DrawCircle(cx, cy, r)
	_ = dc.Stroke()
}

func dimColor(c render.RGBA, th *core.Theme) render.RGBA {
	dis := th.Color(core.TokenColorDisabledText)
	if dis.A < 0.05 {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A * 0.35}
	}
	// blend toward disabled
	return render.RGBA{
		R: c.R*0.4 + dis.R*0.6,
		G: c.G*0.4 + dis.G*0.6,
		B: c.B*0.4 + dis.B*0.6,
		A: math.Max(c.A*0.5, dis.A),
	}
}

// badgeHost sizes to content / child and places the indicator half-out top-right.
type badgeHost struct {
	core.NodeBase
	b *Badge
}

func (h *badgeHost) TypeID() string { return "kit.Badge" }

func (h *badgeHost) Layout(c core.Constraints) core.Size {
	if h == nil {
		return core.Size{}
	}
	kids := h.Children()
	b := h.b
	if b == nil || len(kids) == 0 {
		out := c.Tighten(core.Size{})
		h.SetSize(out)
		return out
	}

	// Identify content vs mark
	var content, mark core.Node
	if b.halfOut && len(kids) >= 2 {
		content = kids[0]
		mark = kids[1]
	} else if b.halfOut && len(kids) == 1 && b.child == nil {
		mark = kids[0]
	} else {
		// standalone / status: single child fills
		content = kids[0]
		if len(kids) > 1 {
			mark = kids[1]
		}
	}

	var contentSz core.Size
	if content != nil {
		contentSz = content.Layout(core.Constraints{
			MaxWidth:  c.MaxWidth,
			MaxHeight: c.MaxHeight,
		})
		content.Base().SetOffset(core.Point{})
	}

	var markSz core.Size
	if mark != nil {
		markSz = mark.Layout(core.Constraints{
			MaxWidth:  c.MaxWidth,
			MaxHeight: c.MaxHeight,
		})
	}

	// Host size
	out := contentSz
	if content == nil {
		out = markSz
	}
	// Ensure host at least fits non-overflowing standalone
	if !b.halfOut && mark != nil && content == nil {
		out = markSz
	}
	out = c.Tighten(out)
	if c.IsTight() {
		out = core.Size{Width: c.MaxWidth, Height: c.MaxHeight}
	}
	h.SetSize(out)

	if mark != nil {
		var off core.Point
		if b.halfOut && content != nil {
			// antd: top-right + translate(50%, -50%) + offset
			ox, oy := 0.0, 0.0
			if b.offsetSet {
				ox, oy = b.OffsetX, b.OffsetY
			}
			off = core.Point{
				X: out.Width - markSz.Width/2 + ox,
				Y: -markSz.Height/2 + oy,
			}
		} else if b.offsetSet && content != nil {
			// fallback top-right + offset without half-out
			off = core.Point{
				X: out.Width - markSz.Width + b.OffsetX,
				Y: b.OffsetY,
			}
		} else if content != nil && mark != content {
			off = core.Point{X: out.Width - markSz.Width, Y: 0}
		} else {
			off = core.Point{}
		}
		mark.Base().SetOffset(off)
		b.lastMarkOff = off
		b.lastMarkSz = markSz
	} else {
		b.lastMarkOff = core.Point{}
		b.lastMarkSz = core.Size{}
	}

	// extra kids (rare)
	for i, k := range kids {
		if k == content || k == mark {
			continue
		}
		sz := k.Layout(core.Constraints{MaxWidth: c.MaxWidth, MaxHeight: c.MaxHeight})
		k.Base().SetOffset(core.Point{X: out.Width - sz.Width, Y: 0})
		_ = i
	}
	return out
}

func (h *badgeHost) Paint(pc *core.PaintContext) { h.DefaultPaintChildren(pc) }

func (h *badgeHost) HitTest(pt core.Point) core.Node { return h.DefaultHitTest(pt) }

// ── Ribbon (Badge.Ribbon) ──────────────────────────────────────────────────

// Ribbon is Ant Design Badge.Ribbon.
type Ribbon struct {
	Root      *primitive.Stack
	band      *primitive.Decorated
	label     *primitive.Text
	child     core.Node
	Text      string
	ColorHex  string
	ColorRGBA render.RGBA
	colorSet  bool
	Placement RibbonPlacement
	Face      text.Face
	Theme     *core.Theme
	Style     Style
}

// NewRibbon creates a ribbon with the given text (placement=end).
func NewRibbon(text string) *Ribbon {
	r := &Ribbon{Text: text, Placement: RibbonEnd}
	r.rebuild()
	return r
}

// Node returns the ribbon root.
func (r *Ribbon) Node() core.Node {
	if r == nil {
		return nil
	}
	if r.Root == nil {
		r.rebuild()
	}
	return r.Root
}

// SetChild sets the wrapped content.
func (r *Ribbon) SetChild(n core.Node) {
	if r == nil {
		return
	}
	r.child = n
	r.rebuild()
}

// SetText sets ribbon label text.
func (r *Ribbon) SetText(s string) {
	if r == nil {
		return
	}
	r.Text = s
	r.rebuild()
}

// SetColor sets ribbon color from hex.
func (r *Ribbon) SetColor(hex string) {
	if r == nil {
		return
	}
	hex = strings.TrimSpace(hex)
	r.ColorHex = hex
	if hex == "" {
		r.colorSet = false
		r.ColorRGBA = render.RGBA{}
	} else {
		r.ColorRGBA = render.Hex(hex)
		r.colorSet = r.ColorRGBA.A > 0
	}
	r.rebuild()
}

// SetColorRGBA sets ribbon solid color.
func (r *Ribbon) SetColorRGBA(c render.RGBA) {
	if r == nil {
		return
	}
	r.ColorRGBA = c
	r.colorSet = c.A > 0
	r.rebuild()
}

// SetPlacement sets start | end.
func (r *Ribbon) SetPlacement(p RibbonPlacement) {
	if r == nil {
		return
	}
	r.Placement = p
	r.rebuild()
}

// SetTheme sets theme override.
func (r *Ribbon) SetTheme(th *core.Theme) {
	if r == nil {
		return
	}
	r.Theme = th
	r.rebuild()
}

// SetFace sets label face.
func (r *Ribbon) SetFace(face text.Face) {
	if r == nil {
		return
	}
	r.Face = face
	r.rebuild()
}

// SetStyle applies style overrides.
func (r *Ribbon) SetStyle(st Style) {
	if r == nil {
		return
	}
	r.Style = st
	if st.Face != nil {
		r.Face = st.Face
	}
	r.rebuild()
}

// BandNode returns the ribbon band chrome.
func (r *Ribbon) BandNode() core.Node {
	if r == nil {
		return nil
	}
	return r.band
}

func (r *Ribbon) theme() *core.Theme {
	var n core.Node
	if r.Root != nil {
		n = r.Root
	}
	return themeOf(r.Theme, n)
}

func (r *Ribbon) resolveColor(th *core.Theme) render.RGBA {
	if r.Style.hasBG() {
		return r.Style.Background
	}
	if r.colorSet {
		return r.ColorRGBA
	}
	c := th.Color(core.TokenColorPrimary)
	if c.A < 0.1 {
		c = render.Hex("#1677FF")
	}
	return c
}

func (r *Ribbon) rebuild() {
	if r == nil {
		return
	}
	th := r.theme()
	if r.Root == nil {
		r.Root = primitive.NewStack()
		r.Root.Fit = true
	}
	r.Root.ClearChildren()

	if r.child != nil {
		r.Root.AddChild(r.child)
	}

	lab := primitive.NewText(r.Text)
	lab.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
	lab.Face = r.Face
	lab.Color = th.Color(core.TokenColorTextInverse)
	if lab.Color.A < 0.5 {
		lab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	if r.Style.hasText() {
		lab.Color = r.Style.Text
	}
	r.label = lab

	band := primitive.NewDecorated(lab)
	band.Padding = primitive.Symmetric(8, 2)
	band.Radius = 2
	band.Background = r.resolveColor(th)
	r.band = band

	align := core.AlignTopRight
	if r.Placement == RibbonStart {
		align = core.AlignTopLeft
	}
	r.Root.AddChild(primitive.Positioned(align, band))
	r.Root.MarkNeedsLayout()
	r.Root.MarkNeedsPaint()
}
