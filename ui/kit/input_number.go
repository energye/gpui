package kit

import (
	"math"
	"strconv"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design InputNumber defaults — docs/antd/input-number.md §6.2 / §6.10
// https://ant.design/components/input-number
const (
	// DefaultInputNumberWidth is antd controlWidth.
	DefaultInputNumberWidth = 90.0
	// DefaultInputNumberSafeAbs bounds min/max when unset (≈ MAX_SAFE_INTEGER intent).
	DefaultInputNumberSafeAbs = 1e15
	// DefaultInputNumberHandleWidth fallback ≈ controlHeightSM − 2×lineWidth.
	DefaultInputNumberHandleWidth = 22.0
	// DefaultInputNumberFocusRingOutset is visible focus ring outset (§6.2).
	DefaultInputNumberFocusRingOutset = 1.5
)

// InputNumberMode is antd mode: input | spinner.
type InputNumberMode int

const (
	// InputNumberModeInput is the default text field + up/down handlers.
	InputNumberModeInput InputNumberMode = iota
	// InputNumberModeSpinner uses plus/minus action chrome.
	InputNumberModeSpinner
)

// String implements fmt.Stringer.
func (m InputNumberMode) String() string {
	switch m {
	case InputNumberModeSpinner:
		return "spinner"
	default:
		return "input"
	}
}

// InputNumberStepEmitter is antd onStep info.emitter.
type InputNumberStepEmitter string

const (
	InputNumberStepHandler InputNumberStepEmitter = "handler"
	InputNumberStepKeydown InputNumberStepEmitter = "keydown"
	InputNumberStepWheel   InputNumberStepEmitter = "wheel"
)

// InputNumberStepInfo is antd onStep second argument.
type InputNumberStepInfo struct {
	Offset  float64
	Type    string // "up" | "down"
	Emitter InputNumberStepEmitter
}

// InputNumberFormatterInfo is antd formatter info.
type InputNumberFormatterInfo struct {
	UserTyping bool
	Input      string
}

// InputNumber is Ant Design InputNumber (value + step controls).
//
//	inputNumberHost (ScrollHandler)
//	  └─ Decorated chrome
//	       └─ Flex(Row)
//	            Flexible(EditableText) · actions (up/down or ±)
//
// Product contract: docs/antd/input-number.md §6 (P0 DoD).
type InputNumber struct {
	host   *inputNumberHost
	Root   *primitive.Decorated
	editor *primitive.EditableText
	row    *primitive.Flex
	upBtn  *primitive.Pressable
	dnBtn  *primitive.Pressable

	// Product fields (§6.10).
	Value         float64
	DefaultValue  float64
	hasDefault    bool
	hasValue      bool
	Min, Max      float64
	Step          float64
	Precision     int // <0 = unset
	Controls      bool
	Keyboard      bool
	ChangeOnWheel bool
	ChangeOnBlur  bool
	Disabled      bool
	ReadOnly      bool
	Controlled    bool
	StringMode    bool
	Size          InputSize
	Variant       InputVariant
	Status        InputStatus
	Mode          InputNumberMode
	Placeholder   string
	AriaLabel     string
	Face          text.Face
	Theme         *core.Theme
	Style         Style

	Formatter func(value float64, info InputNumberFormatterInfo) string
	Parser    func(display string) (float64, bool)

	OnChange     func(v float64)
	OnStep       func(v float64, info InputNumberStepInfo)
	OnPressEnter func(v float64)

	// Internal.
	focused   bool
	hovered   bool
	drafting  bool // user is typing raw text
	draftText string
	fixedW    float64
	fixedH    float64
}

// inputNumberHost wraps chrome and implements ScrollHandler for changeOnWheel.
type inputNumberHost struct {
	core.NodeBase
	n *InputNumber
}

// NewInputNumber creates an empty InputNumber (no default value).
// Prefer NewInputNumberValue for a seeded defaultValue.
func NewInputNumber() *InputNumber {
	n := &InputNumber{
		Min:          -DefaultInputNumberSafeAbs,
		Max:          DefaultInputNumberSafeAbs,
		Step:         1,
		Precision:    -1,
		Controls:     true,
		Keyboard:     true,
		ChangeOnBlur: true,
		Size:         InputMiddle,
		Variant:      InputOutlined,
		Status:       InputStatusNone,
		Mode:         InputNumberModeInput,
	}
	n.rebuild()
	return n
}

// NewInputNumberValue creates an InputNumber with defaultValue (uncontrolled seed).
// Legacy call sites used NewInputNumber(v); prefer this name for clarity.
func NewInputNumberValue(defaultValue float64) *InputNumber {
	n := NewInputNumber()
	n.SetDefaultValue(defaultValue)
	return n
}

// Node returns the root host node for tree attachment.

// ensureBuilt materializes the control tree if missing (#9).
func (n *InputNumber) ensureBuilt() {
	if n == nil {
		return
	}
	if n.host == nil {
		n.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (n *InputNumber) structureChange() {
	if n == nil {
		return
	}
	n.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (n *InputNumber) chromeChange() {
	if n == nil {
		return
	}
	n.ensureBuilt()
	n.rebuild()
}

func (n *InputNumber) Node() core.Node {
	if n == nil {
		return nil
	}
	n.ensureBuilt()
	return n.host
}

// ChromeNode returns the Decorated field chrome (tests / composition).
func (n *InputNumber) ChromeNode() core.Node {
	if n == nil {
		return nil
	}
	if n.Root == nil {
		n.rebuild()
	}
	return n.Root
}

// Editor returns the inner EditableText (caret / headless inject).
func (n *InputNumber) Editor() *primitive.EditableText {
	if n == nil {
		return nil
	}
	if n.editor == nil {
		n.rebuild()
	}
	return n.editor
}

// GetValue returns the current numeric value (0 when empty).
func (n *InputNumber) GetValue() float64 {
	if n == nil || !n.hasValue {
		return 0
	}
	return n.Value
}

// HasValue reports whether a numeric value is present.
func (n *InputNumber) HasValue() bool {
	return n != nil && n.hasValue
}

// ---------------------------------------------------------------------------
// Setters — product API (§6.10)
// ---------------------------------------------------------------------------

// SetValue writes the value without firing OnChange (parent / controlled writeback).
// Uncontrolled path clamps to min/max; controlled allows out-of-range display (FAQ).
func (n *InputNumber) SetValue(v float64) {
	if n == nil {
		return
	}
	if !n.Controlled {
		v = n.clamp(v)
	}
	n.Value = v
	n.hasValue = true
	n.drafting = false
	n.syncDisplay(false)
}

// SetDefaultValue sets the uncontrolled seed when no value has been set yet.
func (n *InputNumber) SetDefaultValue(v float64) {
	if n == nil {
		return
	}
	n.DefaultValue = v
	n.hasDefault = true
	if !n.Controlled && !n.hasValue {
		n.Value = n.clamp(v)
		n.hasValue = true
		n.syncDisplay(false)
	}
}

// SetControlled marks parent-owned value (antd value={…}).
func (n *InputNumber) SetControlled(v bool) {
	if n == nil {
		return
	}
	n.Controlled = v
}

// SetMin sets the inclusive minimum.
func (n *InputNumber) SetMin(v float64) {
	if n == nil {
		return
	}
	n.Min = v
	if n.hasValue && !n.Controlled {
		n.SetValue(n.Value)
	}
	n.syncActionEnabled()
}

// SetMax sets the inclusive maximum.
func (n *InputNumber) SetMax(v float64) {
	if n == nil {
		return
	}
	n.Max = v
	if n.hasValue && !n.Controlled {
		n.SetValue(n.Value)
	}
	n.syncActionEnabled()
}

// SetStep sets the step amount (must be > 0; non-positive resets to 1).
func (n *InputNumber) SetStep(v float64) {
	if n == nil {
		return
	}
	if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		v = 1
	}
	n.Step = v
}

// SetPrecision sets decimal places (<0 clears).
func (n *InputNumber) SetPrecision(p int) {
	if n == nil {
		return
	}
	n.Precision = p
	if n.hasValue && !n.drafting {
		n.syncDisplay(false)
	}
}

// SetControls toggles up/down (or ±) handlers. Default true.
func (n *InputNumber) SetControls(v bool) {
	if n == nil || n.Controls == v {
		return
	}
	n.Controls = v
	n.rebuild()
}

// SetKeyboard enables ArrowUp/ArrowDown stepping. Default true.
func (n *InputNumber) SetKeyboard(v bool) {
	if n == nil {
		return
	}
	n.Keyboard = v
}

// SetChangeOnWheel enables focused wheel stepping.
func (n *InputNumber) SetChangeOnWheel(v bool) {
	if n == nil {
		return
	}
	n.ChangeOnWheel = v
}

// SetChangeOnBlur enables clamp-on-blur. Default true.
func (n *InputNumber) SetChangeOnBlur(v bool) {
	if n == nil {
		return
	}
	n.ChangeOnBlur = v
}

// SetDisabled toggles disabled chrome and interaction.
func (n *InputNumber) SetDisabled(d bool) {
	if n == nil {
		return
	}
	n.Disabled = d
	if n.editor != nil {
		n.editor.Disabled = d
	}
	n.syncActionEnabled()
	n.applyChrome()
	n.applyA11y()
}

// SetReadOnly toggles read-only (focusable, not editable / no step).
func (n *InputNumber) SetReadOnly(r bool) {
	if n == nil {
		return
	}
	n.ReadOnly = r
	if n.editor != nil {
		n.editor.ReadOnly = r
	}
	n.syncActionEnabled()
}

// SetSize updates control size (small/middle/large → h 24/32/40).
func (n *InputNumber) SetSize(s InputSize) {
	if n == nil || n.Size == s {
		return
	}
	n.Size = s
	n.rebuild()
}

// SetVariant updates visual variant.
func (n *InputNumber) SetVariant(v InputVariant) {
	if n == nil || n.Variant == v {
		return
	}
	n.Variant = v
	n.applyChrome()
}

// SetStatus updates validation chrome.
func (n *InputNumber) SetStatus(s InputStatus) {
	if n == nil || n.Status == s {
		return
	}
	n.Status = s
	n.applyChrome()
	n.applyA11y()
}

// SetMode sets input | spinner.
func (n *InputNumber) SetMode(m InputNumberMode) {
	if n == nil || n.Mode == m {
		return
	}
	n.Mode = m
	n.rebuild()
}

// SetStringMode enables high-precision decimal path (digit.tsx).
func (n *InputNumber) SetStringMode(v bool) {
	if n == nil {
		return
	}
	n.StringMode = v
}

// SetFormatter sets display formatting (formatter.tsx).
func (n *InputNumber) SetFormatter(fn func(value float64, info InputNumberFormatterInfo) string) {
	if n == nil {
		return
	}
	n.Formatter = fn
	if n.hasValue && !n.drafting {
		n.syncDisplay(false)
	}
}

// SetParser sets reverse parse for formatter input.
func (n *InputNumber) SetParser(fn func(display string) (float64, bool)) {
	if n == nil {
		return
	}
	n.Parser = fn
}

// SetPlaceholder updates empty placeholder.
func (n *InputNumber) SetPlaceholder(s string) {
	if n == nil {
		return
	}
	n.Placeholder = s
	if n.editor != nil {
		n.editor.Placeholder = s
		n.editor.MarkNeedsPaint()
	}
}

// SetOnChange sets the change callback.
func (n *InputNumber) SetOnChange(fn func(float64)) {
	if n == nil {
		return
	}
	n.OnChange = fn
}

// SetOnStep sets the step callback (handler/keyboard/wheel).
func (n *InputNumber) SetOnStep(fn func(v float64, info InputNumberStepInfo)) {
	if n == nil {
		return
	}
	n.OnStep = fn
}

// SetOnPressEnter sets Enter callback.
func (n *InputNumber) SetOnPressEnter(fn func(float64)) {
	if n == nil {
		return
	}
	n.OnPressEnter = fn
}

// SetTheme sets an explicit theme override.
func (n *InputNumber) SetTheme(th *core.Theme) {
	if n == nil {
		return
	}
	n.Theme = th
	n.rebuild()
}

// SetFace sets the font face.
func (n *InputNumber) SetFace(face text.Face) {
	if n == nil {
		return
	}
	n.Face = face
	n.Style.Face = face
	if n.editor != nil {
		n.editor.Face = face
	}
}

// SetStyle applies visual overrides.
func (n *InputNumber) SetStyle(st Style) {
	if n == nil {
		return
	}
	n.Style = st
	if st.Face != nil {
		n.SetFace(st.Face)
	}
	if st.FontSize > 0 || st.Height > 0 || st.Width > 0 {
		n.rebuild()
		return
	}
	n.applyChrome()
}

// SetAriaLabel sets the accessible name.
func (n *InputNumber) SetAriaLabel(s string) {
	if n == nil {
		return
	}
	n.AriaLabel = s
	n.applyA11y()
}

// SetFixedSize forces outer chrome size (forms / gallery).
// Pass 0 for an axis to leave that axis on size/token metrics.
func (n *InputNumber) SetFixedSize(w, h float64) {
	if n == nil {
		return
	}
	n.fixedW, n.fixedH = w, h
	if n.Root == nil {
		n.rebuild()
		return
	}
	if w > 0 {
		n.Root.Width = w
		n.Root.MinWidth = w
	}
	if h > 0 {
		n.Root.Height = h
		n.Root.MinHeight = h
	}
	n.Root.MarkNeedsLayout()
}

// StepUp increments by step (handler emitter).
func (n *InputNumber) StepUp() { n.stepBy(1, InputNumberStepHandler) }

// StepDown decrements by step (handler emitter).
func (n *InputNumber) StepDown() { n.stepBy(-1, InputNumberStepHandler) }

// HandleKey implements keyboard stepping when focused (also call from host if needed).
func (n *InputNumber) HandleKey(ev *core.KeyEvent) {
	if n == nil || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if n.Disabled || n.ReadOnly || !n.Keyboard {
		return
	}
	switch ev.Key {
	case "Up", "ArrowUp":
		n.stepBy(1, InputNumberStepKeydown)
		ev.Handled = true
	case "Down", "ArrowDown":
		n.stepBy(-1, InputNumberStepKeydown)
		ev.Handled = true
	}
}

// HandleScroll implements changeOnWheel when focused.
func (n *InputNumber) HandleScroll(ev *core.ScrollEvent) {
	if n == nil || ev == nil || ev.Handled {
		return
	}
	if !n.ChangeOnWheel || n.Disabled || n.ReadOnly || !n.focused {
		return
	}
	if ev.DY == 0 {
		return
	}
	// Positive DY = scroll down → decrease (browser-like).
	dir := -1.0
	if ev.DY < 0 {
		dir = 1
	}
	n.stepBy(dir, InputNumberStepWheel)
	ev.Handled = true
}

// ---------------------------------------------------------------------------
// Host node
// ---------------------------------------------------------------------------

func (h *inputNumberHost) TypeID() string { return "kit.InputNumberHost" }

func (h *inputNumberHost) Layout(c core.Constraints) core.Size {
	if h == nil {
		return core.Size{}
	}
	kids := h.Children()
	if len(kids) == 0 {
		sz := c.Tighten(core.Size{})
		h.SetSize(sz)
		return sz
	}
	sz := kids[0].Layout(c)
	h.SetSize(sz)
	kids[0].Base().SetOffset(core.Point{})
	return sz
}

func (h *inputNumberHost) Paint(pc *core.PaintContext) {
	if h != nil {
		h.DefaultPaintChildren(pc)
	}
}

func (h *inputNumberHost) HitTest(p core.Point) core.Node {
	if h == nil {
		return nil
	}
	return h.DefaultHitTest(p)
}

func (h *inputNumberHost) HandleScroll(ev *core.ScrollEvent) {
	if h != nil && h.n != nil {
		h.n.HandleScroll(ev)
	}
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (n *InputNumber) theme() *core.Theme {
	var node core.Node
	if n.Root != nil {
		node = n.Root
	} else if n.host != nil {
		node = n.host
	}
	return themeOf(n.Theme, node)
}

func (n *InputNumber) metrics() (padH, height, fontSize, radius, lineW, handleW, width float64) {
	th := n.theme()
	fontSize = th.SizeOr(core.TokenFontSize, 14)
	radius = th.SizeOr(core.TokenBorderRadius, 6)
	lineW = th.SizeOr(core.TokenLineWidth, 1)
	padH = th.SizeOr(core.TokenControlPaddingInline, DefaultInputPaddingInline)
	width = DefaultInputNumberWidth
	switch n.Size {
	case InputSmall:
		height = th.SizeOr(core.TokenControlHeightSM, 24)
		fontSize = th.SizeOr(core.TokenFontSizeSM, 12)
		padH = th.SizeOr(core.TokenControlPaddingInlineSM, 7)
		radius = th.SizeOr(core.TokenBorderRadiusSM, 4)
	case InputLarge:
		height = th.SizeOr(core.TokenControlHeightLG, 40)
		fontSize = th.SizeOr(core.TokenFontSizeLG, 16)
		padH = th.SizeOr(core.TokenControlPaddingInlineLG, 11)
		radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	default:
		height = th.SizeOr(core.TokenControlHeight, 32)
	}
	// handleWidth ≈ controlHeightSM − 2×lineWidth
	smH := th.SizeOr(core.TokenControlHeightSM, 24)
	handleW = smH - lineW*2
	if handleW < 16 {
		handleW = DefaultInputNumberHandleWidth
	}
	if n.Style.FontSize > 0 {
		fontSize = n.Style.FontSize
	}
	if n.Style.Height > 0 {
		height = n.Style.Height
	}
	if n.Style.Width > 0 {
		width = n.Style.Width
	}
	if n.Style.hasRadius() {
		radius = n.Style.Radius
	}
	return padH, height, fontSize, radius, lineW, handleW, width
}

func (n *InputNumber) clamp(v float64) float64 {
	if math.IsNaN(v) {
		return v
	}
	min, max := n.Min, n.Max
	if max < min {
		min, max = max, min
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return n.applyPrecision(v)
}

func (n *InputNumber) applyPrecision(v float64) float64 {
	if n.Precision < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	p := math.Pow(10, float64(n.Precision))
	return math.Round(v*p) / p
}

func (n *InputNumber) stepBy(dir float64, emitter InputNumberStepEmitter) {
	if n == nil || n.Disabled || n.ReadOnly {
		return
	}
	if dir == 0 {
		return
	}
	step := n.Step
	if step <= 0 {
		step = 1
	}
	cur := 0.0
	if n.hasValue {
		cur = n.Value
	} else if n.hasDefault {
		cur = n.DefaultValue
	}
	// When drafting, prefer parsed draft.
	if n.drafting {
		if v, ok := n.parseInput(n.draftText); ok {
			cur = v
		}
	}
	offset := dir * step
	next := n.clamp(cur + offset)
	// At bound: max then up stays (INN-S3).
	if n.hasValue && next == n.Value && !n.drafting {
		// still fire onStep? antd fires onStep only when value changes via step.
		// Keep quiet when clamped no-op.
		n.syncActionEnabled()
		return
	}
	typ := "up"
	if dir < 0 {
		typ = "down"
	}
	n.commitValue(next, true)
	if n.OnStep != nil {
		n.OnStep(next, InputNumberStepInfo{Offset: math.Abs(offset), Type: typ, Emitter: emitter})
	}
	n.syncActionEnabled()
}

func (n *InputNumber) commitValue(v float64, fire bool) {
	v = n.applyPrecision(v)
	if n.Controlled {
		if fire && n.OnChange != nil {
			n.OnChange(v)
		}
		// Display stays until parent SetValue — but if parent not controlled writeback yet,
		// still show provisional for uncontrolled-feeling demos when Controlled false only.
		return
	}
	n.Value = v
	n.hasValue = true
	n.drafting = false
	n.syncDisplay(false)
	if fire && n.OnChange != nil {
		n.OnChange(v)
	}
}

func (n *InputNumber) formatValue(v float64, userTyping bool, input string) string {
	if n.Formatter != nil {
		return n.Formatter(v, InputNumberFormatterInfo{UserTyping: userTyping, Input: input})
	}
	if n.Precision >= 0 {
		return strconv.FormatFloat(v, 'f', n.Precision, 64)
	}
	// Prefer compact representation; keep trailing zeros only when precision set.
	if n.StringMode {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return formatNum(v)
}

func (n *InputNumber) parseInput(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if n.Parser != nil {
		return n.Parser(s)
	}
	// Strip common formatter noise when no parser: $ , % spaces
	clean := s
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.ReplaceAll(clean, " ", "")
	clean = strings.TrimPrefix(clean, "$")
	clean = strings.TrimSuffix(clean, "%")
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func (n *InputNumber) syncDisplay(userTyping bool) {
	if n == nil || n.editor == nil {
		return
	}
	var text string
	if n.drafting {
		text = n.draftText
	} else if n.hasValue {
		text = n.formatValue(n.Value, userTyping, "")
	} else {
		text = ""
	}
	if n.editor.Value != text {
		n.editor.Value = text
		// Keep caret at end for programmatic updates.
		n.editor.Cursor = len([]rune(text))
		n.editor.SelAnchor = n.editor.Cursor
		n.editor.MarkNeedsPaint()
	}
}

func (n *InputNumber) rebuild() {
	if n == nil {
		return
	}
	th := n.theme()
	padH, height, fontSize, radius, lineW, handleW, width := n.metrics()

	if n.editor == nil {
		n.editor = primitive.NewEditableText()
	}
	n.editor.Placeholder = n.Placeholder
	n.editor.Disabled = n.Disabled
	n.editor.ReadOnly = n.ReadOnly
	n.editor.Face = n.Face
	n.editor.FontSize = fontSize
	n.editor.ShowFocusRing = false
	n.editor.Multiline = false
	n.editor.Color = th.Color(core.TokenColorText)
	n.editor.PlaceholderColor = th.Color(core.TokenColorTextSecondary)
	if n.editor.PlaceholderColor.A < 0.15 {
		n.editor.PlaceholderColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	}
	n.syncDisplay(false)

	n.editor.OnChange = n.handleEditorChange
	n.editor.OnSubmit = func(string) {
		if n.Disabled || n.ReadOnly {
			return
		}
		n.flushDraft(true)
		if n.OnPressEnter != nil {
			n.OnPressEnter(n.GetValue())
		}
	}
	n.editor.OnFocusChange = func(f bool) {
		n.focused = f
		if !f {
			n.onBlur()
		}
		n.applyChrome()
	}
	n.editor.OnHoverChange = func(h bool) {
		n.hovered = h
		n.applyChrome()
	}
	n.editor.VerticalArrowHandler = func(dir int) bool {
		if n.Disabled || n.ReadOnly || !n.Keyboard {
			return false
		}
		// dir: -1 up key, +1 down key
		if dir < 0 {
			n.stepBy(1, InputNumberStepKeydown)
		} else {
			n.stepBy(-1, InputNumberStepKeydown)
		}
		return true
	}

	flexEd := primitive.NewFlexible(1, n.editor)
	flexEd.FillChild = true

	kids := []core.Node{flexEd}
	n.upBtn, n.dnBtn = nil, nil
	if n.Controls {
		actions := n.buildActions(handleW, height, lineW, fontSize, th)
		kids = append(kids, actions)
	}

	n.row = primitive.Row(kids...)
	n.row.Gap = 0
	n.row.CrossAlign = core.CrossStretch

	// Underline bar for underlined variant.
	var body core.Node = n.row
	if n.Variant == InputUnderlined {
		under := primitive.NewDecorated()
		under.Height = lineW
		under.MinHeight = lineW
		under.ExpandWidth = true
		under.Background = th.Color(core.TokenColorBorder)
		under.Hit = core.HitTransparent
		col := primitive.Column(n.row, under)
		col.Gap = 0
		col.CrossAlign = core.CrossStretch
		body = col
	}

	if n.Root == nil {
		n.Root = primitive.NewDecorated(body)
		n.Root.SkinType = TypeInputNumber
	} else {
		n.Root.ClearChildren()
		n.Root.AddChild(body)
	}
	// Right padding is 0 when controls visible (handlers sit on edge).
	rightPad := padH
	if n.Controls {
		rightPad = 0
	}
	n.Root.Padding = primitive.EdgeInsets{Left: padH, Right: rightPad, Top: 0, Bottom: 0}
	n.Root.Radius = radius
	n.Root.BorderWidth = lineW
	n.Root.Hit = core.HitDefer
	n.Root.SetCenterContent(true)
	n.Root.MinHeight = height
	n.Root.Height = height
	w := width
	if n.fixedW > 0 {
		w = n.fixedW
	}
	if w > 0 {
		n.Root.Width = w
		n.Root.MinWidth = w
	}
	if n.fixedH > 0 {
		n.Root.Height = n.fixedH
		n.Root.MinHeight = n.fixedH
	}
	n.Root.SetThemeHook(func(*core.Theme) { n.rebuild() })

	if n.host == nil {
		n.host = &inputNumberHost{n: n}
		n.host.Init(n.host)
		n.host.Hit = core.HitDefer
	} else {
		n.host.n = n
		n.host.ClearChildren()
	}
	n.host.AddChild(n.Root)

	n.applyA11y()
	n.applyChrome()
	n.syncActionEnabled()
	n.Root.MarkNeedsLayout()
	n.Root.MarkNeedsPaint()
}

func (n *InputNumber) buildActions(handleW, height, lineW, fontSize float64, th *core.Theme) core.Node {
	mk := func(label, aria string, up bool, w, h float64) *primitive.Pressable {
		lab := primitive.NewText(label)
		lab.FontSize = math.Max(10, fontSize*0.75)
		lab.Color = th.Color(core.TokenColorTextSecondary)
		var child core.Node = lab
		if n.Mode == InputNumberModeSpinner {
			name := "plus"
			if !up {
				name = "minus"
			}
			ic := primitive.NewIcon(name)
			ic.Size = math.Max(12, fontSize*0.85)
			ic.Color = th.Color(core.TokenColorTextSecondary)
			child = ic
		}
		// Fixed hit box so actions keep Token handle geometry.
		box := primitive.NewBox(child)
		box.Width = w
		box.Height = h
		box.Color = render.RGBA{}
		p := primitive.NewPressable(box)
		p.Focusable = false
		p.ShowFocusRing = false
		p.Base().Label = aria
		p.Base().Role = "button"
		p.Click = func() {
			if up {
				n.StepUp()
			} else {
				n.StepDown()
			}
		}
		return p
	}

	innerH := height - lineW*2
	if innerH < 16 {
		innerH = 16
	}

	if n.Mode == InputNumberModeSpinner {
		n.dnBtn = mk("−", "Decrease value", false, handleW, innerH)
		n.upBtn = mk("+", "Increase value", true, handleW, innerH)
		row := primitive.Row(n.dnBtn, n.upBtn)
		row.Gap = 0
		row.CrossAlign = core.CrossCenter
		return row
	}

	// Default input mode: stacked up/down.
	half := innerH / 2
	if half < 10 {
		half = 10
	}
	n.upBtn = mk("▴", "Increase value", true, handleW, half)
	n.dnBtn = mk("▾", "Decrease value", false, handleW, half)
	col := primitive.Column(n.upBtn, n.dnBtn)
	col.Gap = 0
	col.CrossAlign = core.CrossStretch
	wrap := primitive.NewDecorated(col)
	wrap.BorderWidth = 0
	wrap.Background = render.RGBA{}
	wrap.Hit = core.HitDefer
	wrap.Width = handleW
	wrap.MinWidth = handleW
	return wrap
}

func (n *InputNumber) syncActionEnabled() {
	dis := n.Disabled || n.ReadOnly
	if n.upBtn != nil {
		atMax := n.hasValue && n.Value >= n.Max
		n.upBtn.SetDisabled(dis || atMax)
	}
	if n.dnBtn != nil {
		atMin := n.hasValue && n.Value <= n.Min
		n.dnBtn.SetDisabled(dis || atMin)
	}
}

func (n *InputNumber) handleEditorChange(s string) {
	if n == nil || n.Disabled || n.ReadOnly {
		return
	}
	n.drafting = true
	n.draftText = s
	if n.Controlled {
		if v, ok := n.parseInput(s); ok {
			if n.OnChange != nil {
				n.OnChange(v)
			}
		} else if s == "" && n.OnChange != nil {
			// Empty → treat as 0 for numeric API (antd may pass null; kit uses 0 + hasValue false via parent).
			n.OnChange(0)
		}
		// Revert display to controlled value formatting when parent doesn't write back immediately:
		// keep draft visible while typing for UX (antd keeps input string while focused).
		if n.editor != nil && n.editor.Value != s {
			n.editor.Value = s
		}
		return
	}
	if v, ok := n.parseInput(s); ok {
		// Live update value while typing (antd commits more carefully; kit updates numeric).
		n.Value = v
		n.hasValue = true
		if n.OnChange != nil {
			n.OnChange(v)
		}
	} else if s == "" {
		n.hasValue = false
		n.Value = 0
		if n.OnChange != nil {
			n.OnChange(0)
		}
	}
	// Keep raw draft text in editor while focused.
	n.draftText = s
}

func (n *InputNumber) flushDraft(fire bool) {
	if n == nil || !n.drafting {
		return
	}
	s := n.draftText
	n.drafting = false
	if v, ok := n.parseInput(s); ok {
		if n.ChangeOnBlur || fire {
			v = n.clamp(v)
		}
		if n.Controlled {
			if fire && n.OnChange != nil {
				n.OnChange(v)
			}
			// Wait for parent SetValue for display.
			if n.hasValue {
				n.syncDisplay(false)
			}
			return
		}
		prev := n.Value
		had := n.hasValue
		n.Value = v
		n.hasValue = true
		n.syncDisplay(false)
		if fire && n.OnChange != nil && (!had || prev != v) {
			n.OnChange(v)
		}
		return
	}
	// Invalid → restore last good value.
	n.syncDisplay(false)
}

func (n *InputNumber) onBlur() {
	if n == nil {
		return
	}
	if n.ChangeOnBlur {
		n.flushDraft(true)
		// Clamp current value into range on blur (uncontrolled).
		if !n.Controlled && n.hasValue {
			clamped := n.clamp(n.Value)
			if clamped != n.Value {
				n.Value = clamped
				n.syncDisplay(false)
				if n.OnChange != nil {
					n.OnChange(clamped)
				}
			}
		}
	} else {
		n.drafting = false
		n.syncDisplay(false)
	}
	n.syncActionEnabled()
}

func (n *InputNumber) applyA11y() {
	if n == nil {
		return
	}
	name := n.AriaLabel
	if name == "" {
		name = n.Placeholder
	}
	if n.Status == InputStatusError {
		if name != "" {
			name = name + " (invalid)"
		} else {
			name = "invalid"
		}
	}
	if n.editor != nil {
		n.editor.Base().Role = "spinbutton"
		n.editor.Base().Label = name
	}
	if n.Root != nil {
		n.Root.Base().Role = "group"
		n.Root.Base().Label = name
	}
}

func (n *InputNumber) applyChrome() {
	if n == nil || n.Root == nil {
		return
	}
	th := n.theme()
	_, _, _, radius, lineW, _, _ := n.metrics()
	if n.Style.hasRadius() {
		radius = n.Style.Radius
	}
	n.Root.Radius = radius

	bg := th.Color(core.TokenColorBgContainer)
	bd := th.Color(core.TokenColorBorder)
	bw := lineW

	switch n.Variant {
	case InputFilled:
		bg = th.Color(core.TokenColorFillSecondary)
		if bg.A < 0.01 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bd = render.RGBA{}
		bw = 0
		if n.focused {
			bd = th.Color(core.TokenColorPrimary)
			bw = lineW
		} else if n.hovered && !n.Disabled {
			bg = th.Color(core.TokenColorBgTextHover)
			if bg.A < 0.01 {
				bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
			}
		}
	case InputBorderless:
		bg = render.RGBA{}
		bd = render.RGBA{}
		bw = 0
		if n.focused {
			// subtle focus via primary text caret only; keep borderless
		}
	case InputUnderlined:
		bg = render.RGBA{}
		bd = render.RGBA{}
		bw = 0
	default: // outlined
		if n.focused {
			bd = th.Color(core.TokenColorPrimary)
		} else if n.hovered && !n.Disabled {
			hover := th.Color(core.TokenColorPrimaryHover)
			if hover.A > 0.01 {
				bd = hover
			} else {
				bd = th.Color(core.TokenColorPrimary)
			}
		}
	}

	// status overrides border (not for borderless empty)
	switch n.Status {
	case InputStatusError:
		bd = th.Color(core.TokenColorError)
		if n.Variant == InputOutlined || n.Variant == InputFilled {
			bw = lineW
		}
	case InputStatusWarning:
		bd = th.Color(core.TokenColorWarning)
		if n.Variant == InputOutlined || n.Variant == InputFilled {
			bw = lineW
		}
	}

	if n.Disabled {
		bg = th.Color(core.TokenColorDisabledBg)
		if bg.A < 0.01 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bd = th.Color(core.TokenColorBorder)
		if n.Variant == InputBorderless || n.Variant == InputUnderlined {
			bd = render.RGBA{}
			bw = 0
		}
		if n.editor != nil {
			n.editor.Color = th.Color(core.TokenColorDisabledText)
		}
	} else if n.editor != nil {
		if n.Style.Text.A > 0 {
			n.editor.Color = n.Style.Text
		} else {
			n.editor.Color = th.Color(core.TokenColorText)
		}
	}

	if n.Style.Background.A > 0 && !n.Disabled {
		bg = n.Style.Background
	}

	n.Root.Background = bg
	n.Root.BorderColor = bd
	n.Root.BorderWidth = bw

	// Focus ring via border emphasis (visible primary edge). Outset is visual via thicker feel.
	if n.focused && !n.Disabled && n.Variant != InputBorderless {
		if n.Status == InputStatusNone {
			n.Root.BorderColor = th.Color(core.TokenColorPrimary)
			if n.Root.BorderWidth < lineW {
				n.Root.BorderWidth = lineW
			}
		}
	}

	// Out-of-range warning chrome (controlled FAQ path).
	if n.hasValue && !n.Disabled && (n.Value < n.Min || n.Value > n.Max) && n.Status == InputStatusNone {
		// Soft error edge when value exceeds bounds without status prop.
		errC := th.Color(core.TokenColorError)
		if n.Variant == InputOutlined || n.Variant == InputFilled {
			n.Root.BorderColor = errC
			n.Root.BorderWidth = lineW
		}
	}

	n.Root.MarkNeedsPaint()
}

// DisplayString returns the current formatted display (tests / debug).
func (n *InputNumber) DisplayString() string {
	if n == nil {
		return ""
	}
	if n.drafting {
		return n.draftText
	}
	if n.editor != nil {
		return n.editor.Value
	}
	if n.hasValue {
		return n.formatValue(n.Value, false, "")
	}
	return ""
}

// ControlsVisible reports whether action buttons are mounted (tests INN-08).
func (n *InputNumber) ControlsVisible() bool {
	return n != nil && n.Controls && n.upBtn != nil && n.dnBtn != nil
}

// HeightToken returns the resolved control height for the current size (tests).
func (n *InputNumber) HeightToken() float64 {
	if n == nil {
		return 0
	}
	_, h, _, _, _, _, _ := n.metrics()
	return h
}
