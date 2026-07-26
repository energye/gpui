package kit

import (
	"fmt"
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design ColorPicker defaults — components/color-picker/style + docs/antd/color-picker.md §6.2.
const (
	DefaultColorPickerWidth       = 234.0
	DefaultColorPickerSliderH     = 8.0
	DefaultColorPickerHandler     = 16.0
	DefaultColorPickerHandlerSM   = 12.0
	DefaultColorPickerGap         = 4.0
	DefaultColorPickerPanelPad    = 8.0
	DefaultColorPickerPanelRadius = 8.0  // borderRadiusLG
	DefaultColorPickerBlockSM     = 16.0 // controlHeightXS fallback
	DefaultColorPickerFocusOutset = 1.5
	DefaultColorPickerPrimaryHex  = "#1677ff"
)

// ColorFormat is antd format: hex | rgb | hsb.
type ColorFormat int

const (
	ColorFormatHex ColorFormat = iota // default
	ColorFormatRGB
	ColorFormatHSB
)

func (f ColorFormat) String() string {
	switch f {
	case ColorFormatRGB:
		return "rgb"
	case ColorFormatHSB:
		return "hsb"
	default:
		return "hex"
	}
}

// ColorMode is antd mode: single | gradient.
type ColorMode int

const (
	ColorModeSingle ColorMode = iota // default
	ColorModeGradient
)

func (m ColorMode) String() string {
	if m == ColorModeGradient {
		return "gradient"
	}
	return "single"
}

// ColorTrigger is antd trigger: click | hover.
type ColorTrigger int

const (
	ColorTriggerClick ColorTrigger = iota // default (antd ColorPicker)
	ColorTriggerHover
)

// ColorPlacement is popup placement (antd placement).
type ColorPlacement int

const (
	ColorBottomLeft ColorPlacement = iota // default
	ColorBottomRight
	ColorTopLeft
	ColorTopRight
)

// ColorStop is one gradient stop (antd LineGradientType entry).
type ColorStop struct {
	Color   render.RGBA
	Percent float64 // 0..100
}

// Color is the product color value (antd AggregationColor subset).
// docs/antd/color-picker.md §6.10
type Color struct {
	RGBA    render.RGBA
	Cleared bool
	Stops   []ColorStop // non-empty → gradient (when mode allows)
}

// ColorFromHex builds an opaque Color from a hex string (empty → cleared).
func ColorFromHex(hex string) Color {
	hex = strings.TrimSpace(hex)
	if hex == "" {
		return Color{Cleared: true, RGBA: render.RGBA{A: 0}}
	}
	c := render.Hex(hex)
	return Color{RGBA: c}
}

// ColorFromRGBA builds a Color from components.
func ColorFromRGBA(c render.RGBA) Color {
	if c.A <= 0 && c.R == 0 && c.G == 0 && c.B == 0 {
		// distinguish intentional black from zero-value: treat A=0 as cleared only when all zero.
		return Color{RGBA: c, Cleared: c.A == 0 && c.R == 0 && c.G == 0 && c.B == 0}
	}
	return Color{RGBA: c}
}

// ColorGradient builds a gradient Color from stops (first stop fills RGBA).
func ColorGradient(stops ...ColorStop) Color {
	if len(stops) == 0 {
		return Color{Cleared: true}
	}
	out := Color{
		RGBA:  stops[0].Color,
		Stops: append([]ColorStop(nil), stops...),
	}
	return out
}

// IsEmpty reports cleared / no color.
func (c Color) IsEmpty() bool {
	return c.Cleared
}

// IsGradient reports multi-stop gradient value.
func (c Color) IsGradient() bool {
	return !c.Cleared && len(c.Stops) > 1
}

// ToHexString returns #rrggbb or #rrggbbaa when alpha < 1.
func (c Color) ToHexString() string {
	if c.Cleared {
		return ""
	}
	r := clampByte(c.RGBA.R)
	g := clampByte(c.RGBA.G)
	b := clampByte(c.RGBA.B)
	if c.RGBA.A < 1-1e-6 {
		a := clampByte(c.RGBA.A)
		return fmt.Sprintf("#%02x%02x%02x%02x", r, g, b, a)
	}
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// ToRgbString returns rgb(...) or rgba(...).
func (c Color) ToRgbString() string {
	if c.Cleared {
		return ""
	}
	r := clampByte(c.RGBA.R)
	g := clampByte(c.RGBA.G)
	b := clampByte(c.RGBA.B)
	if c.RGBA.A < 1-1e-6 {
		return fmt.Sprintf("rgba(%d, %d, %d, %.2f)", r, g, b, c.RGBA.A)
	}
	return fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)
}

// ToHsbString returns hsb(...) or hsba(...).
func (c Color) ToHsbString() string {
	if c.Cleared {
		return ""
	}
	h, s, v := rgbToHSB(c.RGBA.R, c.RGBA.G, c.RGBA.B)
	if c.RGBA.A < 1-1e-6 {
		return fmt.Sprintf("hsba(%d, %d%%, %d%%, %.2f)", int(math.Round(h)), int(math.Round(s*100)), int(math.Round(v*100)), c.RGBA.A)
	}
	return fmt.Sprintf("hsb(%d, %d%%, %d%%)", int(math.Round(h)), int(math.Round(s*100)), int(math.Round(v*100)))
}

// ToCssString returns the css form for the given format.
func (c Color) ToCssString(format ColorFormat) string {
	switch format {
	case ColorFormatRGB:
		return c.ToRgbString()
	case ColorFormatHSB:
		return c.ToHsbString()
	default:
		return c.ToHexString()
	}
}

func clampByte(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 255
	}
	return int(math.Round(v * 255))
}

