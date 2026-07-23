package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Form vertical density defaults.
const (
	DefaultFormItemGap  = 24.0 // between form items
	DefaultFormFieldGap = 8.0  // label ↔ control (horizontal/vertical)
	DefaultFormErrorGap = 4.0  // control ↔ error text
)

// FormItem is a labeled field row with optional error text.
type FormItem struct {
	Root         *primitive.Flex
	label        *primitive.Text
	control      core.Node
	errorText    *primitive.Text
	Name         string
	Label        string
	Required     bool
	Layout       string // "vertical" | "horizontal"
	RequiredMark bool   // when true and Required, append " *"
	// FieldGap label↔control (0 → DefaultFormFieldGap).
	FieldGap float64
	// ErrorGap control↔error (0 → DefaultFormErrorGap).
	ErrorGap float64
	Model    *core.FormModel
	Face     text.Face
	Theme    *core.Theme
}

// NewFormItem creates a form item with a control node.
func NewFormItem(name, label string, control core.Node) *FormItem {
	fi := &FormItem{Name: name, Label: label, control: control, Layout: "vertical", RequiredMark: true}
	fi.rebuild()
	return fi
}

// Form hosts FormItems and a submit button bound to FormModel.
type Form struct {
	// ItemGap between items (0 → DefaultFormItemGap).
	ItemGap      float64
	Root         *primitive.Flex
	Model        *core.FormModel
	Items        []*FormItem
	Submit       *Button
	Layout       string // "vertical" | "horizontal"
	RequiredMark bool
	Face         text.Face
	Theme        *core.Theme
	OnFinish     func(values map[string]string)
}

// NewForm creates a form with the given model (or a new one).
func NewForm(model *core.FormModel) *Form {
	if model == nil {
		model = core.NewFormModel()
	}
	f := &Form{Model: model, Layout: "vertical", RequiredMark: true}
	f.rebuild()
	return f
}

// SetLayout sets "vertical" or "horizontal" form layout and rebuilds.
func (f *Form) SetLayout(layout string) {
	if layout != "horizontal" {
		layout = "vertical"
	}
	f.Layout = layout
	for _, it := range f.Items {
		it.Layout = layout
		it.RequiredMark = f.RequiredMark
		it.rebuild()
	}
	f.rebuild()
}

// Node returns the root.
func (fi *FormItem) Node() core.Node {
	if fi.Root == nil {
		fi.rebuild()
	}
	return fi.Root
}

// SyncError updates error chrome from the model.
func (fi *FormItem) SyncError() {
	if fi.Model == nil || fi.errorText == nil {
		return
	}
	st := fi.Model.Field(fi.Name)
	if st == nil || len(st.Errors) == 0 {
		fi.errorText.SetValue("")
		return
	}
	fi.errorText.SetValue(st.Errors[0])
	fi.errorText.Color = fi.theme().Color(core.TokenColorError)
}

func (fi *FormItem) theme() *core.Theme {
	var n core.Node
	if fi.Root != nil {
		n = fi.Root
	}
	return themeOf(fi.Theme, n)
}

func (fi *FormItem) rebuild() {
	th := fi.theme()
	lab := fi.Label
	if fi.Required && fi.RequiredMark {
		lab = lab + " *"
	}
	fi.label = primitive.NewText(lab)
	fi.label.FontSize = th.SizeOr(core.TokenFontSize, 14)
	fi.label.Face = fi.Face
	fi.label.Color = th.Color(core.TokenColorText)

	fi.errorText = primitive.NewText("")
	fi.errorText.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
	fi.errorText.Face = fi.Face
	fi.errorText.Color = th.Color(core.TokenColorError)

	if fi.control == nil {
		fi.control = primitive.NewBox()
	}
	fg := DefaultFormFieldGap
	if fi.FieldGap > 0 {
		fg = fi.FieldGap
	}
	var field *primitive.Flex
	if fi.Layout == "horizontal" {
		field = primitive.Row(fi.label, fi.control)
		field.Gap = fg
		field.CrossAlign = core.CrossCenter
	} else {
		// Ant Form vertical: label above control, error under.
		field = primitive.Column(fi.label, fi.control)
		field.Gap = fg
		field.CrossAlign = core.CrossStart
	}
	eg := DefaultFormErrorGap
	if fi.ErrorGap > 0 {
		eg = fi.ErrorGap
	}
	col := primitive.Column(field, fi.errorText)
	col.Gap = eg
	col.CrossAlign = core.CrossStart
	fi.Root = col
}

