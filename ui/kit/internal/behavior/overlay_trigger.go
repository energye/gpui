package behavior

import (
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
)

// escapeSym matches the platform Escape keysym (0xFF1B). Focus carries
// X11 keysyms, so behavior checks this value directly instead of adding
// a second key vocabulary.
const escapeSym = 0xFF1B

// TriggerConfig describes one floating layer trigger.
type TriggerConfig struct {
	Want      overlay.Placement
	Options   *overlay.ResolveOptions
	Barrier   bool
	FocusTrap bool
	EscCloses bool
}

// OverlayTrigger owns open state, placement, outside-close and focus lock.
// Placement always delegates to overlay.Resolve; this type never
// reimplements the twelve-direction math.
type OverlayTrigger struct {
	cfg      TriggerConfig
	open     bool
	anchor   rendering.Rect
	ow       float64
	oh       float64
	resolved overlay.Resolved
	entry    *overlay.Entry
	lock     *overlay.FocusLock
	node     *focus.FocusNode
	mgr      *focus.FocusManager
	st       *overlay.State
}

// NewTrigger builds the trigger over an overlay stack and focus manager.
// Either ref may be nil for pure resolve tests; Open then only resolves.
func NewTrigger(cfg TriggerConfig, mgr *focus.FocusManager, st *overlay.State) *OverlayTrigger {
	return &OverlayTrigger{cfg: cfg, mgr: mgr, st: st}
}

// ResolveFollower is the canonical D-class placement entry: anchor plus
// overlay size through overlay.Resolve with no local math. Prim keeps its
// F0-2 helper for layout windows; new floating layers call this one.
func ResolveFollower(anchor rendering.Rect, ow, oh float64, want overlay.Placement, opt *overlay.ResolveOptions) overlay.Resolved {
	return overlay.Resolve(anchor, ow, oh, want, opt)
}

// Resolve positions without changing open state.
func (t *OverlayTrigger) Resolve(anchor rendering.Rect, ow, oh float64) overlay.Resolved {
	if t == nil {
		return overlay.Resolve(anchor, ow, oh, overlay.Bottom, nil)
	}
	return overlay.Resolve(anchor, ow, oh, t.cfg.Want, t.cfg.Options)
}

// Open resolves and inserts the entry, then traps focus when configured.
func (t *OverlayTrigger) Open(anchor rendering.Rect, ow, oh float64) bool {
	if t == nil {
		return false
	}
	if t.open {
		return true
	}
	t.anchor = anchor
	t.ow, t.oh = ow, oh
	t.resolved = overlay.Resolve(anchor, ow, oh, t.cfg.Want, t.cfg.Options)
	if t.st != nil {
		if t.cfg.Barrier {
			t.entry = overlay.NewBarrierEntry(t.resolved.X, t.resolved.Y, ow, oh, nil)
		} else {
			t.entry = overlay.NewEntry(nil, t.resolved.X, t.resolved.Y, ow, oh)
		}
		t.st.Insert(t.entry)
	} else {
		if t.cfg.Barrier {
			t.entry = overlay.NewBarrierEntry(t.resolved.X, t.resolved.Y, ow, oh, nil)
		} else {
			t.entry = overlay.NewEntry(nil, t.resolved.X, t.resolved.Y, ow, oh)
		}
	}
	if t.cfg.FocusTrap && t.mgr != nil {
		t.node = focus.NewFocusNode("overlay-trigger")
		t.mgr.Register(t.node)
		t.lock = overlay.NewFocusLock(t.mgr, t.node)
		t.lock.Acquire()
	}
	t.open = true
	return true
}

// Close removes the entry and restores the saved focus.
func (t *OverlayTrigger) Close() bool {
	if t == nil || !t.open {
		return false
	}
	if t.st != nil && t.entry != nil {
		t.st.Remove(t.entry)
	}
	if t.lock != nil {
		t.lock.Release()
		t.lock = nil
	}
	if t.node != nil {
		if t.mgr != nil {
			t.mgr.Unregister(t.node)
		}
		t.node = nil
	}
	t.entry = nil
	t.open = false
	return true
}

// IsOpen reports whether the layer is up.
func (t *OverlayTrigger) IsOpen() bool { return t != nil && t.open }

// ResolvedBox returns the last placement outcome.
func (t *OverlayTrigger) ResolvedBox() overlay.Resolved {
	if t == nil {
		return overlay.Resolved{}
	}
	return t.resolved
}

// Entry exposes the stack entry for HitTest assertions.
func (t *OverlayTrigger) Entry() *overlay.Entry {
	if t == nil {
		return nil
	}
	return t.entry
}

// FocusNode exposes the trapped node for tests.
func (t *OverlayTrigger) FocusNode() *focus.FocusNode {
	if t == nil {
		return nil
	}
	return t.node
}

// IsFocusInside reports whether n belongs to the trapped set.
func (t *OverlayTrigger) IsFocusInside(n *focus.FocusNode) bool {
	if t == nil || t.lock == nil {
		return false
	}
	return t.lock.Contains(n)
}

// HandleOutside closes on pointer down outside every entry.
func (t *OverlayTrigger) HandleOutside(p rendering.Point) bool {
	if t == nil || !t.open {
		return false
	}
	if t.entry != nil && t.entry.Contains(p) {
		return false
	}
	if t.st != nil && !overlay.HitOutside(t.st, p) {
		return false
	}
	t.Close()
	return true
}

// HandleKey closes on Esc when configured. Tab stays with the manager.
func (t *OverlayTrigger) HandleKey(e focus.KeyEvent) bool {
	if t == nil || !t.open {
		return false
	}
	if !e.Pressed {
		return false
	}
	if e.KeyCode == escapeSym && t.cfg.EscCloses {
		t.Close()
		return true
	}
	return false
}
