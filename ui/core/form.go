package core

// FieldStatus is validation chrome for form fields.
type FieldStatus int

const (
	FieldDefault FieldStatus = iota
	FieldError
	FieldWarning
	FieldSuccess
	FieldValidating
)

// FieldState holds one bound field (C-FormBind).
type FieldState struct {
	Name    string
	Value   string
	Touched bool
	Dirty   bool
	Status  FieldStatus
	// Errors are human-readable messages.
	Errors []string
	// Required marks empty as error on validate.
	Required bool
	// Rules are optional predicates; return error message or "".
	Rules []func(value string) string
}

// FormModel binds named fields and runs validation (C-FormBind).
type FormModel struct {
	fields map[string]*FieldState
	// Order of registration for ValidateAll.
	order []string
	// OnChange fires when any field value changes.
	OnChange func(name, value string)
	// OnFinish fires after successful ValidateAll.
	OnFinish func(values map[string]string)
}

// NewFormModel creates an empty form model.
func NewFormModel() *FormModel {
	return &FormModel{fields: make(map[string]*FieldState)}
}

// Register adds or replaces a field by name.
func (f *FormModel) Register(name string, required bool, rules ...func(string) string) *FieldState {
	if f.fields == nil {
		f.fields = make(map[string]*FieldState)
	}
	st, ok := f.fields[name]
	if !ok {
		st = &FieldState{Name: name}
		f.fields[name] = st
		f.order = append(f.order, name)
	}
	st.Required = required
	st.Rules = rules
	return st
}

// Field returns a field state.
func (f *FormModel) Field(name string) *FieldState {
	if f == nil || f.fields == nil {
		return nil
	}
	return f.fields[name]
}

// SetValue updates a field value.
func (f *FormModel) SetValue(name, value string) {
	st := f.Field(name)
	if st == nil {
		st = f.Register(name, false)
	}
	if st.Value == value {
		return
	}
	st.Value = value
	st.Dirty = true
	if f.OnChange != nil {
		f.OnChange(name, value)
	}
}

// Value returns a field value.
func (f *FormModel) Value(name string) string {
	st := f.Field(name)
	if st == nil {
		return ""
	}
	return st.Value
}

// Values returns all field values.
func (f *FormModel) Values() map[string]string {
	out := make(map[string]string, len(f.fields))
	for k, st := range f.fields {
		out[k] = st.Value
	}
	return out
}

// Touch marks a field touched (on blur).
func (f *FormModel) Touch(name string) {
	if st := f.Field(name); st != nil {
		st.Touched = true
	}
}

// Validate runs rules for one field.
func (f *FormModel) Validate(name string) bool {
	st := f.Field(name)
	if st == nil {
		return true
	}
	st.Errors = nil
	st.Status = FieldDefault
	if st.Required && st.Value == "" {
		st.Errors = append(st.Errors, "required")
	}
	for _, rule := range st.Rules {
		if msg := rule(st.Value); msg != "" {
			st.Errors = append(st.Errors, msg)
		}
	}
	if len(st.Errors) > 0 {
		st.Status = FieldError
		return false
	}
	if st.Dirty || st.Touched {
		st.Status = FieldSuccess
	}
	return true
}

// ValidateAll validates every field; on success calls OnFinish.
func (f *FormModel) ValidateAll() bool {
	ok := true
	for _, name := range f.order {
		if !f.Validate(name) {
			ok = false
		}
	}
	if ok && f.OnFinish != nil {
		f.OnFinish(f.Values())
	}
	return ok
}

// Reset clears values and validation state.
func (f *FormModel) Reset() {
	for _, st := range f.fields {
		st.Value = ""
		st.Touched = false
		st.Dirty = false
		st.Status = FieldDefault
		st.Errors = nil
	}
}
