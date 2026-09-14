// Package layout implements the Layout control (antd v6.5.1 spec section 6).
//
// Scope is P0 only: four regions plus nesting, sider width/collapse,
// theme, trigger, breakpoint and overlay. P1 (fixed/sticky, semantic
// hooks, transitions) is out of scope.
package layout

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Geometry fallbacks (used when theme tokens are zero).
const (
	DefaultLayoutHeaderHeight  = 64.0
	DefaultSiderWidth          = 200.0
	DefaultSiderCollapsedWidth = 80.0
	DefaultTriggerHeight       = 48.0
	DefaultZeroTriggerSize     = 40.0
	DefaultLayoutFooterHeight  = 70.0
	siderDarkBgHex             = "#001529"
	triggerDarkBgHex           = "#002140"
	siderSectionTag            = "layout-sider"
	headerSectionTag           = "layout-header"
	footerSectionTag           = "layout-footer"
	contentSectionTag          = "layout-content"
	layoutRootTag              = "layout-root"
)

// SiderTheme selects the sider shell (not the global theme).
type SiderTheme int

const (
	SiderThemeDark SiderTheme = iota
	SiderThemeLight
)

// CollapseType reports how a collapse happened.
type CollapseType string

const (
	CollapseClickTrigger CollapseType = "click"
	CollapseResponsive   CollapseType = "responsive"
)

// LayoutBreakpoint selects the responsive breakpoint.
type LayoutBreakpoint int

const (
	LayoutBreakpointNone LayoutBreakpoint = iota
	LayoutBreakpointXS
	LayoutBreakpointSM
	LayoutBreakpointMD
	LayoutBreakpointLG
	LayoutBreakpointXL
	LayoutBreakpointXXL
	LayoutBreakpointXXXL
)

// Screen widths shared with grid semantics.
const (
	ScreenXS   = 576.0
	ScreenSM   = 576.0
	ScreenMD   = 768.0
	ScreenLG   = 992.0
	ScreenXL   = 1200.0
	ScreenXXL  = 1600.0
	ScreenXXXL = 1920.0
)

// Threshold maps a breakpoint to its collapse threshold.
func (b LayoutBreakpoint) Threshold() float64 {
	switch b {
	case LayoutBreakpointXS:
		return ScreenXS
	case LayoutBreakpointSM:
		return ScreenSM
	case LayoutBreakpointMD:
		return ScreenMD
	case LayoutBreakpointLG:
		return ScreenLG
	case LayoutBreakpointXL:
		return ScreenXL
	case LayoutBreakpointXXL:
		return ScreenXXL
	case LayoutBreakpointXXXL:
		return ScreenXXXL
	default:
		return 0
	}
}

// Insets is logical padding (Y-down).
type Insets struct {
	Top, Right, Bottom, Left float64
}

// NewInsets builds padding.
func NewInsets(top, right, bottom, left float64) Insets {
	return Insets{Top: top, Right: right, Bottom: bottom, Left: left}
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func bounded(v float64) bool { return v < rendering.Unbounded/2 }

// LayoutSection is one direct child of Layout.
type LayoutSection interface {
	Node() rendering.RenderObject
	sectionKind() string
	sectionLayout(c rendering.Constraints) rendering.Size
}

// Header is the top band (default height 64).
type Header struct {
	node     *rendering.RenderBox
	height   float64
	padding  Insets
	bg       render.RGBA
	hasBg    bool
	provider *theme.Provider
	override *theme.Tokens
	aria     string
}

// NewHeader creates a header.
func NewHeader(children ...rendering.RenderObject) *Header {
	h := &Header{}
	h.node = rendering.NewRenderBox()
	h.node.SetRepaintBoundary(true)
	h.node.SetDebugName(headerSectionTag)
	self := h
	h.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	h.SetChildren(children...)
	return h
}

func (h *Header) sectionKind() string { return "header" }

func (h *Header) themeTokens() theme.Tokens {
	if h != nil && h.override != nil {
		return *h.override
	}
	if h != nil && h.provider != nil {
		return h.provider.Current()
	}
	return theme.Default.Current()
}

// SetHeight sets fixed height (<=0 selects default 64).
func (h *Header) SetHeight(v float64) {
	if h == nil {
		return
	}
	h.height = v
	h.node.MarkNeedsLayout()
	h.node.MarkNeedsPaint()
}

// EffectiveHeight returns the laid-out height.
func (h *Header) EffectiveHeight() float64 {
	if h != nil && h.height > 0 {
		return h.height
	}
	tok := h.themeTokens()
	if tok.ControlHeight > 0 {
		return tok.ControlHeight * 2
	}
	return DefaultLayoutHeaderHeight
}

// SetPaddingInsets stores padding.
func (h *Header) SetPaddingInsets(in Insets) {
	if h == nil {
		return
	}
	h.padding = in
	h.node.MarkNeedsPaint()
}

// Padding returns stored padding.
func (h *Header) Padding() Insets {
	if h == nil {
		return Insets{}
	}
	return h.padding
}

// SetBackground overrides header bg (paint only).
func (h *Header) SetBackground(c render.RGBA) {
	if h == nil {
		return
	}
	h.bg = c
	h.hasBg = true
	h.node.MarkNeedsPaint()
}

// EffectiveBackground returns header bg (dark shell default).
func (h *Header) EffectiveBackground() render.RGBA {
	if h != nil && h.hasBg {
		return h.bg
	}
	return render.Hex(siderDarkBgHex)
}

// SetProvider selects theme source.
func (h *Header) SetProvider(p *theme.Provider) {
	if h == nil {
		return
	}
	h.provider = p
	h.node.MarkNeedsPaint()
}

// SetTheme pins exact tokens.
func (h *Header) SetTheme(t *theme.Tokens) {
	if h == nil {
		return
	}
	h.override = t
	h.node.MarkNeedsPaint()
}

// SetAriaLabel names the container.
func (h *Header) SetAriaLabel(s string) {
	if h == nil {
		return
	}
	h.aria = s
}

// AriaLabel returns the accessible name.
func (h *Header) AriaLabel() string {
	if h == nil {
		return ""
	}
	return h.aria
}

// Role has no forced role.
func (h *Header) Role() string { return "" }

// Focusable is always false for the band.
func (h *Header) Focusable() bool { return false }

// Add appends children.
func (h *Header) Add(children ...rendering.RenderObject) {
	if h == nil {
		return
	}
	for _, c := range children {
		if c != nil {
			h.node.AddChild(c)
		}
	}
}

// SetChildren replaces children.
func (h *Header) SetChildren(children ...rendering.RenderObject) {
	if h == nil {
		return
	}
	for _, c := range h.node.Children() {
		h.node.RemoveChild(c)
	}
	h.Add(children...)
}

// ClearChildren removes all children.
func (h *Header) ClearChildren() {
	if h == nil {
		return
	}
	for _, c := range h.node.Children() {
		h.node.RemoveChild(c)
	}
}

// Node returns the tree node.
func (h *Header) Node() rendering.RenderObject {
	if h == nil {
		return nil
	}
	return h.node
}

// Layout sizes the node.
func (h *Header) Layout(c rendering.Constraints) rendering.Size {
	if h == nil {
		return rendering.Size{}
	}
	return h.sectionLayout(c)
}

func (h *Header) sectionLayout(c rendering.Constraints) rendering.Size {
	prefW := c.MaxWidth
	if !bounded(prefW) {
		prefW = 0
	}
	out := c.Tighten(rendering.Size{Width: prefW, Height: h.EffectiveHeight()})
	h.node.FixedWidth = out.Width
	h.node.FixedHeight = out.Height
	h.node.Layout(c)
	return out
}

func (h *Header) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	bg := h.EffectiveBackground()
	rendering.FillRect(pc, 0, 0, size.Width, size.Height, bg.R, bg.G, bg.B, bg.A)
}

