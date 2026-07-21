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
type Trigger struct {
	core.NodeBase

	Mode TriggerMode
	// DelayMs is reserved for hover delay (M2: immediate; delay can be host-driven).
	DelayMs int
	// Open state mirror.
	Open bool
	// OnOpenChange is called when open toggles.
	OnOpenChange func(open bool)

	// Child is the interactive element (Pressable/Focusable typically).
	// Stored as first child.
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
		// hover open is driven by SetHovered on pressable children;
		// also open when pointer moves inside self
		if ev.Type == core.PointerMove {
			t.SetOpen(true)
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

// SetHovered opens/closes in hover mode when the trigger itself is hovered
// (exported for tree; Trigger may not be the hit target — callers may drive SetOpen).
func (t *Trigger) SetHovered(h bool) {
	if t.Mode == TriggerHover {
		t.SetOpen(h)
	}
}
