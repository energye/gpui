package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Drawer defaults (https://ant.design/components/drawer).
const (
	DefaultDrawerWidth     = 378.0
	DefaultDrawerHeight    = 378.0
	DefaultDrawerLargeSize = 736.0
	DefaultDrawerPadding   = 24.0
	DefaultDrawerTitleFont = 16.0
	DefaultDrawerBodyGap   = 12.0
	DefaultDrawerFooterGap = 8.0
	DefaultDrawerHandle    = 6.0
)

// DrawerPlacement is the edge where the panel appears.
type DrawerPlacement string

const (
	DrawerPlacementTop    DrawerPlacement = "top"
	DrawerPlacementRight  DrawerPlacement = "right"
	DrawerPlacementBottom DrawerPlacement = "bottom"
	DrawerPlacementLeft   DrawerPlacement = "left"
)

// DrawerSize is the Ant preset size. Use SetSizePx for a custom numeric size.
type DrawerSize string

const (
	DrawerSizeDefault DrawerSize = "default"
	DrawerSizeLarge   DrawerSize = "large"
)

// Drawer is a side panel over an optional mask (Ant Drawer).
//
//	OverlayPortal
//	  └─ FocusScope
//	       └─ drawerLayer: Mask? + Decorated(panel) + resize handle?
//
// Product contract: docs/antd/drawer.md §6 P0. The portal, focus scope and
// layer stay stable across rebuilds while open.
type Drawer struct {
	Portal *primitive.OverlayPortal
	Scope  *primitive.FocusScope

	layer        *drawerLayer
	panel        *primitive.Decorated
	titleNode    *primitive.Text
	bodySlot     *primitive.Slot
	extraSlot    *primitive.Slot
	footerSlot   *primitive.Slot
	closeBtn     *Button
	resizeHandle *primitive.Draggable
	loadingSk    *Skeleton

	Open       bool
	Title      string
	Placement  DrawerPlacement
	Size       DrawerSize
	SizePixels float64 // >0 overrides Size preset.
	// Width/Height are numeric aliases kept for existing callers and tests.
	Width  float64
	Height float64

	Loading         bool
	Closable        bool
	Mask            bool
	MaskClosable    bool
	Keyboard        bool
	DestroyOnHidden bool
	Resizable       bool
	MinSize         float64
	MaxSize         float64

	// Padding uniform panel inset (0 -> DefaultDrawerPadding).
	Padding       float64
	TitleFontSize float64 // 0 -> DefaultDrawerTitleFont
	BodyGap       float64 // 0 -> DefaultDrawerBodyGap
	FooterGap     float64 // 0 -> DefaultDrawerFooterGap
	Face          text.Face
	Theme         *core.Theme
	Viewport      core.Size

	OnClose         func()
	OnOpenChange    func(open bool)
	AfterOpenChange func(open bool)
	OnResizeStart   func()
	OnResize        func(size float64)
	OnResizeEnd     func()

	content core.Node
	extra   core.Node
	footer  core.Node
	trap    overlayFocusTrap
	pad     primitive.EdgeInsets
	padSet  bool
	life    tickerLifecycle

	resizeStartSize float64
}

// NewDrawer creates a closed drawer.
func NewDrawer(title string) *Drawer {
	d := &Drawer{
		Title:        title,
		Placement:    DrawerPlacementRight,
		Size:         DrawerSizeDefault,
		Width:        DefaultDrawerWidth,
		Height:       DefaultDrawerHeight,
		Closable:     true,
		Mask:         true,
		MaskClosable: true,
		Keyboard:     true,
	}
	d.rebuild()
	return d
}

// Node returns the portal host node.
func (d *Drawer) Node() core.Node {
	if d.Portal == nil {
		d.rebuild()
	}
	return d.Portal
}

// SetOpen shows or hides the drawer.
func (d *Drawer) SetOpen(open bool) {
	if d == nil {
		return
	}
	if d.Portal == nil {
		d.rebuild()
	}
	was := d.Open
	d.Open = open
	if d.Portal != nil {
		d.Portal.SetOpen(open)
	}
	if d.bodySlot != nil {
		d.bodySlot.SetChild(d.bodyNode())
	}
	if open {
		d.trap.wire(d.Scope, true, d.onEscape)
		var prefer core.Node
		if d.closeBtn != nil && d.Closable {
			prefer = d.closeBtn.Root
		}
		d.trap.enter(d.Scope, d.Portal, prefer)
	} else if was {
		d.trap.wire(d.Scope, false, nil)
		d.trap.leave(d.Scope, d.Portal)
	} else {
		d.trap.wire(d.Scope, false, nil)
	}
	if was != open {
		d.life.setActive(d.Loading && open)
		if d.OnOpenChange != nil {
			d.OnOpenChange(open)
		}
		if d.AfterOpenChange != nil {
			d.AfterOpenChange(open)
		}
	}
	if d.layer != nil {
		d.layer.MarkNeedsLayout()
	}
}

// SetTitle updates the title and accessible dialog name.
func (d *Drawer) SetTitle(title string) {
	if d == nil {
		return
	}
	d.Title = title
	if d.titleNode != nil {
		d.titleNode.SetValue(title)
	}
	if d.layer != nil {
		d.layer.Label = title
	}
}

// SetContent sets the body node.
func (d *Drawer) SetContent(n core.Node) {
	if d == nil {
		return
	}
	d.content = n
	if d.bodySlot != nil {
		d.bodySlot.SetChild(d.bodyNode())
	} else {
		d.rebuild()
	}
}

// SetExtra sets the title-bar extra action node.
func (d *Drawer) SetExtra(n core.Node) {
	if d == nil {
		return
	}
	d.extra = n
	if d.extraSlot != nil {
		d.extraSlot.SetChild(n)
	} else {
		d.rebuild()
	}
}

// SetFooter sets the footer node.
func (d *Drawer) SetFooter(n core.Node) {
	if d == nil {
		return
	}
	d.footer = n
	d.rebuild()
}

// SetPlacement sets the edge: top, right, bottom or left. Unknown values fall back to right.
func (d *Drawer) SetPlacement(p DrawerPlacement) {
	if d == nil {
		return
	}
	d.Placement = normalizeDrawerPlacement(p)
	d.rebuild()
}

// SetSize sets an Ant preset size.
func (d *Drawer) SetSize(s DrawerSize) {
	if d == nil {
		return
	}
	if s != DrawerSizeLarge {
		s = DrawerSizeDefault
	}
	d.Size = s
	d.SizePixels = 0
	d.syncAliasDimensions()
	d.rebuild()
}

// SetSizePx sets a custom numeric drawer width/height depending on placement.
func (d *Drawer) SetSizePx(px float64) {
	if d == nil {
		return
	}
	d.setResolvedSize(px, true)
}

// SetWidth is a numeric alias for SetSizePx for left/right drawers.
func (d *Drawer) SetWidth(w float64) { d.SetSizePx(w) }

// SetHeight is a numeric alias for SetSizePx for top/bottom drawers.
func (d *Drawer) SetHeight(h float64) { d.SetSizePx(h) }

// SetLoading toggles body skeleton loading. Loading animates through Tree ticker.
func (d *Drawer) SetLoading(v bool) {
	if d == nil || d.Loading == v {
		return
	}
	d.Loading = v
	d.rebuild()
	d.life.setActive(v && d.Open)
}

func (d *Drawer) SetClosable(v bool) {
	if d == nil || d.Closable == v {
		return
	}
	d.Closable = v
	d.rebuild()
}

func (d *Drawer) SetMask(v bool) {
	if d == nil || d.Mask == v {
		return
	}
	d.Mask = v
	d.rebuild()
}

func (d *Drawer) SetMaskClosable(v bool) {
	if d != nil {
		d.MaskClosable = v
	}
}

func (d *Drawer) SetKeyboard(v bool) {
	if d == nil {
		return
	}
	d.Keyboard = v
	d.trap.wire(d.Scope, d.Open, d.onEscape)
}

func (d *Drawer) SetDestroyOnHidden(v bool) {
	if d == nil {
		return
	}
	d.DestroyOnHidden = v
	if d.bodySlot != nil {
		d.bodySlot.SetChild(d.bodyNode())
	}
}

func (d *Drawer) SetResizable(v bool) {
	if d == nil || d.Resizable == v {
		return
	}
	d.Resizable = v
	d.rebuild()
}

func (d *Drawer) SetResizeBounds(min, max float64) {
	if d == nil {
		return
	}
	d.MinSize, d.MaxSize = min, max
	d.setResolvedSize(d.resolvedSize(), true)
}

// SetFace applies the product font.
func (d *Drawer) SetFace(face text.Face) {
	if d == nil {
		return
	}
	d.Face = face
	d.rebuild()
}

func (d *Drawer) SetTheme(th *core.Theme) {
	if d == nil {
		return
	}
	d.Theme = th
	d.rebuild()
}

// AttachTicker registers loading skeleton animation.
func (d *Drawer) AttachTicker(t *core.Tree) {
	if d != nil {
		d.life.attach(t, d, d.Loading && d.Open)
		if d.loadingSk != nil {
			d.loadingSk.AttachTicker(t)
		}
	}
}

// Tick advances loading skeleton animation.
func (d *Drawer) Tick(dt float64) bool {
	if d == nil || !d.Loading || !d.Open {
		return false
	}
	if d.loadingSk != nil {
		_ = d.loadingSk.Tick(dt)
	}
	if d.bodySlot != nil {
		d.bodySlot.MarkNeedsPaint()
	}
	return d.Loading && d.Open
}

// SetPadding sets uniform panel inset (0 -> DefaultDrawerPadding).
func (d *Drawer) SetPadding(px float64) {
	if d == nil {
		return
	}
	d.Padding = px
	d.padSet = false
	d.rebuild()
}

// SetPaddingInsets sets per-side panel inset (explicit, including all-zero).
func (d *Drawer) SetPaddingInsets(p primitive.EdgeInsets) {
	if d == nil {
		return
	}
	d.pad = p
	d.padSet = true
	d.rebuild()
}

func (d *Drawer) onEscape() {
	if d == nil || !d.Open || !d.Keyboard {
		return
	}
	d.closeFromUser()
}

func (d *Drawer) closeFromUser() {
	if d == nil {
		return
	}
	if d.OnClose != nil {
		d.OnClose()
	}
	d.SetOpen(false)
}

func (d *Drawer) theme() *core.Theme {
	var n core.Node
	if d.Portal != nil {
		n = d.Portal
	}
	return themeOf(d.Theme, n)
}

func (d *Drawer) panelPadding() primitive.EdgeInsets {
	if d != nil && d.padSet {
		return d.pad
	}
	px := DefaultDrawerPadding
	if d != nil && d.Padding > 0 {
		px = d.Padding
	}
	return primitive.All(px)
}

func (d *Drawer) titleFont() float64 {
	if d != nil && d.TitleFontSize > 0 {
		return d.TitleFontSize
	}
	return DefaultDrawerTitleFont
}

func (d *Drawer) bodyGap() float64 {
	if d != nil && d.BodyGap > 0 {
		return d.BodyGap
	}
	return DefaultDrawerBodyGap
}

func (d *Drawer) footerGap() float64 {
	if d != nil && d.FooterGap > 0 {
		return d.FooterGap
	}
	return DefaultDrawerFooterGap
}

func (d *Drawer) bodyNode() core.Node {
	if d == nil {
		return nil
	}
	if d.DestroyOnHidden && !d.Open {
		return nil
	}
	if d.Loading {
		d.loadingSk = NewSkeleton()
		d.loadingSk.SetStyle(Style{Width: 260})
		d.loadingSk.SetParagraphRows(3)
		d.loadingSk.SetActive(true)
		d.loadingSk.SetTheme(d.Theme)
		return d.loadingSk.Node()
	}
	d.loadingSk = nil
	return d.content
}

func (d *Drawer) rebuild() {
	th := d.theme()
	d.Placement = normalizeDrawerPlacement(d.Placement)
	if d.Size == "" {
		d.Size = DrawerSizeDefault
	}
	d.syncAliasDimensions()

	d.titleNode = primitive.NewText(d.Title)
	d.titleNode.FontSize = d.titleFont()
	d.titleNode.Face = d.Face
	d.titleNode.Color = th.Color(core.TokenColorText)

	d.extraSlot = primitive.NewSlot("extra", d.extra)
	d.extraSlot.ExpandFill = false

	headKids := []core.Node{}
	if d.Closable {
		d.closeBtn = NewButton("x")
		d.closeBtn.SetType(ButtonText)
		d.closeBtn.SetAriaLabel("Close Button")
		d.closeBtn.SetFace(d.Face)
		d.closeBtn.SetOnClick(func() { d.closeFromUser() })
		headKids = append(headKids, d.closeBtn.Node())
	} else {
		d.closeBtn = nil
	}
	headKids = append(headKids, d.titleNode, primitive.Spacer(), d.extraSlot)
	head := primitive.Row(headKids...)
	head.Gap = 8
	head.CrossAlign = core.CrossCenter

	d.bodySlot = primitive.NewSlot("body", d.bodyNode())
	d.bodySlot.ExpandFill = true

	col := primitive.Column(head, primitive.NewDivider(), d.bodySlot)
	col.Gap = d.bodyGap()
	col.CrossAlign = core.CrossStretch
	if d.footer != nil {
		d.footerSlot = primitive.NewSlot("footer", d.footer)
		col.AddChild(primitive.NewDivider())
		col.AddChild(d.footerSlot)
		col.Gap = d.footerGap()
	} else {
		d.footerSlot = nil
	}

	d.panel = primitive.NewDecorated(col)
	d.panel.Padding = d.panelPadding()
	d.panel.Background = th.Color(core.TokenColorBgContainer)
	d.panel.Radius = 0
	d.panel.BorderWidth = 0

	var mask *primitive.Mask
	if d.Mask {
		mask = primitive.NewMask()
		mask.OnDismiss = func() {
			if d.MaskClosable {
				d.closeFromUser()
			}
		}
	}

	if d.Resizable {
		bar := primitive.NewBox()
		bar.Color = th.Color(core.TokenColorBorder)
		if bar.Color.A <= 0 {
			bar.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.18}
		}
		if d.isHorizontalPlacement() {
			bar.Width, bar.Height = DefaultDrawerHandle, 1
		} else {
			bar.Width, bar.Height = 1, DefaultDrawerHandle
		}
		d.resizeHandle = primitive.NewDraggable(bar)
		d.resizeHandle.Axis = 1
		if !d.isHorizontalPlacement() {
			d.resizeHandle.Axis = 2
		}
		d.resizeHandle.Cursor = core.CursorMove
		d.resizeHandle.OnDragStart = func() {
			d.resizeStartSize = d.resolvedSize()
			if d.OnResizeStart != nil {
				d.OnResizeStart()
			}
		}
		d.resizeHandle.OnDrag = func(dx, dy float64) {
			next := d.resizeStartSize
			switch d.Placement {
			case DrawerPlacementLeft:
				next += dx
			case DrawerPlacementRight:
				next -= dx
			case DrawerPlacementTop:
				next += dy
			case DrawerPlacementBottom:
				next -= dy
			}
			d.setResolvedSize(next, false)
		}
		d.resizeHandle.OnDragEnd = func(dx, dy float64) {
			if d.OnResizeEnd != nil {
				d.OnResizeEnd()
			}
		}
	} else {
		d.resizeHandle = nil
	}

	if d.layer == nil {
		d.layer = &drawerLayer{drawer: d}
		d.layer.Init(d.layer)
		d.layer.Hit = core.HitDefer
		d.layer.Role = "dialog"
	}
	d.layer.drawer = d
	d.layer.mask = mask
	d.layer.panel = d.panel
	d.layer.handle = d.resizeHandle
	d.layer.Label = d.Title
	d.layer.ClearChildren()
	if mask != nil {
		d.layer.AddChild(mask)
	}
	d.layer.AddChild(d.panel)
	if d.resizeHandle != nil {
		d.resizeHandle.PaintOrder = 1
		d.layer.AddChild(d.resizeHandle)
	}

	if d.Scope == nil {
		d.Scope = primitive.NewFocusScope(d.layer)
	}
	d.trap.wire(d.Scope, d.Open, d.onEscape)

	if d.Portal == nil {
		d.Portal = primitive.NewOverlayPortal(d.Scope)
		d.Portal.ID = "drawer"
		d.Portal.ZOrder = OverlayZDrawer
	} else {
		d.Portal.Content = d.Scope
		d.Portal.ZOrder = OverlayZDrawer
	}
	d.Portal.SetThemeHook(func(*core.Theme) { d.rebuild() })
	if d.Open {
		d.Portal.SetOpen(true)
		d.layer.MarkNeedsLayout()
		d.layer.MarkNeedsPaint()
	}
	d.life.setActive(d.Loading && d.Open)
}

