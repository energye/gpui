package kit

import (
	"math"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design QRCode tokens — components/qr-code/style/index.ts + docs/antd/qr-code.md §6.2.
const (
	// DefaultQRCodeSize is API size default (outer box).
	DefaultQRCodeSize = 160.0
	// DefaultQRCodeIconSize is API iconSize default.
	DefaultQRCodeIconSize = 40.0
	// DefaultQRCodePadding is paddingSM when bordered.
	DefaultQRCodePadding = 8.0
	// DefaultQRCodeRadius is borderRadiusLG.
	DefaultQRCodeRadius = 8.0
	// DefaultQRCodeLineWidth is lineWidth.
	DefaultQRCodeLineWidth = 1.0
	// DefaultQRCodeFontSize is status text fontSize.
	DefaultQRCodeFontSize = 14.0
	// DefaultQRCodeFocusRingOutset approximates Ant focus-visible outset.
	DefaultQRCodeFocusRingOutset = 1.5
	// DefaultQRCodeCoverAlpha is colorBgContainer alpha for cover (0.96).
	DefaultQRCodeCoverAlpha = 0.96
	// DefaultQRCodeExpired is locale en_US QRCode.expired.
	DefaultQRCodeExpired = "QR code expired"
	// DefaultQRCodeRefresh is locale en_US QRCode.refresh.
	DefaultQRCodeRefresh = "Refresh"
	// DefaultQRCodeScanned is locale en_US QRCode.scanned.
	DefaultQRCodeScanned = "Scanned"
	// qrcodeSpinRPS is loading ring revolutions per second.
	qrcodeSpinRPS = 0.9
)

// QRCodeType is the render backend label (antd type).
type QRCodeType int

const (
	// QRCodeTypeCanvas is type="canvas" (default).
	QRCodeTypeCanvas QRCodeType = iota
	// QRCodeTypeSVG is type="svg" (desktop still paints modules; export differs P1).
	QRCodeTypeSVG
)

// QRErrorLevel is antd errorLevel.
type QRErrorLevel int

const (
	// QRErrorLevelM is Medium (default).
	QRErrorLevelM QRErrorLevel = iota
	// QRErrorLevelL is Low (~7%).
	QRErrorLevelL
	// QRErrorLevelQ is Quartile (~25%).
	QRErrorLevelQ
	// QRErrorLevelH is High (~30%).
	QRErrorLevelH
)

// QRStatus is antd status.
type QRStatus int

const (
	// QRStatusActive shows modules only.
	QRStatusActive QRStatus = iota
	// QRStatusExpired shows cover + refresh.
	QRStatusExpired
	// QRStatusLoading shows cover + spinner (Ticker).
	QRStatusLoading
	// QRStatusScanned shows cover + scanned label.
	QRStatusScanned
)

// QRCodeClassNames holds shallow semantic class tags (antd classNames).
type QRCodeClassNames struct {
	Root  string
	Cover string
}

// QRStatusRenderInfo is antd StatusRenderInfo (desktop subset).
type QRStatusRenderInfo struct {
	Status    QRStatus // expired | loading | scanned
	Expired   string
	Refresh   string
	Scanned   string
	OnRefresh func()
}

// QRCode is Ant Design QRCode — value → module matrix with status cover.
//
//	qrcodeHost (Decorated, size×size, pad/border/radius)  // OnMount binds Ticker
//	  └─ Stack
//	       modules Painter · icon? · cover?
//
// Product contract: docs/antd/qr-code.md §6 (P0 DoD).
// Encoding: pure-Go skip2/go-qrcode. Icon URL decode is host/P1; use IconNode for P0.
type QRCode struct {
	Root     *qrcodeHost
	stack    *primitive.Stack
	painter  *primitive.PainterNode
	iconHost *primitive.Decorated
	cover    *primitive.Decorated
	spinCV   *primitive.Canvas
	refresh  *Button

	// --- product fields ---
	value string
	typ   QRCodeType
	size  float64 // 0 → DefaultQRCodeSize

	iconSrc  string
	iconNode core.Node
	iconW    float64 // 0 → DefaultQRCodeIconSize
	iconH    float64

	color       render.RGBA
	colorSet    bool
	bgColor     render.RGBA
	bgColorSet  bool
	bordered    bool
	borderedSet bool
	errorLevel  QRErrorLevel
	marginSize  int
	status      QRStatus

	OnRefresh    func()
	StatusRender func(info QRStatusRenderInfo) core.Node

	Style      Style // root
	CoverStyle Style
	ClassNames QRCodeClassNames
	AriaLabel  string
	Face       text.Face
	Theme      *core.Theme

	// encode cache
	modules [][]bool
	moduleN int

	// loading ticker
	spinPhase float64
	life      tickerLifecycle

	// cached L2
	pad        float64
	radius     float64
	lineW      float64
	fgColor    render.RGBA
	resolvedBG render.RGBA
	borderCol  render.RGBA
	coverBG    render.RGBA
}

// NewQRCode creates a QRCode with antd defaults (size=160, status=active, bordered, level=M).
func NewQRCode(value string) *QRCode {
	q := &QRCode{
		value:      value,
		typ:        QRCodeTypeCanvas,
		bordered:   true,
		errorLevel: QRErrorLevelM,
		status:     QRStatusActive,
	}
	q.rebuild()
	return q
}

// Node returns the mount root (stable Decorated when possible).

// ensureBuilt materializes the control tree if missing (#9).
func (q *QRCode) ensureBuilt() {
	if q == nil {
		return
	}
	if q.Root == nil {
		q.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (q *QRCode) structureChange() {
	if q == nil {
		return
	}
	q.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (q *QRCode) chromeChange() {
	if q == nil {
		return
	}
	q.ensureBuilt()
	q.rebuild()
}

func (q *QRCode) Node() core.Node {
	if q == nil {
		return nil
	}
	q.ensureBuilt()
	return q.Root
}

// ChromeNode returns the visual shell.
func (q *QRCode) ChromeNode() core.Node { return q.Node() }

// CoverNode returns the status cover (nil when active).
func (q *QRCode) CoverNode() core.Node {
	if q == nil {
		return nil
	}
	return q.cover
}

// IconHost returns the center icon container (may be nil).
func (q *QRCode) IconHost() core.Node {
	if q == nil {
		return nil
	}
	return q.iconHost
}

// RefreshButton returns the expired refresh button when present.
func (q *QRCode) RefreshButton() *Button {
	if q == nil {
		return nil
	}
	return q.refresh
}

// --- getters ---

// Value returns the encoded payload.
func (q *QRCode) Value() string {
	if q == nil {
		return ""
	}
	return q.value
}

// Size returns the outer box edge length.
func (q *QRCode) Size() float64 {
	if q == nil {
		return DefaultQRCodeSize
	}
	if q.size > 0 {
		return q.size
	}
	if q.Style.Width > 0 {
		return q.Style.Width
	}
	if q.Style.Height > 0 {
		return q.Style.Height
	}
	return DefaultQRCodeSize
}

// Type returns canvas|svg label.
func (q *QRCode) Type() QRCodeType {
	if q == nil {
		return QRCodeTypeCanvas
	}
	return q.typ
}

// Status returns the current status.
func (q *QRCode) Status() QRStatus {
	if q == nil {
		return QRStatusActive
	}
	return q.status
}

// Bordered reports whether the chrome border is on.
func (q *QRCode) Bordered() bool {
	if q == nil {
		return true
	}
	if q.borderedSet {
		return q.bordered
	}
	return true
}

// ErrorLevel returns the recovery level.
func (q *QRCode) ErrorLevel() QRErrorLevel {
	if q == nil {
		return QRErrorLevelM
	}
	return q.errorLevel
}

// MarginSize returns quiet-zone modules.
func (q *QRCode) MarginSize() int {
	if q == nil {
		return 0
	}
	if q.marginSize < 0 {
		return 0
	}
	return q.marginSize
}

// Modules returns the encoded module grid edge (excluding quiet zone paint expansion).
func (q *QRCode) Modules() int {
	if q == nil {
		return 0
	}
	return q.moduleN
}

// HasCover is true when status != active.
func (q *QRCode) HasCover() bool {
	return q != nil && q.status != QRStatusActive
}

// HasIcon is true when icon src or node is set.
func (q *QRCode) HasIcon() bool {
	return q != nil && (q.iconNode != nil || q.iconSrc != "")
}

// Padding returns resolved root padding (0 when borderless).
func (q *QRCode) Padding() float64 {
	if q == nil {
		return DefaultQRCodePadding
	}
	if !q.Bordered() {
		return 0
	}
	if q.pad > 0 {
		return q.pad
	}
	return DefaultQRCodePadding
}

// Radius returns resolved corner radius (0 when borderless).
func (q *QRCode) Radius() float64 {
	if q == nil {
		return DefaultQRCodeRadius
	}
	if !q.Bordered() {
		return 0
	}
	if q.Style.hasRadius() {
		return q.Style.Radius
	}
	if q.radius > 0 {
		return q.radius
	}
	return DefaultQRCodeRadius
}

// LineWidth returns border stroke width (0 when borderless).
func (q *QRCode) LineWidth() float64 {
	if q == nil {
		return DefaultQRCodeLineWidth
	}
	if !q.Bordered() {
		return 0
	}
	if q.lineW > 0 {
		return q.lineW
	}
	return DefaultQRCodeLineWidth
}

// ContentSize is the inner drawable edge (size − 2·padding).
func (q *QRCode) ContentSize() float64 {
	sz := q.Size()
	pad := q.Padding()
	inner := sz - 2*pad
	if inner < 1 {
		return 1
	}
	return inner
}

// IconSize returns resolved icon width/height (square when only one set).
func (q *QRCode) IconSize() (w, h float64) {
	if q == nil {
		return DefaultQRCodeIconSize, DefaultQRCodeIconSize
	}
	w, h = q.iconW, q.iconH
	if w <= 0 {
		w = DefaultQRCodeIconSize
	}
	if h <= 0 {
		h = w
	}
	return w, h
}

// ResolvedColor is the module foreground.
func (q *QRCode) ResolvedColor() render.RGBA {
	if q == nil {
		return render.RGBA{R: 0, G: 0, B: 0, A: 1}
	}
	if q.colorSet && q.color.A > 0.01 {
		return q.color
	}
	if q.fgColor.A > 0.01 {
		return q.fgColor
	}
	return q.theme().Color(core.TokenColorText)
}

// ResolvedBgColor is the module/root background (may be transparent).
func (q *QRCode) ResolvedBgColor() render.RGBA {
	if q == nil {
		return render.RGBA{}
	}
	if q.bgColorSet {
		return q.bgColor
	}
	return q.resolvedBG
}

// BorderColor returns the chrome border color.
func (q *QRCode) BorderColor() render.RGBA {
	if q == nil {
		return render.RGBA{}
	}
	if q.Style.hasBorder() {
		return q.Style.Border
	}
	return q.borderCol
}

// CoverBackground returns the status mask color.
func (q *QRCode) CoverBackground() render.RGBA {
	if q == nil {
		return render.RGBA{}
	}
	if q.CoverStyle.hasBG() {
		return q.CoverStyle.Background
	}
	return q.coverBG
}

// --- setters ---

// SetValue updates the payload and re-encodes.
func (q *QRCode) SetValue(v string) {
	if q == nil {
		return
	}
	q.value = v
	q.rebuild()
}

// SetType sets canvas|svg label.
func (q *QRCode) SetType(t QRCodeType) {
	if q == nil {
		return
	}
	q.typ = t
	q.rebuild()
}

// SetSize sets the outer box edge length in px.
func (q *QRCode) SetSize(px float64) {
	if q == nil {
		return
	}
	q.size = px
	q.rebuild()
}

// SetIcon sets a string icon source (placeholder box; true HTTP decode is P1).
func (q *QRCode) SetIcon(src string) {
	if q == nil {
		return
	}
	q.iconSrc = src
	q.iconNode = nil
	q.rebuild()
}

// SetIconNode sets a custom center icon node.
func (q *QRCode) SetIconNode(n core.Node) {
	if q == nil {
		return
	}
	q.iconNode = n
	if n != nil {
		q.iconSrc = ""
	}
	q.rebuild()
}

// SetIconSize sets a square icon size (default 40).
func (q *QRCode) SetIconSize(px float64) {
	if q == nil {
		return
	}
	q.iconW, q.iconH = px, px
	q.rebuild()
}

// SetIconSizeWH sets rectangular icon size.
func (q *QRCode) SetIconSizeWH(w, h float64) {
	if q == nil {
		return
	}
	q.iconW, q.iconH = w, h
	q.rebuild()
}

// SetColor sets module foreground (A>0 applies).
func (q *QRCode) SetColor(c render.RGBA) {
	if q == nil {
		return
	}
	q.color = c
	q.colorSet = true
	q.rebuild()
}

// SetBgColor sets module/root background (including transparent).
func (q *QRCode) SetBgColor(c render.RGBA) {
	if q == nil {
		return
	}
	q.bgColor = c
	q.bgColorSet = true
	q.rebuild()
}

// SetBordered toggles chrome border + padding.
func (q *QRCode) SetBordered(v bool) {
	if q == nil {
		return
	}
	q.bordered = v
	q.borderedSet = true
	q.rebuild()
}

// SetErrorLevel sets L/M/Q/H recovery.
func (q *QRCode) SetErrorLevel(lv QRErrorLevel) {
	if q == nil {
		return
	}
	q.errorLevel = lv
	q.rebuild()
}

// SetMarginSize sets quiet-zone modules (0 = none).
func (q *QRCode) SetMarginSize(n int) {
	if q == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	q.marginSize = n
	q.rebuild()
}

// SetStatus sets active|expired|loading|scanned.
func (q *QRCode) SetStatus(s QRStatus) {
	if q == nil {
		return
	}
	q.status = s
	q.life.setActive(q.needsTicker())
	q.rebuild()
}

// SetStatusRender sets a custom cover body builder.
func (q *QRCode) SetStatusRender(fn func(QRStatusRenderInfo) core.Node) {
	if q == nil {
		return
	}
	q.StatusRender = fn
	q.rebuild()
}

// SetOnRefresh sets the expired refresh callback.
func (q *QRCode) SetOnRefresh(fn func()) {
	if q == nil {
		return
	}
	q.OnRefresh = fn
	q.rebuild()
}

// SetTheme sets an explicit theme override.
func (q *QRCode) SetTheme(th *core.Theme) {
	if q == nil {
		return
	}
	q.Theme = th
	q.rebuild()
}

// SetFace sets the status text face.
func (q *QRCode) SetFace(face text.Face) {
	if q == nil {
		return
	}
	q.Face = face
	q.rebuild()
}

// SetStyle sets root Style overrides.
func (q *QRCode) SetStyle(st Style) {
	if q == nil {
		return
	}
	q.Style = st
	q.rebuild()
}

// SetCoverStyle sets cover Style overrides.
func (q *QRCode) SetCoverStyle(st Style) {
	if q == nil {
		return
	}
	q.CoverStyle = st
	q.rebuild()
}

// SetClassNames sets shallow semantic class tags.
func (q *QRCode) SetClassNames(cn QRCodeClassNames) {
	if q == nil {
		return
	}
	q.ClassNames = cn
	q.rebuild()
}

// SetAriaLabel sets the accessible name on the root.
func (q *QRCode) SetAriaLabel(s string) {
	if q == nil {
		return
	}
	q.AriaLabel = s
	if q.Root != nil {
		q.Root.Base().Label = s
	}
}

// AttachTicker binds loading animation (also automatic OnMount via qrcodeHost).
func (q *QRCode) AttachTicker(t *core.Tree) {
	if q == nil {
		return
	}
	q.life.attach(t, q, q.needsTicker())
}

// qrcodeHost is Decorated + mount lifecycle for loading Ticker.
type qrcodeHost struct {
	primitive.Decorated
	qr *QRCode
}

func (h *qrcodeHost) TypeID() string { return "kit.QRCode" }

func (h *qrcodeHost) OnMount() {
	if h == nil || h.qr == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.qr.life.attach(t, h.qr, h.qr.needsTicker())
	}
}

func (h *qrcodeHost) OnUnmount() {
	if h != nil && h.qr != nil {
		h.qr.life.unmount()
	}
}

// Tick advances the loading ring.
func (q *QRCode) Tick(dt float64) (still bool) {
	if q == nil || !q.needsTicker() {
		return false
	}
	var nt *core.Tree
	if q.Root != nil {
		nt = q.Root.Tree()
	}
	if !q.life.stillMounted(nt) {
		return false
	}
	q.spinPhase += dt * qrcodeSpinRPS
	if q.spinPhase > 1 {
		q.spinPhase -= math.Floor(q.spinPhase)
	}
	if q.spinCV != nil {
		q.spinCV.MarkNeedsPaint()
	} else if q.Root != nil {
		q.Root.MarkNeedsPaint()
	}
	return true
}

func (q *QRCode) needsTicker() bool {
	return q != nil && q.status == QRStatusLoading
}

func (q *QRCode) theme() *core.Theme {
	var n core.Node
	if q != nil && q.Root != nil {
		n = q.Root
	}
	if q != nil {
		return themeOf(q.Theme, n)
	}
	return DefaultTheme()
}

func (q *QRCode) encodeModules() {
	q.modules = nil
	q.moduleN = 0
	if q == nil || q.value == "" {
		return
	}
	level := qrcode.Medium
	switch q.errorLevel {
	case QRErrorLevelL:
		level = qrcode.Low
	case QRErrorLevelQ:
		level = qrcode.High
	case QRErrorLevelH:
		level = qrcode.Highest
	default:
		level = qrcode.Medium
	}
	code, err := qrcode.New(q.value, level)
	if err != nil || code == nil {
		// Fallback: tiny deterministic stand-in so layout never collapses.
		q.modules = pseudoQRModules(q.value, 21)
		q.moduleN = len(q.modules)
		return
	}
	code.DisableBorder = true
	bm := code.Bitmap()
	if len(bm) == 0 {
		q.modules = pseudoQRModules(q.value, 21)
		q.moduleN = len(q.modules)
		return
	}
	q.modules = bm
	q.moduleN = len(bm)
}

func pseudoQRModules(text string, n int) [][]bool {
	if n < 21 {
		n = 21
	}
	hash := 0
	for _, r := range text {
		hash = hash*31 + int(r)
	}
	out := make([][]bool, n)
	for y := 0; y < n; y++ {
		row := make([]bool, n)
		for x := 0; x < n; x++ {
			v := (hash + x*17 + y*31) & 1
			// finder-ish corners
			if (x < 7 && y < 7) || (x >= n-7 && y < 7) || (x < 7 && y >= n-7) {
				v = 1
				if x > 1 && x < 5 && y > 1 && y < 5 {
					v = 0
				}
				if x > 2 && x < 4 && y > 2 && y < 4 {
					v = 1
				}
			}
			if y == 6 || x == 6 {
				if (x+y)%2 == 0 {
					v = 1
				} else {
					v = 0
				}
			}
			row[x] = v == 1
		}
		out[y] = row
	}
	return out
}

func (q *QRCode) rebuild() {
	if q == nil {
		return
	}
	th := q.theme()

	// resolve tokens
	q.pad = th.SizeOr(core.TokenPaddingSM, DefaultQRCodePadding)
	if q.pad <= 0 {
		q.pad = DefaultQRCodePadding
	}
	q.radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultQRCodeRadius)
	if q.radius <= 0 {
		q.radius = DefaultQRCodeRadius
	}
	q.lineW = th.SizeOr(core.TokenLineWidth, DefaultQRCodeLineWidth)
	if q.lineW <= 0 {
		q.lineW = DefaultQRCodeLineWidth
	}

	q.fgColor = th.Color(core.TokenColorText)
	if q.fgColor.A < 0.05 {
		q.fgColor = render.RGBA{R: 0, G: 0, B: 0, A: 1}
	}
	q.borderCol = th.Color(core.TokenColorSplit)
	if q.borderCol.A < 0.02 {
		q.borderCol = render.Hex("#F0F0F0")
	}
	// root white when bordered and no explicit bg (antd colorWhite)
	q.resolvedBG = render.RGBA{}
	if q.Bordered() && !q.bgColorSet {
		// antd default backgroundColor: colorWhite
		q.resolvedBG = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	if q.bgColorSet {
		q.resolvedBG = q.bgColor
	}
	// cover
	cbg := th.Color(core.TokenColorBgContainer)
	if cbg.A < 0.05 {
		cbg = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	cbg.A = DefaultQRCodeCoverAlpha
	q.coverBG = cbg

	q.encodeModules()

	outer := q.Size()
	pad := q.Padding()
	radius := q.Radius()
	lineW := q.LineWidth()
	fg := q.ResolvedColor()
	// module bg: explicit bgColor, else white under modules for contrast when transparent
	modBG := q.ResolvedBgColor()
	paintBG := modBG
	if paintBG.A < 0.01 {
		paintBG = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}

	// Root (stable host)
	if q.Root == nil {
		h := &qrcodeHost{qr: q}
		h.Init(h)
		h.Hit = core.HitBlock
		q.Root = h
	} else {
		q.Root.ClearChildren()
		q.Root.qr = q
	}
	q.Root.Hit = core.HitBlock
	q.Root.Width = outer
	q.Root.Height = outer
	q.Root.StretchChild = true
	q.Root.Padding = primitive.EdgeInsets{Left: pad, Right: pad, Top: pad, Bottom: pad}
	q.Root.Radius = radius
	q.Root.BorderWidth = lineW
	if q.Bordered() {
		q.Root.BorderColor = q.BorderColor()
	} else {
		q.Root.BorderColor = render.RGBA{}
		q.Root.BorderWidth = 0
	}
	// root background
	rootBG := q.resolvedBG
	if q.Style.hasBG() {
		rootBG = q.Style.Background
	}
	q.Root.Background = rootBG
	if q.Style.hasBorder() && q.Bordered() {
		q.Root.BorderColor = q.Style.Border
	}
	if q.Style.hasRadius() && q.Bordered() {
		q.Root.Radius = q.Style.Radius
	}

	q.Root.Base().Role = "img"
	label := q.AriaLabel
	if label == "" {
		if q.value != "" {
			label = "QR code"
		} else {
			label = "QR code empty"
		}
	}
	q.Root.Base().Label = label
	if q.ClassNames.Root != "" {
		q.Root.Base().Key = q.ClassNames.Root
	}

	// modules painter
	modules := q.modules
	margin := q.MarginSize()
	iconW, iconH := q.IconSize()
	hasIcon := q.HasIcon()
	inner := outer - 2*pad
	if inner < 1 {
		inner = 1
	}

	q.painter = primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		// fill module area
		pc.FillLocalRect(0, 0, sz.Width, sz.Height, paintBG)
		if len(modules) == 0 {
			return
		}
		n := len(modules)
		total := n + 2*margin
		if total < 1 {
			total = 1
		}
		cell := sz.Width / float64(total)
		if cell <= 0 {
			return
		}
		// excavate rect in module coords
		var ex0, ey0, ex1, ey1 float64
		if hasIcon {
			cw, ch := iconW, iconH
			// icon sits in content box; map to module space
			ex0 = (sz.Width - cw) / 2
			ey0 = (sz.Height - ch) / 2
			ex1 = ex0 + cw
			ey1 = ey0 + ch
		}
		for y := 0; y < n; y++ {
			row := modules[y]
			for x := 0; x < len(row); x++ {
				if !row[x] {
					continue
				}
				px := float64(x+margin) * cell
				py := float64(y+margin) * cell
				// excavate under icon
				if hasIcon {
					cx := px + cell/2
					cy := py + cell/2
					if cx >= ex0 && cx <= ex1 && cy >= ey0 && cy <= ey1 {
						continue
					}
				}
				pc.FillLocalRect(px, py, cell+0.5, cell+0.5, fg)
			}
		}
	})
	q.painter.Width = inner
	q.painter.Height = inner

	// stack children
	kids := []core.Node{q.painter}

	// icon host (center)
	q.iconHost = nil
	if hasIcon {
		var iconChild core.Node
		if q.iconNode != nil {
			iconChild = q.iconNode
		} else {
			// string src placeholder — labeled box (host may replace via SetIconNode)
			box := primitive.NewDecorated()
			box.Width = iconW
			box.Height = iconH
			box.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
			box.BorderWidth = 1
			box.BorderColor = q.borderCol
			box.Radius = 4
			box.Base().Role = "img"
			box.Base().Label = "QR icon"
			// simple brand-ish glyph using text
			lab := primitive.NewText("◆")
			lab.FontSize = math.Min(iconW, iconH) * 0.45
			lab.Color = fg
			lab.Face = q.Face
			box.CenterContent = true
			box.AddChild(lab)
			iconChild = box
		}
		host := primitive.NewDecorated(iconChild)
		host.Width = iconW
		host.Height = iconH
		host.Hit = core.HitDefer
		host.CenterContent = true
		host.StretchChild = true
		// white plate under icon for excavate readability
		host.Background = paintBG
		host.Radius = 2
		q.iconHost = host
		kids = append(kids, primitive.Positioned(core.AlignCenter, host))
	}

	// cover
	q.cover = nil
	q.refresh = nil
	q.spinCV = nil
	if q.HasCover() {
		body := q.buildStatusBody(th, fg)
		cv := primitive.NewDecorated()
		cv.Hit = core.HitBlock // block interaction to modules; allow refresh
		cv.StretchChild = true
		cv.CenterContent = true
		cv.Background = q.CoverBackground()
		if q.CoverStyle.hasBorder() {
			cv.BorderColor = q.CoverStyle.Border
			cv.BorderWidth = 1
		}
		if q.ClassNames.Cover != "" {
			cv.Base().Key = q.ClassNames.Cover
		}
		if body != nil {
			cv.AddChild(body)
		}
		// force cover to fill stack via Positioned stretch — use full-size positioned
		// Stack lays Positioned children; use AlignStretch if available, else set size
		cv.Width = inner
		cv.Height = inner
		q.cover = cv
		kids = append(kids, primitive.Positioned(core.AlignCenter, cv))
	}

	q.stack = primitive.NewStack(kids...)
	q.Root.AddChild(q.stack)

	q.life.setActive(q.needsTicker())

	q.Root.SetThemeHook(func(th *core.Theme) {
		if th != nil {
			q.Theme = th
		}
		q.rebuild()
	})
}

func (q *QRCode) buildStatusBody(th *core.Theme, fg render.RGBA) core.Node {
	info := QRStatusRenderInfo{
		Status:    q.status,
		Expired:   DefaultQRCodeExpired,
		Refresh:   DefaultQRCodeRefresh,
		Scanned:   DefaultQRCodeScanned,
		OnRefresh: q.OnRefresh,
	}
	if q.StatusRender != nil && q.status != QRStatusActive {
		if n := q.StatusRender(info); n != nil {
			return n
		}
	}
	fs := th.SizeOr(core.TokenFontSize, DefaultQRCodeFontSize)
	if fs <= 0 {
		fs = DefaultQRCodeFontSize
	}
	textCol := th.Color(core.TokenColorText)
	if textCol.A < 0.05 {
		textCol = fg
	}

	switch q.status {
	case QRStatusLoading:
		size := th.SizeOr(core.TokenSpinSize, 20)
		if size <= 0 {
			size = 20
		}
		track := th.Color(core.TokenColorFillSecondary)
		if track.A < 0.08 {
			track = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
		}
		fill := th.Color(core.TokenColorPrimary)
		phase := &q.spinPhase
		q.spinCV = primitive.NewCanvas(size, size, func(pc *core.PaintContext, sz core.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			stroke := size * 0.12
			if stroke < 2 {
				stroke = 2
			}
			if stroke > 3 {
				stroke = 3
			}
			dc := pc.DC
			cx := pc.Origin.X + sz.Width/2
			cy := pc.Origin.Y + sz.Height/2
			r := size/2 - stroke
			if r < 1 {
				r = 1
			}
			dc.SetLineCap(render.LineCapRound)
			dc.SetLineJoin(render.LineJoinRound)
			dc.SetLineWidth(stroke)
			dc.SetRGBA(track.R, track.G, track.B, track.A)
			dc.DrawCircle(cx, cy, r)
			_ = dc.Stroke()
			start := -math.Pi/2 + (*phase)*2*math.Pi
			end := start + 2*math.Pi*0.75
			dc.SetRGBA(fill.R, fill.G, fill.B, fill.A)
			for i := 0; i <= 64; i++ {
				a := start + (end-start)*float64(i)/64
				x := cx + r*math.Cos(a)
				y := cy + r*math.Sin(a)
				if i == 0 {
					dc.MoveTo(x, y)
				} else {
					dc.LineTo(x, y)
				}
			}
			_ = dc.Stroke()
		})
		q.spinCV.Base().Role = "status"
		q.spinCV.Base().Label = "Loading"
		q.spinCV.Base().Live = "polite"
		return q.spinCV

	case QRStatusExpired:
		col := primitive.Column()
		col.Gap = 4
		col.CrossAlign = core.CrossCenter
		col.MainAlign = core.MainCenter
		lab := primitive.NewText(DefaultQRCodeExpired)
		lab.FontSize = fs
		lab.Face = q.Face
		lab.Color = textCol
		col.AddChild(lab)
		if q.OnRefresh != nil {
			btn := NewButton(DefaultQRCodeRefresh)
			btn.SetType(ButtonLink)
			btn.SetFace(q.Face)
			btn.Theme = th
			btn.SetAriaLabel(DefaultQRCodeRefresh)
			on := q.OnRefresh
			btn.SetOnClick(func() {
				if on != nil {
					on()
				}
			})
			q.refresh = btn
			col.AddChild(btn.Node())
		}
		return col

	case QRStatusScanned:
		lab := primitive.NewText(DefaultQRCodeScanned)
		lab.FontSize = fs
		lab.Face = q.Face
		lab.Color = textCol
		return lab
	}
	return nil
}
