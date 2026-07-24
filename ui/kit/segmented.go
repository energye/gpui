package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Segmented is Ant Design Segmented control.
// https://ant.design/components/segmented
//
// Outer Root stays stable. Option structure is rebuilt only when Options/Face/Theme
// change; SetValue only recolors chips (must not ClearChildren during pointer Click —
// that collapsed sibling Flex layouts until the next window resize).
type Segmented struct {
	Root     *primitive.Decorated
	Options  []string
	Value    string
	Face     text.Face
	Theme    *core.Theme
	OnChange func(v string)

	row   *primitive.Flex
	chips []*primitive.Decorated // per-option chrome (background)
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

// SetValue selects an option without tearing down the option tree.
func (s *Segmented) SetValue(v string) {
	if s == nil {
		return
	}
	prev := s.Value
	s.Value = v
	if s.row == nil || len(s.chips) != len(s.Options) {
		s.rebuild()
	} else {
		s.applySelection()
	}
	if prev != v || s.OnChange != nil {
		// Always notify (antd onChange on click); structure is stable.
		if s.OnChange != nil {
			s.OnChange(v)
		}
	}
}

// SetFace sets font (rebuilds labels).
func (s *Segmented) SetFace(face text.Face) {
	s.Face = face
	s.rebuild()
}

func (s *Segmented) theme() *core.Theme {
	if s != nil && s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Segmented) rebuild() {
	th := s.theme()
	s.row = primitive.Row()
	s.row.Gap = 2
	s.row.CrossAlign = core.CrossCenter
	s.chips = s.chips[:0]
	for _, opt := range s.Options {
		opt := opt
		lab := primitive.NewText(opt)
		lab.FontSize = 14
		lab.Face = s.Face
		lab.Color = th.Color(core.TokenColorText)
		host := primitive.NewDecorated(lab)
		host.Padding = primitive.Symmetric(11, 4)
		host.Radius = 4
		s.chips = append(s.chips, host)
		p := primitive.NewPressable(host)
		p.ShowFocusRing = false
		p.Click = func() { s.SetValue(opt) }
		s.row.AddChild(p)
	}
	s.applySelection()
	if s.Root == nil {
		s.Root = primitive.NewDecorated(s.row)
	} else {
		s.Root.ClearChildren()
		s.Root.AddChild(s.row)
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

// applySelection updates chip backgrounds only (no child list mutation).
func (s *Segmented) applySelection() {
	if s == nil {
		return
	}
	th := s.theme()
	selectedBG := th.Color(core.TokenColorBgContainer)
	if selectedBG.A < 0.5 {
		selectedBG = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	for i, host := range s.chips {
		if host == nil || i >= len(s.Options) {
			continue
		}
		if s.Options[i] == s.Value {
			host.Background = selectedBG
		} else {
			host.Background = render.RGBA{} // transparent
		}
		host.MarkNeedsPaint()
	}
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
}
