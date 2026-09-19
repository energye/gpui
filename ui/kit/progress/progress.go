// Package progress implements the progress control (docs/antd/progress.md §6).
//
// Ant Design v6.5.1 P0+P1 scope: line / circle / dashboard, percent 0..100,
// status auto (100→success), showInfo/format, active sweep on line,
// steps, strokeLinecap, gradient, percentPosition, success split.
// Deferred (React/browser-only): semantic classNames/styles depth,
// pixel keyframes, ConfigProvider defaults, debug hash (see spec §6.8).
package progress

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// Geometry fallbacks from spec §6.2.1 (Token-first, numbers are fallback).
const (
	LineHeightMedium  = 8.0
	LineHeightSmall   = 6.0
	CircleEdgeMedium  = 120.0
	CircleEdgeSmall   = 60.0
	DefaultStrokePct  = 6.0
	DefaultGapDegree  = 75.0
	LineFallbackWidth = 160.0
	LineInfoFontSize  = 14.0
	sweepPeriodSec    = 1.2
)

// ProgressType selects line / circle / dashboard (§6.10).
type ProgressType string

const (
	TypeLine      ProgressType = "line"
	TypeCircle    ProgressType = "circle"
	TypeDashboard ProgressType = "dashboard"
)

// ProgressSize selects small / medium preset (§6.10).
type ProgressSize string

const (
	SizeSmall  ProgressSize = "small"
	SizeMedium ProgressSize = "medium"
)

// ProgressStatus selects explicit status; "" means auto (§6.4).
type ProgressStatus string

const (
	StatusAuto      ProgressStatus = ""
	StatusNormal    ProgressStatus = "normal"
	StatusActive    ProgressStatus = "active"
	StatusSuccess   ProgressStatus = "success"
	StatusException ProgressStatus = "exception"
)

// ProgressGapPlacement selects dashboard gap side (§6.10).
type ProgressGapPlacement string

const (
	GapTop    ProgressGapPlacement = "top"
	GapBottom ProgressGapPlacement = "bottom"
	GapStart  ProgressGapPlacement = "start"
	GapEnd    ProgressGapPlacement = "end"
)

// StrokeLinecap selects line end caps (antd strokeLinecap).
type StrokeLinecap string

const (
	LinecapRound  StrokeLinecap = "round"
	LinecapButt   StrokeLinecap = "butt"
	LinecapSquare StrokeLinecap = "square"
)

// PercentAlign selects info horizontal anchor for line percentPosition.
type PercentAlign string

const (
	AlignStart  PercentAlign = "start"
	AlignCenter PercentAlign = "center"
	AlignEnd    PercentAlign = "end"
)

// PercentPosType selects info inside vs outside the line track.
type PercentPosType string

const (
	PosOuter PercentPosType = "outer"
	PosInner PercentPosType = "inner"
)

// Progress is the progress widget (spec §6.10, mapped to current pkgs).
//
// Host is a plain AbsoluteBox (never a RepaintBoundary per §6.11); the
// inner track and info nodes are RepaintBoundaries so color/percent dirt
// stays local. Put Node() in the tree, drive Tick via AttachTicker.
type Progress struct {
	// percent/successPercent are atomic bits: SetPercent etc. write (UI),
	// paint reads (raster).
	percent      atomic.Uint64 // math.Float64bits
	ptype        ProgressType
	size         ProgressSize
	sizePx       float64
	width        float64
	strokeWidth  float64
	gapDegree    float64
	gapPlacement ProgressGapPlacement
	status       ProgressStatus
	showInfo     bool
	format       func(percent, successPercent float64) string

	strokeColor render.RGBA
	railColor   render.RGBA

	steps          int
	stepGap        float64
	stepGapSet     bool
	linecap        StrokeLinecap
	gradFrom       render.RGBA
	gradTo         render.RGBA
	gradDir        string
	hasGradient    bool
	stepColors     []render.RGBA
	align          PercentAlign
	posType        PercentPosType
	successPercent atomic.Uint64 // math.Float64bits, same threading
	successColor   render.RGBA
	hasSuccessCol  bool
	rounding       func(float64) float64

	provider     *theme.Provider
	override     *theme.Tokens
	ariaLabel    string
	reduceMotion bool
	// phase is atomic bits: Tick writes (UI), paint reads (raster).
	phase atomic.Uint64

	// snap is the last frozen paint input (R2-6, button snapshot paradigm):
	// stored by refreshSnapshot (UI, markPaint + New) and loaded once per
	// paint on raster.
	snap atomic.Value // ProgressSnap

	host  *rendering.AbsoluteBox
	track *rendering.RenderBox
	info  *rendering.RenderText

	attached *scheduler.TickerRegistry
}

// markPaint is the single UI funnel for every paint-affecting setter:
// refresh the frozen snapshot, then dirty the track (the only node that
// ever repaints). Tick keeps its direct track mark (phase is atomic live).
func (p *Progress) markPaint() {
	if p == nil {
		return
	}
	p.refreshSnapshot()
	if p.track != nil {
		p.track.MarkNeedsPaint()
	}
}

// NewProgress creates a line/medium progress at percent (clamped 0..100).
func NewProgress(percent float64) *Progress {
	p := &Progress{ptype: TypeLine, size: SizeMedium, showInfo: true}
	p.host = rendering.NewAbsoluteBox(0, 0)
	p.track = rendering.NewRenderBox()
	p.track.SetRepaintBoundary(true)
	paint := p
	p.track.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paint.paintTrack(pc, size)
	}
	p.info = rendering.NewRenderText("")
	p.info.SetRepaintBoundary(true)
	p.host.AddChild(p.track)
	p.host.AddChild(p.info)
	p.percent.Store(math.Float64bits(clampPercent(percent)))
	p.syncInfo()
	p.refreshSnapshot()
	return p
}

