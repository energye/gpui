package kit

import (
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Spin geometry — components/spin/style + Indicator.
// Seeds: controlHeight=32, controlHeightLG=40.
const (
	DefaultSpinDotSize   = 20.0 // controlHeightLG / 2 · medium
	DefaultSpinDotSizeSM = 14.0 // controlHeightLG * 0.35 · small
	DefaultSpinDotSizeLG = 32.0 // controlHeight · large
	DefaultSpinGap       = 8.0  // paddingSM between indicator & description
	DefaultSpinFontSize  = 14.0 // fontSize
	DefaultSpinContentH  = 400.0
	// spinRPS is revolutions per second (antd antRotate 1.2s → ~0.833).
	spinRPS = 1.0 / 1.2
	// spinAutoInterval is percent="auto" step period (usePercent AUTO_INTERVAL).
	spinAutoInterval = 0.2
	// Nested spinning mask alpha over colorBgContainer.
	spinNestedMaskA = 0.4
)

// SpinSize is antd size: small | medium | large ("default" maps to medium).
type SpinSize string

const (
	SpinSizeSmall  SpinSize = "small"
	SpinSizeMedium SpinSize = "medium"
	SpinSizeLarge  SpinSize = "large"
)

// SpinClassNames holds shallow semantic class tags (antd SemanticDOM).
type SpinClassNames struct {
	Root        string
	Section     string
	Indicator   string
	Description string
	Container   string
}

// SpinStyles holds shallow semantic style overrides.
type SpinStyles struct {
	Root        Style
	Section     Style
	Indicator   Style
	Description Style
	Container   Style
}

// Package-level default indicator (antd Spin.setDefaultIndicator).
var defaultSpinIndicator core.Node

// SetDefaultIndicator sets the global default Spin indicator node.
func SetDefaultIndicator(n core.Node) { defaultSpinIndicator = n }

// DefaultSpinIndicator returns the global default indicator (may be nil).
func DefaultSpinIndicator() core.Node { return defaultSpinIndicator }

// Spin is Ant Design Spin — page/block loading indicator.
//
//	spinHost (RepaintBoundary)
//	  simple: section(indicator + description?)
//	  nested: stack{ container(children), mask?, section? }
//
// Ticker drives delay reveal, 4-dot / ring rotation, and percent=auto.
type Spin struct {
	Root *spinHost

	// Spinning is the props spinning flag (antd spinning). Default true.
	Spinning bool
	// Delay is reveal delay in milliseconds (antd delay). 0 = immediate.
	Delay float64
	// Size small|medium|large. Empty → medium.
	Size SpinSize
	// Description is the tip text under the indicator (antd description).
	Description string
	// Tip is the deprecated antd tip alias; merged as Description when Description empty.
	Tip string
	// Percent is numeric progress 0..100 when percentSet && !PercentAuto.
	Percent float64
	// PercentAuto enables percent="auto" mock progress.
	PercentAuto bool
	percentSet  bool

	// Indicator is a custom indicator node (instance-level).
	Indicator core.Node
	// content is nested children (nil → simple mode).
	content core.Node

	// Fullscreen is P1 (API reserved; not implemented this stage).
	Fullscreen bool

	ClassNames SpinClassNames
	Styles     SpinStyles
	Style      Style
	Theme      *core.Theme
	AriaLabel  string

	// displaySpinning is the delayed visible state.
	displaySpinning bool
	delayAcc        float64
	waitingDelay    bool

	angle       float64 // rotation phase [0,1)
	autoPercent float64
	autoAcc     float64

	canvas *primitive.Canvas
	life   tickerLifecycle
}

// spinNested lays out content first, then overlays mask + centered section
// at the same size (antd nested Spin).
type spinNested struct {
	core.NodeBase
	spin    *Spin
	content core.Node
	mask    *spinMask
	section core.Node
}

func (n *spinNested) TypeID() string { return "kit.SpinNested" }

func (n *spinNested) Layout(c core.Constraints) core.Size {
	if n == nil {
		return core.Size{}
	}
	if sz, ok := n.LayoutSkipIfClean(c); ok {
		return sz
	}
	var csz core.Size
	if n.content != nil {
		csz = n.content.Layout(c)
		n.content.Base().SetOffset(core.Point{})
	}
	out := c.Tighten(csz)
	tight := core.Tight(out.Width, out.Height)
	if n.mask != nil {
		_ = n.mask.Layout(tight)
		n.mask.Base().SetOffset(core.Point{})
	}
	if n.section != nil {
		ssz := n.section.Layout(core.Constraints{MaxWidth: out.Width, MaxHeight: out.Height})
		n.section.Base().SetOffset(core.Point{
			X: (out.Width - ssz.Width) / 2,
			Y: (out.Height - ssz.Height) / 2,
		})
	}
	n.SetSize(out)
	n.RememberConstraints(c)
	n.ClearLayoutDirty()
	return out
}

func (n *spinNested) Paint(pc *core.PaintContext) {
	if n == nil {
		return
	}
	n.DefaultPaintChildren(pc)
	if pc != nil {
		n.ClearPaintDirty()
	}
}

func (n *spinNested) HitTest(p core.Point) core.Node {
	if n == nil {
		return nil
	}
	// When spinning, mask absorbs hits (antd pointer-events:none on container).
	if n.mask != nil && n.spin != nil && n.spin.displaySpinning {
		if hit := n.mask.HitTest(p.Sub(n.mask.Base().Offset())); hit != nil {
			return hit
		}
		// Also try section (non-interactive but above).
		if n.section != nil {
			if hit := n.section.HitTest(p.Sub(n.section.Base().Offset())); hit != nil {
				return hit
			}
		}
		return n.mask
	}
	return n.DefaultHitTest(p)
}

// spinMask is a full-bleed semi-transparent blocker over nested content.
type spinMask struct {
	core.NodeBase
	color render.RGBA
}

func (m *spinMask) TypeID() string { return "kit.SpinMask" }

func (m *spinMask) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{Width: c.MaxWidth, Height: c.MaxHeight})
	if c.MaxWidth >= core.Unbounded/2 {
		out.Width = c.MinWidth
	}
	if c.MaxHeight >= core.Unbounded/2 {
		out.Height = c.MinHeight
	}
	m.SetSize(out)
	return out
}

