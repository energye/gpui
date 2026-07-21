package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Indicator size is fixed at 16 logical px (Ant checkbox/radio). Geometry tokens
// (radius, line width, gaps) come from Theme where available.
const indicatorSize = 16.0

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

// IndicatorNode returns the bare indicator (no label) for composition / visual tests.
func (c *Checkbox) IndicatorNode() core.Node {
	if c.box == nil {
		c.rebuild()
	}
	return c.box
}

// SetChecked updates checked state.
func (c *Checkbox) SetChecked(v bool) {
	if c.Checked == v && !c.Indeterminate {
		return
	}
	c.Checked = v
	c.Indeterminate = false
	c.applyChrome()
}

// SetIndeterminate sets the mixed state (clears pure checked for display).
func (c *Checkbox) SetIndeterminate(v bool) {
	c.Indeterminate = v
	if v {
		// Mixed UI still uses primary chrome; Checked may remain for form models.
	}
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
	size := indicatorSize
	radius := th.SizeOr(core.TokenBorderRadiusSM, 4)
	border := th.SizeOr(core.TokenLineWidth, 1)
	gap := th.SizeOr(core.TokenMarginSM, 8)

	c.box = primitive.NewDecorated()
	c.box.Width, c.box.Height = size, size
	c.box.MinWidth, c.box.MinHeight = size, size
	c.box.Radius = radius
	c.box.BorderWidth = border

	// Check / indeterminate mark overlaid in the indicator box.
	mark := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if (!c.Checked && !c.Indeterminate) || pc == nil || pc.DC == nil {
			return
		}
		col := th.Color(core.TokenColorTextInverse)
		if c.Disabled {
			col = th.Color(core.TokenColorDisabledText)
			if col.A < 0.2 {
				col = render.RGBA{R: 1, G: 1, B: 1, A: 0.55}
			}
		}
		w, h := sz.Width, sz.Height
		if w <= 0 {
			w = size
		}
		if h <= 0 {
			h = size
		}
		if c.Indeterminate {
			// Centered horizontal bar.
			barH := h * 0.125
			if barH < 2 {
				barH = 2
			}
			padX := w * 0.2
			pc.FillLocalRect(padX, (h-barH)/2, w-2*padX, barH, col)
			return
		}
		// Check mark geometry (centered in the square).
		dc := pc.DC
		dc.SetRGBA(col.R, col.G, col.B, col.A)
		lw := w * 0.12
		if lw < 1.4 {
			lw = 1.4
		}
		if lw > 2.2 {
			lw = 2.2
		}
		dc.SetLineWidth(lw)
		dc.SetLineCap(render.LineCapRound)
		dc.SetLineJoin(render.LineJoinRound)
		o := pc.Origin
		// Relative proportions of a 16px Ant-style check.
		dc.MoveTo(o.X+w*0.22, o.Y+h*0.50)
		dc.LineTo(o.X+w*0.42, o.Y+h*0.70)
		dc.LineTo(o.X+w*0.78, o.Y+h*0.30)
		_ = dc.Stroke()
	})
	mark.Width, mark.Height = size, size
	c.box.AddChild(mark)

	c.label = primitive.NewText(c.Label)
	c.label.FontSize = th.SizeOr(core.TokenFontSize, 14)
	c.label.Face = c.Face
	c.label.Color = th.Color(core.TokenColorText)

	row := primitive.Row(c.box, c.label)
	row.Gap = gap
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
		if c.Checked || c.Indeterminate {
			// Muted primary so the mark stays legible on disabled checked.
			p := th.Color(core.TokenColorPrimary)
			c.box.Background = render.RGBA{R: p.R, G: p.G, B: p.B, A: 0.45}
			c.box.BorderColor = c.box.Background
		} else {
			c.box.Background = th.Color(core.TokenColorDisabledBg)
			c.box.BorderColor = th.Color(core.TokenColorBorder)
		}
		if c.label != nil {
			c.label.Color = th.Color(core.TokenColorDisabledText)
		}
		c.box.MarkNeedsPaint()
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

// IndicatorNode returns the bare radio circle (no label).
func (r *Radio) IndicatorNode() core.Node {
	if r.dot == nil {
		r.rebuild()
	}
	return r.dot
}

// SetSelected updates selection chrome.
func (r *Radio) SetSelected(v bool) {
	r.Selected = v
	r.applyChrome()
}

// SetFace sets the label font.
func (r *Radio) SetFace(face text.Face) {
	r.Face = face
	if r.label != nil {
		r.label.Face = face
	}
}

