package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// overlayFocusTrap is shared keyboard focus handling for full-screen portal
// overlays (Modal, Drawer). Pattern: FocusScope.Active + Escape + restore focus.
//
// Present Z-order is compositor dual-band (MAP §4.1), not this helper.
type overlayFocusTrap struct {
	prev core.Node
}

func (tr *overlayFocusTrap) wire(scope *primitive.FocusScope, active bool, onEsc func()) {
	if scope == nil {
		return
	}
	scope.Active = active
	if active {
		scope.OnEscape = onEsc
	} else {
		scope.OnEscape = nil
	}
}

func (tr *overlayFocusTrap) enter(scope *primitive.FocusScope, portal *primitive.OverlayPortal, prefer core.Node) {
	if tr == nil || scope == nil {
		return
	}
	t := scope.Tree()
	if t == nil && portal != nil {
		t = portal.Tree()
	}
	if t == nil {
		return
	}
	tr.prev = t.Focus()
	list := core.CollectFocusables(scope)
	if len(list) == 0 {
		return
	}
	next := list[0]
	if prefer != nil {
		for _, n := range list {
			if n == prefer {
				next = n
				break
			}
		}
	}
	t.SetFocus(next)
}

func (tr *overlayFocusTrap) leave(scope *primitive.FocusScope, portal *primitive.OverlayPortal) {
	if tr == nil {
		return
	}
	var t *core.Tree
	if scope != nil {
		t = scope.Tree()
	}
	if t == nil && portal != nil {
		t = portal.Tree()
	}
	prev := tr.prev
	tr.prev = nil
	if t == nil {
		return
	}
	if prev != nil {
		if f, ok := prev.(core.FocusTarget); ok && f.CanFocus() && prev.Base() != nil && prev.Base().Tree() == t {
			t.SetFocus(prev)
			return
		}
	}
	t.SetFocus(nil)
}

// resolveOverlayViewport returns a finite client size for full-screen masks.
// Prefer explicit Viewport, else tree viewport, else defaults.
func resolveOverlayViewport(explicit core.Size, portal *primitive.OverlayPortal, maxW, maxH float64) (vw, vh float64) {
	vw, vh = explicit.Width, explicit.Height
	if (vw <= 0 || vh <= 0) && portal != nil {
		if t := portal.Tree(); t != nil {
			tv := t.Viewport()
			if vw <= 0 && tv.Width > 0 {
				vw = tv.Width
			}
			if vh <= 0 && tv.Height > 0 {
				vh = tv.Height
			}
		}
	}
	if vw <= 0 {
		if maxW > 0 && maxW < core.Unbounded/2 {
			vw = maxW
		} else {
			vw = 800
		}
	}
	if vh <= 0 {
		if maxH > 0 && maxH < core.Unbounded/2 {
			vh = maxH
		} else {
			vh = 600
		}
	}
	if vw >= core.Unbounded/2 {
		vw = 800
	}
	if vh >= core.Unbounded/2 {
		vh = 600
	}
	return vw, vh
}