// ColorPicker is Ant Design ColorPicker — trigger + anchored panel.
//
//	Column (Wrap)
//	  ├─ Pressable trigger (or TriggerNode)
//	  │    └─ Decorated [ color-block · text? · clear? ]
//	  └─ AnchoredPopup
//	       └─ panel (mode · SV · hue · alpha? · format text)
//
// Product contract: docs/antd/color-picker.md §6 (P0 DoD).
type ColorPicker struct {
	Wrap  *primitive.Flex
	Root  *primitive.Pressable // trigger shell
	decor *primitive.Decorated
	block *colorBlockHost
	text  *primitive.Text
	clear *primitive.Pressable
	popup *primitive.AnchoredPopup
	panel *primitive.Decorated

	// Product fields (§6.10).
	Value          Color
	DefaultValue   Color
	Size           InputSize
	Disabled       bool
	AllowClear     bool
	DisabledAlpha  bool
	ShowText       bool
	ShowTextRender func(Color) string
	Format         ColorFormat
	Mode           ColorMode
	Modes          []ColorMode // empty → [Mode]
	Open           bool
	Placement      ColorPlacement
	Trigger        ColorTrigger
	TriggerNode    core.Node
	AriaLabel      string
	Face           text.Face
	Theme          *core.Theme
	Viewport       core.Size

	OnChange         func(c Color, css string)
	OnChangeComplete func(c Color)
	OnOpenChange     func(open bool)
	OnClear          func()
	OnFormatChange   func(f ColorFormat) // P1 hook kept for API completeness

	// HSB working state for the active (single / gradient stop) color.
	hue, sat, bri, alpha float64
	activeStop           int // gradient stop index

	// openControlled: SetOpen used (antd open prop).
	openControlled bool
	defaultOpen    bool
	defaultOpenSet bool
	appliedDefault bool
	appliedDefVal  bool
	defaultValSet  bool

	// panel hosts
	palette   *cpPaletteHost
	hueBar    *cpSliderHost
	alphaBar  *cpSliderHost
	modeRow   *primitive.Flex
	formatLab *primitive.Text
}

// NewColorPicker creates a ColorPicker.
// Defaults (§6.10): middle, hex, single, click, bottomLeft, closed, value cleared.
func NewColorPicker() *ColorPicker {
	cp := &ColorPicker{
		Size:      InputMiddle,
		Format:    ColorFormatHex,
		Mode:      ColorModeSingle,
		Trigger:   ColorTriggerClick,
		Placement: ColorBottomLeft,
		Value:     Color{Cleared: true, RGBA: render.RGBA{A: 0}},
		alpha:     1,
	}
	cp.rebuild()
	return cp
}

// Node returns the composition root (trigger + popup host).