// Footer is the bottom band.
type Footer struct {
	node     *rendering.RenderBox
	padding  Insets
	bg       render.RGBA
	hasBg    bool
	provider *theme.Provider
	override *theme.Tokens
	aria     string
}

// NewFooter creates a footer.
func NewFooter(children ...rendering.RenderObject) *Footer {
	f := &Footer{}
	f.node = rendering.NewRenderBox()
	f.node.SetRepaintBoundary(true)
	f.node.SetDebugName(footerSectionTag)
	self := f
	f.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	f.SetChildren(children...)
	return f
}

func (f *Footer) sectionKind() string { return "footer" }

func (f *Footer) themeTokens() theme.Tokens {
	if f != nil && f.override != nil {
		return *f.override
	}
	if f != nil && f.provider != nil {
		return f.provider.Current()
	}
	return theme.Default.Current()
}

// EffectiveHeight returns footer height (padding derived, fallback 70).
func (f *Footer) EffectiveHeight() float64 {
	tok := f.themeTokens()
	if tok.PaddingLG > 0 && tok.FontHeight > 0 {
		return tok.PaddingLG*2 + tok.FontHeight
	}
	return DefaultLayoutFooterHeight
}

// SetPaddingInsets stores padding.
func (f *Footer) SetPaddingInsets(in Insets) {
	if f == nil {
		return
	}
	f.padding = in
	f.node.MarkNeedsPaint()
}

// Padding returns stored padding.
func (f *Footer) Padding() Insets {
	if f == nil {
		return Insets{}
	}
	return f.padding
}

// SetBackground overrides footer bg (paint only).
func (f *Footer) SetBackground(c render.RGBA) {
	if f == nil {
		return
	}
	f.bg = c
	f.hasBg = true
	f.node.MarkNeedsPaint()
}

// EffectiveBackground returns footer bg (body bg default).
func (f *Footer) EffectiveBackground() render.RGBA {
	if f != nil && f.hasBg {
		return f.bg
	}
	return themeToRGBA(f.themeTokens().ColorBgLayout)
}

// SetProvider selects theme source.
func (f *Footer) SetProvider(p *theme.Provider) {
	if f == nil {
		return
	}
	f.provider = p
	f.node.MarkNeedsPaint()
}

// SetTheme pins exact tokens.
func (f *Footer) SetTheme(t *theme.Tokens) {
	if f == nil {
		return
	}
	f.override = t
	f.node.MarkNeedsPaint()
}

// SetAriaLabel names the container.
func (f *Footer) SetAriaLabel(s string) {
	if f == nil {
		return
	}
	f.aria = s
}

// AriaLabel returns the accessible name.
func (f *Footer) AriaLabel() string {
	if f == nil {
		return ""
	}
	return f.aria
}

// Role has no forced role.
func (f *Footer) Role() string { return "" }

// Focusable is always false for the band.
func (f *Footer) Focusable() bool { return false }

// Add appends children.
func (f *Footer) Add(children ...rendering.RenderObject) {
	if f == nil {
		return
	}
	for _, c := range children {
		if c != nil {
			f.node.AddChild(c)
		}
	}
}

// SetChildren replaces children.
func (f *Footer) SetChildren(children ...rendering.RenderObject) {
	if f == nil {
		return
	}
	for _, c := range f.node.Children() {
		f.node.RemoveChild(c)
	}
	f.Add(children...)
}

// ClearChildren removes all children.
func (f *Footer) ClearChildren() {
	if f == nil {
		return
	}
	for _, c := range f.node.Children() {
		f.node.RemoveChild(c)
	}
}

// Node returns the tree node.
func (f *Footer) Node() rendering.RenderObject {
	if f == nil {
		return nil
	}
	return f.node
}

// Layout sizes the node.
func (f *Footer) Layout(c rendering.Constraints) rendering.Size {
	if f == nil {
		return rendering.Size{}
	}
	return f.sectionLayout(c)
}

