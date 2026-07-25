package kit

import (
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
)

// Ant Design Watermark defaults — docs/antd/watermark.md §6.2
// components/watermark + useClips.ts / index.tsx.
const (
	// DefaultWatermarkRotate is antd rotate default (degrees).
	DefaultWatermarkRotate = -22.0
	// DefaultWatermarkGapX / GapY are antd gap default.
	DefaultWatermarkGapX = 100.0
	DefaultWatermarkGapY = 100.0
	// DefaultWatermarkWidth / Height are image (and explicit) mark defaults.
	DefaultWatermarkWidth  = 120.0
	DefaultWatermarkHeight = 64.0
	// DefaultWatermarkFontSize is Font.fontSize / fontSizeLG.
	DefaultWatermarkFontSize = 16.0
	// DefaultWatermarkFontGap is useClips FontGap between multi-line rows.
	DefaultWatermarkFontGap = 3.0
	// DefaultWatermarkZIndex is kit paint-order z (antd docs 999 / zIndexPopupBase-1).
	DefaultWatermarkZIndex = 9
	// DefaultWatermarkColor is Font.color ≈ rgba(0,0,0,.15) (antd colorFill family).
	// Not a brand primary — semantic fill.
	DefaultWatermarkColorA = 0.15
)

// DefaultWatermarkColor returns the antd default mark ink.
func DefaultWatermarkColor() render.RGBA {
	return render.RGBA{R: 0, G: 0, B: 0, A: DefaultWatermarkColorA}
}

// WatermarkFont is antd Font (text style for marks).
type WatermarkFont struct {
	// Color is fill ink; A==0 means unset → default/theme.
	Color render.RGBA
	// FontSize in px; 0 → DefaultWatermarkFontSize (16).
	FontSize float64
	// FontWeight: normal|lighter|bold|bolder (informational; face may not vary).
	FontWeight string
	// FontStyle: none|normal|italic|oblique.
	FontStyle string
	// FontFamily informational (desktop face comes from SetFace / Theme).
	FontFamily string
	// TextAlign: left|center|right; empty → center.
	TextAlign string
}

// WatermarkContentLine is one content row (string or WatermarkText).
type WatermarkContentLine struct {
	Text string
	// Font is optional per-line override (antd WatermarkText.font). Zero = use root Font.
	Font WatermarkFont
	// HasFont is true when Font was explicitly provided for this line.
	HasFont bool
}

// Watermark is Ant Design Watermark — tiled decorative overlay over children.
//
//	watermarkHost
//	  ├─ children (interactive)
//	  └─ markLayer (HitTransparent · tiled rotate text/image)
//
// Product contract: docs/antd/watermark.md §6 (P0).
// Image pixels are host-decoded via SetImagePixels (no HTTP in kit).
type Watermark struct {
	Root *watermarkHost

	child core.Node
	layer *watermarkLayer

	// Content lines (watermark text). Empty + no image → no mark.
	lines []WatermarkContentLine

	// Width / Height of one mark cell. 0 → auto (text measure) or 120×64 (image).
	Width  float64
	Height float64

	// Rotate in degrees. rotateSet distinguishes explicit 0 from default -22.
	Rotate    float64
	rotateSet bool

	// ZIndex is logical stacking (paint order already above children).
	ZIndex    int
	zIndexSet bool

	// Image is the source label (antd image). Priority over text when pixels OK.
	Image string

	// Gap between marks. 0,0 with gapSet=false → default 100,100.
	GapX, GapY float64
	gapSet     bool

	// Offset from container top-left. Unset → gap/2.
	OffsetX, OffsetY float64
	offsetSet        bool

	// Font is the root text style.
	Font WatermarkFont

	// Inherit controls portal propagation (default true). Use Wrap for Modal/Drawer.
	Inherit    bool
	inheritSet bool

	// OnRemove is called from NotifyRemoved (DOM-remove mapping).
	OnRemove func()

	// Loading drives Ticker while image is pending host pixels.
	Loading bool

	Face      text.Face
	Theme     *core.Theme
	Style     Style
	AriaLabel string

	// image host state
	imageW, imageH int
	imageRGBA      []byte
	imageOK        bool
	imageErr       bool
	imageBuf       *render.ImageBuf

	life      tickerLifecycle
	boundTree *core.Tree
}

