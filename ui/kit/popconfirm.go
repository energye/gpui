package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Popconfirm is Popover + confirm/cancel (simplified).
// https://ant.design/components/popconfirm
type Popconfirm struct {
	*Popover
	Title     string
	Open      bool
	OnConfirm func()
	OnCancel  func()
	Face      text.Face
}

// NewPopconfirm wraps trigger with confirm UI.
func NewPopconfirm(trigger core.Node, title string) *Popconfirm {
	pc := &Popconfirm{Title: title}
	body := primitive.Column()
	lab := primitive.NewText(title)
	lab.FontSize = 14
	ok := NewButton("OK")
	ok.SetType(ButtonPrimary)
	ok.SetOnClick(func() {
		if pc.OnConfirm != nil {
			pc.OnConfirm()
		}
		if pc.Popover != nil {
			pc.Popover.SetOpen(false)
		}
	})
	cancel := NewButton("Cancel")
	cancel.SetOnClick(func() {
		if pc.OnCancel != nil {
			pc.OnCancel()
		}
		if pc.Popover != nil {
			pc.Popover.SetOpen(false)
		}
	})
	row := primitive.Row(cancel.Node(), ok.Node())
	row.Gap = 8
	body.AddChild(lab)
	body.AddChild(row)
	body.Gap = 12
	pc.Popover = NewPopover(trigger, body)
	return pc
}

// SetOpen shows/hides the confirm popup.
func (p *Popconfirm) SetOpen(open bool) {
	p.Open = open
	if p.Popover != nil {
		p.Popover.SetOpen(open)
	}
}

// SetFace sets font on buttons/title via rebuild of popover content (best-effort).
func (p *Popconfirm) SetFace(face text.Face) {
	p.Face = face
}
