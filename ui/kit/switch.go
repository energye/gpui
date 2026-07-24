package kit

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Switch defaults — prepareComponentToken.
// docs/antd/switch.md §6.2 / §6.10
const (
	DefaultSwitchTrackHeight     = 22.0
	DefaultSwitchTrackMinWidth   = 44.0
	DefaultSwitchTrackHeightSM   = 16.0
	DefaultSwitchTrackMinWidthSM = 28.0
	DefaultSwitchTrackPadding    = 2.0
	DefaultSwitchInnerFont       = 12.0
	DefaultSwitchThumbDuration   = 0.3
)

// stackOffsetter is satisfied by primitive.PositionedAt hosts (SetStackOffset).
type stackOffsetter interface {
	SetStackOffset(x, y float64)
}

// Switch is an on/off toggle composed from Pressable + Decorated track + Stack.
//
//	Pressable (role=switch)
//	  └─ Decorated track (pill)
//	       └─ Stack
//	            ├─ PositionedAt(0,0) Canvas  — dual labels (checked/unchecked)
//	            └─ PositionedAt thumb
//	                 └─ Stack AlignCenter → spinner? (loading)
//
// Labels: both drawn every frame; position/alpha follow thumbPos (antd slide
// appear/hide). Vertical center uses DrawStringAnchored(ay=0.5) on free-slot
// mid-Y (= track mid) — general face-metrics centering, no optical constants.
//
// Product contract: docs/antd/switch.md §6 (P0 DoD).
type Switch struct {
	Root    *primitive.Pressable
	track   *primitive.Decorated
	stack   *primitive.Stack
	thumb   *primitive.Decorated
	label   *primitive.Canvas // full-track dual-label paint
	spinner *primitive.Canvas

	thumbHost core.Node
	labelHost core.Node

	Checked           bool
	Size              SwitchSize
	Disabled          bool
	Loading           bool
	Controlled        bool
	CheckedChildren   string
	UnCheckedChildren string
	AriaLabel         string
	OnChange          func(checked bool)
	OnClick           func(checked bool)
	Face              text.Face
	Theme             *core.Theme
	Style             Style

	trackW, trackH float64
	thumbSize      float64
	pad            float64
	labelColor     render.RGBA
	lastHovered    bool
	lastPressed    bool
	lastFocused    bool

	// Cached label paint metrics (invalidated on Face / children / size rebuild).
	// Avoids face.Glyphs/Advance every frame while the thumb slides.
	labelFace      text.Face
	labelChecked   labelPaintCache
	labelUnchecked labelPaintCache

	thumbPos  primitive.FloatAnim
	spinPhase float64
	boundTree *core.Tree
	// lastThumbW avoids redundant thumb.Layout during slide (rest size is fixed).
	lastThumbW float64
}

// labelPaintCache stores per-string draw metrics for free-slot labels.
type labelPaintCache struct {
	text    string
	advance float64
	// inkMidY is (inkMinY+inkMaxY)/2 relative to baseline; baseY = midY - inkMidY.
	inkMidY float64
	hasInk  bool
	ready   bool
}

// NewSwitch creates a Switch with Ant defaults (unchecked, medium, interactive).
func NewSwitch() *Switch {
	s := &Switch{Size: SwitchMedium}
	s.rebuild()
	return s
}

func (s *Switch) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

func (s *Switch) ChromeNode() core.Node {
	if s.track == nil {
		s.rebuild()
	}
	return s.track
}

func (s *Switch) IndicatorNode() core.Node { return s.ChromeNode() }

func (s *Switch) ThumbNode() core.Node {
	if s.thumb == nil {
		s.rebuild()
	}
	return s.thumb
}

func (s *Switch) SetChecked(v bool) {
	if s.Checked == v {
		s.applyChrome()
		return
	}
	s.Checked = v
	s.animateThumb()
	s.applyChrome()
	s.applyA11yName()
}

func (s *Switch) SetValue(v bool) {
	s.Controlled = true
	s.SetChecked(v)
}

func (s *Switch) Value() bool { return s.Checked }

func (s *Switch) SetDefaultChecked(v bool) {
	if s.Controlled {
		return
	}
	s.SetChecked(v)
}

func (s *Switch) SetDefaultValue(v bool) { s.SetDefaultChecked(v) }

