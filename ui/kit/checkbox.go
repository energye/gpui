package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Checkbox is a toggleable check control with optional label.
// Indicator is 16 logical px (Ant Design); hover border uses primary.
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

	lastHovered bool
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

// SyncState applies hover border chrome from Pressable.
func (c *Checkbox) SyncState() {
	if c.Root == nil {
		return
	}
	h := c.Root.State.Hovered
	if h == c.lastHovered {
		return
	}
	c.lastHovered = h
	c.applyChrome()
}

func (c *Checkbox) theme() *core.Theme {
	if c.Theme != nil {
		return c.Theme
	}
	return DefaultTheme()
}

func (c *Checkbox) rebuild() {
	th := c.theme()
	size := th.SizeOr(core.TokenSizeIndicator, 16)
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
			// Centered horizontal bar (Ant mixed state).
			barH := h * 0.125
			if barH < 2 {
				barH = 2
			}
			padX := w * 0.2
			pc.FillLocalRect(padX, (h-barH)/2, w-2*padX, barH, col)
			return
		}
		// Shared check stroke (AA + round caps via PaintContext).
		pc.PaintLocalCheck(w, h, 0, col)
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
	c.Root.ShowFocusRing = false // Ant: no mouse-focus outline on checkbox
	c.Root.OnStateChange = c.SyncState
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
	hovered := c.Root != nil && c.Root.State.Hovered && !c.Disabled
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
		if hovered {
			c.box.Background = th.Color(core.TokenColorPrimaryHover)
			c.box.BorderColor = th.Color(core.TokenColorPrimaryHover)
		}
	} else {
		c.box.Background = th.Color(core.TokenColorBgContainer)
		c.box.BorderColor = th.Color(core.TokenColorBorder)
		if hovered {
			// Ant: unchecked hover → primary border.
			c.box.BorderColor = th.Color(core.TokenColorPrimary)
		}
	}
	if c.label != nil {
		c.label.Color = th.Color(core.TokenColorText)
	}
	c.box.MarkNeedsPaint()
}

// Radio is a single radio option; group coordination via RadioGroup.
// Outer ring + inner disc use true circle paths (not rounded rects) for AA quality.
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

	lastHovered bool
	indSize     float64
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

