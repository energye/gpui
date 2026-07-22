package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// TextArea is a multi-line Input.
type TextArea struct {
	*Input
	Rows int
}

// NewTextArea creates a multi-line field.
func NewTextArea(placeholder string, rows int) *TextArea {
	if rows < 2 {
		rows = 3
	}
	ta := &TextArea{Rows: rows}
	ta.Input = NewInput(placeholder)
	if ta.editor != nil {
		ta.editor.Multiline = true
		fs := ta.theme().SizeOr(core.TokenFontSize, 14)
		// Ant line-height ≈ 1.5714285714 (22/14)
		ta.editor.Height = fs * 1.5714285714 * float64(rows)
		if ta.Root != nil {
			ta.Root.Height = 0
			ta.Root.MinHeight = ta.editor.Height + 8
			ta.Root.Padding = primitive.Symmetric(
				ta.theme().SizeOr(core.TokenControlPaddingInline, 11),
				ta.theme().SizeOr(core.TokenPaddingXS, 4),
			)
		}
	}
	return ta
}
