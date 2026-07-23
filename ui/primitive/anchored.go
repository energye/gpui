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
//
// P0.2 (docs/UI_FOUNDATION_P0.md):
//   - Viewport defaults from Tree.Viewport() when unset
//   - AnchorNode auto-refreshes bounds each layout while open
//   - flip/shift clamps into viewport
//   - DismissOnOutside registers core.Tree outside-dismiss
type AnchoredPopup struct {
	core.NodeBase

	Portal    *OverlayPortal
	Content   core.Node
	Placement Placement
	Gap       float64
	// Anchor bounds in absolute coordinates (updated by caller, UpdateAnchor, or AnchorNode).
	Anchor core.Rect
	// AnchorNode when non-nil refreshes Anchor each layout/open (preferred).
	AnchorNode core.Node
	// Viewport for flip/shift; zero → Tree.Viewport() when available.
	Viewport core.Size
	// DismissOnOutside closes the popup on pointer-down outside content+anchor.
	DismissOnOutside bool
	// OnDismiss is called when closed via outside pointer (after SetOpen(false)).
	OnDismiss func()

	Open bool
}

// NewAnchoredPopup creates a closed popup with the given content.
func NewAnchoredPopup(content core.Node) *AnchoredPopup {
	a := &AnchoredPopup{Content: content, Gap: 4, Placement: PlaceBottom, DismissOnOutside: true}
	a.Init(a)
	a.Hit = core.HitTransparent
	a.Portal = NewOverlayPortal(content)
	// Empty → unique auto-id on first push (concurrent popups must not clobber).
	a.Portal.ID = ""
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
		if a.AnchorNode != nil {
			a.Anchor = core.AbsoluteBounds(a.AnchorNode)
		}
		_ = a.Content.Layout(core.Loose(c.MaxWidth, c.MaxHeight))
		a.reposition()
		// Re-bind outside dismiss after mount/layout (tree may be nil at first SetOpen).
		a.syncOutsideDismiss()
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
	was := a.Open
	a.Open = open
	if a.Portal != nil {
		a.Portal.SetOpen(open)
	}
	if open {
		if a.AnchorNode != nil {
			a.Anchor = core.AbsoluteBounds(a.AnchorNode)
		}
		a.reposition()
		a.syncOutsideDismiss()
	} else {
		a.clearOutsideDismiss()
		// Only fire OnDismiss when transitioning open→closed via outside path;
		// callers that SetOpen(false) intentionally may still want a hook — see dismissOutside.
	}
	if was != open {
		if t := a.treeRef(); t != nil {
			t.MarkDirty()
		}
	}
}

// dismissOutside is the RegisterOutsideDismiss callback.
func (a *AnchoredPopup) dismissOutside() {
	if a == nil || !a.Open {
		return
	}
	a.SetOpen(false)
	if a.OnDismiss != nil {
		a.OnDismiss()
	}
}

func (a *AnchoredPopup) outsideID() string {
	if a == nil || a.Portal == nil {
		return ""
	}
	if a.Portal.ID == "" {
		// Ensure portal has an id for stable registration.
		a.Portal.ID = nextPortalID()
	}
	return "outside:" + a.Portal.ID
}

func (a *AnchoredPopup) treeRef() *core.Tree {
	if a == nil {
		return nil
	}
	if t := a.Tree(); t != nil {
		return t
	}
	if a.Portal != nil {
		if t := a.Portal.Tree(); t != nil {
			return t
		}
	}
	return nil
}

func (a *AnchoredPopup) syncOutsideDismiss() {
	if a == nil || !a.DismissOnOutside || !a.Open {
		a.clearOutsideDismiss()
		return
	}
	t := a.treeRef()
	if t == nil {
		return
	}
	keep := []core.Node{a.Content}
	if a.AnchorNode != nil {
		keep = append(keep, a.AnchorNode)
	}
	t.RegisterOutsideDismiss(a.outsideID(), a.dismissOutside, keep...)
}

func (a *AnchoredPopup) clearOutsideDismiss() {
	if a == nil {
		return
	}
	if t := a.treeRef(); t != nil {
		t.UnregisterOutsideDismiss(a.outsideID())
	}
}

// OnUnmount clears outside-dismiss registration.
func (a *AnchoredPopup) OnUnmount() {
	a.clearOutsideDismiss()
}

// UpdateAnchor sets the anchor rect (absolute) and repositions if open.
func (a *AnchoredPopup) UpdateAnchor(r core.Rect) {
	a.Anchor = r
	if a.Open {
		a.reposition()
	}
}

// UpdateAnchorFromNode uses a node's absolute bounds as the anchor and remembers AnchorNode.
func (a *AnchoredPopup) UpdateAnchorFromNode(n core.Node) {
	if n == nil {
		return
	}
	a.AnchorNode = n
	a.UpdateAnchor(core.AbsoluteBounds(n))
	if a.Open {
		a.syncOutsideDismiss()
	}
}

func (a *AnchoredPopup) resolveViewport() core.Size {
	if a.Viewport.Width > 0 && a.Viewport.Height > 0 {
		return a.Viewport
	}
	if t := a.treeRef(); t != nil {
		return t.Viewport()
	}
	return core.Size{}
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
	place := a.Placement
	var x, y float64

	placeOnce := func(pl Placement) (px, py float64) {
		switch pl {
		case PlaceTop, PlaceTopStart:
			px = ar.Min.X
			if pl == PlaceTop {
				px = ar.Min.X + (ar.Width()-cs.Width)/2
			}
			py = ar.Min.Y - gap - cs.Height
		case PlaceLeft:
			px = ar.Min.X - gap - cs.Width
			py = ar.Min.Y + (ar.Height()-cs.Height)/2
		case PlaceRight:
			px = ar.Max.X + gap
			py = ar.Min.Y + (ar.Height()-cs.Height)/2
		case PlaceBottomStart:
			px = ar.Min.X
			py = ar.Max.Y + gap
		default: // Bottom
			px = ar.Min.X + (ar.Width()-cs.Width)/2
			py = ar.Max.Y + gap
		}
		return px, py
	}

	x, y = placeOnce(place)

	vp := a.resolveViewport()
	if vp.Width > 0 && vp.Height > 0 {
		// Flip primary axis when overflowing and opposite side fits.
		switch place {
		case PlaceBottom, PlaceBottomStart:
			if y+cs.Height > vp.Height {
				altY := ar.Min.Y - gap - cs.Height
				if altY >= 0 {
					y = altY
				}
			}
		case PlaceTop, PlaceTopStart:
			if y < 0 {
				altY := ar.Max.Y + gap
				if altY+cs.Height <= vp.Height {
					y = altY
				}
			}
		case PlaceLeft:
			if x < 0 {
				altX := ar.Max.X + gap
				if altX+cs.Width <= vp.Width {
					x = altX
				}
			}
		case PlaceRight:
			if x+cs.Width > vp.Width {
				altX := ar.Min.X - gap - cs.Width
				if altX >= 0 {
					x = altX
				}
			}
		}
		// Shift clamp into viewport.
		if x+cs.Width > vp.Width {
			x = vp.Width - cs.Width
		}
		if y+cs.Height > vp.Height {
			y = vp.Height - cs.Height
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
