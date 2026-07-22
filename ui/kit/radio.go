package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

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
	Style    Style

	lastHovered bool
	indSize     float64
}

// NewRadio creates a radio option.
func NewRadio(value, label string) *Radio {
	r := &Radio{Value: value, Label: label}
	r.rebuild()
	return r
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
	r.Style.Face = face
	if r.label != nil {
		r.label.Face = face
	}
}

// SetStyle applies visual overrides.
func (r *Radio) SetStyle(st Style) {
	r.Style = st
	if st.Face != nil {
		r.SetFace(st.Face)
	}
	if st.FontSize > 0 && r.label != nil {
		r.label.FontSize = st.FontSize
	}
	r.applyChrome()
}

// SetTextColor overrides label color.
func (r *Radio) SetTextColor(c render.RGBA) {
	r.Style.Text = c
	r.applyChrome()
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
		// Unselected: thin hollow ring. Selected: thicker primary hollow ring (空心圆).
		lw := lineW
		if r.Selected {
			lw = lineW + 1.5
			if lw < 2 {
				lw = 2
			}
		}
		pc.StrokeLocalCircle(cx, cy, outerR, lw, bd)
		// No solid fill — selected is hollow circle only.
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
	r.Root.ShowFocusRing = false      // Ant: no mouse-focus outline on radio
	r.Root.FocusRingRadius = size / 2 // circular indicator-led soft corner for row
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
			if r.Style.hasText() {
				r.label.Color = r.Style.Text
			}
		}
	}
	r.dot.MarkNeedsPaint()
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