func (d *Drawer) isHorizontalPlacement() bool {
	return d == nil || d.Placement == DrawerPlacementLeft || d.Placement == DrawerPlacementRight
}

func (d *Drawer) resolvedSize() float64 {
	if d == nil {
		return DefaultDrawerWidth
	}
	if d.SizePixels > 0 {
		return d.clampSize(d.SizePixels)
	}
	if d.Size == DrawerSizeLarge {
		return d.clampSize(DefaultDrawerLargeSize)
	}
	if d.isHorizontalPlacement() {
		if d.Width > 0 && d.Width != DefaultDrawerWidth {
			return d.clampSize(d.Width)
		}
		return d.clampSize(DefaultDrawerWidth)
	}
	if d.Height > 0 && d.Height != DefaultDrawerHeight {
		return d.clampSize(d.Height)
	}
	return d.clampSize(DefaultDrawerHeight)
}

func (d *Drawer) syncAliasDimensions() {
	if d == nil {
		return
	}
	size := DefaultDrawerWidth
	if d.SizePixels > 0 {
		size = d.SizePixels
	} else if d.Size == DrawerSizeLarge {
		size = DefaultDrawerLargeSize
	}
	if size <= 0 {
		size = DefaultDrawerWidth
	}
	d.Width = size
	d.Height = size
}