// Node returns the root.
func (f *Form) Node() core.Node {
	if f.Root == nil {
		f.rebuild()
	}
	return f.Root
}

// AddItem appends a form item and registers the field.
func (f *Form) AddItem(item *FormItem) {
	if item == nil {
		return
	}
	item.Model = f.Model
	item.Theme = f.Theme
	item.Face = f.Face
	item.Layout = f.Layout
	item.RequiredMark = f.RequiredMark
	item.rebuild()
	f.Model.Register(item.Name, item.Required)
	// wire input if kit.Input
	if in, ok := findInput(item.control); ok {
		in.SetOnChange(func(v string) {
			f.Model.SetValue(item.Name, v)
			item.SyncError()
		})
		// seed
		if v := f.Model.Value(item.Name); v != "" {
			in.SetValue(v)
		}
	}
	f.Items = append(f.Items, item)
	// rebuild list body without submit
	f.rebuild()
}

// BindInput links a kit.Input to a field name.
func (f *Form) BindInput(name string, in *Input, required bool, label string) *FormItem {
	if in == nil {
		return nil
	}
	f.Model.Register(name, required)
	in.SetOnChange(func(v string) {
		f.Model.SetValue(name, v)
		for _, it := range f.Items {
			if it.Name == name {
				it.SyncError()
			}
		}
	})
	item := NewFormItem(name, label, in.Node())
	item.Required = required
	item.RequiredMark = f.RequiredMark
	item.Layout = f.Layout
	item.Model = f.Model
	item.Face = f.Face
	item.Theme = f.Theme
	item.rebuild()
	f.Items = append(f.Items, item)
	f.rebuild()
	return item
}

// SetItemGap sets vertical spacing between items (0 → DefaultFormItemGap).
func (f *Form) SetItemGap(px float64) {
	if f == nil {
		return
	}
	f.ItemGap = px
	if f.Root != nil {
		gap := DefaultFormItemGap
		if px > 0 {
			gap = px
		}
		f.Root.Gap = gap
		f.Root.MarkNeedsLayout()
	}
}

// Validate runs ValidateAll and syncs error texts.
func (f *Form) Validate() bool {
	ok := f.Model.ValidateAll()
	for _, it := range f.Items {
		it.SyncError()
	}
	return ok
}

func (f *Form) theme() *core.Theme {
	var n core.Node
	if f.Root != nil {
		n = f.Root
	}
	return themeOf(f.Theme, n)
}

func (f *Form) rebuild() {
	th := f.theme()
	f.Root = primitive.Column()
	gap := DefaultFormItemGap
	if f.ItemGap > 0 {
		gap = f.ItemGap
	}
	f.Root.Gap = gap
	f.Root.CrossAlign = core.CrossStart
	for _, it := range f.Items {
		it.Layout = f.Layout
		it.RequiredMark = f.RequiredMark
		it.rebuild()
		f.Root.AddChild(it.Node())
	}
	if f.Submit == nil {
		f.Submit = NewButton("Submit")
		f.Submit.SetType(ButtonPrimary)
		f.Submit.SetFace(f.Face)
		f.Submit.Theme = th
		f.Submit.SetOnClick(func() {
			f.Model.OnFinish = f.OnFinish
			f.Validate()
		})
	}
	f.Root.AddChild(f.Submit.Node())
}