// watermarkHost sizes to children and paints the transparent mark layer on top.
type watermarkHost struct {
	core.NodeBase
	wm    *Watermark
	child core.Node
	layer *watermarkLayer
}

func (h *watermarkHost) TypeID() string { return "kit.Watermark" }

func (h *watermarkHost) Layout(c core.Constraints) core.Size {
	if h == nil {
		return core.Size{}
	}
	if sz, ok := h.LayoutSkipIfClean(c); ok {
		return sz
	}
	var csz core.Size
	if h.child != nil {
		csz = h.child.Layout(c)
		h.child.Base().SetOffset(core.Point{})
	}
	out := c.Tighten(csz)
	if c.IsTight() {
		out = core.Size{Width: c.MaxWidth, Height: c.MaxHeight}
		if h.child != nil {
			_ = h.child.Layout(core.Tight(out.Width, out.Height))
			h.child.Base().SetOffset(core.Point{})
		}
	}
	if h.layer != nil {
		_ = h.layer.Layout(core.Tight(out.Width, out.Height))
		h.layer.Base().SetOffset(core.Point{})
	}
	h.SetSize(out)
	h.RememberConstraints(c)
	h.ClearLayoutDirty()
	return out
}

func (h *watermarkHost) Paint(pc *core.PaintContext) {
	if h == nil {
		return
	}
	// Clip to host box (antd overflow:hidden on container).
	if pc != nil {
		if out := h.Size(); out.Width > 0 && out.Height > 0 {
			pc.PushClipLocal(0, 0, out.Width, out.Height)
			defer pc.Pop()
		}
	}
	// Children order: body then mark layer (AddChild order).
	h.DefaultPaintChildren(pc)
	if pc != nil {
		h.ClearPaintDirty()
	}
}

func (h *watermarkHost) HitTest(p core.Point) core.Node {
	if h == nil {
		return nil
	}
	// Mark is pointer-events:none — only children participate (WM-S5).
	if h.child != nil {
		if hit := h.child.HitTest(p.Sub(h.child.Base().Offset())); hit != nil {
			return hit
		}
	}
	return nil
}

func (h *watermarkHost) OnMount() {
	if h == nil || h.wm == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.wm.boundTree = t
		h.wm.life.attach(t, h.wm, h.wm.needsTicker())
	}
}

func (h *watermarkHost) OnUnmount() {
	if h == nil || h.wm == nil {
		return
	}
	h.wm.life.unmount()
	h.wm.boundTree = nil
}

// watermarkLayer paints tiled marks; never takes hits.
type watermarkLayer struct {
	core.NodeBase
	wm *Watermark
}

func (l *watermarkLayer) TypeID() string { return "kit.WatermarkMark" }

func (l *watermarkLayer) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{Width: c.MaxWidth, Height: c.MaxHeight})
	if c.MaxWidth > 0 && c.MaxHeight > 0 {
		out = core.Size{Width: c.MaxWidth, Height: c.MaxHeight}
	}
	l.SetSize(out)
	return out
}

func (l *watermarkLayer) Paint(pc *core.PaintContext) {
	if l == nil || l.wm == nil || pc == nil {
		return
	}
	l.wm.paintMarks(pc, l.Size())
	if pc != nil {
		l.ClearPaintDirty()
	}
}

func (l *watermarkLayer) HitTest(core.Point) core.Node { return nil }

// NewWatermark wraps child with a watermark overlay (antd children).
// Content text is empty until SetContent / SetContentLines / SetImage.
// Defaults: rotate=-22, gap=100×100, inherit=true.
func NewWatermark(child core.Node) *Watermark {
	w := &Watermark{
		child:   child,
		Inherit: true,
		Rotate:  DefaultWatermarkRotate,
		GapX:    DefaultWatermarkGapX,
		GapY:    DefaultWatermarkGapY,
		ZIndex:  DefaultWatermarkZIndex,
	}
	w.rebuild()
	return w
}

// Node returns the root core.Node.
func (w *Watermark) Node() core.Node {
	if w == nil {
		return nil
	}
	if w.Root == nil {
		w.rebuild()
	}
	return w.Root
}

