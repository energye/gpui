package kit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Form defaults — components/form/style prepareComponentToken.
// docs/antd/form.md §6.2 / §6.10
const (
	DefaultFormItemGap          = 24.0 // itemMarginBottom = marginLG
	DefaultFormInlineItemGap    = 16.0 // horizontal gap between inline items
	DefaultFormFieldGap         = 8.0  // vertical label↔control (paddingSM)
	DefaultFormErrorGap         = 4.0  // control↔error (marginXXS)
	DefaultFormColonMarginStart = 2.0  // labelColonMarginInlineStart
	DefaultFormColonMarginEnd   = 8.0  // labelColonMarginInlineEnd
	DefaultFormLabelColSpan     = 8
	DefaultFormWrapperColSpan   = 16
	DefaultFormRequiredMessage  = "required"
	DefaultFormOptionalText     = "optional"
	DefaultFormRequiredMarkChar = "*"
)

// FormLayout is antd Form layout.
type FormLayout int

const (
	FormHorizontal FormLayout = iota
	FormVertical
	FormInline
)

// FormSize propagates to bound field controls.
type FormSize int

const (
	FormMiddle FormSize = iota
	FormSmall
	FormLarge
)

// FormVariant propagates to bound field controls.
type FormVariant int

const (
	FormOutlined FormVariant = iota
	FormFilled
	FormBorderless
	FormUnderlined
)

// FormRequiredMark controls required/optional chrome.
type FormRequiredMark int

const (
	// FormRequiredMarkDefault shows red * before required labels (antd true).
	FormRequiredMarkDefault FormRequiredMark = iota
	// FormRequiredMarkOptional shows "optional" after non-required labels.
	FormRequiredMarkOptional
	// FormRequiredMarkHidden shows no required/optional chrome (antd false).
	FormRequiredMarkHidden
)

// FormLabelAlign is label text alignment in horizontal layout.
type FormLabelAlign int

const (
	FormLabelRight FormLabelAlign = iota
	FormLabelLeft
)

// FormValidateTrigger is when field rules run.
type FormValidateTrigger int

const (
	FormValidateOnChange FormValidateTrigger = iota
	FormValidateOnBlur
	FormValidateOnSubmit
)

// FormCol is a simplified Col span/offset for label/wrapper.
type FormCol struct {
	Span   int
	Offset int
}

// FormName is a field path (antd NamePath). Empty means non-field wrapper item.
type FormName []string

