package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Layout — components/layout/{layout,Sider}.tsx + style/
// Product contract: docs/antd/layout.md §6 (P0 DoD).
// https://ant.design/components/layout

const (
	// Type IDs for hasSider / flex-role detection.
	TypeLayout  = "kit.Layout"
	TypeHeader  = "kit.Header"
	TypeFooter  = "kit.Footer"
	TypeContent = "kit.Content"
	TypeSider   = "kit.Sider"

	// Defaults — antd prepareComponentToken + Sider props.
	DefaultLayoutHeaderHeight        = 64.0 // controlHeight×2
	DefaultLayoutHeaderPaddingInline = 50.0 // controlHeightLG×1.25
	DefaultLayoutFooterPaddingBlock  = 24.0 // controlHeightSM
	DefaultLayoutFooterPaddingInline = 50.0
	DefaultSiderWidth                = 200.0
	DefaultSiderCollapsedWidth       = 80.0
	DefaultSiderTriggerHeight        = 48.0 // controlHeightLG + marginXXS×2
	DefaultSiderZeroTriggerSize      = 40.0 // controlHeightLG
	DefaultLayoutFontSize            = 14.0
	DefaultLayoutBorderRadius        = 6.0
	DefaultLayoutLineWidth           = 1.0
)

// Layout chrome colors (antd component tokens; not brand primary).
var (
	DefaultLayoutHeaderBg  = render.Hex("#001529")
	DefaultLayoutSiderBg   = render.Hex("#001529")
	DefaultLayoutTriggerBg = render.Hex("#002140")
)

// SiderTheme is antd Sider.theme: dark | light.
type SiderTheme int

const (
	SiderThemeDark SiderTheme = iota
	SiderThemeLight
)

// CollapseType is antd onCollapse second arg.
type CollapseType int

const (
	CollapseClickTrigger CollapseType = iota
	CollapseResponsive
)

// LayoutBreakpoint is antd Sider.breakpoint (Grid screen keys).
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

// breakpointMaxWidth is antd dimensionMaxMap (max-width media).
func breakpointMaxWidth(bp LayoutBreakpoint) float64 {
	switch bp {
	case LayoutBreakpointXS:
		return 479.98
	case LayoutBreakpointSM:
		return 575.98
	case LayoutBreakpointMD:
		return 767.98
	case LayoutBreakpointLG:
		return 991.98
	case LayoutBreakpointXL:
		return 1199.98
	case LayoutBreakpointXXL:
		return 1599.98
	case LayoutBreakpointXXXL:
		return 1839.98
	default:
		return -1
	}
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// Layout is antd Layout: flex column by default; row when hasSider.
//
//	Layout (Flex)
//	  ├─ Header / Footer (fixed)
//	  ├─ Sider (fixed width)
//	  ├─ Content / nested Layout (Flexible grow)
//	  └─ Overlay Sider via Stack when Sider.Overlay
//
// hit == layout == paint via primitive.Flex / Decorated.
type Layout struct {
	Root *layoutRoot
	flex *primitive.Flex

	HasSider    bool
	hasSiderSet bool
	AriaLabel   string
	Theme       *core.Theme

	bgSet bool
	bg    render.RGBA

	// Overlay host when a child Sider uses Overlay.
	stack *primitive.Stack
}

type layoutRoot struct {
	core.NodeBase
	owner *Layout
}

func (r *layoutRoot) TypeID() string { return TypeLayout }

func (r *layoutRoot) Layout(c core.Constraints) core.Size {
	if r == nil || r.owner == nil {
		out := c.Tighten(core.Size{})
		if r != nil {
			r.SetSize(out)
		}
		return out
	}
	if sz, ok := r.LayoutSkipIfClean(c); ok {
		return sz
	}
	r.owner.syncDirection(c)
	kids := r.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		r.SetSize(out)
		r.RememberConstraints(c)
		return out
	}
	// Single child host (flex or stack) fills the layout box.
	child := kids[0]
	sz := child.Layout(c)
	child.Base().SetOffset(core.Point{})
	// ExpandMax-like fill when parent gives finite max.
	out := sz
	if c.HasBoundedWidth() && c.MaxWidth < core.Unbounded && out.Width < c.MaxWidth {
		out.Width = c.MaxWidth
	}
	if c.HasBoundedHeight() && c.MaxHeight < core.Unbounded && out.Height < c.MaxHeight {
		out.Height = c.MaxHeight
	}
	out = c.Tighten(out)
	// Re-layout child tight to final size so Flexible children expand.
	if out.Width != sz.Width || out.Height != sz.Height {
		tight := core.Tight(out.Width, out.Height)
		_ = child.Layout(tight)
		child.Base().SetOffset(core.Point{})
	}
	r.SetSize(out)
	r.RememberConstraints(c)
	return out
}

func (r *layoutRoot) Paint(pc *core.PaintContext) {
	if r == nil {
		return
	}
	sz := r.Size()
	if r.owner != nil {
		bg := r.owner.bodyBg()
		if bg.A > 0 && pc != nil {
			pc.FillLocalRect(0, 0, sz.Width, sz.Height, bg)
		}
	}
	r.DefaultPaintChildren(pc)
	if pc != nil {
		r.ClearPaintDirty()
	}
}