// Child returns the wrapped content node.
func (w *Watermark) Child() core.Node {
	if w == nil {
		return nil
	}
	return w.child
}

// ContentLines returns a copy of watermark text rows.
func (w *Watermark) ContentLines() []WatermarkContentLine {
	if w == nil || len(w.lines) == 0 {
		return nil
	}
	out := make([]WatermarkContentLine, len(w.lines))
	copy(out, w.lines)
	return out
}

// ContentText joins content lines with newline (convenience).
func (w *Watermark) ContentText() string {
	if w == nil || len(w.lines) == 0 {
		return ""
	}
	parts := make([]string, 0, len(w.lines))
	for _, ln := range w.lines {
		parts = append(parts, ln.Text)
	}
	return strings.Join(parts, "\n")
}

// HasMark reports whether a watermark will paint (text and/or image OK).
func (w *Watermark) HasMark() bool {
	if w == nil {
		return false
	}
	if w.imageOK && w.Image != "" && len(w.imageRGBA) > 0 {
		return true
	}
	for _, ln := range w.lines {
		if strings.TrimSpace(ln.Text) != "" {
			return true
		}
	}
	// Image pending/err with content fallback already covered by lines.
	// Image-only without pixels yet: still "has mark intent" for loading demos.
	if w.Image != "" && !w.imageErr {
		return true
	}
	return false
}

// UsesImageMark is true when image pixels win over text.
func (w *Watermark) UsesImageMark() bool {
	return w != nil && w.imageOK && w.Image != "" && len(w.imageRGBA) > 0 && !w.imageErr
}

// SetChild replaces the wrapped body.
func (w *Watermark) SetChild(n core.Node) {
	if w == nil {
		return
	}
	w.child = n
	w.rebuild()
}

// SetContent sets a single-line watermark text (antd content=string).
func (w *Watermark) SetContent(text string) {
	if w == nil {
		return
	}
	if text == "" {
		w.lines = nil
	} else {
		w.lines = []WatermarkContentLine{{Text: text}}
	}
	w.rebuild()
}

// SetContentStrings sets multi-line string content.
func (w *Watermark) SetContentStrings(lines ...string) {
	if w == nil {
		return
	}
	w.lines = w.lines[:0]
	for _, s := range lines {
		if s == "" {
			continue
		}
		w.lines = append(w.lines, WatermarkContentLine{Text: s})
	}
	w.rebuild()
}

// SetContentLines sets full content rows (supports per-line Font).
func (w *Watermark) SetContentLines(lines ...WatermarkContentLine) {
	if w == nil {
		return
	}
	w.lines = w.lines[:0]
	for _, ln := range lines {
		if strings.TrimSpace(ln.Text) == "" {
			continue
		}
		w.lines = append(w.lines, ln)
	}
	w.rebuild()
}

// SetWidth sets mark width (0 → auto/default).
func (w *Watermark) SetWidth(v float64) {
	if w == nil {
		return
	}
	w.Width = v
	w.markPaint()
}

// SetHeight sets mark height (0 → auto/default).
func (w *Watermark) SetHeight(v float64) {
	if w == nil {
		return
	}
	w.Height = v
	w.markPaint()
}

// SetRotate sets rotation in degrees (explicit, including 0).
func (w *Watermark) SetRotate(deg float64) {
	if w == nil {
		return
	}
	w.Rotate = deg
	w.rotateSet = true
	w.markPaint()
}

// SetZIndex sets logical z-index.
func (w *Watermark) SetZIndex(z int) {
	if w == nil {
		return
	}
	w.ZIndex = z
	w.zIndexSet = true
}

// SetGap sets gap between marks (antd gap=[x,y]).
func (w *Watermark) SetGap(x, y float64) {
	if w == nil {
		return
	}
	w.GapX, w.GapY = x, y
	w.gapSet = true
	w.markPaint()
}

// SetOffset sets offset from top-left (antd offset). Explicit including 0.
func (w *Watermark) SetOffset(x, y float64) {
	if w == nil {
		return
	}
	w.OffsetX, w.OffsetY = x, y
	w.offsetSet = true
	w.markPaint()
}

// ClearOffset restores offset = gap/2.
func (w *Watermark) ClearOffset() {
	if w == nil {
		return
	}
	w.offsetSet = false
	w.OffsetX, w.OffsetY = 0, 0
	w.markPaint()
}