func (s *Switch) SetControlled(c bool) { s.Controlled = c }

func (s *Switch) SetSize(sz SwitchSize) {
	if s.Size == sz {
		return
	}
	s.Size = sz
	s.rebuild()
}

func (s *Switch) SetDisabled(d bool) {
	s.Disabled = d
	if s.Root != nil {
		s.Root.SetDisabled(d || s.Loading)
	}
	s.applyChrome()
}

func (s *Switch) SetLoading(v bool) {
	if s.Loading == v {
		return
	}
	s.Loading = v
	if s.Root != nil {
		s.Root.SetDisabled(s.Disabled || s.Loading)
	}
	s.rebuild()
	if s.boundTree != nil {
		if v || s.thumbPos.Active() {
			s.boundTree.AddTicker(s)
		} else if !s.thumbPos.Active() {
			s.boundTree.RemoveTicker(s)
		}
	}
}

func (s *Switch) SetCheckedChildren(text string) {
	s.CheckedChildren = text
	s.rebuild()
}

func (s *Switch) SetUnCheckedChildren(text string) {
	s.UnCheckedChildren = text
	s.rebuild()
}

func (s *Switch) SetOnChange(fn func(bool)) { s.OnChange = fn }
func (s *Switch) SetOnClick(fn func(bool))  { s.OnClick = fn }

func (s *Switch) SetAriaLabel(name string) {
	s.AriaLabel = name
	s.applyA11yName()
}

func (s *Switch) SetFace(face text.Face) {
	s.Face = face
	if s.CheckedChildren != "" || s.UnCheckedChildren != "" {
		s.rebuild()
		return
	}
	if s.label != nil {
		s.label.MarkNeedsPaint()
	}
}

func (s *Switch) SetStyle(st Style) {
	s.Style = st
	if st.Face != nil {
		s.SetFace(st.Face)
	}
	s.applyChrome()
}

func (s *Switch) SetBackground(c render.RGBA) {
	s.Style.Background = c
	s.applyChrome()
}

func (s *Switch) SetActiveColor(c render.RGBA) {
	s.Style.BackgroundActive = c
	s.applyChrome()
}

func (s *Switch) AttachTicker(t *core.Tree) {
	if s == nil || t == nil {
		return
	}
	s.boundTree = t
	t.BindTicker(s, s.thumbPos.Active() || s.Loading)
}

func (s *Switch) Tick(dt float64) bool {
	if s == nil {
		return false
	}
	// FloatAnim.OnUpdate → applyThumbPad (paint-only). Do not re-Layout here.
	still := s.thumbPos.Tick(dt)
	if s.Loading {
		s.spinPhase += dt * 1.4
		if s.spinPhase > 1 {
			s.spinPhase -= 1
		}
		if s.spinner != nil {
			s.spinner.MarkNeedsPaint()
		}
		still = true
	}
	if !still && s.boundTree != nil {
		s.boundTree.RemoveTicker(s)
	}
	return still
}

func (s *Switch) SyncState() {
	if s.Root == nil {
		return
	}
	h, p := s.Root.State.Hovered, s.Root.State.Pressed
	f := s.Root.State.Focused && s.Root.State.FocusVisible
	if h == s.lastHovered && p == s.lastPressed && f == s.lastFocused {
		return
	}
	s.lastHovered, s.lastPressed, s.lastFocused = h, p, f
	s.applyThumbShape()
	s.applyChrome()
}

func (s *Switch) theme() *core.Theme {
	var n core.Node
	if s.Root != nil {
		n = s.Root
	}
	return themeOf(s.Theme, n)
}

func (s *Switch) applyA11yName() {
	if s.Root == nil {
		return
	}
	name := s.AriaLabel
	if name == "" {
		if s.Checked && s.CheckedChildren != "" {
			name = s.CheckedChildren
		} else if !s.Checked && s.UnCheckedChildren != "" {
			name = s.UnCheckedChildren
		}
	}
	s.Root.Base().Label = name
	s.Root.Base().Role = "switch"
}

func (s *Switch) fireToggle() {
	if s.Disabled || s.Loading {
		return
	}
	next := !s.Checked
	if !s.Controlled {
		s.Checked = next
		s.animateThumb()
		s.applyChrome()
		s.applyA11yName()
	}
	if s.OnClick != nil {
		s.OnClick(next)
	}
	if s.OnChange != nil {
		s.OnChange(next)
	}
}

