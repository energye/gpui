package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Title is Ant Typography.Title.
// https://ant.design/components/typography
type Title struct {
	Root *primitive.Text
}

// NewTitle creates title text (level 1–5).
func NewTitle(value string, level int) *Title {
	t := primitive.NewText(value)
	switch level {
	case 1:
		t.FontSize = 38
	case 2:
		t.FontSize = 30
	case 3:
		t.FontSize = 24
	case 4:
		t.FontSize = 20
	default:
		t.FontSize = 16
	}
	return &Title{Root: t}
}

// Node returns text node.
func (t *Title) Node() core.Node {
	if t == nil {
		return nil
	}
	return t.Root
}

// SetFace sets font face.
func (t *Title) SetFace(face text.Face) {
	if t != nil && t.Root != nil {
		t.Root.Face = face
	}
}
