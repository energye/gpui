package kit

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design 6 Skeleton geometry (controlHeight=32 baseline).
// titleHeight / paragraphLiHeight = controlHeight/2; blockRadius = borderRadiusSM;
// button width = controlHeight*2; input width = controlHeight*5;
// image = (controlHeight*1.5)*2.
const (
	DefaultSkeletonTitleH       = 16.0 // controlHeight / 2
	DefaultSkeletonParagraphH   = 16.0 // controlHeight / 2
	DefaultSkeletonAvatarSize   = 40.0 // controlHeightLG
	DefaultSkeletonAvatarSizeMD = 32.0 // controlHeight
	DefaultSkeletonAvatarSizeSM = 24.0 // controlHeightSM
	DefaultSkeletonGap          = 8.0  // controlHeightXS ≈ row gap
	DefaultSkeletonItemGap      = 16.0 // paddingInlineEnd of header
	DefaultSkeletonLineRadius   = 4.0  // borderRadiusSM / blockRadius
	DefaultSkeletonRoundRadius  = 999.0
	DefaultSkeletonButtonW      = 64.0  // controlHeight * 2
	DefaultSkeletonInputW       = 160.0 // controlHeight * 5
	DefaultSkeletonImageW       = 96.0  // imageSizeBase*2
	DefaultSkeletonImageH       = 96.0
	DefaultSkeletonNodeW        = 96.0
	DefaultSkeletonNodeH        = 96.0
	DefaultSkeletonBodyW        = 240.0
	// Title width ratios (antd getTitleBasicProps).
	DefaultSkeletonTitleRatioAvatar  = 0.50
	DefaultSkeletonTitleRatioCompact = 0.38
	DefaultSkeletonTitleRatioSolo    = 1.00
	// Last paragraph row width when rows≥3 or antd basic width='61%'.
	DefaultSkeletonParagraphLastRatio = 0.61
	DefaultSkeletonBandWidth          = 0.45
	DefaultSkeletonPhaseSpeed         = 1.45 // ~1.4s motion
)

// SkeletonSize is the shared size ladder for Skeleton.Avatar / Button / Input.
type SkeletonSize int

const (
	SkeletonSmall SkeletonSize = iota
	SkeletonMiddle
	SkeletonLarge
)

// SkeletonAvatarShape is the avatar placeholder shape.
type SkeletonAvatarShape int

const (
	SkeletonAvatarCircle SkeletonAvatarShape = iota
	SkeletonAvatarSquare
)

// SkeletonElementShape is the button-like placeholder shape.
type SkeletonElementShape int

const (
	SkeletonElementDefault SkeletonElementShape = iota
	SkeletonElementCircle
	SkeletonElementRound
	SkeletonElementSquare
)

// SkeletonClassNames holds shallow semantic class tags.
type SkeletonClassNames struct {
	Root      string
	Header    string
	Section   string
	Avatar    string
	Title     string
	Paragraph string
}

// SkeletonStyles holds shallow semantic style overrides.
type SkeletonStyles struct {
	Root      Style
	Header    Style
	Section   Style
	Avatar    Style
	Title     Style
	Paragraph Style
}

type skeletonHost struct {
	primitive.RepaintBoundary
	sk *Skeleton
}

func (h *skeletonHost) TypeID() string { return "kit.Skeleton" }

func (h *skeletonHost) OnMount() {
	if h == nil || h.sk == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.sk.life.attach(t, h.sk, h.sk.Loading && h.sk.Active)
	}
}

func (h *skeletonHost) OnUnmount() {
	if h != nil && h.sk != nil {
		h.sk.life.unmount()
	}
}

type skeletonPlate struct {
	core.NodeBase

	width         float64
	height        float64
	ratio         float64
	defaultWidth  float64
	defaultHeight float64
	expand        bool
	radius        float64
	active        bool
	style         Style
	className     string

	sk    *Skeleton
	phase func() float64
	theme func() *core.Theme
	child core.Node
	paint *primitive.PainterNode
}

func (p *skeletonPlate) TypeID() string { return "kit.SkeletonPlate" }