// SetFont replaces root Font.
func (w *Watermark) SetFont(f WatermarkFont) {
	if w == nil {
		return
	}
	w.Font = f
	w.markPaint()
}

// SetFontColor sets Font.color.
func (w *Watermark) SetFontColor(c render.RGBA) {
	if w == nil {
		return
	}
	w.Font.Color = c
	w.markPaint()
}

// SetFontSize sets Font.fontSize (0 → default 16 on resolve).
func (w *Watermark) SetFontSize(px float64) {
	if w == nil {
		return
	}
	w.Font.FontSize = px
	w.markPaint()
}

// SetImage sets image source label. Clears prior pixels until SetImagePixels.
func (w *Watermark) SetImage(src string) {
	if w == nil {
		return
	}
	w.Image = src
	w.imageOK = false
	w.imageErr = false
	w.imageRGBA = nil
	w.imageBuf = nil
	w.imageW, w.imageH = 0, 0
	if src != "" {
		w.Loading = true
	} else {
		w.Loading = false
	}
	w.life.setActive(w.needsTicker())
	w.rebuild()
}

// SetImagePixels installs host-decoded RGBA and marks image OK (antd image load).
func (w *Watermark) SetImagePixels(width, height int, rgba []byte) {
	if w == nil {
		return
	}
	w.imageW, w.imageH = width, height
	if width > 0 && height > 0 && len(rgba) >= width*height*4 {
		w.imageRGBA = append([]byte(nil), rgba...)
		w.imageOK = true
		w.imageErr = false
		w.Loading = false
		w.imageBuf = nil
	} else {
		w.imageBuf = nil
		w.imageRGBA = nil
		w.imageOK = false
	}
	w.life.setActive(w.needsTicker())
	w.markPaint()
}

// NotifyImageError marks image load failure; content text becomes fallback (FAQ).
func (w *Watermark) NotifyImageError() {
	if w == nil {
		return
	}
	w.imageErr = true
	w.imageOK = false
	w.Loading = false
	w.imageRGBA = nil
	w.imageBuf = nil
	w.life.setActive(w.needsTicker())
	w.markPaint()
}

// SetInherit sets whether mark config should propagate to portals (default true).
func (w *Watermark) SetInherit(v bool) {
	if w == nil {
		return
	}
	w.Inherit = v
	w.inheritSet = true
}

// ResolvedInherit returns inherit (default true).
func (w *Watermark) ResolvedInherit() bool {
	if w == nil {
		return true
	}
	if w.inheritSet {
		return w.Inherit
	}
	return true
}

// Wrap returns a new Watermark with the same mark configuration around child.
// Used for Modal/Drawer content when Inherit is true (desktop portal mapping).
func (w *Watermark) Wrap(child core.Node) *Watermark {
	if w == nil {
		return NewWatermark(child)
	}
	nw := NewWatermark(child)
	nw.copyMarkFrom(w)
	return nw
}

// SetOnRemove sets the onRemove callback.
func (w *Watermark) SetOnRemove(fn func()) {
	if w == nil {
		return
	}
	w.OnRemove = fn
}

// NotifyRemoved fires OnRemove (antd watermark DOM removed mapping).
func (w *Watermark) NotifyRemoved() {
	if w == nil || w.OnRemove == nil {
		return
	}
	w.OnRemove()
}

// SetLoading toggles image-loading ticker.
func (w *Watermark) SetLoading(v bool) {
	if w == nil {
		return
	}
	w.Loading = v
	w.life.setActive(w.needsTicker())
	if w.Root != nil {
		w.Root.MarkNeedsPaint()
	}
}

// SetTheme sets explicit theme.
func (w *Watermark) SetTheme(th *core.Theme) {
	if w == nil {
		return
	}
	w.Theme = th
	w.markPaint()
}

// SetFace sets the text face used for marks.
func (w *Watermark) SetFace(f text.Face) {
	if w == nil {
		return
	}
	w.Face = f
	w.markPaint()
}

// SetStyle sets optional style overrides (Text → font color).
func (w *Watermark) SetStyle(st Style) {
	if w == nil {
		return
	}
	w.Style = st
	w.markPaint()
}