func (r *layoutRoot) HitTest(p core.Point) core.Node { return r.DefaultHitTest(p) }

// NewLayout creates an antd Layout with optional region children.
func NewLayout(children ...core.Node) *Layout {
	l := &Layout{}
	l.Root = &layoutRoot{owner: l}
	l.Root.Init(l.Root)
	l.Root.Hit = core.HitDefer
	l.flex = primitive.Column()
	l.flex.CrossAlign = core.CrossStretch
	l.flex.MainAlign = core.MainStart
	l.flex.ExpandMax = true
	l.Root.AddChild(l.flex)
	l.SetChildren(children...)
	l.applyA11y()
	return l
}

// Node returns the stable mount root.
func (l *Layout) Node() core.Node {
	if l == nil {
		return nil
	}
	if l.Root == nil {
		l.Root = &layoutRoot{owner: l}
		l.Root.Init(l.Root)
		l.Root.Hit = core.HitDefer
		l.flex = primitive.Column()
		l.flex.CrossAlign = core.CrossStretch
		l.flex.ExpandMax = true
		l.Root.AddChild(l.flex)
	}
	return l.Root
}

// ChromeNode is the layout root (visual tests).
func (l *Layout) ChromeNode() core.Node { return l.Node() }

// SetHasSider forces has-sider row direction (antd hasSider).
func (l *Layout) SetHasSider(v bool) {
	if l == nil {
		return
	}
	l.HasSider = v
	l.hasSiderSet = true
	l.rebuild()
}

// SetTheme stores product theme.
func (l *Layout) SetTheme(th *core.Theme) {
	if l == nil {
		return
	}
	l.Theme = th
	l.markPaint()
}

// SetAriaLabel sets optional accessible name.
func (l *Layout) SetAriaLabel(s string) {
	if l == nil {
		return
	}
	l.AriaLabel = s
	l.applyA11y()
}

// SetBackground overrides body background (default colorBgLayout).
func (l *Layout) SetBackground(c render.RGBA) {
	if l == nil {
		return
	}
	l.bg = c
	l.bgSet = true
	l.markPaint()
}

// Add appends a child region node.
func (l *Layout) Add(n core.Node) {
	if l == nil || n == nil {
		return
	}
	_ = l.Node()
	// Children live on flex (or will be rebuilt onto stack).
	l.rebuildWith(append(l.childNodes(), n)...)
}

// SetChildren replaces all children.
func (l *Layout) SetChildren(children ...core.Node) {
	if l == nil {
		return
	}
	_ = l.Node()
	l.rebuildWith(children...)
}

// ClearChildren removes all children.
func (l *Layout) ClearChildren() {
	if l == nil {
		return
	}
	l.rebuildWith()
}

func (l *Layout) applyA11y() {
	if l == nil || l.Root == nil {
		return
	}
	l.Root.Base().Label = l.AriaLabel
}

func (l *Layout) theme() *core.Theme {
	return themeOf(l.Theme, l.Node())
}

func (l *Layout) bodyBg() render.RGBA {
	if l != nil && l.bgSet {
		return l.bg
	}
	return l.theme().Color(core.TokenColorBgLayout)
}

func (l *Layout) markPaint() {
	if l != nil && l.Root != nil {
		l.Root.MarkNeedsPaint()
	}
}

func (l *Layout) mark() {
	if l != nil && l.Root != nil {
		l.Root.MarkNeedsLayout()
	}
}

func (l *Layout) childNodes() []core.Node {
	if l == nil || l.flex == nil {
		return nil
	}
	// Prefer flex children; if stack mode, recover from stack structure.
	if l.stack != nil && l.Root != nil {
		// stack kids: [flowFlex, positioned sider...]
		var out []core.Node
		for _, k := range l.stack.Children() {
			if k == l.flex {
				for _, c := range l.flex.Children() {
					out = append(out, unwrapFlexHost(c))
				}
				continue
			}
			if s := unwrapPositioned(k); s != nil {
				out = append(out, s)
			}
		}
		return out
	}
	var out []core.Node
	for _, c := range l.flex.Children() {
		out = append(out, unwrapFlexHost(c))
	}
	return out
}

func unwrapFlexHost(n core.Node) core.Node {
	if n == nil {
		return nil
	}
	// Flexible hosts Content/Layout.
	if n.TypeID() == primitive.TypeFlexible {
		kids := n.Base().Children()
		if len(kids) > 0 {
			return kids[0]
		}
	}
	return n
}

func unwrapPositioned(n core.Node) core.Node {
	if n == nil {
		return nil
	}
	if n.TypeID() == "primitive.Positioned" {
		kids := n.Base().Children()
		if len(kids) > 0 {
			return kids[0]
		}
	}
	return nil
}

func (l *Layout) rebuild() {
	if l == nil {
		return
	}
	l.rebuildWith(l.childNodes()...)
}

