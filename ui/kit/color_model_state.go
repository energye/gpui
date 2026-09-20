package kit

import (
	"sync"

	"github.com/energye/gpui/ui/kit/internal/scope"
)

// ColorModelFocus names the keyboard trap target.
type ColorModelFocus string

const (
	ColorModelFocusNone    ColorModelFocus = ""
	ColorModelFocusTrigger ColorModelFocus = "trigger"
	ColorModelFocusSV      ColorModelFocus = "sv"
	ColorModelFocusHue     ColorModelFocus = "hue"
	ColorModelFocusAlpha   ColorModelFocus = "alpha"
	ColorModelFocusStop    ColorModelFocus = "stop"
	ColorModelFocusClose   ColorModelFocus = "close"
)

// ColorModelInstance owns the color value plus the controlled panel
// (open/format/mode) for one model mount.
type ColorModelInstance struct {
	props    ColorModelProps
	ctx      scope.Ctx
	mu       sync.Mutex
	value    ColorModelValue
	format   ColorModelFormat
	mode     ColorModelMode
	open     bool
	focused  ColorModelFocus
	active   int
	dragging bool

	ctlValue  bool
	ctlOpen   bool
	ctlFormat bool
	ctlMode   bool

	onChange       func(v ColorModelValue, css string)
	onComplete     func(v ColorModelValue)
	onFormatChange func(f ColorModelFormat)
	onOpenChange   func(open bool)
	onClear        func()
	mounted        bool
}

func newColorModelInstance(ctx scope.Ctx, props ColorModelProps) *ColorModelInstance {
	in := &ColorModelInstance{props: props, ctx: ctx.Normalize()}
	if props.Value != nil {
		in.ctlValue = true
		in.value = *props.Value
	} else if props.DefaultValueSet {
		in.value = props.DefaultValue
	} else {
		in.value = ColorModelValue{Cleared: true}
	}
	in.value.Stops = NormalizeColorModelStops(in.value.Stops)
	if props.OpenSet {
		in.ctlOpen = true
		in.open = props.Open
	} else {
		in.open = props.DefaultOpen
	}
	if props.FormatSet {
		in.ctlFormat = true
		in.format = props.Format
	} else {
		in.format = ResolveColorModelFormat(props, "")
	}
	if props.ModeSet {
		in.ctlMode = true
		in.mode = props.Mode
	} else {
		in.mode = ResolveColorModelMode(props, "")
	}
	in.syncModeLocked()
	return in
}

// syncModeLocked aligns the value shape with the current mode.
func (in *ColorModelInstance) syncModeLocked() {
	if in.mode == ColorModelModeGradient && !in.value.Gradient {
		base := in.value.Single
		in.value = ColorModelValue{
			Gradient: true,
			Cleared:  in.value.Cleared,
			Stops: []ColorModelStop{
				{Color: base, Percent: 0},
				{Color: base, Percent: 100},
			},
		}
	}
	if in.mode == ColorModelModeSingle && in.value.Gradient {
		single := in.value.Single
		if len(in.value.Stops) > 0 {
			idx := in.active
			if idx < 0 || idx >= len(in.value.Stops) {
				idx = 0
			}
			single = in.value.Stops[idx].Color
		}
		in.value = ColorModelValue{Single: single, Cleared: in.value.Cleared}
	}
	if in.active < 0 {
		in.active = 0
	}
	if in.value.Gradient && in.active >= len(in.value.Stops) {
		in.active = len(in.value.Stops) - 1
	}
	if in.active < 0 {
		in.active = 0
	}
}

// Mount marks the host live.
func (in *ColorModelInstance) Mount(ctx scope.Ctx) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.mounted = true
}