// SetAriaLabel sets accessible name (decorative default stays empty / presentation).
func (w *Watermark) SetAriaLabel(s string) {
	if w == nil {
		return
	}
	w.AriaLabel = s
	w.applyA11y()
}

// ResolvedRotate returns rotate degrees (default -22).
func (w *Watermark) ResolvedRotate() float64 {
	if w == nil {
		return DefaultWatermarkRotate
	}
	if w.rotateSet {
		return w.Rotate
	}
	if w.Rotate != 0 {
		return w.Rotate
	}
	return DefaultWatermarkRotate
}

// ResolvedGap returns gap x,y (default 100,100).
func (w *Watermark) ResolvedGap() (float64, float64) {
	if w == nil {
		return DefaultWatermarkGapX, DefaultWatermarkGapY
	}
	if w.gapSet {
		return w.GapX, w.GapY
	}
	// NewWatermark seeds GapX/Y with defaults; treat zero pair as default.
	if w.GapX == 0 && w.GapY == 0 {
		return DefaultWatermarkGapX, DefaultWatermarkGapY
	}
	gx, gy := w.GapX, w.GapY
	if gx == 0 {
		gx = DefaultWatermarkGapX
	}
	if gy == 0 {
		gy = DefaultWatermarkGapY
	}
	return gx, gy
}

// ResolvedOffset returns offset; default gap/2.
func (w *Watermark) ResolvedOffset() (float64, float64) {
	if w == nil {
		return DefaultWatermarkGapX / 2, DefaultWatermarkGapY / 2
	}
	if w.offsetSet {
		return w.OffsetX, w.OffsetY
	}
	gx, gy := w.ResolvedGap()
	return gx / 2, gy / 2
}

// ResolvedFontSize returns root font size (default 16 / fontSizeLG).
func (w *Watermark) ResolvedFontSize() float64 {
	if w == nil {
		return DefaultWatermarkFontSize
	}
	if w.Font.FontSize > 0 {
		return w.Font.FontSize
	}
	if w.Style.FontSize > 0 {
		return w.Style.FontSize
	}
	th := w.theme()
	if th != nil {
		if v := th.SizeOr(core.TokenFontSizeLG, DefaultWatermarkFontSize); v > 0 {
			return v
		}
	}
	return DefaultWatermarkFontSize
}

// ResolvedColor returns mark ink color.
func (w *Watermark) ResolvedColor() render.RGBA {
	if w == nil {
		return DefaultWatermarkColor()
	}
	if w.Style.hasText() {
		return w.Style.Text
	}
	if w.Font.Color.A > 0 {
		return w.Font.Color
	}
	// Prefer non-brand fill; no TokenColorFill in kit — use documented default.
	return DefaultWatermarkColor()
}

// ResolvedZIndex returns z-index (default 9).
func (w *Watermark) ResolvedZIndex() int {
	if w == nil {
		return DefaultWatermarkZIndex
	}
	if w.zIndexSet {
		return w.ZIndex
	}
	if w.ZIndex != 0 {
		return w.ZIndex
	}
	return DefaultWatermarkZIndex
}

// ResolvedMarkSize returns one tile content size (before rotate expand).
func (w *Watermark) ResolvedMarkSize() (float64, float64) {
	if w == nil {
		return DefaultWatermarkWidth, DefaultWatermarkHeight
	}
	if w.UsesImageMark() {
		mw, mh := w.Width, w.Height
		if mw <= 0 {
			mw = DefaultWatermarkWidth
		}
		if mh <= 0 {
			mh = DefaultWatermarkHeight
		}
		return mw, mh
	}
	// Text path
	if w.Width > 0 && w.Height > 0 {
		return w.Width, w.Height
	}
	mw, mh := w.measureTextMark()
	if w.Width > 0 {
		mw = w.Width
	}
	if w.Height > 0 {
		mh = w.Height
	}
	if mw <= 0 {
		mw = DefaultWatermarkWidth
	}
	if mh <= 0 {
		mh = DefaultWatermarkHeight
	}
	return mw, mh
}

// AttachTicker binds loading animation lifecycle.
func (w *Watermark) AttachTicker(t *core.Tree) {
	if w == nil || t == nil {
		return
	}
	w.boundTree = t
	w.life.attach(t, w, w.needsTicker())
}

