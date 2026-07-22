package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// FormItem is a labeled field row with optional error text.
type FormItem struct {
	Root      *primitive.Flex
	label     *primitive.Text
	control   core.Node
	errorText *primitive.Text
	Name      string
	Label     string
	Required  bool
	Model     *core.FormModel
	Face      text.Face
	Theme     *core.Theme
}

// NewFormItem creates a form item with a control node.
func NewFormItem(name, label string, control core.Node) *FormItem {
	fi := &FormItem{Name: name, Label: label, control: control}
	fi.rebuild()
	return fi
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
	if fi.Theme != nil {
		return fi.Theme
	}
	return DefaultTheme()
}

func (fi *FormItem) rebuild() {
	th := fi.theme()
	lab := fi.Label
	if fi.Required {
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
	// Ant Form vertical: label above control (gap 8), error under (gap 4).
	field := primitive.Column(fi.label, fi.control)
	field.Gap = 8
	field.CrossAlign = core.CrossStart
	col := primitive.Column(field, fi.errorText)
	col.Gap = 4
	col.CrossAlign = core.CrossStart
	fi.Root = col
}

// Form hosts FormItems and a submit button bound to FormModel.
type Form struct {
	Root     *primitive.Flex
	Model    *core.FormModel
	Items    []*FormItem
	Submit   *Button
	Face     text.Face
	Theme    *core.Theme
	OnFinish func(values map[string]string)
}

// NewForm creates a form with the given model (or a new one).
func NewForm(model *core.FormModel) *Form {
	if model == nil {
		model = core.NewFormModel()
	}
	f := &Form{Model: model}
	f.rebuild()
	return f
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

func findInput(n core.Node) (*Input, bool) {
	// control is the root of Input when we stored in.Node()
	// We can't type-assert Input from node easily; callers pass Input.Node().
	// Bind via name externally if needed.
	return nil, false
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
	item.Model = f.Model
	item.Face = f.Face
	item.Theme = f.Theme
	item.rebuild()
	f.Items = append(f.Items, item)
	f.rebuild()
	return item
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
	if f.Theme != nil {
		return f.Theme
	}
	return DefaultTheme()
}

func (f *Form) rebuild() {
	th := f.theme()
	f.Root = primitive.Column()
	f.Root.Gap = 24 // Ant Form vertical layout margin
	f.Root.CrossAlign = core.CrossStart
	for _, it := range f.Items {
		if it.Root == nil {
			it.rebuild()
		}
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