func (f *Footer) sectionLayout(c rendering.Constraints) rendering.Size {
	prefW := c.MaxWidth
	if !bounded(prefW) {
		prefW = 0
	}
	out := c.Tighten(rendering.Size{Width: prefW, Height: f.EffectiveHeight()})
	f.node.FixedWidth = out.Width
	f.node.FixedHeight = out.Height
	f.node.Layout(c)
	return out
}

func (f *Footer) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	bg := f.EffectiveBackground()
	if bg.A <= 0 {
		return
	}
	rendering.FillRect(pc, 0, 0, size.Width, size.Height, bg.R, bg.G, bg.B, bg.A)
}

// Content is the flexible middle.
type Content struct {
	node      *rendering.RenderBox
	minHeight float64
	padding   Insets
	bg        render.RGBA
	hasBg     bool
	provider  *theme.Provider
	override  *theme.Tokens
	aria      string
}

// NewContent creates content.
func NewContent(children ...rendering.RenderObject) *Content {
	c := &Content{}
	c.node = rendering.NewRenderBox()
	c.node.SetRepaintBoundary(true)
	c.node.SetDebugName(contentSectionTag)
	self := c
	c.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	c.SetChildren(children...)
	return c
}

func (c *Content) sectionKind() string { return "content" }

func (c *Content) themeTokens() theme.Tokens {
	if c != nil && c.override != nil {
		return *c.override
	}
	if c != nil && c.provider != nil {
		return c.provider.Current()
	}
	return theme.Default.Current()
}

// SetMinHeight sets the minimum height.
func (c *Content) SetMinHeight(v float64) {
	if c == nil {
		return
	}
	c.minHeight = v
	c.node.MarkNeedsLayout()
}

// EffectiveMinHeight returns the minimum height.
func (c *Content) EffectiveMinHeight() float64 {
	if c == nil {
		return 0
	}
	return c.minHeight
}

// SetPaddingInsets stores padding.
func (c *Content) SetPaddingInsets(in Insets) {
	if c == nil {
		return
	}
	c.padding = in
	c.node.MarkNeedsPaint()
}

// Padding returns stored padding.
func (c *Content) Padding() Insets {
	if c == nil {
		return Insets{}
	}
	return c.padding
}

// SetBackground overrides content bg (paint only).
func (c *Content) SetBackground(v render.RGBA) {
	if c == nil {
		return
	}
	c.bg = v
	c.hasBg = true
	c.node.MarkNeedsPaint()
}

// EffectiveBackground returns content bg (transparent default).
func (c *Content) EffectiveBackground() render.RGBA {
	if c != nil && c.hasBg {
		return c.bg
	}
	return render.RGBA{}
}

// SetProvider selects theme source.
func (c *Content) SetProvider(p *theme.Provider) {
	if c == nil {
		return
	}
	c.provider = p
	c.node.MarkNeedsPaint()
}

// SetTheme pins exact tokens.
func (c *Content) SetTheme(t *theme.Tokens) {
	if c == nil {
		return
	}
	c.override = t
	c.node.MarkNeedsPaint()
}

// SetAriaLabel names the container.
func (c *Content) SetAriaLabel(s string) {
	if c == nil {
		return
	}
	c.aria = s
}

// AriaLabel returns the accessible name.
func (c *Content) AriaLabel() string {
	if c == nil {
		return ""
	}
	return c.aria
}

// Role has no forced role.
func (c *Content) Role() string { return "" }

// Focusable is always false for the band.
func (c *Content) Focusable() bool { return false }

// Add appends children.
func (c *Content) Add(children ...rendering.RenderObject) {
	if c == nil {
		return
	}
	for _, n := range children {
		if n != nil {
			c.node.AddChild(n)
		}
	}
}

// SetChildren replaces children.
func (c *Content) SetChildren(children ...rendering.RenderObject) {
	if c == nil {
		return
	}
	for _, n := range c.node.Children() {
		c.node.RemoveChild(n)
	}
	c.Add(children...)
}

// ClearChildren removes all children.
func (c *Content) ClearChildren() {
	if c == nil {
		return
	}
	for _, n := range c.node.Children() {
		c.node.RemoveChild(n)
	}
}

// Node returns the tree node.
func (c *Content) Node() rendering.RenderObject {
	if c == nil {
		return nil
	}
	return c.node
}

// Layout sizes the node.
func (c *Content) Layout(cs rendering.Constraints) rendering.Size {
	if c == nil {
		return rendering.Size{}
	}
	return c.sectionLayout(cs)
}

func (c *Content) sectionLayout(cs rendering.Constraints) rendering.Size {
	prefW := cs.MaxWidth
	if !bounded(prefW) {
		prefW = 0
	}
	prefH := cs.MaxHeight
	if !bounded(prefH) {
		prefH = c.minHeight
	}
	if prefH < c.minHeight {
		prefH = c.minHeight
	}
	out := cs.Tighten(rendering.Size{Width: prefW, Height: prefH})
	if out.Height < c.minHeight {
		out.Height = c.minHeight
	}
	c.node.FixedWidth = out.Width
	c.node.FixedHeight = out.Height
	c.node.Layout(cs)
	return out
}

func (c *Content) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	bg := c.EffectiveBackground()
	if bg.A <= 0 {
		return
	}
	rendering.FillRect(pc, 0, 0, size.Width, size.Height, bg.R, bg.G, bg.B, bg.A)
}

// Sider is the side band with collapse support.
type Sider struct {
	node              *rendering.RenderBox
	width             float64
	collapsedWidth    float64
	collapsedWidthSet bool
	collapsible       bool
	collapsed         bool
	collapsedSet      bool
	internalCollapsed bool
	defaultCollapsed  bool
	siderTheme        SiderTheme
	reverseArrow      bool
	breakpoint        LayoutBreakpoint
	viewportWidth     float64
	viewportSet       bool
	lastBroken        bool
	triggerNode       rendering.RenderObject
	hasCustomTrigger  bool
	hideTrigger       bool
	overlay           bool
	onCollapse        func(bool, CollapseType)
	onBreakpoint      func(bool)
	provider          *theme.Provider
	override          *theme.Tokens
	bg                render.RGBA
	hasBg             bool
	aria              string
	focusNode         *focus.FocusNode
	focused           bool
	hovered           bool
}