// Update swaps props/ctx snapshots. Controlled value/open/format/mode win.
func (in *ColorModelInstance) Update(ctx scope.Ctx, next ColorModelProps) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.props = next
	if next.Value != nil {
		in.ctlValue = true
		in.value = *next.Value
		in.value.Stops = NormalizeColorModelStops(in.value.Stops)
	} else {
		in.ctlValue = false
	}
	if next.OpenSet {
		in.ctlOpen = true
		in.open = next.Open
	} else {
		in.ctlOpen = false
	}
	if next.FormatSet {
		in.ctlFormat = true
		in.format = next.Format
	} else {
		in.ctlFormat = false
		if in.format == "" {
			in.format = ResolveColorModelFormat(next, "")
		}
	}
	if next.ModeSet {
		in.ctlMode = true
		in.mode = next.Mode
	} else {
		in.ctlMode = false
	}
	in.syncModeLocked()
}

// Unmount marks the host dead.
func (in *ColorModelInstance) Unmount() {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.mounted = false
}

// SetState runs f under the lock (sole state mutation gate).
func (in *ColorModelInstance) SetState(f func(*ColorModelInstance)) {
	if in == nil || f == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	f(in)
}

// Disabled reports the OR of subtree and widget disable.
func (in *ColorModelInstance) Disabled() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return scope.DisabledOr(in.ctx.Disabled, in.props.Disabled)
}

// EffectiveAlphaForcings reports the alpha slider visibility.
func (in *ColorModelInstance) HasAlphaSlider() bool {
	if in == nil {
		return true
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return !in.props.DisabledAlpha
}

// Value returns a copy of the current value.
func (in *ColorModelInstance) Value() ColorModelValue {
	if in == nil {
		return ColorModelValue{Cleared: true}
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.value
}

// Format returns the current format.
func (in *ColorModelInstance) Format() ColorModelFormat {
	if in == nil {
		return ColorModelFormatHex
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.format == "" {
		return ColorModelFormatHex
	}
	return in.format
}

// Mode returns the current value mode.
func (in *ColorModelInstance) Mode() ColorModelMode {
	if in == nil {
		return ColorModelModeSingle
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.mode == "" {
		return ColorModelModeSingle
	}
	return in.mode
}

// IsOpen reports the popup visibility.
func (in *ColorModelInstance) IsOpen() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.open
}

// Focused reports the keyboard trap target.
func (in *ColorModelInstance) Focused() ColorModelFocus {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.focused
}

// ActiveStop reports the edited gradient stop.
func (in *ColorModelInstance) ActiveStop() int {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.active
}

// SetOnChange registers the drag/change callback.
func (in *ColorModelInstance) SetOnChange(fn func(v ColorModelValue, css string)) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onChange = fn
}

// SetOnComplete registers the release/commit callback.
func (in *ColorModelInstance) SetOnComplete(fn func(v ColorModelValue)) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onComplete = fn
}

// SetOnFormatChange registers the format callback.
func (in *ColorModelInstance) SetOnFormatChange(fn func(f ColorModelFormat)) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onFormatChange = fn
}

// SetOnOpenChange registers the visibility callback.
func (in *ColorModelInstance) SetOnOpenChange(fn func(open bool)) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onOpenChange = fn
}

// SetOnClear registers the clear callback.
func (in *ColorModelInstance) SetOnClear(fn func()) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onClear = fn
}

// fireChangeLocked stores the value (uncontrolled) and notifies.
func (in *ColorModelInstance) fireChangeLocked(v ColorModelValue) {
	if !in.ctlValue {
		in.value = v
	}
	cb := in.onChange
	css := v.ToCssString()
	out := v
	in.mu.Unlock()
	if cb != nil {
		cb(out, css)
	}
	in.mu.Lock()
}

// applyColorLocked writes one single color through the gradient-aware path.
func (in *ColorModelInstance) applyColorLocked(c ColorModelColor) {
	if in.props.DisabledAlpha {
		c.A = 1
		r, g, bl := ColorModelHSBToRGB(c.H, c.S, c.B)
		c.R, c.G, c.BL = r, g, bl
	}
	v := in.value
	v.Cleared = false
	if v.Gradient && len(v.Stops) > 0 {
		idx := in.active
		if idx < 0 || idx >= len(v.Stops) {
			idx = 0
		}
		v.Stops[idx].Color = c
	} else {
		v.Single = c
	}
	in.fireChangeLocked(v)
}

