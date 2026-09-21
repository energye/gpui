package behavior

import (
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/gestures"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/semantics"
)

// InteractiveConfig describes one pressable control.
type InteractiveConfig struct {
	Disabled  bool
	Loading   bool
	Focusable bool
	Label     string
	Role      semantics.Role
}

// Interactive owns the five-state machine for one pressable control:
// idle, hover, pressed, keyboard focus, disabled, loading.
//
// It wraps ui/gestures tap arbitration and ui/focus keyboard handling.
// It never draws and never hardcodes colors; render resolves appearance
// from States via scope.StateResolver.
type Interactive struct {
	cfg    InteractiveConfig
	states scope.WidgetState
	tap    *gestures.TapGestureRecognizer
	node   *focus.FocusNode
	mgr    *focus.FocusManager

	clicks      int
	activations int
}

// NewInteractive builds the machine from config.
func NewInteractive(cfg InteractiveConfig) *Interactive {
	in := &Interactive{cfg: cfg}
	if cfg.Disabled {
		in.states = in.states.With(scope.StateDisabled)
	}
	if cfg.Loading {
		in.states = in.states.With(scope.StateLoading)
	}
	in.tap = gestures.NewTap()
	in.tap.OnTap = func(gestures.PointerEvent) { in.activateFromPointer() }
	role := cfg.Role
	if role == semantics.RoleNone {
		role = semantics.RoleButton
	}
	_ = role
	in.node = focus.NewFocusNode(cfg.Label)
	in.node.Enabled = !cfg.Disabled && cfg.Focusable
	if cfg.Focusable {
		in.node.TabIndex = 0
	} else {
		in.node.TabIndex = -1
	}
	in.node.OnFocusChange = func(focused bool) { in.onFocusChange(focused) }
	in.node.OnActivate = func() { in.activateFromKeyboard() }
	return in
}

// Mount registers the focus node with mgr.
func (in *Interactive) Mount(mgr *focus.FocusManager) {
	if in == nil {
		return
	}
	in.mgr = mgr
	if mgr == nil || in.node == nil {
		return
	}
	mgr.Register(in.node)
}

// Unmount unregisters the focus node.
func (in *Interactive) Unmount() {
	if in == nil {
		return
	}
	if in.node != nil {
		in.node.Unregister()
	}
	in.mgr = nil
}

// States reports the current machine bits.
func (in *Interactive) States() scope.WidgetState {
	if in == nil {
		return 0
	}
	return in.states
}

// CanActivate reports whether a press counts: neither disabled nor loading.
func (in *Interactive) CanActivate() bool {
	if in == nil {
		return false
	}
	return !in.states.Has(scope.StateDisabled) && !in.states.Has(scope.StateLoading)
}

// Clicks counts accepted activations (pointer plus keyboard).
func (in *Interactive) Clicks() int {
	if in == nil {
		return 0
	}
	return in.clicks
}

// Activations counts keyboard activations only.
func (in *Interactive) Activations() int {
	if in == nil {
		return 0
	}
	return in.activations
}

// TapRecognizer exposes the arena member for window wiring.
func (in *Interactive) TapRecognizer() *gestures.TapGestureRecognizer {
	if in == nil {
		return nil
	}
	return in.tap
}

// FocusNode exposes the keyboard node for window wiring.
func (in *Interactive) FocusNode() *focus.FocusNode {
	if in == nil {
		return nil
	}
	return in.node
}

// JoinArena adds the tap member to an arena for one pointer down.
func (in *Interactive) JoinArena(m *gestures.GestureArenaManager, id int, e gestures.PointerEvent) {
	if in == nil || in.tap == nil || m == nil {
		return
	}
	a := m.Arena(id)
	a.Add(in.tap)
	in.tap.AddPointer(id, e)
}