// NewSider creates a sider.
func NewSider(children ...rendering.RenderObject) *Sider {
	s := &Sider{}
	s.node = rendering.NewRenderBox()
	s.node.SetRepaintBoundary(true)
	s.node.SetDebugName(siderSectionTag)
	s.focusNode = focus.NewFocusNode("sider-trigger")
	s.focusNode.TabIndex = 0
	s.focusNode.Enabled = false
	self := s
	s.focusNode.OnActivate = func() { self.ActivateTrigger() }
	s.focusNode.OnFocusChange = func(f bool) {
		self.focused = f
		self.node.MarkNeedsPaint()
	}
	s.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	s.SetChildren(children...)
	s.syncFocusEnabled()
	return s
}

func (s *Sider) sectionKind() string { return "sider" }

func (s *Sider) themeTokens() theme.Tokens {
	if s != nil && s.override != nil {
		return *s.override
	}
	if s != nil && s.provider != nil {
		return s.provider.Current()
	}
	return theme.Default.Current()
}

// SetWidth sets expanded width (<=0 selects 200).
func (s *Sider) SetWidth(w float64) {
	if s == nil {
		return
	}
	s.width = w
	s.node.MarkNeedsLayout()
	s.node.MarkNeedsPaint()
}

// EffectiveWidth returns expanded width.
func (s *Sider) EffectiveWidth() float64 {
	if s != nil && s.width > 0 {
		return s.width
	}
	return DefaultSiderWidth
}

// SetCollapsedWidth sets collapsed width (explicit 0 enables zero trigger).
func (s *Sider) SetCollapsedWidth(w float64) {
	if s == nil {
		return
	}
	s.collapsedWidth = w
	s.collapsedWidthSet = true
	s.node.MarkNeedsLayout()
	s.node.MarkNeedsPaint()
}

// EffectiveCollapsedWidth returns collapsed width.
func (s *Sider) EffectiveCollapsedWidth() float64 {
	if s != nil && s.collapsedWidthSet {
		return s.collapsedWidth
	}
	return DefaultSiderCollapsedWidth
}

// SetCollapsible enables the trigger.
func (s *Sider) SetCollapsible(b bool) {
	if s == nil {
		return
	}
	s.collapsible = b
	s.syncFocusEnabled()
	s.updateTriggerChild()
	s.node.MarkNeedsPaint()
}

// Collapsible reports the flag.
func (s *Sider) Collapsible() bool { return s != nil && s.collapsible }

// SetCollapsed sets controlled collapse (fires click callback on change).
func (s *Sider) SetCollapsed(b bool) {
	if s == nil {
		return
	}
	prev := s.CollapsedState()
	s.collapsed = b
	s.collapsedSet = true
	s.syncFocusEnabled()
	if prev != b && s.onCollapse != nil {
		s.onCollapse(b, CollapseClickTrigger)
	}
	s.node.MarkNeedsLayout()
	s.node.MarkNeedsPaint()
}

// SetDefaultCollapsed sets uncontrolled initial collapse.
func (s *Sider) SetDefaultCollapsed(b bool) {
	if s == nil {
		return
	}
	s.defaultCollapsed = b
	if !s.collapsedSet {
		s.internalCollapsed = b
		s.node.MarkNeedsLayout()
		s.node.MarkNeedsPaint()
	}
}

// CollapsedState returns current collapse.
func (s *Sider) CollapsedState() bool {
	if s == nil {
		return false
	}
	if s.collapsedSet {
		return s.collapsed
	}
	return s.internalCollapsed
}

// EffectiveSiderWidth returns painted width.
func (s *Sider) EffectiveSiderWidth() float64 {
	if s == nil {
		return DefaultSiderWidth
	}
	if s.CollapsedState() {
		return s.EffectiveCollapsedWidth()
	}
	return s.EffectiveWidth()
}

// FlowWidth returns main-axis width (0 for overlay).
func (s *Sider) FlowWidth() float64 {
	if s == nil || s.overlay {
		return 0
	}
	return s.EffectiveSiderWidth()
}

// SetTheme selects dark/light shell.
func (s *Sider) SetTheme(t SiderTheme) {
	if s == nil {
		return
	}
	s.siderTheme = t
	s.node.MarkNeedsPaint()
}

// Theme returns the shell.
func (s *Sider) Theme() SiderTheme {
	if s == nil {
		return SiderThemeDark
	}
	return s.siderTheme
}

// SetReverseArrow flips the arrow.
func (s *Sider) SetReverseArrow(b bool) {
	if s == nil {
		return
	}
	s.reverseArrow = b
	s.node.MarkNeedsPaint()
}

// ReverseArrow reports the flag.
func (s *Sider) ReverseArrow() bool { return s != nil && s.reverseArrow }

// ArrowPointsLeft reports arrow direction.
func (s *Sider) ArrowPointsLeft() bool {
	if s == nil {
		return true
	}
	collapsed := s.CollapsedState()
	if !s.reverseArrow {
		return !collapsed
	}
	return collapsed
}

// SetBreakpoint sets the responsive breakpoint.
func (s *Sider) SetBreakpoint(b LayoutBreakpoint) {
	if s == nil {
		return
	}
	s.breakpoint = b
	s.evaluateBreakpoint()
}

// Breakpoint returns it.
func (s *Sider) Breakpoint() LayoutBreakpoint {
	if s == nil {
		return LayoutBreakpointNone
	}
	return s.breakpoint
}

// SetViewportWidth injects viewport width for breakpoint tests.
func (s *Sider) SetViewportWidth(w float64) {
	if s == nil {
		return
	}
	s.viewportWidth = w
	s.viewportSet = true
	s.evaluateBreakpoint()
}