// Tick implements core.Ticker — active only while Loading.
func (w *Watermark) Tick(dt float64) bool {
	if w == nil {
		return false
	}
	if !w.life.stillMounted(w.boundTree) {
		return false
	}
	if !w.needsTicker() {
		return false
	}
	// Keep demand frame alive while host loads image pixels; no visual spin required.
	if w.Root != nil {
		w.Root.MarkNeedsPaint()
	}
	return true
}

// --- internals ---

func (w *Watermark) needsTicker() bool {
	return w != nil && w.Loading
}

func (w *Watermark) theme() *core.Theme {
	var n core.Node
	if w != nil && w.Root != nil {
		n = w.Root
	}
	return themeOf(w.Theme, n)
}

func (w *Watermark) markPaint() {
	if w == nil {
		return
	}
	if w.layer != nil {
		w.layer.MarkNeedsPaint()
	}
	if w.Root != nil {
		w.Root.MarkNeedsPaint()
	}
}

func (w *Watermark) copyMarkFrom(src *Watermark) {
	if w == nil || src == nil {
		return
	}
	w.lines = append([]WatermarkContentLine(nil), src.lines...)
	w.Width, w.Height = src.Width, src.Height
	w.Rotate, w.rotateSet = src.Rotate, src.rotateSet
	w.ZIndex, w.zIndexSet = src.ZIndex, src.zIndexSet
	w.Image = src.Image
	w.GapX, w.GapY, w.gapSet = src.GapX, src.GapY, src.gapSet
	w.OffsetX, w.OffsetY, w.offsetSet = src.OffsetX, src.OffsetY, src.offsetSet
	w.Font = src.Font
	w.Inherit, w.inheritSet = src.Inherit, src.inheritSet
	w.Face = src.Face
	w.Theme = src.Theme
	w.Style = src.Style
	w.AriaLabel = src.AriaLabel
	w.imageW, w.imageH = src.imageW, src.imageH
	if len(src.imageRGBA) > 0 {
		w.imageRGBA = append([]byte(nil), src.imageRGBA...)
	}
	w.imageOK, w.imageErr = src.imageOK, src.imageErr
	w.Loading = src.Loading
	w.rebuild()
}

func (w *Watermark) rebuild() {
	if w == nil {
		return
	}
	if w.Root == nil {
		w.Root = &watermarkHost{wm: w}
		w.Root.Init(w.Root)
		w.Root.Hit = core.HitDefer
	} else {
		w.Root.wm = w
		w.Root.ClearChildren()
	}
	w.layer = &watermarkLayer{wm: w}
	w.layer.Init(w.layer)
	w.layer.Hit = core.HitTransparent

	w.Root.child = w.child
	w.Root.layer = w.layer
	// Children list for tree walks / tests (paint uses explicit order).
	if w.child != nil {
		w.Root.AddChild(w.child)
	}
	w.Root.AddChild(w.layer)

	w.applyA11y()
	w.life.setActive(w.needsTicker())

	w.Root.SetThemeHook(func(th *core.Theme) {
		if th != nil {
			w.Theme = th
		}
		w.markPaint()
	})
	w.Root.MarkNeedsLayout()
	w.Root.MarkNeedsPaint()
}

func (w *Watermark) applyA11y() {
	if w == nil || w.Root == nil {
		return
	}
	// Decorative overlay: presentation; optional label.
	w.Root.Base().Role = "presentation"
	if w.AriaLabel != "" {
		w.Root.Base().Label = w.AriaLabel
	} else {
		w.Root.Base().Label = ""
	}
	if w.layer != nil {
		w.layer.Base().Role = "presentation"
		w.layer.Base().Label = "watermark"
	}
}

func (w *Watermark) lineFont(ln WatermarkContentLine) WatermarkFont {
	base := w.Font
	if !ln.HasFont {
		return base
	}
	f := ln.Font
	// Merge: zero fields fall back to root.
	if f.Color.A <= 0 {
		f.Color = base.Color
	}
	if f.FontSize <= 0 {
		f.FontSize = base.FontSize
	}
	if f.FontWeight == "" {
		f.FontWeight = base.FontWeight
	}
	if f.FontStyle == "" {
		f.FontStyle = base.FontStyle
	}
	if f.FontFamily == "" {
		f.FontFamily = base.FontFamily
	}
	if f.TextAlign == "" {
		f.TextAlign = base.TextAlign
	}
	return f
}

