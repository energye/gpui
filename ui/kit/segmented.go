package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Segmented is Ant Design Segmented control.
// https://ant.design/components/segmented
// Outer Root stays stable; option row is rebuilt in place.
type Segmented struct {
	Root     *primitive.Decorated
	Options  []string
	Value    string
	Face     text.Face
	Theme    *core.Theme
	OnChange func(v string)
}

// NewSegmented creates a segmented control.
func NewSegmented(options ...string) *Segmented {
	s := &Segmented{Options: append([]string(nil), options...)}
	if len(options) > 0 {
		s.Value = options[0]
	}
	s.rebuild()
	return s
}

// Node returns root.
func (s *Segmented) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// SetValue selects an option.
func (s *Segmented) SetValue(v string) {
	s.Value = v
	s.rebuild()
	if s.OnChange != nil {
		s.OnChange(v)
	}
}

// SetFace sets font.
func (s *Segmented) SetFace(face text.Face) {
	s.Face = face
	s.rebuild()
}

func (s *Segmented) rebuild() {
	th := DefaultTheme()
	if s.Theme != nil {
		th = s.Theme
	}
	row := primitive.Row()
	row.Gap = 2
	row.CrossAlign = core.CrossCenter
	for _, opt := range s.Options {
		opt := opt
		lab := primitive.NewText(opt)
		lab.FontSize = 14
		lab.Face = s.Face
		lab.Color = th.Color(core.TokenColorText)
		host := primitive.NewDecorated(lab)
		host.Padding = primitive.Symmetric(11, 4)
		host.Radius = 4
		if opt == s.Value {
			host.Background = th.Color(core.TokenColorBgContainer)
			if host.Background.A < 0.5 {
				host.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
			}
		}
		p := primitive.NewPressable(host)
		p.ShowFocusRing = false
		p.Click = func() { s.SetValue(opt) }
		row.AddChild(p)
	}
	if s.Root == nil {
		s.Root = primitive.NewDecorated(row)
	} else {
		s.Root.ClearChildren()
		s.Root.AddChild(row)
	}
	s.Root.Padding = primitive.All(2)
	s.Root.Radius = 6
	s.Root.Background = th.Color(core.TokenColorBgLayout)
	if s.Root.Background.A < 0.5 {
		s.Root.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
	}
	s.Root.Hit = core.HitBlock
	s.Root.MarkNeedsLayout()
	s.Root.MarkNeedsPaint()
}
