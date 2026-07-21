package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Checkbox is a toggleable check control with optional label.
type Checkbox struct {
	Root          *primitive.Pressable
	box           *primitive.Decorated
	label         *primitive.Text
	Checked       bool
	Disabled      bool
	Label         string
	Indeterminate bool
	Face          text.Face
	Theme         *core.Theme
	OnChange      func(checked bool)
}

// NewCheckbox creates a checkbox with label.
func NewCheckbox(label string) *Checkbox {
	c := &Checkbox{Label: label}
	c.rebuild()
	return c
}

// Node returns the root.
func (c *Checkbox) Node() core.Node {
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// SetChecked updates checked state.
func (c *Checkbox) SetChecked(v bool) {
	if c.Checked == v {
		return
	}
	c.Checked = v
	c.Indeterminate = false
	c.applyChrome()
}

// SetDisabled toggles disabled.
func (c *Checkbox) SetDisabled(d bool) {
	c.Disabled = d
	if c.Root != nil {
		c.Root.SetDisabled(d)
	}
	c.applyChrome()
}

// SetOnChange sets the callback.
func (c *Checkbox) SetOnChange(fn func(bool)) { c.OnChange = fn }

// SetFace sets label font.
func (c *Checkbox) SetFace(face text.Face) {
	c.Face = face
	if c.label != nil {
		c.label.Face = face
	}
}

func (c *Checkbox) theme() *core.Theme {
	if c.Theme != nil {
		return c.Theme
	}
	return DefaultTheme()
}

func (c *Checkbox) rebuild() {
	th := c.theme()
	c.box = primitive.NewDecorated()
	c.box.Width, c.box.Height = 16, 16
	c.box.MinWidth, c.box.MinHeight = 16, 16
	c.box.Radius = 4
	c.box.BorderWidth = 1

	// check icon overlay via PainterNode when checked
	mark := primitive.NewPainterNode(func(pc *core.PaintContext, size core.Size) {
		if !c.Checked && !c.Indeterminate {
			return
		}
		col := th.Color(core.TokenColorTextInverse)
		if c.Indeterminate {
			pc.FillLocalRect(3, 7, 10, 2, col)
			return
		}
		// simple check
		dc := pc.DC
		if dc == nil {
			return
		}
		dc.SetRGBA(col.R, col.G, col.B, col.A)
		dc.SetLineWidth(1.6)
		o := pc.Origin
		dc.MoveTo(o.X+3.5, o.Y+8)
		dc.LineTo(o.X+7, o.Y+11.5)
		dc.LineTo(o.X+12.5, o.Y+4.5)
		_ = dc.Stroke()
	})
	mark.Width, mark.Height = 16, 16
	c.box.AddChild(mark)

	c.label = primitive.NewText(c.Label)
	c.label.FontSize = th.SizeOr(core.TokenFontSize, 14)
	c.label.Face = c.Face
	c.label.Color = th.Color(core.TokenColorText)

	row := primitive.Row(c.box, c.label)
	row.Gap = 8
	row.CrossAlign = core.CrossCenter

	c.Root = primitive.NewPressable(row)
	c.Root.Focusable = true
	c.Root.Click = func() {
		if c.Disabled {
			return
		}
		c.Checked = !c.Checked
		c.Indeterminate = false
		c.applyChrome()
		if c.OnChange != nil {
			c.OnChange(c.Checked)
		}
	}
	c.Root.SetDisabled(c.Disabled)
	c.applyChrome()
}

func (c *Checkbox) applyChrome() {
	if c.box == nil {
		return
	}
	th := c.theme()
	if c.Disabled {
		c.box.Background = th.Color(core.TokenColorDisabledBg)
		c.box.BorderColor = th.Color(core.TokenColorBorder)
		if c.label != nil {
			c.label.Color = th.Color(core.TokenColorDisabledText)
		}
		return
	}
	if c.Checked || c.Indeterminate {
		c.box.Background = th.Color(core.TokenColorPrimary)
		c.box.BorderColor = th.Color(core.TokenColorPrimary)
	} else {
		c.box.Background = th.Color(core.TokenColorBgContainer)
		c.box.BorderColor = th.Color(core.TokenColorBorder)
	}
	if c.label != nil {
		c.label.Color = th.Color(core.TokenColorText)
	}
	c.box.MarkNeedsPaint()
}

// Radio is a single radio option; group coordination via RadioGroup.
type Radio struct {
	Root     *primitive.Pressable
	dot      *primitive.Decorated
	label    *primitive.Text
	Value    string
	Selected bool
	Disabled bool
	Label    string
	Face     text.Face
	Theme    *core.Theme
	OnSelect func(value string)
}

// NewRadio creates a radio option.
func NewRadio(value, label string) *Radio {
	r := &Radio{Value: value, Label: label}
	r.rebuild()
	return r
}

// Node returns the root.
func (r *Radio) Node() core.Node {
	if r.Root == nil {
		r.rebuild()
	}
	return r.Root
}

// SetSelected updates selection chrome.
func (r *Radio) SetSelected(v bool) {
	r.Selected = v
	r.applyChrome()
}

func (r *Radio) theme() *core.Theme {
	if r.Theme != nil {
		return r.Theme
	}
	return DefaultTheme()
}

func (r *Radio) rebuild() {
	th := r.theme()
	r.dot = primitive.NewDecorated()
	r.dot.Width, r.dot.Height = 16, 16
	r.dot.MinWidth, r.dot.MinHeight = 16, 16
	r.dot.Radius = 8
	r.dot.BorderWidth = 1
	inner := primitive.NewPainterNode(func(pc *core.PaintContext, size core.Size) {
		if !r.Selected || pc == nil || pc.DC == nil {
			return
		}
		col := th.Color(core.TokenColorPrimary)
		pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
		o := pc.Origin
		pc.DC.DrawCircle(o.X+8, o.Y+8, 4)
		_ = pc.DC.Fill()
	})
	inner.Width, inner.Height = 16, 16
	r.dot.AddChild(inner)

	r.label = primitive.NewText(r.Label)
	r.label.FontSize = th.SizeOr(core.TokenFontSize, 14)
	r.label.Face = r.Face
	row := primitive.Row(r.dot, r.label)
	row.Gap = 8
	row.CrossAlign = core.CrossCenter
	r.Root = primitive.NewPressable(row)
	r.Root.Focusable = true
	r.Root.Click = func() {
		if r.Disabled {
			return
		}
		if r.OnSelect != nil {
			r.OnSelect(r.Value)
		}
	}
	r.applyChrome()
}

func (r *Radio) applyChrome() {
	if r.dot == nil {
		return
	}
	th := r.theme()
	r.dot.Background = th.Color(core.TokenColorBgContainer)
	if r.Selected {
		r.dot.BorderColor = th.Color(core.TokenColorPrimary)
	} else {
		r.dot.BorderColor = th.Color(core.TokenColorBorder)
	}
	if r.label != nil {
		r.label.Color = th.Color(core.TokenColorText)
	}
	r.dot.MarkNeedsPaint()
}

// RadioGroup coordinates exclusive selection among radios.
type RadioGroup struct {
	Value    string
	Items    []*Radio
	OnChange func(value string)
	root     *primitive.Flex
}

// NewRadioGroup creates a vertical radio group.
func NewRadioGroup(items ...*Radio) *RadioGroup {
	g := &RadioGroup{Items: items}
	g.root = primitive.Column()
	g.root.Gap = 8
	for _, it := range items {
		it := it
		it.OnSelect = func(v string) { g.Select(v) }
		g.root.AddChild(it.Node())
	}
	return g
}

// Node returns the column root.
func (g *RadioGroup) Node() core.Node { return g.root }

// Select sets the active value.
func (g *RadioGroup) Select(v string) {
	g.Value = v
	for _, it := range g.Items {
		it.SetSelected(it.Value == v)
	}
	if g.OnChange != nil {
		g.OnChange(v)
	}
}

// Switch is an on/off toggle.
type Switch struct {
	Root     *primitive.Pressable
	track    *primitive.Decorated
	thumb    *primitive.Box
	Checked  bool
	Disabled bool
	Theme    *core.Theme
	OnChange func(checked bool)
}

// NewSwitch creates a switch.
func NewSwitch() *Switch {
	s := &Switch{}
	s.rebuild()
	return s
}

// Node returns the root.
func (s *Switch) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// SetChecked updates state.
func (s *Switch) SetChecked(v bool) {
	s.Checked = v
	s.applyChrome()
}

func (s *Switch) theme() *core.Theme {
	if s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Switch) rebuild() {
	s.thumb = primitive.NewBox()
	s.thumb.Width, s.thumb.Height = 18, 18
	s.thumb.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}

	s.track = primitive.NewDecorated(s.thumb)
	s.track.Width, s.track.Height = 44, 22
	s.track.MinWidth, s.track.MinHeight = 44, 22
	s.track.Radius = 11
	s.track.Padding = primitive.EdgeInsets{Left: 2, Top: 2, Right: 2, Bottom: 2}

	s.Root = primitive.NewPressable(s.track)
	s.Root.Focusable = true
	s.Root.Click = func() {
		if s.Disabled {
			return
		}
		s.Checked = !s.Checked
		s.applyChrome()
		if s.OnChange != nil {
			s.OnChange(s.Checked)
		}
	}
	s.applyChrome()
}

func (s *Switch) applyChrome() {
	if s.track == nil {
		return
	}
	th := s.theme()
	if s.Checked {
		s.track.Background = th.Color(core.TokenColorPrimary)
		// thumb to the right via padding trick: left pad expands
		s.track.Padding = primitive.EdgeInsets{Left: 24, Top: 2, Right: 2, Bottom: 2}
	} else {
		s.track.Background = th.Color(core.TokenColorFillSecondary)
		if s.track.Background.A < 0.2 {
			s.track.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		}
		s.track.Padding = primitive.EdgeInsets{Left: 2, Top: 2, Right: 2, Bottom: 2}
	}
	if s.Disabled {
		s.track.Background = th.Color(core.TokenColorDisabledBg)
	}
	s.track.MarkNeedsLayout()
	s.track.MarkNeedsPaint()
}