// SetDisabled toggles disabled chrome.
func (r *Radio) SetDisabled(d bool) {
	r.Disabled = d
	if r.Root != nil {
		r.Root.SetDisabled(d)
	}
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
	size := indicatorSize
	// True circle: radius = half side.
	radius := size / 2
	border := th.SizeOr(core.TokenLineWidth, 1)
	gap := th.SizeOr(core.TokenMarginSM, 8)

	r.dot = primitive.NewDecorated()
	r.dot.Width, r.dot.Height = size, size
	r.dot.MinWidth, r.dot.MinHeight = size, size
	r.dot.Radius = radius
	r.dot.BorderWidth = border

	inner := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if !r.Selected || pc == nil {
			return
		}
		col := th.Color(core.TokenColorPrimary)
		if r.Disabled {
			col = render.RGBA{R: col.R, G: col.G, B: col.B, A: 0.45}
		}
		w, h := sz.Width, sz.Height
		if w <= 0 {
			w = size
		}
		if h <= 0 {
			h = size
		}
		// Inner disc ~ half the outer diameter, centered.
		outer := w
		if h < outer {
			outer = h
		}
		innerR := outer * 0.25 // diameter ≈ outer/2
		if innerR < 3 {
			innerR = 3
		}
		cx, cy := w/2, h/2
		pc.FillLocalRoundRect(cx-innerR, cy-innerR, innerR*2, innerR*2, innerR, col)
	})
	inner.Width, inner.Height = size, size
	r.dot.AddChild(inner)

	r.label = primitive.NewText(r.Label)
	r.label.FontSize = th.SizeOr(core.TokenFontSize, 14)
	r.label.Face = r.Face
	row := primitive.Row(r.dot, r.label)
	row.Gap = gap
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
	r.Root.SetDisabled(r.Disabled)
	r.applyChrome()
}

func (r *Radio) applyChrome() {
	if r.dot == nil {
		return
	}
	th := r.theme()
	r.dot.Background = th.Color(core.TokenColorBgContainer)
	if r.Disabled {
		r.dot.Background = th.Color(core.TokenColorDisabledBg)
		r.dot.BorderColor = th.Color(core.TokenColorBorder)
		if r.label != nil {
			r.label.Color = th.Color(core.TokenColorDisabledText)
		}
		r.dot.MarkNeedsPaint()
		return
	}
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
	thumb    *primitive.Decorated
	Checked  bool
	Disabled bool
	Theme    *core.Theme
	OnChange func(checked bool)

	trackW, trackH float64
	thumbSize      float64
	pad            float64
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

// IndicatorNode returns the track (includes thumb) for visual tests.
func (s *Switch) IndicatorNode() core.Node {
	if s.track == nil {
		s.rebuild()
	}
	return s.track
}

// SetChecked updates state.
func (s *Switch) SetChecked(v bool) {
	s.Checked = v
	s.applyChrome()
}

// SetDisabled toggles disabled.
func (s *Switch) SetDisabled(d bool) {
	s.Disabled = d
	if s.Root != nil {
		s.Root.SetDisabled(d)
	}
	s.applyChrome()
}

func (s *Switch) theme() *core.Theme {
	if s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Switch) rebuild() {
	// Ant-like switch geometry (stable defaults; height ≈ SM control).
	s.trackW, s.trackH = 44, 22
	s.pad = 2
	s.thumbSize = s.trackH - 2*s.pad // 18

	s.thumb = primitive.NewDecorated()
	s.thumb.Width, s.thumb.Height = s.thumbSize, s.thumbSize
	s.thumb.MinWidth, s.thumb.MinHeight = s.thumbSize, s.thumbSize
	s.thumb.Radius = s.thumbSize / 2 // true circle
	s.thumb.BorderWidth = 0
	s.thumb.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}

	s.track = primitive.NewDecorated(s.thumb)
	s.track.Width, s.track.Height = s.trackW, s.trackH
	s.track.MinWidth, s.track.MinHeight = s.trackW, s.trackH
	s.track.Radius = s.trackH / 2 // pill
	s.track.BorderWidth = 0
	s.track.Padding = primitive.EdgeInsets{Left: s.pad, Top: s.pad, Right: s.pad, Bottom: s.pad}

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
	s.Root.SetDisabled(s.Disabled)
	s.applyChrome()
}

func (s *Switch) applyChrome() {
	if s.track == nil {
		return
	}
	th := s.theme()
	// Thumb slides by changing left padding (content offset inside track).
	leftOn := s.trackW - s.pad - s.thumbSize
	if leftOn < s.pad {
		leftOn = s.pad
	}
	if s.Checked {
		s.track.Background = th.Color(core.TokenColorPrimary)
		s.track.Padding = primitive.EdgeInsets{Left: leftOn, Top: s.pad, Right: s.pad, Bottom: s.pad}
	} else {
		bg := th.Color(core.TokenColorFillSecondary)
		if bg.A < 0.15 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		}
		s.track.Background = bg
		s.track.Padding = primitive.EdgeInsets{Left: s.pad, Top: s.pad, Right: s.pad, Bottom: s.pad}
	}
	if s.Disabled {
		s.track.Background = th.Color(core.TokenColorDisabledBg)
		// Keep a subtle border so the track remains visible on white.
		s.track.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
		s.track.BorderColor = th.Color(core.TokenColorBorder)
	} else {
		s.track.BorderWidth = 0
	}
	if s.thumb != nil {
		s.thumb.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		s.thumb.MarkNeedsPaint()
	}
	s.track.MarkNeedsLayout()
	s.track.MarkNeedsPaint()
}
