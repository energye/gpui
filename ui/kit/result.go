package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Result is Ant Design Result status page.
// https://ant.design/components/result
type Result struct {
	Root     *primitive.Flex
	Status   string // success, error, info, warning
	Title    string
	SubTitle string
	Face     text.Face
	Theme    *core.Theme
}

// NewResult creates a result block.
func NewResult(status, title, sub string) *Result {
	r := &Result{Status: status, Title: title, SubTitle: sub}
	r.rebuild()
	return r
}

// Node returns root.
func (r *Result) Node() core.Node {
	if r.Root == nil {
		r.rebuild()
	}
	return r.Root
}

// SetFace sets font.
func (r *Result) SetFace(face text.Face) {
	r.Face = face
	r.rebuild()
}

// SetStatus updates status type and rebuilds.
func (r *Result) SetStatus(s string) {
	r.Status = s
	r.rebuild()
}

// SetTitle updates title and rebuilds.
func (r *Result) SetTitle(s string) {
	r.Title = s
	r.rebuild()
}

// SetSubTitle updates subtitle and rebuilds.
func (r *Result) SetSubTitle(s string) {
	r.SubTitle = s
	r.rebuild()
}

func (r *Result) rebuild() {
	th := DefaultTheme()
	if r.Theme != nil {
		th = r.Theme
	}
	icon := primitive.NewText(alertIcon(r.Status))
	icon.FontSize = 48
	icon.Face = r.Face
	icon.Color = alertColor(th, r.Status)
	title := primitive.NewText(r.Title)
	title.FontSize = 24
	title.Face = r.Face
	title.Color = th.Color(core.TokenColorText)
	sub := primitive.NewText(r.SubTitle)
	sub.FontSize = 14
	sub.Face = r.Face
	sub.Color = th.Color(core.TokenColorTextSecondary)
	r.Root = primitive.Column(icon, title, sub)
	r.Root.Gap = 12
	r.Root.CrossAlign = core.CrossCenter
	r.Root.Padding = primitive.All(24)
}