func (w *Watermark) measureTextMark() (float64, float64) {
	if w == nil || len(w.lines) == 0 {
		return 0, 0
	}
	var maxW, totalH float64
	face := w.Face
	for i, ln := range w.lines {
		f := w.lineFont(ln)
		fs := f.FontSize
		if fs <= 0 {
			fs = w.ResolvedFontSize()
		}
		var tw float64
		use := wmFaceAtSize(face, fs)
		if use != nil {
			tw, _ = text.Measure(ln.Text, use)
		}
		if tw <= 0 {
			// Fallback estimate when no face in headless tests.
			tw = float64(len([]rune(ln.Text))) * fs * 0.6
		}
		if tw > maxW {
			maxW = tw
		}
		totalH += fs
		if i < len(w.lines)-1 {
			totalH += DefaultWatermarkFontGap
		}
	}
	return math.Ceil(maxW), math.Ceil(totalH)
}

func wmFaceAtSize(face text.Face, size float64) text.Face {
	if face == nil || size <= 0 {
		return face
	}
	if fs := face.Size(); fs > 0 && math.Abs(fs-size) < 0.25 {
		return face
	}
	type atSizer interface {
		AtSize(float64) text.Face
	}
	if a, ok := face.(atSizer); ok {
		return a.AtSize(size)
	}
	if src := face.Source(); src != nil {
		return src.Face(size)
	}
	return face
}

func (w *Watermark) paintMarks(pc *core.PaintContext, size core.Size) {
	if w == nil || pc == nil || pc.DC == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	if !w.HasMark() {
		return
	}
	mw, mh := w.ResolvedMarkSize()
	if mw <= 0 || mh <= 0 {
		return
	}
	gx, gy := w.ResolvedGap()
	ox, oy := w.ResolvedOffset()
	rot := w.ResolvedRotate() * math.Pi / 180

	// Step matches antd tile cell (mark + gap). Alternate column offset ≈ useClips.
	stepX := mw + gx
	stepY := mh + gy
	if stepX < 1 {
		stepX = 1
	}
	if stepY < 1 {
		stepY = 1
	}

	// backgroundPosition-ish: offset relative to gap center.
	startX := ox - gx/2
	startY := oy - gy/2
	// Ensure coverage of negative start.
	for startX > 0 {
		startX -= stepX
	}
	for startY > 0 {
		startY -= stepY
	}

	dc := pc.DC
	col := w.ResolvedColor()

	// Prepare image buf once.
	var img *render.ImageBuf
	if w.UsesImageMark() {
		img = w.ensureImageBuf()
	}

	face := w.Face
	if face == nil && w.Style.Face != nil {
		face = w.Style.Face
	}

	for y := startY; y < size.Height+stepY; y += stepY {
		colIdx := 0
		for x := startX; x < size.Width+stepX; x += stepX {
			// Primary tile
			w.drawOneMark(pc, dc, x+mw/2, y+mh/2, mw, mh, rot, col, face, img)
			// Alternate tile (antd BaseSize=2 diamond)
			w.drawOneMark(pc, dc, x+mw/2+stepX/2, y+mh/2+stepY/2, mw, mh, rot, col, face, img)
			colIdx++
			_ = colIdx
		}
	}
}

func (w *Watermark) ensureImageBuf() *render.ImageBuf {
	if w == nil || !w.imageOK || w.imageW <= 0 || w.imageH <= 0 || len(w.imageRGBA) < w.imageW*w.imageH*4 {
		return nil
	}
	if w.imageBuf != nil {
		return w.imageBuf
	}
	buf, err := render.NewImageBuf(w.imageW, w.imageH, render.FormatRGBA8)
	if err != nil || buf == nil {
		return nil
	}
	dst := buf.Data()
	n := copy(dst, w.imageRGBA)
	if n <= 0 {
		return nil
	}
	buf.NotifyPixelsChanged()
	w.imageBuf = buf
	return w.imageBuf
}