// ViewportWidth returns injected width.
func (s *Sider) ViewportWidth() float64 {
	if s == nil {
		return 0
	}
	return s.viewportWidth
}

func (s *Sider) evaluateBreakpoint() {
	if s == nil || s.breakpoint == LayoutBreakpointNone {
		return
	}
	vw := 0.0
	if s.viewportSet {
		vw = s.viewportWidth
	}
	if vw <= 0 {
		return
	}
	broken := vw < s.breakpoint.Threshold()
	changed := broken != s.lastBroken
	s.lastBroken = broken
	if changed && s.onBreakpoint != nil {
		s.onBreakpoint(broken)
	}
	if !s.collapsedSet {
		if s.internalCollapsed != broken {
			s.internalCollapsed = broken
			if s.onCollapse != nil {
				s.onCollapse(broken, CollapseResponsive)
			}
			s.node.MarkNeedsLayout()
			s.node.MarkNeedsPaint()
		} else if changed && s.onCollapse != nil {
			s.onCollapse(broken, CollapseResponsive)
		}
	} else if changed && s.onCollapse != nil {
		s.onCollapse(broken, CollapseResponsive)
	}
}

// SetTrigger installs a custom trigger (clears hidden).
func (s *Sider) SetTrigger(n rendering.RenderObject) {
	if s == nil {
		return
	}
	if s.triggerNode != nil && s.hasCustomTrigger {
		s.node.RemoveChild(s.triggerNode)
	}
	s.triggerNode = n
	s.hasCustomTrigger = n != nil
	s.hideTrigger = false
	s.updateTriggerChild()
	s.syncFocusEnabled()
	s.node.MarkNeedsPaint()
}

// SetHideTrigger hides the trigger (trigger=null).
func (s *Sider) SetHideTrigger() {
	if s == nil {
		return
	}
	s.hideTrigger = true
	s.updateTriggerChild()
	s.syncFocusEnabled()
	s.node.MarkNeedsPaint()
}

// ShowTrigger restores the default trigger.
func (s *Sider) ShowTrigger() {
	if s == nil {
		return
	}
	s.hideTrigger = false
	if s.hasCustomTrigger && s.triggerNode != nil {
		s.node.RemoveChild(s.triggerNode)
	}
	s.triggerNode = nil
	s.hasCustomTrigger = false
	s.updateTriggerChild()
	s.syncFocusEnabled()
	s.node.MarkNeedsPaint()
}

// TriggerVisible reports whether a trigger shows.
func (s *Sider) TriggerVisible() bool {
	if s == nil || s.hideTrigger || !s.collapsible {
		return false
	}
	return true
}

// IsZeroTrigger reports the 40x40 special trigger.
func (s *Sider) IsZeroTrigger() bool {
	if s == nil || !s.TriggerVisible() {
		return false
	}
	return s.CollapsedState() && s.EffectiveCollapsedWidth() == 0
}

// EffectiveTriggerHeight returns 48 (40 for zero).
func (s *Sider) EffectiveTriggerHeight() float64 {
	tok := s.themeTokens()
	lg := tok.ControlHeightLG
	if lg <= 0 {
		lg = 40
	}
	if s.IsZeroTrigger() {
		return lg
	}
	xxs := tok.MarginXXS
	if xxs <= 0 {
		xxs = 4
	}
	h := lg + xxs*2
	if h <= 0 {
		h = DefaultTriggerHeight
	}
	return h
}

// EffectiveTriggerWidth returns trigger width.
func (s *Sider) EffectiveTriggerWidth() float64 {
	if s.IsZeroTrigger() {
		tok := s.themeTokens()
		if tok.ControlHeightLG > 0 {
			return tok.ControlHeightLG
		}
		return DefaultZeroTriggerSize
	}
	return s.EffectiveSiderWidth()
}

// TriggerFocusable reports keyboard reachability.
func (s *Sider) TriggerFocusable() bool { return s.TriggerVisible() }

// TriggerFocusNode exposes the focus node.
func (s *Sider) TriggerFocusNode() *focus.FocusNode {
	if s == nil {
		return nil
	}
	return s.focusNode
}

// TriggerRole is button when visible.
func (s *Sider) TriggerRole() string {
	if !s.TriggerVisible() {
		return ""
	}
	return "button"
}

// TriggerAriaLabel names the trigger.
func (s *Sider) TriggerAriaLabel() string {
	if !s.TriggerVisible() {
		return ""
	}
	if s.CollapsedState() {
		return "expand sider"
	}
	return "collapse sider"
}

// FocusTrigger marks trigger focused (headless helper).
func (s *Sider) FocusTrigger() {
	if s == nil {
		return
	}
	s.focused = true
	s.node.MarkNeedsPaint()
}

// BlurTrigger clears focus.
func (s *Sider) BlurTrigger() {
	if s == nil {
		return
	}
	s.focused = false
	s.node.MarkNeedsPaint()
}

// TriggerHasFocus reports focus.
func (s *Sider) TriggerHasFocus() bool {
	if s == nil {
		return false
	}
	if s.focusNode != nil && s.focusNode.HasFocus() {
		return true
	}
	return s.focused
}

// SetTriggerHovered sets hover (paint only).
func (s *Sider) SetTriggerHovered(b bool) {
	if s == nil {
		return
	}
	s.hovered = b
	s.node.MarkNeedsPaint()
}

// TriggerHovered reports hover.
func (s *Sider) TriggerHovered() bool { return s != nil && s.hovered }

// ActivateTrigger toggles collapse (click path).
func (s *Sider) ActivateTrigger() {
	if s == nil || !s.TriggerVisible() {
		return
	}
	next := !s.CollapsedState()
	if s.collapsedSet {
		s.collapsed = next
	} else {
		s.internalCollapsed = next
	}
	if s.onCollapse != nil {
		s.onCollapse(next, CollapseClickTrigger)
	}
	s.node.MarkNeedsLayout()
	s.node.MarkNeedsPaint()
}

