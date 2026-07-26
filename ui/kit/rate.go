package kit

import (
	"fmt"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Rate defaults — components/rate/style prepareComponentToken.
// docs/antd/rate.md §6.2 / §6.10
//
//	starSize   = controlHeight   * 0.625 → 20
//	starSizeSM = controlHeightSM * 0.625 → 15
//	starSizeLG = controlHeightLG * 0.625 → 25
//	starColor  = yellow6 ≈ #FADB14
//	starBg     = colorFillContent ≈ colorFillSecondary
//	gap        = marginXS (antd 8; kit DefaultRateStarGap)
const (
	DefaultRateCount       = 5
	DefaultRateStarSize    = 20.0 // middle
	DefaultRateStarSizeSM  = 15.0 // small
	DefaultRateStarSizeLG  = 25.0 // large
	DefaultRateStarGap     = 8.0  // antd marginXS between stars
	DefaultRateStarColor   = "#FADB14"
	DefaultRateCharacter   = "★"
	DefaultRateFocusOutset = 1.5
)

// RateSize is antd size: small | middle | large.
type RateSize int

const (
	RateMiddle RateSize = iota // starSize 20
	RateSmall                  // starSize 15
	RateLarge                  // starSize 25
)

// RateCharacterFunc renders the character for star index (0-based),
// matching antd character={({ index }) => …} for string characters.
type RateCharacterFunc func(index int) string

// Rate is Ant Design Rate (star rating).
//
//	Flex Row (Root, role=radiogroup)
//	  └─ rateStar × count (role=radio)
//	       paint: empty / half-clip / full
//
// Product contract: docs/antd/rate.md §6 (P0 DoD).
// Value is float64 so allowHalf can express n.5.
type Rate struct {
	Root  *primitive.Flex
	stars []*rateStar

	// Value is the committed score (0..Count; half steps when AllowHalf).
	Value float64
	// Count is star total (antd count, default 5).
	Count int
	// AllowClear: click current value again → 0 (antd default true).
	AllowClear bool
	// AllowHalf enables half-star hit/paint.
	AllowHalf bool
	// Disabled: read-only, no interaction.
	Disabled bool
	// Controlled: activation only fires OnChange; parent must SetValue.
	Controlled bool
	// Keyboard enables arrow-key adjustment (antd default true).
	Keyboard bool
	// Size ladder (antd SizeType).
	Size RateSize
	// Character is the glyph for every star (default ★).
	Character string
	// CharacterAt overrides Character when non-nil (index 0-based).
	CharacterAt RateCharacterFunc
	// Tooltips are per-star hover titles (index 0 → star 1).
	Tooltips []string

	OnChange      func(value float64)
	OnHoverChange func(value float64)
	OnFocus       func()
	OnBlur        func()
	OnKeyDown     func(ev *core.KeyEvent)

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// hover
	hovering   bool
	hoverValue float64

	// cached metrics for applyChrome
	starSize  float64
	starGap   float64
	starFill  render.RGBA
	starEmpty render.RGBA
}

// rateStar is one focusable star: hit == layout == paint box of starSize².
// Left half → index-0.5 when AllowHalf; right half → index (1-based).
type rateStar struct {
	core.NodeBase
	rate  *Rate
	index int // 1-based

	size   float64
	char   string
	face   text.Face
	amount float64 // 0 | 0.5 | 1 for paint
	filled render.RGBA
	empty  render.RGBA

	hovered      bool
	focused      bool
	focusVisible bool
}

// NewRate creates a Rate with Ant defaults (value 0, count 5, allowClear true).
func NewRate() *Rate {
	r := &Rate{
		Count:      DefaultRateCount,
		Size:       RateMiddle,
		AllowClear: true,
		Keyboard:   true,
	}
	r.rebuild()
	return r
}

// Node returns the root row (stable across SetValue).

// ensureBuilt materializes the control tree if missing (#9).
func (r *Rate) ensureBuilt() {
	if r == nil {
		return
	}
	if r.Root == nil {
		r.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (r *Rate) structureChange() {
	if r == nil {
		return
	}
	r.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (r *Rate) chromeChange() {
	if r == nil {
		return
	}
	r.ensureBuilt()
	r.rebuild()
}

func (r *Rate) Node() core.Node {
	r.ensureBuilt()
	return r.Root
}

// StarNodes returns each star node (tests / geometry).
func (r *Rate) StarNodes() []core.Node {
	if r.Root == nil {
		r.rebuild()
	}
	out := make([]core.Node, len(r.stars))
	for i, s := range r.stars {
		out[i] = s
	}
	return out
}

// StarSize returns the resolved star edge length for the current size ladder.
func (r *Rate) StarSize() float64 {
	if r.starSize <= 0 {
		r.resolveMetrics()
	}
	return r.starSize
}

// StarGap returns the resolved gap between stars.
func (r *Rate) StarGap() float64 {
	if r.starGap <= 0 {
		r.resolveMetrics()
	}
	return r.starGap
}

// StarFillColor returns the active star color (tests / L2).
func (r *Rate) StarFillColor() render.RGBA {
	r.resolveMetrics()
	return r.starFill
}

// StarEmptyColor returns the empty star color (tests / L2).
func (r *Rate) StarEmptyColor() render.RGBA {
	r.resolveMetrics()
	return r.starEmpty
}

// HoverValue is the preview value while hovering (0 when not hovering).
func (r *Rate) HoverValue() float64 {
	if !r.hovering {
		return 0
	}
	return r.hoverValue
}

// HoverTooltip returns tooltips[n-1] for the current hover/display value star index.
func (r *Rate) HoverTooltip() string {
	v := r.displayValue()
	if v <= 0 || len(r.Tooltips) == 0 {
		return ""
	}
	idx := int(math.Ceil(v)) - 1
	if idx < 0 || idx >= len(r.Tooltips) {
		return ""
	}
	return r.Tooltips[idx]
}

// TooltipAt returns tooltips[i] (0-based) or "".
func (r *Rate) TooltipAt(i int) string {
	if i < 0 || i >= len(r.Tooltips) {
		return ""
	}
	return r.Tooltips[i]
}

// SetValue writes the committed value without firing OnChange.
func (r *Rate) SetValue(v float64) {
	v = r.clamp(v)
	if r.Value == v {
		r.applyChrome()
		return
	}
	r.Value = v
	r.applyChrome()
	r.applyA11y()
}

// ValueOf is an alias for reading Value (API symmetry with other controls).
func (r *Rate) ValueOf() float64 { return r.Value }

// SetDefaultValue sets the initial value when not controlled.
func (r *Rate) SetDefaultValue(v float64) {
	if r.Controlled {
		return
	}
	r.SetValue(v)
}

// SetControlled marks parent-owned value (antd value={…}).
func (r *Rate) SetControlled(c bool) { r.Controlled = c }

// SetCount sets star total (antd count, default 5).
func (r *Rate) SetCount(n int) {
	if n <= 0 {
		n = DefaultRateCount
	}
	if r.Count == n && len(r.stars) == n {
		return
	}
	r.Count = n
	if r.Value > float64(n) {
		r.Value = float64(n)
	}
	r.rebuild()
}

// SetAllowClear toggles re-click clear (default true).
func (r *Rate) SetAllowClear(v bool) {
	r.AllowClear = v
}

// SetAllowHalf enables half-star interaction/paint.
func (r *Rate) SetAllowHalf(v bool) {
	if r.AllowHalf == v {
		return
	}
	r.AllowHalf = v
	r.applyChrome()
}

// SetSize sets the size ladder.
func (r *Rate) SetSize(sz RateSize) {
	if r.Size == sz {
		return
	}
	r.Size = sz
	r.rebuild()
}

// SetCharacter sets a uniform character (antd character as string).
func (r *Rate) SetCharacter(ch string) {
	if ch == "" {
		ch = DefaultRateCharacter
	}
	if r.Character == ch && r.CharacterAt == nil {
		return
	}
	r.Character = ch
	r.CharacterAt = nil
	r.rebuild()
}

// SetCharacterAt sets per-index character renderer (antd character function).
func (r *Rate) SetCharacterAt(fn RateCharacterFunc) {
	r.CharacterAt = fn
	r.rebuild()
}

// SetTooltips sets per-star hover titles.
func (r *Rate) SetTooltips(tips []string) {
	r.Tooltips = tips
	r.applyA11y()
}

// SetKeyboard enables/disables arrow-key adjustment (default true).
func (r *Rate) SetKeyboard(v bool) {
	r.Keyboard = v
}

// SetDisabled toggles read-only mode.
func (r *Rate) SetDisabled(d bool) {
	r.Disabled = d
	if d {
		r.clearHover()
	}
	r.applyChrome()
	r.applyA11y()
}

// SetOnChange sets the change callback.
func (r *Rate) SetOnChange(fn func(float64)) { r.OnChange = fn }

// SetOnHoverChange sets the hover-preview callback.
func (r *Rate) SetOnHoverChange(fn func(float64)) { r.OnHoverChange = fn }

// SetOnFocus sets the focus callback.
func (r *Rate) SetOnFocus(fn func()) { r.OnFocus = fn }

// SetOnBlur sets the blur callback.
func (r *Rate) SetOnBlur(fn func()) { r.OnBlur = fn }

// SetOnKeyDown sets the key callback (fires before internal handling).
func (r *Rate) SetOnKeyDown(fn func(*core.KeyEvent)) { r.OnKeyDown = fn }

// SetAriaLabel sets the radiogroup accessible name.
func (r *Rate) SetAriaLabel(name string) {
	r.AriaLabel = name
	r.applyA11y()
}

// SetFace sets the font face for star glyphs.
func (r *Rate) SetFace(face text.Face) {
	r.Face = face
	r.rebuild()
}

// SetTheme sets an explicit theme override.
func (r *Rate) SetTheme(th *core.Theme) {
	r.Theme = th
	r.applyChrome()
}

// SetStyle sets optional style overrides (Style.Text → star fill color).
func (r *Rate) SetStyle(st Style) {
	r.Style = st
	r.applyChrome()
}

func (r *Rate) theme() *core.Theme {
	var n core.Node
	if r.Root != nil {
		n = r.Root
	}
	return themeOf(r.Theme, n)
}

func (r *Rate) resolveMetrics() {
	th := r.theme()
	ch := th.SizeOr(core.TokenControlHeight, 32)
	chSM := th.SizeOr(core.TokenControlHeightSM, 24)
	chLG := th.SizeOr(core.TokenControlHeightLG, 40)
	switch r.Size {
	case RateSmall:
		r.starSize = chSM * 0.625
		if r.starSize <= 0 {
			r.starSize = DefaultRateStarSizeSM
		}
	case RateLarge:
		r.starSize = chLG * 0.625
		if r.starSize <= 0 {
			r.starSize = DefaultRateStarSizeLG
		}
	default:
		r.starSize = ch * 0.625
		if r.starSize <= 0 {
			r.starSize = DefaultRateStarSize
		}
	}
	// Prefer explicit DefaultRateStarGap (antd marginXS=8); TokenMarginXS in kit is 4.
	r.starGap = DefaultRateStarGap
	if g := th.SizeOr(core.TokenMarginSM, 0); g >= 8 {
		// TokenMarginSM=8 is the closest kit alias for antd marginXS.
		r.starGap = g
	}
	r.starFill = render.Hex(DefaultRateStarColor)
	if r.Style.Text.A > 0 {
		r.starFill = r.Style.Text
	}
	r.starEmpty = th.Color(core.TokenColorFillSecondary)
	if r.starEmpty.A < 0.04 {
		r.starEmpty = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	if r.Disabled {
		// Disabled: lower contrast fill, no hover high-light path.
		dis := th.Color(core.TokenColorDisabledText)
		if dis.A > 0 {
			// Blend star color toward disabled text alpha for filled stars.
			r.starFill = render.RGBA{
				R: r.starFill.R*0.45 + dis.R*0.55,
				G: r.starFill.G*0.45 + dis.G*0.55,
				B: r.starFill.B*0.45 + dis.B*0.55,
				A: 0.55,
			}
		}
	}
}

func (r *Rate) clamp(v float64) float64 {
	if v < 0 || math.IsNaN(v) {
		return 0
	}
	max := float64(r.count())
	if v > max {
		return max
	}
	if r.AllowHalf {
		// Snap to 0.5 steps.
		return math.Round(v*2) / 2
	}
	return math.Round(v)
}

func (r *Rate) count() int {
	if r.Count <= 0 {
		return DefaultRateCount
	}
	return r.Count
}

func (r *Rate) displayValue() float64 {
	if r.hovering && !r.Disabled {
		return r.hoverValue
	}
	return r.Value
}

func (r *Rate) charAt(zeroBased int) string {
	if r.CharacterAt != nil {
		if s := r.CharacterAt(zeroBased); s != "" {
			return s
		}
	}
	if r.Character != "" {
		return r.Character
	}
	return DefaultRateCharacter
}

func (r *Rate) rebuild() {
	r.resolveMetrics()
	n := r.count()
	r.Count = n

	if r.Root == nil {
		r.Root = primitive.Row()
	} else {
		r.Root.ClearChildren()
	}
	r.Root.Gap = r.starGap
	r.Root.CrossAlign = core.CrossCenter
	r.Root.MainAlign = core.MainStart
	r.Root.Hit = core.HitDefer

	r.stars = make([]*rateStar, n)
	for i := 1; i <= n; i++ {
		s := &rateStar{rate: r, index: i}
		s.Init(s)
		s.Hit = core.HitTarget
		s.Cursor = core.CursorPointer
		r.stars[i-1] = s
		r.Root.AddChild(s)
	}
	r.applyChrome()
	r.applyA11y()
	r.Root.MarkNeedsLayout()
	r.Root.MarkNeedsPaint()
}

func (r *Rate) applyChrome() {
	if r.Root == nil {
		return
	}
	r.resolveMetrics()
	r.Root.Gap = r.starGap
	dv := r.displayValue()
	for _, s := range r.stars {
		if s == nil {
			continue
		}
		s.size = r.starSize
		s.char = r.charAt(s.index - 1)
		s.face = r.Face
		s.filled = r.starFill
		s.empty = r.starEmpty
		s.amount = starAmount(dv, s.index)
		if r.Disabled {
			s.Cursor = core.CursorDefault
		} else {
			s.Cursor = core.CursorPointer
		}
		s.MarkNeedsPaint()
	}
	r.Root.MarkNeedsPaint()
}

func starAmount(display float64, index int) float64 {
	fi := float64(index)
	if display >= fi {
		return 1
	}
	if display >= fi-0.5 {
		return 0.5
	}
	return 0
}

func (r *Rate) applyA11y() {
	if r.Root == nil {
		return
	}
	r.Root.Base().Role = "radiogroup"
	r.Root.Base().Label = r.AriaLabel
	if r.AriaLabel == "" {
		r.Root.Base().Label = "Rate"
	}
	for _, s := range r.stars {
		if s == nil {
			continue
		}
		s.Base().Role = "radio"
		name := fmt.Sprintf("%d star", s.index)
		if s.index != 1 {
			name = fmt.Sprintf("%d stars", s.index)
		}
		if tip := r.TooltipAt(s.index - 1); tip != "" {
			name = tip
		}
		s.Base().Label = name
	}
}

// commitValue applies a user selection (click / keyboard).
func (r *Rate) commitValue(v float64) {
	if r.Disabled {
		return
	}
	v = r.clamp(v)
	if r.AllowClear && v > 0 && almostEq(v, r.Value) {
		v = 0
	}
	if r.Controlled {
		if r.OnChange != nil {
			r.OnChange(v)
		}
		return
	}
	if almostEq(r.Value, v) {
		return
	}
	r.Value = v
	r.applyChrome()
	r.applyA11y()
	if r.OnChange != nil {
		r.OnChange(v)
	}
}

func almostEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func (r *Rate) setHover(v float64) {
	if r.Disabled {
		return
	}
	v = r.clamp(v)
	changed := !r.hovering || !almostEq(r.hoverValue, v)
	r.hovering = true
	r.hoverValue = v
	if changed {
		r.applyChrome()
		if r.OnHoverChange != nil {
			r.OnHoverChange(v)
		}
	}
}

func (r *Rate) clearHover() {
	if !r.hovering {
		return
	}
	r.hovering = false
	r.hoverValue = 0
	r.applyChrome()
	if r.OnHoverChange != nil {
		r.OnHoverChange(0)
	}
}

// valueFromLocalX maps local X inside a star to the score for that star index.
func (r *Rate) valueFromLocalX(index int, lx, width float64) float64 {
	if r.AllowHalf && width > 0 && lx < width/2 {
		return float64(index) - 0.5
	}
	return float64(index)
}

// HandleKey implements keyboard adjustment (antd keyboard, default true).
// Host may call after focus; also used from tests. Returns true if handled.
func (r *Rate) HandleKey(ev *core.KeyEvent) bool {
	if r == nil || r.Disabled || ev == nil || ev.Type != core.KeyDown {
		return false
	}
	if r.OnKeyDown != nil {
		r.OnKeyDown(ev)
		if ev.Handled {
			return true
		}
	}
	if !r.Keyboard {
		return false
	}
	step := 1.0
	if r.AllowHalf {
		step = 0.5
	}
	var next float64
	switch ev.Key {
	case "ArrowRight", "Right", "ArrowUp", "Up":
		next = r.Value + step
	case "ArrowLeft", "Left", "ArrowDown", "Down":
		next = r.Value - step
	case "Home":
		next = 0
	case "End":
		next = float64(r.count())
	default:
		return false
	}
	r.commitValue(next)
	ev.Handled = true
	return true
}

// ---------- rateStar node ----------

func (s *rateStar) TypeID() string { return "kit.rateStar" }

func (s *rateStar) Layout(c core.Constraints) core.Size {
	sz := s.size
	if sz <= 0 && s.rate != nil {
		sz = s.rate.StarSize()
	}
	if sz <= 0 {
		sz = DefaultRateStarSize
	}
	out := c.Tighten(core.Size{Width: sz, Height: sz})
	s.SetSize(out)
	return out
}

func (s *rateStar) Paint(pc *core.PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	sz := s.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	ch := s.char
	if ch == "" {
		ch = DefaultRateCharacter
	}
	face := s.face
	fs := sz.Height
	if face != nil {
		// Prefer a face sized to star edge for crisp glyphs.
		if src := face.Source(); src != nil {
			face = src.Face(fs)
		}
	}
	dc := pc.DC
	if face != nil {
		dc.SetFont(face)
	}
	ascent := fs * 0.8
	if face != nil {
		ascent = face.Metrics().Ascent
	}
	// Center glyph roughly in the box.
	var textW float64
	if face != nil {
		textW = face.Advance(ch)
	} else {
		textW = fs * float64(len([]rune(ch))) * 0.6
	}
	x := (sz.Width - textW) / 2
	if x < 0 {
		x = 0
	}
	y := ascent + (sz.Height-fs)/2
	if y < ascent {
		y = ascent
	}
	// Empty / base layer.
	empty := s.empty
	if empty.A <= 0 {
		empty = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	dc.SetRGBA(empty.R, empty.G, empty.B, empty.A)
	dc.DrawString(ch, pc.Origin.X+x, pc.Origin.Y+y)

	amount := s.amount
	if amount <= 0 {
		s.paintFocus(pc, sz)
		return
	}
	filled := s.filled
	if filled.A <= 0 {
		filled = render.Hex(DefaultRateStarColor)
	}
	if amount >= 1 {
		dc.SetRGBA(filled.R, filled.G, filled.B, filled.A)
		dc.DrawString(ch, pc.Origin.X+x, pc.Origin.Y+y)
		s.paintFocus(pc, sz)
		return
	}
	// Half: clip left 50% and paint filled glyph (full glyph, clipped).
	pc.PushClipLocal(0, 0, sz.Width/2, sz.Height)
	dc.SetRGBA(filled.R, filled.G, filled.B, filled.A)
	dc.DrawString(ch, pc.Origin.X+x, pc.Origin.Y+y)
	pc.Pop()
	s.paintFocus(pc, sz)
}

func (s *rateStar) paintFocus(pc *core.PaintContext, sz core.Size) {
	if !s.focused || !s.focusVisible || pc == nil {
		return
	}
	col := render.Hex(DefaultRateStarColor)
	if s.rate != nil {
		col = s.rate.starFill
	}
	outset := DefaultRateFocusOutset
	pc.StrokeLocalRoundRect(-outset, -outset, sz.Width+2*outset, sz.Height+2*outset, 2, 1.0, col)
}

func (s *rateStar) HitTest(p core.Point) core.Node {
	if s.rate != nil && s.rate.Disabled {
		// Still hit-testable for layout identity, but pointer handler no-ops.
		// Disabled stars absorb nothing so parent can receive events if needed.
		return nil
	}
	if s.LocalBounds().Contains(p) {
		return s
	}
	return nil
}

func (s *rateStar) HandlePointer(ev *core.PointerEvent) {
	if s.rate == nil || s.rate.Disabled || ev == nil {
		return
	}
	abs := core.AbsoluteBounds(s)
	lx := ev.X - abs.Min.X
	w := s.Size().Width
	if w <= 0 {
		w = s.size
	}
	val := s.rate.valueFromLocalX(s.index, lx, w)
	switch ev.Type {
	case core.PointerMove:
		s.rate.setHover(val)
		ev.Handled = true
	case core.PointerDown:
		// Preview + pending commit on click.
		s.rate.setHover(val)
		ev.Handled = true
	case core.PointerUp, core.PointerCancel:
		ev.Handled = true
	}
}

func (s *rateStar) OnClick(ev *core.PointerEvent) {
	if s.rate == nil || s.rate.Disabled {
		return
	}
	lx := s.size / 2
	w := s.Size().Width
	if w <= 0 {
		w = s.size
	}
	if ev != nil {
		abs := core.AbsoluteBounds(s)
		lx = ev.X - abs.Min.X
	}
	val := s.rate.valueFromLocalX(s.index, lx, w)
	s.rate.commitValue(val)
	if ev != nil {
		ev.Handled = true
	}
}

func (s *rateStar) CanFocus() bool {
	return s.rate != nil && !s.rate.Disabled
}

func (s *rateStar) IsFocused() bool { return s.focused }

func (s *rateStar) SetFocused(f bool) {
	if s.focused == f {
		return
	}
	s.focused = f
	if !f {
		s.focusVisible = false
		if s.rate != nil && s.rate.OnBlur != nil {
			s.rate.OnBlur()
		}
	} else if s.rate != nil && s.rate.OnFocus != nil {
		s.rate.OnFocus()
	}
	s.MarkNeedsPaint()
}

func (s *rateStar) SetFocusVisible(v bool) {
	if s.focusVisible == v {
		return
	}
	s.focusVisible = v
	s.MarkNeedsPaint()
}

func (s *rateStar) SetHovered(h bool) {
	if s.hovered == h {
		return
	}
	s.hovered = h
	if s.rate == nil {
		s.MarkNeedsPaint()
		return
	}
	if h {
		// Tentative full-star preview; PointerMove refines half via local X.
		s.rate.setHover(float64(s.index))
	} else {
		// Leaving this star. Tree order is leave-old then enter-new in the same
		// DispatchPointer, so a sibling SetHovered(true) re-applies immediately
		// before paint. Leaving the whole Rate leaves hover cleared.
		s.rate.clearHover()
	}
	s.MarkNeedsPaint()
}

func (s *rateStar) HandleKey(ev *core.KeyEvent) {
	if s.rate == nil {
		return
	}
	if s.rate.HandleKey(ev) && ev != nil {
		ev.Handled = true
	}
}
