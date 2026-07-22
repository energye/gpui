package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// AutoComplete is Input + suggestion list (simplified Ant AutoComplete).
// https://ant.design/components/auto-complete
type AutoComplete struct {
	Root     *primitive.Flex
	input    *Input
	list     *primitive.Flex
	Options  []string
	Value    string
	Face     text.Face
	Theme    *core.Theme
	OnChange func(v string)
	OnSelect func(v string)
}

// NewAutoComplete creates autocomplete with options.
func NewAutoComplete(placeholder string, options ...string) *AutoComplete {
	a := &AutoComplete{Options: append([]string(nil), options...)}
	a.input = NewInput(placeholder)
	a.rebuild()
	return a
}

// Node returns root.
func (a *AutoComplete) Node() core.Node {
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// SetFace sets font.
func (a *AutoComplete) SetFace(face text.Face) {
	a.Face = face
	if a.input != nil {
		a.input.SetFace(face)
	}
	a.rebuildList()
}

// SetValue sets input value.
func (a *AutoComplete) SetValue(v string) {
	a.Value = v
	if a.input != nil {
		a.input.SetValue(v)
	}
}

func (a *AutoComplete) rebuild() {
	a.input.SetFace(a.Face)
	a.input.SetOnChange(func(v string) {
		a.Value = v
		if a.OnChange != nil {
			a.OnChange(v)
		}
		a.rebuildList()
	})
	a.list = primitive.Column()
	a.list.Gap = 2
	a.rebuildList()
	panel := primitive.NewDecorated(a.list)
	panel.Padding = primitive.All(4)
	panel.BorderWidth = 1
	th := DefaultTheme()
	if a.Theme != nil {
		th = a.Theme
	}
	panel.BorderColor = th.Color(core.TokenColorBorder)
	panel.Background = th.Color(core.TokenColorBgContainer)
	a.Root = primitive.Column(a.input.Node(), panel)
	a.Root.Gap = 4
	a.Root.CrossAlign = core.CrossStart
}

func (a *AutoComplete) rebuildList() {
	if a.list == nil {
		return
	}
	a.list.ClearChildren()
	th := DefaultTheme()
	if a.Theme != nil {
		th = a.Theme
	}
	q := a.Value
	for _, opt := range a.Options {
		if q != "" && !containsFold(opt, q) {
			continue
		}
		opt := opt
		lab := primitive.NewText(opt)
		lab.FontSize = 14
		lab.Face = a.Face
		lab.Color = th.Color(core.TokenColorText)
		p := primitive.NewPressable(lab)
		p.Padding = primitive.Symmetric(8, 4)
		p.ShowFocusRing = false
		p.Click = func() {
			a.SetValue(opt)
			if a.OnSelect != nil {
				a.OnSelect(opt)
			}
		}
		a.list.AddChild(p)
	}
}
