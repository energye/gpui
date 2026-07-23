package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Drawer defaults (https://ant.design/components/drawer).
const (
	DefaultDrawerWidth     = 378.0
	DefaultDrawerPadding   = 24.0 // Ant content padding
	DefaultDrawerTitleFont = 16.0
	DefaultDrawerBodyGap   = 12.0
)

// Drawer is a side panel with mask (Ant Drawer).
//
//	OverlayPortal (Z=400)
//	  └─ FocusScope
//	       └─ drawerLayer: Mask + Decorated(panel)
//
// Shares Modal overlay contracts: dual-band present, Esc, focus trap, full-viewport mask.
// Portal/Scope stay stable across rebuilds while open.
// Defaults match Ant; override via SetWidth / SetPadding.
type Drawer struct {
	Portal   *primitive.OverlayPortal
	Scope    *primitive.FocusScope
	layer    *drawerLayer
	panel    *primitive.Decorated
	title    *primitive.Text
	body     *primitive.Slot
	closeBtn *Button
	Open     bool
	Title    string
	Width    float64 // 0 → DefaultDrawerWidth
	// Padding uniform panel inset (0 → DefaultDrawerPadding).
	Padding       float64
	TitleFontSize float64 // 0 → DefaultDrawerTitleFont
	BodyGap       float64 // 0 → DefaultDrawerBodyGap
	// Placement: "left" or "right" (default right).
	Placement string
	// Left is true when Placement is "left" (kept for layout).
	Left bool
	// MaskClosable: click mask to close (default true, Ant).
	MaskClosable bool
	Face         text.Face
	Theme        *core.Theme
	Viewport     core.Size
	OnClose      func()
	content      core.Node
	trap         overlayFocusTrap
	pad          primitive.EdgeInsets
	padSet       bool
}

// NewDrawer creates a closed drawer.
func NewDrawer(title string) *Drawer {
	d := &Drawer{Title: title, Width: DefaultDrawerWidth, MaskClosable: true}
	d.rebuild()
	return d
}

// Node returns the portal node.
func (d *Drawer) Node() core.Node {
	if d.Portal == nil {
		d.rebuild()
	}
	return d.Portal
}

// SetContent sets body.
func (d *Drawer) SetContent(n core.Node) {
	d.content = n
	if d.body != nil {
		d.body.SetChild(n)
	} else {
		d.rebuild()
	}
}

// SetOpen shows/hides. Opens focus trap + Esc → OnClose (aligned with Modal).
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
	if open {
		d.trap.wire(d.Scope, true, d.onEscape)
		var prefer core.Node
		if d.closeBtn != nil {
			prefer = d.closeBtn.Root
		}
		d.trap.enter(d.Scope, d.Portal, prefer)
	} else if was {
		d.trap.wire(d.Scope, false, nil)
		d.trap.leave(d.Scope, d.Portal)
	} else {
		d.trap.wire(d.Scope, false, nil)
	}
}

func (d *Drawer) onEscape() {
	if d == nil || !d.Open {
		return
	}
	if d.OnClose != nil {
		d.OnClose()
	}
	d.SetOpen(false)
}

// SetPlacement sets side ("left" or "right").
func (d *Drawer) SetPlacement(p string) {
	d.Placement = p
	d.Left = p == "left"
	if d.layer != nil {
		d.layer.MarkNeedsLayout()
	} else if d.Portal != nil {
		d.Portal.MarkNeedsLayout()
	}
}

// SetWidth sets panel width.
func (d *Drawer) SetWidth(w float64) {
	d.Width = w
	if d.layer != nil {
		d.layer.MarkNeedsLayout()
	} else if d.Portal != nil {
		d.Portal.MarkNeedsLayout()
	}
}

// SetFace applies the product font.
func (d *Drawer) SetFace(face text.Face) {
	d.Face = face
	if d.title != nil {
		d.title.Face = face
	}
	if d.closeBtn != nil {
		d.closeBtn.SetFace(face)
	}
}

func (d *Drawer) theme() *core.Theme {
	var n core.Node
	if d.Portal != nil {
		n = d.Portal
	}
	return themeOf(d.Theme, n)
}

// SetPadding sets uniform panel inset (0 → DefaultDrawerPadding).
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

func (d *Drawer) rebuild() {
	th := d.theme()
	d.title = primitive.NewText(d.Title)
	d.title.FontSize = d.titleFont()
	d.title.Face = d.Face
	d.title.Color = th.Color(core.TokenColorText)
	d.body = primitive.NewSlot("body", d.content)
	d.closeBtn = NewButton("Close")
	d.closeBtn.SetType(ButtonText)
	d.closeBtn.SetFace(d.Face)
	d.closeBtn.SetOnClick(func() {
		if d.OnClose != nil {
			d.OnClose()
		}
		d.SetOpen(false)
	})
	head := primitive.Row(d.title, primitive.Spacer(), d.closeBtn.Node())
	head.CrossAlign = core.CrossCenter
	col := primitive.Column(head, primitive.NewDivider(), d.body)
	col.Gap = d.bodyGap()
	col.CrossAlign = core.CrossStart

	d.panel = primitive.NewDecorated(col)
	d.panel.Padding = d.panelPadding()
	d.panel.Background = th.Color(core.TokenColorBgContainer)
	d.panel.Radius = 0

	mask := primitive.NewMask()
	mask.OnDismiss = func() {
		if !d.MaskClosable {
			return
		}
		if d.OnClose != nil {
			d.OnClose()
		}
		d.SetOpen(false)
	}

	if d.layer == nil {
		d.layer = &drawerLayer{drawer: d}
		d.layer.Init(d.layer)
		d.layer.Hit = core.HitDefer
		d.layer.Role = "dialog"
		d.layer.Label = d.Title
	}
	d.layer.mask = mask
	d.layer.panel = d.panel
	d.layer.ClearChildren()
	d.layer.AddChild(mask)
	d.layer.AddChild(d.panel)

	if d.Scope == nil {
		d.Scope = primitive.NewFocusScope(d.layer)
	}
	d.trap.wire(d.Scope, d.Open, d.onEscape)

	if d.Portal == nil {
		d.Portal = primitive.NewOverlayPortal(d.Scope)
		d.Portal.ID = "drawer" // singleton per shell; override for multi-drawer
		d.Portal.ZOrder = OverlayZDrawer
	} else {
		d.Portal.Content = d.Scope
		d.Portal.ZOrder = OverlayZDrawer
	}
	if d.Open {
		d.Portal.SetOpen(true)
		d.layer.MarkNeedsLayout()
		d.layer.MarkNeedsPaint()
	}
}

type drawerLayer struct {
	core.NodeBase
	mask   *primitive.Mask
	panel  *primitive.Decorated
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

	w := 360.0
	if l.drawer != nil && l.drawer.Width > 0 {
		w = l.drawer.Width
	}
	if l.panel != nil {
		_ = l.panel.Layout(core.Tight(w, vh))
		x := vw - w
		if l.drawer != nil && l.drawer.Left {
			x = 0
		}
		l.panel.SetOffset(core.Point{X: x, Y: 0})
	}
	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func (l *drawerLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *drawerLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }
