package kit

import (
	"fmt"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Progress defaults — https://ant.design/components/progress
// Source: components/progress/{progress,Line,Circle,utils}.tsx + style/
const (
	DefaultProgressLineHeight      = 8.0
	DefaultProgressLineHeightSmall = 6.0
	DefaultProgressCircleSize      = 120.0
	DefaultProgressCircleSizeSmall = 60.0
	DefaultProgressStrokeWidth     = 6.0 // % of circle size
	DefaultProgressInfoGap         = 8.0 // marginXS
	DefaultProgressDashboardGap    = 75.0
	DefaultProgressLineFallbackW   = 160.0
	DefaultProgressInfoFont        = 14.0
)

// ProgressType is antd Progress type.
type ProgressType string

const (
	ProgressLine      ProgressType = "line"
	ProgressCircle    ProgressType = "circle"
	ProgressDashboard ProgressType = "dashboard"
)

// ProgressStatus is antd status (empty = auto).
type ProgressStatus string

const (
	ProgressStatusAuto      ProgressStatus = ""
	ProgressStatusNormal    ProgressStatus = "normal"
	ProgressStatusException ProgressStatus = "exception"
	ProgressStatusActive    ProgressStatus = "active"
	ProgressStatusSuccess   ProgressStatus = "success"
)

// ProgressSize is the preset size token (small | medium).
type ProgressSize string

const (
	ProgressSizeMedium ProgressSize = "medium"
	ProgressSizeSmall  ProgressSize = "small"
)

// ProgressGapPlacement is dashboard/circle gap side.
type ProgressGapPlacement string

const (
	ProgressGapTop    ProgressGapPlacement = "top"
	ProgressGapBottom ProgressGapPlacement = "bottom"
	ProgressGapStart  ProgressGapPlacement = "start"
	ProgressGapEnd    ProgressGapPlacement = "end"
)

// Progress is antd Progress (line / circle / dashboard).
// Root is NOT a RepaintBoundary (ScrollViewport holes). Active line uses Ticker.
type Progress struct {
	Root *progressHost

	// Percent 0..100.
	Percent float64
	// Type line|circle|dashboard. Empty → line.
	Type ProgressType
	// Status ""=auto (percent≥100 → success). active only animates line.
	Status ProgressStatus
	// Size preset; SizePx overrides geometry when > 0.
	Size   ProgressSize
	SizePx float64
	// Width is line track width; 0 → parent max or DefaultProgressLineFallbackW.
	Width float64
	// ShowInfo shows percent text / status icon. Default true.
	ShowInfo bool
	// Format overrides info text. nil → default "%"/icons.
	Format func(percent, successPercent float64) string
	// StrokeWidth is circle stroke as % of size; 0 → DefaultProgressStrokeWidth.
	StrokeWidth float64
	// GapDegree for dashboard; gapDegreeSet distinguishes 0 from unset.
	GapDegree    float64
	gapDegreeSet bool
	// GapPlacement for dashboard gap; empty → bottom on dashboard.
	GapPlacement ProgressGapPlacement

	strokeColor    render.RGBA
	strokeColorSet bool
	railColor      render.RGBA
	railColorSet   bool

	Theme     *core.Theme
	AriaLabel string

	// internals
	track    *progressTrack
	ring     *progressRing
	info     *primitive.Text
	body     core.Node
	activePh float64
	life     tickerLifecycle
}

type progressHost struct {
	core.NodeBase
	prog *Progress
}

func (h *progressHost) TypeID() string { return "kit.Progress" }

func (h *progressHost) OnMount() {
	if h == nil || h.prog == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.prog.life.attach(t, h.prog, h.prog.needsActiveTicker())
	}
}

func (h *progressHost) OnUnmount() {
	if h != nil && h.prog != nil {
		h.prog.life.unmount()
	}
}

func (h *progressHost) Layout(c core.Constraints) core.Size {
	if sz, ok := h.LayoutSkipIfClean(c); ok {
		return sz
	}
	kids := h.Children()
	if len(kids) == 0 {
		h.SetSize(core.Size{})
		h.RememberConstraints(c)
		h.ClearLayoutDirty()
		return core.Size{}
	}
	sz := kids[0].Layout(c)
	kids[0].Base().SetOffset(core.Point{})
	h.SetSize(sz)
	h.RememberConstraints(c)
	h.ClearLayoutDirty()
	return sz
}

func (h *progressHost) Paint(pc *core.PaintContext) {
	h.DefaultPaintChildren(pc)
	if pc != nil {
		h.ClearPaintDirty()
	}
}

func (h *progressHost) HitTest(p core.Point) core.Node {
	return h.DefaultHitTest(p)
}

// NewProgress creates Progress with percent (clamped) and antd defaults.
func NewProgress(percent float64) *Progress {
	p := &Progress{
		Percent:  clampProgress(percent),
		Type:     ProgressLine,
		Size:     ProgressSizeMedium,
		ShowInfo: true,
	}
	// Lazy rebuild on Node() so callers can set Width/ShowInfo/Type fields first.
	return p
}

// Node returns the mount root.
func (p *Progress) Node() core.Node {
	if p == nil {
		return nil
	}
	if p.Root == nil {
		p.rebuild()
	}
	return p.Root
}

// AttachTicker registers active-line shimmer (also automatic OnMount).
func (p *Progress) AttachTicker(t *core.Tree) {
	if p != nil {
		p.life.attach(t, p, p.needsActiveTicker())
	}
}

// Tick advances active shimmer. Returns false when idle/unmounted.
func (p *Progress) Tick(dt float64) bool {
	if p == nil || !p.needsActiveTicker() {
		return false
	}
	var nt *core.Tree
	if p.Root != nil {
		nt = p.Root.Tree()
	}
	if !p.life.stillMounted(nt) {
		return false
	}
	p.activePh += dt * 0.55
	if p.activePh > 1 {
		p.activePh -= 1
	}
	if p.track != nil {
		p.track.MarkNeedsPaint()
	} else if p.Root != nil {
		p.Root.MarkNeedsPaint()
	}
	return true
}

// SetType sets line|circle|dashboard.
func (p *Progress) SetType(t ProgressType) {
	if p == nil {
		return
	}
	if t == "" {
		t = ProgressLine
	}
	if p.Type == t {
		return
	}
	p.Type = t
	p.rebuild()
}

// SetSize sets small|medium preset (clears numeric override semantics only when SizePx==0).
func (p *Progress) SetSize(s ProgressSize) {
	if p == nil {
		return
	}
	if s == "" {
		s = ProgressSizeMedium
	}
	if p.Size == s {
		return
	}
	p.Size = s
	p.rebuild()
}

// SetSizePx sets custom geometry: line height or circle edge. 0 → preset Size.
func (p *Progress) SetSizePx(px float64) {
	if p == nil {
		return
	}
	if px < 0 {
		px = 0
	}
	if p.SizePx == px {
		return
	}
	p.SizePx = px
	p.rebuild()
}

// SetWidth sets line track width. 0 → parent constraint / fallback.
func (p *Progress) SetWidth(w float64) {
	if p == nil {
		return
	}
	if w < 0 {
		w = 0
	}
	if p.Width == w {
		return
	}
	p.Width = w
	if p.track != nil {
		p.track.MarkNeedsLayout()
		p.track.MarkNeedsPaint()
	}
	if p.Root != nil {
		p.Root.MarkNeedsLayout()
	} else {
		p.rebuild()
	}
}

// SetStrokeWidth sets circle stroke width as % of size. 0 → default 6.
func (p *Progress) SetStrokeWidth(pct float64) {
	if p == nil {
		return
	}
	if pct < 0 {
		pct = 0
	}
	if p.StrokeWidth == pct {
		return
	}
	p.StrokeWidth = pct
	if p.ring != nil {
		p.ring.MarkNeedsPaint()
	} else {
		p.rebuild()
	}
}

// SetGapDegree sets dashboard gap in degrees (0 is valid).
func (p *Progress) SetGapDegree(deg float64) {
	if p == nil {
		return
	}
	if deg < 0 {
		deg = 0
	}
	if deg > 295 {
		deg = 295
	}
	p.GapDegree = deg
	p.gapDegreeSet = true
	if p.ring != nil {
		p.ring.MarkNeedsPaint()
	} else {
		p.rebuild()
	}
}

// SetGapPlacement sets dashboard gap side.
func (p *Progress) SetGapPlacement(g ProgressGapPlacement) {
	if p == nil {
		return
	}
	if p.GapPlacement == g {
		return
	}
	p.GapPlacement = g
	if p.ring != nil {
		p.ring.MarkNeedsPaint()
	} else {
		p.rebuild()
	}
}

// SetPercent updates fill 0..100 without rebuilding the node tree.
func (p *Progress) SetPercent(v float64) {
	if p == nil {
		return
	}
	v = clampProgress(v)
	if p.Percent == v {
		return
	}
	p.Percent = v
	p.applyValue()
}

// SetStatus sets status ("" = auto).
func (p *Progress) SetStatus(s ProgressStatus) {
	if p == nil {
		return
	}
	if p.Status == s {
		return
	}
	p.Status = s
	p.applyValue()
	p.syncTicker()
}

// SetShowInfo toggles percent/icon text.
func (p *Progress) SetShowInfo(v bool) {
	if p == nil {
		return
	}
	if p.ShowInfo == v {
		return
	}
	p.ShowInfo = v
	p.rebuild()
}

// SetFormat sets info formatter; nil restores default.
func (p *Progress) SetFormat(fn func(percent, successPercent float64) string) {
	if p == nil {
		return
	}
	p.Format = fn
	p.applyValue()
}

// SetStrokeColor overrides fill color (zero+unset → Token).
func (p *Progress) SetStrokeColor(c render.RGBA) {
	if p == nil {
		return
	}
	p.strokeColor = c
	p.strokeColorSet = true
	p.applyValue()
}

// SetRailColor overrides trail/rail color.
func (p *Progress) SetRailColor(c render.RGBA) {
	if p == nil {
		return
	}
	p.railColor = c
	p.railColorSet = true
	p.applyValue()
}

// SetTheme sets local theme override.
func (p *Progress) SetTheme(th *core.Theme) {
	if p == nil {
		return
	}
	p.Theme = th
	p.rebuild()
}

// SetAriaLabel sets accessible name override.
func (p *Progress) SetAriaLabel(s string) {
	if p == nil {
		return
	}
	p.AriaLabel = s
	p.applyA11y()
}

// EffectiveStatus returns resolved status (auto success at 100%).
func (p *Progress) EffectiveStatus() ProgressStatus {
	if p == nil {
		return ProgressStatusNormal
	}
	st := p.Status
	switch st {
	case ProgressStatusNormal, ProgressStatusException, ProgressStatusActive, ProgressStatusSuccess:
		return st
	default:
		// invalid or empty → auto
		if p.Percent >= 100 {
			return ProgressStatusSuccess
		}
		return ProgressStatusNormal
	}
}

// LineHeight returns resolved line track height.
func (p *Progress) LineHeight() float64 {
	if p == nil {
		return DefaultProgressLineHeight
	}
	if p.SizePx > 0 && (p.Type == ProgressLine || p.Type == "") {
		return p.SizePx
	}
	th := p.theme()
	if p.Size == ProgressSizeSmall {
		return DefaultProgressLineHeightSmall
	}
	return th.SizeOr(core.TokenProgressHeight, DefaultProgressLineHeight)
}

// CircleSize returns resolved circle/dashboard edge length.
func (p *Progress) CircleSize() float64 {
	if p == nil {
		return DefaultProgressCircleSize
	}
	if p.SizePx > 0 {
		return p.SizePx
	}
	if p.Size == ProgressSizeSmall {
		return DefaultProgressCircleSizeSmall
	}
	return DefaultProgressCircleSize
}