// Name builds a FormName path.
func Name(parts ...string) FormName {
	out := make(FormName, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ParseFormName splits a dotted path "a.b.0.c".
func ParseFormName(s string) FormName {
	if s == "" {
		return nil
	}
	return Name(strings.Split(s, ".")...)
}

func (n FormName) String() string {
	if len(n) == 0 {
		return ""
	}
	return strings.Join(n, ".")
}

func (n FormName) clone() FormName {
	if len(n) == 0 {
		return nil
	}
	out := make(FormName, len(n))
	copy(out, n)
	return out
}

func (n FormName) equal(o FormName) bool {
	if len(n) != len(o) {
		return false
	}
	for i := range n {
		if n[i] != o[i] {
			return false
		}
	}
	return true
}

// FormRule is one validation rule (antd Rule subset).
type FormRule struct {
	Required  bool
	Message   string
	Validator func(value any) string // "" = ok
}

// FormFailInfo is passed to OnFinishFailed.
type FormFailInfo struct {
	Values      map[string]any
	ErrorFields []FormFieldError
}

// FormFieldError is one failed field after validate.
type FormFieldError struct {
	Name   FormName
	Errors []string
}

// formFieldState is internal store state for one registered path.
type formFieldState struct {
	name     FormName
	value    any
	errors   []string
	touched  bool
	dirty    bool
	required bool
	rules    []FormRule
}

// Form is Ant Design Form: layout shell + data store + item registry.
//
//	Form Root
//	  ├─ FormItem* (label + control + error)
//	  ├─ FormList* metadata (rows via NameAt + AddItem)
//	  └─ free children (submit buttons, …)
//
// Product contract: docs/antd/form.md §6 (P0 DoD).
type Form struct {
	Root *primitive.Flex

	Layout          FormLayout
	Size            FormSize
	Variant         FormVariant
	Disabled        bool
	Colon           bool
	RequiredMark    FormRequiredMark
	LabelAlign      FormLabelAlign
	LabelWrap       bool
	LabelCol        FormCol
	WrapperCol      FormCol
	Name            string
	Preserve        bool
	ValidateTrigger FormValidateTrigger
	ItemGap         float64 // 0 → DefaultFormItemGap
	InitialValues   map[string]any
	AriaLabel       string
	Face            text.Face
	Theme           *core.Theme
	OnFinish        func(values map[string]any)
	OnFinishFailed  func(info FormFailInfo)
	OnValuesChange  func(changed, all map[string]any)

	items  []*FormItem
	lists  []*FormList
	extras []core.Node
	fields map[string]*formFieldState // key = FormName.String()
	order  []string
	values map[string]any // nested tree mirror of flat fields + lists
	inited bool
}

// NewForm creates a Form with antd defaults (horizontal, middle, outlined).
func NewForm() *Form {
	f := &Form{
		Layout:          FormHorizontal,
		Size:            FormMiddle,
		Variant:         FormOutlined,
		Colon:           true,
		RequiredMark:    FormRequiredMarkDefault,
		LabelAlign:      FormLabelRight,
		LabelCol:        FormCol{Span: DefaultFormLabelColSpan},
		WrapperCol:      FormCol{Span: DefaultFormWrapperColSpan},
		Preserve:        true,
		ValidateTrigger: FormValidateOnChange,
		fields:          make(map[string]*formFieldState),
		values:          make(map[string]any),
	}
	f.rebuild()
	return f
}

// Node returns the form root.
func (f *Form) Node() core.Node {
	if f == nil {
		return nil
	}
	if f.Root == nil {
		f.rebuild()
	}
	return f.Root
}

// ChromeNode returns the same root (form is layout chrome).
func (f *Form) ChromeNode() core.Node { return f.Node() }

// SetLayout sets horizontal | vertical | inline.
func (f *Form) SetLayout(l FormLayout) {
	if f == nil {
		return
	}
	f.Layout = l
	f.rebuild()
}

// SetSize propagates size to bound controls.
func (f *Form) SetSize(s FormSize) {
	if f == nil {
		return
	}
	f.Size = s
	f.applyControlChrome()
	f.rebuild()
}

// SetVariant propagates variant to bound controls.
func (f *Form) SetVariant(v FormVariant) {
	if f == nil {
		return
	}
	f.Variant = v
	f.applyControlChrome()
	f.rebuild()
}

// SetDisabled disables bound controls.
func (f *Form) SetDisabled(d bool) {
	if f == nil {
		return
	}
	f.Disabled = d
	f.applyControlChrome()
}

// SetColon sets default colon for horizontal items.
func (f *Form) SetColon(v bool) {
	if f == nil {
		return
	}
	f.Colon = v
	f.rebuildItemsOnly()
}

// SetRequiredMark sets required mark mode.
func (f *Form) SetRequiredMark(m FormRequiredMark) {
	if f == nil {
		return
	}
	f.RequiredMark = m
	f.rebuildItemsOnly()
}

// SetInitialValues sets defaults applied on mount and ResetFields.
// Keys may be dotted paths ("user.name") or top-level nested maps.
func (f *Form) SetInitialValues(v map[string]any) {
	if f == nil {
		return
	}
	f.InitialValues = cloneAnyMap(v)
	if !f.inited {
		f.applyInitialValues()
		f.inited = true
		f.syncAllControls()
	}
}

// SetName sets form name (id prefix semantics).
func (f *Form) SetName(name string) {
	if f == nil {
		return
	}
	f.Name = name
}

// SetLabelAlign sets label alignment.
func (f *Form) SetLabelAlign(a FormLabelAlign) {
	if f == nil {
		return
	}
	f.LabelAlign = a
	f.rebuildItemsOnly()
}

// SetLabelWrap toggles label wrap (layout flag; P0 stores + rebuilds).
func (f *Form) SetLabelWrap(v bool) {
	if f == nil {
		return
	}
	f.LabelWrap = v
	f.rebuildItemsOnly()
}

// SetLabelCol sets default label col.
func (f *Form) SetLabelCol(c FormCol) {
	if f == nil {
		return
	}
	if c.Span <= 0 {
		c.Span = DefaultFormLabelColSpan
	}
	f.LabelCol = c
	f.rebuildItemsOnly()
}

// SetWrapperCol sets default wrapper col.
func (f *Form) SetWrapperCol(c FormCol) {
	if f == nil {
		return
	}
	if c.Span <= 0 {
		c.Span = DefaultFormWrapperColSpan
	}
	f.WrapperCol = c
	f.rebuildItemsOnly()
}

// SetValidateTrigger sets default validate trigger.
func (f *Form) SetValidateTrigger(t FormValidateTrigger) {
	if f == nil {
		return
	}
	f.ValidateTrigger = t
}

// SetItemGap sets spacing between items (0 → DefaultFormItemGap).
func (f *Form) SetItemGap(px float64) {
	if f == nil {
		return
	}
	f.ItemGap = px
	if f.Root != nil {
		f.Root.Gap = f.itemGap()
		f.Root.MarkNeedsLayout()
	}
}

// SetOnFinish sets success callback.
func (f *Form) SetOnFinish(fn func(map[string]any)) {
	if f != nil {
		f.OnFinish = fn
	}
}

// SetOnFinishFailed sets failure callback.
func (f *Form) SetOnFinishFailed(fn func(FormFailInfo)) {
	if f != nil {
		f.OnFinishFailed = fn
	}
}

// SetOnValuesChange sets values change callback.
func (f *Form) SetOnValuesChange(fn func(changed, all map[string]any)) {
	if f != nil {
		f.OnValuesChange = fn
	}
}

// SetTheme sets theme override.
func (f *Form) SetTheme(th *core.Theme) {
	if f == nil {
		return
	}
	f.Theme = th
	f.rebuild()
}

// SetFace sets label/error face.
func (f *Form) SetFace(face text.Face) {
	if f == nil {
		return
	}
	f.Face = face
	f.rebuildItemsOnly()
}

// SetAriaLabel sets accessible name on root.
func (f *Form) SetAriaLabel(s string) {
	if f == nil {
		return
	}
	f.AriaLabel = s
	if f.Root != nil {
		f.Root.Label = s
	}
}

// AddItem registers and mounts a FormItem.
func (f *Form) AddItem(item *FormItem) *FormItem {
	if f == nil || item == nil {
		return item
	}
	item.attach(f)
	f.items = append(f.items, item)
	if len(item.Name) > 0 {
		f.registerField(item)
		// seed control from store / initial
		item.pullValue()
	}
	f.rebuild()
	return item
}

// AddList registers a Form.List (array field metadata).
func (f *Form) AddList(list *FormList) *FormList {
	if f == nil || list == nil {
		return list
	}
	list.attach(f)
	f.lists = append(f.lists, list)
	if _, ok := f.values[list.nameKey()]; !ok {
		f.values[list.nameKey()] = []any{}
	}
	return list
}

// AddChild appends a non-field node (submit row, divider, …).
func (f *Form) AddChild(n core.Node) {
	if f == nil || n == nil {
		return
	}
	f.extras = append(f.extras, n)
	f.rebuild()
}

// Submit validates all fields and fires onFinish / onFinishFailed.
func (f *Form) Submit() bool {
	if f == nil {
		return false
	}
	vals, err := f.ValidateFields()
	if err != nil {
		return false
	}
	if f.OnFinish != nil {
		f.OnFinish(vals)
	}
	return true
}

// ValidateFields validates named fields (or all when empty).
// On failure calls OnFinishFailed when validating the full set (names empty).
func (f *Form) ValidateFields(names ...FormName) (map[string]any, error) {
	if f == nil {
		return nil, fmt.Errorf("nil form")
	}
	keys := f.order
	if len(names) > 0 {
		keys = make([]string, 0, len(names))
		for _, n := range names {
			if k := n.String(); k != "" {
				keys = append(keys, k)
			}
		}
	}
	var errs []FormFieldError
	ok := true
	for _, k := range keys {
		st := f.fields[k]
		if st == nil {
			continue
		}
		if !f.validateFieldState(st) {
			ok = false
			errs = append(errs, FormFieldError{Name: st.name.clone(), Errors: append([]string{}, st.errors...)})
		}
		f.syncItemErrors(st.name)
	}
	vals := f.GetFieldsValue()
	if !ok {
		if len(names) == 0 && f.OnFinishFailed != nil {
			f.OnFinishFailed(FormFailInfo{Values: vals, ErrorFields: errs})
		}
		return vals, fmt.Errorf("validation failed")
	}
	return vals, nil
}

// SetFieldsValue merges values into the store and updates bound controls.
func (f *Form) SetFieldsValue(values map[string]any) {
	if f == nil || values == nil {
		return
	}
	flat := flattenValues(values)
	changed := make(map[string]any)
	for k, v := range flat {
		f.writePath(ParseFormName(k), v, false)
		changed[k] = v
	}
	f.syncAllControls()
	f.fireValuesChange(changed)
}

// SetFieldValue sets one path.
func (f *Form) SetFieldValue(name FormName, value any) {
	if f == nil || len(name) == 0 {
		return
	}
	f.writePath(name, value, true)
	f.syncControl(name)
}

// GetFieldValue returns one path value.
func (f *Form) GetFieldValue(name FormName) any {
	if f == nil || len(name) == 0 {
		return nil
	}
	if st := f.fields[name.String()]; st != nil {
		return st.value
	}
	return getAtPath(f.values, name)
}

// GetFieldsValue returns a nested map of current values.
func (f *Form) GetFieldsValue() map[string]any {
	if f == nil {
		return nil
	}
	// rebuild nested from registered fields + list arrays
	out := make(map[string]any)
	for _, k := range f.order {
		st := f.fields[k]
		if st == nil {
			continue
		}
		setAtPath(out, st.name, st.value)
	}
	// include list roots even if empty
	for _, list := range f.lists {
		if list == nil {
			continue
		}
		key := list.nameKey()
		if _, ok := out[key]; !ok {
			if v, ok2 := f.values[key]; ok2 {
				out[key] = deepClone(v)
			} else {
				out[key] = []any{}
			}
		}
	}
	return out
}

// ResetFields resets named fields (or all) to initialValues and clears errors.
func (f *Form) ResetFields(names ...FormName) {
	if f == nil {
		return
	}
	initFlat := flattenValues(f.InitialValues)
	if len(names) == 0 {
		for _, st := range f.fields {
			key := st.name.String()
			if v, ok := initFlat[key]; ok {
				st.value = deepClone(v)
			} else {
				st.value = zeroFor(st.value)
			}
			st.errors = nil
			st.touched = false
			st.dirty = false
			setAtPath(f.values, st.name, st.value)
		}
		// reset lists length to initial
		for _, list := range f.lists {
			if list == nil {
				continue
			}
			list.resetFromInitial()
		}
	} else {
		for _, n := range names {
			st := f.fields[n.String()]
			if st == nil {
				continue
			}
			key := n.String()
			if v, ok := initFlat[key]; ok {
				st.value = deepClone(v)
			} else {
				st.value = zeroFor(st.value)
			}
			st.errors = nil
			st.touched = false
			st.dirty = false
			setAtPath(f.values, st.name, st.value)
		}
	}
	f.syncAllControls()
	for _, it := range f.items {
		if it != nil {
			it.SyncChrome()
		}
	}
}

// FieldErrors returns validation messages for a path.
func (f *Form) FieldErrors(name FormName) []string {
	if f == nil {
		return nil
	}
	st := f.fields[name.String()]
	if st == nil {
		return nil
	}
	return append([]string{}, st.errors...)
}

// FieldError returns the first error message (or "").
func (f *Form) FieldError(name FormName) string {
	errs := f.FieldErrors(name)
	if len(errs) == 0 {
		return ""
	}
	return errs[0]
}

// ---- internals ----

func (f *Form) theme() *core.Theme {
	var n core.Node
	if f.Root != nil {
		n = f.Root
	}
	return themeOf(f.Theme, n)
}

func (f *Form) itemGap() float64 {
	if f.ItemGap > 0 {
		return f.ItemGap
	}
	// Prefer paddingLG (24) as marginLG stand-in.
	th := f.theme()
	if th != nil {
		if g := th.SizeOr(core.TokenPaddingLG, DefaultFormItemGap); g > 0 {
			return g
		}
	}
	return DefaultFormItemGap
}

func (f *Form) registerField(item *FormItem) {
	if item == nil || len(item.Name) == 0 {
		return
	}
	key := item.Name.String()
	st, ok := f.fields[key]
	if !ok {
		st = &formFieldState{name: item.Name.clone()}
		f.fields[key] = st
		f.order = append(f.order, key)
		// seed from initial / existing nested
		if v := getAtPath(f.values, item.Name); v != nil {
			st.value = deepClone(v)
		} else if flat := flattenValues(f.InitialValues); flat != nil {
			if v, ok := flat[key]; ok {
				st.value = deepClone(v)
				setAtPath(f.values, item.Name, st.value)
			}
		}
	}
	st.rules = append([]FormRule{}, item.Rules...)
	st.required = item.isRequired()
	if item.initialSet {
		st.value = deepClone(item.InitialValue)
		setAtPath(f.values, item.Name, st.value)
	}
}

func (f *Form) applyInitialValues() {
	if f.InitialValues == nil {
		return
	}
	flat := flattenValues(f.InitialValues)
	for k, v := range flat {
		name := ParseFormName(k)
		setAtPath(f.values, name, deepClone(v))
		if st := f.fields[k]; st != nil {
			st.value = deepClone(v)
		}
	}
	// list roots
	for k, v := range f.InitialValues {
		if arr, ok := v.([]any); ok {
			f.values[k] = deepClone(arr)
		}
	}
}

func (f *Form) writePath(name FormName, value any, fire bool) {
	key := name.String()
	st := f.fields[key]
	if st == nil {
		st = &formFieldState{name: name.clone()}
		f.fields[key] = st
		f.order = append(f.order, key)
	}
	st.value = value
	st.dirty = true
	setAtPath(f.values, name, value)
	if fire {
		changed := map[string]any{key: value}
		f.fireValuesChange(changed)
		f.maybeValidateOnChange(name)
		f.revalidateDependencies(name)
	}
}

func (f *Form) fireValuesChange(changed map[string]any) {
	if f.OnValuesChange == nil {
		return
	}
	f.OnValuesChange(changed, f.GetFieldsValue())
}

func (f *Form) maybeValidateOnChange(name FormName) {
	item := f.findItem(name)
	trig := f.ValidateTrigger
	if item != nil && item.triggerSet {
		trig = item.ValidateTrigger
	}
	if trig == FormValidateOnChange {
		f.ValidateFields(name)
	}
}

func (f *Form) maybeValidateOnBlur(name FormName) {
	item := f.findItem(name)
	trig := f.ValidateTrigger
	if item != nil && item.triggerSet {
		trig = item.ValidateTrigger
	}
	if trig == FormValidateOnBlur {
		if st := f.fields[name.String()]; st != nil {
			st.touched = true
		}
		f.ValidateFields(name)
	}
}

func (f *Form) revalidateDependencies(changed FormName) {
	for _, it := range f.items {
		if it == nil || len(it.Name) == 0 {
			continue
		}
		for _, dep := range it.Dependencies {
			if dep.equal(changed) {
				f.ValidateFields(it.Name)
				break
			}
		}
	}
}

func (f *Form) validateFieldState(st *formFieldState) bool {
	if st == nil {
		return true
	}
	st.errors = nil
	val := st.value
	// merge rules from latest item
	if it := f.findItem(st.name); it != nil {
		st.rules = append([]FormRule{}, it.Rules...)
		st.required = it.isRequired()
	}
	empty := isEmptyValue(val)
	if st.required && empty {
		msg := DefaultFormRequiredMessage
		for _, r := range st.rules {
			if r.Required && r.Message != "" {
				msg = r.Message
				break
			}
		}
		st.errors = append(st.errors, msg)
	}
	for _, r := range st.rules {
		if r.Required {
			continue // already handled
		}
		if r.Validator != nil {
			if msg := r.Validator(val); msg != "" {
				st.errors = append(st.errors, msg)
			}
		}
	}
	// also run required validators that only set Validator
	for _, r := range st.rules {
		if r.Required && r.Validator != nil {
			if msg := r.Validator(val); msg != "" {
				st.errors = append(st.errors, msg)
			}
		}
	}
	return len(st.errors) == 0
}

func (f *Form) findItem(name FormName) *FormItem {
	for _, it := range f.items {
		if it != nil && it.Name.equal(name) {
			return it
		}
	}
	return nil
}

func (f *Form) syncItemErrors(name FormName) {
	if it := f.findItem(name); it != nil {
		it.SyncChrome()
	}
}

func (f *Form) syncControl(name FormName) {
	if it := f.findItem(name); it != nil {
		it.pullValue()
		it.SyncChrome()
	}
}

func (f *Form) syncAllControls() {
	for _, it := range f.items {
		if it != nil {
			it.pullValue()
			it.SyncChrome()
		}
	}
}

func (f *Form) applyControlChrome() {
	for _, it := range f.items {
		if it != nil {
			it.applyFormChrome()
		}
	}
}

func (f *Form) rebuildItemsOnly() {
	for _, it := range f.items {
		if it != nil {
			it.rebuild()
		}
	}
	// re-parent into root without recreating item roots unnecessarily
	f.rebuild()
}

func (f *Form) rebuild() {
	th := f.theme()
	_ = th
	var root *primitive.Flex
	if f.Layout == FormInline {
		root = primitive.Row()
		root.Gap = DefaultFormInlineItemGap
		root.CrossAlign = core.CrossStart
		root.MainAlign = core.MainStart
		root.Wrap = true
	} else {
		root = primitive.Column()
		root.Gap = f.itemGap()
		root.CrossAlign = core.CrossStretch
		root.MainAlign = core.MainStart
	}
	root.Label = f.AriaLabel
	for _, it := range f.items {
		if it == nil {
			continue
		}
		it.form = f
		it.rebuild()
		if it.Hidden {
			continue
		}
		root.AddChild(it.Node())
	}
	for _, n := range f.extras {
		if n != nil {
			root.AddChild(n)
		}
	}
	f.Root = root
	if !f.inited && f.InitialValues != nil {
		f.applyInitialValues()
		f.inited = true
		f.syncAllControls()
	}
}

// ---- FormItem ----

// FormItem is a labeled field row with optional error text.
type FormItem struct {
	Root *primitive.Flex

	Name            FormName
	Label           string
	Rules           []FormRule
	Required        bool
	requiredSet     bool
	Dependencies    []FormName
	ValidateTrigger FormValidateTrigger
	triggerSet      bool
	ValuePropName   string // "value" | "checked"
	Layout          FormLayout
	layoutSet       bool
	Colon           *bool
	Help            string
	Hidden          bool
	NoStyle         bool
	LabelCol        FormCol
	labelColSet     bool
	WrapperCol      FormCol
	wrapperColSet   bool
	InitialValue    any
	initialSet      bool
	FieldGap        float64 // 0 → DefaultFormFieldGap
	ErrorGap        float64 // 0 → DefaultFormErrorGap
	Face            text.Face
	Theme           *core.Theme

	form      *Form
	labelText *primitive.Text
	errorText *primitive.Text
	control   core.Node

	// bound controls (at most one primary)
	input    *Input
	checkbox *Checkbox
	sw       *Switch
	sel      *Select
}

// NewFormItem creates an empty FormItem (set name/label/control then AddItem).
func NewFormItem() *FormItem {
	return &FormItem{ValuePropName: "value"}
}

// NewFormItemName creates a FormItem with name and label.
func NewFormItemName(name, label string) *FormItem {
	fi := NewFormItem()
	if name != "" {
		fi.Name = ParseFormName(name)
	}
	fi.Label = label
	return fi
}

// Node returns the item root.
func (fi *FormItem) Node() core.Node {
	if fi == nil {
		return nil
	}
	if fi.Root == nil {
		fi.rebuild()
	}
	return fi.Root
}

// SetName sets a dotted or multi-segment path.
func (fi *FormItem) SetName(parts ...string) *FormItem {
	if fi == nil {
		return nil
	}
	fi.Name = Name(parts...)
	return fi
}

// SetNamePath sets FormName directly.
func (fi *FormItem) SetNamePath(n FormName) *FormItem {
	if fi == nil {
		return nil
	}
	fi.Name = n.clone()
	return fi
}

// SetLabel sets label text.
func (fi *FormItem) SetLabel(s string) *FormItem {
	if fi == nil {
		return nil
	}
	fi.Label = s
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// SetRules replaces validation rules.
func (fi *FormItem) SetRules(rules ...FormRule) *FormItem {
	if fi == nil {
		return nil
	}
	fi.Rules = append([]FormRule{}, rules...)
	if fi.form != nil && len(fi.Name) > 0 {
		fi.form.registerField(fi)
	}
	return fi
}

// SetRequired forces required mark / rule.
func (fi *FormItem) SetRequired(v bool) *FormItem {
	if fi == nil {
		return nil
	}
	fi.Required = v
	fi.requiredSet = true
	if fi.form != nil && len(fi.Name) > 0 {
		fi.form.registerField(fi)
	}
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// SetDependencies sets fields that re-trigger this item's validation.
func (fi *FormItem) SetDependencies(deps ...FormName) *FormItem {
	if fi == nil {
		return nil
	}
	fi.Dependencies = append([]FormName{}, deps...)
	return fi
}

// SetValidateTrigger overrides form-level trigger.
func (fi *FormItem) SetValidateTrigger(t FormValidateTrigger) *FormItem {
	if fi == nil {
		return nil
	}
	fi.ValidateTrigger = t
	fi.triggerSet = true
	return fi
}

// SetValuePropName sets "value" or "checked".
func (fi *FormItem) SetValuePropName(name string) *FormItem {
	if fi == nil {
		return nil
	}
	if name == "" {
		name = "value"
	}
	fi.ValuePropName = name
	return fi
}

// SetLayout overrides form layout for this item (horizontal|vertical).
func (fi *FormItem) SetLayout(l FormLayout) *FormItem {
	if fi == nil {
		return nil
	}
	if l == FormInline {
		l = FormVertical
	}
	fi.Layout = l
	fi.layoutSet = true
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// SetColon overrides colon for this item (nil = inherit).
func (fi *FormItem) SetColon(v *bool) *FormItem {
	if fi == nil {
		return nil
	}
	fi.Colon = v
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// SetHelp sets static help text (shown when no error).
func (fi *FormItem) SetHelp(s string) *FormItem {
	if fi == nil {
		return nil
	}
	fi.Help = s
	fi.SyncChrome()
	return fi
}

// SetHidden hides the item UI (value still collected when named).
func (fi *FormItem) SetHidden(v bool) *FormItem {
	if fi == nil {
		return nil
	}
	fi.Hidden = v
	if fi.form != nil {
		fi.form.rebuild()
	}
	return fi
}

// SetNoStyle skips label/error chrome (control only).
func (fi *FormItem) SetNoStyle(v bool) *FormItem {
	if fi == nil {
		return nil
	}
	fi.NoStyle = v
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// SetLabelCol overrides label col.
func (fi *FormItem) SetLabelCol(c FormCol) *FormItem {
	if fi == nil {
		return nil
	}
	fi.LabelCol = c
	fi.labelColSet = true
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// SetWrapperCol overrides wrapper col.
func (fi *FormItem) SetWrapperCol(c FormCol) *FormItem {
	if fi == nil {
		return nil
	}
	fi.WrapperCol = c
	fi.wrapperColSet = true
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// SetInitialValue sets item-level default (Form initialValues wins if both set).
func (fi *FormItem) SetInitialValue(v any) *FormItem {
	if fi == nil {
		return nil
	}
	fi.InitialValue = v
	fi.initialSet = true
	return fi
}

// SetControl sets an unbound display node.
func (fi *FormItem) SetControl(n core.Node) *FormItem {
	if fi == nil {
		return nil
	}
	fi.control = n
	fi.input, fi.checkbox, fi.sw, fi.sel = nil, nil, nil, nil
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// BindInput binds a kit.Input as the field control (value string).
func (fi *FormItem) BindInput(in *Input) *FormItem {
	if fi == nil || in == nil {
		return fi
	}
	fi.input = in
	fi.checkbox, fi.sw, fi.sel = nil, nil, nil
	fi.control = in.Node()
	if fi.form != nil {
		fi.wireInput(in)
		fi.applyFormChrome()
	}
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// BindCheckbox binds a Checkbox (valuePropName defaults to checked).
func (fi *FormItem) BindCheckbox(c *Checkbox) *FormItem {
	if fi == nil || c == nil {
		return fi
	}
	fi.checkbox = c
	fi.input, fi.sw, fi.sel = nil, nil, nil
	if fi.ValuePropName == "" || fi.ValuePropName == "value" {
		fi.ValuePropName = "checked"
	}
	fi.control = c.Node()
	if fi.form != nil {
		fi.wireCheckbox(c)
		fi.applyFormChrome()
	}
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// BindSwitch binds a Switch (valuePropName defaults to checked).
func (fi *FormItem) BindSwitch(s *Switch) *FormItem {
	if fi == nil || s == nil {
		return fi
	}
	fi.sw = s
	fi.input, fi.checkbox, fi.sel = nil, nil, nil
	if fi.ValuePropName == "" || fi.ValuePropName == "value" {
		fi.ValuePropName = "checked"
	}
	fi.control = s.Node()
	if fi.form != nil {
		fi.wireSwitch(s)
		fi.applyFormChrome()
	}
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// BindSelect binds a Select (value string).
func (fi *FormItem) BindSelect(s *Select) *FormItem {
	if fi == nil || s == nil {
		return fi
	}
	fi.sel = s
	fi.input, fi.checkbox, fi.sw = nil, nil, nil
	fi.control = s.Node()
	if fi.form != nil {
		fi.wireSelect(s)
		fi.applyFormChrome()
	}
	if fi.Root != nil {
		fi.rebuild()
	}
	return fi
}

// Error returns the first validation error.
func (fi *FormItem) Error() string {
	if fi == nil || fi.form == nil {
		return ""
	}
	return fi.form.FieldError(fi.Name)
}

// SyncChrome refreshes error text and control status.
func (fi *FormItem) SyncChrome() {
	if fi == nil {
		return
	}
	err := fi.Error()
	if fi.errorText != nil {
		if err != "" {
			fi.errorText.SetValue(err)
			if th := fi.theme(); th != nil {
				fi.errorText.Color = th.Color(core.TokenColorError)
			}
		} else if fi.Help != "" {
			fi.errorText.SetValue(fi.Help)
			if th := fi.theme(); th != nil {
				fi.errorText.Color = th.Color(core.TokenColorTextSecondary)
			}
		} else {
			fi.errorText.SetValue("")
		}
	}
	// status on input/select
	if fi.input != nil {
		if err != "" {
			fi.input.SetStatus(InputStatusError)
		} else {
			fi.input.SetStatus(InputStatusNone)
		}
	}
	if fi.sel != nil {
		if err != "" {
			fi.sel.SetStatus(InputStatusError)
		} else {
			fi.sel.SetStatus(InputStatusNone)
		}
	}
}

func (fi *FormItem) attach(f *Form) {
	fi.form = f
	if fi.Face == nil && f != nil {
		fi.Face = f.Face
	}
	if fi.Theme == nil && f != nil {
		fi.Theme = f.Theme
	}
	fi.applyFormChrome()
	// re-wire callbacks with form context
	if fi.input != nil {
		fi.wireInput(fi.input)
	}
	if fi.checkbox != nil {
		fi.wireCheckbox(fi.checkbox)
	}
	if fi.sw != nil {
		fi.wireSwitch(fi.sw)
	}
	if fi.sel != nil {
		fi.wireSelect(fi.sel)
	}
}

func (fi *FormItem) isRequired() bool {
	if fi.requiredSet {
		return fi.Required
	}
	for _, r := range fi.Rules {
		if r.Required {
			return true
		}
	}
	return fi.Required
}

func (fi *FormItem) effectiveLayout() FormLayout {
	if fi.layoutSet {
		return fi.Layout
	}
	if fi.form != nil {
		if fi.form.Layout == FormInline {
			return FormVertical // inline form: items stack label/control tightly vertical-ish; use compact vertical
		}
		return fi.form.Layout
	}
	return FormHorizontal
}

func (fi *FormItem) effectiveColon() bool {
	if fi.Colon != nil {
		return *fi.Colon
	}
	if fi.form != nil {
		return fi.form.Colon
	}
	return true
}

func (fi *FormItem) effectiveLabelCol() FormCol {
	if fi.labelColSet {
		return fi.LabelCol
	}
	if fi.form != nil {
		return fi.form.LabelCol
	}
	return FormCol{Span: DefaultFormLabelColSpan}
}

func (fi *FormItem) effectiveWrapperCol() FormCol {
	if fi.wrapperColSet {
		return fi.WrapperCol
	}
	if fi.form != nil {
		return fi.form.WrapperCol
	}
	return FormCol{Span: DefaultFormWrapperColSpan}
}

func (fi *FormItem) theme() *core.Theme {
	var n core.Node
	if fi.Root != nil {
		n = fi.Root
	}
	return themeOf(fi.Theme, n)
}

func (fi *FormItem) fieldGap() float64 {
	if fi.FieldGap > 0 {
		return fi.FieldGap
	}
	th := fi.theme()
	if th != nil {
		if g := th.SizeOr(core.TokenPaddingSM, DefaultFormFieldGap); g > 0 {
			return g
		}
	}
	return DefaultFormFieldGap
}

func (fi *FormItem) errorGap() float64 {
	if fi.ErrorGap > 0 {
		return fi.ErrorGap
	}
	return DefaultFormErrorGap
}

func (fi *FormItem) labelString() string {
	lab := fi.Label
	req := fi.isRequired()
	mark := FormRequiredMarkDefault
	if fi.form != nil {
		mark = fi.form.RequiredMark
	}
	switch mark {
	case FormRequiredMarkHidden:
		// no mark
	case FormRequiredMarkOptional:
		if !req && lab != "" {
			lab = lab + " (" + DefaultFormOptionalText + ")"
		}
		if req {
			lab = DefaultFormRequiredMarkChar + " " + lab
		}
	default: // Default
		if req && lab != "" {
			lab = DefaultFormRequiredMarkChar + " " + lab
		}
	}
	// colon for horizontal
	if lab != "" && fi.effectiveLayout() == FormHorizontal && fi.effectiveColon() {
		lab = lab + ":"
	}
	return lab
}

func (fi *FormItem) rebuild() {
	th := fi.theme()
	if fi.NoStyle {
		if fi.control == nil {
			fi.control = primitive.NewBox()
		}
		col := primitive.Column(fi.control)
		col.CrossAlign = core.CrossStart
		fi.Root = col
		fi.labelText, fi.errorText = nil, nil
		return
	}

	lab := fi.labelString()
	fi.labelText = primitive.NewText(lab)
	fi.labelText.FontSize = th.SizeOr(core.TokenFontSize, 14)
	fi.labelText.Face = fi.Face
	fi.labelText.Color = th.Color(core.TokenColorText)
	// required * is part of string; color whole label text — mark color approximated via error only on *
	// (full rich text mark is P1)

	fi.errorText = primitive.NewText("")
	fi.errorText.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
	fi.errorText.Face = fi.Face
	fi.errorText.Color = th.Color(core.TokenColorError)

	if fi.control == nil {
		fi.control = primitive.NewBox()
	}

	ctrlCol := primitive.Column(fi.control, fi.errorText)
	ctrlCol.Gap = fi.errorGap()
	ctrlCol.CrossAlign = core.CrossStart

	layout := fi.effectiveLayout()
	fg := fi.fieldGap()
	if layout == FormHorizontal {
		lc := fi.effectiveLabelCol()
		wc := fi.effectiveWrapperCol()
		ls, ws := float64(lc.Span), float64(wc.Span)
		if ls <= 0 {
			ls = DefaultFormLabelColSpan
		}
		if ws <= 0 {
			ws = DefaultFormWrapperColSpan
		}
		labelHost := primitive.Row(fi.labelText)
		labelHost.CrossAlign = core.CrossCenter
		if fi.form != nil && fi.form.LabelAlign == FormLabelRight {
			labelHost.MainAlign = core.MainEnd
		} else {
			labelHost.MainAlign = core.MainStart
		}
		// Box height ≈ controlHeight so label aligns with middle-size controls.
		labelWrap := primitive.NewBox(labelHost)
		labelWrap.Height = th.SizeOr(core.TokenControlHeight, 32)
		row := primitive.Row(
			primitive.NewFlexible(ls, labelWrap),
			primitive.NewFlexible(ws, ctrlCol),
		)
		row.Gap = DefaultFormColonMarginEnd
		row.CrossAlign = core.CrossStart
		fi.Root = row
	} else {
		// vertical
		col := primitive.Column(fi.labelText, ctrlCol)
		col.Gap = fg
		col.CrossAlign = core.CrossStart
		fi.Root = col
	}
	// a11y: expose label on control when possible
	if fi.input != nil && fi.Label != "" && fi.input.AriaLabel == "" {
		fi.input.SetAriaLabel(fi.Label)
	}
	if fi.sel != nil && fi.Label != "" && fi.sel.Title == "" {
		fi.sel.SetTitle(fi.Label)
	}
	fi.SyncChrome()
}

func (fi *FormItem) pullValue() {
	if fi.form == nil || len(fi.Name) == 0 {
		return
	}
	v := fi.form.GetFieldValue(fi.Name)
	if fi.input != nil {
		fi.input.SetValue(anyToString(v))
	}
	if fi.checkbox != nil {
		fi.checkbox.SetChecked(anyToBool(v))
	}
	if fi.sw != nil {
		fi.sw.SetChecked(anyToBool(v))
	}
	if fi.sel != nil {
		fi.sel.SetValue(anyToString(v))
	}
}

func (fi *FormItem) applyFormChrome() {
	if fi.form == nil {
		return
	}
	dis := fi.form.Disabled
	if fi.input != nil {
		fi.input.SetDisabled(dis)
		fi.input.SetSize(formSizeToInput(fi.form.Size))
		fi.input.SetVariant(formVariantToInput(fi.form.Variant))
		if fi.Face != nil {
			fi.input.SetFace(fi.Face)
		} else if fi.form.Face != nil {
			fi.input.SetFace(fi.form.Face)
		}
	}
	if fi.checkbox != nil {
		fi.checkbox.SetDisabled(dis)
	}
	if fi.sw != nil {
		fi.sw.SetDisabled(dis)
		// Switch size only medium/small
		if fi.form.Size == FormSmall {
			fi.sw.SetSize(SwitchSmall)
		} else {
			fi.sw.SetSize(SwitchMedium)
		}
	}
	if fi.sel != nil {
		fi.sel.SetDisabled(dis)
		fi.sel.SetSize(formSizeToInput(fi.form.Size))
		fi.sel.SetVariant(formVariantToInput(fi.form.Variant))
	}
}

func (fi *FormItem) pushValue(v any) {
	if fi.form == nil || len(fi.Name) == 0 {
		return
	}
	fi.form.writePath(fi.Name, v, true)
	fi.SyncChrome()
}

func (fi *FormItem) wireInput(in *Input) {
	prev := in.OnChange
	in.SetOnChange(func(s string) {
		fi.pushValue(s)
		if prev != nil {
			prev(s)
		}
	})
	prevFocus := in.OnFocusChange
	in.OnFocusChange = func(focused bool) {
		if prevFocus != nil {
			prevFocus(focused)
		}
		if !focused && fi.form != nil {
			fi.form.maybeValidateOnBlur(fi.Name)
		}
	}
}

func (fi *FormItem) wireCheckbox(c *Checkbox) {
	prev := c.OnChange
	c.SetOnChange(func(checked bool) {
		// parent form owns value; still update display when uncontrolled checkbox
		if !c.Controlled {
			c.SetChecked(checked)
		}
		fi.pushValue(checked)
		if prev != nil {
			prev(checked)
		}
	})
}

func (fi *FormItem) wireSwitch(s *Switch) {
	prev := s.OnChange
	s.SetOnChange(func(checked bool) {
		if !s.Controlled {
			s.SetChecked(checked)
		}
		fi.pushValue(checked)
		if prev != nil {
			prev(checked)
		}
	})
}

func (fi *FormItem) wireSelect(s *Select) {
	prev := s.OnChange
	s.SetOnChange(func(v string) {
		if !s.Controlled {
			s.SetValue(v)
		}
		fi.pushValue(v)
		if prev != nil {
			prev(v)
		}
	})
}

// ---- FormList ----

// FormList is antd Form.List: array field with add/remove.
type FormList struct {
	name  string
	form  *Form
	count int
	Root  *primitive.Flex // optional visual host; rows usually separate Items
}

// NewFormList creates a list bound to array field name.
func NewFormList(name string) *FormList {
	return &FormList{name: name}
}

func (l *FormList) attach(f *Form) {
	l.form = f
	if l.Root == nil {
		l.Root = primitive.Column()
		l.Root.Gap = 8
		l.Root.CrossAlign = core.CrossStart
	}
	// seed count from values
	if f != nil {
		if arr, ok := f.values[l.name].([]any); ok {
			l.count = len(arr)
		} else if flat := flattenValues(f.InitialValues); flat != nil {
			// count max index from initial "name.i.field"
			max := -1
			prefix := l.name + "."
			for k := range flat {
				if strings.HasPrefix(k, prefix) {
					rest := strings.TrimPrefix(k, prefix)
					parts := strings.SplitN(rest, ".", 2)
					if len(parts) > 0 {
						if i, err := strconv.Atoi(parts[0]); err == nil && i > max {
							max = i
						}
					}
				}
			}
			if max >= 0 {
				l.count = max + 1
				arr := make([]any, l.count)
				for i := 0; i < l.count; i++ {
					arr[i] = map[string]any{}
				}
				f.values[l.name] = arr
			}
		}
		if _, ok := f.values[l.name]; !ok {
			f.values[l.name] = []any{}
		}
	}
}

func (l *FormList) nameKey() string { return l.name }

// Node returns an empty column host (optional).
func (l *FormList) Node() core.Node {
	if l == nil {
		return nil
	}
	if l.Root == nil {
		l.Root = primitive.Column()
	}
	return l.Root
}

// Len returns row count.
func (l *FormList) Len() int {
	if l == nil {
		return 0
	}
	return l.count
}

// Add appends one empty row; returns new index.
func (l *FormList) Add() int {
	if l == nil {
		return -1
	}
	idx := l.count
	l.count++
	if l.form != nil {
		arr, _ := l.form.values[l.name].([]any)
		if arr == nil {
			arr = []any{}
		}
		arr = append(arr, map[string]any{})
		l.form.values[l.name] = arr
	}
	return idx
}

// Remove deletes row at index and reindexes field paths.
func (l *FormList) Remove(index int) {
	if l == nil || index < 0 || index >= l.count {
		return
	}
	l.count--
	if l.form == nil {
		return
	}
	// rebuild array + field map for this list
	arr, _ := l.form.values[l.name].([]any)
	if index < len(arr) {
		arr = append(arr[:index], arr[index+1:]...)
		l.form.values[l.name] = arr
	}
	// reindex registered fields name.i.*
	prefix := l.name + "."
	type mv struct {
		oldKey string
		st     *formFieldState
		newIdx int
		field  string
	}
	var moves []mv
	for k, st := range l.form.fields {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		parts := strings.SplitN(rest, ".", 2)
		if len(parts) < 2 {
			continue
		}
		i, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if i == index {
			delete(l.form.fields, k)
			continue
		}
		if i > index {
			moves = append(moves, mv{oldKey: k, st: st, newIdx: i - 1, field: parts[1]})
		}
	}
	for _, m := range moves {
		delete(l.form.fields, m.oldKey)
	}
	for _, m := range moves {
		newName := Name(l.name, strconv.Itoa(m.newIdx), m.field)
		m.st.name = newName
		l.form.fields[newName.String()] = m.st
	}
	// rebuild order
	l.form.order = l.form.order[:0]
	for k := range l.form.fields {
		l.form.order = append(l.form.order, k)
	}
	// drop items pointing at removed index; reindex others
	var kept []*FormItem
	for _, it := range l.form.items {
		if it == nil || len(it.Name) < 2 || it.Name[0] != l.name {
			kept = append(kept, it)
			continue
		}
		i, err := strconv.Atoi(it.Name[1])
		if err != nil {
			kept = append(kept, it)
			continue
		}
		if i == index {
			continue
		}
		if i > index {
			field := ""
			if len(it.Name) > 2 {
				field = strings.Join(it.Name[2:], ".")
			}
			it.Name = Name(l.name, strconv.Itoa(i-1), field)
		}
		kept = append(kept, it)
	}
	l.form.items = kept
	l.form.rebuild()
}

// NameAt returns FormName for list[index].field.
func (l *FormList) NameAt(index int, field string) FormName {
	if l == nil {
		return nil
	}
	return Name(l.name, strconv.Itoa(index), field)
}

func (l *FormList) resetFromInitial() {
	if l == nil || l.form == nil {
		return
	}
	if v, ok := l.form.InitialValues[l.name]; ok {
		if arr, ok2 := v.([]any); ok2 {
			l.count = len(arr)
			l.form.values[l.name] = deepClone(arr)
			return
		}
	}
	l.count = 0
	l.form.values[l.name] = []any{}
}

// ---- helpers ----

func formSizeToInput(s FormSize) InputSize {
	switch s {
	case FormSmall:
		return InputSmall
	case FormLarge:
		return InputLarge
	default:
		return InputMiddle
	}
}

func formVariantToInput(v FormVariant) InputVariant {
	switch v {
	case FormFilled:
		return InputFilled
	case FormBorderless:
		return InputBorderless
	case FormUnderlined:
		return InputUnderlined
	default:
		return InputOutlined
	}
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return t == ""
	case bool:
		return false // false is a value
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return false
	}
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func anyToBool(v any) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	default:
		return false
	}
}

func zeroFor(v any) any {
	switch v.(type) {
	case bool:
		return false
	case string:
		return ""
	default:
		return nil
	}
}

func deepClone(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return cloneAnyMap(t)
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = deepClone(t[i])
		}
		return out
	default:
		return v
	}
}

func cloneAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepClone(v)
	}
	return out
}

func flattenValues(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any)
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch t := v.(type) {
		case map[string]any:
			if len(t) == 0 && prefix != "" {
				out[prefix] = map[string]any{}
				return
			}
			for k, child := range t {
				p := k
				if prefix != "" {
					p = prefix + "." + k
				}
				walk(p, child)
			}
		case []any:
			if len(t) == 0 && prefix != "" {
				out[prefix] = []any{}
				return
			}
			for i, child := range t {
				p := strconv.Itoa(i)
				if prefix != "" {
					p = prefix + "." + p
				}
				walk(p, child)
			}
		default:
			if prefix != "" {
				out[prefix] = v
			}
		}
	}
	// support already-flat dotted keys mixed with nested
	for k, v := range m {
		if strings.Contains(k, ".") {
			// if value is leaf, keep dotted key
			if _, isMap := v.(map[string]any); !isMap {
				if _, isArr := v.([]any); !isArr {
					out[k] = v
					continue
				}
			}
		}
		walk(k, v)
	}
	return out
}

func getAtPath(root map[string]any, path FormName) any {
	if root == nil || len(path) == 0 {
		return nil
	}
	var cur any = root
	for _, p := range path {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[p]
		case []any:
			i, err := strconv.Atoi(p)
			if err != nil || i < 0 || i >= len(node) {
				return nil
			}
			cur = node[i]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

func setAtPath(root map[string]any, path FormName, value any) {
	if root == nil || len(path) == 0 {
		return
	}
	cur := root
	for i := 0; i < len(path)-1; i++ {
		p := path[i]
		next := path[i+1]
		_, nextIsIdx := atoiOK(next)
		if nextIsIdx {
			// ensure array
			arr, _ := cur[p].([]any)
			if arr == nil {
				arr = []any{}
			}
			idx, _ := strconv.Atoi(next)
			for len(arr) <= idx {
				arr = append(arr, map[string]any{})
			}
			cur[p] = arr
			// if this is last-1 and path ends next, handled below differently
			if i == len(path)-2 {
				// parent of leaf is array element
				if len(path) == i+2 {
					// leaf is array index only
					arr[idx] = value
					cur[p] = arr
					return
				}
			}
			// move into element map
			el, ok := arr[idx].(map[string]any)
			if !ok || el == nil {
				el = map[string]any{}
				arr[idx] = el
				cur[p] = arr
			}
			// continue with remaining path after index
			// handle remaining from i+2
			if i+2 >= len(path) {
				arr[idx] = value
				cur[p] = arr
				return
			}
			setAtPath(el, path[i+2:], value)
			return
		}
		// map branch
		child, _ := cur[p].(map[string]any)
		if child == nil {
			child = map[string]any{}
			cur[p] = child
		}
		cur = child
	}
	leaf := path[len(path)-1]
	if idx, ok := atoiOK(leaf); ok {
		// shouldn't usually hit; array handled above
		_ = idx
	}
	cur[leaf] = value
}

func atoiOK(s string) (int, bool) {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return i, true
}
