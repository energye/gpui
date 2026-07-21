package primitive

import "github.com/energye/gpui/ui/core"

// Placement for AnchoredPopup relative to the anchor.
type Placement int

const (
	PlaceBottom Placement = iota
	PlaceTop
	PlaceLeft
	PlaceRight
	PlaceBottomStart
	PlaceTopStart
)

// AnchoredPopup positions content relative to an anchor rect (C-Anchor + C-Overlay).
// Uses OverlayPortal internally.
type AnchoredPopup struct {
	core.NodeBase

	Portal    *OverlayPortal
	Content   core.Node
	Placement Placement
	Gap       float64
	// Anchor bounds in absolute coordinates (updated by caller or UpdateAnchor).
	Anchor core.Rect
	// Viewport for flip; zero = no flip.
	Viewport core.Size

	Open bool
}

// NewAnchoredPopup creates a closed popup with the given content.
func NewAnchoredPopup(content core.Node) *AnchoredPopup {
	// Wrap content so Offset is applied on the wrapper
	a := &AnchoredPopup{Content: content, Gap: 4, Placement: PlaceBottom}
	a.Init(a)
	a.Hit = core.HitTransparent
	a.Portal = NewOverlayPortal(content)
	a.Portal.ID = "anchored"
	a.Portal.ZOrder = 200
	a.AddChild(a.Portal)
	return a
}

// TypeID implements core.Node.
func (a *AnchoredPopup) TypeID() string { return TypeAnchoredPopup }

// Layout implements core.Node.
func (a *AnchoredPopup) Layout(c core.Constraints) core.Size {
	// Portal is zero-sized; measure content for positioning if open.
	if a.Open && a.Content != nil {
		_ = a.Content.Layout(core.Loose(c.MaxWidth, c.MaxHeight))
		a.reposition()
	}
	out := a.Portal.Layout(c)
	a.SetSize(out)
	return out
}

// Paint implements core.Node.
func (a *AnchoredPopup) Paint(pc *core.PaintContext) {
	// content painted via overlay host
	a.DefaultPaintChildren(pc)
}

// HitTest implements core.Node.
func (a *AnchoredPopup) HitTest(p core.Point) core.Node { return nil }

// SetOpen shows or hides the popup.
func (a *AnchoredPopup) SetOpen(open bool) {
	a.Open = open
	if a.Portal != nil {
		a.Portal.SetOpen(open)
	}
	if open {
		a.reposition()
	}
}

// UpdateAnchor sets the anchor rect (absolute) and repositions if open.
func (a *AnchoredPopup) UpdateAnchor(r core.Rect) {
	a.Anchor = r
	if a.Open {
		a.reposition()
	}
}

// UpdateAnchorFromNode uses a node's absolute bounds as the anchor.
func (a *AnchoredPopup) UpdateAnchorFromNode(n core.Node) {
	if n == nil {
		return
	}
	a.UpdateAnchor(core.AbsoluteBounds(n))
}

func (a *AnchoredPopup) reposition() {
	if a.Content == nil {
		return
	}
	cs := a.Content.Base().Size()
	if cs.IsZero() {
		// try measure
		cs = a.Content.Layout(core.Loose(400, 400))
	}
	ar := a.Anchor
	gap := a.Gap
	var x, y float64
	switch a.Placement {
	case PlaceTop, PlaceTopStart:
		x = ar.Min.X
		if a.Placement == PlaceTop {
			x = ar.Min.X + (ar.Width()-cs.Width)/2
		}
		y = ar.Min.Y - gap - cs.Height
	case PlaceLeft:
		x = ar.Min.X - gap - cs.Width
		y = ar.Min.Y + (ar.Height()-cs.Height)/2
	case PlaceRight:
		x = ar.Max.X + gap
		y = ar.Min.Y + (ar.Height()-cs.Height)/2
	case PlaceBottomStart:
		x = ar.Min.X
		y = ar.Max.Y + gap
	default: // Bottom
		x = ar.Min.X + (ar.Width()-cs.Width)/2
		y = ar.Max.Y + gap
	}
	// Simple flip if viewport known
	if a.Viewport.Width > 0 && a.Viewport.Height > 0 {
		if y+cs.Height > a.Viewport.Height && ar.Min.Y-gap-cs.Height >= 0 {
			y = ar.Min.Y - gap - cs.Height
		}
		if x+cs.Width > a.Viewport.Width {
			x = a.Viewport.Width - cs.Width
		}
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
	}
	a.Portal.SetContentOffset(core.Point{X: x, Y: y})
}
