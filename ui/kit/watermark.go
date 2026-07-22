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
	Gap     float64 // spacing hint for tiled marks (state; paint uses single center)
	Face    text.Face
}

// NewWatermark wraps content with a watermark label.
func NewWatermark(content core.Node, text string) *Watermark {
	w := &Watermark{Content: content, Text: text, Gap: 100}
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

// SetText updates watermark text and rebuilds.
func (w *Watermark) SetText(s string) {
	w.Text = s
	w.rebuild()
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
	// Gap reserved for multi-tile layout; single center mark for now.
	_ = w.Gap
	w.Root.AddChild(primitive.Positioned(core.AlignCenter, lab))
}