func (s *Switch) innerFontSize() float64 {
	if s.Size == SwitchSmall {
		sm := s.theme().SizeOr(core.TokenFontSizeSM, DefaultSwitchInnerFont)
		if sm > 2 {
			return sm - 2
		}
		return 10
	}
	return s.theme().SizeOr(core.TokenFontSizeSM, DefaultSwitchInnerFont)
}

func (s *Switch) measureInnerText(label string) float64 {
	if label == "" {
		return 0
	}
	fs := s.innerFontSize()
	if face := switchFaceAtSize(s.Face, fs); face != nil {
		if w := face.Advance(label); w > 0 {
			return w + s.pad*2
		}
	}
	t := primitive.NewText(label)
	t.FontSize = fs
	t.Face = s.Face
	t.MaxLines = 1
	sz := t.Layout(core.Loose(1000, 100))
	w := sz.Width
	if s.Face == nil {
		guess := float64(len([]rune(label))) * fs * 0.65
		if guess > w {
			w = guess
		}
	}
	return w + 2
}

func switchFaceAtSize(face text.Face, size float64) text.Face {
	if face == nil {
		return nil
	}
	if size <= 0 {
		return face
	}
	if fs := face.Size(); fs > 0 && math.Abs(fs-size) < 0.25 {
		return face
	}
	src := face.Source()
	if src == nil {
		return face
	}
	return src.Face(size)
}

func (s *Switch) rebuild() {
	th := s.theme()
	s.pad = DefaultSwitchTrackPadding
	switch s.Size {
	case SwitchSmall:
		ch := th.SizeOr(core.TokenControlHeight, 32)
		s.trackH = ch / 2
		if s.trackH <= 0 {
			s.trackH = DefaultSwitchTrackHeightSM
		}
		s.thumbSize = s.trackH - 2*s.pad
		s.trackW = s.thumbSize*2 + s.pad*2
		if s.trackW < DefaultSwitchTrackMinWidthSM {
			s.trackW = DefaultSwitchTrackMinWidthSM
		}
	default:
		s.trackH = th.SizeOr(core.TokenSwitchHeight, DefaultSwitchTrackHeight)
		s.trackW = th.SizeOr(core.TokenSwitchWidth, DefaultSwitchTrackMinWidth)
		s.thumbSize = s.trackH - 2*s.pad
		minW := s.thumbSize*2 + s.pad*4
		if s.trackW < minW {
			s.trackW = minW
		}
	}

	// Track grows with children (antd innerMin/Max formula).
	innerMin := s.thumbSize / 2
	innerMax := s.thumbSize + s.pad*3
	maxTextW := s.measureInnerText(s.CheckedChildren)
	if w := s.measureInnerText(s.UnCheckedChildren); w > maxTextW {
		maxTextW = w
	}
	if maxTextW > 0 {
		need := maxTextW + innerMin + innerMax
		if need > s.trackW {
			s.trackW = need
		}
	}

	s.thumbPos.Duration = DefaultSwitchThumbDuration
	s.thumbPos.Easing = primitive.EaseInOutCubic
	if s.Checked {
		s.thumbPos.Snap(1)
	} else {
		s.thumbPos.Snap(0)
	}
	s.thumbPos.OnUpdate = func(float64) { s.applyThumbShape() }

	// Thumb.
	s.thumb = primitive.NewDecorated()
	s.thumb.Width, s.thumb.Height = s.thumbSize, s.thumbSize
	s.thumb.MinWidth, s.thumb.MinHeight = s.thumbSize, s.thumbSize
	s.thumb.Radius = s.thumbSize / 2
	s.thumb.BorderWidth = 0
	s.thumb.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}

	spinSz := s.thumbSize * 0.55
	if spinSz < 8 {
		spinSz = 8
	}
	if s.Loading {
		s.spinner = primitive.NewCanvas(spinSz, spinSz, s.paintSpinner)
		spinHost := primitive.NewStack(primitive.Positioned(core.AlignCenter, s.spinner))
		spinHost.Fit = true
		s.thumb.ClearChildren()
		s.thumb.AddChild(spinHost)
		s.thumb.StretchChild = true
		s.thumb.SetCenterContent(false)
	} else {
		s.spinner = nil
		s.thumb.ClearChildren()
		s.thumb.StretchChild = false
		s.thumb.SetCenterContent(false)
	}

	s.stack = primitive.NewStack()
	s.stack.Fit = true

	// Dual labels on a full-track canvas (under thumb in z-order).
	if s.CheckedChildren != "" || s.UnCheckedChildren != "" {
		s.label = primitive.NewCanvas(s.trackW, s.trackH, s.paintInnerLabels)
		s.labelHost = primitive.PositionedAt(0, 0, s.label)
		s.stack.AddChild(s.labelHost)
	} else {
		s.label = nil
		s.labelHost = nil
	}

	s.thumbHost = primitive.PositionedAt(s.pad, s.pad, s.thumb)
	s.stack.AddChild(s.thumbHost)

	if s.track == nil {
		s.track = primitive.NewDecorated(s.stack)
	} else {
		s.track.ClearChildren()
		s.track.AddChild(s.stack)
	}
	s.track.Width, s.track.Height = s.trackW, s.trackH
	s.track.MinWidth, s.track.MinHeight = s.trackW, s.trackH
	s.track.Radius = s.trackH / 2
	s.track.BorderWidth = 0
	s.track.Padding = primitive.EdgeInsets{}
	s.track.StretchChild = true

	if s.Root == nil {
		s.Root = primitive.NewPressable(s.track)
	} else {
		s.Root.ClearChildren()
		s.Root.AddChild(s.track)
	}
	s.Root.Focusable = true
	s.Root.ShowFocusRing = true
	s.Root.FocusRingRadius = s.trackH / 2
	s.Root.FocusRingOutset = 1.5
	s.Root.OnStateChange = s.SyncState
	s.Root.Click = s.fireToggle
	s.Root.SetDisabled(s.Disabled || s.Loading)
	s.Root.SetThemeHook(func(*core.Theme) { s.rebuild() })
	s.applyA11yName()

	s.lastHovered, s.lastPressed, s.lastFocused = false, false, false
	s.lastThumbW = 0 // force thumb layout once after rebuild
	s.labelChecked.ready = false
	s.labelUnchecked.ready = false
	s.ensureLabelCaches()
	s.applyChrome()
	s.applyThumbShape()
	s.Root.MarkNeedsLayout()
	s.Root.MarkNeedsPaint()
}