// HandleTriggerKey toggles on Enter/Space.
func (s *Sider) HandleTriggerKey(code int) bool {
	if s == nil || !s.TriggerFocusable() {
		return false
	}
	if code == focus.KeySpace || code == focus.KeyEnter || code == focus.KeyReturn {
		s.ActivateTrigger()
		return true
	}
	return false
}

// HitTrigger reports whether local point hits the trigger.
func (s *Sider) HitTrigger(x, y float64) bool {
	if !s.TriggerVisible() {
		return false
	}
	sz := s.node.Size()
	w, h := sz.Width, sz.Height
	if w <= 0 || h <= 0 {
		w = s.EffectiveSiderWidth()
		h = w
	}
	if s.IsZeroTrigger() {
		tw := s.EffectiveTriggerWidth()
		th := s.EffectiveTriggerHeight()
		return x >= 0 && y >= 0 && x < tw && y < th
	}
	th := s.EffectiveTriggerHeight()
	tw := s.EffectiveSiderWidth()
	if tw <= 0 {
		tw = w
	}
	return x >= 0 && y >= h-th && x < tw && y < h
}

// SetOverlay enables covering collapse.
func (s *Sider) SetOverlay(b bool) {
	if s == nil {
		return
	}
	s.overlay = b
	s.node.MarkNeedsLayout()
	s.node.MarkNeedsPaint()
}

// Overlay reports the flag.
func (s *Sider) Overlay() bool { return s != nil && s.overlay }

// SetOnCollapse installs the callback.
func (s *Sider) SetOnCollapse(fn func(bool, CollapseType)) {
	if s == nil {
		return
	}
	s.onCollapse = fn
}

// SetOnBreakpoint installs the callback.
func (s *Sider) SetOnBreakpoint(fn func(bool)) {
	if s == nil {
		return
	}
	s.onBreakpoint = fn
}

// SetBackground overrides sider bg (paint only).
func (s *Sider) SetBackground(c render.RGBA) {
	if s == nil {
		return
	}
	s.bg = c
	s.hasBg = true
	s.node.MarkNeedsPaint()
}

// EffectiveBackground returns sider bg.
func (s *Sider) EffectiveBackground() render.RGBA {
	if s != nil && s.hasBg {
		return s.bg
	}
	if s != nil && s.siderTheme == SiderThemeLight {
		return themeToRGBA(s.themeTokens().ColorBgContainer)
	}
	return render.Hex(siderDarkBgHex)
}

// EffectiveTriggerBackground returns trigger bg.
func (s *Sider) EffectiveTriggerBackground() render.RGBA {
	tok := s.themeTokens()
	if s != nil && s.siderTheme == SiderThemeLight {
		if tok.ColorBorder.R+tok.ColorBorder.G+tok.ColorBorder.B > 0 {
			return themeToRGBA(tok.ColorBorder)
		}
		return render.Hex("#d9d9d9")
	}
	base := render.Hex(triggerDarkBgHex)
	if s != nil && s.hovered {
		// Hover feedback: lighten slightly.
		return render.RGBA{R: base.R + 0.08, G: base.G + 0.08, B: base.B + 0.08, A: base.A}
	}
	return base
}

// SetProvider selects theme source.
func (s *Sider) SetProvider(p *theme.Provider) {
	if s == nil {
		return
	}
	s.provider = p
	s.node.MarkNeedsPaint()
}

// SetThemeTokens pins exact tokens.
func (s *Sider) SetThemeTokens(t *theme.Tokens) {
	if s == nil {
		return
	}
	s.override = t
	s.node.MarkNeedsPaint()
}

// SetTheme is an alias keeping the spec name.
func (s *Sider) SetThemeAlias(t SiderTheme) { s.SetTheme(t) }

// SetAriaLabel names the container.
func (s *Sider) SetAriaLabel(v string) {
	if s == nil {
		return
	}
	s.aria = v
}

// AriaLabel returns the name.
func (s *Sider) AriaLabel() string {
	if s == nil {
		return ""
	}
	return s.aria
}

// Role has no forced role.
func (s *Sider) Role() string { return "" }

// Add appends content children.
func (s *Sider) Add(children ...rendering.RenderObject) {
	if s == nil {
		return
	}
	for _, c := range children {
		if c == nil || (s.hasCustomTrigger && c == s.triggerNode) {
			continue
		}
		s.node.AddChild(c)
	}
}

// SetChildren replaces content children (keeps custom trigger).
func (s *Sider) SetChildren(children ...rendering.RenderObject) {
	if s == nil {
		return
	}
	keep := s.triggerNode
	hasCustom := s.hasCustomTrigger && keep != nil && s.TriggerVisible()
	for _, c := range s.node.Children() {
		if hasCustom && c == keep {
			continue
		}
		s.node.RemoveChild(c)
	}
	s.Add(children...)
	s.updateTriggerChild()
}

// ClearChildren removes content (keeps custom trigger).
func (s *Sider) ClearChildren() { s.SetChildren() }

// Node returns the tree node.
func (s *Sider) Node() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.node
}

// Layout sizes the node.
func (s *Sider) Layout(c rendering.Constraints) rendering.Size {
	if s == nil {
		return rendering.Size{}
	}
	return s.sectionLayout(c)
}

func (s *Sider) sectionLayout(c rendering.Constraints) rendering.Size {
	prefW := s.EffectiveSiderWidth()
	prefH := c.MaxHeight
	if !bounded(prefH) {
		prefH = c.MinHeight
	}
	out := c.Tighten(rendering.Size{Width: prefW, Height: prefH})
	s.node.FixedWidth = out.Width
	s.node.FixedHeight = out.Height
	s.node.Layout(c)
	return out
}

func (s *Sider) syncFocusEnabled() {
	if s == nil || s.focusNode == nil {
		return
	}
	s.focusNode.Enabled = s.TriggerVisible()
	s.focusNode.TabIndex = 0
}