// SetHSB changes the color and fires onChange (CP-S1).
func (in *ColorModelInstance) SetHSB(h, s, b, a float64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	if in.props.DisabledAlpha {
		a = 1
	}
	in.applyColorLocked(NewColorModelColor(h, s, b, a))
	in.mu.Unlock()
	return true
}

// SetHex parses and applies one color string.
func (in *ColorModelInstance) SetHex(text string) bool {
	c, ok := ParseColorModelColor(text)
	if !ok {
		return false
	}
	return in.SetColor(c)
}

// SetColor applies one parsed color.
func (in *ColorModelInstance) SetColor(c ColorModelColor) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	in.applyColorLocked(c)
	in.mu.Unlock()
	return true
}

// BeginDrag marks a panel drag; EndDrag fires onComplete (CP-S2).
func (in *ColorModelInstance) BeginDrag() {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.dragging = true
}

// EndDrag ends a panel drag and fires onComplete.
func (in *ColorModelInstance) EndDrag() {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.dragging = false
	cb := in.onComplete
	out := in.value
	in.mu.Unlock()
	if cb != nil {
		cb(out)
	}
}

// CommitChange fires onComplete without a drag (CP-S2).
func (in *ColorModelInstance) CommitChange() {
	if in == nil {
		return
	}
	in.mu.Lock()
	cb := in.onComplete
	out := in.value
	in.mu.Unlock()
	if cb != nil {
		cb(out)
	}
}

// DragSV moves saturation/brightness on the 2D panel (0..1 each).
func (in *ColorModelInstance) DragSV(x, y float64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	cur := in.currentColorLocked()
	c := NewColorModelColor(cur.H, clamp01(x), 1-clamp01(y), cur.A)
	in.applyColorLocked(c)
	in.mu.Unlock()
	return true
}

// DragHue moves the hue slider (0..1 maps 0..360).
func (in *ColorModelInstance) DragHue(t float64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	cur := in.currentColorLocked()
	in.applyColorLocked(NewColorModelColor(clamp01(t)*360, cur.S, cur.B, cur.A))
	in.mu.Unlock()
	return true
}

// DragAlpha moves the alpha slider (0..1).
func (in *ColorModelInstance) DragAlpha(t float64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) || in.props.DisabledAlpha {
		in.mu.Unlock()
		return false
	}
	cur := in.currentColorLocked()
	in.applyColorLocked(NewColorModelColor(cur.H, cur.S, cur.B, clamp01(t)))
	in.mu.Unlock()
	return true
}

func (in *ColorModelInstance) currentColorLocked() ColorModelColor {
	if in.value.Gradient && len(in.value.Stops) > 0 {
		idx := in.active
		if idx < 0 || idx >= len(in.value.Stops) {
			idx = 0
		}
		return in.value.Stops[idx].Color
	}
	return in.value.Single
}

// CurrentColor returns a copy of the edited color.
func (in *ColorModelInstance) CurrentColor() ColorModelColor {
	if in == nil {
		return ColorModelColor{A: 1}
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.currentColorLocked()
}

// Clear empties the value when allowClear (CP-S4).
func (in *ColorModelInstance) Clear() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if !in.props.AllowClear || scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	v := in.value
	v.Cleared = true
	if !in.ctlValue {
		in.value = v
	}
	cbClear := in.onClear
	cbChange := in.onChange
	css := v.ToCssString()
	out := v
	in.mu.Unlock()
	if cbClear != nil {
		cbClear()
	}
	if cbChange != nil {
		cbChange(out, css)
	}
	return true
}

// SetValue drives controlled value from outside without callbacks (CP-S8).
func (in *ColorModelInstance) SetValue(v ColorModelValue) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctlValue = true
	v.Stops = NormalizeColorModelStops(v.Stops)
	in.value = v
	if in.value.Gradient && in.active >= len(in.value.Stops) {
		in.active = 0
	}
}