func (d *Drawer) setResolvedSize(px float64, rebuild bool) {
	if d == nil {
		return
	}
	px = d.clampSize(px)
	if px <= 0 {
		px = DefaultDrawerWidth
	}
	d.SizePixels = px
	d.Width = px
	d.Height = px
	if d.OnResize != nil {
		d.OnResize(px)
	}
	if rebuild {
		d.rebuild()
	} else if d.layer != nil {
		d.layer.MarkNeedsLayout()
		d.layer.MarkNeedsPaint()
	}
}

func (d *Drawer) clampSize(px float64) float64 {
	if d == nil {
		return px
	}
	if d.MinSize > 0 && px < d.MinSize {
		px = d.MinSize
	}
	if d.MaxSize > 0 && px > d.MaxSize {
		px = d.MaxSize
	}
	return px
}

func normalizeDrawerPlacement(p DrawerPlacement) DrawerPlacement {
	switch p {
	case DrawerPlacementTop, DrawerPlacementRight, DrawerPlacementBottom, DrawerPlacementLeft:
		return p
	default:
		return DrawerPlacementRight
	}
}

type drawerLayer struct {
	core.NodeBase
	mask   *primitive.Mask
	panel  *primitive.Decorated
	handle *primitive.Draggable
	drawer *Drawer
}

