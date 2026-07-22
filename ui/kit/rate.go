package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Rate is Ant Design Rate (star rating).
// https://ant.design/components/rate
type Rate struct {
	Root       *primitive.Flex
	Value      int
	Count      int
	AllowClear bool
	Face       text.Face
	Theme      *core.Theme
	OnChange   func(v int)
}

// NewRate creates a 5-star rate control.
func NewRate(value int) *Rate {
	r := &Rate{Value: value, Count: 5}
	r.rebuild()
	return r
}

// Node returns root.
func (r *Rate) Node() core.Node {
	if r.Root == nil {
		r.rebuild()
	}
	return r.Root
}

// SetValue sets stars filled.
func (r *Rate) SetValue(v int) {
	if v < 0 {
		v = 0
	}
	if v > r.Count {
		v = r.Count
	}
	r.Value = v
	r.rebuild()
	if r.OnChange != nil {
		r.OnChange(v)
	}
}

// SetFace sets font.
func (r *Rate) SetFace(face text.Face) {
	r.Face = face
	r.rebuild()
}

// SetCount sets the number of stars (default 5).
func (r *Rate) SetCount(n int) {
	if n <= 0 {
		n = 5
	}
	r.Count = n
	if r.Value > r.Count {
		r.Value = r.Count
	}
	r.rebuild()
}

func (r *Rate) rebuild() {
	th := DefaultTheme()
	if r.Theme != nil {
		th = r.Theme
	}
	if r.Count <= 0 {
		r.Count = 5
	}
	r.Root = primitive.Row()
	r.Root.Gap = 4
	r.Root.CrossAlign = core.CrossCenter
	gold := render.Hex("#FADB14")
	muted := th.Color(core.TokenColorTextSecondary)
	if muted.A < 0.2 {
		muted = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	}
	for i := 1; i <= r.Count; i++ {
		i := i
		star := primitive.NewText("★")
		star.FontSize = 20
		star.Face = r.Face
		if i <= r.Value {
			star.Color = gold
		} else {
			star.Color = muted
		}
		p := primitive.NewPressable(star)
		p.EnableRipple = false
		p.ShowFocusRing = false
		p.Click = func() {
			if r.AllowClear && r.Value == i {
				r.SetValue(0)
				return
			}
			r.SetValue(i)
		}
		r.Root.AddChild(p)
	}
}