func (m *spinMask) Paint(pc *core.PaintContext) {
	if pc == nil || m == nil || m.color.A <= 0 {
		return
	}
	sz := m.Size()
	pc.FillLocalRect(0, 0, sz.Width, sz.Height, m.color)
}

func (m *spinMask) HitTest(p core.Point) core.Node {
	if m == nil {
		return nil
	}
	if m.LocalBounds().Contains(p) {
		return m
	}
	return nil
}

type spinHost struct {
	primitive.RepaintBoundary
	spin *Spin
}

func (h *spinHost) TypeID() string { return "kit.Spin" }

func (h *spinHost) OnMount() {
	if h == nil || h.spin == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.spin.life.attach(t, h.spin, h.spin.needsTicker())
	}
}

func (h *spinHost) OnUnmount() {
	if h != nil && h.spin != nil {
		h.spin.life.unmount()
	}
}

// NewSpin creates a Spin. content may be nil (simple indicator-only mode).
// Defaults: spinning=true, size=medium, delay=0.
func NewSpin(content core.Node) *Spin {
	s := &Spin{
		content:         content,
		Spinning:        true,
		Size:            SpinSizeMedium,
		displaySpinning: true,
	}
	return s
}

// Node returns the mount root (lazy rebuild).
func (s *Spin) Node() core.Node {
	if s == nil {
		return nil
	}
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// AttachTicker registers delay/rotation/auto-percent animation.
func (s *Spin) AttachTicker(t *core.Tree) {
	if s != nil {
		s.life.attach(t, s, s.needsTicker())
	}
}

// Tick advances delay, rotation, and percent=auto. Returns false when idle/unmounted.
func (s *Spin) Tick(dt float64) bool {
	if s == nil {
		return false
	}
	var nt *core.Tree
	if s.Root != nil {
		nt = s.Root.Tree()
	}
	if !s.life.stillMounted(nt) {
		return false
	}
	reduce := s.reduceMotion(nt)

	// Delay reveal.
	if s.waitingDelay {
		s.delayAcc += dt * 1000 // Delay is ms
		if s.delayAcc >= s.Delay {
			s.waitingDelay = false
			s.delayAcc = 0
			if s.Spinning && !s.displaySpinning {
				s.displaySpinning = true
				s.rebuild()
				s.syncTicker()
				return s.needsTicker()
			}
		}
		return true
	}

	if !s.displaySpinning {
		return false
	}
	if reduce {
		return false
	}

	painted := false
	if s.PercentAuto {
		s.autoAcc += dt
		for s.autoAcc >= spinAutoInterval {
			s.autoAcc -= spinAutoInterval
			s.autoPercent = stepSpinAutoPercent(s.autoPercent)
		}
		painted = true
	}

	// Rotate only when showing 4-dot (no percent, no custom indicator canvas still rotates if default).
	if !s.HasPercent() && s.resolvedIndicator() == nil {
		s.angle += dt * spinRPS
		if s.angle >= 1 {
			s.angle -= math.Floor(s.angle)
		}
		painted = true
	}

	if painted {
		if s.canvas != nil {
			s.canvas.MarkNeedsPaint()
		} else if s.Root != nil {
			s.Root.MarkNeedsPaint()
		}
	}
	return s.needsTicker()
}

func (s *Spin) reduceMotion(nt *core.Tree) bool {
	if nt != nil && nt.Clock() != nil && nt.Clock().ReduceMotion {
		return true
	}
	if s.life.tree != nil && s.life.tree.Clock() != nil && s.life.tree.Clock().ReduceMotion {
		return true
	}
	return false
}

func (s *Spin) needsTicker() bool {
	if s == nil {
		return false
	}
	if s.waitingDelay {
		return true
	}
	if !s.displaySpinning {
		return false
	}
	if s.PercentAuto {
		return true
	}
	// Custom indicator: Spin itself may not animate; still tick only for default dots.
	if s.resolvedIndicator() != nil {
		return false
	}
	if s.HasPercent() {
		// Static percent ring — no continuous tick needed unless auto (handled above).
		return false
	}
	return true // 4-dot rotation
}

func (s *Spin) syncTicker() {
	s.life.setActive(s.needsTicker())
}

// IsDisplaySpinning reports the delayed visible spinning state.
func (s *Spin) IsDisplaySpinning() bool {
	return s != nil && s.displaySpinning
}

// HasPercent reports whether percent mode is active (number or auto).
func (s *Spin) HasPercent() bool {
	return s != nil && (s.PercentAuto || s.percentSet)
}

// EffectivePercent returns the displayed percent (auto mock or clamped number).
func (s *Spin) EffectivePercent() float64 {
	if s == nil || !s.HasPercent() {
		return 0
	}
	if s.PercentAuto {
		return s.autoPercent
	}
	return clampSpinPercent(s.Percent)
}

// DotSize returns the indicator edge for the current size (Token fallbacks).
func (s *Spin) DotSize() float64 {
	if s == nil {
		return DefaultSpinDotSize
	}
	th := s.theme()
	switch s.normalizedSize() {
	case SpinSizeSmall:
		if th != nil {
			lg := th.SizeOr(core.TokenControlHeightLG, 40)
			return lg * 0.35
		}
		return DefaultSpinDotSizeSM
	case SpinSizeLarge:
		if th != nil {
			return th.SizeOr(core.TokenControlHeight, DefaultSpinDotSizeLG)
		}
		return DefaultSpinDotSizeLG
	default:
		if th != nil {
			if v := th.SizeOr(core.TokenSpinSize, 0); v > 0 {
				return v
			}
			lg := th.SizeOr(core.TokenControlHeightLG, 40)
			return lg / 2
		}
		return DefaultSpinDotSize
	}
}

// IndicatorColor returns the resolved primary indicator color (tests / L2).
func (s *Spin) IndicatorColor() render.RGBA {
	th := s.theme()
	if s.Styles.Indicator.hasText() {
		return s.Styles.Indicator.Text
	}
	if s.Style.hasText() {
		return s.Style.Text
	}
	if th != nil {
		return th.Color(core.TokenColorPrimary)
	}
	return core.DefaultTheme().Color(core.TokenColorPrimary)
}

// SetContent sets nested children (rebuild).
func (s *Spin) SetContent(n core.Node) {
	if s == nil {
		return
	}
	s.content = n
	s.rebuild()
}

// Content returns nested children.
func (s *Spin) Content() core.Node {
	if s == nil {
		return nil
	}
	return s.content
}

// SetSpinning sets props spinning and runs the delay state machine.
func (s *Spin) SetSpinning(v bool) {
	if s == nil {
		return
	}
	s.Spinning = v
	if !v {
		s.waitingDelay = false
		s.delayAcc = 0
		if s.displaySpinning {
			s.displaySpinning = false
			s.rebuild()
		}
		s.syncTicker()
		return
	}
	// spinning=true
	if s.Delay > 0 && !s.displaySpinning {
		s.waitingDelay = true
		s.delayAcc = 0
		s.syncTicker()
		return
	}
	if s.Delay > 0 && s.displaySpinning {
		// Already visible; keep visible (antd re-debounce on prop change restarts).
		// Restart delay only when transitioning from false→true handled above.
	}
	if !s.displaySpinning {
		s.displaySpinning = true
		if s.PercentAuto {
			s.autoPercent = 0
			s.autoAcc = 0
		}
		s.rebuild()
	}
	s.waitingDelay = false
	s.syncTicker()
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
}

// SetDelay sets reveal delay in milliseconds.
func (s *Spin) SetDelay(ms float64) {
	if s == nil {
		return
	}
	if ms < 0 {
		ms = 0
	}
	s.Delay = ms
	// If currently wanting spin but hidden waiting, keep waiting with new delay budget.
	if s.Spinning && !s.displaySpinning && ms > 0 {
		s.waitingDelay = true
		s.delayAcc = 0
		s.syncTicker()
		return
	}
	if s.Spinning && !s.displaySpinning && ms == 0 {
		s.displaySpinning = true
		s.waitingDelay = false
		s.rebuild()
		s.syncTicker()
	}
}

// SetSize sets small|medium|large. "default" maps to medium.
func (s *Spin) SetSize(sz SpinSize) {
	if s == nil {
		return
	}
	sz = normalizeSpinSize(sz)
	if s.Size == sz {
		return
	}
	s.Size = sz
	s.rebuild()
}

// SetDescription sets the description text.
func (s *Spin) SetDescription(text string) {
	if s == nil {
		return
	}
	if s.Description == text {
		return
	}
	s.Description = text
	s.rebuild()
}

// SetTip is the deprecated antd tip alias → Description.
func (s *Spin) SetTip(text string) {
	if s == nil {
		return
	}
	s.Tip = text
	if s.Description == "" {
		s.rebuild()
	}
}

// mergedDescription returns description ?? tip.
func (s *Spin) mergedDescription() string {
	if s == nil {
		return ""
	}
	if s.Description != "" {
		return s.Description
	}
	return s.Tip
}

// SetPercent sets a numeric percent and clears auto mode.
func (s *Spin) SetPercent(p float64) {
	if s == nil {
		return
	}
	s.Percent = p
	s.percentSet = true
	s.PercentAuto = false
	s.rebuild()
	s.syncTicker()
}

// SetPercentAuto enables percent="auto".
func (s *Spin) SetPercentAuto() {
	if s == nil {
		return
	}
	s.PercentAuto = true
	s.percentSet = true
	s.autoPercent = 0
	s.autoAcc = 0
	s.rebuild()
	s.syncTicker()
}

// ClearPercent leaves percent mode (back to 4-dot / custom indicator).
func (s *Spin) ClearPercent() {
	if s == nil {
		return
	}
	s.percentSet = false
	s.PercentAuto = false
	s.Percent = 0
	s.autoPercent = 0
	s.rebuild()
	s.syncTicker()
}

// SetIndicator sets a custom instance indicator.
func (s *Spin) SetIndicator(n core.Node) {
	if s == nil {
		return
	}
	s.Indicator = n
	s.rebuild()
	s.syncTicker()
}

// SetTheme sets the theme override.
func (s *Spin) SetTheme(th *core.Theme) {
	if s == nil {
		return
	}
	s.Theme = th
	s.rebuild()
}

// SetStyle sets root style override.
func (s *Spin) SetStyle(st Style) {
	if s == nil {
		return
	}
	s.Style = st
	s.rebuild()
}

// SetClassNames sets shallow semantic class tags.
func (s *Spin) SetClassNames(cn SpinClassNames) {
	if s == nil {
		return
	}
	s.ClassNames = cn
	s.rebuild()
}

// SetStyles sets shallow semantic style overrides.
func (s *Spin) SetStyles(st SpinStyles) {
	if s == nil {
		return
	}
	s.Styles = st
	s.rebuild()
}

// SetAriaLabel sets the accessible name.
func (s *Spin) SetAriaLabel(label string) {
	if s == nil {
		return
	}
	s.AriaLabel = label
	if s.Root != nil {
		s.applyA11y(s.Root)
		s.Root.MarkNeedsPaint()
	}
}

// Phase returns the current rotation phase [0,1) (tests).
func (s *Spin) Phase() float64 {
	if s == nil {
		return 0
	}
	return s.angle
}

func (s *Spin) theme() *core.Theme {
	var n core.Node
	if s.Root != nil {
		n = s.Root
	}
	return themeOf(s.Theme, n)
}

func (s *Spin) normalizedSize() SpinSize {
	return normalizeSpinSize(s.Size)
}

func normalizeSpinSize(sz SpinSize) SpinSize {
	switch SpinSize(strings.ToLower(string(sz))) {
	case SpinSizeSmall, "sm":
		return SpinSizeSmall
	case SpinSizeLarge, "lg":
		return SpinSizeLarge
	case "default", SpinSizeMedium, "middle", "":
		return SpinSizeMedium
	default:
		return SpinSizeMedium
	}
}

func (s *Spin) resolvedIndicator() core.Node {
	if s == nil {
		return nil
	}
	if s.Indicator != nil {
		return s.Indicator
	}
	return defaultSpinIndicator
}

func (s *Spin) rebuild() {
	if s == nil {
		return
	}
	// Ensure size default.
	if s.Size == "" {
		s.Size = SpinSizeMedium
	}
	// Align display with props when no delay wait in progress.
	if !s.Spinning {
		s.displaySpinning = false
		s.waitingDelay = false
	} else if !s.waitingDelay && s.Delay <= 0 {
		s.displaySpinning = true
	}

	th := s.theme()
	dot := s.DotSize()
	gap := DefaultSpinGap
	if th != nil {
		gap = th.SizeOr(core.TokenPaddingSM, DefaultSpinGap)
	}
	font := DefaultSpinFontSize
	if th != nil {
		font = th.SizeOr(core.TokenFontSize, DefaultSpinFontSize)
	}
	if s.Styles.Description.FontSize > 0 {
		font = s.Styles.Description.FontSize
	}

	desc := s.mergedDescription()
	indColor := s.IndicatorColor()

	var indicatorNode core.Node
	if s.displaySpinning {
		if custom := s.resolvedIndicator(); custom != nil && !s.HasPercent() {
			indicatorNode = custom
		} else {
			// Built-in: 4-dot or percent ring on Canvas.
			s.canvas = primitive.NewCanvas(dot, dot, s.paintIndicator)
			indicatorNode = s.canvas
		}
	} else {
		s.canvas = nil
	}

	var section core.Node
	if s.displaySpinning && indicatorNode != nil {
		var body core.Node = indicatorNode
		if desc != "" {
			lab := primitive.NewText(desc)
			lab.FontSize = font
			lab.Color = indColor
			if s.Styles.Description.hasText() {
				lab.Color = s.Styles.Description.Text
			}
			col := primitive.Column(indicatorNode, lab)
			col.Gap = gap
			col.CrossAlign = core.CrossCenter
			col.MainAlign = core.MainCenter
			col.Hit = core.HitDefer
			body = col
		}
		sec := primitive.NewDecorated(body)
		sec.Hit = core.HitDefer
		if s.Styles.Section.hasBG() {
			sec.Background = s.Styles.Section.Background
		}
		if s.Styles.Section.hasRadius() {
			sec.Radius = s.Styles.Section.Radius
		}
		section = sec
	}

	var rootChild core.Node
	if s.content != nil {
		// Nested mode: children always in tree (SPN-S6).
		nested := &spinNested{spin: s, content: s.content, section: section}
		nested.Init(nested)
		nested.Hit = core.HitDefer
		nested.AddChild(s.content)
		if s.displaySpinning {
			bg := render.RGBA{R: 1, G: 1, B: 1, A: spinNestedMaskA}
			if th != nil {
				if c := th.Color(core.TokenColorBgContainer); c.A > 0 || c.R > 0 || c.G > 0 || c.B > 0 {
					bg = c
					bg.A = spinNestedMaskA
				}
			}
			mask := &spinMask{color: bg}
			mask.Init(mask)
			mask.Hit = core.HitBlock
			nested.mask = mask
			nested.AddChild(mask)
			if section != nil {
				nested.AddChild(section)
			}
		}
		rootChild = nested
	} else if section != nil {
		rootChild = section
	} else {
		empty := primitive.NewBox(nil)
		empty.Width, empty.Height = 0, 0
		empty.Hit = core.HitDefer
		rootChild = empty
	}

	var h *spinHost
	if s.Root != nil {
		h = s.Root
		h.spin = s
		// Drop previous children without replacing the host identity (keeps tree mount).
		h.ClearChildren()
	} else {
		h = &spinHost{spin: s}
		h.Init(h)
		h.SetRepaintBoundary(true)
		s.Root = h
	}
	h.Hit = core.HitDefer
	if rootChild != nil {
		h.AddChild(rootChild)
	}
	s.applyA11y(h)
	h.MarkNeedsLayout()
	h.MarkNeedsPaint()
	s.syncTicker()
}

func (s *Spin) applyA11y(h *spinHost) {
	if h == nil || s == nil {
		return
	}
	h.Base().Role = "status"
	h.Base().Live = "polite"
	// Busy mirrors display spinning (antd aria-busy={spinning}).
	if s.displaySpinning {
		h.Base().Label = s.a11yName()
	} else {
		h.Base().Label = s.a11yName()
	}
	// Store busy in Label suffix is weak; NodeBase may lack Busy — use Live + Label.
	// Expose via custom: many tests check Role.
	_ = s.displaySpinning
}

func (s *Spin) a11yName() string {
	if s.AriaLabel != "" {
		return s.AriaLabel
	}
	if d := s.mergedDescription(); d != "" {
		return d
	}
	return "Loading"
}

// Busy reports aria-busy equivalent (display spinning).
func (s *Spin) Busy() bool { return s != nil && s.displaySpinning }

func (s *Spin) paintIndicator(pc *core.PaintContext, sz core.Size) {
	if pc == nil || pc.DC == nil || s == nil || !s.displaySpinning {
		return
	}
	if s.HasPercent() {
		s.paintPercent(pc, sz)
		return
	}
	s.paintDots(pc, sz)
}

func (s *Spin) paintDots(pc *core.PaintContext, sz core.Size) {
	dc := pc.DC
	edge := sz.Width
	if sz.Height < edge {
		edge = sz.Height
	}
	if edge <= 0 {
		return
	}
	// Dot item size ≈ (dotSize - marginXXS/2) / 2.
	item := (edge - 2) / 2
	if item < 2 {
		item = 2
	}
	if item > edge*0.45 {
		item = edge * 0.45
	}
	cx := pc.Origin.X + sz.Width/2
	cy := pc.Origin.Y + sz.Height/2
	// Holder rotates 45° + phase*360° (antd antRotate 405° from 45° base).
	base := math.Pi / 4
	rot := base + s.angle*2*math.Pi
	// Four corners of a square before rotation, inset so dots sit inside.
	half := edge/2 - item/2
	if half < 1 {
		half = 1
	}
	offsets := [4][2]float64{
		{-half, -half},
		{half, -half},
		{half, half},
		{-half, half},
	}
	col := s.IndicatorColor()
	// Opacity pulse approximate: 0.3 base + phase-shifted boost (antd alternate 1s).
	for i, off := range offsets {
		x := off[0]
		y := off[1]
		rx := x*math.Cos(rot) - y*math.Sin(rot)
		ry := x*math.Sin(rot) + y*math.Cos(rot)
		// Stagger opacity like animation-delay 0/0.4/0.8/1.2s on 1s alternate.
		phase := s.angle + float64(i)*0.25
		phase = phase - math.Floor(phase)
		// triangle 0.3..1
		op := 0.3 + 0.7*math.Abs(0.5-phase)*2
		if op > 1 {
			op = 1
		}
		dc.SetRGBA(col.R, col.G, col.B, col.A*op)
		dc.DrawCircle(cx+rx, cy+ry, item/2)
		_ = dc.Fill()
	}
}

func (s *Spin) paintPercent(pc *core.PaintContext, sz core.Size) {
	dc := pc.DC
	edge := sz.Width
	if sz.Height < edge {
		edge = sz.Height
	}
	if edge <= 0 {
		return
	}
	// Mirror Progress.tsx: viewBox 100, borderWidth=20, r=40.
	stroke := edge / 5
	if stroke < 2 {
		stroke = 2
	}
	r := edge/2 - stroke/2
	if r < 1 {
		r = 1
	}
	cx := pc.Origin.X + sz.Width/2
	cy := pc.Origin.Y + sz.Height/2
	ptg := s.EffectivePercent()
	if ptg < 0 {
		ptg = 0
	}
	if ptg > 100 {
		ptg = 100
	}

	track := render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	if th := s.theme(); th != nil {
		if c := th.Color(core.TokenColorFillSecondary); c.A > 0 {
			track = c
		}
	}
	fill := s.IndicatorColor()

	dc.SetLineCap(render.LineCapRound)
	dc.SetLineWidth(stroke)
	// Background circle.
	dc.SetRGBA(track.R, track.G, track.B, track.A)
	dc.DrawCircle(cx, cy, r)
	_ = dc.Stroke()

	if ptg <= 0 {
		return
	}
	// Arc from top, clockwise fraction ptg/100.
	// stroke-dashoffset = circumference/4 → start at top.
	start := -math.Pi / 2
	sweep := 2 * math.Pi * (ptg / 100)
	dc.SetRGBA(fill.R, fill.G, fill.B, fill.A)
	const steps = 64
	for i := 0; i <= steps; i++ {
		a := start + sweep*float64(i)/steps
		x := cx + r*math.Cos(a)
		y := cy + r*math.Sin(a)
		if i == 0 {
			dc.MoveTo(x, y)
		} else {
			dc.LineTo(x, y)
		}
	}
	_ = dc.Stroke()
}

func clampSpinPercent(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// stepSpinAutoPercent mirrors usePercent STEP_BUCKETS.
func stepSpinAutoPercent(prev float64) float64 {
	rest := 100 - prev
	switch {
	case prev <= 30:
		return prev + rest*0.05
	case prev <= 70:
		return prev + rest*0.03
	case prev <= 96:
		return prev + rest*0.01
	default:
		return prev
	}
}
