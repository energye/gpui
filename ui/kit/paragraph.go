package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Paragraph is Ant Typography.Paragraph.
type Paragraph struct {
	Root *primitive.Text
}

// NewParagraph creates body paragraph text.
func NewParagraph(value string) *Paragraph {
	t := primitive.NewText(value)
	t.FontSize = 14
	return &Paragraph{Root: t}
}

// Node returns text node.
func (p *Paragraph) Node() core.Node {
	if p == nil {
		return nil
	}
	return p.Root
}

// SetFace sets font face.
func (p *Paragraph) SetFace(face text.Face) {
	if p != nil && p.Root != nil {
		p.Root.Face = face
	}
}