// SetHover tracks pointer enter and leave. Disabled clears hover.
func (in *Interactive) SetHover(over bool) bool {
	if in == nil {
		return false
	}
	if in.states.Has(scope.StateDisabled) {
		in.states = in.states.Without(scope.StateHover | scope.StatePressed)
		return false
	}
	if over {
		in.states = in.states.With(scope.StateHover)
		return true
	}
	in.states = in.states.Without(scope.StateHover)
	return false
}

// SetPressed tracks pointer down and up. Disabled clears pressed.
func (in *Interactive) SetPressed(down bool) bool {
	if in == nil {
		return false
	}
	if in.states.Has(scope.StateDisabled) {
		in.states = in.states.Without(scope.StatePressed)
		return false
	}
	if down {
		in.states = in.states.With(scope.StatePressed)
		return true
	}
	in.states = in.states.Without(scope.StatePressed)
	return false
}

// DirectTap simulates one arena-winning tap for scripted checks.
func (in *Interactive) DirectTap() bool {
	if in == nil {
		return false
	}
	return in.activateFromPointer()
}

// HandleKey routes Tab through the manager and Space/Enter as activation.
// Pointer clicks never set focus; only this keyboard path can activate
// with the focused bit managed by onFocusChange.
func (in *Interactive) HandleKey(e focus.KeyEvent) bool {
	if in == nil {
		return false
	}
	if in.states.Has(scope.StateDisabled) {
		return false
	}
	if e.IsTab() {
		if in.mgr != nil {
			return in.mgr.HandleKey(e)
		}
		return false
	}
	if e.Pressed && e.IsActivate() {
		if !in.CanActivate() {
			return false
		}
		in.activations++
		in.clicks++
		return true
	}
	return false
}

// SetDisabled flips the disabled bit with OR semantics: callers pass the
// per-widget flag, while scope.DisabledOr combines it with the subtree flag
// at resolve time. Disabling clears hover, pressed and focus.
func (in *Interactive) SetDisabled(disabled bool) {
	if in == nil {
		return
	}
	in.cfg.Disabled = disabled
	if disabled {
		in.states = in.states.With(scope.StateDisabled)
		in.states = in.states.Without(scope.StateHover | scope.StatePressed | scope.StateFocused)
		if in.node != nil {
			in.node.Enabled = false
			in.node.Unfocus()
		}
		return
	}
	in.states = in.states.Without(scope.StateDisabled)
	if in.node != nil {
		in.node.Enabled = in.cfg.Focusable
	}
}

// SetLoading flips the loading bit. Loading suppresses activation.
func (in *Interactive) SetLoading(loading bool) {
	if in == nil {
		return
	}
	in.cfg.Loading = loading
	if loading {
		in.states = in.states.With(scope.StateLoading)
		return
	}
	in.states = in.states.Without(scope.StateLoading)
}

// Semantics reports the accessible name, role and focusability.
// Every pressable keeps a name and a role; unnamed callers fall back to
// the button role so screen readers never see a nameless control.
func (in *Interactive) Semantics() *semantics.Node {
	if in == nil {
		return semantics.New(semantics.RoleButton, "")
	}
	role := in.cfg.Role
	if role == semantics.RoleNone {
		role = semantics.RoleButton
	}
	n := semantics.New(role, in.cfg.Label)
	n.Focusable = in.cfg.Focusable && !in.states.Has(scope.StateDisabled)
	return n
}

// Contains reports rect hit for window routing: Hit must equal paint.
func Contains(x, y, w, h, px, py float64) bool {
	return px >= x && px < x+w && py >= y && py < y+h
}

func (in *Interactive) activateFromPointer() bool {
	if in == nil || !in.CanActivate() {
		return false
	}
	in.clicks++
	return true
}

func (in *Interactive) activateFromKeyboard() {
	if in == nil || !in.CanActivate() {
		return
	}
	in.activations++
	in.clicks++
}

func (in *Interactive) onFocusChange(focused bool) {
	if in == nil {
		return
	}
	if in.states.Has(scope.StateDisabled) {
		return
	}
	if focused {
		in.states = in.states.With(scope.StateFocused)
		return
	}
	in.states = in.states.Without(scope.StateFocused)
}