// SetFormat switches the display format unless disabledFormat.
func (in *ColorModelInstance) SetFormat(f ColorModelFormat) bool {
	if in == nil || f == "" {
		return false
	}
	in.mu.Lock()
	if in.props.DisabledFormat {
		in.mu.Unlock()
		return false
	}
	if !in.ctlFormat {
		in.format = f
	}
	cb := in.onFormatChange
	in.mu.Unlock()
	if cb != nil {
		cb(f)
	}
	return true
}

// SetMode switches single/gradient when allowed, converting the value.
func (in *ColorModelInstance) SetMode(m ColorModelMode) bool {
	if in == nil || m == "" {
		return false
	}
	in.mu.Lock()
	if !ColorModelModeAllowed(in.props, m) {
		in.mu.Unlock()
		return false
	}
	if !in.ctlMode {
		in.mode = m
	}
	in.syncModeLocked()
	v := in.value
	cb := in.onChange
	css := v.ToCssString()
	out := v
	in.mu.Unlock()
	if cb != nil {
		cb(out, css)
	}
	return true
}

// SelectPreset applies one preset entry.
func (in *ColorModelInstance) SelectPreset(group, index int) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	if group < 0 || group >= len(in.props.Presets) {
		in.mu.Unlock()
		return false
	}
	colors := in.props.Presets[group].Colors
	if index < 0 || index >= len(colors) {
		in.mu.Unlock()
		return false
	}
	v := colors[index]
	v.Stops = NormalizeColorModelStops(v.Stops)
	if v.Gradient {
		if !in.ctlMode && ColorModelModeAllowed(in.props, ColorModelModeGradient) {
			in.mode = ColorModelModeGradient
		}
	} else {
		if !in.ctlMode && ColorModelModeAllowed(in.props, ColorModelModeSingle) {
			in.mode = ColorModelModeSingle
		}
	}
	in.active = 0
	in.fireChangeLocked(v)
	in.mu.Unlock()
	return true
}

// AddStop inserts a stop copying the active color (gradient only).
func (in *ColorModelInstance) AddStop(percent float64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	if !in.value.Gradient {
		in.mu.Unlock()
		return false
	}
	v := in.value
	v.Stops = NormalizeColorModelStops(append(v.Stops, ColorModelStop{Color: in.currentColorLocked(), Percent: percent}))
	in.active = len(v.Stops) - 1
	in.fireChangeLocked(v)
	in.mu.Unlock()
	return true
}

// MoveStop drags one stop percent (fires onChange, not complete).
func (in *ColorModelInstance) MoveStop(i int, percent float64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	if !in.value.Gradient || i < 0 || i >= len(in.value.Stops) {
		in.mu.Unlock()
		return false
	}
	v := in.value
	v.Stops[i].Percent = percent
	v.Stops = NormalizeColorModelStops(v.Stops)
	in.fireChangeLocked(v)
	in.mu.Unlock()
	return true
}

// RemoveStop deletes a non-terminal stop (Delete key semantics).
func (in *ColorModelInstance) RemoveStop(i int) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	if !in.value.Gradient || len(in.value.Stops) <= 2 {
		in.mu.Unlock()
		return false
	}
	if i <= 0 || i >= len(in.value.Stops)-1 {
		in.mu.Unlock()
		return false
	}
	v := in.value
	v.Stops = append(v.Stops[:i], v.Stops[i+1:]...)
	if in.active >= len(v.Stops) {
		in.active = len(v.Stops) - 1
	}
	in.fireChangeLocked(v)
	in.mu.Unlock()
	return true
}

// SelectStop picks the edited stop handle.
func (in *ColorModelInstance) SelectStop(i int) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.value.Gradient || i < 0 || i >= len(in.value.Stops) {
		return false
	}
	in.active = i
	return true
}

// SetOpen drives controlled visibility from outside.
func (in *ColorModelInstance) SetOpen(open bool) {
	if in == nil {
		return
	}
	in.mu.Lock()
	in.ctlOpen = true
	if in.open == open {
		in.mu.Unlock()
		return
	}
	in.open = open
	cb := in.onOpenChange
	in.mu.Unlock()
	if cb != nil {
		cb(open)
	}
}