func (l *Layout) rebuildWith(children ...core.Node) {
	if l == nil {
		return
	}
	_ = l.Node()
	if l.flex == nil {
		l.flex = primitive.Column()
		l.flex.CrossAlign = core.CrossStretch
		l.flex.ExpandMax = true
	}

	// Partition overlay siders vs flow children.
	var flow []core.Node
	var overlays []*Sider
	hasSider := l.hasSiderSet && l.HasSider
	for _, c := range children {
		if c == nil {
			continue
		}
		if s := siderOf(c); s != nil {
			hasSider = true
			if s.Overlay {
				overlays = append(overlays, s)
				continue
			}
		} else if c.TypeID() == TypeSider {
			hasSider = true
		}
		flow = append(flow, c)
	}
	if l.hasSiderSet {
		hasSider = l.HasSider
	}
	l.HasSider = hasSider

	if hasSider {
		l.flex.Axis = core.AxisHorizontal
	} else {
		l.flex.Axis = core.AxisVertical
	}
	l.flex.CrossAlign = core.CrossStretch
	l.flex.MainAlign = core.MainStart
	l.flex.ClearChildren()

	for _, c := range flow {
		l.flex.AddChild(l.wrapChild(c, hasSider))
	}

	l.Root.ClearChildren()
	if len(overlays) > 0 {
		l.stack = primitive.NewStack()
		l.stack.Fit = true
		// Flow fills stack.
		flowHost := primitive.NewFlexible(1, l.flex)
		flowHost.FillChild = true
		// Stack sizes to constraints; put flex directly.
		l.stack.AddChild(l.flex)
		for _, s := range overlays {
			// Align start (left/top).
			l.stack.AddChild(primitive.Positioned(core.AlignTopLeft, s.Node()))
		}
		l.Root.AddChild(l.stack)
	} else {
		l.stack = nil
		l.Root.AddChild(l.flex)
	}
	l.applyA11y()
	l.mark()
}

func (l *Layout) wrapChild(c core.Node, hasSider bool) core.Node {
	if c == nil {
		return nil
	}
	tid := c.TypeID()
	switch tid {
	case TypeSider, TypeHeader, TypeFooter:
		return c
	case TypeContent, TypeLayout:
		host := primitive.NewFlexible(1, c)
		host.FillChild = true
		return host
	default:
		// Raw nodes: grow in hasSider row / column content area.
		if hasSider || tid == primitive.TypeFlex {
			host := primitive.NewFlexible(1, c)
			host.FillChild = true
			return host
		}
		// Column layout raw: treat as content grow for typical pages.
		host := primitive.NewFlexible(1, c)
		host.FillChild = true
		return host
	}
}

func (l *Layout) syncDirection(c core.Constraints) {
	if l == nil {
		return
	}
	// Re-evaluate responsive siders against viewport.
	vp := 0.0
	if c.HasBoundedWidth() && c.MaxWidth < core.Unbounded {
		vp = c.MaxWidth
	}
	for _, n := range l.childNodes() {
		if s := siderOf(n); s != nil {
			s.applyBreakpoint(vp)
			s.applyChrome()
		}
	}
	// Direction already set in rebuild; ensure axis matches hasSider.
	if l.flex != nil {
		if l.HasSider {
			l.flex.Axis = core.AxisHorizontal
		} else {
			l.flex.Axis = core.AxisVertical
		}
	}
}

