package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Statistic is Ant Design Statistic (title + value).
// https://ant.design/components/statistic
type Statistic struct {
	Root   *primitive.Flex
	Title  string
	Value  string
	Prefix string
	Suffix string
	Face   text.Face
	Theme  *core.Theme
}

// NewStatistic creates a statistic block.
func NewStatistic(title, value string) *Statistic {
	s := &Statistic{Title: title, Value: value}
	s.rebuild()
	return s
}

// Node returns root.
func (s *Statistic) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// SetValue updates value text.
func (s *Statistic) SetValue(v string) {
	s.Value = v
	s.rebuild()
}

// SetTitle updates title text.
func (s *Statistic) SetTitle(t string) {
	s.Title = t
	s.rebuild()
}

// SetPrefix sets value prefix.
func (s *Statistic) SetPrefix(p string) {
	s.Prefix = p
	s.rebuild()
}

// SetSuffix sets value suffix.
func (s *Statistic) SetSuffix(suf string) {
	s.Suffix = suf
	s.rebuild()
}

// SetFace sets font.
func (s *Statistic) SetFace(face text.Face) {
	s.Face = face
	s.rebuild()
}

func (s *Statistic) rebuild() {
	th := DefaultTheme()
	if s.Theme != nil {
		th = s.Theme
	}
	title := primitive.NewText(s.Title)
	title.FontSize = 14
	title.Face = s.Face
	title.Color = th.Color(core.TokenColorTextSecondary)
	valStr := s.Prefix + s.Value + s.Suffix
	val := primitive.NewText(valStr)
	val.FontSize = 24
	val.Face = s.Face
	val.Color = th.Color(core.TokenColorText)
	s.Root = primitive.Column(title, val)
	s.Root.Gap = 4
	s.Root.CrossAlign = core.CrossStart
}