func (s *Sider) updateTriggerChild() {
	if s == nil || s.node == nil {
		return
	}
	if s.hasCustomTrigger && s.triggerNode != nil && s.TriggerVisible() {
		found := false
		for _, c := range s.node.Children() {
			if c == s.triggerNode {
				found = true
				break
			}
		}
		if !found {
			s.node.AddChild(s.triggerNode)
		}
		return
	}
	if s.hasCustomTrigger && s.triggerNode != nil && !s.TriggerVisible() {
		s.node.RemoveChild(s.triggerNode)
	}
}

func (s *Sider) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || size.Width < 0 || size.Height <= 0 {
		return
	}
	w := size.Width
	if w < 0 {
		w = 0
	}
	if w > 0 {
		bg := s.EffectiveBackground()
		rendering.FillRect(pc, 0, 0, w, size.Height, bg.R, bg.G, bg.B, bg.A)
	}
	if !s.TriggerVisible() {
		return
	}
	tbg := s.EffectiveTriggerBackground()
	if s.IsZeroTrigger() {
		tw := s.EffectiveTriggerWidth()
		th := s.EffectiveTriggerHeight()
		rendering.FillRect(pc, 0, 0, tw, th, tbg.R, tbg.G, tbg.B, tbg.A)
		s.paintArrow(pc, 0, 0, tw, th)
		if s.TriggerHasFocus() {
			s.paintFocusRing(pc, 0, 0, tw, th)
		}
		return
	}
	if w <= 0 {
		return
	}
	th := s.EffectiveTriggerHeight()
	if th > size.Height {
		th = size.Height
	}
	y := size.Height - th
	rendering.FillRect(pc, 0, y, w, th, tbg.R, tbg.G, tbg.B, tbg.A)
	s.paintArrow(pc, 0, y, w, th)
	if s.TriggerHasFocus() {
		s.paintFocusRing(pc, 0, y, w, th)
	}
}

func (s *Sider) paintArrow(pc *rendering.PaintContext, x, y, w, h float64) {
	if pc == nil || w <= 0 || h <= 0 {
		return
	}
	var r, g, b, a float64
	if s.siderTheme == SiderThemeLight {
		tok := s.themeTokens()
		c := themeToRGBA(tok.ColorText)
		r, g, b, a = c.R, c.G, c.B, 1
	} else {
		r, g, b, a = 1, 1, 1, 1
	}
	cx, cy := x+w/2, y+h/2
	arm := 7.0
	if w < 40 {
		arm = w * 0.18
	}
	if s.ArrowPointsLeft() {
		rendering.StrokeLine(pc, cx+arm*0.6, cy-arm, cx-arm*0.6, cy, 2, r, g, b, a)
		rendering.StrokeLine(pc, cx-arm*0.6, cy, cx+arm*0.6, cy+arm, 2, r, g, b, a)
		return
	}
	rendering.StrokeLine(pc, cx-arm*0.6, cy-arm, cx+arm*0.6, cy, 2, r, g, b, a)
	rendering.StrokeLine(pc, cx+arm*0.6, cy, cx-arm*0.6, cy+arm, 2, r, g, b, a)
}

func (s *Sider) paintFocusRing(pc *rendering.PaintContext, x, y, w, h float64) {
	if pc == nil {
		return
	}
	tok := s.themeTokens()
	c := themeToRGBA(tok.ColorPrimary)
	if c.A <= 0 {
		c = render.RGBA{R: 0.09, G: 0.47, B: 1, A: 1}
	}
	rendering.StrokeRect(pc, x+1.5, y+1.5, w-3, h-3, 2, c.R, c.G, c.B, c.A)
}

// Layout is the root container.
type Layout struct {
	root        *rendering.AbsoluteBox
	sections    []LayoutSection
	hasSider    bool
	hasSiderSet bool
	provider    *theme.Provider
	override    *theme.Tokens
	bg          render.RGBA
	hasBg       bool
	aria        string
}

// NewLayout creates a container.
func NewLayout(sections ...LayoutSection) *Layout {
	l := &Layout{}
	l.root = rendering.NewAbsoluteBox(0, 0)
	l.root.SetRepaintBoundary(true)
	l.root.SetDebugName(layoutRootTag)
	l.Add(sections...)
	l.syncBackground()
	return l
}

func (l *Layout) sectionKind() string { return "layout" }

func (l *Layout) themeTokens() theme.Tokens {
	if l != nil && l.override != nil {
		return *l.override
	}
	if l != nil && l.provider != nil {
		return l.provider.Current()
	}
	return theme.Default.Current()
}

// SetHasSider forces row direction.
func (l *Layout) SetHasSider(b bool) {
	if l == nil {
		return
	}
	l.hasSider = b
	l.hasSiderSet = true
	l.root.MarkNeedsLayout()
}

// HasSiderEffective reports row direction.
func (l *Layout) HasSiderEffective() bool {
	if l == nil {
		return false
	}
	if l.hasSiderSet {
		return l.hasSider
	}
	for _, s := range l.sections {
		if s != nil && s.sectionKind() == "sider" {
			return true
		}
	}
	return false
}

// IsRow reports horizontal layout.
func (l *Layout) IsRow() bool { return l.HasSiderEffective() }

// SetProvider selects theme source.
func (l *Layout) SetProvider(p *theme.Provider) {
	if l == nil {
		return
	}
	l.provider = p
	l.syncBackground()
	l.root.MarkNeedsPaint()
}

// SetTheme pins exact tokens.
func (l *Layout) SetTheme(t *theme.Tokens) {
	if l == nil {
		return
	}
	l.override = t
	l.syncBackground()
	l.root.MarkNeedsPaint()
}

// SetBackground overrides body bg (paint only).
func (l *Layout) SetBackground(c render.RGBA) {
	if l == nil {
		return
	}
	l.bg = c
	l.hasBg = true
	l.syncBackground()
	l.root.MarkNeedsPaint()
}