// SyncState applies hover chrome from Pressable.
func (r *Radio) SyncState() {
	if r.Root == nil {
		return
	}
	h := r.Root.State.Hovered
	if h == r.lastHovered {
		return
	}
	r.lastHovered = h
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
	size := th.SizeOr(core.TokenSizeIndicator, 16)
	r.indSize = size
	gap := th.SizeOr(core.TokenMarginSM, 8)
	lineW := th.SizeOr(core.TokenLineWidth, 1)

	// Transparent Decorated shell for layout size; circle painted by PainterNode.
	r.dot = primitive.NewDecorated()
	r.dot.Width, r.dot.Height = size, size
	r.dot.MinWidth, r.dot.MinHeight = size, size
	r.dot.Radius = size / 2
	r.dot.BorderWidth = 0
	r.dot.Background = render.RGBA{}

	// Full indicator: outer fill + stroke + optional inner disc via true circles.
	ring := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		w, h := sz.Width, sz.Height
		if w <= 0 {
			w = size
		}
		if h <= 0 {
			h = size
		}
		cx, cy := w/2, h/2
		outerR := w / 2
		if h/2 < outerR {
			outerR = h / 2
		}
		// Background fill.
		bg := th.Color(core.TokenColorBgContainer)
		if r.Disabled {
			bg = th.Color(core.TokenColorDisabledBg)
		}
		pc.FillLocalCircle(cx, cy, outerR, bg)

		// Border color by state.
		bd := th.Color(core.TokenColorBorder)
		if r.Disabled {
			bd = th.Color(core.TokenColorBorder)
		} else if r.Selected {
			bd = th.Color(core.TokenColorPrimary)
		} else if r.Root != nil && r.Root.State.Hovered {
			bd = th.Color(core.TokenColorPrimary)
		}
		pc.StrokeLocalCircle(cx, cy, outerR, lineW, bd)

		if !r.Selected {
			return
		}
		// Inner disc ≈ half outer diameter (Ant radio).
		innerR := outerR * 0.5
		if innerR < 3 {
			innerR = 3
		}
		// Leave a clear gap from the ring (stroke already inside outerR).
		if innerR > outerR-lineW-1.5 {
			innerR = outerR - lineW - 1.5
		}
		col := th.Color(core.TokenColorPrimary)
		if r.Disabled {
			col = render.RGBA{R: col.R, G: col.G, B: col.B, A: 0.45}
		}
		pc.FillLocalCircle(cx, cy, innerR, col)
	})
	ring.Width, ring.Height = size, size
	r.dot.AddChild(ring)

	r.label = primitive.NewText(r.Label)
	r.label.FontSize = th.SizeOr(core.TokenFontSize, 14)
	r.label.Face = r.Face
	row := primitive.Row(r.dot, r.label)
	row.Gap = gap
	row.CrossAlign = core.CrossCenter
	r.Root = primitive.NewPressable(row)
	r.Root.Focusable = true
	r.Root.ShowFocusRing = false // Ant: no mouse-focus outline on radio
	r.Root.OnStateChange = r.SyncState
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
	// Circle paint reads Selected/Disabled/Hovered each frame; just dirty paint.
	if r.label != nil {
		if r.Disabled {
			r.label.Color = th.Color(core.TokenColorDisabledText)
		} else {
			r.label.Color = th.Color(core.TokenColorText)
		}
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

// Switch is an on/off toggle (Ant default 44×22).
// Thumb slide uses primitive.FloatAnim (shared demand-frame animation primitive).
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
	lastHovered    bool

	// thumbPos is animated 0 (off) → 1 (on); left padding derives from it.
	thumbPos  primitive.FloatAnim
	boundTree *core.Tree
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

// SetChecked updates state and animates the thumb.
func (s *Switch) SetChecked(v bool) {
	if s.Checked == v {
		return
	}
	s.Checked = v
	s.animateThumb()
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

// SyncState applies hover chrome.
func (s *Switch) SyncState() {
	if s.Root == nil {
		return
	}
	h := s.Root.State.Hovered
	if h == s.lastHovered {
		return
	}
	s.lastHovered = h
	s.applyChrome()
}

func (s *Switch) theme() *core.Theme {
	if s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Switch) rebuild() {
	th := s.theme()
	s.trackW = th.SizeOr(core.TokenSwitchWidth, 44)
	s.trackH = th.SizeOr(core.TokenSwitchHeight, 22)
	s.pad = 2
	s.thumbSize = s.trackH - 2*s.pad // 18

	// Shared FloatAnim for thumb travel.
	s.thumbPos.Duration = 0.3 // Ant motionDurationMid-ish; ease-in-out feels steadier
	s.thumbPos.Easing = primitive.EaseInOutCubic
	if s.Checked {
		s.thumbPos.Snap(1)
	} else {
		s.thumbPos.Snap(0)
	}
	s.thumbPos.OnUpdate = func(v float64) {
		s.applyThumbPad(v)
	}

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
	s.track.SetCenterContent(false) // keep thumb circular; pad positions it

	s.Root = primitive.NewPressable(s.track)
	s.Root.Focusable = true
	s.Root.ShowFocusRing = false // Ant: no focus ring on switch
	s.Root.OnStateChange = s.SyncState
	s.Root.Click = func() {
		if s.Disabled {
			return
		}
		s.Checked = !s.Checked
		s.animateThumb()
		s.applyChrome()
		if s.OnChange != nil {
			s.OnChange(s.Checked)
		}
	}
	s.Root.SetDisabled(s.Disabled)
	s.applyChrome()
}

// AttachTicker registers Switch for demand-frame ANIMATING (thumb slide).
func (s *Switch) AttachTicker(t *core.Tree) {
	if s == nil || t == nil {
		return
	}
	s.boundTree = t
	if s.thumbPos.Active() {
		t.AddTicker(s)
	}
}

// Tick advances thumb animation. Implements core.Ticker.
func (s *Switch) Tick(dt float64) bool {
	if s == nil {
		return false
	}
	still := s.thumbPos.Tick(dt)
	if !still && s.boundTree != nil {
		s.boundTree.RemoveTicker(s)
	}
	return still
}

func (s *Switch) animateThumb() {
	to := 0.0
	if s.Checked {
		to = 1
	}
	// Prefer live tree from mounted Root so gallery works without AttachTicker.
	if s.boundTree == nil && s.Root != nil {
		s.boundTree = s.Root.Tree()
	}
	// Without a tree there is no demand-frame Tick — snap for headless/tests.
	if s.boundTree == nil {
		s.thumbPos.Snap(to)
		s.applyThumbPad(to)
		return
	}
	s.thumbPos.Duration = 0.3
	s.thumbPos.SetTarget(to)
	if s.thumbPos.Active() {
		s.boundTree.AddTicker(s)
	}
}

func (s *Switch) applyThumbPad(t float64) {
	if s.track == nil {
		return
	}
	leftOff := s.pad
	leftOn := s.trackW - s.pad - s.thumbSize
	if leftOn < s.pad {
		leftOn = s.pad
	}
	left := leftOff + (leftOn-leftOff)*t
	s.track.Padding = primitive.EdgeInsets{Left: left, Top: s.pad, Right: s.pad, Bottom: s.pad}
	// Padding change must re-layout (LayoutSkipIfClean would keep old thumb offset).
	s.track.MarkNeedsLayout()
	s.track.MarkNeedsPaint()
	if s.Root != nil {
		s.Root.MarkNeedsLayout()
		s.Root.MarkNeedsPaint()
	}
	// Immediately re-layout track under last constraints so thumb moves this frame.
	if s.track.Width > 0 && s.track.Height > 0 {
		_ = s.track.Layout(core.Tight(s.track.Width, s.track.Height))
	}
}

func (s *Switch) applyChrome() {
	if s.track == nil {
		return
	}
	th := s.theme()
	hovered := s.Root != nil && s.Root.State.Hovered && !s.Disabled
	if s.Checked {
		bg := th.Color(core.TokenColorPrimary)
		if hovered {
			bg = th.Color(core.TokenColorPrimaryHover)
		}
		s.track.Background = bg
	} else {
		bg := render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		if hovered {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.35}
		}
		s.track.Background = bg
	}
	if s.Disabled {
		s.track.Background = th.Color(core.TokenColorDisabledBg)
		s.track.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
		s.track.BorderColor = th.Color(core.TokenColorBorder)
	} else {
		s.track.BorderWidth = 0
	}
	s.applyThumbPad(s.thumbPos.Current)
	if s.thumb != nil {
		s.thumb.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		s.thumb.MarkNeedsPaint()
	}
}
