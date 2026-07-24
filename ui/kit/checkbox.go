package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Checkbox defaults — style/index.ts prepareComponentToken.
// docs/antd/checkbox.md §6.2 / §6.10
const (
	DefaultCheckboxIndicator = 16.0
	DefaultCheckboxRadius    = 4.0 // borderRadiusSM
	DefaultCheckboxGap       = 8.0 // marginXS — indicator↔label & Group columnGap
	DefaultCheckboxLineWidth = 1.0
)

// Checkbox is a toggleable check control with optional label.
//
//	Pressable (role=checkbox)
//	  └─ Row
//	       ├─ Decorated indicator (icon) 16×16
//	       │    └─ PainterNode (check / indeterminate bar)
//	       └─ Text label
//
// Product contract: docs/antd/checkbox.md §6 (P0 DoD).
type Checkbox struct {
	Root  *primitive.Pressable
	box   *primitive.Decorated
	label *primitive.Text

	// Label is the visible text after the indicator (antd children).
	Label string
	// Title is the option title (antd title); also a11y name fallback.
	Title string
	// Value is the Group option value (antd value). Not the checked bool.
	Value string

	Checked       bool
	Indeterminate bool
	Disabled      bool
	// Controlled: click only fires OnChange; parent must SetChecked.
	Controlled bool

	OnChange  func(checked bool)
	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// group, if set, routes toggles through CheckboxGroup (options/children mode).
	group *CheckboxGroup

	lastHovered bool
	lastFocused bool
	indSize     float64
}

// CheckboxOption is one Group options[] entry (antd CheckboxOptionType).
type CheckboxOption struct {
	Label    string
	Value    string
	Disabled bool
	Title    string
}

// CheckboxGroup coordinates multi-select among checkboxes (antd Checkbox.Group).
//
// Default layout is inline-flex wrap with columnGap=marginXS (8). Custom layout
// (layout.tsx Row/Col) uses Add + SetBody.
type CheckboxGroup struct {
	root *primitive.Flex
	body core.Node

	Value    []string
	Options  []CheckboxOption
	Items    []*Checkbox
	Disabled bool
	Name     string
	// Controlled: toggle only fires OnChange; parent must SetValue.
	Controlled bool

	OnChange  func(values []string)
	Face      text.Face
	Theme     *core.Theme
	Style     Style
	AriaLabel string
}

// NewCheckbox creates a checkbox with the given label (antd children).
func NewCheckbox(label string) *Checkbox {
	c := &Checkbox{Label: label}
	c.rebuild()
	return c
}

