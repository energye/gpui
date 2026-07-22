package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Watermark is a full-area overlay label (simplified).
// https://ant.design/components/watermark
type Watermark struct {
	Root    *primitive.Stack
	Content core.Node
	Text    string
	Face    text.Face
}

// NewWatermark wraps content with a watermark label.
func NewWatermark(content core.Node, text string) *Watermark {
	w := &Watermark{Content: content, Text: text}
	w.rebuild()
	return w
}

// Node returns root.
func (w *Watermark) Node() core.Node {
	if w.Root == nil {
		w.rebuild()
	}
	return w.Root
}

func (w *Watermark) rebuild() {
	w.Root = primitive.NewStack()
	if w.Content != nil {
		w.Root.AddChild(w.Content)
	}
	lab := primitive.NewText(w.Text)
	lab.FontSize = 18
	lab.Face = w.Face
	lab.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.08}
	w.Root.AddChild(primitive.Positioned(core.AlignCenter, lab))
}