func clampPercent(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// SetType selects line | circle | dashboard.
func (p *Progress) SetType(t ProgressType) {
	if p == nil {
		return
	}
	if t != TypeLine && t != TypeCircle && t != TypeDashboard {
		t = TypeLine
	}
	if p.ptype == t {
		return
	}
	p.ptype = t
	p.syncInfo()
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// Type returns the current type.
func (p *Progress) Type() ProgressType {
	if p == nil {
		return TypeLine
	}
	return p.ptype
}

// SetSize selects small | medium preset.
func (p *Progress) SetSize(s ProgressSize) {
	if p == nil {
		return
	}
	if s != SizeSmall && s != SizeMedium {
		s = SizeMedium
	}
	if p.size == s {
		return
	}
	p.size = s
	p.syncInfo()
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// Size returns the preset.
func (p *Progress) Size() ProgressSize {
	if p == nil {
		return SizeMedium
	}
	return p.size
}

// SetSizePx sets custom edge (line height or circle edge); <=0 uses preset.
func (p *Progress) SetSizePx(v float64) {
	if p == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	if p.sizePx == v {
		return
	}
	p.sizePx = v
	p.syncInfo()
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// SizePx returns the custom size.
func (p *Progress) SizePx() float64 {
	if p == nil {
		return 0
	}
	return p.sizePx
}

// SetWidth sets line track width; 0 means parent constraint or 160 fallback.
func (p *Progress) SetWidth(v float64) {
	if p == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	if p.width == v {
		return
	}
	p.width = v
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// Width returns the explicit track width.
func (p *Progress) Width() float64 {
	if p == nil {
		return 0
	}
	return p.width
}

// SetStrokeWidth sets circle stroke width in % of edge; 0 means default 6.
func (p *Progress) SetStrokeWidth(v float64) {
	if p == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	if p.strokeWidth == v {
		return
	}
	p.strokeWidth = v
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// StrokeWidth returns the configured percent.
func (p *Progress) StrokeWidth() float64 {
	if p == nil {
		return 0
	}
	return p.strokeWidth
}

// SetGapDegree sets dashboard gap angle; unset dashboard defaults to 75.
func (p *Progress) SetGapDegree(v float64) {
	if p == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	if v > 295 {
		v = 295
	}
	if p.gapDegree == v {
		return
	}
	p.gapDegree = v
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// GapDegree returns the configured gap.
func (p *Progress) GapDegree() float64 {
	if p == nil {
		return 0
	}
	return p.gapDegree
}

// SetGapPlacement selects dashboard gap side.
func (p *Progress) SetGapPlacement(g ProgressGapPlacement) {
	if p == nil {
		return
	}
	if p.gapPlacement == g {
		return
	}
	p.gapPlacement = g
	p.markPaint()
}

// GapPlacement returns the gap side.
func (p *Progress) GapPlacement() ProgressGapPlacement {
	if p == nil {
		return ""
	}
	return p.gapPlacement
}

// SetGapPosition is the deprecated alias of SetGapPlacement.
// Accepts top|bottom|left|right; left maps to start, right to end.
func (p *Progress) SetGapPosition(s string) {
	if p == nil {
		return
	}
	switch s {
	case "top":
		p.SetGapPlacement(GapTop)
	case "bottom":
		p.SetGapPlacement(GapBottom)
	case "left", "start":
		p.SetGapPlacement(GapStart)
	case "right", "end":
		p.SetGapPlacement(GapEnd)
	default:
		p.SetGapPlacement(GapBottom)
	}
}

// SetPercent sets 0..100 clamped; never rebuilds the tree, only paint+info.
func (p *Progress) SetPercent(v float64) {
	if p == nil {
		return
	}
	v = clampPercent(v)
	if math.Float64frombits(p.percent.Load()) == v {
		return
	}
	p.percent.Store(math.Float64bits(v))
	p.syncInfo()
	p.markPaint()
}

// Percent returns the clamped value.
func (p *Progress) Percent() float64 {
	if p == nil {
		return 0
	}
	return math.Float64frombits(p.percent.Load())
}

// FillRatio returns 0..1 fill proportion (PRG-S1/S2 probe).
func (p *Progress) FillRatio() float64 {
	if p == nil {
		return 0
	}
	return math.Float64frombits(p.percent.Load()) / 100
}

// SetStatus sets ""=auto | normal | exception | active | success.
func (p *Progress) SetStatus(s ProgressStatus) {
	if p == nil {
		return
	}
	if p.status == s {
		return
	}
	p.status = s
	p.syncInfo()
	p.markPaint()
}

// Status returns the explicit status.
func (p *Progress) Status() ProgressStatus {
	if p == nil {
		return StatusAuto
	}
	return p.status
}

// EffectiveStatus resolves auto: >=100→success else normal (§6.4).
func (p *Progress) EffectiveStatus() ProgressStatus {
	if p == nil {
		return StatusNormal
	}
	if p.status != StatusAuto {
		return p.status
	}
	if p.Percent() >= 100 {
		return StatusSuccess
	}
	return StatusNormal
}

// SetShowInfo toggles the info node (default true).
func (p *Progress) SetShowInfo(b bool) {
	if p == nil || p.showInfo == b {
		return
	}
	p.showInfo = b
	p.syncChildren()
	p.syncInfo()
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// ShowInfo reports the flag.
func (p *Progress) ShowInfo() bool {
	return p != nil && p.showInfo
}

// SetFormat sets custom info template; nil restores default.
func (p *Progress) SetFormat(f func(percent, successPercent float64) string) {
	if p == nil {
		return
	}
	p.format = f
	p.syncInfo()
	p.markPaint()
}

// SetStrokeColor overrides fill color (zero value walks Token).
func (p *Progress) SetStrokeColor(c render.RGBA) {
	if p == nil {
		return
	}
	p.strokeColor = c
	p.markPaint()
}

// SetRailColor overrides rail color (also covers trailColor alias).
func (p *Progress) SetRailColor(c render.RGBA) {
	if p == nil {
		return
	}
	p.railColor = c
	p.markPaint()
}

// SetTrailColor is the deprecated alias of SetRailColor.
func (p *Progress) SetTrailColor(c render.RGBA) { p.SetRailColor(c) }

// SetSteps sets line/circle step count; <=0 disables steps (default).
// Line draws equal blocks with StepGap; circle draws count arcs.
func (p *Progress) SetSteps(n int) {
	if p == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	if p.steps == n {
		return
	}
	p.steps = n
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// Steps returns the step count (0 = disabled).
func (p *Progress) Steps() int {
	if p == nil {
		return 0
	}
	return p.steps
}

// SetStepGap sets the gap between steps: line px, circle degrees.
// Unset defaults to 2 (spec §6.8 + style token marginXXS/2).
func (p *Progress) SetStepGap(v float64) {
	if p == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	if p.stepGapSet && p.stepGap == v {
		return
	}
	p.stepGap = v
	p.stepGapSet = true
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// StepGap returns the raw gap (0 when unset; see EffectiveStepGap).
func (p *Progress) StepGap() float64 {
	if p == nil {
		return 0
	}
	return p.stepGap
}

// EffectiveStepGap resolves the gap (default 2).
func (p *Progress) EffectiveStepGap() float64 {
	if p != nil && p.stepGapSet {
		return p.stepGap
	}
	return 2
}

// SetStrokeLinecap selects round (default) | butt | square.
func (p *Progress) SetStrokeLinecap(c StrokeLinecap) {
	if p == nil {
		return
	}
	if c != LinecapButt && c != LinecapSquare {
		c = LinecapRound
	}
	if p.linecap == c {
		return
	}
	p.linecap = c
	p.markPaint()
}

// StrokeLinecap returns the raw cap ("" means default round).
func (p *Progress) StrokeLinecap() StrokeLinecap {
	if p == nil {
		return ""
	}
	return p.linecap
}

// EffectiveStrokeLinecap resolves the cap (default round).
func (p *Progress) EffectiveStrokeLinecap() StrokeLinecap {
	if p != nil && (p.linecap == LinecapButt || p.linecap == LinecapSquare) {
		return p.linecap
	}
	return LinecapRound
}

// SetStrokeGradient enables a from→to linear gradient along progress.
// Direction is kept for compat (line: horizontal; circle: angular).
// Clear by passing two fully transparent colors.
func (p *Progress) SetStrokeGradient(from, to render.RGBA, dir string) {
	if p == nil {
		return
	}
	if from.A <= 0 && to.A <= 0 {
		if !p.hasGradient {
			return
		}
		p.hasGradient = false
		p.markPaint()
		return
	}
	p.gradFrom, p.gradTo, p.gradDir = from, to, dir
	p.hasGradient = true
	p.markPaint()
}

// HasStrokeGradient reports whether a gradient override is active.
func (p *Progress) HasStrokeGradient() bool { return p != nil && p.hasGradient }

// StrokeGradientEnds returns the gradient stops and direction.
func (p *Progress) StrokeGradientEnds() (render.RGBA, render.RGBA, string) {
	if p == nil {
		return render.RGBA{}, render.RGBA{}, ""
	}
	return p.gradFrom, p.gradTo, p.gradDir
}

// SetStepColors sets per-step fill colors for steps line (array form).
// Index i colors step i; missing entries fall back to stroke color.
func (p *Progress) SetStepColors(cs []render.RGBA) {
	if p == nil {
		return
	}
	cp := append([]render.RGBA(nil), cs...)
	p.stepColors = cp
	p.markPaint()
}

// StepColors returns a copy of the per-step colors.
func (p *Progress) StepColors() []render.RGBA {
	if p == nil || len(p.stepColors) == 0 {
		return nil
	}
	return append([]render.RGBA(nil), p.stepColors...)
}

// SetPercentPosition selects info placement for line (default end/outer).
func (p *Progress) SetPercentPosition(align PercentAlign, typ PercentPosType) {
	if p == nil {
		return
	}
	if align != AlignStart && align != AlignCenter {
		align = AlignEnd
	}
	if typ != PosInner {
		typ = PosOuter
	}
	if p.align == align && p.posType == typ {
		return
	}
	p.align, p.posType = align, typ
	p.syncInfo()
	p.host.MarkNeedsLayout()
	p.markPaint()
}

// PercentAlign returns the raw align ("" means default end).
func (p *Progress) PercentAlign() PercentAlign {
	if p == nil {
		return ""
	}
	return p.align
}

// PercentPositionType returns the raw position ("" means default outer).
func (p *Progress) PercentPositionType() PercentPosType {
	if p == nil {
		return ""
	}
	return p.posType
}

// EffectivePercentAlign resolves the align (default end).
func (p *Progress) EffectivePercentAlign() PercentAlign {
	if p != nil && (p.align == AlignStart || p.align == AlignCenter) {
		return p.align
	}
	return AlignEnd
}

// EffectivePercentPosition resolves the position (default outer).
func (p *Progress) EffectivePercentPosition() PercentPosType {
	if p != nil && p.posType == PosInner {
		return PosInner
	}
	return PosOuter
}

// IsInnerInfo reports whether info draws inside the line track.
func (p *Progress) IsInnerInfo() bool {
	return p != nil && p.ptype == TypeLine && p.steps <= 0 &&
		p.EffectivePercentPosition() == PosInner
}

// SetSuccessPercent sets the success split 0..100 (clamped).
func (p *Progress) SetSuccessPercent(v float64) {
	if p == nil {
		return
	}
	v = clampPercent(v)
	if math.Float64frombits(p.successPercent.Load()) == v {
		return
	}
	p.successPercent.Store(math.Float64bits(v))
	p.syncInfo()
	p.markPaint()
}

// SuccessPercent returns the success split.
func (p *Progress) SuccessPercent() float64 {
	if p == nil {
		return 0
	}
	return math.Float64frombits(p.successPercent.Load())
}

// EffectiveSuccessPercent resolves the clamped split.
func (p *Progress) EffectiveSuccessPercent() float64 {
	if p == nil {
		return 0
	}
	return clampPercent(math.Float64frombits(p.successPercent.Load()))
}

// SetSuccessStrokeColor overrides the success segment color.
func (p *Progress) SetSuccessStrokeColor(c render.RGBA) {
	if p == nil {
		return
	}
	p.successColor, p.hasSuccessCol = c, true
	p.markPaint()
}

// EffectiveSuccessColor resolves override else success token.
func (p *Progress) EffectiveSuccessColor() render.RGBA {
	if p != nil && p.hasSuccessCol && p.successColor.A > 0 {
		return p.successColor
	}
	return themeToRGBA(p.tokens().ColorSuccess)
}

// SetRounding sets custom step rounding (nil restores Math.round).
func (p *Progress) SetRounding(f func(float64) float64) {
	if p == nil {
		return
	}
	p.rounding = f
	p.markPaint()
}

// EffectiveRounding resolves the rounding func (default math.Round).
func (p *Progress) EffectiveRounding() func(float64) float64 {
	if p != nil && p.rounding != nil {
		return p.rounding
	}
	return math.Round
}

// ActiveSteps returns filled step count for steps mode.
func (p *Progress) ActiveSteps() int {
	if p == nil || p.steps <= 0 {
		return 0
	}
	n := int(p.EffectiveRounding()(float64(p.steps) * p.Percent() / 100))
	if n < 0 {
		n = 0
	}
	if n > p.steps {
		n = p.steps
	}
	return n
}

// SetProvider selects the theme source (nil selects process default).
func (p *Progress) SetProvider(pr *theme.Provider) {
	if p == nil {
		return
	}
	p.provider = pr
	p.syncInfo()
	p.markPaint()
}

// SetTheme pins exact tokens (nil clears to provider).
func (p *Progress) SetTheme(t *theme.Tokens) {
	if p == nil {
		return
	}
	p.override = t
	p.syncInfo()
	p.markPaint()
}

// SetTextFace sets the paint-only font face for the info text.
func (p *Progress) SetTextFace(f text.Face) {
	if p == nil || p.info == nil {
		return
	}
	p.info.SetFace(f)
}

// SetAriaLabel sets the accessible name ("" restores default).
func (p *Progress) SetAriaLabel(s string) {
	if p == nil {
		return
	}
	p.ariaLabel = s
}

// AriaLabel returns explicit label or "".
func (p *Progress) AriaLabel() string {
	if p == nil {
		return ""
	}
	return p.ariaLabel
}

// AccessibleName is the exposed name: explicit label, else format/default.
func (p *Progress) AccessibleName() string {
	if p == nil {
		return ""
	}
	if p.ariaLabel != "" {
		return p.ariaLabel
	}
	if !p.showInfo {
		return fmt.Sprintf("%g percent", p.Percent())
	}
	if p.format != nil {
		return p.format(p.Percent(), p.EffectiveSuccessPercent())
	}
	if st := p.EffectiveStatus(); st == StatusSuccess || st == StatusException {
		if p.IsInnerInfo() {
			return fmt.Sprintf("%g percent", p.Percent())
		}
		return p.InfoText()
	}
	return fmt.Sprintf("%g percent", p.Percent())
}

// Role is always progressbar (§6.6); Focusable is always false.
func (p *Progress) Role() string { return "progressbar" }

// Focusable is always false: display-only, never takes Tab.
func (p *Progress) Focusable() bool { return false }

// AriaValueNow mirrors percent; min 0 max 100.
func (p *Progress) AriaValueNow() float64 {
	if p == nil {
		return 0
	}
	return p.Percent()
}

// AriaValueMin returns 0.
func (p *Progress) AriaValueMin() float64 { return 0 }

// AriaValueMax returns 100.
func (p *Progress) AriaValueMax() float64 { return 100 }

// SetReduceMotion freezes the active sweep (accessibility).
func (p *Progress) SetReduceMotion(b bool) {
	if p == nil || p.reduceMotion == b {
		return
	}
	p.reduceMotion = b
	p.markPaint()
}

// ReduceMotion reports the flag.
func (p *Progress) ReduceMotion() bool { return p != nil && p.reduceMotion }

// LineHeight returns line track height (§6.2.1).
func (p *Progress) LineHeight() float64 {
	if p == nil {
		return LineHeightMedium
	}
	if p.sizePx > 0 {
		return p.sizePx
	}
	if p.size == SizeSmall {
		return LineHeightSmall
	}
	return LineHeightMedium
}

// CircleSize returns circle/dashboard edge (§6.2.1).
func (p *Progress) CircleSize() float64 {
	if p == nil {
		return CircleEdgeMedium
	}
	if p.sizePx > 0 {
		return p.sizePx
	}
	if p.size == SizeSmall {
		return CircleEdgeSmall
	}
	return CircleEdgeMedium
}

// InfoGap returns line/info spacing from MarginXS token (fallback 8).
func (p *Progress) InfoGap() float64 {
	tok := p.tokens()
	if tok.MarginXS > 0 {
		return tok.MarginXS
	}
	return 8
}

// InfoText returns the info string ("", percent, format, or status mark).
// Inner position forces percent text even for success/exception (antd Line).
func (p *Progress) InfoText() string {
	if p == nil || !p.showInfo {
		return ""
	}
	if p.format != nil {
		return p.format(p.Percent(), p.EffectiveSuccessPercent())
	}
	if p.IsInnerInfo() {
		return formatPercent(p.Percent())
	}
	switch p.EffectiveStatus() {
	case StatusSuccess:
		return "✓"
	case StatusException:
		return "✗"
	default:
		return formatPercent(p.Percent())
	}
}

// HasInfoNode reports whether the info child is attached.
func (p *Progress) HasInfoNode() bool {
	if p == nil || p.host == nil || p.info == nil {
		return false
	}
	for _, ch := range p.host.Children() {
		if ch == p.info {
			return true
		}
	}
	return false
}

// Node returns the tree node (layout/paint/hit through it).
func (p *Progress) Node() rendering.RenderObject {
	if p == nil {
		return nil
	}
	return p.host
}

// Layout sizes the host under constraints (Exact/Min/Max matrix).
func (p *Progress) Layout(c rendering.Constraints) rendering.Size {
	if p == nil || p.host == nil {
		return rendering.Size{}
	}
	p.syncChildren()
	p.syncInfo()
	if p.ptype == TypeCircle || p.ptype == TypeDashboard {
		return p.layoutCircle(c)
	}
	return p.layoutLine(c)
}

// AttachTicker registers the active sweep ticker.
func (p *Progress) AttachTicker(reg *scheduler.TickerRegistry) {
	if p == nil || reg == nil {
		return
	}
	if p.attached != nil && p.attached != reg {
		p.attached.Remove(p)
	}
	p.attached = reg
	reg.Add(p)
}

// Attach is an alias of AttachTicker.
func (p *Progress) Attach(reg *scheduler.TickerRegistry) { p.AttachTicker(reg) }

// Detach unregisters the ticker.
func (p *Progress) Detach() {
	if p == nil || p.attached == nil {
		return
	}
	p.attached.Remove(p)
	p.attached = nil
}

// Tick advances the active sweep; stays registered (scheduler.Ticker).
func (p *Progress) Tick(dt float64) bool {
	if p == nil {
		return false
	}
	if p.EffectiveStatus() != StatusActive || p.ptype != TypeLine || p.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	ph := math.Float64frombits(p.phase.Load()) + dt/sweepPeriodSec
	ph = math.Mod(ph, 1)
	if ph < 0 {
		ph++
	}
	p.phase.Store(math.Float64bits(ph))
	// Phase only: snapshot contents are unchanged, so mark the track
	// directly (paint reads the atomic phase live).
	p.track.MarkNeedsPaint()
	return true
}

// WantsFrame reports sweep demand (scheduler.FrameWanter).
func (p *Progress) WantsFrame() bool {
	return p != nil && p.EffectiveStatus() == StatusActive &&
		p.ptype == TypeLine && !p.reduceMotion
}

// Phase returns the sweep phase in [0,1).
func (p *Progress) Phase() float64 {
	if p == nil {
		return 0
	}
	return math.Float64frombits(p.phase.Load())
}

func (p *Progress) tokens() theme.Tokens {
	if p != nil && p.override != nil {
		return *p.override
	}
	if p != nil && p.provider != nil {
		return p.provider.Current()
	}
	return theme.Default.Current()
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// EffectiveFillColor resolves stroke override else status token (§6.2.2).
// Gradient does not change the probe color: solid start color is returned.
func (p *Progress) EffectiveFillColor() render.RGBA {
	if p != nil && p.hasGradient {
		return p.gradFrom
	}
	if p != nil && p.strokeColor.A > 0 {
		return p.strokeColor
	}
	tok := p.tokens()
	switch p.EffectiveStatus() {
	case StatusSuccess:
		return themeToRGBA(tok.ColorSuccess)
	case StatusException:
		return themeToRGBA(tok.ColorError)
	default:
		return themeToRGBA(tok.ColorPrimary)
	}
}

// EffectiveRailColor resolves rail override else fill-secondary token.
func (p *Progress) EffectiveRailColor() render.RGBA {
	if p != nil && p.railColor.A > 0 {
		return p.railColor
	}
	return themeToRGBA(p.tokens().ColorFillSecondary)
}

// EffectiveInfoColor follows status else text token (§6.2.2).
// Inner info draws white on the fill (antd indicator-inner).
func (p *Progress) EffectiveInfoColor() render.RGBA {
	if p != nil && p.IsInnerInfo() {
		return themeToRGBA(p.tokens().ColorWhite)
	}
	tok := p.tokens()
	switch p.EffectiveStatus() {
	case StatusSuccess:
		return themeToRGBA(tok.ColorSuccess)
	case StatusException:
		return themeToRGBA(tok.ColorError)
	default:
		return themeToRGBA(tok.ColorText)
	}
}

// EffectiveStrokePct resolves circle stroke percent (default 6).
func (p *Progress) EffectiveStrokePct() float64 {
	if p != nil && p.strokeWidth > 0 {
		return p.strokeWidth
	}
	return DefaultStrokePct
}

// EffectiveGapDegree resolves dashboard gap (default 75, clamp 0..295).
func (p *Progress) EffectiveGapDegree() float64 {
	if p == nil || (p.ptype != TypeDashboard && p.ptype != TypeCircle) {
		return 0
	}
	if p.ptype == TypeCircle {
		return 0
	}
	if p.gapDegree > 0 {
		return p.gapDegree
	}
	return DefaultGapDegree
}

// EffectiveGapPlacement resolves dashboard gap side (default bottom).
func (p *Progress) EffectiveGapPlacement() ProgressGapPlacement {
	if p != nil && p.gapPlacement != "" {
		return p.gapPlacement
	}
	return GapBottom
}

// EffectiveStrokePx resolves circle stroke logical px (min 3).
func (p *Progress) EffectiveStrokePx() float64 {
	edge := p.CircleSize()
	px := edge * p.EffectiveStrokePct() / 100
	if px < 3 {
		px = 3
	}
	return px
}

// InfoFontSize resolves info text size (§6.2.1).
func (p *Progress) InfoFontSize() float64 {
	if p == nil {
		return LineInfoFontSize
	}
	if p.ptype == TypeCircle || p.ptype == TypeDashboard {
		edge := p.CircleSize()
		return edge*0.15 + 6
	}
	tok := p.tokens()
	if tok.FontSize > 0 {
		return tok.FontSize
	}
	return LineInfoFontSize
}

func formatPercent(v float64) string {
	v = clampPercent(v)
	if v == math.Trunc(v) {
		return fmt.Sprintf("%d%%", int(v))
	}
	return fmt.Sprintf("%g%%", v)
}

func (p *Progress) syncChildren() {
	if p == nil || p.host == nil || p.track == nil || p.info == nil {
		return
	}
	hasTrack, hasInfo := false, false
	for _, ch := range p.host.Children() {
		if ch == rendering.RenderObject(p.track) {
			hasTrack = true
		}
		if ch == rendering.RenderObject(p.info) {
			hasInfo = true
		}
	}
	if !hasTrack {
		p.host.AddChild(p.track)
	}
	if p.showInfo && !hasInfo {
		p.host.AddChild(p.info)
	}
	if !p.showInfo && hasInfo {
		p.host.RemoveChild(p.info)
	}
}

func (p *Progress) syncInfo() {
	if p == nil || p.info == nil {
		return
	}
	p.syncChildren()
	text := p.InfoText()
	if p.info.Text != text {
		p.info.SetText(text)
	}
	ic := p.EffectiveInfoColor()
	p.info.SetColor(ic.R, ic.G, ic.B, ic.A)
	p.info.SetFontSize(p.InfoFontSize())
}

func (p *Progress) layoutLine(c rendering.Constraints) rendering.Size {
	if p.steps > 0 {
		return p.layoutStepsLine(c)
	}
	if p.IsInnerInfo() {
		return p.layoutInnerLine(c)
	}
	align := p.EffectivePercentAlign()
	if p.EffectivePercentPosition() == PosOuter && align == AlignCenter {
		return p.layoutOuterCenterLine(c)
	}
	if p.EffectivePercentPosition() == PosOuter && align == AlignStart {
		return p.layoutOuterStartLine(c)
	}
	return p.layoutOuterEndLine(c)
}

func (p *Progress) trackWidthFor(c rendering.Constraints, infoW, gap float64) float64 {
	trackW := p.width
	if trackW <= 0 {
		trackW = LineFallbackWidth
		if c.MaxWidth < rendering.Unbounded/2 {
			avail := c.MaxWidth
			if p.showInfo && p.EffectivePercentPosition() == PosOuter &&
				p.EffectivePercentAlign() != AlignCenter {
				avail -= infoW + gap
			}
			if avail < 0 {
				avail = 0
			}
			if avail < trackW {
				trackW = avail
			}
		}
	}
	return trackW
}

func (p *Progress) layoutOuterEndLine(c rendering.Constraints) rendering.Size {
	gap := p.InfoGap()
	var infoSize rendering.Size
	if p.showInfo {
		infoSize = p.info.Layout(rendering.Loose(c.MaxWidth, c.MaxHeight))
	}
	trackW := p.trackWidthFor(c, infoSize.Width, gap)
	trackH := p.LineHeight()
	prefW := trackW
	prefH := trackH
	if p.showInfo {
		prefW += gap + infoSize.Width
		if infoSize.Height > prefH {
			prefH = infoSize.Height
		}
	}
	p.host.FixedWidth, p.host.FixedHeight = prefW, prefH
	p.track.FixedWidth, p.track.FixedHeight = trackW, trackH
	trackY := 0.0
	infoY := 0.0
	if prefH > trackH {
		trackY = (prefH - trackH) / 2
	}
	if p.showInfo && prefH > infoSize.Height {
		infoY = (prefH - infoSize.Height) / 2
	}
	p.track.SetOffset(rendering.Point{X: 0, Y: trackY})
	if p.showInfo {
		p.info.SetOffset(rendering.Point{X: trackW + gap, Y: infoY})
	}
	return p.host.Layout(c)
}

func (p *Progress) layoutOuterStartLine(c rendering.Constraints) rendering.Size {
	gap := p.InfoGap()
	var infoSize rendering.Size
	if p.showInfo {
		infoSize = p.info.Layout(rendering.Loose(c.MaxWidth, c.MaxHeight))
	}
	trackW := p.trackWidthFor(c, infoSize.Width, gap)
	trackH := p.LineHeight()
	prefW := trackW
	prefH := trackH
	if p.showInfo {
		prefW += gap + infoSize.Width
		if infoSize.Height > prefH {
			prefH = infoSize.Height
		}
	}
	p.host.FixedWidth, p.host.FixedHeight = prefW, prefH
	p.track.FixedWidth, p.track.FixedHeight = trackW, trackH
	trackY, infoY := 0.0, 0.0
	if prefH > trackH {
		trackY = (prefH - trackH) / 2
	}
	if p.showInfo && prefH > infoSize.Height {
		infoY = (prefH - infoSize.Height) / 2
	}
	if p.showInfo {
		p.info.SetOffset(rendering.Point{X: 0, Y: infoY})
		p.track.SetOffset(rendering.Point{X: infoSize.Width + gap, Y: trackY})
	} else {
		p.track.SetOffset(rendering.Point{X: 0, Y: trackY})
	}
	return p.host.Layout(c)
}

func (p *Progress) layoutOuterCenterLine(c rendering.Constraints) rendering.Size {
	var infoSize rendering.Size
	if p.showInfo {
		infoSize = p.info.Layout(rendering.Loose(c.MaxWidth, c.MaxHeight))
	}
	trackW := p.trackWidthFor(c, infoSize.Width, 0)
	trackH := p.LineHeight()
	tok := p.tokens()
	colGap := tok.MarginXXS
	if colGap <= 0 {
		colGap = 4
	}
	prefW := trackW
	prefH := trackH
	if p.showInfo {
		if infoSize.Width > prefW {
			prefW = infoSize.Width
		}
		prefH += colGap + infoSize.Height
	}
	p.host.FixedWidth, p.host.FixedHeight = prefW, prefH
	p.track.FixedWidth, p.track.FixedHeight = trackW, trackH
	trackX := (prefW - trackW) / 2
	if trackX < 0 {
		trackX = 0
	}
	p.track.SetOffset(rendering.Point{X: trackX, Y: 0})
	if p.showInfo {
		ix := (prefW - infoSize.Width) / 2
		if ix < 0 {
			ix = 0
		}
		p.info.SetOffset(rendering.Point{X: ix, Y: trackH + colGap})
	}
	return p.host.Layout(c)
}

func (p *Progress) layoutInnerLine(c rendering.Constraints) rendering.Size {
	// Inner info lives inside the track; host equals track box.
	trackW := p.width
	if trackW <= 0 {
		trackW = LineFallbackWidth
		if c.MaxWidth < rendering.Unbounded/2 && c.MaxWidth < trackW {
			trackW = c.MaxWidth
			if trackW < 0 {
				trackW = 0
			}
		}
	}
	trackH := p.LineHeight()
	// Inner text needs vertical room: grow to info height if taller.
	var infoSize rendering.Size
	if p.showInfo {
		infoSize = p.info.Layout(rendering.Loose(trackW, c.MaxHeight))
		if infoSize.Height > trackH {
			trackH = infoSize.Height
		}
	}
	p.host.FixedWidth, p.host.FixedHeight = trackW, trackH
	p.track.FixedWidth, p.track.FixedHeight = trackW, trackH
	p.track.SetOffset(rendering.Point{X: 0, Y: 0})
	if p.showInfo {
		tok := p.tokens()
		pad := tok.PaddingXXS
		if pad <= 0 {
			pad = 4
		}
		var ix float64
		switch p.EffectivePercentAlign() {
		case AlignStart:
			ix = pad
		case AlignCenter:
			ix = (trackW - infoSize.Width) / 2
		default:
			ix = trackW - infoSize.Width - pad
		}
		if ix < 0 {
			ix = 0
		}
		iy := (trackH - infoSize.Height) / 2
		if iy < 0 {
			iy = 0
		}
		p.info.SetOffset(rendering.Point{X: ix, Y: iy})
	}
	return p.host.Layout(c)
}

func (p *Progress) stepBlockWidth() float64 {
	if p.size == SizeSmall {
		return 2
	}
	return 14
}

func (p *Progress) layoutStepsLine(c rendering.Constraints) rendering.Size {
	gap := p.EffectiveStepGap()
	n := p.steps
	blockW := p.stepBlockWidth()
	trackH := p.LineHeight()
	totalStepsW := float64(n)*blockW + float64(n-1)*gap
	if p.width > 0 {
		totalStepsW = p.width
		blockW = (totalStepsW - float64(n-1)*gap) / float64(n)
		if blockW < 0 {
			blockW = 0
		}
	} else if c.MaxWidth < rendering.Unbounded/2 {
		avail := c.MaxWidth
		var infoSize rendering.Size
		if p.showInfo {
			infoSize = p.info.Layout(rendering.Loose(c.MaxWidth, c.MaxHeight))
			avail -= infoSize.Width + p.InfoGap()
		}
		if avail < 0 {
			avail = 0
		}
		if totalStepsW > avail {
			totalStepsW = avail
			blockW = (totalStepsW - float64(n-1)*gap) / float64(n)
			if blockW < 0 {
				blockW = 0
			}
		}
	}
	var infoSize rendering.Size
	if p.showInfo {
		infoSize = p.info.Layout(rendering.Loose(c.MaxWidth, c.MaxHeight))
	}
	prefW := totalStepsW
	prefH := trackH
	if p.showInfo {
		prefW += p.InfoGap() + infoSize.Width
		if infoSize.Height > prefH {
			prefH = infoSize.Height
		}
	}
	p.host.FixedWidth, p.host.FixedHeight = prefW, prefH
	p.track.FixedWidth, p.track.FixedHeight = totalStepsW, trackH
	trackY := 0.0
	if prefH > trackH {
		trackY = (prefH - trackH) / 2
	}
	p.track.SetOffset(rendering.Point{X: 0, Y: trackY})
	if p.showInfo {
		infoY := 0.0
		if prefH > infoSize.Height {
			infoY = (prefH - infoSize.Height) / 2
		}
		p.info.SetOffset(rendering.Point{X: totalStepsW + p.InfoGap(), Y: infoY})
	}
	return p.host.Layout(c)
}

func (p *Progress) layoutCircle(c rendering.Constraints) rendering.Size {
	edge := p.CircleSize()
	var infoSize rendering.Size
	if p.showInfo {
		infoSize = p.info.Layout(rendering.Loose(edge, edge))
	}
	p.host.FixedWidth, p.host.FixedHeight = edge, edge
	p.track.FixedWidth, p.track.FixedHeight = edge, edge
	p.track.SetOffset(rendering.Point{X: 0, Y: 0})
	if p.showInfo {
		ix := (edge - infoSize.Width) / 2
		iy := (edge - infoSize.Height) / 2
		if ix < 0 {
			ix = 0
		}
		if iy < 0 {
			iy = 0
		}
		p.info.SetOffset(rendering.Point{X: ix, Y: iy})
	}
	return p.host.Layout(c)
}

func (p *Progress) paintTrack(pc *rendering.PaintContext, size rendering.Size) {
	if p == nil || pc == nil {
		return
	}
	// R2-6: one frozen paint-input load (stored on UI via markPaint/New),
	// read here on raster — never live reads of theme/config fields.
	// percent/successPercent/phase stay atomic live reads.
	S := p.loadSnapshot()
	if S.Ptype == TypeCircle || S.Ptype == TypeDashboard {
		p.paintRing(pc, size, S)
		return
	}
	p.paintLine(pc, size, S)
}

func (p *Progress) solidFillColor() render.RGBA {
	if p != nil && p.strokeColor.A > 0 {
		return p.strokeColor
	}
	tok := p.tokens()
	switch p.EffectiveStatus() {
	case StatusSuccess:
		return themeToRGBA(tok.ColorSuccess)
	case StatusException:
		return themeToRGBA(tok.ColorError)
	default:
		return themeToRGBA(tok.ColorPrimary)
	}
}

func lerpRGBA(a, b render.RGBA, t float64) render.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return render.RGBA{
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
	}
}

func lineRadiusFor(linecap StrokeLinecap, h float64) float64 {
	if linecap != LinecapRound {
		return 0
	}
	return h / 2
}

func renderCapFor(linecap StrokeLinecap) render.LineCap {
	if linecap == LinecapButt {
		return render.LineCapButt
	}
	if linecap == LinecapSquare {
		return render.LineCapSquare
	}
	return render.LineCapRound
}

func (p *Progress) paintLine(pc *rendering.PaintContext, size rendering.Size, S ProgressSnap) {
	w := size.Width
	h := size.Height
	if w <= 0 {
		w = p.track.FixedWidth
	}
	if h <= 0 {
		h = S.LineHeight
	}
	if w <= 0 || h <= 0 {
		return
	}
	if S.Steps > 0 {
		p.paintStepsLine(pc, w, h, S)
		p.paintActiveSweep(pc, w, h, S)
		return
	}
	rail := S.Rail
	radius := lineRadiusFor(S.Linecap, h)
	rendering.FillRoundRect(pc, 0, 0, w, h, radius, rail.R, rail.G, rail.B, rail.A)
	fillW := w * p.FillRatio()
	if fillW > w {
		fillW = w
	}
	if fillW > 0 {
		if S.HasGradient {
			p.paintGradientFill(pc, w, h, fillW, radius, S)
		} else {
			fill := S.SolidFill
			rendering.FillRoundRect(pc, 0, 0, fillW, h, radius, fill.R, fill.G, fill.B, fill.A)
		}
	}
	if s := p.EffectiveSuccessPercent(); s > 0 {
		sw := w * s / 100
		if sw > w {
			sw = w
		}
		if sw > 0 {
			sc := S.SuccessColor
			rendering.FillRoundRect(pc, 0, 0, sw, h, radius, sc.R, sc.G, sc.B, sc.A)
		}
	}
	p.paintActiveSweep(pc, w, h, S)
}

func (p *Progress) paintGradientFill(pc *rendering.PaintContext, w, h, fillW, radius float64, S ProgressSnap) {
	if fillW <= 0 {
		return
	}
	pc.PushClipRRect(0, 0, w, h, radius)
	const slices = 32
	for i := 0; i < slices; i++ {
		t0 := float64(i) / slices
		t1 := float64(i+1) / slices
		x0 := t0 * fillW
		x1 := t1*fillW + 1
		if x0 >= fillW {
			break
		}
		if x1 > fillW {
			x1 = fillW
		}
		c := lerpRGBA(S.GradFrom, S.GradTo, (t0+t1)/2)
		rendering.FillRect(pc, x0, 0, x1-x0, h, c.R, c.G, c.B, c.A)
	}
	pc.PopClip()
}

func (p *Progress) paintStepsLine(pc *rendering.PaintContext, w, h float64, S ProgressSnap) {
	n := S.Steps
	if n <= 0 {
		return
	}
	gap := S.StepGap
	blockW := (w - float64(n-1)*gap) / float64(n)
	if blockW < 0 {
		blockW = 0
	}
	rail := S.Rail
	radius := lineRadiusFor(S.Linecap, h)
	active := p.ActiveSteps()
	for i := 0; i < n; i++ {
		x := float64(i) * (blockW + gap)
		var c render.RGBA
		if i < active {
			if i < len(S.StepColors) && S.StepColors[i].A > 0 {
				c = S.StepColors[i]
			} else {
				c = S.SolidFill
			}
		} else {
			c = rail
		}
		if blockW <= 0 {
			continue
		}
		rendering.FillRoundRect(pc, x, 0, blockW, h, radius, c.R, c.G, c.B, c.A)
	}
}

func (p *Progress) paintActiveSweep(pc *rendering.PaintContext, w, h float64, S ProgressSnap) {
	if S.Status != StatusActive || S.ReduceMotion {
		return
	}
	radius := lineRadiusFor(S.Linecap, h)
	sweep := S.SweepLite
	sweepW := w * 0.25
	if sweepW < 8 {
		sweepW = 8
	}
	if sweepW > w {
		sweepW = w
	}
	sweepX := math.Float64frombits(p.phase.Load())*(w+sweepW) - sweepW
	pc.PushClipRRect(0, 0, w, h, radius)
	rendering.FillRoundRect(pc, sweepX, 0, sweepW, h, radius, sweep.R, sweep.G, sweep.B, 0.45)
	pc.PopClip()
}

func (p *Progress) paintRing(pc *rendering.PaintContext, size rendering.Size, S ProgressSnap) {
	edge := size.Width
	if size.Height < edge {
		edge = size.Height
	}
	if edge <= 0 {
		edge = S.CircleSize
	}
	if edge <= 0 {
		return
	}
	strokePx := edge * S.StrokePct / 100
	if strokePx < 3 {
		strokePx = 3
	}
	if strokePx >= edge/2 {
		strokePx = edge / 2
		if strokePx <= 0 {
			return
		}
	}
	cx, cy := edge/2, edge/2
	radius := (edge - strokePx) / 2
	if radius <= 0 {
		return
	}
	rail := S.Rail
	fill := S.SolidFill
	if pc.DC != nil {
		if S.Steps > 0 {
			pc.DC.SetLineCap(render.LineCapButt)
		} else {
			pc.DC.SetLineCap(renderCapFor(S.Linecap))
		}
	}
	if S.Steps > 0 {
		p.paintRingSteps(pc, cx, cy, radius, strokePx, rail, S)
		return
	}
	if S.Ptype == TypeDashboard {
		gapRad := S.GapDegree * math.Pi / 180
		total := 2*math.Pi - gapRad
		center := gapCenterAngle(S.GapPlacement)
		start := center + gapRad/2
		rendering.StrokeArc(pc, cx, cy, radius, start, start+total, strokePx, rail.R, rail.G, rail.B, rail.A)
		if r := p.FillRatio(); r > 0 {
			if S.HasGradient {
				p.paintGradientArc(pc, cx, cy, radius, start, total*r, strokePx, S)
			} else {
				rendering.StrokeArc(pc, cx, cy, radius, start, start+total*r, strokePx, fill.R, fill.G, fill.B, fill.A)
			}
		}
		if s := p.EffectiveSuccessPercent(); s > 0 {
			sc := S.SuccessColor
			rendering.StrokeArc(pc, cx, cy, radius, start, start+total*s/100, strokePx, sc.R, sc.G, sc.B, sc.A)
		}
		return
	}
	rendering.StrokeCircle(pc, cx, cy, radius, strokePx, rail.R, rail.G, rail.B, rail.A)
	if r := p.FillRatio(); r > 0 {
		start := -math.Pi / 2
		if S.HasGradient {
			p.paintGradientArc(pc, cx, cy, radius, start, 2*math.Pi*r, strokePx, S)
		} else {
			rendering.StrokeArc(pc, cx, cy, radius, start, start+2*math.Pi*r, strokePx, fill.R, fill.G, fill.B, fill.A)
		}
	}
	if s := p.EffectiveSuccessPercent(); s > 0 {
		sc := S.SuccessColor
		start := -math.Pi / 2
		rendering.StrokeArc(pc, cx, cy, radius, start, start+2*math.Pi*s/100, strokePx, sc.R, sc.G, sc.B, sc.A)
	}
}

func (p *Progress) paintGradientArc(pc *rendering.PaintContext, cx, cy, radius, start, sweep, strokePx float64, S ProgressSnap) {
	if sweep <= 0 {
		return
	}
	const segs = 24
	for i := 0; i < segs; i++ {
		t0 := float64(i) / segs
		t1 := float64(i+1) / segs
		c := lerpRGBA(S.GradFrom, S.GradTo, (t0+t1)/2)
		rendering.StrokeArc(pc, cx, cy, radius, start+sweep*t0, start+sweep*t1+0.02, strokePx, c.R, c.G, c.B, c.A)
	}
}

func (p *Progress) paintRingSteps(pc *rendering.PaintContext, cx, cy, radius, strokePx float64, rail render.RGBA, S ProgressSnap) {
	n := S.Steps
	if n <= 0 {
		return
	}
	gapDeg := S.StepGap
	gapRad := gapDeg * math.Pi / 180
	var start0, total float64
	if S.Ptype == TypeDashboard {
		gapRad0 := S.GapDegree * math.Pi / 180
		total = 2*math.Pi - gapRad0
		center := gapCenterAngle(S.GapPlacement)
		start0 = center + gapRad0/2
	} else {
		total = 2 * math.Pi
		start0 = -math.Pi / 2
	}
	stepSweep := (total - gapRad*float64(n-1)) / float64(n)
	if stepSweep < 0 {
		stepSweep = 0
	}
	active := p.ActiveSteps()
	for i := 0; i < n; i++ {
		s := start0 + float64(i)*(stepSweep+gapRad)
		var c render.RGBA
		if i < active {
			if i < len(S.StepColors) && S.StepColors[i].A > 0 {
				c = S.StepColors[i]
			} else {
				c = S.SolidFill
			}
		} else {
			c = rail
		}
		rendering.StrokeArc(pc, cx, cy, radius, s, s+stepSweep, strokePx, c.R, c.G, c.B, c.A)
	}
}

func gapCenterAngle(g ProgressGapPlacement) float64 {
	switch g {
	case GapTop:
		return -math.Pi / 2
	case GapStart:
		return math.Pi
	case GapEnd:
		return 0
	default:
		return math.Pi / 2
	}
}
