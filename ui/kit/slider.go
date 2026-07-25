package kit

import (
	"fmt"
	"math"
	"sort"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Slider defaults — components/slider/style prepareComponentToken.
// docs/antd/slider.md §6.2 / §6.10
//
//	controlSize       = controlHeightLG / 4 → 10
//	railSize          = 4
//	handleSize        = controlSize → 10
//	handleSizeHover   = controlHeightSM / 2 → 12
//	handleLineWidth   = lineWidth + 1 → 2
//	dotSize           = 8
const (
	DefaultSliderMin              = 0.0
	DefaultSliderMax              = 100.0
	DefaultSliderStep             = 1.0
	DefaultSliderControlSize      = 10.0
	DefaultSliderRailSize         = 4.0
	DefaultSliderHandleSize       = 10.0
	DefaultSliderHandleSizeHover  = 12.0
	DefaultSliderHandleLineWidth  = 2.0
	DefaultSliderDotSize          = 8.0
	DefaultSliderFocusOutset      = 1.5
	DefaultSliderFallbackWidth    = 200.0
	DefaultSliderFallbackHeight   = 200.0
	DefaultSliderMarkLabelGap     = 8.0
	DefaultSliderMarkLabelHeight  = 18.0
	DefaultSliderTooltipPadX      = 8.0
	DefaultSliderTooltipPadY      = 4.0
	DefaultSliderHandleHitPadding = 4.0 // hit ≥ visual
)

// SliderOrientation is antd orientation: horizontal | vertical.
type SliderOrientation int

const (
	SliderHorizontal SliderOrientation = iota
	SliderVertical
)

// SliderTooltipOpen maps antd tooltip.open tri-state.
type SliderTooltipOpen int

const (
	// SliderTooltipAuto shows while dragging or hovering the handle (antd default).
	SliderTooltipAuto SliderTooltipOpen = iota
	// SliderTooltipAlways keeps the tip open (tooltip.open = true).
	SliderTooltipAlways
	// SliderTooltipNever never shows the tip (tooltip.open = false).
	SliderTooltipNever
)

// SliderMark is one antd marks entry (value → label).
type SliderMark struct {
	Value float64
	Label string
	// Color optional label color; zero A → theme text secondary.
	Color render.RGBA
}

// Slider is Ant Design Slider (single or range).
//
//	sliderHost (role=slider, hit==layout==paint)
//	  rail / track / dots / marks / handle×1|2 / tooltip bubble
//
// Product contract: docs/antd/slider.md §6 (P0 DoD).
type Slider struct {
	Root *sliderHost

	// Value is the single-mode committed value.
	Value float64
	// Values holds range-mode [lo, hi] with lo ≤ hi.
	RangeValues [2]float64
	// Range enables dual handles.
	Range bool

	Min  float64
	Max  float64
	Step float64
	// StepNull maps antd step={null}: snap only to marks (requires Marks).
	StepNull bool

	Disabled   bool
	Keyboard   bool // antd default true
	Controlled bool
	Included   bool // marks included track semantics (default true)
	Dots       bool // only land on step ticks

	Orientation SliderOrientation
	// Reverse is P1 (coordinate flip); field reserved, not applied in P0.
	Reverse bool

	Marks []SliderMark

	// Width/Height preferred track length. 0 → fill parent; unbounded → fallback 200.
	Width  float64
	Height float64

	// Tooltip
	TooltipOpen          SliderTooltipOpen
	TooltipPlacement     primitive.Placement // default PlaceTop
	TooltipFormatter     func(float64) string
	TooltipFormatterNull bool // antd formatter: null

	OnChange         func(v float64)
	OnRangeChange    func(lo, hi float64)
	OnChangeComplete func(vals []float64)
	OnFocus          func()
	OnBlur           func()
	OnKeyDown        func(ev *core.KeyEvent)

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// interaction
	dragging     bool
	activeHandle int // 0 single / low; 1 high (range)
	hovering     bool
	focused      bool
	focusVisible bool
	// lastInteract holds the latest user-driven value(s) for onChangeComplete
	// under controlled mode (parent may not have flushed SetValue yet).
	lastInteract    float64
	lastInteractLo  float64
	lastInteractHi  float64
	hasLastInteract bool

	// tooltip runtime
	tipVisible bool
	tipText    string
	tipX, tipY float64 // local bubble anchor (handle center)

	// cached metrics
	controlSize    float64
	railSize       float64
	handleSize     float64
	handleHover    float64
	handleLine     float64
	dotSize        float64
	railBg         render.RGBA
	railHover      render.RGBA
	trackBg        render.RGBA
	trackHover     render.RGBA
	handleFill     render.RGBA
	handleStroke   render.RGBA
	handleActive   render.RGBA
	dotBorder      render.RGBA
	dotActive      render.RGBA
	markColor      render.RGBA
	disabledTrack  render.RGBA
	disabledHandle render.RGBA
	focusRing      render.RGBA
}

// sliderHost paints track/thumb and maps pointer → value.
// hit == layout == paint.
type sliderHost struct {
	core.NodeBase
	*Slider
}

// NewSlider creates a Slider with Ant defaults (min=0 max=100 step=1).
// defaultValue is the initial non-controlled value.
func NewSlider(defaultValue float64) *Slider {
	s := &Slider{
		Value:            defaultValue,
		Min:              DefaultSliderMin,
		Max:              DefaultSliderMax,
		Step:             DefaultSliderStep,
		Keyboard:         true,
		Included:         true,
		TooltipOpen:      SliderTooltipAuto,
		TooltipPlacement: primitive.PlaceTop,
	}
	s.RangeValues = [2]float64{defaultValue, defaultValue}
	s.rebuild()
	return s
}

// Node returns root (stable across SetValue).
func (s *Slider) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// ChromeNode returns the painted host (tests / geometry).
func (s *Slider) ChromeNode() core.Node { return s.Node() }

// ---------- public getters (tests / gallery) ----------

func (s *Slider) IsVertical() bool {
	return s.Orientation == SliderVertical
}

func (s *Slider) RailSize() float64 {
	s.resolveMetrics()
	return s.railSize
}

func (s *Slider) HandleSize() float64 {
	s.resolveMetrics()
	return s.handleSize
}

func (s *Slider) ControlSize() float64 {
	s.resolveMetrics()
	return s.controlSize
}

func (s *Slider) DotSize() float64 {
	s.resolveMetrics()
	return s.dotSize
}

func (s *Slider) HandleLineWidth() float64 {
	s.resolveMetrics()
	return s.handleLine
}

func (s *Slider) FocusOutset() float64 { return DefaultSliderFocusOutset }

// Values returns range endpoints (sorted). Single mode returns (Value, Value).
func (s *Slider) Values() (lo, hi float64) {
	if s.Range {
		lo, hi = s.RangeValues[0], s.RangeValues[1]
		if lo > hi {
			lo, hi = hi, lo
		}
		return lo, hi
	}
	return s.Value, s.Value
}

// TooltipVisible reports whether the value tip should show.
func (s *Slider) TooltipVisible() bool {
	if s.TooltipFormatterNull {
		return false
	}
	switch s.TooltipOpen {
	case SliderTooltipNever:
		return false
	case SliderTooltipAlways:
		return true
	default:
		return s.dragging || s.hovering
	}
}

// TooltipText is the current tip string (empty when hidden / formatter null).
func (s *Slider) TooltipText() string {
	if !s.TooltipVisible() {
		return ""
	}
	return s.formatTip(s.tipValue())
}

// TipValue is the value shown in the tooltip (active handle).
func (s *Slider) TipValue() float64 { return s.tipValue() }

// ---------- setters ----------

// SetValue writes the committed single value without firing OnChange (antd value=).
func (s *Slider) SetValue(v float64) {
	v = s.clampSnap(v)
	if s.Range {
		lo, hi := s.Values()
		span := hi - lo
		if span < 0 {
			span = 0
		}
		s.RangeValues[0] = v
		s.RangeValues[1] = s.clampSnap(v + span)
		if s.RangeValues[0] > s.RangeValues[1] {
			s.RangeValues[0], s.RangeValues[1] = s.RangeValues[1], s.RangeValues[0]
		}
		s.applyChrome()
		return
	}
	if almostEq(s.Value, v) {
		s.applyChrome()
		return
	}
	s.Value = v
	s.applyChrome()
}

// SetDefaultValue sets the initial value when not controlled.
func (s *Slider) SetDefaultValue(v float64) {
	if s.Controlled {
		return
	}
	s.SetValue(v)
}

// SetValues writes range endpoints without firing callbacks (antd value={[lo,hi]}).
func (s *Slider) SetValues(lo, hi float64) {
	s.Range = true
	s.setRangeInternal(lo, hi, false, false)
}

// SetDefaultValues sets initial range when not controlled.
func (s *Slider) SetDefaultValues(lo, hi float64) {
	if s.Controlled {
		return
	}
	s.Range = true
	s.setRangeInternal(lo, hi, false, false)
}

func (s *Slider) SetRange(on bool) {
	if s.Range == on {
		return
	}
	s.Range = on
	if on {
		lo := s.Value
		hi := s.Value
		if s.RangeValues[1] > s.RangeValues[0] {
			lo, hi = s.RangeValues[0], s.RangeValues[1]
		} else {
			// seed a small span when only single value known
			hi = s.clampSnap(lo + s.stepOr(10))
		}
		s.RangeValues = [2]float64{lo, hi}
	} else {
		s.Value = s.RangeValues[0]
	}
	s.applyA11y()
	s.applyChrome()
}

func (s *Slider) SetMin(v float64) {
	s.Min = v
	s.reclampAll()
	s.applyChrome()
}

func (s *Slider) SetMax(v float64) {
	s.Max = v
	s.reclampAll()
	s.applyChrome()
}

func (s *Slider) SetStep(v float64) {
	if v > 0 {
		s.Step = v
		s.StepNull = false
	}
	s.reclampAll()
	s.applyChrome()
}

func (s *Slider) SetStepNull(on bool) {
	s.StepNull = on
	s.reclampAll()
	s.applyChrome()
}

func (s *Slider) SetDots(on bool) {
	s.Dots = on
	s.applyChrome()
}

func (s *Slider) SetIncluded(on bool) {
	s.Included = on
	s.applyChrome()
}

func (s *Slider) SetMarks(marks ...SliderMark) {
	s.Marks = append([]SliderMark(nil), marks...)
	sort.Slice(s.Marks, func(i, j int) bool { return s.Marks[i].Value < s.Marks[j].Value })
	s.rebuild() // marks affect layout height
}

func (s *Slider) SetOrientation(o SliderOrientation) {
	if s.Orientation == o {
		return
	}
	s.Orientation = o
	s.rebuild()
}

func (s *Slider) SetVertical(v bool) {
	if v {
		s.SetOrientation(SliderVertical)
	} else {
		s.SetOrientation(SliderHorizontal)
	}
}

func (s *Slider) SetDisabled(d bool) {
	s.Disabled = d
	s.applyA11y()
	s.applyChrome()
}

func (s *Slider) SetKeyboard(on bool) { s.Keyboard = on }

func (s *Slider) SetControlled(c bool) { s.Controlled = c }

func (s *Slider) SetWidth(w float64) {
	s.Width = w
	if s.Root != nil {
		s.Root.MarkNeedsLayout()
	}
}

func (s *Slider) SetHeight(h float64) {
	s.Height = h
	if s.Root != nil {
		s.Root.MarkNeedsLayout()
	}
}

func (s *Slider) SetTooltipOpen(mode SliderTooltipOpen) {
	s.TooltipOpen = mode
	s.applyChrome()
}

func (s *Slider) SetTooltipPlacement(p primitive.Placement) {
	s.TooltipPlacement = p
	s.applyChrome()
}

func (s *Slider) SetTooltipFormatter(fn func(float64) string) {
	s.TooltipFormatter = fn
	s.TooltipFormatterNull = false
	s.applyChrome()
}

func (s *Slider) SetTooltipFormatterNull(on bool) {
	s.TooltipFormatterNull = on
	s.applyChrome()
}

func (s *Slider) SetOnChange(fn func(float64))                { s.OnChange = fn }
func (s *Slider) SetOnRangeChange(fn func(lo, hi float64))    { s.OnRangeChange = fn }
func (s *Slider) SetOnChangeComplete(fn func(vals []float64)) { s.OnChangeComplete = fn }

func (s *Slider) SetAriaLabel(label string) {
	s.AriaLabel = label
	s.applyA11y()
}

func (s *Slider) SetTheme(th *core.Theme) {
	s.Theme = th
	s.applyChrome()
}

func (s *Slider) SetFace(face text.Face) {
	s.Face = face
	s.applyChrome()
}

func (s *Slider) SetStyle(st Style) {
	s.Style = st
	s.applyChrome()
}

// ---------- internals ----------

func (s *Slider) theme() *core.Theme {
	var n core.Node
	if s.Root != nil {
		n = s.Root
	}
	return themeOf(s.Theme, n)
}

func (s *Slider) resolveMetrics() {
	th := s.theme()
	chLG := th.SizeOr(core.TokenControlHeightLG, 40)
	chSM := th.SizeOr(core.TokenControlHeightSM, 24)
	lw := th.SizeOr(core.TokenLineWidth, 1)

	s.controlSize = chLG / 4
	if s.controlSize <= 0 {
		s.controlSize = DefaultSliderControlSize
	}
	s.railSize = DefaultSliderRailSize
	s.handleSize = s.controlSize
	s.handleHover = chSM / 2
	if s.handleHover <= 0 {
		s.handleHover = DefaultSliderHandleSizeHover
	}
	s.handleLine = lw + 1
	if s.handleLine < 1 {
		s.handleLine = DefaultSliderHandleLineWidth
	}
	s.dotSize = DefaultSliderDotSize

	// Colors — prefer tokens; never require hardcoded brand as sole path.
	s.railBg = th.Color(core.TokenColorFillSecondary)
	if s.railBg.A < 0.02 {
		s.railBg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
	}
	s.railHover = th.Color(core.TokenColorFillSecondary)
	// slightly stronger hover rail
	if s.railHover.A < 0.08 {
		s.railHover = render.RGBA{R: 0, G: 0, B: 0, A: 0.08}
	}
	s.trackBg = th.Color(core.TokenColorPrimaryBorder)
	if s.trackBg.A < 0.2 {
		s.trackBg = th.Color(core.TokenColorPrimary)
	}
	s.trackHover = th.Color(core.TokenColorPrimaryHover)
	if s.trackHover.A < 0.2 {
		s.trackHover = s.trackBg
	}
	s.handleFill = th.Color(core.TokenColorBgContainer)
	if s.handleFill.A < 0.5 {
		s.handleFill = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	s.handleStroke = th.Color(core.TokenColorPrimaryBorder)
	if s.handleStroke.A < 0.2 {
		s.handleStroke = th.Color(core.TokenColorPrimary)
	}
	s.handleActive = th.Color(core.TokenColorPrimary)
	s.dotBorder = th.Color(core.TokenColorBorderSecondary)
	if s.dotBorder.A < 0.05 {
		s.dotBorder = th.Color(core.TokenColorBorder)
	}
	s.dotActive = th.Color(core.TokenColorPrimaryBorder)
	if s.dotActive.A < 0.2 {
		s.dotActive = th.Color(core.TokenColorPrimary)
	}
	s.markColor = th.Color(core.TokenColorTextSecondary)
	s.disabledTrack = th.Color(core.TokenColorDisabledBg)
	s.disabledHandle = th.Color(core.TokenColorDisabledText)
	s.focusRing = th.Color(core.TokenColorPrimary)
}

func (s *Slider) minMax() (min, max float64) {
	min, max = s.Min, s.Max
	if max <= min {
		min, max = DefaultSliderMin, DefaultSliderMax
	}
	return min, max
}

func (s *Slider) stepOr(fallback float64) float64 {
	if s.StepNull {
		return 0
	}
	if s.Step > 0 {
		return s.Step
	}
	if fallback > 0 {
		return fallback
	}
	return DefaultSliderStep
}

func (s *Slider) clampSnap(v float64) float64 {
	min, max := s.minMax()
	if math.IsNaN(v) {
		v = min
	}
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	if s.StepNull {
		return s.snapToMarks(v)
	}
	step := s.stepOr(0)
	if s.Dots && step > 0 {
		// dots: only step positions
	}
	if step > 0 {
		steps := math.Round((v - min) / step)
		v = min + steps*step
		// re-clamp after snap
		if v < min {
			v = min
		}
		if v > max {
			v = max
		}
		// fix float residue for common integer steps
		if step >= 1 && math.Abs(step-math.Round(step)) < 1e-9 {
			v = math.Round(v)
		} else {
			// keep reasonable precision for fractional steps (e.g. 0.01)
			inv := 1 / step
			if inv > 1 && inv < 1e9 && math.Abs(inv-math.Round(inv)) < 1e-9 {
				v = math.Round(v*inv) / inv
			}
		}
	}
	return v
}

func (s *Slider) snapToMarks(v float64) float64 {
	if len(s.Marks) == 0 {
		min, max := s.minMax()
		if v < min {
			return min
		}
		if v > max {
			return max
		}
		return v
	}
	best := s.Marks[0].Value
	bestD := math.Abs(v - best)
	for _, m := range s.Marks[1:] {
		d := math.Abs(v - m.Value)
		if d < bestD {
			best, bestD = m.Value, d
		}
	}
	min, max := s.minMax()
	if best < min {
		best = min
	}
	if best > max {
		best = max
	}
	return best
}

func (s *Slider) reclampAll() {
	if s.Range {
		lo, hi := s.Values()
		s.RangeValues[0] = s.clampSnap(lo)
		s.RangeValues[1] = s.clampSnap(hi)
		if s.RangeValues[0] > s.RangeValues[1] {
			s.RangeValues[0], s.RangeValues[1] = s.RangeValues[1], s.RangeValues[0]
		}
	} else {
		s.Value = s.clampSnap(s.Value)
	}
}

func (s *Slider) setRangeInternal(lo, hi float64, fireChange, fireComplete bool) {
	lo = s.clampSnap(lo)
	hi = s.clampSnap(hi)
	if lo > hi {
		lo, hi = hi, lo
	}
	s.lastInteractLo, s.lastInteractHi = lo, hi
	s.hasLastInteract = true
	changed := !almostEq(s.RangeValues[0], lo) || !almostEq(s.RangeValues[1], hi)

	if s.Controlled && (fireChange || fireComplete) {
		// user interaction under controlled: do not write, only notify
		if fireChange && changed && s.OnRangeChange != nil {
			s.OnRangeChange(lo, hi)
		}
		if fireChange && changed && s.OnChange != nil {
			if s.activeHandle == 1 {
				s.OnChange(hi)
			} else {
				s.OnChange(lo)
			}
		}
		if fireComplete && s.OnChangeComplete != nil {
			s.OnChangeComplete([]float64{lo, hi})
		}
		s.applyChrome()
		return
	}
	if changed {
		s.RangeValues[0], s.RangeValues[1] = lo, hi
	}
	s.applyChrome()
	if fireChange && changed {
		if s.OnRangeChange != nil {
			s.OnRangeChange(lo, hi)
		}
		if s.OnChange != nil {
			if s.activeHandle == 1 {
				s.OnChange(hi)
			} else {
				s.OnChange(lo)
			}
		}
	}
	if fireComplete && s.OnChangeComplete != nil {
		s.OnChangeComplete([]float64{lo, hi})
	}
}

func (s *Slider) commitSingle(v float64, fireChange, fireComplete bool) {
	v = s.clampSnap(v)
	s.lastInteract = v
	s.hasLastInteract = true
	changed := !almostEq(s.Value, v)

	if s.Controlled && (fireChange || fireComplete) {
		if fireChange && changed && s.OnChange != nil {
			s.OnChange(v)
		}
		if fireComplete && s.OnChangeComplete != nil {
			s.OnChangeComplete([]float64{v})
		}
		s.applyChrome()
		return
	}
	if changed {
		s.Value = v
	}
	s.applyChrome()
	if fireChange && changed && s.OnChange != nil {
		s.OnChange(v)
	}
	if fireComplete && s.OnChangeComplete != nil {
		s.OnChangeComplete([]float64{v})
	}
}

func (s *Slider) tipValue() float64 {
	if s.Range {
		lo, hi := s.Values()
		if s.activeHandle == 1 {
			return hi
		}
		return lo
	}
	return s.Value
}

func (s *Slider) formatTip(v float64) string {
	if s.TooltipFormatterNull {
		return ""
	}
	if s.TooltipFormatter != nil {
		return s.TooltipFormatter(v)
	}
	// trim trailing zeros for clean display
	if math.Abs(v-math.Round(v)) < 1e-9 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%g", v)
}

func (s *Slider) rebuild() {
	s.resolveMetrics()
	s.reclampAll()
	if s.Root == nil {
		h := &sliderHost{Slider: s}
		h.Init(h)
		h.Hit = core.HitTarget
		h.Cursor = core.CursorPointer
		s.Root = h
	} else {
		s.Root.Slider = s
	}
	s.applyA11y()
	s.applyChrome()
	if s.Root != nil {
		s.Root.MarkNeedsLayout()
		s.Root.MarkNeedsPaint()
	}
}

func (s *Slider) applyChrome() {
	s.resolveMetrics()
	s.tipVisible = s.TooltipVisible()
	s.tipText = s.formatTip(s.tipValue())
	if s.Root != nil {
		if s.Disabled {
			s.Root.Cursor = core.CursorDefault
		} else {
			s.Root.Cursor = core.CursorPointer
		}
		s.Root.MarkNeedsPaint()
	}
}

func (s *Slider) applyA11y() {
	if s.Root == nil {
		return
	}
	s.Root.Base().Role = "slider"
	label := s.AriaLabel
	if label == "" {
		label = "Slider"
	}
	s.Root.Base().Label = label
}

// ratioOf maps value → 0..1 along the rail (ignores Reverse in P0).
func (s *Slider) ratioOf(v float64) float64 {
	min, max := s.minMax()
	if max <= min {
		return 0
	}
	r := (v - min) / (max - min)
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// valueFromRatio maps 0..1 → value with snap.
func (s *Slider) valueFromRatio(r float64) float64 {
	min, max := s.minMax()
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	return s.clampSnap(min + r*(max-min))
}

// markLabelSpace returns extra cross-axis space for marks labels.
func (s *Slider) markLabelSpace() float64 {
	if len(s.Marks) == 0 {
		return 0
	}
	return DefaultSliderMarkLabelGap + DefaultSliderMarkLabelHeight
}

// ---------- keyboard ----------

// HandleKey implements keyboard adjustment (antd keyboard, default true).
func (s *Slider) HandleKey(ev *core.KeyEvent) bool {
	if s == nil || s.Disabled || ev == nil || ev.Type != core.KeyDown {
		return false
	}
	if s.OnKeyDown != nil {
		s.OnKeyDown(ev)
		if ev.Handled {
			return true
		}
	}
	if !s.Keyboard {
		return false
	}
	step := s.stepOr(DefaultSliderStep)
	if s.StepNull {
		// move to next/prev mark
		return s.handleKeyMarks(ev)
	}
	if step <= 0 {
		step = DefaultSliderStep
	}
	min, max := s.minMax()
	delta := 0.0
	switch ev.Key {
	case "ArrowRight", "Right", "ArrowUp", "Up":
		delta = step
	case "ArrowLeft", "Left", "ArrowDown", "Down":
		delta = -step
	case "Home":
		s.applyKeyValue(min)
		ev.Handled = true
		return true
	case "End":
		s.applyKeyValue(max)
		ev.Handled = true
		return true
	case "PageUp":
		delta = step * 2
	case "PageDown":
		delta = -step * 2
	default:
		return false
	}
	if s.Range {
		lo, hi := s.Values()
		if s.activeHandle == 1 {
			s.activeHandle = 1
			s.setRangeInternal(lo, hi+delta, true, true)
		} else {
			s.activeHandle = 0
			s.setRangeInternal(lo+delta, hi, true, true)
		}
	} else {
		s.commitSingle(s.Value+delta, true, true)
	}
	ev.Handled = true
	return true
}

func (s *Slider) handleKeyMarks(ev *core.KeyEvent) bool {
	if len(s.Marks) == 0 {
		return false
	}
	cur := s.tipValue()
	idx := 0
	best := math.Abs(s.Marks[0].Value - cur)
	for i, m := range s.Marks {
		d := math.Abs(m.Value - cur)
		if d < best {
			best, idx = d, i
		}
	}
	switch ev.Key {
	case "ArrowRight", "Right", "ArrowUp", "Up", "PageUp":
		if idx < len(s.Marks)-1 {
			idx++
		}
	case "ArrowLeft", "Left", "ArrowDown", "Down", "PageDown":
		if idx > 0 {
			idx--
		}
	case "Home":
		idx = 0
	case "End":
		idx = len(s.Marks) - 1
	default:
		return false
	}
	s.applyKeyValue(s.Marks[idx].Value)
	ev.Handled = true
	return true
}

func (s *Slider) applyKeyValue(v float64) {
	if s.Range {
		lo, hi := s.Values()
		if s.activeHandle == 1 {
			s.setRangeInternal(lo, v, true, true)
		} else {
			s.setRangeInternal(v, hi, true, true)
		}
		return
	}
	s.commitSingle(v, true, true)
}

// ---------- host node ----------

func (h *sliderHost) TypeID() string { return "kit.Slider" }

func (h *sliderHost) Layout(c core.Constraints) core.Size {
	s := h.Slider
	if s == nil {
		out := c.Tighten(core.Size{Width: DefaultSliderFallbackWidth, Height: DefaultSliderControlSize})
		h.SetSize(out)
		return out
	}
	s.resolveMetrics()
	vert := s.IsVertical()
	markExtra := s.markLabelSpace()
	hs := s.handleSize
	if hs < s.controlSize {
		hs = s.controlSize
	}
	// Hit padding expands the box slightly so hit ≥ visual handle.
	pad := DefaultSliderHandleHitPadding
	cross := hs + pad*2
	if cross < s.controlSize+pad*2 {
		cross = s.controlSize + pad*2
	}

	var w, ht float64
	if vert {
		// vertical: width = cross + marks, height = preferred/fill
		w = cross + markExtra
		ht = s.Height
		if ht <= 0 {
			if c.HasBoundedHeight() && c.MaxHeight < core.Unbounded {
				ht = c.MaxHeight
			} else {
				ht = DefaultSliderFallbackHeight
			}
		}
		// prefer tight parent height
		if c.MinHeight == c.MaxHeight && c.HasBoundedHeight() && c.MaxHeight >= hs && c.MaxHeight < core.Unbounded {
			ht = c.MaxHeight
		}
	} else {
		// horizontal: height = cross + marks, width = preferred/fill
		ht = cross + markExtra
		w = s.Width
		if w <= 0 {
			if c.HasBoundedWidth() && c.MaxWidth < core.Unbounded {
				w = c.MaxWidth
			} else {
				w = DefaultSliderFallbackWidth
			}
		}
		if c.MinWidth == c.MaxWidth && c.HasBoundedWidth() && c.MaxWidth >= hs && c.MaxWidth < core.Unbounded {
			w = c.MaxWidth
		}
		// gallery StretchChild may pin height
		if c.MinHeight == c.MaxHeight && c.HasBoundedHeight() && c.MaxHeight >= cross && c.MaxHeight < core.Unbounded {
			// keep markExtra if marks present
			if markExtra > 0 && c.MaxHeight < ht {
				// parent tighter than marks need — still honor parent
				ht = c.MaxHeight
			} else if markExtra == 0 {
				ht = c.MaxHeight
			}
		}
	}
	out := c.Tighten(core.Size{Width: w, Height: ht})
	h.SetSize(out)
	return out
}

func (h *sliderHost) railRect() (x, y, w, ht float64) {
	s := h.Slider
	sz := h.Size()
	hs := s.handleSize
	pad := DefaultSliderHandleHitPadding
	// rail is inset by half handle so handle center stays inside
	inset := hs/2 + pad
	if s.IsVertical() {
		x = pad + (hs-s.railSize)/2
		y = inset
		w = s.railSize
		ht = sz.Height - inset*2
		if ht < 0 {
			ht = 0
		}
		return
	}
	x = inset
	y = pad + (hs-s.railSize)/2
	w = sz.Width - inset*2
	ht = s.railSize
	if w < 0 {
		w = 0
	}
	return
}

func (h *sliderHost) handleCenter(v float64) (cx, cy float64) {
	s := h.Slider
	rx, ry, rw, rh := h.railRect()
	r := s.ratioOf(v)
	hs := s.handleSize
	pad := DefaultSliderHandleHitPadding
	if s.IsVertical() {
		// bottom = min, top = max (antd default)
		cx = pad + hs/2
		cy = ry + rh*(1-r)
		return
	}
	cx = rx + rw*r
	cy = pad + hs/2
	return
}

func (h *sliderHost) valueFromLocal(lx, ly float64) float64 {
	s := h.Slider
	rx, ry, rw, rh := h.railRect()
	var r float64
	if s.IsVertical() {
		if rh < 1 {
			return s.Min
		}
		// y increases downward; bottom=min
		r = 1 - (ly-ry)/rh
	} else {
		if rw < 1 {
			return s.Min
		}
		r = (lx - rx) / rw
	}
	return s.valueFromRatio(r)
}

func (h *sliderHost) nearestHandle(v float64) int {
	s := h.Slider
	if !s.Range {
		return 0
	}
	lo, hi := s.Values()
	if math.Abs(v-lo) <= math.Abs(v-hi) {
		return 0
	}
	return 1
}

func (h *sliderHost) Paint(pc *core.PaintContext) {
	if pc == nil || h.Slider == nil {
		return
	}
	s := h.Slider
	s.resolveMetrics()
	sz := h.Size()
	if sz.Width < 1 || sz.Height < 1 {
		return
	}

	disabled := s.Disabled
	hovered := s.hovering || s.dragging
	railC := s.railBg
	trackC := s.trackBg
	if hovered && !disabled {
		railC = s.railHover
		trackC = s.trackHover
	}
	if disabled {
		railC = s.disabledTrack
		if railC.A < 0.02 {
			railC = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		trackC = s.disabledHandle
		if trackC.A < 0.05 {
			trackC = render.RGBA{R: 0, G: 0, B: 0, A: 0.15}
		}
	}

	rx, ry, rw, rh := h.railRect()
	rr := s.railSize / 2
	// rail
	pc.FillLocalRoundRect(rx, ry, rw, rh, rr, railC)

	// track segment
	var t0, t1 float64
	if s.Range {
		lo, hi := s.Values()
		t0, t1 = s.ratioOf(lo), s.ratioOf(hi)
	} else {
		t0, t1 = 0, s.ratioOf(s.Value)
		if !s.Included && len(s.Marks) > 0 {
			// included=false: no continuous track fill to value (antd: marks independent)
			t0, t1 = 0, 0
		}
	}
	if t1 < t0 {
		t0, t1 = t1, t0
	}
	if t1 > t0 {
		if s.IsVertical() {
			// vertical: ratio 0 at bottom
			y0 := ry + rh*(1-t1)
			y1 := ry + rh*(1-t0)
			pc.FillLocalRoundRect(rx, y0, rw, y1-y0, rr, trackC)
		} else {
			x0 := rx + rw*t0
			x1 := rx + rw*t1
			pc.FillLocalRoundRect(x0, ry, x1-x0, rh, rr, trackC)
		}
	}

	// step dots
	if s.Dots {
		step := s.stepOr(0)
		min, max := s.minMax()
		if step > 0 {
			for v := min; v <= max+step*0.5; v += step {
				if v > max {
					v = max
				}
				h.paintDot(pc, v, disabled)
				if v >= max {
					break
				}
			}
		}
	}

	// marks points + labels
	for _, m := range s.Marks {
		h.paintDot(pc, m.Value, disabled)
		h.paintMarkLabel(pc, m)
	}

	// handles
	if s.Range {
		lo, hi := s.Values()
		h.paintHandle(pc, lo, s.activeHandle == 0, disabled)
		h.paintHandle(pc, hi, s.activeHandle == 1, disabled)
	} else {
		h.paintHandle(pc, s.Value, true, disabled)
	}

	// tooltip bubble
	if s.TooltipVisible() {
		txt := s.formatTip(s.tipValue())
		if txt != "" {
			cx, cy := h.handleCenter(s.tipValue())
			s.tipX, s.tipY = cx, cy
			h.paintTooltip(pc, cx, cy, txt)
		}
	}

	// focus ring on active handle
	if s.focused && s.focusVisible && !disabled {
		cx, cy := h.handleCenter(s.tipValue())
		out := DefaultSliderFocusOutset
		r := s.handleSize/2 + out
		pc.StrokeLocalCircle(cx, cy, r, 1.5, s.focusRing)
	}
}

func (h *sliderHost) paintDot(pc *core.PaintContext, v float64, disabled bool) {
	s := h.Slider
	cx, cy := h.handleCenter(v)
	// snap to rail center
	rx, ry, rw, rh := h.railRect()
	if s.IsVertical() {
		cx = rx + rw/2
	} else {
		cy = ry + rh/2
	}
	r := s.dotSize / 2
	fill := s.handleFill
	border := s.dotBorder
	// active when within track range
	active := false
	if s.Range {
		lo, hi := s.Values()
		active = v >= lo-1e-9 && v <= hi+1e-9
	} else if s.Included {
		active = v <= s.Value+1e-9
	}
	if active && !disabled {
		border = s.dotActive
	}
	if disabled {
		border = s.disabledHandle
	}
	pc.FillLocalCircle(cx, cy, r, fill)
	pc.StrokeLocalCircle(cx, cy, r, 1, border)
}

func (h *sliderHost) paintMarkLabel(pc *core.PaintContext, m SliderMark) {
	s := h.Slider
	if m.Label == "" || pc.DC == nil {
		return
	}
	cx, cy := h.handleCenter(m.Value)
	face := s.Face
	fs := s.theme().SizeOr(core.TokenFontSize, 14)
	if face != nil {
		if src := face.Source(); src != nil {
			face = src.Face(fs)
		}
	}
	col := m.Color
	if col.A < 0.05 {
		col = s.markColor
	}
	if s.Disabled {
		col = s.disabledHandle
	}
	var tw float64
	if face != nil {
		pc.DC.SetFont(face)
		tw = face.Advance(m.Label)
	} else {
		tw = fs * float64(len([]rune(m.Label))) * 0.5
	}
	pad := DefaultSliderHandleHitPadding
	hs := s.handleSize
	if s.IsVertical() {
		// labels to the right of rail
		x := pad + hs + DefaultSliderMarkLabelGap
		y := cy + fs*0.35
		pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
		pc.DC.DrawString(m.Label, pc.Origin.X+x, pc.Origin.Y+y)
		return
	}
	// below rail
	x := cx - tw/2
	y := pad + hs + DefaultSliderMarkLabelGap + fs*0.85
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
	pc.DC.DrawString(m.Label, pc.Origin.X+x, pc.Origin.Y+y)
}

func (h *sliderHost) paintHandle(pc *core.PaintContext, v float64, active, disabled bool) {
	s := h.Slider
	cx, cy := h.handleCenter(v)
	r := s.handleSize / 2
	if (s.hovering || s.dragging) && active && !disabled {
		r = s.handleHover / 2
	}
	fill := s.handleFill
	stroke := s.handleStroke
	lw := s.handleLine
	if active && (s.dragging || s.focused) && !disabled {
		stroke = s.handleActive
	}
	if disabled {
		stroke = s.disabledHandle
		if stroke.A < 0.1 {
			stroke = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		}
	}
	pc.FillLocalCircle(cx, cy, r, fill)
	pc.StrokeLocalCircle(cx, cy, r, lw, stroke)
}

func (h *sliderHost) paintTooltip(pc *core.PaintContext, cx, cy float64, txt string) {
	s := h.Slider
	if pc.DC == nil || txt == "" {
		return
	}
	face := s.Face
	fs := s.theme().SizeOr(core.TokenFontSize, 14)
	if face != nil {
		if src := face.Source(); src != nil {
			face = src.Face(fs)
		}
	}
	var tw float64
	if face != nil {
		pc.DC.SetFont(face)
		tw = face.Advance(txt)
	} else {
		tw = fs * float64(len([]rune(txt))) * 0.55
	}
	padX := DefaultSliderTooltipPadX
	padY := DefaultSliderTooltipPadY
	bw := tw + padX*2
	bh := fs + padY*2
	// default PlaceTop: above handle
	bx := cx - bw/2
	by := cy - s.handleSize/2 - 6 - bh
	switch s.TooltipPlacement {
	case primitive.PlaceBottom:
		by = cy + s.handleSize/2 + 6
	case primitive.PlaceLeft:
		bx = cx - s.handleSize/2 - 6 - bw
		by = cy - bh/2
	case primitive.PlaceRight:
		bx = cx + s.handleSize/2 + 6
		by = cy - bh/2
	}
	bg := render.RGBA{R: 0, G: 0, B: 0, A: 0.85}
	pc.FillLocalRoundRect(bx, by, bw, bh, 4, bg)
	fg := s.theme().Color(core.TokenColorTextInverse)
	if fg.A < 0.5 {
		fg = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	tx := bx + padX
	ty := by + padY + fs*0.85
	if face != nil {
		pc.DC.SetFont(face)
	}
	pc.DC.SetRGBA(fg.R, fg.G, fg.B, fg.A)
	pc.DC.DrawString(txt, pc.Origin.X+tx, pc.Origin.Y+ty)
}

func (h *sliderHost) HitTest(p core.Point) core.Node {
	if h.LocalBounds().Contains(p) {
		return h
	}
	return nil
}

func (h *sliderHost) HandlePointer(ev *core.PointerEvent) {
	if h == nil || ev == nil || h.Slider == nil {
		return
	}
	s := h.Slider
	if s.Disabled {
		return
	}
	abs := core.AbsoluteBounds(h)
	lx := ev.X - abs.Min.X
	ly := ev.Y - abs.Min.Y
	switch ev.Type {
	case core.PointerMove:
		if s.dragging {
			h.applyPointer(lx, ly, true, false)
			ev.Handled = true
			return
		}
		// hover preview for tooltip
		was := s.hovering
		s.hovering = true
		// pick nearest handle under cursor for tip
		v := h.valueFromLocal(lx, ly)
		s.activeHandle = h.nearestHandle(v)
		if !was {
			s.applyChrome()
		} else {
			s.Root.MarkNeedsPaint()
		}
		ev.Handled = true
	case core.PointerDown:
		s.dragging = true
		s.hovering = true
		v := h.valueFromLocal(lx, ly)
		// marks click: if near a mark, snap to it
		if m, ok := h.markNear(lx, ly); ok {
			v = m.Value
		}
		if s.Range {
			s.activeHandle = h.nearestHandle(v)
			lo, hi := s.Values()
			if s.activeHandle == 1 {
				h.applyRangeHandle(lo, v, true, false)
			} else {
				h.applyRangeHandle(v, hi, true, false)
			}
		} else {
			s.commitSingle(v, true, false)
		}
		ev.Handled = true
	case core.PointerUp, core.PointerCancel:
		if s.dragging {
			s.dragging = false
			if s.OnChangeComplete != nil {
				if s.Range {
					lo, hi := s.Values()
					if s.hasLastInteract {
						lo, hi = s.lastInteractLo, s.lastInteractHi
					}
					s.OnChangeComplete([]float64{lo, hi})
				} else {
					v := s.Value
					if s.hasLastInteract {
						v = s.lastInteract
					}
					s.OnChangeComplete([]float64{v})
				}
			}
			s.applyChrome()
		}
		ev.Handled = true
	}
}

func (h *sliderHost) applyPointer(lx, ly float64, fireChange, fireComplete bool) {
	s := h.Slider
	v := h.valueFromLocal(lx, ly)
	if s.Range {
		lo, hi := s.Values()
		if s.activeHandle == 1 {
			h.applyRangeHandle(lo, v, fireChange, fireComplete)
		} else {
			h.applyRangeHandle(v, hi, fireChange, fireComplete)
		}
		return
	}
	s.commitSingle(v, fireChange, fireComplete)
}

func (h *sliderHost) applyRangeHandle(lo, hi float64, fireChange, fireComplete bool) {
	s := h.Slider
	// prevent crossing: clamp active against the other
	if s.activeHandle == 1 && hi < lo {
		hi = lo
	}
	if s.activeHandle == 0 && lo > hi {
		lo = hi
	}
	s.setRangeInternal(lo, hi, fireChange, fireComplete)
}

func (h *sliderHost) markNear(lx, ly float64) (SliderMark, bool) {
	s := h.Slider
	if len(s.Marks) == 0 {
		return SliderMark{}, false
	}
	const hitR = 10.0
	bestI := -1
	bestD := hitR
	for i, m := range s.Marks {
		cx, cy := h.handleCenter(m.Value)
		dx, dy := lx-cx, ly-cy
		d := math.Hypot(dx, dy)
		if d <= bestD {
			bestD = d
			bestI = i
		}
	}
	if bestI < 0 {
		return SliderMark{}, false
	}
	return s.Marks[bestI], true
}

func (h *sliderHost) SetHovered(hov bool) {
	s := h.Slider
	if s == nil {
		return
	}
	if s.hovering == hov {
		return
	}
	s.hovering = hov
	s.applyChrome()
}

func (h *sliderHost) CanFocus() bool {
	return h.Slider != nil && !h.Slider.Disabled
}

func (h *sliderHost) IsFocused() bool {
	if h.Slider == nil {
		return false
	}
	return h.Slider.focused
}

func (h *sliderHost) SetFocused(f bool) {
	s := h.Slider
	if s == nil || s.focused == f {
		return
	}
	s.focused = f
	if !f {
		s.focusVisible = false
		if s.OnBlur != nil {
			s.OnBlur()
		}
	} else if s.OnFocus != nil {
		s.OnFocus()
	}
	s.applyChrome()
}

func (h *sliderHost) SetFocusVisible(v bool) {
	s := h.Slider
	if s == nil || s.focusVisible == v {
		return
	}
	s.focusVisible = v
	s.applyChrome()
}

func (h *sliderHost) HandleKey(ev *core.KeyEvent) {
	if h.Slider == nil {
		return
	}
	if h.Slider.HandleKey(ev) && ev != nil {
		ev.Handled = true
	}
}