func (l *drawerLayer) TypeID() string { return "kit.DrawerLayer" }

func (l *drawerLayer) Layout(c core.Constraints) core.Size {
	var portal *primitive.OverlayPortal
	var vp core.Size
	if l.drawer != nil {
		portal = l.drawer.Portal
		vp = l.drawer.Viewport
	}
	vw, vh := resolveOverlayViewport(vp, portal, c.MaxWidth, c.MaxHeight)
	if l.mask != nil {
		l.mask.Width, l.mask.Height = vw, vh
		_ = l.mask.Layout(core.Tight(vw, vh))
		l.mask.SetOffset(core.Point{})
	}

	size := DefaultDrawerWidth
	placement := DrawerPlacementRight
	if l.drawer != nil {
		size = l.drawer.resolvedSize()
		placement = l.drawer.Placement
	}
	if size < 0 {
		size = 0
	}

	var panelW, panelH float64
	if placement == DrawerPlacementTop || placement == DrawerPlacementBottom {
		if size > vh {
			size = vh
		}
		panelW, panelH = vw, size
	} else {
		if size > vw {
			size = vw
		}
		panelW, panelH = size, vh
	}

	var px, py float64
	switch placement {
	case DrawerPlacementLeft:
		px, py = 0, 0
	case DrawerPlacementTop:
		px, py = 0, 0
	case DrawerPlacementBottom:
		px, py = 0, vh-panelH
	default:
		px, py = vw-panelW, 0
	}

	if l.panel != nil {
		_ = l.panel.Layout(core.Tight(panelW, panelH))
		l.panel.SetOffset(core.Point{X: px, Y: py})
	}
	if l.handle != nil {
		handle := DefaultDrawerHandle
		if placement == DrawerPlacementTop || placement == DrawerPlacementBottom {
			_ = l.handle.Layout(core.Tight(panelW, handle))
			hy := py + panelH - handle/2
			if placement == DrawerPlacementBottom {
				hy = py - handle/2
			}
			l.handle.SetOffset(core.Point{X: px, Y: hy})
		} else {
			_ = l.handle.Layout(core.Tight(handle, panelH))
			hx := px - handle/2
			if placement == DrawerPlacementLeft {
				hx = px + panelW - handle/2
			}
			l.handle.SetOffset(core.Point{X: hx, Y: py})
		}
	}

	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func (l *drawerLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *drawerLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }
