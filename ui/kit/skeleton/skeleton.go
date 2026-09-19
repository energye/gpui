// Package skeleton implements the Skeleton control (docs/antd/skeleton.md §6).
//
// Single owner, no second engine: layout/paint through ui/rendering,
// tokens through ui/theme, ticks through ui/scheduler.
package skeleton

import (
	"math"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// Geometry baselines (docs/antd/skeleton.md §6.2, controlHeight=32).
const (
	TitleHeight       = 16.0
	ParagraphLiHeight = 16.0
	BlockRadius       = 4.0
	RoundRadius       = 999.0
	AvatarSmall       = 24.0
	AvatarMiddle      = 32.0
	AvatarLarge       = 40.0
	ImageSize         = 96.0
	NodeSize          = 96.0
	DefaultWidth      = 400.0
	LastRowRatio      = 0.61
	TitleNoAvatarRat  = 0.38
	TitleAvatarRat    = 0.50
	TitleGap          = 16.0
	RowGap            = 16.0
	AvatarGap         = 16.0
	ShimmerPeriodSec  = 1.4
)

// SkeletonSize selects the control-height档 (small|middle|large).
type SkeletonSize string

const (
	SizeSmall  SkeletonSize = "small"
	SizeMiddle SkeletonSize = "middle"
	SizeLarge  SkeletonSize = "large"
)

// SkeletonAvatarShape selects avatar geometry (circle|square).
type SkeletonAvatarShape string

const (
	AvatarCircle SkeletonAvatarShape = "circle"
	AvatarSquare SkeletonAvatarShape = "square"
)

// SkeletonButtonShape selects button geometry.
type SkeletonButtonShape string

const (
	ButtonDefault SkeletonButtonShape = "default"
	ButtonCircle  SkeletonButtonShape = "circle"
	ButtonRound   SkeletonButtonShape = "round"
	ButtonSquare  SkeletonButtonShape = "square"
)

// Style is a shallow style hook (no CSS engine).
type Style map[string]string

// SkeletonClassNames holds shallow semantic hooks.
type SkeletonClassNames struct {
	Root      string
	Header    string
	Section   string
	Avatar    string
	Title     string
	Paragraph string
}

// SkeletonStyles holds shallow style hooks per semantic part.
type SkeletonStyles struct {
	Root      Style
	Header    Style
	Section   Style
	Avatar    Style
	Title     Style
	Paragraph Style
}

// Skeleton is the skeleton placeholder (docs/antd/skeleton.md §6.10).
type Skeleton struct {
	loading            atomic.Bool // setters write (UI), paint reads (raster)
	active             atomic.Bool // Tick/setters write (UI), paint reads (raster)
	hasAvatar          bool
	hasTitle           bool
	hasParagraph       bool
	paragraphRows      int
	round              bool
	content            rendering.RenderObject
	avatarShape        SkeletonAvatarShape
	avatarSize         SkeletonSize
	avatarSizePx       float64
	titleWidth         float64
	titleWidthStr      string
	paragraphWidths    []float64
	paragraphWidthStrs []string
	provider           *theme.Provider
	override           *theme.Tokens
	style              Style
	classNames         SkeletonClassNames
	styles             SkeletonStyles
	phase              atomic.Uint64 // math.Float64bits, same threading as active
	reduceMotion       bool
	// snap freezes every paint input on the UI thread (R2-6 button
	// paradigm). Raster paint reads only this snapshot plus the active /
	// loading / phase atomics. See snapshot.go.
	snap atomic.Value // SkeletonSnap
	host               *rendering.RenderBox
	attached           *scheduler.TickerRegistry
}

// NewSkeleton creates the basic placeholder (title + 3 rows, last 61%).
func NewSkeleton() *Skeleton {
	s := &Skeleton{
		hasTitle:     true,
		hasParagraph: true,
		avatarShape:  AvatarCircle,
		avatarSize:   SizeLarge,
	}
	s.loading.Store(true)
	s.host = rendering.NewRenderBox()
	s.host.SetRepaintBoundary(true)
	s.host.SetRelayoutBoundary(true)
	s.refreshSnapshot()
	paint := s
	s.host.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paint.paint(pc, size)
	}
	return s
}

// Loading reports the loading flag.
func (s *Skeleton) Loading() bool { return s == nil || s.loading.Load() }

// SetLoading toggles skeleton vs children (default true).
func (s *Skeleton) SetLoading(b bool) {
	if s == nil || s.loading.Load() == b {
		return
	}
	s.loading.Store(b)
	s.syncContent()
	s.dirty()
}

// Active reports the shimmer flag.
func (s *Skeleton) Active() bool { return s != nil && s.active.Load() }

// SetActive toggles the 1.4s shimmer (default false).
func (s *Skeleton) SetActive(b bool) {
	if s == nil || s.active.Load() == b {
		return
	}
	s.active.Store(b)
	s.dirtyPaint()
}

// SetAvatar toggles the avatar block (default false).
func (s *Skeleton) SetAvatar(b bool) {
	if s == nil || s.hasAvatar == b {
		return
	}
	s.hasAvatar = b
	s.dirty()
}

// HasAvatar reports the avatar flag.
func (s *Skeleton) HasAvatar() bool { return s != nil && s.hasAvatar }

// SetTitle toggles the title bar (default true).
func (s *Skeleton) SetTitle(b bool) {
	if s == nil || s.hasTitle == b {
		return
	}
	s.hasTitle = b
	s.dirty()
}

// HasTitle reports the title flag.
func (s *Skeleton) HasTitle() bool { return s != nil && s.hasTitle }

// SetParagraph toggles paragraph rows (default true).
func (s *Skeleton) SetParagraph(b bool) {
	if s == nil || s.hasParagraph == b {
		return
	}
	s.hasParagraph = b
	s.dirty()
}

// HasParagraph reports the paragraph flag.
func (s *Skeleton) HasParagraph() bool { return s != nil && s.hasParagraph }

// SetParagraphRows sets explicit rows (<=0 selects antd default 3/2).
func (s *Skeleton) SetParagraphRows(n int) {
	if s == nil || s.paragraphRows == n {
		return
	}
	s.paragraphRows = n
	s.dirty()
}

// ParagraphRows returns the raw rows setting (0 means auto).
func (s *Skeleton) ParagraphRows() int {
	if s == nil {
		return 0
	}
	return s.paragraphRows
}

// EffectiveRows resolves rows (0 when paragraph off).
func (s *Skeleton) EffectiveRows() int {
	if s == nil || !s.hasParagraph {
		return 0
	}
	if s.paragraphRows > 0 {
		return s.paragraphRows
	}
	if s.hasAvatar {
		return 2
	}
	return 3
}

// SetRound toggles capsule radius for title/paragraph (default false).
func (s *Skeleton) SetRound(b bool) {
	if s == nil || s.round == b {
		return
	}
	s.round = b
	s.dirtyPaint()
}

// Round reports the round flag.
func (s *Skeleton) Round() bool { return s != nil && s.round }

// EffectiveRadius returns capsule or block radius.
func (s *Skeleton) EffectiveRadius() float64 {
	tok := s.tokens()
	base := BlockRadius
	if tok.RadiusSM > 0 {
		base = tok.RadiusSM
	}
	if s != nil && s.round {
		return RoundRadius
	}
	return base
}

// SetContent sets the loading=false children.
func (s *Skeleton) SetContent(n rendering.RenderObject) {
	if s == nil {
		return
	}
	s.content = n
	s.syncContent()
	s.dirty()
}

// Content returns the children node (nil when unset).
func (s *Skeleton) Content() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.content
}

// IsShowingContent reports loading=false with content attached.
func (s *Skeleton) IsShowingContent() bool {
	if s == nil || s.loading.Load() || s.content == nil {
		return false
	}
	return true
}

// SetAvatarShape sets circle|square (unknown maps to circle).
func (s *Skeleton) SetAvatarShape(sh SkeletonAvatarShape) {
	if s == nil {
		return
	}
	if sh != AvatarSquare {
		sh = AvatarCircle
	}
	if s.avatarShape == sh {
		return
	}
	s.avatarShape = sh
	s.dirty()
}

// AvatarShape returns the effective shape.
func (s *Skeleton) AvatarShape() SkeletonAvatarShape {
	if s == nil || s.avatarShape == "" {
		return AvatarCircle
	}
	return s.avatarShape
}

// SetAvatarSize sets small|middle|large.
func (s *Skeleton) SetAvatarSize(sz SkeletonSize) {
	if s == nil {
		return
	}
	if s.avatarSize == sz && s.avatarSizePx == 0 {
		return
	}
	s.avatarSize = sz
	s.avatarSizePx = 0
	s.dirty()
}

// AvatarSize returns the raw size setting.
func (s *Skeleton) AvatarSize() SkeletonSize {
	if s == nil || s.avatarSize == "" {
		return SizeLarge
	}
	return s.avatarSize
}

// EffectiveAvatarSize resolves px from theme control heights.
func (s *Skeleton) EffectiveAvatarSize() float64 {
	if s != nil && s.avatarSizePx > 0 {
		return s.avatarSizePx
	}
	tok := s.tokens()
	sm, mid, lg := controlHeights(tok)
	switch s.AvatarSize() {
	case SizeSmall:
		return sm
	case SizeMiddle:
		return mid
	default:
		return lg
	}
}

// SetAvatarSizePx sets a numeric avatar edge (P1 number form, >0 wins).
func (s *Skeleton) SetAvatarSizePx(px float64) {
	if s == nil {
		return
	}
	if px <= 0 {
		px = 0
	}
	if s.avatarSizePx == px {
		return
	}
	s.avatarSizePx = px
	s.dirty()
}

// AvatarSizePx returns the numeric override (0 means enum wins).
func (s *Skeleton) AvatarSizePx() float64 {
	if s == nil {
		return 0
	}
	return s.avatarSizePx
}

// SetTitleWidth sets P0 title width (<=0 auto, <=1 ratio, else px).
func (s *Skeleton) SetTitleWidth(w float64) {
	if s == nil {
		return
	}
	if s.titleWidth == w && s.titleWidthStr == "" {
		return
	}
	s.titleWidth = w
	s.titleWidthStr = ""
	s.dirty()
}

// TitleWidth returns the raw setting.
func (s *Skeleton) TitleWidth() float64 {
	if s == nil {
		return 0
	}
	return s.titleWidth
}

// SetTitleWidthStr sets P1 title width ("50%", "200px", "200").
func (s *Skeleton) SetTitleWidthStr(w string) {
	if s == nil {
		return
	}
	w = strings.TrimSpace(w)
	if s.titleWidthStr == w {
		return
	}
	s.titleWidthStr = w
	if w != "" {
		s.titleWidth = 0
	}
	s.dirty()
}

// TitleWidthStr returns the raw P1 string setting.
func (s *Skeleton) TitleWidthStr() string {
	if s == nil {
		return ""
	}
	return s.titleWidthStr
}

// EffectiveTitleWidth resolves px against total width.
func (s *Skeleton) EffectiveTitleWidth(totalW float64) float64 {
	if s == nil || !s.hasTitle {
		return 0
	}
	avail := s.rightAvail(totalW)
	if s.titleWidthStr != "" {
		if v, ok := parseWidthStr(s.titleWidthStr, avail); ok {
			return v
		}
	}
	if s.titleWidth > 0 {
		return resolveWidth(s.titleWidth, avail)
	}
	if !s.hasParagraph {
		return avail
	}
	if s.hasAvatar {
		return avail * TitleAvatarRat
	}
	return avail * TitleNoAvatarRat
}

// SetParagraphWidths sets P0 widths (one value presses last row only).
func (s *Skeleton) SetParagraphWidths(ws ...float64) {
	if s == nil {
		return
	}
	s.paragraphWidths = append([]float64(nil), ws...)
	s.paragraphWidthStrs = nil
	s.dirty()
}

// ParagraphWidths returns a copy of the raw settings.
func (s *Skeleton) ParagraphWidths() []float64 {
	if s == nil {
		return nil
	}
	return append([]float64(nil), s.paragraphWidths...)
}

// SetParagraphWidthsStr sets P1 widths ("61%", "200px", "200").
func (s *Skeleton) SetParagraphWidthsStr(ws ...string) {
	if s == nil {
		return
	}
	cp := make([]string, len(ws))
	for i, w := range ws {
		cp[i] = strings.TrimSpace(w)
	}
	s.paragraphWidthStrs = cp
	s.paragraphWidths = nil
	s.dirty()
}

// ParagraphWidthsStr returns a copy of the P1 string settings.
func (s *Skeleton) ParagraphWidthsStr() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.paragraphWidthStrs...)
}

// EffectiveParagraphWidths resolves per-row px against total width.
func (s *Skeleton) EffectiveParagraphWidths(totalW float64) []float64 {
	rows := s.EffectiveRows()
	if rows <= 0 {
		return nil
	}
	avail := s.rightAvail(totalW)
	out := make([]float64, rows)
	if len(s.paragraphWidthStrs) > 0 {
		ex := s.paragraphWidthStrs
		switch {
		case len(ex) == 1:
			for i := range out {
				if i == rows-1 {
					if v, ok := parseWidthStr(ex[0], avail); ok {
						out[i] = v
					} else {
						out[i] = avail * LastRowRatio
					}
				} else {
					out[i] = avail
				}
			}
		default:
			for i := range out {
				if i < len(ex) && ex[i] != "" {
					if v, ok := parseWidthStr(ex[i], avail); ok {
						out[i] = v
						continue
					}
				}
				if i == rows-1 {
					out[i] = avail * LastRowRatio
				} else {
					out[i] = avail
				}
			}
		}
		return out
	}
	ex := s.paragraphWidths
	switch {
	case len(ex) == 1:
		for i := range out {
			if i == rows-1 {
				if ex[0] <= 0 {
					out[i] = avail * LastRowRatio
				} else {
					out[i] = resolveWidth(ex[0], avail)
				}
			} else {
				out[i] = avail
			}
		}
	case len(ex) > 1:
		for i := range out {
			if i < len(ex) && ex[i] > 0 {
				out[i] = resolveWidth(ex[i], avail)
			} else if i == rows-1 {
				out[i] = avail * LastRowRatio
			} else {
				out[i] = avail
			}
		}
	default:
		for i := range out {
			if i == rows-1 {
				out[i] = avail * LastRowRatio
			} else {
				out[i] = avail
			}
		}
	}
	return out
}

// SetProvider selects the theme source (nil selects process default).
func (s *Skeleton) SetProvider(p *theme.Provider) {
	if s == nil {
		return
	}
	s.provider = p
	s.dirtyPaint()
}

// SetTheme pins exact tokens (nil clears to provider).
func (s *Skeleton) SetTheme(t *theme.Tokens) {
	if s == nil {
		return
	}
	s.override = t
	s.dirtyPaint()
}

func (s *Skeleton) tokens() theme.Tokens {
	if s != nil && s.override != nil {
		return *s.override
	}
	if s != nil && s.provider != nil {
		return s.provider.Current()
	}
	return theme.Default.Current()
}

// EffectiveFillColor returns the skeleton base fill (theme ColorFillSecondary).
func (s *Skeleton) EffectiveFillColor() render.RGBA {
	tok := s.tokens()
	c := tok.ColorFillSecondary
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// EffectiveTitleHeight returns controlHeight/2 (fallback 16).
func (s *Skeleton) EffectiveTitleHeight() float64 {
	tok := s.tokens()
	if tok.ControlHeight > 0 {
		return tok.ControlHeight / 2
	}
	return TitleHeight
}

// EffectiveRowHeight returns controlHeight/2 (fallback 16).
func (s *Skeleton) EffectiveRowHeight() float64 {
	tok := s.tokens()
	if tok.ControlHeight > 0 {
		return tok.ControlHeight / 2
	}
	return ParagraphLiHeight
}

// SetStyle stores the root shallow style.
func (s *Skeleton) SetStyle(st Style) {
	if s == nil {
		return
	}
	s.style = st
	s.dirtyPaint()
}

// Style returns the stored root style.
func (s *Skeleton) Style() Style {
	if s == nil {
		return nil
	}
	return s.style
}

// SetClassNames stores semantic hooks.
func (s *Skeleton) SetClassNames(c SkeletonClassNames) {
	if s == nil {
		return
	}
	s.classNames = c
	s.dirtyPaint()
}

// ClassNames returns the stored hooks.
func (s *Skeleton) ClassNames() SkeletonClassNames {
	if s == nil {
		return SkeletonClassNames{}
	}
	return s.classNames
}

// SetStyles stores shallow styles per part.
func (s *Skeleton) SetStyles(st SkeletonStyles) {
	if s == nil {
		return
	}
	s.styles = st
	s.dirtyPaint()
}

// Styles returns the stored per-part styles.
func (s *Skeleton) Styles() SkeletonStyles {
	if s == nil {
		return SkeletonStyles{}
	}
	return s.styles
}

// SemanticParts lists the §1.5 nodes for _semantic coverage.
func (s *Skeleton) SemanticParts() []string {
	return []string{"root", "header", "section", "avatar", "title", "paragraph"}
}

// Focusable is always false: decorative placeholder never takes Tab.
func (s *Skeleton) Focusable() bool { return false }

// Role returns "" for the decorative skeleton (children own a11y).
func (s *Skeleton) Role() string { return "" }

// AriaLabel returns "" (decorative, no name).
func (s *Skeleton) AriaLabel() string { return "" }

// Node returns the tree node.
func (s *Skeleton) Node() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.host
}

// ChromeNode returns the host for golden probes.
func (s *Skeleton) ChromeNode() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.host
}

// Layout sizes the host (fill MaxWidth, height from blocks or content).
func (s *Skeleton) Layout(c rendering.Constraints) rendering.Size {
	if s == nil || s.host == nil {
		return rendering.Size{}
	}
	if !s.loading.Load() && s.content != nil {
		s.syncContent()
		s.host.FixedWidth, s.host.FixedHeight = 0, 0
		return s.host.Layout(c)
	}
	s.syncContent()
	W := DefaultWidth
	if c.MaxWidth < rendering.Unbounded/2 {
		W = c.MaxWidth
	} else if c.MinWidth > W {
		W = c.MinWidth
	}
	H := s.prefHeight(W)
	out := c.Tighten(rendering.Size{Width: W, Height: H})
	s.host.FixedWidth, s.host.FixedHeight = out.Width, out.Height
	return s.host.Layout(c)
}

// SetReduceMotion freezes the shimmer phase.
func (s *Skeleton) SetReduceMotion(b bool) {
	if s == nil {
		return
	}
	s.reduceMotion = b
	s.refreshSnapshot()
	s.dirtyPaint()
}

// ReduceMotion reports the flag.
func (s *Skeleton) ReduceMotion() bool { return s != nil && s.reduceMotion }

// Phase returns the shimmer phase in [0,1).
func (s *Skeleton) Phase() float64 {
	if s == nil {
		return 0
	}
	return math.Float64frombits(s.phase.Load())
}

// Attach registers the shimmer ticker.
func (s *Skeleton) Attach(reg *scheduler.TickerRegistry) {
	if s == nil || reg == nil {
		return
	}
	if s.attached != nil && s.attached != reg {
		s.attached.Remove(s)
	}
	s.attached = reg
	reg.Add(s)
}

// Detach unregisters the ticker.
func (s *Skeleton) Detach() {
	if s == nil || s.attached == nil {
		return
	}
	s.attached.Remove(s)
	s.attached = nil
}

// Tick advances the 1.4s shimmer (stays registered).
func (s *Skeleton) Tick(dt float64) bool {
	if s == nil {
		return false
	}
	if !s.active.Load() || s.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	ph := math.Float64frombits(s.phase.Load()) + dt/ShimmerPeriodSec
	ph = math.Mod(ph, 1)
	if ph < 0 {
		ph++
	}
	s.phase.Store(math.Float64bits(ph))
	s.host.MarkNeedsPaint()
	return true
}

// WantsFrame reports shimmer demand.
func (s *Skeleton) WantsFrame() bool {
	return s != nil && s.active.Load() && !s.reduceMotion && s.loading.Load()
}

func (s *Skeleton) rightAvail(totalW float64) float64 {
	if s != nil && s.hasAvatar {
		avail := totalW - s.EffectiveAvatarSize() - AvatarGap
		if avail < 0 {
			return 0
		}
		return avail
	}
	if totalW < 0 {
		return 0
	}
	return totalW
}

func (s *Skeleton) prefHeight(totalW float64) float64 {
	if s == nil {
		return 0
	}
	titleH := s.EffectiveTitleHeight()
	rowH := s.EffectiveRowHeight()
	rows := s.EffectiveRows()
	right := 0.0
	if s.hasTitle {
		right += titleH
	}
	if s.hasTitle && rows > 0 {
		right += TitleGap
	}
	if rows > 0 {
		right += float64(rows)*rowH + float64(max(0, rows-1))*RowGap
	}
	if s.hasAvatar {
		av := s.EffectiveAvatarSize()
		if av > right {
			return av
		}
		return right
	}
	return right
}

func (s *Skeleton) syncContent() {
	if s == nil || s.host == nil {
		return
	}
	has := false
	for _, ch := range s.host.Children() {
		if ch == s.content {
			has = true
			break
		}
	}
	if !s.loading.Load() && s.content != nil {
		if !has {
			s.host.AddChild(s.content)
		}
		return
	}
	if s.content != nil && has {
		s.host.RemoveChild(s.content)
	}
}

func (s *Skeleton) dirty() {
	if s == nil || s.host == nil {
		return
	}
	s.refreshSnapshot()
	s.host.MarkNeedsLayout()
}

func (s *Skeleton) dirtyPaint() {
	if s == nil || s.host == nil {
		return
	}
	s.refreshSnapshot()
	s.host.MarkNeedsPaint()
}

func (s *Skeleton) paint(pc *rendering.PaintContext, size rendering.Size) {
	if s == nil || pc == nil || !s.loading.Load() {
		return
	}
	W, H := size.Width, size.Height
	if W <= 0 || H <= 0 {
		return
	}
	S := s.loadSnapshot()
	fill := S.Fill
	radius := S.Radius
	titleH := S.TitleHeight
	rowH := S.RowHeight
	widths := s.effParagraphWidthsSnap(S, W)
	titleW := s.effTitleWidthSnap(S, W)
	y := 0.0
	x0 := 0.0
	if S.HasAvatar {
		av := S.AvatarSizePx
		drawAvatarBlock(pc, 0, 0, av, S.AvatarShape, fill)
		x0 = av + AvatarGap
	}
	if S.HasTitle && titleW > 0 {
		rendering.FillRoundRect(pc, x0, y, titleW, titleH, radius, fill.R, fill.G, fill.B, fill.A)
		y += titleH + TitleGap
	} else if S.HasTitle {
		y += 0
	}
	for _, w := range widths {
		if w <= 0 {
			continue
		}
		rendering.FillRoundRect(pc, x0, y, w, rowH, radius, fill.R, fill.G, fill.B, fill.A)
		y += rowH + RowGap
	}
	if s.active.Load() && !S.ReduceMotion {
		hw := 80.0
		hx := -hw + (math.Float64frombits(s.phase.Load()))*(W+2*hw)
		rendering.FillRect(pc, hx, 0, hw, H, 1, 1, 1, 0.35)
	}
}

func drawAvatarBlock(pc *rendering.PaintContext, x, y, edge float64, sh SkeletonAvatarShape, fill render.RGBA) {
	if edge <= 0 {
		return
	}
	if sh == AvatarSquare {
		rendering.FillRoundRect(pc, x, y, edge, edge, BlockRadius, fill.R, fill.G, fill.B, fill.A)
		return
	}
	rendering.FillCircle(pc, x+edge/2, y+edge/2, edge/2, fill.R, fill.G, fill.B, fill.A)
}

func resolveWidth(v, avail float64) float64 {
	if v <= 0 || avail <= 0 {
		return avail
	}
	if v <= 1 {
		return v * avail
	}
	if v > avail {
		return avail
	}
	return v
}

