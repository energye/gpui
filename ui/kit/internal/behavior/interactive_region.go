//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package behavior

import (
	"github.com/energye/gpui/ui/gestures"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/rendering"
)

// InteractiveRegion is the Listener/InkWell hit target (KIT_FRAME_BINDING_PLAN
// 件 3 node side): a render node that wraps the visual subtree and joins the
// gesture arena automatically once the input router walks the down hit path.
// Zero window-side wiring — no manual hit tests, no hand-fed arenas.
//
// Behavior mapping:
//   - Down: router hands the path to the arena; this node's tap recognizer
//     joins via JoinPointer and pressed sets (arena members see the down
//     through Manager.Route in the same walk).
//   - Up within slop: tap wins → OnTap; pressing clears. Out of slop or
//     arena loss → tap rejects and pressing clears ( recognizer.Reject path).
//   - Move: hover on; exit is synthesized by the router when the hit path
//     stops containing this node (PointerExitHandler).
//
// The pressed/hover bits live in the composed Interactive machine, so the
// region reuses the five-state semantics (disabled swallows, loading guards)
// instead of a parallel state store.
type InteractiveRegion struct {
	*rendering.RenderBox

	ctl *Interactive
	tap *gestures.TapGestureRecognizer

	// OnTap fires once per arena-winning tap (up within slop).
	OnTap func()
}

// NewInteractiveRegion wraps child with arena participation. cfg feeds the
// composed state machine (Disabled/Loading/Focusable semantics included).
func NewInteractiveRegion(cfg InteractiveConfig, child rendering.RenderObject) *InteractiveRegion {
	r := &InteractiveRegion{
		RenderBox: rendering.NewRenderBox(),
		ctl:       NewInteractive(cfg),
	}
	// Init BEFORE AddChild so the child's parent slot records the outer
	// region (the promotion wrapper), not the bare *RenderBox — the
	// router's hit-path walk dispatches by dynamic type, and a wrapper
	// whose parent chain skips it never receives OnPointer/JoinPointer.
	r.RenderBox.Init(r)
	if child != nil {
		r.RenderBox.AddChild(child)
	}
	r.tap = gestures.NewTap()
	r.tap.OnTap = func(gestures.PointerEvent) {
		if r.ctl.activateFromPointer() && r.OnTap != nil {
			r.OnTap()
		}
	}
	return r
}

// HitTest resolves self-hits to the region (not the embedded box) so the
// router's handler walk starts from a node that carries OnPointer.
func (r *InteractiveRegion) HitTest(p rendering.Point) rendering.RenderObject {
	if r == nil || r.RenderBox == nil {
		return nil
	}
	ch := r.RenderBox.HitTest(p)
	if ch == nil {
		return nil
	}
	if ch == r.RenderBox {
		return r
	}
	return ch
}

// JoinPointer implements gestures.PointerTarget: the router calls this on
// the down hit path; the tap recognizer joins the shared arena here.
func (r *InteractiveRegion) JoinPointer(m *gestures.GestureArenaManager, pointerID int, e gestures.PointerEvent) {
	if r == nil || m == nil || r.tap == nil {
		return
	}
	if r.ctl.States().Has(scope.StateDisabled | scope.StateLoading) {
		return
	}
	a := m.Arena(pointerID)
	a.Add(r.tap)
	r.tap.AddPointer(pointerID, e)
}

// OnPointer implements input.PointerHandler for the non-arena edges:
// pressed follows down/up arbitration results, hover follows enter/leave
// and in-bounds moves.
func (r *InteractiveRegion) OnPointer(ev input.PointerEvent) {
	if r == nil || r.ctl == nil {
		return
	}
	switch ev.Kind {
	case input.PointerDown:
		if r.hitLocal(ev) {
			r.ctl.SetPressed(true)
		}
	case input.PointerUp:
		r.ctl.SetPressed(false)
	case input.PointerEnter:
		r.ctl.SetHover(true)
	case input.PointerLeave:
		r.ctl.SetHover(false)
	case input.PointerMove:
		r.ctl.SetHover(r.hitLocal(ev))
	}
}

// ReplaceChild swaps the wrapped visual node (nil-child construction).
// The new child joins under the region node with layout dirtying handled
// by AddChild.
func (r *InteractiveRegion) ReplaceChild(child rendering.RenderObject) {
	if r == nil || r.RenderBox == nil || child == nil {
		return
	}
	for _, old := range r.RenderBox.Children() {
		r.RenderBox.RemoveChild(old)
	}
	r.RenderBox.AddChild(child)
}

// Ctl exposes the composed Interactive machine (focus mounting, keyboard
// activation, click counters for tests/windows).
func (r *InteractiveRegion) Ctl() *Interactive {
	return r.ctl
}

// TapRecognizer exposes the arena member for inspection.
func (r *InteractiveRegion) TapRecognizer() *gestures.TapGestureRecognizer {
	if r == nil {
		return nil
	}
	return r.tap
}

// States reports the composed machine bits (hover/pressed/disabled/loading)
// for windows and tests.
func (r *InteractiveRegion) States() scope.WidgetState {
	if r == nil || r.ctl == nil {
		return 0
	}
	return r.ctl.States()
}

// hoverOn reports the composed hover bit (window/test observation).
func (r *InteractiveRegion) hoverOn() bool {
	return r.ctl.States().Has(scope.StateHover)
}

// hitLocal maps a window-space pointer sample into this box's bounds.
// Offset is set by the parent's Layout; before layout (unit-built trees)
// offset is zero and the box size is the fixed preference.
func (r *InteractiveRegion) hitLocal(ev input.PointerEvent) bool {
	off := r.Offset()
	sz := r.Size()
	x, y := ev.X-off.X, ev.Y-off.Y
	return x >= 0 && y >= 0 && x < sz.Width && y < sz.Height
}
