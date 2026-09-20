package scope

// WidgetState names the current interaction situation of one mounted
// instance. States combine as bits; idle is the zero value.
type WidgetState uint32

const (
	// StateHover reports the pointer is over the widget.
	StateHover WidgetState = 1 << iota
	// StatePressed reports the widget is held down.
	StatePressed
	// StateFocused reports keyboard focus (focus-visible only;
	// pointer clicks never set it).
	StateFocused
	// StateDisabled reports the widget (or its subtree) is disabled.
	// Disabled swallows events and never shows a focus ring.
	StateDisabled
	// StateLoading reports a pending action with spinner state.
	StateLoading
	// StateSelected reports a toggled-on state.
	StateSelected
	// StateError reports a validation error state.
	StateError
)

// Has reports whether all bits in want are set.
func (s WidgetState) Has(want WidgetState) bool {
	return s&want == want
}

// With returns the state with want bits set.
func (s WidgetState) With(want WidgetState) WidgetState {
	return s | want
}

// Without returns the state with want bits cleared.
func (s WidgetState) Without(want WidgetState) WidgetState {
	return s &^ want
}

// StateResolver maps the current WidgetState to one value, mirroring
// Flutter's WidgetStateProperty. Render code calls Resolve instead of
// branching on hover/pressed/focused inline.
type StateResolver[T any] interface {
	Resolve(s WidgetState) T
}

// StateResolverFunc adapts a plain function to StateResolver.
type StateResolverFunc[T any] func(WidgetState) T

// Resolve calls the adapted function.
func (f StateResolverFunc[T]) Resolve(s WidgetState) T {
	return f(s)
}

// StaticResolver always returns the same value.
type StaticResolver[T any] struct {
	Value T
}

// Resolve returns the static value.
func (r StaticResolver[T]) Resolve(WidgetState) T {
	return r.Value
}

// MapResolver looks up each state exactly. Missing states fall back
// to Default, so tables stay total even when a caller forgets a row.
type MapResolver[T any] struct {
	Values  map[WidgetState]T
	Default T
}

// Resolve returns the table entry or Default when absent.
func (r MapResolver[T]) Resolve(s WidgetState) T {
	if v, ok := r.Values[s]; ok {
		return v
	}
	return r.Default
}

// DisabledFirstResolver picks Disabled whenever the disabled bit is
// set, otherwise it delegates. This keeps the disabled look from
// leaking hover/pressed colors.
type DisabledFirstResolver[T any] struct {
	Disabled T
	Rest     StateResolver[T]
}

// Resolve returns Disabled when StateDisabled is set.
func (r DisabledFirstResolver[T]) Resolve(s WidgetState) T {
	if s.Has(StateDisabled) {
		return r.Disabled
	}
	if r.Rest == nil {
		var zero T
		return zero
	}
	return r.Rest.Resolve(s)
}
