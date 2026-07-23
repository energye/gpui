package primitive

import "github.com/energye/gpui/ui/core"

// TriggerMode controls how a Trigger opens its target.
type TriggerMode int

const (
	TriggerClick TriggerMode = iota
	TriggerHover
	TriggerFocus
)

// Trigger wires hover/click/focus on a child to open/close a callback (C-Trigger).
// Used by Tooltip/Popover without owning popup geometry.
//
// DelayMs > 0 delays hover open via Tree ticker (P0.2 B5). Leave closes immediately.
type Trigger struct {
	core.NodeBase

	Mode TriggerMode
	// DelayMs is hover open delay in milliseconds (0 = immediate).
	DelayMs int
	// Open state mirror.
	Open bool
	// OnOpenChange is called when open toggles.
	OnOpenChange func(open bool)

	// hover delay state (Ticker)
	pendingOpen bool
	delayLeft   float64 // ms remaining
}

// NewTrigger wraps a child as a trigger shell.
func NewTrigger(child core.Node) *Trigger {
	t := &Trigger{Mode: TriggerHover}
	t.Init(t)
	t.Hit = core.HitDefer
	if child != nil {
		t.AddChild(child)
	}
	return t
}

// TypeID implements core.Node.
func (t *Trigger) TypeID() string { return TypeTrigger }

// Layout implements core.Node.
func (t *Trigger) Layout(c core.Constraints) core.Size {
	kids := t.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		t.SetSize(out)
		return out
	}
	sz := kids[0].Layout(c.Expand())
	kids[0].Base().SetOffset(core.Point{})
	out := c.Tighten(sz)
	t.SetSize(out)
	return out
}

// Paint implements core.Node.
func (t *Trigger) Paint(pc *core.PaintContext) { t.DefaultPaintChildren(pc) }

// HitTest implements core.Node — prefer child; fall back to self for hover.
func (t *Trigger) HitTest(p core.Point) core.Node {
	if hit := t.DefaultHitTest(p); hit != nil {
		return hit
	}
	if t.LocalBounds().Contains(p) {
		return t
	}
	return nil
}

// SetOpen updates open state and notifies.
func (t *Trigger) SetOpen(open bool) {
	if t.Open == open {
		return
	}
	t.Open = open
	if t.OnOpenChange != nil {
		t.OnOpenChange(open)
	}
}

// HandlePointer implements open/close for click/hover modes.
func (t *Trigger) HandlePointer(ev *core.PointerEvent) {
	if ev == nil {
		return
	}
	switch t.Mode {
	case TriggerClick:
		// click handled via OnClick
	case TriggerHover:
		// hover open is driven by SetHovered; also open path on move inside self
		if ev.Type == core.PointerMove {
			t.SetHovered(true)
		}
	}
}

// OnClick toggles for click mode.
func (t *Trigger) OnClick(ev *core.PointerEvent) {
	if t.Mode == TriggerClick {
		t.SetOpen(!t.Open)
		if ev != nil {
			ev.Handled = true
		}
	}
}

// SetHovered opens/closes in hover mode. DelayMs defers open via Ticker.
func (t *Trigger) SetHovered(h bool) {
	if t.Mode != TriggerHover {
		return
	}
	if h {
		if t.Open || t.pendingOpen {
			return
		}
		if t.DelayMs <= 0 {
			t.SetOpen(true)
			return
		}
		t.pendingOpen = true
		t.delayLeft = float64(t.DelayMs)
		if tr := t.Tree(); tr != nil {
			tr.AddTicker(t)
		}
		return
	}
	// leave: cancel pending and close
	t.pendingOpen = false
	t.delayLeft = 0
	if tr := t.Tree(); tr != nil {
		tr.RemoveTicker(t)
	}
	t.SetOpen(false)
}

// Tick implements core.Ticker for hover delay (returns still while waiting).
func (t *Trigger) Tick(dt float64) bool {
	if t == nil || !t.pendingOpen {
		return false
	}
	t.delayLeft -= dt * 1000
	if t.delayLeft > 0 {
		return true
	}
	t.pendingOpen = false
	t.delayLeft = 0
	t.SetOpen(true)
	return false
}

// OnUnmount cancels pending delay ticker.
func (t *Trigger) OnUnmount() {
	t.pendingOpen = false
	if tr := t.Tree(); tr != nil {
		tr.RemoveTicker(t)
	}
}