func (p *skeletonPlate) Layout(c core.Constraints) core.Size {
	if p == nil {
		return core.Size{}
	}
	w := p.width
	if w <= 0 {
		switch {
		case p.expand && c.HasBoundedWidth():
			w = c.MaxWidth
		case p.ratio > 0 && c.HasBoundedWidth():
			w = c.MaxWidth * p.ratio
		case p.defaultWidth > 0:
			w = p.defaultWidth
		default:
			w = 64
		}
	}
	h := p.height
	if h <= 0 {
		if p.defaultHeight > 0 {
			h = p.defaultHeight
		} else {
			h = 16
		}
	}
	if p.paint != nil {
		p.paint.Width = w
		p.paint.Height = h
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	if p.paint != nil {
		p.paint.Width = out.Width
		p.paint.Height = out.Height
		_ = p.paint.Layout(core.Tight(out.Width, out.Height))
	}
	p.SetSize(out)
	return out
}

func (p *skeletonPlate) Paint(pc *core.PaintContext) {
	if p == nil || p.paint == nil {
		return
	}
	p.paint.Paint(pc)
}

func (p *skeletonPlate) HitTest(pt core.Point) core.Node { return p.DefaultHitTest(pt) }

// Skeleton is the Ant-style loading placeholder.
//
// Root (RepaintBoundary)
//
//	└─ shell (Decorated)
//	     └─ avatar? + section(column: title? + paragraph?)
//
// loading=false swaps shell content to children.
type Skeleton struct {
	Root *skeletonHost

	shell   *primitive.Decorated
	body    *primitive.Flex
	content core.Node

	Loading   bool
	Active    bool
	Avatar    bool
	Title     bool
	Paragraph bool
	// ParagraphRows <= 0 uses Ant default (3 when avatar=false,title=true; else 2).
	ParagraphRows int
	Round         bool

	AvatarShape SkeletonAvatarShape
	AvatarSize  SkeletonSize
	TitleWidth  float64
	// ParagraphWidths applies to each row; the last row falls back to default
	// width if the slice is shorter than ParagraphRows.
	ParagraphWidths []float64

	Style          Style
	HeaderStyle    Style
	SectionStyle   Style
	AvatarStyle    Style
	TitleStyle     Style
	ParagraphStyle Style
	ClassNames     SkeletonClassNames
	Styles         SkeletonStyles

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme

	phase float64
	life  tickerLifecycle
}

// NewSkeleton creates a loading Skeleton with Ant-like defaults.
func NewSkeleton() *Skeleton {
	s := &Skeleton{
		Loading:       true,
		Title:         true,
		Paragraph:     true,
		Active:        false,
		AvatarShape:   SkeletonAvatarCircle,
		AvatarSize:    SkeletonLarge,
		ParagraphRows: 0,
	}
	s.rebuild()
	return s
}

// Node returns the root.
func (s *Skeleton) Node() core.Node {
	if s == nil {
		return nil
	}
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// ChromeNode returns the visible shell.
func (s *Skeleton) ChromeNode() core.Node { return s.Node() }

// SetLoading toggles skeleton vs children.
func (s *Skeleton) SetLoading(v bool) {
	if s == nil {
		return
	}
	if s.Loading == v {
		return
	}
	s.Loading = v
	if s.Root != nil && s.Root.Tree() != nil {
		s.life.setActive(s.Loading && s.Active)
	}
	s.rebuild()
}

// SetActive toggles animation.
func (s *Skeleton) SetActive(v bool) {
	if s == nil {
		return
	}
	if s.Active == v {
		return
	}
	s.Active = v
	if s.Root != nil && s.Root.Tree() != nil {
		s.life.setActive(s.Loading && s.Active)
	}
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
}

// SetAvatar toggles avatar placeholder.
func (s *Skeleton) SetAvatar(v bool) {
	if s == nil || s.Avatar == v {
		return
	}
	s.Avatar = v
	s.rebuild()
}

// SetTitle toggles title placeholder.
func (s *Skeleton) SetTitle(v bool) {
	if s == nil || s.Title == v {
		return
	}
	s.Title = v
	s.rebuild()
}

// SetParagraph toggles paragraph placeholder.
func (s *Skeleton) SetParagraph(v bool) {
	if s == nil {
		return
	}
	if s.Paragraph == v {
		return
	}
	s.Paragraph = v
	s.rebuild()
}

// SetParagraphRows changes the paragraph row count.
func (s *Skeleton) SetParagraphRows(n int) {
	if s == nil {
		return
	}
	if s.ParagraphRows == n {
		return
	}
	s.ParagraphRows = n
	s.rebuild()
}

// SetRows is a compatibility alias for SetParagraphRows.
func (s *Skeleton) SetRows(n int) { s.SetParagraphRows(n) }

// SetRound toggles round bars.
func (s *Skeleton) SetRound(v bool) {
	if s == nil || s.Round == v {
		return
	}
	s.Round = v
	s.rebuild()
}

// SetAvatarShape updates avatar shape.
func (s *Skeleton) SetAvatarShape(sh SkeletonAvatarShape) {
	if s == nil || s.AvatarShape == sh {
		return
	}
	s.AvatarShape = sh
	s.rebuild()
}

// SetAvatarSize updates avatar size ladder.
func (s *Skeleton) SetAvatarSize(sz SkeletonSize) {
	if s == nil || s.AvatarSize == sz {
		return
	}
	s.AvatarSize = sz
	s.rebuild()
}

// SetTitleWidth updates the title bar width.
func (s *Skeleton) SetTitleWidth(w float64) {
	if s == nil || s.TitleWidth == w {
		return
	}
	s.TitleWidth = w
	s.rebuild()
}

// SetParagraphWidths updates paragraph widths.
func (s *Skeleton) SetParagraphWidths(widths ...float64) {
	if s == nil {
		return
	}
	if len(widths) == 0 {
		s.ParagraphWidths = nil
		s.rebuild()
		return
	}
	s.ParagraphWidths = append(s.ParagraphWidths[:0], widths...)
	s.rebuild()
}

// SetContent updates the non-loading body.
func (s *Skeleton) SetContent(n core.Node) {
	if s == nil {
		return
	}
	s.content = n
	s.rebuild()
}

// SetTheme overrides the control theme.
func (s *Skeleton) SetTheme(th *core.Theme) {
	if s == nil || s.Theme == th {
		return
	}
	s.Theme = th
	s.rebuild()
}

// SetStyle updates shell style.
func (s *Skeleton) SetStyle(st Style) {
	if s == nil {
		return
	}
	s.Style = st
	s.rebuild()
}

// SetHeaderStyle updates the semantic header wrapper style.
func (s *Skeleton) SetHeaderStyle(st Style) {
	if s == nil {
		return
	}
	s.HeaderStyle = st
	s.rebuild()
}

// SetSectionStyle updates the semantic section wrapper style.
func (s *Skeleton) SetSectionStyle(st Style) {
	if s == nil {
		return
	}
	s.SectionStyle = st
	s.rebuild()
}

// SetAvatarStyle updates the avatar placeholder style.
func (s *Skeleton) SetAvatarStyle(st Style) {
	if s == nil {
		return
	}
	s.AvatarStyle = st
	s.rebuild()
}

// SetTitleStyle updates the title placeholder style.
func (s *Skeleton) SetTitleStyle(st Style) {
	if s == nil {
		return
	}
	s.TitleStyle = st
	s.rebuild()
}

// SetParagraphStyle updates the paragraph placeholder style.
func (s *Skeleton) SetParagraphStyle(st Style) {
	if s == nil {
		return
	}
	s.ParagraphStyle = st
	s.rebuild()
}

// SetClassNames updates shallow semantic hooks.
func (s *Skeleton) SetClassNames(cn SkeletonClassNames) {
	if s == nil {
		return
	}
	s.ClassNames = cn
	s.rebuild()
}

// SetStyles updates shallow semantic style hooks.
func (s *Skeleton) SetStyles(st SkeletonStyles) {
	if s == nil {
		return
	}
	s.Styles = st
	s.rebuild()
}

// AttachTicker registers the active ticker.
func (s *Skeleton) AttachTicker(t *core.Tree) {
	if s != nil {
		s.life.attach(t, s, s.Loading && s.Active)
	}
}

// Tick advances shimmer.
func (s *Skeleton) Tick(dt float64) bool {
	if s == nil || !s.Loading || !s.Active {
		return false
	}
	if !skeletonCanTick(&s.life, s.Root) {
		return false
	}
	s.phase += dt * DefaultSkeletonPhaseSpeed
	if s.phase > 1 {
		s.phase -= math.Floor(s.phase)
	}
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
	return true
}

func skeletonCanTick(l *tickerLifecycle, n core.Node) bool {
	var t *core.Tree
	if n != nil {
		t = n.Base().Tree()
	}
	if l != nil && !l.stillMounted(t) {
		return false
	}
	if t != nil && t.Clock().ReduceMotion {
		return false
	}
	return true
}

func (s *Skeleton) theme() *core.Theme {
	var n core.Node
	if s != nil && s.Root != nil {
		n = s.Root
	}
	return themeOf(s.Theme, n)
}

func (s *Skeleton) rebuild() {
	if s == nil {
		return
	}
	th := s.theme()

	if s.Root == nil {
		s.Root = &skeletonHost{sk: s}
		s.Root.Init(s.Root)
		s.Root.Hit = core.HitDefer
		s.Root.SetRepaintBoundary(true)
	}
	s.Root.sk = s
	s.Root.ClearChildren()

	s.shell = primitive.NewDecorated()
	s.shell.Hit = core.HitDefer
	s.shell.ExpandWidth = true
	s.shell.SetCenterContent(false)
	s.applyShellStyle(th)
	if s.ClassNames.Root != "" {
		s.shell.Base().Key = s.ClassNames.Root
	}

	if s.Loading {
		body := s.buildLoadingBody(th)
		if body != nil {
			s.shell.AddChild(body)
		}
	} else if s.content != nil {
		s.shell.AddChild(s.content)
	}

	s.Root.AddChild(s.shell)
	s.Root.SetThemeHook(func(*core.Theme) { s.rebuild() })
	s.applyTickerState()
	s.Root.MarkNeedsPaint()
}

func (s *Skeleton) applyTickerState() {
	if s == nil || s.Root == nil {
		return
	}
	if s.Root.Tree() != nil {
		s.life.setActive(s.Loading && s.Active)
	}
}

func (s *Skeleton) applyShellStyle(th *core.Theme) {
	if s == nil || s.shell == nil {
		return
	}
	st := mergeSkeletonStyle(s.Style, s.Styles.Root)
	if st.hasBG() {
		s.shell.Background = st.Background
	} else {
		s.shell.Background = render.RGBA{}
	}
	if st.hasBorder() {
		s.shell.BorderColor = st.Border
		s.shell.BorderWidth = 1
	} else {
		s.shell.BorderWidth = 0
	}
	if st.hasRadius() {
		s.shell.Radius = st.Radius
	} else {
		s.shell.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	}
	if st.Width > 0 {
		s.shell.Width = st.Width
	}
	if st.Height > 0 {
		s.shell.Height = st.Height
	}
	if st.hasText() {
		s.shell.Base().Label = ""
	}
}

func (s *Skeleton) buildLoadingBody(th *core.Theme) core.Node {
	if s == nil {
		return nil
	}
	col := primitive.Column()
	col.Gap = DefaultSkeletonGap
	col.CrossAlign = core.CrossStart
	col.MainAlign = core.MainStart
	if s.ClassNames.Section != "" {
		col.Base().Key = s.ClassNames.Section
	}

	hasTitle := s.Title
	hasParagraph := s.Paragraph
	rows := s.ParagraphRows
	if rows < 1 {
		if !s.Avatar && hasTitle {
			rows = 3
		} else {
			rows = 2
		}
	}

	// Avatar row.
	if s.Avatar {
		row := primitive.Row()
		row.Gap = DefaultSkeletonItemGap
		row.CrossAlign = core.CrossStart
		row.MainAlign = core.MainStart
		if s.ClassNames.Header != "" {
			row.Base().Key = s.ClassNames.Header
		}

		avatar := s.newAvatarPlate(th)
		row.AddChild(avatar)

		section := primitive.Column()
		section.Gap = DefaultSkeletonGap
		section.CrossAlign = core.CrossStart
		section.MainAlign = core.MainStart
		if s.ClassNames.Section != "" {
			section.Base().Key = s.ClassNames.Section
		}

		if hasTitle {
			section.AddChild(s.newTitlePlate(th, true, hasParagraph))
		}
		if hasParagraph {
			section.AddChild(s.newParagraphPlate(th, rows, hasTitle))
		}

		sectionNode := core.Node(section)
		if wrapped := wrapSkeletonSemantic(section, s.ClassNames.Section, mergeSkeletonStyle(s.SectionStyle, s.Styles.Section)); wrapped != nil {
			sectionNode = wrapped
		}
		row.AddChild(sectionNode)
		if wrapped := wrapSkeletonSemantic(row, s.ClassNames.Header, mergeSkeletonStyle(s.HeaderStyle, s.Styles.Header)); wrapped != nil {
			col.AddChild(wrapped)
		} else {
			col.AddChild(row)
		}
		return col
	}

	if hasTitle {
		col.AddChild(s.newTitlePlate(th, false, hasParagraph))
	}
	if hasParagraph {
		col.AddChild(s.newParagraphPlate(th, rows, hasTitle))
	}
	return col
}

func (s *Skeleton) newAvatarPlate(th *core.Theme) core.Node {
	size := s.avatarPx(th)
	radius := size / 2
	if s.AvatarShape == SkeletonAvatarSquare {
		radius = th.SizeOr(core.TokenBorderRadius, 6)
	}
	p := newSkeletonPlate(s, size, size, 0, size, size, false, radius, mergeSkeletonStyle(s.AvatarStyle, s.Styles.Avatar), s.ClassNames.Avatar)
	p.defaultWidth = size
	p.defaultHeight = size
	return p
}

func (s *Skeleton) newTitlePlate(th *core.Theme, withAvatar, withParagraph bool) core.Node {
	st := mergeSkeletonStyle(s.TitleStyle, s.Styles.Title)
	radius := DefaultSkeletonLineRadius
	if s.Round {
		radius = DefaultSkeletonRoundRadius
	}
	h := DefaultSkeletonTitleH
	if st.Height > 0 {
		h = st.Height
	}

	// Explicit width wins; else antd ratio (38% / 50% / 100%).
	w := s.TitleWidth
	if w <= 0 && st.Width > 0 {
		w = st.Width
	}
	ratio := 0.0
	expand := false
	if w <= 0 {
		switch {
		case withAvatar && withParagraph:
			ratio = DefaultSkeletonTitleRatioAvatar
		case !withAvatar && withParagraph:
			ratio = DefaultSkeletonTitleRatioCompact
		default:
			ratio = DefaultSkeletonTitleRatioSolo
			expand = true
		}
		w = 0
	}
	defW := DefaultSkeletonBodyW
	if ratio > 0 {
		defW = DefaultSkeletonBodyW * ratio
	}
	return newSkeletonPlate(s, w, h, ratio, defW, h, expand, radius, st, s.ClassNames.Title)
}

func (s *Skeleton) newParagraphPlate(th *core.Theme, rows int, hasTitle bool) core.Node {
	st := mergeSkeletonStyle(s.ParagraphStyle, s.Styles.Paragraph)
	col := primitive.Column()
	col.Gap = DefaultSkeletonGap
	col.CrossAlign = core.CrossStart
	col.MainAlign = core.MainStart
	if s.ClassNames.Paragraph != "" {
		col.Base().Key = s.ClassNames.Paragraph
	}
	// antd: last row width='61%' when !avatar||!title, and CSS last:not(:nth-child(2)).
	useLastRatio := len(s.ParagraphWidths) == 0 && (rows >= 3 || !s.Avatar || !hasTitle)
	for i := 0; i < rows; i++ {
		radius := DefaultSkeletonLineRadius
		if s.Round {
			radius = DefaultSkeletonRoundRadius
		}
		h := DefaultSkeletonParagraphH
		if st.Height > 0 {
			h = st.Height
		}

		w := 0.0
		ratio := 0.0
		expand := true
		defW := DefaultSkeletonBodyW
		if len(s.ParagraphWidths) > 0 {
			if i < len(s.ParagraphWidths) {
				w = s.ParagraphWidths[i]
			} else {
				w = s.ParagraphWidths[len(s.ParagraphWidths)-1]
			}
			if w > 0 {
				expand = false
				defW = w
			}
		} else if i == rows-1 && useLastRatio {
			ratio = DefaultSkeletonParagraphLastRatio
			expand = false
			defW = DefaultSkeletonBodyW * ratio
		} else if st.Width > 0 {
			w = st.Width
			expand = false
			defW = w
		}
		col.AddChild(newSkeletonPlate(s, w, h, ratio, defW, h, expand, radius, st, ""))
	}
	return col
}

func wrapSkeletonSemantic(n core.Node, key string, st Style) core.Node {
	if n == nil || (!st.hasBG() && !st.hasBorder() && !st.hasRadius() && st.Width <= 0 && st.Height <= 0) {
		return nil
	}
	d := primitive.NewDecorated(n)
	d.Hit = core.HitDefer
	if key != "" {
		d.Base().Key = key
	}
	if st.hasBG() {
		d.Background = st.Background
	}
	if st.hasBorder() {
		d.BorderColor = st.Border
		d.BorderWidth = 1
	}
	if st.hasRadius() {
		d.Radius = st.Radius
	}
	if st.Width > 0 {
		d.Width = st.Width
	}
	if st.Height > 0 {
		d.Height = st.Height
	}
	return d
}

func (s *Skeleton) avatarPx(th *core.Theme) float64 {
	switch s.AvatarSize {
	case SkeletonSmall:
		return DefaultSkeletonAvatarSizeSM
	case SkeletonLarge:
		return DefaultSkeletonAvatarSize
	default:
		if v := th.SizeOr(core.TokenControlHeight, 0); v > 0 {
			return v
		}
		return DefaultSkeletonAvatarSizeMD
	}
}

func mergeSkeletonStyle(a, b Style) Style {
	out := a
	if b.hasBG() {
		out.Background = b.Background
	}
	if b.hasBGHover() {
		out.BackgroundHover = b.BackgroundHover
	}
	if b.hasBGActive() {
		out.BackgroundActive = b.BackgroundActive
	}
	if b.hasBorder() {
		out.Border = b.Border
	}
	if b.hasText() {
		out.Text = b.Text
	}
	if b.FontSize > 0 {
		out.FontSize = b.FontSize
	}
	if b.Height > 0 {
		out.Height = b.Height
	}
	if b.Width > 0 {
		out.Width = b.Width
	}
	if b.hasRadius() {
		out.Radius = b.Radius
		out.ForceRadius = b.ForceRadius
	}
	if b.Face != nil {
		out.Face = b.Face
	}
	return out
}

func newSkeletonPlate(sk *Skeleton, w, h, ratio, defaultW, defaultH float64, expand bool, radius float64, st Style, key string) *skeletonPlate {
	p := &skeletonPlate{
		width:         w,
		height:        h,
		ratio:         ratio,
		defaultWidth:  defaultW,
		defaultHeight: defaultH,
		expand:        expand,
		radius:        radius,
		active:        sk != nil && sk.Active,
		style:         st,
		className:     key,
		sk:            sk,
	}
	p.Init(p)
	p.Hit = core.HitDefer
	// Semantic classNames attach to the plate itself so tree walkers see them
	// (paint helper is not a child of the plate).
	if key != "" {
		p.Base().Key = key
	}
	p.paint = primitive.NewPainterNode(func(pc *core.PaintContext, size core.Size) {
		if pc == nil {
			return
		}
		th := DefaultTheme()
		if sk != nil {
			th = sk.theme()
		} else if p.theme != nil {
			th = p.theme()
		}
		phase := 0.0
		active := p.active
		if sk != nil {
			phase = sk.phase
			active = sk.Loading && sk.Active
		} else if p.phase != nil {
			phase = p.phase()
		}
		r := p.radius
		if p.style.hasRadius() {
			r = p.style.Radius
		}
		// antd gradientFromColor ≈ colorFillContent / colorFillSecondary
		base := th.Color(core.TokenColorFillSecondary)
		if p.style.hasBG() {
			base = p.style.Background
		}
		if base.A <= 0 {
			base = render.RGBA{R: 0.89, G: 0.90, B: 0.92, A: 1}
		}
		paintSkeletonPlate(pc, size, th, phase, active, r, base)
	})
	return p
}

func paintSkeletonPlate(pc *core.PaintContext, size core.Size, th *core.Theme, phase float64, active bool, radius float64, base render.RGBA) {
	if pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	if radius < 0 {
		radius = 0
	}
	if radius == 0 && size.Height > 0 {
		radius = 2
	}
	pc.FillLocalRoundRect(0, 0, size.Width, size.Height, radius, base)
	if !active {
		return
	}
	band := th.Color(core.TokenColorBgContainer)
	if band.A <= 0 {
		band = render.RGBA{R: 1, G: 1, B: 1, A: 0.18}
	} else {
		band.A *= 0.22
	}
	bandW := size.Width * DefaultSkeletonBandWidth
	if bandW < 12 {
		bandW = 12
	}
	bandX := (size.Width+bandW)*phase - bandW
	pc.PushClipLocal(0, 0, size.Width, size.Height)
	pc.FillLocalRoundRect(bandX, 0, bandW, size.Height, radius, band)
	pc.Pop()
}

// SkeletonAvatar is the standalone avatar placeholder.
type SkeletonAvatar struct {
	Root   *skeletonPlate
	Size   SkeletonSize
	Shape  SkeletonAvatarShape
	Active bool
	Style  Style
	Theme  *core.Theme
	phase  float64
	life   tickerLifecycle
}

// NewSkeletonAvatar creates the avatar placeholder.
func NewSkeletonAvatar() *SkeletonAvatar {
	a := &SkeletonAvatar{Size: SkeletonMiddle, Shape: SkeletonAvatarCircle}
	a.rebuild()
	return a
}

func (a *SkeletonAvatar) Node() core.Node {
	if a == nil {
		return nil
	}
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

func (a *SkeletonAvatar) SetSize(sz SkeletonSize) {
	if a == nil || a.Size == sz {
		return
	}
	a.Size = sz
	a.rebuild()
}

func (a *SkeletonAvatar) SetShape(sh SkeletonAvatarShape) {
	if a == nil || a.Shape == sh {
		return
	}
	a.Shape = sh
	a.rebuild()
}

func (a *SkeletonAvatar) SetActive(v bool) {
	if a == nil || a.Active == v {
		return
	}
	a.Active = v
	if a.Root != nil && a.Root.Tree() != nil {
		a.life.setActive(v)
	}
	if a.Root != nil {
		a.Root.MarkNeedsPaint()
	}
}

func (a *SkeletonAvatar) SetStyle(st Style) {
	if a == nil {
		return
	}
	a.Style = st
	a.rebuild()
}

func (a *SkeletonAvatar) SetTheme(th *core.Theme) {
	if a == nil || a.Theme == th {
		return
	}
	a.Theme = th
	a.rebuild()
}

func (a *SkeletonAvatar) AttachTicker(t *core.Tree) {
	if a != nil {
		a.life.attach(t, a, a.Active)
	}
}

func (a *SkeletonAvatar) Tick(dt float64) bool {
	if a == nil || !a.Active {
		return false
	}
	if !skeletonCanTick(&a.life, a.Root) {
		return false
	}
	a.phase += dt * DefaultSkeletonPhaseSpeed
	if a.phase > 1 {
		a.phase -= math.Floor(a.phase)
	}
	if a.Root != nil {
		a.Root.MarkNeedsPaint()
	}
	return true
}

func (a *SkeletonAvatar) theme() *core.Theme {
	return themeOf(a.Theme, nil)
}

func (a *SkeletonAvatar) rebuild() {
	if a == nil {
		return
	}
	if a.Root == nil {
		a.Root = newSkeletonPlate(nil, 0, 0, 0, 0, 0, false, 0, Style{}, "")
	}
	size := a.avatarPx(a.theme())
	if a.Root != nil {
		a.Root.width = size
		a.Root.height = size
		a.Root.defaultWidth = size
		a.Root.defaultHeight = size
		a.Root.active = a.Active
		a.Root.style = a.Style
		a.Root.phase = func() float64 { return a.phase }
		a.Root.theme = func() *core.Theme { return a.theme() }
		a.Root.paint.Width = size
		a.Root.paint.Height = size
		a.Root.paint.Padding = primitive.EdgeInsets{}
		a.Root.paint.Base().Key = ""
		if a.Shape == SkeletonAvatarSquare {
			a.Root.radius = a.theme().SizeOr(core.TokenBorderRadius, 6)
		} else {
			a.Root.radius = size / 2
		}
	}
}

func (a *SkeletonAvatar) avatarPx(th *core.Theme) float64 {
	switch a.Size {
	case SkeletonSmall:
		return DefaultSkeletonAvatarSizeSM
	case SkeletonLarge:
		return DefaultSkeletonAvatarSize
	default:
		if v := th.SizeOr(core.TokenControlHeight, 0); v > 0 {
			return v
		}
		return DefaultSkeletonAvatarSizeMD
	}
}

// SkeletonButton is the button placeholder.
type SkeletonButton struct {
	Root   *skeletonPlate
	Size   SkeletonSize
	Shape  SkeletonElementShape
	Block  bool
	Active bool
	Style  Style
	Theme  *core.Theme
	phase  float64
	life   tickerLifecycle
}

// NewSkeletonButton creates the button placeholder.
func NewSkeletonButton() *SkeletonButton {
	b := &SkeletonButton{Size: SkeletonMiddle, Shape: SkeletonElementDefault}
	b.rebuild()
	return b
}

func (b *SkeletonButton) Node() core.Node {
	if b == nil {
		return nil
	}
	if b.Root == nil {
		b.rebuild()
	}
	return b.Root
}

func (b *SkeletonButton) SetSize(sz SkeletonSize) {
	if b == nil || b.Size == sz {
		return
	}
	b.Size = sz
	b.rebuild()
}

func (b *SkeletonButton) SetShape(sh SkeletonElementShape) {
	if b == nil || b.Shape == sh {
		return
	}
	b.Shape = sh
	b.rebuild()
}

func (b *SkeletonButton) SetBlock(v bool) {
	if b == nil || b.Block == v {
		return
	}
	b.Block = v
	b.rebuild()
}

func (b *SkeletonButton) SetActive(v bool) {
	if b == nil || b.Active == v {
		return
	}
	b.Active = v
	if b.Root != nil && b.Root.Tree() != nil {
		b.life.setActive(v)
	}
	if b.Root != nil {
		b.Root.MarkNeedsPaint()
	}
}

func (b *SkeletonButton) SetStyle(st Style) {
	if b == nil {
		return
	}
	b.Style = st
	b.rebuild()
}

func (b *SkeletonButton) SetTheme(th *core.Theme) {
	if b == nil || b.Theme == th {
		return
	}
	b.Theme = th
	b.rebuild()
}

func (b *SkeletonButton) AttachTicker(t *core.Tree) {
	if b != nil {
		b.life.attach(t, b, b.Active)
	}
}

func (b *SkeletonButton) Tick(dt float64) bool {
	if b == nil || !b.Active {
		return false
	}
	if !skeletonCanTick(&b.life, b.Root) {
		return false
	}
	b.phase += dt * DefaultSkeletonPhaseSpeed
	if b.phase > 1 {
		b.phase -= math.Floor(b.phase)
	}
	if b.Root != nil {
		b.Root.MarkNeedsPaint()
	}
	return true
}

func (b *SkeletonButton) theme() *core.Theme { return themeOf(b.Theme, nil) }

func (b *SkeletonButton) rebuild() {
	if b == nil {
		return
	}
	w, h := b.sizePx(b.theme())
	if b.Shape == SkeletonElementCircle || b.Shape == SkeletonElementSquare {
		w = h
	}
	if b.Block {
		w = 0
	}
	radius := DefaultSkeletonLineRadius
	switch b.Shape {
	case SkeletonElementCircle:
		radius = h / 2
	case SkeletonElementRound:
		radius = DefaultSkeletonRoundRadius
	case SkeletonElementSquare:
		radius = 0
	}
	if b.Root == nil {
		b.Root = newSkeletonPlate(nil, w, h, 0, DefaultSkeletonButtonW, h, b.Block, radius, b.Style, "")
	}
	b.Root.width = w
	b.Root.height = h
	b.Root.defaultWidth = DefaultSkeletonButtonW
	b.Root.defaultHeight = h
	b.Root.expand = b.Block
	b.Root.radius = radius
	b.Root.active = b.Active
	b.Root.style = b.Style
	b.Root.phase = func() float64 { return b.phase }
	b.Root.theme = func() *core.Theme { return b.theme() }
	b.Root.paint.Width = w
	b.Root.paint.Height = h
	b.Root.paint.Padding = primitive.EdgeInsets{}
}

func (b *SkeletonButton) sizePx(th *core.Theme) (float64, float64) {
	// antd: width = controlHeight * 2, height = controlHeight
	h := th.SizeOr(core.TokenControlHeight, 32)
	switch b.Size {
	case SkeletonSmall:
		h = th.SizeOr(core.TokenControlHeightSM, 24)
	case SkeletonLarge:
		h = th.SizeOr(core.TokenControlHeightLG, 40)
	}
	return h * 2, h
}

// SkeletonInput is the input placeholder.
type SkeletonInput struct {
	Root   *skeletonPlate
	Size   SkeletonSize
	Block  bool
	Active bool
	Style  Style
	Theme  *core.Theme
	phase  float64
	life   tickerLifecycle
}

// NewSkeletonInput creates the input placeholder.
func NewSkeletonInput() *SkeletonInput {
	i := &SkeletonInput{Size: SkeletonMiddle}
	i.rebuild()
	return i
}

func (i *SkeletonInput) Node() core.Node {
	if i == nil {
		return nil
	}
	if i.Root == nil {
		i.rebuild()
	}
	return i.Root
}

func (i *SkeletonInput) SetSize(sz SkeletonSize) {
	if i == nil || i.Size == sz {
		return
	}
	i.Size = sz
	i.rebuild()
}

func (i *SkeletonInput) SetBlock(v bool) {
	if i == nil || i.Block == v {
		return
	}
	i.Block = v
	i.rebuild()
}

func (i *SkeletonInput) SetActive(v bool) {
	if i == nil || i.Active == v {
		return
	}
	i.Active = v
	if i.Root != nil && i.Root.Tree() != nil {
		i.life.setActive(v)
	}
	if i.Root != nil {
		i.Root.MarkNeedsPaint()
	}
}

func (i *SkeletonInput) SetStyle(st Style) {
	if i == nil {
		return
	}
	i.Style = st
	i.rebuild()
}

func (i *SkeletonInput) SetTheme(th *core.Theme) {
	if i == nil || i.Theme == th {
		return
	}
	i.Theme = th
	i.rebuild()
}

func (i *SkeletonInput) AttachTicker(t *core.Tree) {
	if i != nil {
		i.life.attach(t, i, i.Active)
	}
}

func (i *SkeletonInput) Tick(dt float64) bool {
	if i == nil || !i.Active {
		return false
	}
	if !skeletonCanTick(&i.life, i.Root) {
		return false
	}
	i.phase += dt * DefaultSkeletonPhaseSpeed
	if i.phase > 1 {
		i.phase -= math.Floor(i.phase)
	}
	if i.Root != nil {
		i.Root.MarkNeedsPaint()
	}
	return true
}

func (i *SkeletonInput) theme() *core.Theme { return themeOf(i.Theme, nil) }

func (i *SkeletonInput) rebuild() {
	if i == nil {
		return
	}
	w, h := i.sizePx(i.theme())
	if i.Block {
		w = 0
	}
	if i.Root == nil {
		i.Root = newSkeletonPlate(nil, w, h, 0, DefaultSkeletonInputW, h, i.Block, DefaultSkeletonLineRadius, i.Style, "")
	}
	i.Root.width = w
	i.Root.height = h
	i.Root.defaultWidth = DefaultSkeletonInputW
	i.Root.defaultHeight = h
	i.Root.expand = i.Block
	i.Root.radius = DefaultSkeletonLineRadius
	i.Root.active = i.Active
	i.Root.style = i.Style
	i.Root.phase = func() float64 { return i.phase }
	i.Root.theme = func() *core.Theme { return i.theme() }
	i.Root.paint.Width = w
	i.Root.paint.Height = h
	i.Root.paint.Padding = primitive.EdgeInsets{}
}

func (i *SkeletonInput) sizePx(th *core.Theme) (float64, float64) {
	// antd: width = controlHeight * 5, height = controlHeight
	h := th.SizeOr(core.TokenControlHeight, 32)
	switch i.Size {
	case SkeletonSmall:
		h = th.SizeOr(core.TokenControlHeightSM, 24)
	case SkeletonLarge:
		h = th.SizeOr(core.TokenControlHeightLG, 40)
	}
	return h * 5, h
}

// SkeletonImage is the image placeholder.
type SkeletonImage struct {
	Root   *skeletonPlate
	Active bool
	Style  Style
	Theme  *core.Theme
	phase  float64
	life   tickerLifecycle
}

// NewSkeletonImage creates the image placeholder.
func NewSkeletonImage() *SkeletonImage {
	im := &SkeletonImage{}
	im.rebuild()
	return im
}

func (im *SkeletonImage) Node() core.Node {
	if im == nil {
		return nil
	}
	if im.Root == nil {
		im.rebuild()
	}
	return im.Root
}

func (im *SkeletonImage) SetActive(v bool) {
	if im == nil || im.Active == v {
		return
	}
	im.Active = v
	if im.Root != nil && im.Root.Tree() != nil {
		im.life.setActive(v)
	}
	if im.Root != nil {
		im.Root.MarkNeedsPaint()
	}
}

func (im *SkeletonImage) SetStyle(st Style) {
	if im == nil {
		return
	}
	im.Style = st
	im.rebuild()
}

func (im *SkeletonImage) SetTheme(th *core.Theme) {
	if im == nil || im.Theme == th {
		return
	}
	im.Theme = th
	im.rebuild()
}

func (im *SkeletonImage) AttachTicker(t *core.Tree) {
	if im != nil {
		im.life.attach(t, im, im.Active)
	}
}

func (im *SkeletonImage) Tick(dt float64) bool {
	if im == nil || !im.Active {
		return false
	}
	if !skeletonCanTick(&im.life, im.Root) {
		return false
	}
	im.phase += dt * DefaultSkeletonPhaseSpeed
	if im.phase > 1 {
		im.phase -= math.Floor(im.phase)
	}
	if im.Root != nil {
		im.Root.MarkNeedsPaint()
	}
	return true
}

func (im *SkeletonImage) theme() *core.Theme { return themeOf(im.Theme, nil) }

func (im *SkeletonImage) rebuild() {
	if im == nil {
		return
	}
	w, h := DefaultSkeletonImageW, DefaultSkeletonImageH
	if im.Style.Width > 0 {
		w = im.Style.Width
	}
	if im.Style.Height > 0 {
		h = im.Style.Height
	}
	if im.Root == nil {
		im.Root = newSkeletonPlate(nil, w, h, 0, DefaultSkeletonImageW, DefaultSkeletonImageH, false, DefaultSkeletonLineRadius, im.Style, "")
	}
	im.Root.width = w
	im.Root.height = h
	im.Root.defaultWidth = DefaultSkeletonImageW
	im.Root.defaultHeight = DefaultSkeletonImageH
	im.Root.expand = false
	im.Root.radius = DefaultSkeletonLineRadius
	im.Root.active = im.Active
	im.Root.style = im.Style
	im.Root.phase = func() float64 { return im.phase }
	im.Root.theme = func() *core.Theme { return im.theme() }
	im.Root.paint.Width = w
	im.Root.paint.Height = h
	im.Root.paint.Padding = primitive.EdgeInsets{}
}

// SkeletonNode is the custom child placeholder.
type SkeletonNode struct {
	Root   *skeletonPlate
	Child  core.Node
	Active bool
	Style  Style
	Theme  *core.Theme
	phase  float64
	life   tickerLifecycle
}

// NewSkeletonNode creates a custom placeholder around child.
func NewSkeletonNode(child core.Node) *SkeletonNode {
	n := &SkeletonNode{Child: child}
	n.rebuild()
	return n
}

func (n *SkeletonNode) Node() core.Node {
	if n == nil {
		return nil
	}
	if n.Root == nil {
		n.rebuild()
	}
	return n.Root
}

func (n *SkeletonNode) SetChild(child core.Node) {
	if n == nil {
		return
	}
	n.Child = child
	n.rebuild()
}

func (n *SkeletonNode) SetActive(v bool) {
	if n == nil || n.Active == v {
		return
	}
	n.Active = v
	if n.Root != nil && n.Root.Tree() != nil {
		n.life.setActive(v)
	}
	if n.Root != nil {
		n.Root.MarkNeedsPaint()
	}
}

func (n *SkeletonNode) SetStyle(st Style) {
	if n == nil {
		return
	}
	n.Style = st
	n.rebuild()
}

func (n *SkeletonNode) SetTheme(th *core.Theme) {
	if n == nil || n.Theme == th {
		return
	}
	n.Theme = th
	n.rebuild()
}

func (n *SkeletonNode) AttachTicker(t *core.Tree) {
	if n != nil {
		n.life.attach(t, n, n.Active)
	}
}

func (n *SkeletonNode) Tick(dt float64) bool {
	if n == nil || !n.Active {
		return false
	}
	if !skeletonCanTick(&n.life, n.Root) {
		return false
	}
	n.phase += dt * DefaultSkeletonPhaseSpeed
	if n.phase > 1 {
		n.phase -= math.Floor(n.phase)
	}
	if n.Root != nil {
		n.Root.MarkNeedsPaint()
	}
	return true
}

func (n *SkeletonNode) theme() *core.Theme { return themeOf(n.Theme, nil) }

func (n *SkeletonNode) rebuild() {
	if n == nil {
		return
	}
	w, h := DefaultSkeletonNodeW, DefaultSkeletonNodeH
	if n.Style.Width > 0 {
		w = n.Style.Width
	}
	if n.Style.Height > 0 {
		h = n.Style.Height
	}
	if n.Root == nil {
		n.Root = newSkeletonPlate(nil, w, h, 0, DefaultSkeletonNodeW, DefaultSkeletonNodeH, false, DefaultSkeletonLineRadius, n.Style, "")
	}
	n.Root.width = w
	n.Root.height = h
	n.Root.defaultWidth = DefaultSkeletonNodeW
	n.Root.defaultHeight = DefaultSkeletonNodeH
	n.Root.expand = false
	n.Root.radius = DefaultSkeletonLineRadius
	n.Root.active = n.Active
	n.Root.style = n.Style
	n.Root.phase = func() float64 { return n.phase }
	n.Root.theme = func() *core.Theme { return n.theme() }
	n.Root.paint.Width = w
	n.Root.paint.Height = h
	n.Root.paint.Padding = primitive.EdgeInsets{Left: 12, Right: 12, Top: 4, Bottom: 4}
	n.Root.paint.ClearChildren()
	if n.Child != nil {
		n.Root.paint.AddChild(n.Child)
	}
}