// ToggleOpen flips the popup unless disabled (CP-S9).
func (in *ColorModelInstance) ToggleOpen() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return false
	}
	if !in.ctlOpen {
		in.open = !in.open
	}
	open := in.open
	cb := in.onOpenChange
	in.mu.Unlock()
	if cb != nil {
		cb(open)
	}
	return true
}

// PressEsc closes the popup.
func (in *ColorModelInstance) PressEsc() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if !in.open {
		in.mu.Unlock()
		return false
	}
	if !in.ctlOpen {
		in.open = false
	}
	cb := in.onOpenChange
	in.mu.Unlock()
	if cb != nil {
		cb(false)
	}
	return true
}

// PressTriggerKey opens/closes with Space/Enter and enters the panel.
func (in *ColorModelInstance) PressTriggerKey() bool {
	if in == nil {
		return false
	}
	if !in.ToggleOpen() {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.open {
		in.focused = ColorModelFocusSV
	} else {
		in.focused = ColorModelFocusTrigger
	}
	return true
}

// Focus moves the keyboard trap target.
func (in *ColorModelInstance) Focus(f ColorModelFocus) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.focused = f
}

// PressArrow steps the focused handle by 1% (CP keyboard rule).
func (in *ColorModelInstance) PressArrow(dir string) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) || !in.open {
		in.mu.Unlock()
		return false
	}
	delta := 0.01
	if dir == "left" || dir == "down" {
		delta = -0.01
	} else if dir != "right" && dir != "up" {
		in.mu.Unlock()
		return false
	}
	switch in.focused {
	case ColorModelFocusHue:
		cur := in.currentColorLocked()
		in.applyColorLocked(NewColorModelColor(cur.H+delta*360, cur.S, cur.B, cur.A))
	case ColorModelFocusAlpha:
		if in.props.DisabledAlpha {
			in.mu.Unlock()
			return false
		}
		cur := in.currentColorLocked()
		in.applyColorLocked(NewColorModelColor(cur.H, cur.S, cur.B, cur.A+delta))
	case ColorModelFocusStop:
		if !in.value.Gradient || len(in.value.Stops) == 0 {
			in.mu.Unlock()
			return false
		}
		v := in.value
		idx := in.active
		v.Stops[idx].Percent += delta * 100
		v.Stops = NormalizeColorModelStops(v.Stops)
		in.fireChangeLocked(v)
	case ColorModelFocusSV:
		cur := in.currentColorLocked()
		in.applyColorLocked(NewColorModelColor(cur.H, cur.S+delta, cur.B, cur.A))
	default:
		in.mu.Unlock()
		return false
	}
	in.mu.Unlock()
	return true
}

// DisplayText renders the trigger text for the current format (CP-S7).
func (in *ColorModelInstance) DisplayText() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.value.Cleared {
		return ""
	}
	if in.props.ShowTextRender != nil {
		return in.props.ShowTextRender(in.value)
	}
	if !in.props.ShowText {
		return ""
	}
	c := in.currentColorLocked()
	switch in.format {
	case ColorModelFormatRGB:
		return c.ToRgbString()
	case ColorModelFormatHSB:
		return c.ToHsbString()
	default:
		return c.ToHexString()
	}
}

// RenderPanel runs the panelRender hook or returns the default desc.
func (in *ColorModelInstance) RenderPanel() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	desc := "color panel " + string(in.mode)
	if in.props.PanelRender != nil {
		return in.props.PanelRender(desc, true, len(in.props.Presets) > 0)
	}
	return desc
}

// HolderContent renders static-call holder copy through Ctx.
func (in *ColorModelInstance) HolderContent() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	ctx := in.ctx
	in.mu.Unlock()
	if ctx.HolderRender == nil {
		return "color holder"
	}
	return ctx.HolderRender("color")
}