func siderOf(n core.Node) *Sider {
	if n == nil {
		return nil
	}
	if s, ok := n.(*siderRoot); ok && s.owner != nil {
		return s.owner
	}
	// Node() may return siderRoot directly.
	if n.TypeID() == TypeSider {
		if s, ok := n.(*siderRoot); ok {
			return s.owner
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Header
// ---------------------------------------------------------------------------

// Header is antd Layout.Header (height 64, dark chrome by default).
type Header struct {
	Root *headerRoot

	Height    float64 // 0 → DefaultLayoutHeaderHeight
	Theme     *core.Theme
	AriaLabel string

	padSet bool
	pad    primitive.EdgeInsets
	bgSet  bool
	bg     render.RGBA
}

type headerRoot struct {
	primitive.Decorated
	owner *Header
}

func (h *headerRoot) TypeID() string { return TypeHeader }

// NewHeader creates a Layout.Header.
func NewHeader(children ...core.Node) *Header {
	h := &Header{}
	h.Root = &headerRoot{owner: h}
	h.Root.Init(h.Root)
	h.Root.Hit = core.HitDefer
	h.Root.StretchChild = true
	for _, c := range children {
		if c != nil {
			h.Root.AddChild(c)
		}
	}
	h.apply()
	return h
}

// Node returns the header root.
func (h *Header) Node() core.Node {
	if h == nil {
		return nil
	}
	if h.Root == nil {
		h.Root = &headerRoot{owner: h}
		h.Root.Init(h.Root)
		h.Root.Hit = core.HitDefer
		h.Root.StretchChild = true
		h.apply()
	}
	return h.Root
}

// SetHeight sets header height (0 → default 64).
func (h *Header) SetHeight(v float64) {
	if h == nil {
		return
	}
	h.Height = v
	h.apply()
}

// SetPaddingInsets sets header padding (explicit, including zero).
func (h *Header) SetPaddingInsets(p primitive.EdgeInsets) {
	if h == nil {
		return
	}
	h.pad = p
	h.padSet = true
	h.apply()
}

// SetBackground overrides header background.
func (h *Header) SetBackground(c render.RGBA) {
	if h == nil {
		return
	}
	h.bg = c
	h.bgSet = true
	h.apply()
}

// SetTheme stores theme.
func (h *Header) SetTheme(th *core.Theme) {
	if h == nil {
		return
	}
	h.Theme = th
	h.apply()
}

// SetAriaLabel sets accessible name.
func (h *Header) SetAriaLabel(s string) {
	if h == nil {
		return
	}
	h.AriaLabel = s
	h.applyA11y()
}

// SetChildren replaces children.
func (h *Header) SetChildren(children ...core.Node) {
	if h == nil {
		return
	}
	_ = h.Node()
	h.Root.ClearChildren()
	for _, c := range children {
		if c != nil {
			h.Root.AddChild(c)
		}
	}
	h.apply()
}

func (h *Header) applyA11y() {
	if h == nil || h.Root == nil {
		return
	}
	h.Root.Base().Role = "banner"
	h.Root.Base().Label = h.AriaLabel
}

func (h *Header) apply() {
	if h == nil {
		return
	}
	_ = h.Node()
	th := themeOf(h.Theme, h.Root)
	hh := DefaultLayoutHeaderHeight
	if h.Height > 0 {
		hh = h.Height
	} else {
		ch := th.SizeOr(core.TokenControlHeight, 32)
		if ch > 0 {
			hh = ch * 2
		}
	}
	h.Root.Height = hh
	h.Root.MinHeight = hh
	if h.padSet {
		h.Root.Padding = h.pad
	} else {
		inline := DefaultLayoutHeaderPaddingInline
		if lg := th.SizeOr(core.TokenControlHeightLG, 40); lg > 0 {
			inline = lg * 1.25
		}
		h.Root.Padding = primitive.EdgeInsets{Left: inline, Right: inline}
	}
	if h.bgSet {
		h.Root.Background = h.bg
	} else {
		h.Root.Background = DefaultLayoutHeaderBg
	}
	h.Root.ExpandWidth = true
	h.Root.StretchChild = true
	h.applyA11y()
	h.Root.MarkNeedsLayout()
}

// ---------------------------------------------------------------------------
// Footer
// ---------------------------------------------------------------------------

// Footer is antd Layout.Footer.
type Footer struct {
	Root *footerRoot

	Theme     *core.Theme
	AriaLabel string

	padSet bool
	pad    primitive.EdgeInsets
	bgSet  bool
	bg     render.RGBA
}

type footerRoot struct {
	primitive.Decorated
	owner *Footer
}

func (f *footerRoot) TypeID() string { return TypeFooter }

// NewFooter creates a Layout.Footer.
func NewFooter(children ...core.Node) *Footer {
	f := &Footer{}
	f.Root = &footerRoot{owner: f}
	f.Root.Init(f.Root)
	f.Root.Hit = core.HitDefer
	f.Root.StretchChild = true
	for _, c := range children {
		if c != nil {
			f.Root.AddChild(c)
		}
	}
	f.apply()
	return f
}

// Node returns the footer root.
func (f *Footer) Node() core.Node {
	if f == nil {
		return nil
	}
	if f.Root == nil {
		f.Root = &footerRoot{owner: f}
		f.Root.Init(f.Root)
		f.Root.Hit = core.HitDefer
		f.apply()
	}
	return f.Root
}

// SetPaddingInsets sets footer padding.
func (f *Footer) SetPaddingInsets(p primitive.EdgeInsets) {
	if f == nil {
		return
	}
	f.pad = p
	f.padSet = true
	f.apply()
}

// SetBackground overrides footer background.
func (f *Footer) SetBackground(c render.RGBA) {
	if f == nil {
		return
	}
	f.bg = c
	f.bgSet = true
	f.apply()
}

// SetTheme stores theme.
func (f *Footer) SetTheme(th *core.Theme) {
	if f == nil {
		return
	}
	f.Theme = th
	f.apply()
}

// SetAriaLabel sets accessible name.
func (f *Footer) SetAriaLabel(s string) {
	if f == nil {
		return
	}
	f.AriaLabel = s
	f.applyA11y()
}

// SetChildren replaces children.
func (f *Footer) SetChildren(children ...core.Node) {
	if f == nil {
		return
	}
	_ = f.Node()
	f.Root.ClearChildren()
	for _, c := range children {
		if c != nil {
			f.Root.AddChild(c)
		}
	}
	f.apply()
}

func (f *Footer) applyA11y() {
	if f == nil || f.Root == nil {
		return
	}
	f.Root.Base().Role = "contentinfo"
	f.Root.Base().Label = f.AriaLabel
}

func (f *Footer) apply() {
	if f == nil {
		return
	}
	_ = f.Node()
	th := themeOf(f.Theme, f.Root)
	if f.padSet {
		f.Root.Padding = f.pad
	} else {
		v := DefaultLayoutFooterPaddingBlock
		h := DefaultLayoutFooterPaddingInline
		if sm := th.SizeOr(core.TokenControlHeightSM, 24); sm > 0 {
			v = sm
		}
		if lg := th.SizeOr(core.TokenControlHeightLG, 40); lg > 0 {
			h = lg * 1.25
		}
		f.Root.Padding = primitive.EdgeInsets{Top: v, Bottom: v, Left: h, Right: h}
	}
	if f.bgSet {
		f.Root.Background = f.bg
	} else {
		f.Root.Background = th.Color(core.TokenColorBgLayout)
	}
	f.Root.ExpandWidth = true
	f.Root.StretchChild = true
	f.applyA11y()
	f.Root.MarkNeedsLayout()
}

// ---------------------------------------------------------------------------
// Content
// ---------------------------------------------------------------------------

// Content is antd Layout.Content (flex auto).
type Content struct {
	Root *contentRoot

	MinHeight float64
	Theme     *core.Theme
	AriaLabel string

	padSet bool
	pad    primitive.EdgeInsets
	bgSet  bool
	bg     render.RGBA
}

type contentRoot struct {
	primitive.Decorated
	owner *Content
}

func (c *contentRoot) TypeID() string { return TypeContent }

// NewContent creates a Layout.Content.
func NewContent(children ...core.Node) *Content {
	c := &Content{}
	c.Root = &contentRoot{owner: c}
	c.Root.Init(c.Root)
	c.Root.Hit = core.HitDefer
	c.Root.StretchChild = true
	for _, ch := range children {
		if ch != nil {
			c.Root.AddChild(ch)
		}
	}
	c.apply()
	return c
}

// Node returns the content root.
func (c *Content) Node() core.Node {
	if c == nil {
		return nil
	}
	if c.Root == nil {
		c.Root = &contentRoot{owner: c}
		c.Root.Init(c.Root)
		c.Root.Hit = core.HitDefer
		c.apply()
	}
	return c.Root
}

// SetMinHeight sets min content height.
func (c *Content) SetMinHeight(h float64) {
	if c == nil {
		return
	}
	c.MinHeight = h
	c.apply()
}

// SetPaddingInsets sets content padding.
func (c *Content) SetPaddingInsets(p primitive.EdgeInsets) {
	if c == nil {
		return
	}
	c.pad = p
	c.padSet = true
	c.apply()
}

// SetBackground overrides content background.
func (c *Content) SetBackground(col render.RGBA) {
	if c == nil {
		return
	}
	c.bg = col
	c.bgSet = true
	c.apply()
}

// SetTheme stores theme.
func (c *Content) SetTheme(th *core.Theme) {
	if c == nil {
		return
	}
	c.Theme = th
	c.apply()
}

// SetAriaLabel sets accessible name.
func (c *Content) SetAriaLabel(s string) {
	if c == nil {
		return
	}
	c.AriaLabel = s
	c.applyA11y()
}

// SetChildren replaces children.
func (c *Content) SetChildren(children ...core.Node) {
	if c == nil {
		return
	}
	_ = c.Node()
	c.Root.ClearChildren()
	for _, ch := range children {
		if ch != nil {
			c.Root.AddChild(ch)
		}
	}
	c.apply()
}

func (c *Content) applyA11y() {
	if c == nil || c.Root == nil {
		return
	}
	c.Root.Base().Role = "main"
	c.Root.Base().Label = c.AriaLabel
}

func (c *Content) apply() {
	if c == nil {
		return
	}
	_ = c.Node()
	if c.padSet {
		c.Root.Padding = c.pad
	} else {
		c.Root.Padding = primitive.EdgeInsets{}
	}
	if c.bgSet {
		c.Root.Background = c.bg
	} else {
		c.Root.Background = render.RGBA{} // transparent; parent bodyBg shows
	}
	c.Root.MinHeight = c.MinHeight
	c.Root.ExpandWidth = true
	c.Root.StretchChild = true
	c.applyA11y()
	c.Root.MarkNeedsLayout()
}

// ---------------------------------------------------------------------------
// Sider
// ---------------------------------------------------------------------------

// Sider is antd Layout.Sider: fixed-width collapsible side rail.
type Sider struct {
	Root *siderRoot

	Width             float64 // 0 → DefaultSiderWidth
	CollapsedWidth    float64 // meaningful when collapsedWidthSet; default 80
	collapsedWidthSet bool

	Collapsible      bool
	Collapsed        bool
	collapsedCtrl    bool // SetCollapsed used (controlled)
	DefaultCollapsed bool
	ReverseArrow     bool
	SiderTheme       SiderTheme
	Overlay          bool

	Breakpoint    LayoutBreakpoint
	breakpointSet bool
	ViewportWidth float64
	viewportSet   bool

	// Trigger: custom node; hideTrigger ≡ antd trigger={null}.
	Trigger     core.Node
	hideTrigger bool

	OnCollapse   func(collapsed bool, typ CollapseType)
	OnBreakpoint func(broken bool)

	Theme     *core.Theme
	AriaLabel string

	// Style overrides.
	bgSet bool
	bg    render.RGBA

	// Internal chrome.
	body    *primitive.Decorated
	col     *primitive.Flex
	trigger *primitive.Pressable
	trigLab *primitive.Text

	below        bool
	lastBroken   bool
	brokenInited bool
	zeroTrigger  *primitive.Pressable
	zeroTrigHost *primitive.Decorated
}

type siderRoot struct {
	primitive.Decorated
	owner *Sider
}

func (s *siderRoot) TypeID() string { return TypeSider }

// NewSider creates a Layout.Sider (theme dark, width 200).
func NewSider(children ...core.Node) *Sider {
	s := &Sider{
		SiderTheme:     SiderThemeDark,
		CollapsedWidth: DefaultSiderCollapsedWidth,
	}
	s.Root = &siderRoot{owner: s}
	s.Root.Init(s.Root)
	s.Root.Hit = core.HitBlock
	s.col = primitive.Column()
	s.col.CrossAlign = core.CrossStretch
	s.col.MainAlign = core.MainStart
	s.body = primitive.NewDecorated()
	s.body.StretchChild = true
	s.body.ExpandWidth = true
	bodyFlex := primitive.NewFlexible(1, s.body)
	bodyFlex.FillChild = true
	s.col.AddChild(bodyFlex)
	s.Root.AddChild(s.col)
	s.Root.StretchChild = true
	for _, c := range children {
		if c != nil {
			s.body.AddChild(c)
		}
	}
	s.applyChrome()
	return s
}

// Node returns the sider root.
func (s *Sider) Node() core.Node {
	if s == nil {
		return nil
	}
	if s.Root == nil {
		// Reconstruct minimal root (should not happen after NewSider).
		s.Root = &siderRoot{owner: s}
		s.Root.Init(s.Root)
		s.Root.Hit = core.HitBlock
		s.applyChrome()
	}
	return s.Root
}

// CollapsedState reports current collapsed (controlled or default).
func (s *Sider) CollapsedState() bool {
	if s == nil {
		return false
	}
	if s.collapsedCtrl {
		return s.Collapsed
	}
	// Uncontrolled: Collapsed field tracks internal state after first toggle;
	// initial is DefaultCollapsed until toggled (we keep Collapsed in sync).
	return s.Collapsed
}

// SetWidth sets expanded width (0 → default 200).
func (s *Sider) SetWidth(w float64) {
	if s == nil {
		return
	}
	s.Width = w
	s.applyChrome()
}

// SetCollapsedWidth sets collapsed width; 0 enables zero-width trigger.
func (s *Sider) SetCollapsedWidth(w float64) {
	if s == nil {
		return
	}
	if w < 0 {
		w = 0
	}
	s.CollapsedWidth = w
	s.collapsedWidthSet = true
	s.applyChrome()
}

// SetCollapsible enables bottom trigger.
func (s *Sider) SetCollapsible(v bool) {
	if s == nil {
		return
	}
	s.Collapsible = v
	s.applyChrome()
}

// SetCollapsed sets controlled collapsed state.
func (s *Sider) SetCollapsed(v bool) {
	if s == nil {
		return
	}
	s.Collapsed = v
	s.collapsedCtrl = true
	s.applyChrome()
}

// SetDefaultCollapsed sets uncontrolled initial collapsed.
func (s *Sider) SetDefaultCollapsed(v bool) {
	if s == nil {
		return
	}
	s.DefaultCollapsed = v
	if !s.collapsedCtrl {
		s.Collapsed = v
	}
	s.applyChrome()
}

// SetSiderTheme sets dark|light chrome.
func (s *Sider) SetSiderTheme(t SiderTheme) {
	if s == nil {
		return
	}
	s.SiderTheme = t
	s.applyChrome()
}

// SetReverseArrow flips collapse arrow direction.
func (s *Sider) SetReverseArrow(v bool) {
	if s == nil {
		return
	}
	s.ReverseArrow = v
	s.applyChrome()
}

// SetBreakpoint enables responsive auto-collapse.
func (s *Sider) SetBreakpoint(bp LayoutBreakpoint) {
	if s == nil {
		return
	}
	s.Breakpoint = bp
	s.breakpointSet = bp != LayoutBreakpointNone
	s.applyChrome()
}

// SetViewportWidth injects viewport for breakpoint (tests / host).
func (s *Sider) SetViewportWidth(w float64) {
	if s == nil {
		return
	}
	s.ViewportWidth = w
	s.viewportSet = true
	s.applyBreakpoint(w)
	s.applyChrome()
}

// SetTrigger sets custom trigger content (replaces default arrow).
func (s *Sider) SetTrigger(n core.Node) {
	if s == nil {
		return
	}
	s.Trigger = n
	s.hideTrigger = false
	s.applyChrome()
}

// SetHideTrigger hides trigger (antd trigger={null}).
func (s *Sider) SetHideTrigger() {
	if s == nil {
		return
	}
	s.hideTrigger = true
	s.Trigger = nil
	s.applyChrome()
}

// SetOverlay makes sider overlay content (no flow width).
func (s *Sider) SetOverlay(v bool) {
	if s == nil {
		return
	}
	s.Overlay = v
	s.applyChrome()
}

// SetOnCollapse sets collapse callback.
func (s *Sider) SetOnCollapse(fn func(collapsed bool, typ CollapseType)) {
	if s == nil {
		return
	}
	s.OnCollapse = fn
}

// SetOnBreakpoint sets breakpoint callback.
func (s *Sider) SetOnBreakpoint(fn func(broken bool)) {
	if s == nil {
		return
	}
	s.OnBreakpoint = fn
}

// SetBackground overrides sider background.
func (s *Sider) SetBackground(c render.RGBA) {
	if s == nil {
		return
	}
	s.bg = c
	s.bgSet = true
	s.applyChrome()
}

// SetTheme stores product theme (token sizes).
func (s *Sider) SetTheme(th *core.Theme) {
	if s == nil {
		return
	}
	s.Theme = th
	s.applyChrome()
}

// SetAriaLabel sets accessible name (role=complementary).
func (s *Sider) SetAriaLabel(name string) {
	if s == nil {
		return
	}
	s.AriaLabel = name
	s.applyA11y()
}

// SetChildren replaces body children.
func (s *Sider) SetChildren(children ...core.Node) {
	if s == nil {
		return
	}
	_ = s.Node()
	if s.body == nil {
		return
	}
	s.body.ClearChildren()
	for _, c := range children {
		if c != nil {
			s.body.AddChild(c)
		}
	}
	s.applyChrome()
}

// EffectiveWidth returns current rendered width (collapsed or expanded).
func (s *Sider) EffectiveWidth() float64 {
	if s == nil {
		return 0
	}
	if s.CollapsedState() {
		return s.resolvedCollapsedWidth()
	}
	return s.resolvedWidth()
}

func (s *Sider) resolvedWidth() float64 {
	if s.Width > 0 {
		return s.Width
	}
	return DefaultSiderWidth
}

func (s *Sider) resolvedCollapsedWidth() float64 {
	if s.collapsedWidthSet {
		return s.CollapsedWidth
	}
	if s.CollapsedWidth > 0 {
		return s.CollapsedWidth
	}
	return DefaultSiderCollapsedWidth
}

func (s *Sider) applyA11y() {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.Base().Role = "complementary"
	s.Root.Base().Label = s.AriaLabel
}

func (s *Sider) theme() *core.Theme {
	return themeOf(s.Theme, s.Node())
}

func (s *Sider) siderBg() render.RGBA {
	if s.bgSet {
		return s.bg
	}
	if s.SiderTheme == SiderThemeLight {
		return s.theme().Color(core.TokenColorBgContainer)
	}
	return DefaultLayoutSiderBg
}

func (s *Sider) triggerBg() render.RGBA {
	if s.SiderTheme == SiderThemeLight {
		return s.theme().Color(core.TokenColorBgContainer)
	}
	return DefaultLayoutTriggerBg
}

func (s *Sider) triggerFg() render.RGBA {
	if s.SiderTheme == SiderThemeLight {
		return s.theme().Color(core.TokenColorText)
	}
	return s.theme().Color(core.TokenColorTextInverse)
}

func (s *Sider) toggle(typ CollapseType) {
	if s == nil {
		return
	}
	next := !s.CollapsedState()
	if !s.collapsedCtrl {
		s.Collapsed = next
	}
	// Controlled: still fire callback; caller must SetCollapsed.
	if s.OnCollapse != nil {
		s.OnCollapse(next, typ)
	}
	if !s.collapsedCtrl {
		s.applyChrome()
	}
}

func (s *Sider) applyBreakpoint(layoutMaxW float64) {
	if s == nil || !s.breakpointSet || s.Breakpoint == LayoutBreakpointNone {
		return
	}
	vp := layoutMaxW
	if s.viewportSet && s.ViewportWidth > 0 {
		vp = s.ViewportWidth
	}
	if vp <= 0 {
		return
	}
	maxW := breakpointMaxWidth(s.Breakpoint)
	if maxW < 0 {
		return
	}
	broken := vp <= maxW
	if s.brokenInited && broken == s.lastBroken {
		s.below = broken
		return
	}
	prevBroken := s.lastBroken
	s.below = broken
	s.lastBroken = broken
	wasInit := s.brokenInited
	s.brokenInited = true
	if s.OnBreakpoint != nil && (wasInit || broken) {
		// Fire on first evaluation and on changes (antd fires on mount match too).
		if !wasInit || broken != prevBroken {
			s.OnBreakpoint(broken)
		}
	}
	// Auto-collapse when broken state changes to match antd.
	if !wasInit || broken != prevBroken {
		if s.CollapsedState() != broken {
			if !s.collapsedCtrl {
				s.Collapsed = broken
			}
			if s.OnCollapse != nil {
				s.OnCollapse(broken, CollapseResponsive)
			}
		}
	}
}

func (s *Sider) applyChrome() {
	if s == nil {
		return
	}
	_ = s.Node()
	if s.col == nil || s.body == nil {
		return
	}

	// Uncontrolled init from DefaultCollapsed once.
	if !s.collapsedCtrl && !s.brokenInited && s.DefaultCollapsed && !s.Collapsed {
		// Only seed if never toggled; Collapsed starts false.
		// DefaultCollapsed applied at construction path via SetDefaultCollapsed.
	}

	w := s.EffectiveWidth()
	// Overlay: still paints at visual width; parent Layout omits from flow.
	s.Root.Width = w
	s.Root.MinWidth = w
	if w == 0 {
		// Keep a minimal hit for zero-width trigger host.
		s.Root.MinWidth = 0
	}
	s.Root.Background = s.siderBg()
	s.Root.ExpandWidth = false
	s.Root.StretchChild = true

	// Rebuild trigger chrome as trailing flex child.
	// Remove previous trigger / zero trigger nodes beyond body.
	for {
		kids := s.col.Children()
		if len(kids) <= 1 {
			break
		}
		// Keep only first (body flex).
		s.col.ClearChildren()
		bodyFlex := primitive.NewFlexible(1, s.body)
		bodyFlex.FillChild = true
		s.col.AddChild(bodyFlex)
		break
	}

	showBottomTrigger := s.Collapsible && !s.hideTrigger && s.resolvedCollapsedWidth() > 0
	showZeroTrigger := (s.Collapsible || s.below) && !s.hideTrigger && s.resolvedCollapsedWidth() == 0

	if showBottomTrigger {
		th := s.theme()
		trigH := DefaultSiderTriggerHeight
		if lg := th.SizeOr(core.TokenControlHeightLG, 40); lg > 0 {
			xs := th.SizeOr(core.TokenMarginXS, 4)
			trigH = lg + xs*2
		}
		var child core.Node
		if s.Trigger != nil {
			child = s.Trigger
		} else {
			s.trigLab = primitive.NewText(s.arrowGlyph())
			s.trigLab.Color = s.triggerFg()
			s.trigLab.FontSize = th.SizeOr(core.TokenFontSize, DefaultLayoutFontSize)
			child = s.trigLab
		}
		host := primitive.NewDecorated(child)
		host.Background = s.triggerBg()
		host.Height = trigH
		host.MinHeight = trigH
		host.Width = w
		host.MinWidth = w
		host.ExpandWidth = true
		host.StretchChild = true
		host.SetCenterContent(true)
		s.trigger = primitive.NewPressable(host)
		s.trigger.Focusable = true
		s.trigger.Click = func() {
			s.toggle(CollapseClickTrigger)
		}
		s.trigger.Base().Role = "button"
		s.trigger.Base().Label = "Toggle sider"
		s.col.AddChild(s.trigger)
		// antd has-trigger padding bottom.
		s.Root.Padding = primitive.EdgeInsets{}
	} else if showZeroTrigger {
		th := s.theme()
		sz := DefaultSiderZeroTriggerSize
		if lg := th.SizeOr(core.TokenControlHeightLG, 40); lg > 0 {
			sz = lg
		}
		var child core.Node
		if s.Trigger != nil {
			child = s.Trigger
		} else {
			lab := primitive.NewText("☰")
			lab.Color = s.triggerFg()
			lab.FontSize = th.SizeOr(core.TokenFontSizeLG, 16)
			child = lab
		}
		s.zeroTrigHost = primitive.NewDecorated(child)
		s.zeroTrigHost.Background = s.siderBg()
		s.zeroTrigHost.Width = sz
		s.zeroTrigHost.Height = sz
		s.zeroTrigHost.MinWidth = sz
		s.zeroTrigHost.MinHeight = sz
		s.zeroTrigHost.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
		s.zeroTrigHost.SetCenterContent(true)
		s.zeroTrigger = primitive.NewPressable(s.zeroTrigHost)
		s.zeroTrigger.Focusable = true
		s.zeroTrigger.Click = func() {
			s.toggle(CollapseClickTrigger)
		}
		s.zeroTrigger.Base().Role = "button"
		s.zeroTrigger.Base().Label = "Toggle sider"
		// Place zero trigger as sibling so it remains hittable when width=0:
		// use min width for root when collapsed to 0 so trigger can stick out.
		// antd positions it outside; we expand root to hold trigger when w==0.
		if w == 0 {
			s.Root.Width = sz
			s.Root.MinWidth = sz
			s.Root.Background = render.RGBA{} // transparent rail; trigger paints
		}
		s.col.AddChild(s.zeroTrigger)
	} else {
		s.trigger = nil
		s.zeroTrigger = nil
	}

	s.applyA11y()
	s.Root.MarkNeedsLayout()
}

func (s *Sider) arrowGlyph() string {
	// reverseArrow flips; collapsed shows expand direction.
	// default expanded → « (collapse to left); collapsed → »
	collapsed := s.CollapsedState()
	// reverseIcon logic simplified (LTR): reverseArrow swaps.
	if s.ReverseArrow {
		if collapsed {
			return "‹"
		}
		return "›"
	}
	if collapsed {
		return "›"
	}
	return "‹"
}