// parseWidthStr parses P1 width strings ("50%", "200px", "200", "0.61").
func parseWidthStr(s string, avail float64) (float64, bool) {
	t := strings.TrimSpace(s)
	if t == "" || avail <= 0 {
		return 0, false
	}
	low := strings.ToLower(t)
	if strings.HasSuffix(low, "%") {
		num := strings.TrimSpace(t[:len(t)-1])
		v, err := strconv.ParseFloat(num, 64)
		if err != nil || v < 0 {
			return 0, false
		}
		got := v / 100 * avail
		if got > avail {
			got = avail
		}
		return got, true
	}
	if strings.HasSuffix(low, "px") {
		num := strings.TrimSpace(t[:len(t)-2])
		v, err := strconv.ParseFloat(num, 64)
		if err != nil || v < 0 {
			return 0, false
		}
		if v > avail {
			return avail, true
		}
		return v, true
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return resolveWidth(v, avail), true
}

func controlHeights(tok theme.Tokens) (sm, mid, lg float64) {
	sm, mid, lg = AvatarSmall, AvatarMiddle, AvatarLarge
	if tok.ControlHeightSM > 0 {
		sm = tok.ControlHeightSM
	}
	if tok.ControlHeight > 0 {
		mid = tok.ControlHeight
	}
	if tok.ControlHeightLG > 0 {
		lg = tok.ControlHeightLG
	}
	return sm, mid, lg
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SkeletonAvatar is the standalone avatar placeholder.
type SkeletonAvatar struct {
	size         SkeletonSize
	sizePx       float64
	shape        SkeletonAvatarShape
	active       atomic.Bool   // Tick/setters write (UI), paint reads (raster)
	phase        atomic.Uint64 // math.Float64bits, same threading as active
	reduceMotion bool
	provider     *theme.Provider
	override     *theme.Tokens
	style        Style
	snap         atomic.Value // SkeletonAvatarSnap
	host         *rendering.RenderBox
	attached     *scheduler.TickerRegistry
}

// NewSkeletonAvatar creates a medium circle avatar placeholder.
func NewSkeletonAvatar() *SkeletonAvatar {
	a := &SkeletonAvatar{size: SizeMiddle, shape: AvatarCircle}
	a.host = rendering.NewRenderBox()
	a.host.SetRepaintBoundary(true)
	a.refreshSnapshot()
	paint := a
	a.host.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paint.paint(pc, size)
	}
	return a
}

// SetSize sets small|middle|large.
func (a *SkeletonAvatar) SetSize(sz SkeletonSize) {
	if a == nil {
		return
	}
	if a.size == sz && a.sizePx == 0 {
		return
	}
	a.size = sz
	a.sizePx = 0
	a.refreshSnapshot()
	a.host.MarkNeedsLayout()
}

// Size returns the raw setting.
func (a *SkeletonAvatar) Size() SkeletonSize {
	if a == nil || a.size == "" {
		return SizeMiddle
	}
	return a.size
}

// EffectiveSize resolves px from theme.
func (a *SkeletonAvatar) EffectiveSize() float64 {
	if a != nil && a.sizePx > 0 {
		return a.sizePx
	}
	var tok theme.Tokens
	if a != nil && a.override != nil {
		tok = *a.override
	} else if a != nil && a.provider != nil {
		tok = a.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	sm, mid, lg := controlHeights(tok)
	switch a.Size() {
	case SizeSmall:
		return sm
	case SizeLarge:
		return lg
	default:
		return mid
	}
}

// SetSizePx sets a numeric edge (P1 number form, >0 wins).
func (a *SkeletonAvatar) SetSizePx(px float64) {
	if a == nil {
		return
	}
	if px <= 0 {
		px = 0
	}
	if a.sizePx == px {
		return
	}
	a.sizePx = px
	a.refreshSnapshot()
	a.host.MarkNeedsLayout()
}

// SizePx returns the numeric override (0 means enum wins).
func (a *SkeletonAvatar) SizePx() float64 {
	if a == nil {
		return 0
	}
	return a.sizePx
}

// SetShape sets circle|square.
func (a *SkeletonAvatar) SetShape(sh SkeletonAvatarShape) {
	if a == nil {
		return
	}
	if sh != AvatarSquare {
		sh = AvatarCircle
	}
	if a.shape == sh {
		return
	}
	a.shape = sh
	a.refreshSnapshot()
	a.host.MarkNeedsPaint()
}

// Shape returns the effective shape.
func (a *SkeletonAvatar) Shape() SkeletonAvatarShape {
	if a == nil || a.shape == "" {
		return AvatarCircle
	}
	return a.shape
}

// SetActive toggles shimmer.
func (a *SkeletonAvatar) SetActive(b bool) {
	if a == nil || a.active.Load() == b {
		return
	}
	a.active.Store(b)
	a.host.MarkNeedsPaint()
}

// Active reports the flag.
func (a *SkeletonAvatar) Active() bool { return a != nil && a.active.Load() }

// SetReduceMotion freezes shimmer.
func (a *SkeletonAvatar) SetReduceMotion(b bool) {
	if a == nil {
		return
	}
	a.reduceMotion = b
	a.refreshSnapshot()
	a.host.MarkNeedsPaint()
}

// Phase returns shimmer phase.
func (a *SkeletonAvatar) Phase() float64 {
	if a == nil {
		return 0
	}
	return math.Float64frombits(a.phase.Load())
}

// SetProvider selects theme source.
func (a *SkeletonAvatar) SetProvider(p *theme.Provider) {
	if a == nil {
		return
	}
	a.provider = p
	a.refreshSnapshot()
	a.host.MarkNeedsPaint()
}

// SetTheme pins tokens.
func (a *SkeletonAvatar) SetTheme(t *theme.Tokens) {
	if a == nil {
		return
	}
	a.override = t
	a.refreshSnapshot()
	a.host.MarkNeedsPaint()
}

// SetStyle stores shallow style.
func (a *SkeletonAvatar) SetStyle(st Style) {
	if a == nil {
		return
	}
	a.style = st
	a.refreshSnapshot()
	a.host.MarkNeedsPaint()
}

// Focusable is always false.
func (a *SkeletonAvatar) Focusable() bool { return false }

// Role returns "" (decorative).
func (a *SkeletonAvatar) Role() string { return "" }

// AriaLabel returns "".
func (a *SkeletonAvatar) AriaLabel() string { return "" }

// Node returns the tree node.
func (a *SkeletonAvatar) Node() rendering.RenderObject {
	if a == nil {
		return nil
	}
	return a.host
}

// Layout sizes the square.
func (a *SkeletonAvatar) Layout(c rendering.Constraints) rendering.Size {
	if a == nil || a.host == nil {
		return rendering.Size{}
	}
	edge := a.EffectiveSize()
	out := c.Tighten(rendering.Size{Width: edge, Height: edge})
	a.host.FixedWidth, a.host.FixedHeight = out.Width, out.Height
	return a.host.Layout(c)
}

// Attach registers ticker.
func (a *SkeletonAvatar) Attach(reg *scheduler.TickerRegistry) {
	if a == nil || reg == nil {
		return
	}
	if a.attached != nil && a.attached != reg {
		a.attached.Remove(a)
	}
	a.attached = reg
	reg.Add(a)
}

// Detach unregisters ticker.
func (a *SkeletonAvatar) Detach() {
	if a == nil || a.attached == nil {
		return
	}
	a.attached.Remove(a)
	a.attached = nil
}

// Tick advances shimmer.
func (a *SkeletonAvatar) Tick(dt float64) bool {
	if a == nil {
		return false
	}
	if !a.active.Load() || a.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	ph := math.Float64frombits(a.phase.Load()) + dt/ShimmerPeriodSec
	ph = math.Mod(ph, 1)
	if ph < 0 {
		ph++
	}
	a.phase.Store(math.Float64bits(ph))
	a.host.MarkNeedsPaint()
	return true
}

// WantsFrame reports demand.
func (a *SkeletonAvatar) WantsFrame() bool { return a != nil && a.active.Load() && !a.reduceMotion }

func (a *SkeletonAvatar) fill() render.RGBA {
	var tok theme.Tokens
	if a != nil && a.override != nil {
		tok = *a.override
	} else if a != nil && a.provider != nil {
		tok = a.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	c := tok.ColorFillSecondary
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func (a *SkeletonAvatar) paint(pc *rendering.PaintContext, size rendering.Size) {
	if a == nil || pc == nil || size.Width <= 0 {
		return
	}
	S := a.loadSnapshot()
	fill := S.Fill
	drawAvatarBlock(pc, 0, 0, size.Width, S.Shape, fill)
	if a.active.Load() && !S.ReduceMotion {
		hw := size.Width * 0.5
		hx := -hw + (math.Float64frombits(a.phase.Load()))*(size.Width+2*hw)
		rendering.FillRect(pc, hx, 0, hw, size.Height, 1, 1, 1, 0.35)
	}
}

// SkeletonButton is the button placeholder (h × 2h).
type SkeletonButton struct {
	size         SkeletonSize
	shape        SkeletonButtonShape
	block        bool
	active       atomic.Bool   // Tick/setters write (UI), paint reads (raster)
	phase        atomic.Uint64 // math.Float64bits, same threading as active
	reduceMotion bool
	provider     *theme.Provider
	override     *theme.Tokens
	style        Style
	snap         atomic.Value // SkeletonButtonSnap
	host         *rendering.RenderBox
	attached     *scheduler.TickerRegistry
}

// NewSkeletonButton creates a middle default button placeholder.
func NewSkeletonButton() *SkeletonButton {
	b := &SkeletonButton{size: SizeMiddle, shape: ButtonDefault}
	b.host = rendering.NewRenderBox()
	b.host.SetRepaintBoundary(true)
	b.refreshSnapshot()
	paint := b
	b.host.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paint.paint(pc, size)
	}
	return b
}

// SetSize sets small|middle|large.
func (b *SkeletonButton) SetSize(sz SkeletonSize) {
	if b == nil || b.size == sz {
		return
	}
	b.size = sz
	b.refreshSnapshot()
	b.host.MarkNeedsLayout()
}

// Size returns raw setting.
func (b *SkeletonButton) Size() SkeletonSize {
	if b == nil || b.size == "" {
		return SizeMiddle
	}
	return b.size
}

// EffectiveHeight resolves h from theme.
func (b *SkeletonButton) EffectiveHeight() float64 {
	var tok theme.Tokens
	if b != nil && b.override != nil {
		tok = *b.override
	} else if b != nil && b.provider != nil {
		tok = b.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	sm, mid, lg := controlHeights(tok)
	switch b.Size() {
	case SizeSmall:
		return sm
	case SizeLarge:
		return lg
	default:
		return mid
	}
}

// EffectiveWidth returns 2h (block fills MaxWidth at layout).
func (b *SkeletonButton) EffectiveWidth() float64 { return 2 * b.EffectiveHeight() }

// SetShape sets default|circle|round|square.
func (b *SkeletonButton) SetShape(sh SkeletonButtonShape) {
	if b == nil {
		return
	}
	if b.shape == sh {
		return
	}
	b.shape = sh
	b.refreshSnapshot()
	b.host.MarkNeedsPaint()
}

// Shape returns raw shape.
func (b *SkeletonButton) Shape() SkeletonButtonShape {
	if b == nil || b.shape == "" {
		return ButtonDefault
	}
	return b.shape
}

// SetBlock toggles full-width.
func (b *SkeletonButton) SetBlock(v bool) {
	if b == nil || b.block == v {
		return
	}
	b.block = v
	b.refreshSnapshot()
	b.host.MarkNeedsLayout()
}

// Block reports the flag.
func (b *SkeletonButton) Block() bool { return b != nil && b.block }

// SetActive toggles shimmer.
func (b *SkeletonButton) SetActive(v bool) {
	if b == nil || b.active.Load() == v {
		return
	}
	b.active.Store(v)
	b.host.MarkNeedsPaint()
}

// Active reports the flag.
func (b *SkeletonButton) Active() bool { return b != nil && b.active.Load() }

// SetReduceMotion freezes shimmer.
func (b *SkeletonButton) SetReduceMotion(v bool) {
	if b == nil {
		return
	}
	b.reduceMotion = v
	b.refreshSnapshot()
	b.host.MarkNeedsPaint()
}

// Phase returns shimmer phase.
func (b *SkeletonButton) Phase() float64 {
	if b == nil {
		return 0
	}
	return math.Float64frombits(b.phase.Load())
}

// SetProvider selects theme source.
func (b *SkeletonButton) SetProvider(p *theme.Provider) {
	if b == nil {
		return
	}
	b.provider = p
	b.refreshSnapshot()
	b.host.MarkNeedsPaint()
}

// SetTheme pins tokens.
func (b *SkeletonButton) SetTheme(t *theme.Tokens) {
	if b == nil {
		return
	}
	b.override = t
	b.refreshSnapshot()
	b.host.MarkNeedsPaint()
}

// SetStyle stores style.
func (b *SkeletonButton) SetStyle(st Style) {
	if b == nil {
		return
	}
	b.style = st
	b.refreshSnapshot()
	b.host.MarkNeedsPaint()
}

// Focusable is always false.
func (b *SkeletonButton) Focusable() bool { return false }

// Role returns "".
func (b *SkeletonButton) Role() string { return "" }

// AriaLabel returns "".
func (b *SkeletonButton) AriaLabel() string { return "" }

// Node returns tree node.
func (b *SkeletonButton) Node() rendering.RenderObject {
	if b == nil {
		return nil
	}
	return b.host
}

// Layout sizes h×2h (block fills MaxWidth).
func (b *SkeletonButton) Layout(c rendering.Constraints) rendering.Size {
	if b == nil || b.host == nil {
		return rendering.Size{}
	}
	h := b.EffectiveHeight()
	w := b.EffectiveWidth()
	if b.block && c.MaxWidth < rendering.Unbounded/2 {
		w = c.MaxWidth
	}
	out := c.Tighten(rendering.Size{Width: w, Height: h})
	b.host.FixedWidth, b.host.FixedHeight = out.Width, out.Height
	return b.host.Layout(c)
}

// Attach registers ticker.
func (b *SkeletonButton) Attach(reg *scheduler.TickerRegistry) {
	if b == nil || reg == nil {
		return
	}
	if b.attached != nil && b.attached != reg {
		b.attached.Remove(b)
	}
	b.attached = reg
	reg.Add(b)
}

// Detach unregisters ticker.
func (b *SkeletonButton) Detach() {
	if b == nil || b.attached == nil {
		return
	}
	b.attached.Remove(b)
	b.attached = nil
}

// Tick advances shimmer.
func (b *SkeletonButton) Tick(dt float64) bool {
	if b == nil {
		return false
	}
	if !b.active.Load() || b.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	ph := math.Float64frombits(b.phase.Load()) + dt/ShimmerPeriodSec
	ph = math.Mod(ph, 1)
	if ph < 0 {
		ph++
	}
	b.phase.Store(math.Float64bits(ph))
	b.host.MarkNeedsPaint()
	return true
}

// WantsFrame reports demand.
func (b *SkeletonButton) WantsFrame() bool { return b != nil && b.active.Load() && !b.reduceMotion }

func (b *SkeletonButton) fill() render.RGBA {
	var tok theme.Tokens
	if b != nil && b.override != nil {
		tok = *b.override
	} else if b != nil && b.provider != nil {
		tok = b.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	c := tok.ColorFillSecondary
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func (b *SkeletonButton) radius() float64 {
	switch b.Shape() {
	case ButtonCircle, ButtonRound:
		return RoundRadius
	case ButtonSquare:
		return 0
	default:
		return BlockRadius
	}
}

func (b *SkeletonButton) paint(pc *rendering.PaintContext, size rendering.Size) {
	if b == nil || pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	S := b.loadSnapshot()
	fill := S.Fill
	rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, S.Radius, fill.R, fill.G, fill.B, fill.A)
	if b.active.Load() && !S.ReduceMotion {
		hw := 40.0
		hx := -hw + (math.Float64frombits(b.phase.Load()))*(size.Width+2*hw)
		rendering.FillRect(pc, hx, 0, hw, size.Height, 1, 1, 1, 0.35)
	}
}

// SkeletonInput is the input placeholder (h × 5h).
type SkeletonInput struct {
	size         SkeletonSize
	block        bool
	active       atomic.Bool   // Tick/setters write (UI), paint reads (raster)
	phase        atomic.Uint64 // math.Float64bits, same threading as active
	reduceMotion bool
	provider     *theme.Provider
	override     *theme.Tokens
	style        Style
	snap         atomic.Value // SkeletonInputSnap
	host         *rendering.RenderBox
	attached     *scheduler.TickerRegistry
}

// NewSkeletonInput creates a middle input placeholder.
func NewSkeletonInput() *SkeletonInput {
	in := &SkeletonInput{size: SizeMiddle}
	in.host = rendering.NewRenderBox()
	in.host.SetRepaintBoundary(true)
	in.refreshSnapshot()
	paint := in
	in.host.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paint.paint(pc, size)
	}
	return in
}

// SetSize sets small|middle|large.
func (in *SkeletonInput) SetSize(sz SkeletonSize) {
	if in == nil || in.size == sz {
		return
	}
	in.size = sz
	in.refreshSnapshot()
	in.host.MarkNeedsLayout()
}

// Size returns raw setting.
func (in *SkeletonInput) Size() SkeletonSize {
	if in == nil || in.size == "" {
		return SizeMiddle
	}
	return in.size
}

// EffectiveHeight resolves h from theme.
func (in *SkeletonInput) EffectiveHeight() float64 {
	var tok theme.Tokens
	if in != nil && in.override != nil {
		tok = *in.override
	} else if in != nil && in.provider != nil {
		tok = in.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	sm, mid, lg := controlHeights(tok)
	switch in.Size() {
	case SizeSmall:
		return sm
	case SizeLarge:
		return lg
	default:
		return mid
	}
}

// EffectiveWidth returns 5h.
func (in *SkeletonInput) EffectiveWidth() float64 { return 5 * in.EffectiveHeight() }

// SetBlock toggles full-width.
func (in *SkeletonInput) SetBlock(v bool) {
	if in == nil || in.block == v {
		return
	}
	in.block = v
	in.refreshSnapshot()
	in.host.MarkNeedsLayout()
}

// Block reports the flag.
func (in *SkeletonInput) Block() bool { return in != nil && in.block }

// SetActive toggles shimmer.
func (in *SkeletonInput) SetActive(v bool) {
	if in == nil || in.active.Load() == v {
		return
	}
	in.active.Store(v)
	in.host.MarkNeedsPaint()
}

// Active reports the flag.
func (in *SkeletonInput) Active() bool { return in != nil && in.active.Load() }

// SetReduceMotion freezes shimmer.
func (in *SkeletonInput) SetReduceMotion(v bool) {
	if in == nil {
		return
	}
	in.reduceMotion = v
	in.refreshSnapshot()
	in.host.MarkNeedsPaint()
}

// Phase returns shimmer phase.
func (in *SkeletonInput) Phase() float64 {
	if in == nil {
		return 0
	}
	return math.Float64frombits(in.phase.Load())
}

// SetProvider selects theme source.
func (in *SkeletonInput) SetProvider(p *theme.Provider) {
	if in == nil {
		return
	}
	in.provider = p
	in.refreshSnapshot()
	in.host.MarkNeedsPaint()
}

// SetTheme pins tokens.
func (in *SkeletonInput) SetTheme(t *theme.Tokens) {
	if in == nil {
		return
	}
	in.override = t
	in.refreshSnapshot()
	in.host.MarkNeedsPaint()
}

// SetStyle stores style.
func (in *SkeletonInput) SetStyle(st Style) {
	if in == nil {
		return
	}
	in.style = st
	in.refreshSnapshot()
	in.host.MarkNeedsPaint()
}

// Focusable is always false.
func (in *SkeletonInput) Focusable() bool { return false }

// Role returns "".
func (in *SkeletonInput) Role() string { return "" }

// AriaLabel returns "".
func (in *SkeletonInput) AriaLabel() string { return "" }

// Node returns tree node.
func (in *SkeletonInput) Node() rendering.RenderObject {
	if in == nil {
		return nil
	}
	return in.host
}

// Layout sizes h×5h (block fills MaxWidth).
func (in *SkeletonInput) Layout(c rendering.Constraints) rendering.Size {
	if in == nil || in.host == nil {
		return rendering.Size{}
	}
	h := in.EffectiveHeight()
	w := in.EffectiveWidth()
	if in.block && c.MaxWidth < rendering.Unbounded/2 {
		w = c.MaxWidth
	}
	out := c.Tighten(rendering.Size{Width: w, Height: h})
	in.host.FixedWidth, in.host.FixedHeight = out.Width, out.Height
	return in.host.Layout(c)
}

// Attach registers ticker.
func (in *SkeletonInput) Attach(reg *scheduler.TickerRegistry) {
	if in == nil || reg == nil {
		return
	}
	if in.attached != nil && in.attached != reg {
		in.attached.Remove(in)
	}
	in.attached = reg
	reg.Add(in)
}

// Detach unregisters ticker.
func (in *SkeletonInput) Detach() {
	if in == nil || in.attached == nil {
		return
	}
	in.attached.Remove(in)
	in.attached = nil
}

// Tick advances shimmer.
func (in *SkeletonInput) Tick(dt float64) bool {
	if in == nil {
		return false
	}
	if !in.active.Load() || in.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	ph := math.Float64frombits(in.phase.Load()) + dt/ShimmerPeriodSec
	ph = math.Mod(ph, 1)
	if ph < 0 {
		ph++
	}
	in.phase.Store(math.Float64bits(ph))
	in.host.MarkNeedsPaint()
	return true
}

// WantsFrame reports demand.
func (in *SkeletonInput) WantsFrame() bool { return in != nil && in.active.Load() && !in.reduceMotion }

func (in *SkeletonInput) fill() render.RGBA {
	var tok theme.Tokens
	if in != nil && in.override != nil {
		tok = *in.override
	} else if in != nil && in.provider != nil {
		tok = in.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	c := tok.ColorFillSecondary
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func (in *SkeletonInput) paint(pc *rendering.PaintContext, size rendering.Size) {
	if in == nil || pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	S := in.loadSnapshot()
	fill := S.Fill
	rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, BlockRadius, fill.R, fill.G, fill.B, fill.A)
	if in.active.Load() && !S.ReduceMotion {
		hw := 40.0
		hx := -hw + (math.Float64frombits(in.phase.Load()))*(size.Width+2*hw)
		rendering.FillRect(pc, hx, 0, hw, size.Height, 1, 1, 1, 0.35)
	}
}

// SkeletonImage is the 96×96 image placeholder.
type SkeletonImage struct {
	active       atomic.Bool   // Tick/setters write (UI), paint reads (raster)
	phase        atomic.Uint64 // math.Float64bits, same threading as active
	reduceMotion bool
	provider     *theme.Provider
	override     *theme.Tokens
	style        Style
	snap         atomic.Value // SkeletonImageSnap
	host         *rendering.RenderBox
	attached     *scheduler.TickerRegistry
}

// NewSkeletonImage creates the 96×96 placeholder.
func NewSkeletonImage() *SkeletonImage {
	im := &SkeletonImage{}
	im.host = rendering.NewRenderBox()
	im.host.SetRepaintBoundary(true)
	im.refreshSnapshot()
	paint := im
	im.host.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paint.paint(pc, size)
	}
	return im
}

// EffectiveSize returns theme ControlHeight*3 (fallback 96).
func (im *SkeletonImage) EffectiveSize() float64 {
	var tok theme.Tokens
	if im != nil && im.override != nil {
		tok = *im.override
	} else if im != nil && im.provider != nil {
		tok = im.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	if tok.ControlHeight > 0 {
		return tok.ControlHeight * 3
	}
	return ImageSize
}

// SetActive toggles shimmer.
func (im *SkeletonImage) SetActive(v bool) {
	if im == nil || im.active.Load() == v {
		return
	}
	im.active.Store(v)
	im.host.MarkNeedsPaint()
}

// Active reports the flag.
func (im *SkeletonImage) Active() bool { return im != nil && im.active.Load() }

// SetReduceMotion freezes shimmer.
func (im *SkeletonImage) SetReduceMotion(v bool) {
	if im == nil {
		return
	}
	im.reduceMotion = v
	im.refreshSnapshot()
	im.host.MarkNeedsPaint()
}

// Phase returns shimmer phase.
func (im *SkeletonImage) Phase() float64 {
	if im == nil {
		return 0
	}
	return math.Float64frombits(im.phase.Load())
}

// SetProvider selects theme source.
func (im *SkeletonImage) SetProvider(p *theme.Provider) {
	if im == nil {
		return
	}
	im.provider = p
	im.refreshSnapshot()
	im.host.MarkNeedsPaint()
}

// SetTheme pins tokens.
func (im *SkeletonImage) SetTheme(t *theme.Tokens) {
	if im == nil {
		return
	}
	im.override = t
	im.refreshSnapshot()
	im.host.MarkNeedsPaint()
}

// SetStyle stores style.
func (im *SkeletonImage) SetStyle(st Style) {
	if im == nil {
		return
	}
	im.style = st
	im.refreshSnapshot()
	im.host.MarkNeedsPaint()
}

// Focusable is always false.
func (im *SkeletonImage) Focusable() bool { return false }

// Role returns "".
func (im *SkeletonImage) Role() string { return "" }

// AriaLabel returns "".
func (im *SkeletonImage) AriaLabel() string { return "" }

// Node returns tree node.
func (im *SkeletonImage) Node() rendering.RenderObject {
	if im == nil {
		return nil
	}
	return im.host
}

// Layout sizes the square.
func (im *SkeletonImage) Layout(c rendering.Constraints) rendering.Size {
	if im == nil || im.host == nil {
		return rendering.Size{}
	}
	edge := im.EffectiveSize()
	out := c.Tighten(rendering.Size{Width: edge, Height: edge})
	im.host.FixedWidth, im.host.FixedHeight = out.Width, out.Height
	return im.host.Layout(c)
}

// Attach registers ticker.
func (im *SkeletonImage) Attach(reg *scheduler.TickerRegistry) {
	if im == nil || reg == nil {
		return
	}
	if im.attached != nil && im.attached != reg {
		im.attached.Remove(im)
	}
	im.attached = reg
	reg.Add(im)
}

// Detach unregisters ticker.
func (im *SkeletonImage) Detach() {
	if im == nil || im.attached == nil {
		return
	}
	im.attached.Remove(im)
	im.attached = nil
}

// Tick advances shimmer.
func (im *SkeletonImage) Tick(dt float64) bool {
	if im == nil {
		return false
	}
	if !im.active.Load() || im.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	ph := math.Float64frombits(im.phase.Load()) + dt/ShimmerPeriodSec
	ph = math.Mod(ph, 1)
	if ph < 0 {
		ph++
	}
	im.phase.Store(math.Float64bits(ph))
	im.host.MarkNeedsPaint()
	return true
}

// WantsFrame reports demand.
func (im *SkeletonImage) WantsFrame() bool { return im != nil && im.active.Load() && !im.reduceMotion }

func (im *SkeletonImage) fill() render.RGBA {
	var tok theme.Tokens
	if im != nil && im.override != nil {
		tok = *im.override
	} else if im != nil && im.provider != nil {
		tok = im.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	c := tok.ColorFillSecondary
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func (im *SkeletonImage) paint(pc *rendering.PaintContext, size rendering.Size) {
	if im == nil || pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	S := im.loadSnapshot()
	fill := S.Fill
	rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, BlockRadius, fill.R, fill.G, fill.B, fill.A)
	cx, cy := size.Width/2, size.Height/2
	r := size.Width * 0.18
	if r > 14 {
		r = 14
	}
	rendering.StrokeCircle(pc, cx, cy, r+6, 2, 0.75, 0.75, 0.75, 1)
	rendering.FillCircle(pc, cx-4, cy-4, 2, 0.75, 0.75, 0.75, 1)
	if im.active.Load() && !S.ReduceMotion {
		hw := 40.0
		hx := -hw + (math.Float64frombits(im.phase.Load()))*(size.Width+2*hw)
		rendering.FillRect(pc, hx, 0, hw, size.Height, 1, 1, 1, 0.35)
	}
}

// SkeletonNode is the 96×96 custom-node placeholder.
type SkeletonNode struct {
	child        rendering.RenderObject
	active       atomic.Bool   // Tick/setters write (UI), paint reads (raster)
	phase        atomic.Uint64 // math.Float64bits, same threading as active
	reduceMotion bool
	provider     *theme.Provider
	override     *theme.Tokens
	style        Style
	snap         atomic.Value // SkeletonNodeSnap
	host         *rendering.RenderBox
	attached     *scheduler.TickerRegistry
}

// NewSkeletonNode creates a placeholder wrapping child (nil keeps 96×96).
func NewSkeletonNode(child rendering.RenderObject) *SkeletonNode {
	n := &SkeletonNode{child: child}
	n.host = rendering.NewRenderBox()
	n.host.SetRepaintBoundary(true)
	if child != nil {
		n.host.AddChild(child)
	}
	n.refreshSnapshot()
	paint := n
	n.host.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paint.paint(pc, size)
	}
	return n
}

// SetChild replaces the wrapped node.
func (n *SkeletonNode) SetChild(c rendering.RenderObject) {
	if n == nil {
		return
	}
	if n.child != nil {
		n.host.RemoveChild(n.child)
	}
	n.child = c
	if c != nil {
		n.host.AddChild(c)
	}
	n.refreshSnapshot()
	n.host.MarkNeedsLayout()
}

// Child returns the wrapped node.
func (n *SkeletonNode) Child() rendering.RenderObject {
	if n == nil {
		return nil
	}
	return n.child
}

// EffectiveSize returns 96 (or theme ControlHeight*3).
func (n *SkeletonNode) EffectiveSize() float64 {
	var tok theme.Tokens
	if n != nil && n.override != nil {
		tok = *n.override
	} else if n != nil && n.provider != nil {
		tok = n.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	if tok.ControlHeight > 0 {
		return tok.ControlHeight * 3
	}
	return NodeSize
}

// SetActive toggles shimmer.
func (n *SkeletonNode) SetActive(v bool) {
	if n == nil || n.active.Load() == v {
		return
	}
	n.active.Store(v)
	n.host.MarkNeedsPaint()
}

// Active reports the flag.
func (n *SkeletonNode) Active() bool { return n != nil && n.active.Load() }

// SetReduceMotion freezes shimmer.
func (n *SkeletonNode) SetReduceMotion(v bool) {
	if n == nil {
		return
	}
	n.reduceMotion = v
	n.refreshSnapshot()
	n.host.MarkNeedsPaint()
}

// Phase returns shimmer phase.
func (n *SkeletonNode) Phase() float64 {
	if n == nil {
		return 0
	}
	return math.Float64frombits(n.phase.Load())
}

// SetProvider selects theme source.
func (n *SkeletonNode) SetProvider(p *theme.Provider) {
	if n == nil {
		return
	}
	n.provider = p
	n.refreshSnapshot()
	n.host.MarkNeedsPaint()
}

// SetTheme pins tokens.
func (n *SkeletonNode) SetTheme(t *theme.Tokens) {
	if n == nil {
		return
	}
	n.override = t
	n.refreshSnapshot()
	n.host.MarkNeedsPaint()
}

// SetStyle stores style.
func (n *SkeletonNode) SetStyle(st Style) {
	if n == nil {
		return
	}
	n.style = st
	n.refreshSnapshot()
	n.host.MarkNeedsPaint()
}

// Focusable is always false.
func (n *SkeletonNode) Focusable() bool { return false }

// Role returns "".
func (n *SkeletonNode) Role() string { return "" }

// AriaLabel returns "".
func (n *SkeletonNode) AriaLabel() string { return "" }

// Node returns tree node.
func (n *SkeletonNode) Node() rendering.RenderObject {
	if n == nil {
		return nil
	}
	return n.host
}

// Layout sizes to child or 96×96.
func (n *SkeletonNode) Layout(c rendering.Constraints) rendering.Size {
	if n == nil || n.host == nil {
		return rendering.Size{}
	}
	if n.child == nil {
		edge := n.EffectiveSize()
		out := c.Tighten(rendering.Size{Width: edge, Height: edge})
		n.host.FixedWidth, n.host.FixedHeight = out.Width, out.Height
		return n.host.Layout(c)
	}
	n.host.FixedWidth, n.host.FixedHeight = 0, 0
	return n.host.Layout(c)
}

// Attach registers ticker.
func (n *SkeletonNode) Attach(reg *scheduler.TickerRegistry) {
	if n == nil || reg == nil {
		return
	}
	if n.attached != nil && n.attached != reg {
		n.attached.Remove(n)
	}
	n.attached = reg
	reg.Add(n)
}

// Detach unregisters ticker.
func (n *SkeletonNode) Detach() {
	if n == nil || n.attached == nil {
		return
	}
	n.attached.Remove(n)
	n.attached = nil
}

// Tick advances shimmer.
func (n *SkeletonNode) Tick(dt float64) bool {
	if n == nil {
		return false
	}
	if !n.active.Load() || n.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	ph := math.Float64frombits(n.phase.Load()) + dt/ShimmerPeriodSec
	ph = math.Mod(ph, 1)
	if ph < 0 {
		ph++
	}
	n.phase.Store(math.Float64bits(ph))
	n.host.MarkNeedsPaint()
	return true
}

// WantsFrame reports demand.
func (n *SkeletonNode) WantsFrame() bool { return n != nil && n.active.Load() && !n.reduceMotion }

func (n *SkeletonNode) fill() render.RGBA {
	var tok theme.Tokens
	if n != nil && n.override != nil {
		tok = *n.override
	} else if n != nil && n.provider != nil {
		tok = n.provider.Current()
	} else {
		tok = theme.Default.Current()
	}
	c := tok.ColorFillSecondary
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func (n *SkeletonNode) paint(pc *rendering.PaintContext, size rendering.Size) {
	if n == nil || pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	S := n.loadSnapshot()
	if S.HasChild {
		return
	}
	fill := S.Fill
	rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, BlockRadius, fill.R, fill.G, fill.B, fill.A)
	if n.active.Load() && !S.ReduceMotion {
		hw := 40.0
		hx := -hw + (math.Float64frombits(n.phase.Load()))*(size.Width+2*hw)
		rendering.FillRect(pc, hx, 0, hw, size.Height, 1, 1, 1, 0.35)
	}
}
