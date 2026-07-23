package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Paragraph is Ant Typography.Paragraph (multi-line body text).
type Paragraph struct {
	Root *primitive.Text
}

// NewParagraph creates body paragraph text (wraps, ellipsis off by default).
func NewParagraph(value string) *Paragraph {
	t := primitive.NewText(value)
	t.FontSize = 14
	t.MaxLines = 8 // sensible default wrap budget for dense UI
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

// SetEllipsis enables last-line ellipsis when MaxLines is exceeded.
func (p *Paragraph) SetEllipsis(on bool) {
	if p != nil && p.Root != nil {
		p.Root.SetEllipsis(on)
	}
}

// SetMaxLines sets wrap line budget.
func (p *Paragraph) SetMaxLines(n int) {
	if p != nil && p.Root != nil {
		p.Root.SetMaxLines(n)
	}
}

// SetMaxWidth sets preferred max width for wrap/ellipsis.
func (p *Paragraph) SetMaxWidth(w float64) {
	if p != nil && p.Root != nil {
		p.Root.MaxWidth = w
		p.Root.MarkNeedsLayout()
	}
}