// Node returns the root Pressable.
func (c *Checkbox) Node() core.Node {
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// ChromeNode returns the root chrome (Pressable).
func (c *Checkbox) ChromeNode() core.Node {
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// IndicatorNode returns the bare indicator (icon semantic part, no label).
func (c *Checkbox) IndicatorNode() core.Node {
	if c.box == nil {
		c.rebuild()
	}
	return c.box
}

// LabelNode returns the label text node (label semantic part).
func (c *Checkbox) LabelNode() core.Node {
	if c.label == nil {
		c.rebuild()
	}
	return c.label
}

// SetChecked updates checked state and clears indeterminate (display pure checked).
func (c *Checkbox) SetChecked(v bool) {
	if c.Checked == v && !c.Indeterminate {
		c.applyChrome()
		return
	}
	c.Checked = v
	c.Indeterminate = false
	c.applyChrome()
	c.applyA11y()
}

// SetDefaultChecked sets the initial checked state when not controlled.
func (c *Checkbox) SetDefaultChecked(v bool) {
	if c.Controlled {
		return
	}
	c.SetChecked(v)
}

// SetControlled marks parent-owned checked (antd checked={…} controlled).
func (c *Checkbox) SetControlled(v bool) { c.Controlled = v }

// SetIndeterminate sets the mixed-state visual (antd: style only).
// Does not change Checked; paint prefers indeterminate mark over check.
func (c *Checkbox) SetIndeterminate(v bool) {
	c.Indeterminate = v
	c.applyChrome()
	c.applyA11y()
}

// SetDisabled toggles disabled chrome and interaction.
func (c *Checkbox) SetDisabled(d bool) {
	c.Disabled = d
	if c.Root != nil {
		c.Root.SetDisabled(d || c.groupDisabled())
	}
	c.applyChrome()
	c.applyA11y()
}

// SetLabel updates the visible label text.
func (c *Checkbox) SetLabel(s string) {
	c.Label = s
	if c.label != nil {
		c.label.SetValue(s)
	}
	c.applyA11y()
}

// SetTitle sets antd title (option title / tooltip attribute).
func (c *Checkbox) SetTitle(s string) {
	c.Title = s
	c.applyA11y()
}

// SetValue sets the Group option value (not checked).
func (c *Checkbox) SetValue(v string) { c.Value = v }

// SetOnChange sets the change callback (desired next checked).
func (c *Checkbox) SetOnChange(fn func(bool)) { c.OnChange = fn }

// SetAriaLabel sets the accessible name override.
func (c *Checkbox) SetAriaLabel(name string) {
	c.AriaLabel = name
	c.applyA11y()
}

// SetFace sets the label font.
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
	if c.box != nil && st.hasRadius() {
		c.box.Radius = st.Radius
	}
	c.applyChrome()
}

// SetTheme sets an explicit theme (highest priority).
func (c *Checkbox) SetTheme(th *core.Theme) {
	c.Theme = th
	c.applyChrome()
}

// SetTextColor overrides label color.
func (c *Checkbox) SetTextColor(col render.RGBA) {
	c.Style.Text = col
	c.applyChrome()
}

// SyncState reapplies hover/focus chrome from Pressable.
func (c *Checkbox) SyncState() {
	if c.Root == nil {
		return
	}
	h := c.Root.State.Hovered
	f := c.Root.State.Focused && c.Root.State.FocusVisible
	if h == c.lastHovered && f == c.lastFocused {
		return
	}
	c.lastHovered = h
	c.lastFocused = f
	c.applyChrome()
}

func (c *Checkbox) theme() *core.Theme {
	var n core.Node
	if c.Root != nil {
		n = c.Root
	}
	return themeOf(c.Theme, n)
}

func (c *Checkbox) groupDisabled() bool {
	return c.group != nil && c.group.Disabled
}

func (c *Checkbox) isDisabled() bool {
	return c.Disabled || c.groupDisabled()
}

func (c *Checkbox) rebuild() {
	th := c.theme()
	size := th.SizeOr(core.TokenSizeIndicator, DefaultCheckboxIndicator)
	c.indSize = size
	radius := th.SizeOr(core.TokenBorderRadiusSM, DefaultCheckboxRadius)
	border := th.SizeOr(core.TokenLineWidth, DefaultCheckboxLineWidth)
	gap := th.SizeOr(core.TokenMarginSM, DefaultCheckboxGap)

	c.box = primitive.NewDecorated()
	c.box.Width, c.box.Height = size, size
	c.box.MinWidth, c.box.MinHeight = size, size
	c.box.Radius = radius
	if c.Style.hasRadius() {
		c.box.Radius = c.Style.Radius
	}
	c.box.BorderWidth = border

	// Check / indeterminate mark overlaid in the indicator box.
	mark := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if (!c.Checked && !c.Indeterminate) || pc == nil || pc.DC == nil {
			return
		}
		col := th.Color(core.TokenColorTextInverse)
		if c.isDisabled() {
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
			// Centered horizontal bar (antd mixed state).
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
	if c.Style.FontSize > 0 {
		c.label.FontSize = c.Style.FontSize
	}
	c.label.Face = c.Face

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
	c.Root.ShowFocusRing = true // §6.6 focus ring visible
	c.Root.FocusRingRadius = radius
	c.Root.OnStateChange = c.SyncState
	c.Root.Click = c.onActivate
	c.Root.SetDisabled(c.isDisabled())
	c.applyA11y()
	c.applyChrome()
}

func (c *Checkbox) onActivate() {
	if c.isDisabled() {
		return
	}
	// Group path: toggle membership; group owns value array.
	if c.group != nil {
		c.group.toggleOption(c)
		return
	}
	// CB-S4: indeterminate click → enter checked and clear half-select.
	next := !c.Checked
	if c.Indeterminate {
		next = true
	}
	if !c.Controlled {
		c.Checked = next
		c.Indeterminate = false
		c.applyChrome()
		c.applyA11y()
	}
	if c.OnChange != nil {
		c.OnChange(next)
	}
}

func (c *Checkbox) applyA11y() {
	if c.Root == nil {
		return
	}
	name := c.AriaLabel
	if name == "" {
		name = c.Label
	}
	if name == "" {
		name = c.Title
	}
	c.Root.Base().Role = "checkbox"
	c.Root.Base().Label = name
}

func (c *Checkbox) applyChrome() {
	if c.box == nil {
		return
	}
	th := c.theme()
	disabled := c.isDisabled()
	hovered := c.Root != nil && c.Root.State.Hovered && !disabled

	// Style overrides for icon (style-class object styles approximation).
	if c.Style.hasBG() && (c.Checked || c.Indeterminate) && !disabled {
		c.box.Background = c.Style.Background
	}
	if c.Style.hasBorder() && !disabled {
		// applied after state colors when unchecked; see below
	}

	if disabled {
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
		if c.Style.hasBG() {
			c.box.Background = c.Style.Background
			c.box.BorderColor = c.Style.Background
		}
		if hovered {
			if c.Style.hasBGHover() {
				c.box.Background = c.Style.BackgroundHover
				c.box.BorderColor = c.Style.BackgroundHover
			} else if !c.Style.hasBG() {
				c.box.Background = th.Color(core.TokenColorPrimaryHover)
				c.box.BorderColor = th.Color(core.TokenColorPrimaryHover)
			}
		}
	} else {
		c.box.Background = th.Color(core.TokenColorBgContainer)
		c.box.BorderColor = th.Color(core.TokenColorBorder)
		if c.Style.hasBorder() {
			c.box.BorderColor = c.Style.Border
		}
		if hovered {
			// antd: unchecked hover → primary border
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

// ── Checkbox.Group ──────────────────────────────────────────────────────────

// NewCheckboxGroup creates an empty multi-select group (antd Checkbox.Group).
func NewCheckboxGroup() *CheckboxGroup {
	g := &CheckboxGroup{Value: nil}
	g.root = primitive.Row()
	g.root.Gap = DefaultCheckboxGap
	g.root.Wrap = true
	g.root.CrossAlign = core.CrossCenter
	g.applyA11y()
	return g
}

// Node returns the group root (flex wrap or custom body host).
func (g *CheckboxGroup) Node() core.Node {
	if g.root == nil {
		g.root = primitive.Row()
		g.root.Gap = DefaultCheckboxGap
		g.root.Wrap = true
	}
	return g.root
}

// SetOptions replaces options and rebuilds item checkboxes (antd options).
func (g *CheckboxGroup) SetOptions(opts ...CheckboxOption) {
	g.Options = append([]CheckboxOption(nil), opts...)
	g.rebuildFromOptions()
}

// SetStringOptions is plainOptions sugar: label=value=each string.
func (g *CheckboxGroup) SetStringOptions(labels ...string) {
	opts := make([]CheckboxOption, len(labels))
	for i, s := range labels {
		opts[i] = CheckboxOption{Label: s, Value: s}
	}
	g.SetOptions(opts...)
}

// SetValue sets the selected values (controlled-friendly).
func (g *CheckboxGroup) SetValue(v []string) {
	g.Value = append([]string(nil), v...)
	g.Controlled = true
	g.syncItemsFromValue()
}

// SetDefaultValue sets initial selection when not controlled.
func (g *CheckboxGroup) SetDefaultValue(v []string) {
	if g.Controlled {
		return
	}
	g.Value = append([]string(nil), v...)
	g.syncItemsFromValue()
}

// Values returns a copy of the current selection.
func (g *CheckboxGroup) Values() []string {
	return append([]string(nil), g.Value...)
}

// SetDisabled disables the whole group (and all items).
func (g *CheckboxGroup) SetDisabled(d bool) {
	g.Disabled = d
	for _, it := range g.Items {
		// item may have own Disabled; group OR is applied via isDisabled()
		if it.Root != nil {
			it.Root.SetDisabled(it.Disabled || d)
		}
		it.applyChrome()
	}
}

// SetName sets the group name (antd name; desktop a11y metadata).
func (g *CheckboxGroup) SetName(name string) {
	g.Name = name
	g.applyA11y()
}

// SetOnChange sets the group change callback (selected values, option order).
func (g *CheckboxGroup) SetOnChange(fn func([]string)) { g.OnChange = fn }

// SetFace propagates face to option-built items.
func (g *CheckboxGroup) SetFace(face text.Face) {
	g.Face = face
	for _, it := range g.Items {
		it.SetFace(face)
	}
}

// SetTheme sets an explicit theme.
func (g *CheckboxGroup) SetTheme(th *core.Theme) {
	g.Theme = th
	for _, it := range g.Items {
		it.SetTheme(th)
	}
}

// SetAriaLabel sets the group accessible name.
func (g *CheckboxGroup) SetAriaLabel(s string) {
	g.AriaLabel = s
	g.applyA11y()
}

// Add registers child checkboxes (layout / children mode) and binds them.
// Does not change the visual tree; use SetBody or Node().AddChild for layout.
func (g *CheckboxGroup) Add(items ...*Checkbox) {
	for _, it := range items {
		if it == nil {
			continue
		}
		it.group = g
		if g.Face != nil && it.Face == nil {
			it.SetFace(g.Face)
		}
		if g.Theme != nil {
			it.SetTheme(g.Theme)
		}
		// Avoid double-register.
		found := false
		for _, ex := range g.Items {
			if ex == it {
				found = true
				break
			}
		}
		if !found {
			g.Items = append(g.Items, it)
		}
		// Sync checked from current value.
		it.Checked = g.contains(it.Value)
		it.Indeterminate = false
		if it.Root != nil {
			it.Root.SetDisabled(it.Disabled || g.Disabled)
		}
		it.applyChrome()
		it.applyA11y()
	}
}

// SetBody replaces the group visual children with a custom layout node
// (e.g. Row/Col for layout.tsx). Bound items still coordinate via Add.
func (g *CheckboxGroup) SetBody(n core.Node) {
	g.body = n
	if g.root == nil {
		g.root = primitive.Row()
	}
	g.root.ClearChildren()
	if n != nil {
		g.root.AddChild(n)
	}
	// Stretch to host width when custom body (layout demo).
	g.root.Wrap = false
	g.root.CrossAlign = core.CrossStretch
	g.applyA11y()
}

func (g *CheckboxGroup) applyA11y() {
	if g.root == nil {
		return
	}
	g.root.Base().Role = "group"
	g.root.Base().Label = g.AriaLabel
	if g.AriaLabel == "" && g.Name != "" {
		g.root.Base().Label = g.Name
	}
}

func (g *CheckboxGroup) rebuildFromOptions() {
	// Drop previous option-built items; keep only if re-bound via Add.
	g.Items = g.Items[:0]
	if g.root == nil {
		g.root = primitive.Row()
		g.root.Gap = DefaultCheckboxGap
		g.root.Wrap = true
	}
	if g.body == nil {
		g.root.ClearChildren()
		g.root.Wrap = true
		g.root.CrossAlign = core.CrossCenter
		g.root.Gap = DefaultCheckboxGap
	}
	for _, opt := range g.Options {
		opt := opt
		cb := NewCheckbox(opt.Label)
		cb.Value = opt.Value
		if opt.Value == "" {
			cb.Value = opt.Label
		}
		cb.Title = opt.Title
		cb.Disabled = opt.Disabled
		cb.group = g
		if g.Face != nil {
			cb.SetFace(g.Face)
		}
		if g.Theme != nil {
			cb.SetTheme(g.Theme)
		}
		cb.Checked = g.contains(cb.Value)
		cb.Root.SetDisabled(cb.Disabled || g.Disabled)
		cb.applyChrome()
		g.Items = append(g.Items, cb)
		if g.body == nil {
			g.root.AddChild(cb.Node())
		}
	}
	g.applyA11y()
}

func (g *CheckboxGroup) syncItemsFromValue() {
	for _, it := range g.Items {
		want := g.contains(it.Value)
		if it.Checked != want || it.Indeterminate {
			it.Checked = want
			it.Indeterminate = false
			it.applyChrome()
			it.applyA11y()
		}
	}
}

func (g *CheckboxGroup) contains(v string) bool {
	for _, x := range g.Value {
		if x == v {
			return true
		}
	}
	return false
}

func (g *CheckboxGroup) toggleOption(cb *Checkbox) {
	if g.Disabled || cb == nil || cb.Disabled {
		return
	}
	val := cb.Value
	if val == "" {
		val = cb.Label
	}
	next := append([]string(nil), g.Value...)
	idx := -1
	for i, x := range next {
		if x == val {
			idx = i
			break
		}
	}
	if idx >= 0 {
		next = append(next[:idx], next[idx+1:]...)
	} else {
		next = append(next, val)
	}
	// Keep option order (antd Group sort by options index).
	next = g.orderValues(next)

	if !g.Controlled {
		g.Value = next
		g.syncItemsFromValue()
	}
	// Fire item-level onChange too (antd option.onChange).
	if cb.OnChange != nil {
		cb.OnChange(idx < 0) // true if newly checked
	}
	if g.OnChange != nil {
		g.OnChange(append([]string(nil), next...))
	}
}

func (g *CheckboxGroup) orderValues(vals []string) []string {
	if len(g.Options) == 0 {
		// Preserve registration order of Items.
		out := make([]string, 0, len(vals))
		for _, it := range g.Items {
			v := it.Value
			if v == "" {
				v = it.Label
			}
			for _, x := range vals {
				if x == v {
					out = append(out, v)
					break
				}
			}
		}
		// Append any stray values not in items.
		for _, x := range vals {
			found := false
			for _, y := range out {
				if y == x {
					found = true
					break
				}
			}
			if !found {
				out = append(out, x)
			}
		}
		return out
	}
	out := make([]string, 0, len(vals))
	for _, opt := range g.Options {
		v := opt.Value
		if v == "" {
			v = opt.Label
		}
		for _, x := range vals {
			if x == v {
				out = append(out, v)
				break
			}
		}
	}
	return out
}