// ensureBuilt materializes the control tree if missing (#9).
func (cp *ColorPicker) ensureBuilt() {
	if cp == nil {
		return
	}
	if cp.Wrap == nil {
		cp.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (cp *ColorPicker) structureChange() {
	if cp == nil {
		return
	}
	cp.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (cp *ColorPicker) chromeChange() {
	if cp == nil {
		return
	}
	cp.ensureBuilt()
	cp.rebuild()
}

func (cp *ColorPicker) Node() core.Node {
	if cp == nil {
		return nil
	}
	cp.ensureBuilt()
	return cp.Wrap
}

// Popup returns the anchored popup (tests / advanced hosts).
func (cp *ColorPicker) Popup() *primitive.AnchoredPopup {
	if cp == nil {
		return nil
	}
	return cp.popup
}

// Panel returns the dropdown panel chrome (tests).
func (cp *ColorPicker) Panel() *primitive.Decorated {
	if cp == nil {
		return nil
	}
	return cp.panel
}

// TriggerShell returns the trigger pressable (tests / a11y).
func (cp *ColorPicker) TriggerShell() *primitive.Pressable {
	if cp == nil {
		return nil
	}
	return cp.Root
}

// IsOpen reports whether the panel is visible.
func (cp *ColorPicker) IsOpen() bool {
	return cp != nil && cp.Open
}

// GetValue returns the current color value.
func (cp *ColorPicker) GetValue() Color {
	if cp == nil {
		return Color{Cleared: true}
	}
	return cp.cloneValue(cp.Value)
}

// HasAlphaSlider reports whether the alpha slider is shown (CP-06).
func (cp *ColorPicker) HasAlphaSlider() bool {
	return cp != nil && !cp.DisabledAlpha
}

// DisplayText returns the current trigger text (showText).
func (cp *ColorPicker) DisplayText() string {
	if cp == nil {
		return ""
	}
	return cp.computeText()
}

// ---------------------------------------------------------------------------
// Setters — product API (§6.10)
// ---------------------------------------------------------------------------

// SetValue sets the color without firing OnChange (API write / controlled).
func (cp *ColorPicker) SetValue(v Color) {
	if cp == nil {
		return
	}
	cp.Value = cp.cloneValue(v)
	cp.syncHSBFromValue()
	cp.refreshChrome()
	if cp.Open {
		cp.refreshPanel()
	}
}

// SetDefaultValue seeds uncontrolled value when still cleared / unset.
func (cp *ColorPicker) SetDefaultValue(v Color) {
	if cp == nil {
		return
	}
	cp.DefaultValue = cp.cloneValue(v)
	cp.defaultValSet = true
	if !cp.appliedDefVal && cp.Value.Cleared {
		cp.appliedDefVal = true
		cp.Value = cp.cloneValue(v)
		cp.syncHSBFromValue()
		cp.refreshChrome()
	}
}

// SetHex is a convenience for SetValue(ColorFromHex).
func (cp *ColorPicker) SetHex(hex string) {
	cp.SetValue(ColorFromHex(hex))
}

// Clear resets to empty (fires OnChange / OnClear).
func (cp *ColorPicker) Clear() {
	if cp == nil || cp.Disabled {
		return
	}
	cp.Value = Color{Cleared: true, RGBA: render.RGBA{A: 0}}
	cp.alpha = 1
	cp.hue, cp.sat, cp.bri = 0, 0, 1
	cp.fireChange()
	if cp.OnClear != nil {
		cp.OnClear()
	}
	cp.refreshChrome()
	if cp.Open {
		cp.refreshPanel()
	}
}

// SetSize updates control height via Token.
func (cp *ColorPicker) SetSize(s InputSize) {
	if cp == nil {
		return
	}
	cp.Size = s
	cp.rebuild()
}

// SetDisabled toggles disabled (no open / no pick).
func (cp *ColorPicker) SetDisabled(d bool) {
	if cp == nil {
		return
	}
	cp.Disabled = d
	if cp.Root != nil {
		cp.Root.SetDisabled(d)
	}
	if d && cp.Open {
		cp.applyOpen(false, false)
	}
	cp.applyChrome()
	cp.applyA11y()
}

// SetAllowClear toggles clear affordance.
func (cp *ColorPicker) SetAllowClear(v bool) {
	if cp == nil {
		return
	}
	cp.AllowClear = v
	cp.rebuild()
}

// SetDisabledAlpha hides the alpha slider and forces A=1.
func (cp *ColorPicker) SetDisabledAlpha(v bool) {
	if cp == nil {
		return
	}
	cp.DisabledAlpha = v
	if v {
		cp.alpha = 1
		if !cp.Value.Cleared {
			cp.Value.RGBA.A = 1
			for i := range cp.Value.Stops {
				cp.Value.Stops[i].Color.A = 1
			}
		}
	}
	cp.rebuild()
}

// SetShowText toggles trigger text.
func (cp *ColorPicker) SetShowText(v bool) {
	if cp == nil {
		return
	}
	cp.ShowText = v
	cp.rebuild()
}

// SetShowTextRender sets a custom text renderer (antd showText function).
func (cp *ColorPicker) SetShowTextRender(fn func(Color) string) {
	if cp == nil {
		return
	}
	cp.ShowTextRender = fn
	if fn != nil {
		cp.ShowText = true
	}
	cp.refreshChrome()
}

// SetFormat sets the color format for css / showText.
func (cp *ColorPicker) SetFormat(f ColorFormat) {
	if cp == nil {
		return
	}
	if cp.Format == f {
		return
	}
	cp.Format = f
	if cp.OnFormatChange != nil {
		cp.OnFormatChange(f)
	}
	cp.refreshChrome()
	if cp.Open {
		cp.refreshPanel()
	}
}

// SetMode sets the active mode (single | gradient).
func (cp *ColorPicker) SetMode(m ColorMode) {
	if cp == nil {
		return
	}
	cp.Mode = m
	if m == ColorModeGradient && len(cp.Value.Stops) < 2 && !cp.Value.Cleared {
		// seed two stops from current solid
		c := cp.Value.RGBA
		cp.Value.Stops = []ColorStop{
			{Color: c, Percent: 0},
			{Color: c, Percent: 100},
		}
	}
	if m == ColorModeSingle && len(cp.Value.Stops) > 0 {
		// keep RGBA from active stop
		if cp.activeStop >= 0 && cp.activeStop < len(cp.Value.Stops) {
			cp.Value.RGBA = cp.Value.Stops[cp.activeStop].Color
		}
		cp.Value.Stops = nil
	}
	cp.syncHSBFromValue()
	cp.rebuild()
}

// SetModes sets available mode tabs (empty → only current Mode).
func (cp *ColorPicker) SetModes(modes ...ColorMode) {
	if cp == nil {
		return
	}
	cp.Modes = append([]ColorMode(nil), modes...)
	if len(modes) > 0 {
		found := false
		for _, m := range modes {
			if m == cp.Mode {
				found = true
				break
			}
		}
		if !found {
			cp.Mode = modes[0]
		}
	}
	cp.rebuild()
}

// SetPlacement sets popup placement.
func (cp *ColorPicker) SetPlacement(p ColorPlacement) {
	if cp == nil {
		return
	}
	cp.Placement = p
	if cp.popup != nil {
		cp.popup.Placement = mapColorPlacement(p)
		if cp.Open {
			cp.syncPopupGeometry()
		}
	}
}

// SetTrigger sets open trigger (click | hover).
func (cp *ColorPicker) SetTrigger(t ColorTrigger) {
	if cp == nil {
		return
	}
	cp.Trigger = t
	cp.wireTrigger()
}

// SetTriggerNode sets a custom trigger node (antd children); nil restores default.
func (cp *ColorPicker) SetTriggerNode(n core.Node) {
	if cp == nil {
		return
	}
	cp.TriggerNode = n
	cp.rebuild()
}

// SetOpen sets visibility and marks controlled (antd open prop).
func (cp *ColorPicker) SetOpen(open bool) {
	if cp == nil {
		return
	}
	cp.openControlled = true
	cp.applyOpen(open, false)
}

// SetDefaultOpen sets initial open for uncontrolled mode.
func (cp *ColorPicker) SetDefaultOpen(open bool) {
	if cp == nil || cp.openControlled {
		return
	}
	cp.defaultOpen = open
	cp.defaultOpenSet = true
	if !cp.appliedDefault {
		cp.appliedDefault = true
		cp.applyOpen(open, false)
	}
}

// SetOnChange sets the change callback.
func (cp *ColorPicker) SetOnChange(fn func(Color, string)) { cp.OnChange = fn }

// SetOnChangeComplete sets the complete callback.
func (cp *ColorPicker) SetOnChangeComplete(fn func(Color)) { cp.OnChangeComplete = fn }

// SetOnOpenChange sets the open callback.
func (cp *ColorPicker) SetOnOpenChange(fn func(bool)) { cp.OnOpenChange = fn }

// SetOnClear sets the clear callback.
func (cp *ColorPicker) SetOnClear(fn func()) { cp.OnClear = fn }

// SetAriaLabel sets the accessible name.
func (cp *ColorPicker) SetAriaLabel(name string) {
	if cp == nil {
		return
	}
	cp.AriaLabel = name
	cp.applyA11y()
}

// SetFace sets the label font.
func (cp *ColorPicker) SetFace(face text.Face) {
	if cp == nil {
		return
	}
	cp.Face = face
	if cp.text != nil {
		cp.text.Face = face
	}
	if cp.formatLab != nil {
		cp.formatLab.Face = face
	}
}

// SetTheme sets an explicit theme override.
func (cp *ColorPicker) SetTheme(th *core.Theme) {
	if cp == nil {
		return
	}
	cp.Theme = th
	cp.applyChrome()
	if cp.Open {
		cp.refreshPanel()
	}
}

// SetHSB programmatically updates the active color (fires OnChange).
func (cp *ColorPicker) SetHSB(h, s, b, a float64) {
	if cp == nil || cp.Disabled {
		return
	}
	cp.hue = clamp360(h)
	cp.sat = clamp01(s)
	cp.bri = clamp01(b)
	if cp.DisabledAlpha {
		cp.alpha = 1
	} else {
		cp.alpha = clamp01(a)
	}
	cp.applyHSBToValue()
	cp.fireChange()
	cp.refreshChrome()
	if cp.Open {
		cp.refreshPanel()
	}
}

// CommitChange fires OnChangeComplete (pointer-up path).
func (cp *ColorPicker) CommitChange() {
	if cp == nil || cp.Disabled {
		return
	}
	if cp.OnChangeComplete != nil {
		cp.OnChangeComplete(cp.cloneValue(cp.Value))
	}
}

// ---------------------------------------------------------------------------
// rebuild / chrome
// ---------------------------------------------------------------------------

func (cp *ColorPicker) rebuild() {
	if cp == nil {
		return
	}
	// apply default value once
	if !cp.appliedDefVal && cp.defaultValSet && cp.Value.Cleared {
		cp.Value = cp.cloneValue(cp.DefaultValue)
		cp.appliedDefVal = true
	}
	// default open once
	if !cp.appliedDefault && cp.defaultOpenSet && !cp.openControlled {
		cp.appliedDefault = true
		cp.Open = cp.defaultOpen
	}

	cp.syncHSBFromValue()
	th := cp.theme()
	h := cp.controlHeight()
	blockSz := cp.blockSize()
	radius := cp.triggerRadius(th)
	fontSz := th.SizeOr(core.TokenFontSize, 14)
	if cp.Size == InputSmall {
		fontSz = th.SizeOr(core.TokenFontSizeSM, 12)
	} else if cp.Size == InputLarge {
		fontSz = th.SizeOr(core.TokenFontSizeLG, 16)
	}
	pad := th.SizeOr(core.TokenControlPaddingInline, 11)
	if cp.Size == InputSmall {
		pad = th.SizeOr(core.TokenControlPaddingInlineSM, 7)
	}

	// color block
	if cp.block == nil {
		cp.block = newColorBlockHost(cp)
	}
	cp.block.cp = cp
	cp.block.size = blockSz

	// text
	var textNode core.Node
	if cp.ShowText {
		cp.text = primitive.NewText(cp.computeText())
		cp.text.FontSize = fontSz
		cp.text.Face = cp.Face
		cp.text.Color = th.Color(core.TokenColorText)
		textNode = cp.text
	} else {
		cp.text = nil
	}

	// clear
	var clearNode core.Node
	if cp.AllowClear && !cp.Value.Cleared && !cp.Disabled {
		x := primitive.NewIcon("close")
		x.Size = 10
		x.Color = th.Color(core.TokenColorTextSecondary)
		cp.clear = primitive.NewPressable(x)
		cp.clear.Focusable = false
		cp.clear.ShowFocusRing = false
		cp.clear.EnableRipple = false
		cp.clear.Base().Role = "button"
		cp.clear.Base().Label = "clear"
		cp.clear.Click = func() { cp.Clear() }
		clearNode = cp.clear
	} else {
		cp.clear = nil
	}

	rowKids := []core.Node{cp.block}
	if textNode != nil {
		rowKids = append(rowKids, textNode)
	}
	if clearNode != nil {
		rowKids = append(rowKids, clearNode)
	}
	row := primitive.Row(rowKids...)
	row.Gap = 8
	row.CrossAlign = core.CrossCenter

	cp.decor = primitive.NewDecorated(row)
	cp.decor.SkinType = TypeColorPicker
	cp.decor.Padding = primitive.Symmetric(pad, 0)
	cp.decor.Radius = radius
	cp.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	cp.decor.MinHeight = h
	cp.decor.Height = h
	// min width ≈ height when no text
	if !cp.ShowText {
		cp.decor.MinWidth = h
		cp.decor.Width = h
	} else {
		cp.decor.MinWidth = h + 40
		cp.decor.Width = 0
	}
	cp.decor.SetCenterContent(true)
	cp.applyChrome()

	// panel
	cp.buildPanel(th)

	if cp.popup == nil {
		cp.popup = primitive.NewAnchoredPopup(cp.panel)
	} else {
		cp.popup.Content = cp.panel
	}
	cp.popup.Placement = mapColorPlacement(cp.Placement)
	cp.popup.Gap = DefaultColorPickerGap
	cp.popup.DismissOnOutside = true
	cp.popup.OnDismiss = func() {
		cp.Open = false
		if cp.OnOpenChange != nil {
			cp.OnOpenChange(false)
		}
		cp.applyChrome()
	}

	// trigger
	var triggerChild core.Node = cp.decor
	if cp.TriggerNode != nil {
		triggerChild = cp.TriggerNode
	}
	if cp.Root == nil {
		cp.Root = primitive.NewPressable(triggerChild)
	} else {
		cp.Root.ClearChildren()
		cp.Root.AddChild(triggerChild)
	}
	cp.Root.Focusable = true
	cp.Root.ShowFocusRing = true
	cp.Root.FocusRingRadius = radius
	cp.Root.FocusRingOutset = DefaultColorPickerFocusOutset
	cp.Root.SetDisabled(cp.Disabled)
	cp.Root.OnStateChange = func() { cp.applyChrome() }
	cp.wireTrigger()
	cp.applyA11y()

	if cp.Wrap == nil {
		cp.Wrap = primitive.Column(cp.Root, cp.popup)
	} else {
		cp.Wrap.ClearChildren()
		cp.Wrap.AddChild(cp.Root)
		cp.Wrap.AddChild(cp.popup)
	}
	cp.Wrap.CrossAlign = core.CrossStart
	cp.Wrap.MainAlign = core.MainStart

	if cp.Open {
		cp.applyOpen(true, true)
	}
}

func (cp *ColorPicker) buildPanel(th *core.Theme) {
	w := DefaultColorPickerWidth
	pad := DefaultColorPickerPanelPad
	radius := th.SizeOr(core.TokenBorderRadiusLG, DefaultColorPickerPanelRadius)

	// mode tabs
	modes := cp.effectiveModes()
	var modeNode core.Node
	if len(modes) > 1 {
		cp.modeRow = primitive.Row()
		cp.modeRow.Gap = 4
		for _, m := range modes {
			m := m
			lab := m.String()
			btn := NewButton(lab)
			btn.SetSize(ButtonSmall)
			if m == cp.Mode {
				btn.SetType(ButtonPrimary)
			} else {
				btn.SetType(ButtonDefault)
			}
			btn.SetFace(cp.Face)
			btn.Theme = cp.Theme
			btn.SetOnClick(func() {
				if cp.Disabled {
					return
				}
				cp.SetMode(m)
			})
			cp.modeRow.AddChild(btn.Node())
		}
		modeNode = cp.modeRow
	}

	// SV palette
	if cp.palette == nil {
		cp.palette = &cpPaletteHost{cp: cp}
		cp.palette.Init(cp.palette)
		cp.palette.Hit = core.HitTarget
	}
	cp.palette.cp = cp
	cp.palette.w = w
	cp.palette.h = w * 0.7

	// hue slider
	if cp.hueBar == nil {
		cp.hueBar = newCPSliderHost(cp, cpSliderHue)
	}
	cp.hueBar.cp = cp
	cp.hueBar.kind = cpSliderHue
	cp.hueBar.w = w

	// alpha slider
	var alphaNode core.Node
	if !cp.DisabledAlpha {
		if cp.alphaBar == nil {
			cp.alphaBar = newCPSliderHost(cp, cpSliderAlpha)
		}
		cp.alphaBar.cp = cp
		cp.alphaBar.kind = cpSliderAlpha
		cp.alphaBar.w = w
		alphaNode = cp.alphaBar
	} else {
		cp.alphaBar = nil
	}

	// format label
	cp.formatLab = primitive.NewText(cp.Value.ToCssString(cp.Format))
	cp.formatLab.FontSize = th.SizeOr(core.TokenFontSize, 14)
	cp.formatLab.Face = cp.Face
	cp.formatLab.Color = th.Color(core.TokenColorText)

	// gradient stop strip (P0 simplified: two stops, click to select)
	var gradNode core.Node
	if cp.Mode == ColorModeGradient {
		gradNode = cp.buildGradientStrip(th, w)
	}

	colKids := []core.Node{}
	if modeNode != nil {
		colKids = append(colKids, modeNode)
	}
	if gradNode != nil {
		colKids = append(colKids, gradNode)
	}
	colKids = append(colKids, cp.palette, cp.hueBar)
	if alphaNode != nil {
		colKids = append(colKids, alphaNode)
	}
	colKids = append(colKids, cp.formatLab)

	body := primitive.Column(colKids...)
	body.Gap = 8
	body.CrossAlign = core.CrossStart
	body.MainAlign = core.MainStart

	cp.panel = primitive.NewDecorated(body)
	cp.panel.Padding = primitive.All(pad)
	cp.panel.Radius = radius
	cp.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	cp.panel.BorderColor = th.Color(core.TokenColorBorder)
	bg := th.Color(core.TokenColorBgContainer)
	if bg.A < 0.5 {
		bg = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	cp.panel.Background = bg
	cp.panel.MinWidth = w + pad*2
	cp.panel.Width = w + pad*2
}

func (cp *ColorPicker) buildGradientStrip(th *core.Theme, w float64) core.Node {
	row := primitive.Row()
	row.Gap = 6
	row.CrossAlign = core.CrossCenter
	stops := cp.Value.Stops
	if len(stops) < 2 {
		c := cp.Value.RGBA
		if cp.Value.Cleared {
			c = th.Color(core.TokenColorPrimary)
			if c.A < 0.5 {
				c = render.Hex(DefaultColorPickerPrimaryHex)
			}
		}
		stops = []ColorStop{{Color: c, Percent: 0}, {Color: c, Percent: 100}}
		cp.Value.Stops = stops
		cp.Value.Cleared = false
	}
	for i := range stops {
		i := i
		b := newColorBlockHost(cp)
		b.size = 20
		b.fixed = stops[i].Color
		b.fixedSet = true
		p := primitive.NewPressable(b)
		p.Focusable = false
		p.ShowFocusRing = i == cp.activeStop
		p.Click = func() {
			cp.activeStop = i
			cp.syncHSBFromValue()
			cp.refreshPanel()
		}
		row.AddChild(p)
	}
	return row
}

func (cp *ColorPicker) wireTrigger() {
	if cp == nil || cp.Root == nil {
		return
	}
	cp.Root.Click = nil
	// clear hover open hooks via OnStateChange only for hover mode
	switch cp.Trigger {
	case ColorTriggerHover:
		cp.Root.Click = nil
		cp.Root.OnStateChange = func() {
			cp.applyChrome()
			if cp.Disabled || cp.openControlled {
				return
			}
			if cp.Root.State.Hovered {
				cp.applyOpen(true, false)
			}
		}
	default: // click
		cp.Root.OnStateChange = func() { cp.applyChrome() }
		cp.Root.Click = func() {
			if cp.Disabled {
				return
			}
			if cp.openControlled {
				// notify only; parent owns open
				if cp.OnOpenChange != nil {
					cp.OnOpenChange(!cp.Open)
				}
				return
			}
			cp.applyOpen(!cp.Open, false)
		}
	}
}

func (cp *ColorPicker) applyOpen(open bool, skipCallback bool) {
	if cp == nil {
		return
	}
	if cp.Disabled && open {
		return
	}
	prev := cp.Open
	cp.Open = open
	if cp.popup != nil {
		if open {
			cp.refreshPanel()
			cp.syncPopupGeometry()
			cp.popup.SetOpen(true)
		} else {
			cp.popup.SetOpen(false)
		}
	}
	cp.applyChrome()
	if !skipCallback && prev != open && cp.OnOpenChange != nil {
		cp.OnOpenChange(open)
	}
}

func (cp *ColorPicker) syncPopupGeometry() {
	if cp == nil || cp.popup == nil || cp.Root == nil {
		return
	}
	cp.popup.UpdateAnchorFromNode(cp.Root)
	if cp.Viewport.Width > 0 {
		cp.popup.Viewport = cp.Viewport
	}
}

func (cp *ColorPicker) applyChrome() {
	if cp == nil || cp.decor == nil {
		return
	}
	th := cp.theme()
	border := th.Color(core.TokenColorBorder)
	bg := th.Color(core.TokenColorBgContainer)
	if cp.Disabled {
		bg = th.Color(core.TokenColorDisabledBg)
		border = th.Color(core.TokenColorBorder)
		if cp.text != nil {
			cp.text.Color = th.Color(core.TokenColorDisabledText)
		}
	} else {
		if cp.Root != nil && (cp.Root.State.Hovered || cp.Open) {
			border = th.Color(core.TokenColorPrimaryHover)
			if border.A < 0.3 {
				border = th.Color(core.TokenColorPrimary)
			}
		}
		if cp.Root != nil && cp.Root.State.Focused {
			border = th.Color(core.TokenColorPrimary)
		}
		if cp.text != nil {
			cp.text.Color = th.Color(core.TokenColorText)
		}
	}
	cp.decor.Background = bg
	cp.decor.BorderColor = border
	if cp.block != nil {
		cp.block.MarkNeedsPaint()
	}
}

func (cp *ColorPicker) applyA11y() {
	if cp == nil || cp.Root == nil {
		return
	}
	cp.Root.Base().Role = "combobox"
	label := cp.AriaLabel
	if label == "" {
		if !cp.Value.Cleared {
			label = "ColorPicker " + cp.Value.ToHexString()
		} else {
			label = "ColorPicker"
		}
	}
	cp.Root.Base().Label = label
}

func (cp *ColorPicker) refreshChrome() {
	if cp == nil {
		return
	}
	if cp.text != nil {
		cp.text.SetValue(cp.computeText())
	}
	// clear affordance depends on value
	if cp.AllowClear {
		// rebuild only if clear visibility flips
		hasClear := cp.clear != nil
		wantClear := !cp.Value.Cleared && !cp.Disabled
		if hasClear != wantClear {
			cp.rebuild()
			return
		}
	}
	cp.applyChrome()
	cp.applyA11y()
	if cp.block != nil {
		cp.block.MarkNeedsPaint()
	}
}

func (cp *ColorPicker) refreshPanel() {
	if cp == nil || cp.panel == nil {
		return
	}
	th := cp.theme()
	if cp.formatLab != nil {
		cp.formatLab.SetValue(cp.Value.ToCssString(cp.Format))
		cp.formatLab.Color = th.Color(core.TokenColorText)
	}
	if cp.palette != nil {
		cp.palette.MarkNeedsPaint()
	}
	if cp.hueBar != nil {
		cp.hueBar.MarkNeedsPaint()
	}
	if cp.alphaBar != nil {
		cp.alphaBar.MarkNeedsPaint()
	}
}

func (cp *ColorPicker) computeText() string {
	if cp == nil {
		return ""
	}
	if cp.ShowTextRender != nil {
		return cp.ShowTextRender(cp.cloneValue(cp.Value))
	}
	if cp.Value.Cleared {
		return ""
	}
	return cp.Value.ToCssString(cp.Format)
}

func (cp *ColorPicker) theme() *core.Theme {
	var n core.Node
	if cp.Wrap != nil {
		n = cp.Wrap
	}
	return themeOf(cp.Theme, n)
}

func (cp *ColorPicker) controlHeight() float64 {
	th := cp.theme()
	switch cp.Size {
	case InputSmall:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	case InputLarge:
		return th.SizeOr(core.TokenControlHeightLG, 40)
	default:
		return th.SizeOr(core.TokenControlHeight, 32)
	}
}

func (cp *ColorPicker) blockSize() float64 {
	th := cp.theme()
	switch cp.Size {
	case InputSmall:
		return DefaultColorPickerBlockSM
	case InputLarge:
		return th.SizeOr(core.TokenControlHeight, 32)
	default:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	}
}

func (cp *ColorPicker) triggerRadius(th *core.Theme) float64 {
	switch cp.Size {
	case InputSmall:
		return th.SizeOr(core.TokenBorderRadiusSM, 4)
	case InputLarge:
		return th.SizeOr(core.TokenBorderRadiusLG, 8)
	default:
		return th.SizeOr(core.TokenBorderRadius, 6)
	}
}

func (cp *ColorPicker) effectiveModes() []ColorMode {
	if len(cp.Modes) > 0 {
		return cp.Modes
	}
	return []ColorMode{cp.Mode}
}

func (cp *ColorPicker) cloneValue(v Color) Color {
	out := v
	if len(v.Stops) > 0 {
		out.Stops = append([]ColorStop(nil), v.Stops...)
	}
	return out
}

func (cp *ColorPicker) syncHSBFromValue() {
	if cp == nil {
		return
	}
	c := cp.Value.RGBA
	if cp.Mode == ColorModeGradient && len(cp.Value.Stops) > 0 {
		if cp.activeStop < 0 || cp.activeStop >= len(cp.Value.Stops) {
			cp.activeStop = 0
		}
		c = cp.Value.Stops[cp.activeStop].Color
	}
	if cp.Value.Cleared {
		cp.hue, cp.sat, cp.bri = 0, 0, 1
		cp.alpha = 1
		return
	}
	cp.hue, cp.sat, cp.bri = rgbToHSB(c.R, c.G, c.B)
	cp.alpha = c.A
	if cp.alpha <= 0 {
		cp.alpha = 1
	}
	if cp.DisabledAlpha {
		cp.alpha = 1
	}
}

func (cp *ColorPicker) applyHSBToValue() {
	r, g, b := hsbToRGB(cp.hue, cp.sat, cp.bri)
	col := render.RGBA{R: r, G: g, B: b, A: cp.alpha}
	if cp.DisabledAlpha {
		col.A = 1
	}
	cp.Value.Cleared = false
	if cp.Mode == ColorModeGradient {
		if len(cp.Value.Stops) < 2 {
			cp.Value.Stops = []ColorStop{
				{Color: col, Percent: 0},
				{Color: col, Percent: 100},
			}
			cp.activeStop = 0
		}
		if cp.activeStop < 0 || cp.activeStop >= len(cp.Value.Stops) {
			cp.activeStop = 0
		}
		cp.Value.Stops[cp.activeStop].Color = col
		cp.Value.RGBA = cp.Value.Stops[0].Color
	} else {
		cp.Value.RGBA = col
		cp.Value.Stops = nil
	}
}

func (cp *ColorPicker) fireChange() {
	if cp == nil {
		return
	}
	css := cp.Value.ToCssString(cp.Format)
	if cp.OnChange != nil {
		cp.OnChange(cp.cloneValue(cp.Value), css)
	}
}

func (cp *ColorPicker) activeRGBA() render.RGBA {
	if cp.Value.Cleared {
		return render.RGBA{A: 0}
	}
	if cp.Mode == ColorModeGradient && len(cp.Value.Stops) > 0 {
		i := cp.activeStop
		if i < 0 || i >= len(cp.Value.Stops) {
			i = 0
		}
		return cp.Value.Stops[i].Color
	}
	return cp.Value.RGBA
}

func mapColorPlacement(p ColorPlacement) primitive.Placement {
	switch p {
	case ColorBottomRight:
		return primitive.PlaceBottomEnd
	case ColorTopLeft:
		return primitive.PlaceTopStart
	case ColorTopRight:
		return primitive.PlaceTopEnd
	default:
		return primitive.PlaceBottomStart
	}
}

// ---------------------------------------------------------------------------
// color block host (trigger swatch)
// ---------------------------------------------------------------------------

type colorBlockHost struct {
	core.NodeBase
	cp       *ColorPicker
	size     float64
	fixed    render.RGBA // if A>0 or used as override when fixedSet
	fixedSet bool
}

func newColorBlockHost(cp *ColorPicker) *colorBlockHost {
	h := &colorBlockHost{cp: cp, size: 24}
	h.Init(h)
	h.Hit = core.HitDefer
	return h
}

func (h *colorBlockHost) TypeID() string { return "kit.ColorPickerBlock" }

func (h *colorBlockHost) Layout(c core.Constraints) core.Size {
	sz := h.size
	if sz <= 0 {
		sz = 24
	}
	out := c.Tighten(core.Size{Width: sz, Height: sz})
	h.SetSize(out)
	return out
}

func (h *colorBlockHost) Paint(pc *core.PaintContext) {
	if pc == nil {
		return
	}
	sz := h.Size()
	if sz.Width < 1 {
		return
	}
	// checkerboard for alpha
	cell := 4.0
	for y := 0.0; y < sz.Height; y += cell {
		for x := 0.0; x < sz.Width; x += cell {
			light := (int(x/cell)+int(y/cell))%2 == 0
			c := render.RGBA{R: 1, G: 1, B: 1, A: 1}
			if !light {
				c = render.RGBA{R: 0.85, G: 0.85, B: 0.85, A: 1}
			}
			pc.FillLocalRect(x, y, math.Min(cell, sz.Width-x), math.Min(cell, sz.Height-y), c)
		}
	}
	col := render.RGBA{A: 0}
	if h.fixedSet {
		col = h.fixed
	} else if h.cp != nil {
		if h.cp.Value.IsGradient() {
			// simple two-stop horizontal gradient approximation via strips
			stops := h.cp.Value.Stops
			for i := 0; i < int(sz.Width); i++ {
				t := float64(i) / math.Max(1, sz.Width-1)
				pc.FillLocalRect(float64(i), 0, 1, sz.Height, sampleGradient(stops, t*100))
			}
			// border
			if th := h.cp.theme(); th != nil {
				pc.StrokeLocalRoundRect(0, 0, sz.Width, sz.Height, 2, 1, th.Color(core.TokenColorBorder))
			}
			return
		}
		if !h.cp.Value.Cleared {
			col = h.cp.Value.RGBA
		}
	}
	if col.A > 0 || (h.cp != nil && !h.cp.Value.Cleared) {
		pc.FillLocalRoundRect(0, 0, sz.Width, sz.Height, 2, col)
	} else {
		// cleared: diagonal slash
		pc.StrokeLocalLine(2, sz.Height-2, sz.Width-2, 2, 1.5, render.RGBA{R: 1, G: 0.3, B: 0.3, A: 0.9})
	}
	border := render.RGBA{R: 0, G: 0, B: 0, A: 0.15}
	if h.cp != nil {
		if th := h.cp.theme(); th != nil {
			border = th.Color(core.TokenColorBorder)
		}
	}
	pc.StrokeLocalRoundRect(0, 0, sz.Width, sz.Height, 2, 1, border)
}

func (h *colorBlockHost) HitTest(p core.Point) core.Node {
	if h.LocalBounds().Contains(p) {
		return h
	}
	return nil
}

func sampleGradient(stops []ColorStop, percent float64) render.RGBA {
	if len(stops) == 0 {
		return render.RGBA{}
	}
	if len(stops) == 1 || percent <= stops[0].Percent {
		return stops[0].Color
	}
	if percent >= stops[len(stops)-1].Percent {
		return stops[len(stops)-1].Color
	}
	for i := 0; i < len(stops)-1; i++ {
		a, b := stops[i], stops[i+1]
		if percent >= a.Percent && percent <= b.Percent {
			span := b.Percent - a.Percent
			t := 0.0
			if span > 0 {
				t = (percent - a.Percent) / span
			}
			return a.Color.Lerp(b.Color, t)
		}
	}
	return stops[len(stops)-1].Color
}

// ---------------------------------------------------------------------------
// SV palette host
// ---------------------------------------------------------------------------

type cpPaletteHost struct {
	core.NodeBase
	cp       *ColorPicker
	w, h     float64
	dragging bool
}

func (h *cpPaletteHost) TypeID() string { return "kit.ColorPickerPalette" }

func (h *cpPaletteHost) Layout(c core.Constraints) core.Size {
	w, ht := h.w, h.h
	if w <= 0 {
		w = DefaultColorPickerWidth
	}
	if ht <= 0 {
		ht = w * 0.7
	}
	out := c.Tighten(core.Size{Width: w, Height: ht})
	h.SetSize(out)
	return out
}

func (h *cpPaletteHost) Paint(pc *core.PaintContext) {
	if pc == nil || h.cp == nil {
		return
	}
	sz := h.Size()
	if sz.Width < 1 || sz.Height < 1 {
		return
	}
	// Sample a coarse grid for performance (still readable).
	step := 4.0
	hue := h.cp.hue
	for y := 0.0; y < sz.Height; y += step {
		v := 1 - y/sz.Height
		for x := 0.0; x < sz.Width; x += step {
			s := x / sz.Width
			r, g, b := hsbToRGB(hue, s, v)
			pc.FillLocalRect(x, y, math.Min(step, sz.Width-x), math.Min(step, sz.Height-y),
				render.RGBA{R: r, G: g, B: b, A: 1})
		}
	}
	// cursor
	cx := h.cp.sat * sz.Width
	cy := (1 - h.cp.bri) * sz.Height
	pc.StrokeLocalCircle(cx, cy, 6, 2, render.RGBA{R: 1, G: 1, B: 1, A: 1})
	pc.StrokeLocalCircle(cx, cy, 6, 1, render.RGBA{R: 0, G: 0, B: 0, A: 0.45})
}

func (h *cpPaletteHost) HitTest(p core.Point) core.Node {
	if h.LocalBounds().Contains(p) {
		return h
	}
	return nil
}

func (h *cpPaletteHost) HandlePointer(ev *core.PointerEvent) {
	if h == nil || ev == nil || h.cp == nil || h.cp.Disabled {
		return
	}
	abs := core.AbsoluteBounds(h)
	lx := ev.X - abs.Min.X
	ly := ev.Y - abs.Min.Y
	switch ev.Type {
	case core.PointerDown:
		h.dragging = true
		h.setFrom(lx, ly)
		ev.Handled = true
	case core.PointerMove:
		if h.dragging {
			h.setFrom(lx, ly)
			ev.Handled = true
		}
	case core.PointerUp, core.PointerCancel:
		if h.dragging {
			h.dragging = false
			h.cp.CommitChange()
		}
		ev.Handled = true
	}
}

func (h *cpPaletteHost) setFrom(lx, ly float64) {
	sz := h.Size()
	if sz.Width < 1 || sz.Height < 1 {
		return
	}
	s := clamp01(lx / sz.Width)
	v := clamp01(1 - ly/sz.Height)
	h.cp.sat = s
	h.cp.bri = v
	h.cp.applyHSBToValue()
	h.cp.fireChange()
	h.cp.refreshChrome()
	h.cp.refreshPanel()
}

// ---------------------------------------------------------------------------
// hue / alpha slider host
// ---------------------------------------------------------------------------

type cpSliderKind int

const (
	cpSliderHue cpSliderKind = iota
	cpSliderAlpha
)

type cpSliderHost struct {
	core.NodeBase
	cp       *ColorPicker
	kind     cpSliderKind
	w        float64
	dragging bool
}

func newCPSliderHost(cp *ColorPicker, kind cpSliderKind) *cpSliderHost {
	h := &cpSliderHost{cp: cp, kind: kind, w: DefaultColorPickerWidth}
	h.Init(h)
	h.Hit = core.HitTarget
	return h
}

func (h *cpSliderHost) TypeID() string { return "kit.ColorPickerSlider" }

func (h *cpSliderHost) Layout(c core.Constraints) core.Size {
	w := h.w
	if w <= 0 {
		w = DefaultColorPickerWidth
	}
	ht := DefaultColorPickerSliderH + 8 // room for handle
	out := c.Tighten(core.Size{Width: w, Height: ht})
	h.SetSize(out)
	return out
}

func (h *cpSliderHost) Paint(pc *core.PaintContext) {
	if pc == nil || h.cp == nil {
		return
	}
	sz := h.Size()
	if sz.Width < 1 {
		return
	}
	cy := sz.Height / 2
	trackH := DefaultColorPickerSliderH
	y := cy - trackH/2
	switch h.kind {
	case cpSliderHue:
		// rainbow strips
		steps := 36
		sw := sz.Width / float64(steps)
		for i := 0; i < steps; i++ {
			hue := float64(i) / float64(steps) * 360
			r, g, b := hsbToRGB(hue, 1, 1)
			pc.FillLocalRoundRect(float64(i)*sw, y, sw+0.5, trackH, 0, render.RGBA{R: r, G: g, B: b, A: 1})
		}
		// round mask corners approx
		pc.StrokeLocalRoundRect(0, y, sz.Width, trackH, trackH/2, 1, render.RGBA{R: 0, G: 0, B: 0, A: 0.1})
		ratio := h.cp.hue / 360
		hx := ratio * sz.Width
		pc.FillLocalCircle(hx, cy, DefaultColorPickerHandlerSM/2, render.RGBA{R: 1, G: 1, B: 1, A: 1})
		pc.StrokeLocalCircle(hx, cy, DefaultColorPickerHandlerSM/2, 1, render.RGBA{R: 0, G: 0, B: 0, A: 0.25})
	case cpSliderAlpha:
		// checker + solid→transparent of current hue
		cell := 4.0
		for x := 0.0; x < sz.Width; x += cell {
			for yy := y; yy < y+trackH; yy += cell {
				light := (int(x/cell)+int((yy-y)/cell))%2 == 0
				c := render.RGBA{R: 1, G: 1, B: 1, A: 1}
				if !light {
					c = render.RGBA{R: 0.85, G: 0.85, B: 0.85, A: 1}
				}
				pc.FillLocalRect(x, yy, math.Min(cell, sz.Width-x), math.Min(cell, y+trackH-yy), c)
			}
		}
		base := h.cp.activeRGBA()
		steps := 24
		sw := sz.Width / float64(steps)
		for i := 0; i < steps; i++ {
			a := float64(i) / float64(steps-1)
			pc.FillLocalRect(float64(i)*sw, y, sw+0.5, trackH, render.RGBA{R: base.R, G: base.G, B: base.B, A: a})
		}
		hx := h.cp.alpha * sz.Width
		pc.FillLocalCircle(hx, cy, DefaultColorPickerHandlerSM/2, render.RGBA{R: 1, G: 1, B: 1, A: 1})
		pc.StrokeLocalCircle(hx, cy, DefaultColorPickerHandlerSM/2, 1, render.RGBA{R: 0, G: 0, B: 0, A: 0.25})
	}
}

func (h *cpSliderHost) HitTest(p core.Point) core.Node {
	if h.LocalBounds().Contains(p) {
		return h
	}
	return nil
}

func (h *cpSliderHost) HandlePointer(ev *core.PointerEvent) {
	if h == nil || ev == nil || h.cp == nil || h.cp.Disabled {
		return
	}
	abs := core.AbsoluteBounds(h)
	lx := ev.X - abs.Min.X
	switch ev.Type {
	case core.PointerDown:
		h.dragging = true
		h.setFromX(lx)
		ev.Handled = true
	case core.PointerMove:
		if h.dragging {
			h.setFromX(lx)
			ev.Handled = true
		}
	case core.PointerUp, core.PointerCancel:
		if h.dragging {
			h.dragging = false
			h.cp.CommitChange()
		}
		ev.Handled = true
	}
}

func (h *cpSliderHost) setFromX(lx float64) {
	w := h.Size().Width
	if w < 1 {
		w = h.w
	}
	t := clamp01(lx / w)
	switch h.kind {
	case cpSliderHue:
		h.cp.hue = t * 360
	case cpSliderAlpha:
		if h.cp.DisabledAlpha {
			return
		}
		h.cp.alpha = t
	}
	h.cp.applyHSBToValue()
	h.cp.fireChange()
	h.cp.refreshChrome()
	h.cp.refreshPanel()
}

// ---------------------------------------------------------------------------
// color math
// ---------------------------------------------------------------------------

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clamp360(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	for v < 0 {
		v += 360
	}
	for v >= 360 {
		v -= 360
	}
	return v
}

// rgbToHSB converts sRGB 0..1 → H 0..360, S/V 0..1.
func rgbToHSB(r, g, b float64) (h, s, v float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	v = max
	d := max - min
	if max > 0 {
		s = d / max
	}
	if d < 1e-9 {
		h = 0
		return
	}
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	return
}

// hsbToRGB converts H 0..360, S/V 0..1 → sRGB 0..1.
func hsbToRGB(h, s, v float64) (r, g, b float64) {
	if s <= 0 {
		return v, v, v
	}
	hh := clamp360(h) / 60
	i := int(math.Floor(hh))
	f := hh - float64(i)
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))
	switch i % 6 {
	case 0:
		return v, t, p
	case 1:
		return q, v, p
	case 2:
		return p, v, t
	case 3:
		return p, q, v
	case 4:
		return t, p, v
	default:
		return v, p, q
	}
}