// FillRatio returns percent/100 clamped.
func (p *Progress) FillRatio() float64 {
	if p == nil {
		return 0
	}
	return clampProgress(p.Percent) / 100
}

// InfoText returns the visible info string (empty when hidden).
func (p *Progress) InfoText() string {
	if p == nil || !p.infoEnabled() {
		return ""
	}
	return p.resolveInfoText()
}

func (p *Progress) theme() *core.Theme {
	var n core.Node
	if p.Root != nil {
		n = p.Root
	}
	return themeOf(p.Theme, n)
}

func (p *Progress) infoEnabled() bool {
	return p != nil && p.ShowInfo
}

func (p *Progress) needsActiveTicker() bool {
	return p != nil && p.EffectiveStatus() == ProgressStatusActive && p.resolvedType() == ProgressLine
}

func (p *Progress) resolvedType() ProgressType {
	if p == nil || p.Type == "" {
		return ProgressLine
	}
	return p.Type
}

func (p *Progress) syncTicker() {
	if p == nil {
		return
	}
	active := p.needsActiveTicker()
	if p.Root != nil {
		if t := p.Root.Tree(); t != nil {
			p.life.attach(t, p, active)
			return
		}
	}
	p.life.setActive(active)
}

func (p *Progress) strokeCol(th *core.Theme) render.RGBA {
	if p.strokeColorSet {
		return p.strokeColor
	}
	switch p.EffectiveStatus() {
	case ProgressStatusSuccess:
		return th.Color(core.TokenColorSuccess)
	case ProgressStatusException:
		return th.Color(core.TokenColorError)
	default:
		return th.Color(core.TokenColorPrimary)
	}
}