// EffectiveBackground returns body bg.
func (l *Layout) EffectiveBackground() render.RGBA {
	if l != nil && l.hasBg {
		return l.bg
	}
	return themeToRGBA(l.themeTokens().ColorBgLayout)
}

// SetAriaLabel names the container.
func (l *Layout) SetAriaLabel(s string) {
	if l == nil {
		return
	}
	l.aria = s
}

// AriaLabel returns the name.
func (l *Layout) AriaLabel() string {
	if l == nil {
		return ""
	}
	return l.aria
}

// Role has no forced role.
func (l *Layout) Role() string { return "" }

// Focusable is always false for the container.
func (l *Layout) Focusable() bool { return false }

// Add appends sections.
func (l *Layout) Add(sections ...LayoutSection) {
	if l == nil {
		return
	}
	for _, s := range sections {
		if s == nil {
			continue
		}
		l.sections = append(l.sections, s)
		n := s.Node()
		if n != nil {
			// AbsoluteBox.Place adds and offsets; initial offset zero.
			l.root.Place(n, 0, 0)
		}
	}
}

// SetChildren replaces sections.
func (l *Layout) SetChildren(sections ...LayoutSection) {
	if l == nil {
		return
	}
	for _, s := range l.sections {
		if s == nil {
			continue
		}
		if n := s.Node(); n != nil {
			l.root.RemoveChild(n)
		}
	}
	l.sections = nil
	l.Add(sections...)
}

// ClearChildren removes all sections.
func (l *Layout) ClearChildren() { l.SetChildren() }

// Sections returns direct children.
func (l *Layout) Sections() []LayoutSection {
	if l == nil {
		return nil
	}
	return append([]LayoutSection(nil), l.sections...)
}

// Node returns the tree node.
func (l *Layout) Node() rendering.RenderObject {
	if l == nil {
		return nil
	}
	return l.root
}

// Layout arranges sections.
func (l *Layout) Layout(c rendering.Constraints) rendering.Size {
	if l == nil {
		return rendering.Size{}
	}
	return l.sectionLayout(c)
}

func (l *Layout) sectionLayout(c rendering.Constraints) rendering.Size {
	w := c.MaxWidth
	if !bounded(w) {
		w = c.MinWidth
	}
	h := c.MaxHeight
	if !bounded(h) {
		h = c.MinHeight
	}
	out := c.Tighten(rendering.Size{Width: w, Height: h})
	l.root.FixedWidth = out.Width
	l.root.FixedHeight = out.Height
	l.syncBackground()
	if l.IsRow() {
		l.layoutRow(out)
	} else {
		l.layoutColumn(out)
	}
	l.root.Layout(rendering.Tight(out.Width, out.Height))
	return out
}

func (l *Layout) layoutRow(out rendering.Size) {
	var siders []*Sider
	var flex []LayoutSection
	totalFlow := 0.0
	for _, s := range l.sections {
		if sd, ok := s.(*Sider); ok && sd != nil {
			siders = append(siders, sd)
			totalFlow += sd.FlowWidth()
			continue
		}
		flex = append(flex, s)
	}
	_ = siders
	remain := out.Width - totalFlow
	if remain < 0 {
		remain = 0
	}
	flexW := 0.0
	if len(flex) > 0 {
		flexW = remain / float64(len(flex))
	}
	x := 0.0
	var overlays []*Sider
	for _, s := range l.sections {
		if sd, ok := s.(*Sider); ok && sd != nil {
			paintW := sd.EffectiveSiderWidth()
			flowW := sd.FlowWidth()
			sd.sectionLayout(rendering.Tight(paintW, out.Height))
			if sd.Overlay() {
				l.root.Place(sd.Node(), 0, 0)
				overlays = append(overlays, sd)
			} else {
				l.root.Place(sd.Node(), x, 0)
				x += flowW
			}
			continue
		}
		s.sectionLayout(rendering.Tight(flexW, out.Height))
		l.root.Place(s.Node(), x, 0)
		x += flexW
	}
	// Overlay siders cover the content flow, so they paint last (above).
	for _, sd := range overlays {
		if n := sd.Node(); n != nil {
			l.root.RemoveChild(n)
			l.root.Place(n, 0, 0)
		}
	}
}

func (l *Layout) layoutColumn(out rendering.Size) {
	type fixed struct {
		sec LayoutSection
		h   float64
	}
	var order []LayoutSection
	var flex []LayoutSection
	fixedH := 0.0
	for _, s := range l.sections {
		switch t := s.(type) {
		case *Header:
			h := t.EffectiveHeight()
			fixedH += h
			order = append(order, s)
		case *Footer:
			h := t.EffectiveHeight()
			fixedH += h
			order = append(order, s)
		case *Content, *Layout:
			flex = append(flex, s)
			order = append(order, s)
		case *Sider:
			// Forced column with sider: treat as flex full width.
			flex = append(flex, s)
			order = append(order, s)
		default:
			flex = append(flex, s)
			order = append(order, s)
		}
	}
	remain := out.Height - fixedH
	if remain < 0 {
		remain = 0
	}
	flexH := 0.0
	if len(flex) > 0 {
		flexH = remain / float64(len(flex))
	}
	y := 0.0
	for _, s := range order {
		switch t := s.(type) {
		case *Header:
			h := t.EffectiveHeight()
			s.sectionLayout(rendering.Tight(out.Width, h))
			l.root.Place(s.Node(), 0, y)
			y += h
		case *Footer:
			h := t.EffectiveHeight()
			s.sectionLayout(rendering.Tight(out.Width, h))
			l.root.Place(s.Node(), 0, y)
			y += h
		default:
			s.sectionLayout(rendering.Tight(out.Width, flexH))
			l.root.Place(s.Node(), 0, y)
			y += flexH
		}
	}
}

func (l *Layout) syncBackground() {
	if l == nil || l.root == nil {
		return
	}
	bg := l.EffectiveBackground()
	l.root.Background = &rendering.Color{R: bg.R, G: bg.G, B: bg.B, A: bg.A}
}