func (w *Watermark) drawOneMark(
	pc *core.PaintContext,
	dc *render.Context,
	cx, cy, mw, mh, rot float64,
	col render.RGBA,
	face text.Face,
	img *render.ImageBuf,
) {
	if dc == nil || pc == nil {
		return
	}
	ax := pc.Origin.X + cx
	ay := pc.Origin.Y + cy
	dc.Push()
	dc.Translate(ax, ay)
	if rot != 0 {
		dc.Rotate(rot)
	}
	if img != nil && w.UsesImageMark() {
		dc.DrawImageEx(img, render.DrawImageOptions{
			X:         -mw / 2,
			Y:         -mh / 2,
			DstWidth:  mw,
			DstHeight: mh,
			Opacity:   1,
		})
	} else if len(w.imageRGBA) > 0 && w.UsesImageMark() {
		// Fallback pixel grid in local mark space when ImageBuf unavailable.
		w.drawImagePixelsLocal(dc, mw, mh)
	} else {
		w.drawTextMark(dc, mw, mh, col, face)
	}
	dc.Pop()
}

func (w *Watermark) drawImagePixelsLocal(dc *render.Context, mw, mh float64) {
	if w == nil || dc == nil || w.imageW <= 0 || w.imageH <= 0 {
		return
	}
	pix := w.imageRGBA
	pw, ph := w.imageW, w.imageH
	gx, gy := pw, ph
	if gx > 24 {
		gx = 24
	}
	if gy > 24 {
		gy = 24
	}
	cw, ch := mw/float64(gx), mh/float64(gy)
	for y := 0; y < gy; y++ {
		for x := 0; x < gx; x++ {
			sx := x * pw / gx
			sy := y * ph / gy
			i := (sy*pw + sx) * 4
			if i+3 >= len(pix) {
				continue
			}
			dc.SetRGBA(float64(pix[i])/255, float64(pix[i+1])/255, float64(pix[i+2])/255, float64(pix[i+3])/255)
			dc.DrawRectangle(-mw/2+float64(x)*cw, -mh/2+float64(y)*ch, cw+0.5, ch+0.5)
			_ = dc.Fill()
		}
	}
}

func (w *Watermark) drawTextMark(dc *render.Context, mw, mh float64, col render.RGBA, face text.Face) {
	if w == nil || dc == nil || len(w.lines) == 0 {
		return
	}
	type row struct {
		text string
		fs   float64
		f    WatermarkFont
	}
	rows := make([]row, 0, len(w.lines))
	totalH := 0.0
	for _, ln := range w.lines {
		f := w.lineFont(ln)
		fs := f.FontSize
		if fs <= 0 {
			fs = w.ResolvedFontSize()
		}
		rows = append(rows, row{text: ln.Text, fs: fs, f: f})
		totalH += fs
	}
	if len(rows) == 0 {
		return
	}
	if len(rows) > 1 {
		totalH += DefaultWatermarkFontGap * float64(len(rows)-1)
	}
	y := -totalH/2 + rows[0].fs*0.8
	for i, r := range rows {
		ink := col
		if r.f.Color.A > 0 {
			ink = r.f.Color
		}
		use := wmFaceAtSize(face, r.fs)
		if use != nil {
			dc.SetFont(use)
		}
		dc.SetRGBA(ink.R, ink.G, ink.B, ink.A)
		align := r.f.TextAlign
		if align == "" {
			align = w.Font.TextAlign
		}
		ax := 0.5
		switch strings.ToLower(align) {
		case "left", "start":
			ax = 0
		case "right", "end":
			ax = 1
		}
		x := 0.0
		if ax == 0 {
			x = -mw / 2
		} else if ax == 1 {
			x = mw / 2
		}
		if use != nil {
			dc.DrawStringAnchored(r.text, x, y, ax, 0)
		} else {
			approxW := float64(len([]rune(r.text))) * r.fs * 0.5
			dc.SetRGBA(ink.R, ink.G, ink.B, ink.A*0.5)
			dc.DrawRectangle(x-approxW*ax, y-r.fs*0.7, approxW, r.fs*0.6)
			_ = dc.Fill()
		}
		if i < len(rows)-1 {
			y += r.fs + DefaultWatermarkFontGap
		}
	}
	_ = mh
}
