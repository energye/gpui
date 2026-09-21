package behavior

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// FieldConfig describes one form value: controlled when Value is set,
// uncontrolled when only DefaultValue is set.
type FieldConfig struct {
	Value        *string
	DefaultValue string
	OnChange     func(string)
	Validate     func(string) string
}

// Field owns the committed value plus the in-progress IME composition.
// Composition shows but never counts as the value; OnChange fires only
// on committed submits. Controlled fields never store; they only notify.
type Field struct {
	controlled  bool
	committed   string
	composing   string
	onChange    func(string)
	validate    func(string) string
	errText     string
	changeCalls int
	lastNotify  string
}

// IMEController is the narrow bridge to textinput.Editor. Field satisfies
// it; product adapters wrap Editor without importing textinput here, so
// field never depends on the IME implementation (no inverted dependency).
type IMEController interface {
	Committed() string
	SetComposing(string)
	CommitText(string)
}

// NewField builds the controller from config.
func NewField(cfg FieldConfig) *Field {
	f := &Field{onChange: cfg.OnChange, validate: cfg.Validate}
	if cfg.Value != nil {
		f.controlled = true
		f.committed = *cfg.Value
		return f
	}
	f.committed = cfg.DefaultValue
	return f
}

// IsControlled reports whether the value comes from outside props.
func (f *Field) IsControlled() bool { return f != nil && f.controlled }

// Value returns the committed value, never the composition.
func (f *Field) Value() string {
	if f == nil {
		return ""
	}
	return f.committed
}

// Display returns what render shows: composition while active.
func (f *Field) Display() string {
	if f == nil {
		return ""
	}
	if f.composing != "" {
		return f.composing
	}
	return f.committed
}

// Committed satisfies IMEController.
func (f *Field) Committed() string { return f.Value() }

// IsComposing reports an active pre-edit string.
func (f *Field) IsComposing() bool { return f != nil && f.composing != "" }

// ChangeCalls counts OnChange invocations.
func (f *Field) ChangeCalls() int {
	if f == nil {
		return 0
	}
	return f.changeCalls
}

// LastNotified returns the last value passed to OnChange.
func (f *Field) LastNotified() string {
	if f == nil {
		return ""
	}
	return f.lastNotify
}

// Input commits one submitted text. Controlled fields only notify;
// uncontrolled fields store then notify.
func (f *Field) Input(s string) {
	if f == nil {
		return
	}
	if f.controlled {
		f.notify(s)
		return
	}
	f.committed = s
	f.notify(s)
}

// CommitText satisfies IMEController.
func (f *Field) CommitText(s string) { f.Input(s) }

// SetComposing sets the display-only pre-edit string. No value change,
// no callback, so counters and search stay on the committed value.
func (f *Field) SetComposing(s string) {
	if f == nil {
		return
	}
	f.composing = s
}

// CommitComposing submits the active pre-edit as one committed input.
func (f *Field) CommitComposing() bool {
	if f == nil || f.composing == "" {
		return false
	}
	s := f.composing
	f.composing = ""
	f.Input(s)
	return true
}

// SyncExternal applies a props-side value change for controlled fields
// without notifying. Uncontrolled fields ignore it: they own the value.
func (f *Field) SyncExternal(v string) bool {
	if f == nil || !f.controlled {
		return false
	}
	if f.committed == v {
		return false
	}
	f.committed = v
	return true
}

// Validate runs the configured check on the committed value.
func (f *Field) Validate() string {
	if f == nil {
		return ""
	}
	if f.validate == nil {
		f.errText = ""
		return ""
	}
	f.errText = f.validate(f.committed)
	return f.errText
}

// Error returns the last validation text.
func (f *Field) Error() string {
	if f == nil {
		return ""
	}
	return f.errText
}

// HasError reports a non-empty validation text.
func (f *Field) HasError() bool { return f != nil && f.errText != "" }

// ErrorState maps validation to the scope error bit for StateResolver.
func (f *Field) ErrorState() scope.WidgetState {
	if f.HasError() {
		return scope.StateError
	}
	return 0
}

func (f *Field) notify(s string) {
	f.changeCalls++
	f.lastNotify = s
	if f.onChange != nil {
		f.onChange(s)
	}
}