func (s *Switch) currentChildrenText() string {
	if s.Checked {
		return s.CheckedChildren
	}
	return s.UnCheckedChildren
}

func (s *Switch) animateThumb() {
	to := 0.0
	if s.Checked {
		to = 1
	}
	if s.boundTree == nil && s.Root != nil {
		s.boundTree = s.Root.Tree()
	}
	if s.boundTree == nil {
		s.thumbPos.Snap(to)
		s.applyThumbPad(to)
		return
	}
	s.thumbPos.Duration = DefaultSwitchThumbDuration
	s.thumbPos.SetTarget(to)
	if s.thumbPos.Active() || s.Loading {
		s.boundTree.AddTicker(s)
	}
}

// thumbWidth returns handle visual width.
// Travel endpoints always use rest thumbSize so width changes never shift leftOn.
// Press: full stretch; during slide recovery: stretch eases back to 1.0 with
// progress (same visual as original — stretch recovers while the thumb moves).
func (s *Switch) thumbWidth() float64 {
	base := s.thumbSize
	if base <= 0 {
		base = 18
	}
	const stretch = 1.2
	// Finger down: full press elongation.
	if s.Root != nil && s.Root.State.Pressed && !s.Disabled && !s.Loading {
		return base * stretch
	}
	// Free slide: recover stretch → 1.0 with animation progress.
	if s.thumbPos.Active() {
		den := s.thumbPos.Target - s.thumbPos.From
		prog := 1.0
		if den != 0 {
			prog = (s.thumbPos.Current - s.thumbPos.From) / den
			if prog < 0 {
				prog = -prog
			}
			if prog > 1 {
				prog = 1
			}
		}
		// progress 0 → 1.2×; progress 1 → 1.0×
		return base * (stretch + (1-stretch)*prog)
	}
	return base
}

func (s *Switch) applyThumbShape() {
	if s.thumb == nil {
		return
	}
	s.applyThumbPad(s.thumbPos.Current)
}