func (p *Progress) railCol(th *core.Theme) render.RGBA {
	if p.railColorSet {
		return p.railColor
	}
	c := th.Color(core.TokenColorFillSecondary)
	if c.A < 0.05 {
		c = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	return c
}

func (p *Progress) infoCol(th *core.Theme) render.RGBA {
	switch p.EffectiveStatus() {
	case ProgressStatusSuccess:
		return th.Color(core.TokenColorSuccess)
	case ProgressStatusException:
		return th.Color(core.TokenColorError)
	default:
		c := th.Color(core.TokenColorTextSecondary)
		if c.A < 0.1 {
			c = th.Color(core.TokenColorText)
		}
		return c
	}
}

func (p *Progress) resolveInfoText() string {
	pct := clampProgress(p.Percent)
	if p.Format != nil {
		return p.Format(pct, 0)
	}
	st := p.EffectiveStatus()
	// antd: success/exception without format → status icon
	if st == ProgressStatusException {
		if p.resolvedType() == ProgressLine {
			return "✕"
		}
		return "✕"
	}
	if st == ProgressStatusSuccess {
		if p.resolvedType() == ProgressLine {
			return "✓"
		}
		return "✓"
	}
	return fmt.Sprintf("%.0f%%", pct)
}

func (p *Progress) gapDegree() float64 {
	if p.resolvedType() != ProgressDashboard {
		if p.gapDegreeSet {
			return p.GapDegree
		}
		return 0
	}
	if p.gapDegreeSet {
		return p.GapDegree
	}
	return DefaultProgressDashboardGap
}

func (p *Progress) gapStartAngle() float64 {
	// Full circle starts at -π/2 (12 o'clock). Dashboard leaves a gap.
	// gapPlacement rotates the gap center.
	gap := p.gapDegree() * math.Pi / 180
	// usable arc = 2π - gap; start so gap sits at placement.
	place := p.GapPlacement
	if place == "" && p.resolvedType() == ProgressDashboard {
		place = ProgressGapBottom
	}
	// gap center angle (from +x, CCW, standard math):
	var gapCenter float64
	switch place {
	case ProgressGapTop:
		gapCenter = -math.Pi / 2
	case ProgressGapStart:
		gapCenter = math.Pi // left in LTR
	case ProgressGapEnd:
		gapCenter = 0
	default: // bottom
		gapCenter = math.Pi / 2
	}
	// progress starts at gapCenter + gap/2
	return gapCenter + gap/2
}

func (p *Progress) applyValue() {
	if p == nil {
		return
	}
	if p.Root == nil {
		p.rebuild()
		return
	}
	if p.info != nil {
		if p.infoEnabled() {
			p.info.Value = p.resolveInfoText()
			p.info.Color = p.infoCol(p.theme())
		} else {
			p.info.Value = ""
		}
		p.info.MarkNeedsPaint()
	}
	if p.track != nil {
		p.track.MarkNeedsPaint()
	}
	if p.ring != nil {
		p.ring.MarkNeedsPaint()
	}
	p.applyA11y()
	if p.Root != nil {
		p.Root.MarkNeedsPaint()
	}
	p.syncTicker()
}

func (p *Progress) applyA11y() {
	if p == nil || p.Root == nil {
		return
	}
	p.Root.Base().Role = "progressbar"
	if p.AriaLabel != "" {
		p.Root.Base().Label = p.AriaLabel
	} else if p.infoEnabled() {
		p.Root.Base().Label = p.resolveInfoText()
	} else {
		p.Root.Base().Label = fmt.Sprintf("%.0f percent", p.Percent)
	}
}

func (p *Progress) rebuild() {
	if p == nil {
		return
	}
	th := p.theme()
	typ := p.resolvedType()

	var body core.Node
	p.track = nil
	p.ring = nil
	p.info = nil

	if typ == ProgressCircle || typ == ProgressDashboard {
		body = p.buildCircle(th)
	} else {
		body = p.buildLine(th)
	}
	p.body = body

	h := &progressHost{prog: p}
	h.Init(h)
	h.Hit = core.HitDefer
	// Must NOT be RepaintBoundary (ScrollViewport hole regression).
	h.SetRepaintBoundary(false)
	if body != nil {
		h.AddChild(body)
	}
	p.Root = h
	p.applyA11y()
	p.syncTicker()
	if p.Root != nil {
		p.Root.SetThemeHook(func(th *core.Theme) {
			_ = th
			p.rebuild()
		})
	}
}

func (p *Progress) buildLine(th *core.Theme) core.Node {
	p.track = &progressTrack{prog: p}
	p.track.Init(p.track)
	p.track.Hit = core.HitDefer

	kids := []core.Node{p.track}
	if p.infoEnabled() {
		p.info = primitive.NewText(p.resolveInfoText())
		p.info.FontSize = th.SizeOr(core.TokenFontSize, DefaultProgressInfoFont)
		p.info.Color = p.infoCol(th)
		kids = append(kids, p.info)
	}
	row := primitive.Row(kids...)
	// antd body gap is marginXS token seed (8 in docs §6.2; theme may seed 4).
	row.Gap = DefaultProgressInfoGap
	if g := th.SizeOr(core.TokenMarginXS, 0); g >= DefaultProgressInfoGap {
		row.Gap = g
	}
	row.CrossAlign = core.CrossCenter
	row.ExpandMax = p.Width <= 0
	return row
}

func (p *Progress) buildCircle(th *core.Theme) core.Node {
	sz := p.CircleSize()
	p.ring = &progressRing{prog: p}
	p.ring.Init(p.ring)
	p.ring.Hit = core.HitDefer

	host := &progressCircleBox{prog: p, edge: sz}
	host.Init(host)
	host.Hit = core.HitDefer
	host.AddChild(p.ring)

	if p.infoEnabled() {
		p.info = primitive.NewText(p.resolveInfoText())
		// antd: fontSize = width * 0.15 + 6
		fs := sz*0.15 + 6
		if fs < 10 {
			fs = 10
		}
		if sz <= 20 {
			// micro: keep compact; long format still in tree for a11y/tests
			fs = math.Max(8, sz*0.55)
		}
		p.info.FontSize = fs
		p.info.Color = p.infoCol(th)
		host.AddChild(p.info)
	}
	return host
}

// progressCircleBox is a fixed edge×edge stack: ring + optional centered info.
// Info text must not expand the layout box (micro circle + long format).
type progressCircleBox struct {
	core.NodeBase
	prog *Progress
	edge float64
}

func (b *progressCircleBox) TypeID() string { return "kit.ProgressCircle" }

func (b *progressCircleBox) Layout(c core.Constraints) core.Size {
	if sz, ok := b.LayoutSkipIfClean(c); ok {
		return sz
	}
	edge := b.edge
	if b.prog != nil {
		edge = b.prog.CircleSize()
		b.edge = edge
	}
	if c.HasBoundedWidth() && c.MaxWidth < edge && c.MaxWidth > 0 {
		edge = c.MaxWidth
	}
	if c.HasBoundedHeight() && c.MaxHeight < edge && c.MaxHeight > 0 {
		edge = c.MaxHeight
	}
	if edge < 0 {
		edge = 0
	}
	out := core.Size{Width: edge, Height: edge}
	b.SetSize(out)
	// Children get loose constraints up to edge; ring sizes to edge, info hugs content then centered.
	for _, kid := range b.Children() {
		ksz := kid.Layout(core.Constraints{MaxWidth: edge, MaxHeight: edge})
		ox := (edge - ksz.Width) / 2
		oy := (edge - ksz.Height) / 2
		if ox < 0 {
			ox = 0
		}
		if oy < 0 {
			oy = 0
		}
		kid.Base().SetOffset(core.Point{X: ox, Y: oy})
	}
	b.RememberConstraints(c)
	b.ClearLayoutDirty()
	return out
}

func (b *progressCircleBox) Paint(pc *core.PaintContext) {
	b.DefaultPaintChildren(pc)
	if pc != nil {
		b.ClearPaintDirty()
	}
}

func (b *progressCircleBox) HitTest(p core.Point) core.Node {
	return b.DefaultHitTest(p)
}

// ── line track (layout + paint rail/fill/active) ─────────────────────

type progressTrack struct {
	core.NodeBase
	prog *Progress
}

func (t *progressTrack) TypeID() string { return "kit.ProgressTrack" }

func (t *progressTrack) Layout(c core.Constraints) core.Size {
	if sz, ok := t.LayoutSkipIfClean(c); ok {
		return sz
	}
	p := t.prog
	h := DefaultProgressLineHeight
	if p != nil {
		h = p.LineHeight()
	}
	w := 0.0
	if p != nil {
		w = p.Width
	}
	if w <= 0 {
		if c.HasBoundedWidth() && c.MaxWidth < core.Unbounded && c.MaxWidth > 0 {
			w = c.MaxWidth
		} else {
			w = DefaultProgressLineFallbackW
		}
	}
	if c.MinWidth > w {
		w = c.MinWidth
	}
	if c.HasBoundedWidth() && c.MaxWidth < w {
		w = c.MaxWidth
	}
	if w < 0 {
		w = 0
	}
	sz := core.Size{Width: w, Height: h}
	t.SetSize(sz)
	t.RememberConstraints(c)
	t.ClearLayoutDirty()
	return sz
}

func (t *progressTrack) Paint(pc *core.PaintContext) {
	if pc == nil || t.prog == nil {
		return
	}
	if pc.CompositeOnly && !t.NeedsPaint() && !pc.ForceFullPaint {
		return
	}
	p := t.prog
	th := p.theme()
	sz := t.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	h := sz.Height
	radius := h / 2
	rail := p.railCol(th)
	fill := p.strokeCol(th)
	pc.FillLocalRoundRect(0, 0, sz.Width, h, radius, rail)
	fw := sz.Width * p.FillRatio()
	if fw > 0 {
		if fw < h && p.FillRatio() > 0 {
			// keep a visible nub at very small percents
			if fw < 2 {
				fw = 2
			}
		}
		pc.FillLocalRoundRect(0, 0, fw, h, radius, fill)
		// active shimmer: translucent band sweeping over fill
		if p.EffectiveStatus() == ProgressStatusActive {
			band := math.Max(h*2, fw*0.35)
			x := -band + (fw+band)*p.activePh
			shimmer := th.Color(core.TokenColorBgContainer)
			if shimmer.A < 0.05 {
				shimmer = render.RGBA{R: 1, G: 1, B: 1, A: 0.45}
			} else {
				shimmer.A = 0.45
			}
			// clip-ish: only draw intersection with fill by limiting width
			sx := math.Max(0, x)
			ex := math.Min(fw, x+band)
			if ex > sx {
				pc.FillLocalRoundRect(sx, 0, ex-sx, h, radius, shimmer)
			}
		}
	}
	t.ClearPaintDirty()
}

func (t *progressTrack) HitTest(pt core.Point) core.Node {
	return t.DefaultHitTest(pt)
}

// ── circle / dashboard ring ──────────────────────────────────────────

type progressRing struct {
	core.NodeBase
	prog *Progress
}

func (r *progressRing) TypeID() string { return "kit.ProgressRing" }

func (r *progressRing) Layout(c core.Constraints) core.Size {
	if sz, ok := r.LayoutSkipIfClean(c); ok {
		return sz
	}
	edge := DefaultProgressCircleSize
	if r.prog != nil {
		edge = r.prog.CircleSize()
	}
	sz := core.Size{Width: edge, Height: edge}
	// honor tight parent caps
	if c.HasBoundedWidth() && c.MaxWidth < edge {
		sz.Width = c.MaxWidth
		sz.Height = c.MaxWidth
	}
	if c.HasBoundedHeight() && c.MaxHeight < sz.Height {
		sz.Height = c.MaxHeight
		if sz.Width > sz.Height {
			sz.Width = sz.Height
		}
	}
	r.SetSize(sz)
	r.RememberConstraints(c)
	r.ClearLayoutDirty()
	return sz
}

func (r *progressRing) Paint(pc *core.PaintContext) {
	if pc == nil || r.prog == nil {
		return
	}
	if pc.CompositeOnly && !r.NeedsPaint() && !pc.ForceFullPaint {
		return
	}
	p := r.prog
	th := p.theme()
	sz := r.Size()
	edge := math.Min(sz.Width, sz.Height)
	if edge <= 0 {
		return
	}
	// strokeWidth is % of size (antd); min 3px equivalent
	swPct := p.StrokeWidth
	if swPct <= 0 {
		swPct = DefaultProgressStrokeWidth
	}
	stroke := edge * swPct / 100
	minStroke := 3.0
	if stroke < minStroke {
		stroke = minStroke
	}
	cx, cy := sz.Width/2, sz.Height/2
	rad := edge/2 - stroke/2
	if rad < 1 {
		rad = 1
	}
	rail := p.railCol(th)
	fill := p.strokeCol(th)

	gap := p.gapDegree() * math.Pi / 180
	start := p.gapStartAngle()
	usable := 2*math.Pi - gap
	if usable < 0.01 {
		usable = 0.01
	}

	// rail arc (full usable)
	strokeArc(pc, cx, cy, rad, start, start+usable, stroke, rail)
	// progress arc
	prog := p.FillRatio()
	if prog > 0 {
		strokeArc(pc, cx, cy, rad, start, start+usable*prog, stroke, fill)
	}
	r.ClearPaintDirty()
}

func (r *progressRing) HitTest(pt core.Point) core.Node {
	return r.DefaultHitTest(pt)
}

func strokeArc(pc *core.PaintContext, cx, cy, rad, start, end, lineW float64, col render.RGBA) {
	if pc == nil || col.A <= 0 || rad <= 0 || lineW <= 0 {
		return
	}
	span := end - start
	if math.Abs(span) < 1e-6 {
		return
	}
	steps := int(math.Ceil(math.Abs(span) / (math.Pi / 64)))
	if steps < 4 {
		steps = 4
	}
	if steps > 128 {
		steps = 128
	}
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		a := start + span*t
		pts = append(pts, cx+rad*math.Cos(a), cy+rad*math.Sin(a))
	}
	pc.StrokeLocalPolyline(pts, lineW, col)
}

func clampProgress(v float64) float64 {
	if v < 0 || math.IsNaN(v) {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
