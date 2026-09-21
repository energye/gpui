package gate

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// StateFlow is the window-reported state audit: every state the
// component claims must resolve through StateResolver (no inline
// hover/pressed branches in Render).
type StateFlow struct {
	States   []scope.WidgetState
	Resolved []bool
}

// CheckState reports whether every claimed state resolves.
// Requires the Disabled bit present (disabled-look never leaks
// hover/pressed colors) and every entry resolved.
func CheckState(f StateFlow) bool {
	if len(f.States) == 0 || len(f.States) != len(f.Resolved) {
		return false
	}
	seenDisabled := false
	for i, s := range f.States {
		if !f.Resolved[i] {
			return false
		}
		if s.Has(scope.StateDisabled) {
			seenDisabled = true
		}
	}
	return seenDisabled
}