// applyThumbPad is the hot path during thumb animation: paint-only.
// No tree Layout on the label; thumb.Layout only when size actually changes.
func (s *Switch) applyThumbPad(t float64) {
	rest := s.thumbSize
	if rest <= 0 {
		rest = 18
	}
	leftOff := s.pad
	leftOn := s.trackW - s.pad - rest
	if leftOn < s.pad {
		leftOn = s.pad
	}
	// Rest travel point (stable curve) + center stretch/recovery width on it.
	restLeft := leftOff + (leftOn-leftOff)*t
	tw := s.thumbWidth()
	left := restLeft - (tw-rest)/2
	if left < s.pad*0.5 {
		left = s.pad * 0.5
	}
	maxLeft := s.trackW - s.pad*0.5 - tw
	if maxLeft < left && maxLeft > s.pad*0.5 {
		left = maxLeft
	}

	if s.thumb != nil {
		if tw != s.lastThumbW {
			s.thumb.Width = tw
			s.thumb.MinWidth = tw
			s.thumb.Height = s.thumbSize
			s.thumb.MinHeight = s.thumbSize
			s.thumb.Radius = s.thumbSize / 2
			_ = s.thumb.Layout(core.Tight(tw, s.thumbSize))
			s.lastThumbW = tw
		}
		s.thumb.MarkNeedsPaint()
	}
	if host, ok := s.thumbHost.(stackOffsetter); ok {
		host.SetStackOffset(left, s.pad)
	} else if s.thumbHost != nil {
		s.thumbHost.Base().SetOffset(core.Point{X: left, Y: s.pad})
		s.thumbHost.Base().MarkNeedsPaint()
	}
	// Labels read thumbPos in paint; dirty canvas only (size is fixed at rebuild).
	if s.label != nil {
		s.label.MarkNeedsPaint()
	}
}

// ensureLabelCaches fills Checked/UnChecked paint metrics once per content/face.
func (s *Switch) ensureLabelCaches() {
	face := switchFaceAtSize(s.Face, s.innerFontSize())
	s.labelFace = face
	if face == nil {
		s.labelChecked = labelPaintCache{}
		s.labelUnchecked = labelPaintCache{}
		return
	}
	cacheOne := func(txt string, dst *labelPaintCache) {
		if dst.ready && dst.text == txt {
			return
		}
		*dst = labelPaintCache{text: txt}
		if txt == "" {
			dst.ready = true
			return
		}
		dst.advance = face.Advance(txt)
		var inkMinY, inkMaxY float64
		var hasInk bool
		for g := range face.Glyphs(txt) {
			b := g.Bounds
			if b.MaxX <= b.MinX && b.MaxY <= b.MinY {
				continue
			}
			if !hasInk {
				inkMinY, inkMaxY = b.MinY, b.MaxY
				hasInk = true
				continue
			}
			if b.MinY < inkMinY {
				inkMinY = b.MinY
			}
			if b.MaxY > inkMaxY {
				inkMaxY = b.MaxY
			}
		}
		dst.hasInk = hasInk
		if hasInk {
			dst.inkMidY = (inkMinY + inkMaxY) / 2
		}
		dst.ready = true
	}
	cacheOne(s.CheckedChildren, &s.labelChecked)
	cacheOne(s.UnCheckedChildren, &s.labelUnchecked)
}

