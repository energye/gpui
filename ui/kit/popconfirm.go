package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Popconfirm is Popover + confirm/cancel (simplified).
// https://ant.design/components/popconfirm
// Open state lives on embedded *Popover (no shadowing Open field).
type Popconfirm struct {
	*Popover
	Title     string
	OnConfirm func()
	OnCancel  func()
	Face      text.Face
	titleLab  *primitive.Text
	okBtn     *Button
	cancelBtn *Button
}

// NewPopconfirm wraps trigger with confirm UI (title + OK/Cancel).
func NewPopconfirm(trigger core.Node, title string) *Popconfirm {
	if title == "" {
		title = "Are you sure?"
	}
	pc := &Popconfirm{Title: title}

	pc.titleLab = primitive.NewText(title)
	pc.titleLab.FontSize = 14
	// Explicit color so theme/dark paths never leave A=0 invisible text.
	pc.titleLab.Color = DefaultTheme().Color(core.TokenColorText)

	pc.okBtn = NewButton("OK")
	pc.okBtn.SetType(ButtonPrimary)
	pc.okBtn.SetOnClick(func() {
		if pc.OnConfirm != nil {
			pc.OnConfirm()
		}
		pc.SetOpen(false)
	})
	pc.cancelBtn = NewButton("Cancel")
	pc.cancelBtn.SetOnClick(func() {
		if pc.OnCancel != nil {
			pc.OnCancel()
		}
		pc.SetOpen(false)
	})
	row := primitive.Row(pc.cancelBtn.Node(), pc.okBtn.Node())
	row.Gap = 8
	body := primitive.Column(pc.titleLab, row)
	body.Gap = 12
	body.CrossAlign = core.CrossStart

	pc.Popover = NewPopover(trigger, body)
	return pc
}

// SetOpen shows/hides the confirm popup.
func (p *Popconfirm) SetOpen(open bool) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetOpen(open)
}

// SetFace sets font on title and footer buttons.
func (p *Popconfirm) SetFace(face text.Face) {
	if p == nil {
		return
	}
	p.Face = face
	if p.titleLab != nil {
		p.titleLab.Face = face
		p.titleLab.MarkNeedsLayout()
		p.titleLab.MarkNeedsPaint()
	}
	if p.okBtn != nil {
		p.okBtn.SetFace(face)
	}
	if p.cancelBtn != nil {
		p.cancelBtn.SetFace(face)
	}
}

// Node returns the composed control (trigger + portal).
func (p *Popconfirm) Node() core.Node {
	if p == nil || p.Popover == nil {
		return nil
	}
	return p.Popover.Node()
}
