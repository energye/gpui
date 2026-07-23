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
	Style         Style

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
	c.Style.Face = face
	if c.label != nil {
		c.label.Face = face
	}
}

// SetStyle applies visual overrides.
func (c *Checkbox) SetStyle(st Style) {
	c.Style = st
	if st.Face != nil {
		c.SetFace(st.Face)
	}
	if st.FontSize > 0 && c.label != nil {
		c.label.FontSize = st.FontSize
	}
	c.applyChrome()
}

// SetTextColor overrides label color.
func (c *Checkbox) SetTextColor(col render.RGBA) {
	c.Style.Text = col
	c.applyChrome()
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

	if c.Root == nil {
		c.Root = primitive.NewPressable(row)
	} else {
		c.Root.ClearChildren()
		c.Root.AddChild(row)
	}
	c.Root.Focusable = true
	c.Root.ShowFocusRing = false // Ant: no mouse-focus outline on checkbox
	c.Root.FocusRingRadius = th.SizeOr(core.TokenBorderRadiusSM, 4)
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
		if c.Style.hasText() {
			c.label.Color = c.Style.Text
		}
	}
	c.box.MarkNeedsPaint()
}