// paintInnerLabels draws both labels in free bands beside the *live* handle.
// Uses cached face metrics (no Glyphs/Advance per frame).
func (s *Switch) paintInnerLabels(pc *core.PaintContext, sz core.Size) {
	if pc == nil || pc.DC == nil || s == nil {
		return
	}
	if !s.labelChecked.ready || !s.labelUnchecked.ready || s.labelFace == nil {
		s.ensureLabelCaches()
	}
	face := s.labelFace
	if face == nil {
		return
	}
	pc.DC.SetFont(face)
	col := s.labelColor
	if col.A <= 0 {
		col = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)

	trackW := sz.Width
	trackH := sz.Height
	if trackW < s.trackW {
		trackW = s.trackW
	}
	if trackH < s.trackH {
		trackH = s.trackH
	}
	if trackW <= 0 || trackH <= 0 {
		return
	}

	pc.PushClipLocal(0, 0, trackW, trackH)
	defer pc.Pop()

	// Same geometry as applyThumbPad: rest travel + centered stretch/recovery width.
	rest := s.thumbSize
	if rest <= 0 {
		rest = trackH - 2*s.pad
	}
	tw := s.thumbWidth()
	if tw <= 0 {
		tw = rest
	}
	leftOff := s.pad
	leftOn := trackW - s.pad - rest
	if leftOn < s.pad {
		leftOn = s.pad
	}
	pos := s.thumbPos.Current
	if pos < 0 {
		pos = 0
	}
	if pos > 1 {
		pos = 1
	}
	restLeft := leftOff + (leftOn-leftOff)*pos
	thumbL := restLeft - (tw-rest)/2
	if thumbL < s.pad*0.5 {
		thumbL = s.pad * 0.5
	}
	maxLeft := trackW - s.pad*0.5 - tw
	if maxLeft < thumbL && maxLeft > s.pad*0.5 {
		thumbL = maxLeft
	}
	thumbR := thumbL + tw

	leftL, leftR := s.pad, thumbL-s.pad
	rightL, rightR := thumbR+s.pad, trackW-s.pad
	midY := pc.Origin.Y + trackH/2

	drawCached := func(c labelPaintCache, bandL, bandR float64) {
		if c.text == "" || !c.ready {
			return
		}
		bandW := bandR - bandL
		if bandW < 4 {
			return
		}
		pc.PushClipLocal(bandL, 0, bandW, trackH)
		defer pc.Pop()
		cx := pc.Origin.X + bandL + bandW/2
		if c.hasInk {
			baseY := midY - c.inkMidY
			penX := cx - c.advance/2
			pc.DC.DrawString(c.text, penX, baseY)
			return
		}
		pc.DC.DrawStringAnchored(c.text, cx, midY, 0.5, 0.5)
	}

	drawCached(s.labelUnchecked, rightL, rightR)
	drawCached(s.labelChecked, leftL, leftR)
}

func (s *Switch) paintSpinner(pc *core.PaintContext, sz core.Size) {
	if pc == nil || !s.Loading {
		return
	}
	col := render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	if s.Disabled {
		col = s.theme().Color(core.TokenColorDisabledText)
	}
	track := render.RGBA{R: col.R, G: col.G, B: col.B, A: col.A * 0.35}
	if track.A < 0.1 {
		track.A = 0.15
	}
	stroke := 1.5
	if sz.Width < 10 {
		stroke = 1.2
	}
	cx, cy := sz.Width/2, sz.Height/2
	r := sz.Width/2 - stroke
	if r < 1 {
		r = 1
	}
	pc.StrokeLocalCircle(cx, cy, r, stroke, track)
	start := -math.Pi/2 + s.spinPhase*2*math.Pi
	end := start + 2*math.Pi*0.7
	steps := 32
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		a := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(a), cy+r*math.Sin(a))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

func (s *Switch) applyChrome() {
	if s.track == nil {
		return
	}
	th := s.theme()
	hovered := s.Root != nil && s.Root.State.Hovered && !s.Disabled && !s.Loading
	if s.Checked {
		bg := th.Color(core.TokenColorPrimary)
		if hovered {
			bg = th.Color(core.TokenColorPrimaryHover)
		}
		if s.Style.hasBGActive() {
			bg = s.Style.BackgroundActive
		}
		s.track.Background = bg
	} else {
		bg := render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		if hovered {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.35}
		}
		if s.Style.hasBG() {
			bg = s.Style.Background
			if hovered && s.Style.hasBGHover() {
				bg = s.Style.BackgroundHover
			}
		}
		s.track.Background = bg
	}
	if s.Disabled {
		if s.Checked {
			c := th.Color(core.TokenColorPrimary)
			c.A *= 0.4
			if c.A < 0.15 {
				c.A = 0.25
			}
			s.track.Background = c
		} else {
			s.track.Background = th.Color(core.TokenColorDisabledBg)
		}
		s.track.BorderWidth = 0
	} else {
		s.track.BorderWidth = 0
	}

	s.labelColor = th.Color(core.TokenColorTextInverse)
	if s.Disabled {
		s.labelColor = th.Color(core.TokenColorDisabledText)
	}
	if s.label != nil {
		s.label.MarkNeedsPaint()
	}

	s.applyThumbShape()
	if s.thumb != nil {
		s.thumb.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		s.thumb.MarkNeedsPaint()
	}
}
