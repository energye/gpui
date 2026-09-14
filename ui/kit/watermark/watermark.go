package watermark

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// Defaults follow docs/antd/watermark.md §6.2 and §6.10.
const (
	DefaultRotate   = -22.0
	DefaultGapX     = 100.0
	DefaultGapY     = 100.0
	DefaultWidth    = 120.0
	DefaultHeight   = 64.0
	DefaultFontSize = 16.0
	// FontGap is the multi-line row gap from useClips.
	FontGap       = 3.0
	DefaultZIndex = 9
	// DefaultFontFamily mirrors the antd Font default.
	DefaultFontFamily = "sans-serif"
	DefaultFontStyle  = "normal"
	DefaultTextAlign  = "center"
	approxCharW       = 0.55
)

// WatermarkFont carries one text style (antd Font subset).
type WatermarkFont struct {
	Color      render.RGBA
	HasColor   bool
	FontSize   float64
	FontWeight string
	FontFamily string
	FontStyle  string
	TextAlign  string
}

// WatermarkContentLine is one watermark text row with optional line font.
type WatermarkContentLine struct {
	Text    string
	Font    WatermarkFont
	HasFont bool
}

// Watermark decorates children with tiled text/image marks.
// Host size follows the child; the mark layer is hit-transparent.
type Watermark struct {
	host  *rendering.RenderBox
	mark  *rendering.RenderBox
	child rendering.RenderObject

	lines []WatermarkContentLine

	width, height       float64
	hasWidth, hasHeight bool
	rotate              float64
	gapX, gapY          float64
	offX, offY          float64
	hasOffset           bool
	font                WatermarkFont
	imageSrc            string
	imageW, imageH      int
	imageBuf            *render.ImageBuf
	imageReady          bool
	imageError          bool
	loading             bool
	inherit             bool
	zIndex              int
	onRemove            func()

	provider  *theme.Provider
	override  *theme.Tokens
	face      text.Face
	styleHook string
	ariaLabel string

	attached *scheduler.TickerRegistry
}

// NewWatermark builds a watermark wrapping child (nil allowed).
func NewWatermark(child rendering.RenderObject) *Watermark {
	w := &Watermark{
		rotate:  DefaultRotate,
		gapX:    DefaultGapX,
		gapY:    DefaultGapY,
		inherit: true,
		zIndex:  DefaultZIndex,
	}
	w.host = rendering.NewRenderBox()
	w.host.SetRepaintBoundary(true)
	w.mark = rendering.NewRenderBox()
	w.mark.SetRepaintBoundary(true)
	mark := w.mark
	owner := w
	mark.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
		owner.paintMarks(pc)
	}
	w.host.AddChild(w.mark)
	if child != nil {
		w.SetChild(child)
	}
	return w
}

// SetContent replaces content with a single text row.
func (w *Watermark) SetContent(s string) {
	if w == nil {
		return
	}
	if s == "" {
		w.lines = nil
	} else {
		w.lines = []WatermarkContentLine{{Text: s}}
	}
	w.dirtyMark()
}

// SetContentLines replaces content with per-line fonts.
func (w *Watermark) SetContentLines(lines ...WatermarkContentLine) {
	if w == nil {
		return
	}
	w.lines = append([]WatermarkContentLine(nil), lines...)
	w.dirtyMark()
}

// SetContentStrings replaces content with plain rows.
func (w *Watermark) SetContentStrings(lines ...string) {
	if w == nil {
		return
	}
	w.lines = nil
	for _, s := range lines {
		w.lines = append(w.lines, WatermarkContentLine{Text: s})
	}
	w.dirtyMark()
}

// SetChild swaps the wrapped content; mark stays on top.
func (w *Watermark) SetChild(n rendering.RenderObject) {
	if w == nil || w.host == nil || w.mark == nil {
		return
	}
	if w.child == n {
		return
	}
	if w.child != nil {
		w.host.RemoveChild(w.child)
	}
	w.child = n
	if n != nil {
		w.host.AddChild(n)
	}
	w.host.RemoveChild(w.mark)
	w.host.AddChild(w.mark)
}

// Child returns the wrapped content.
func (w *Watermark) Child() rendering.RenderObject {
	if w == nil {
		return nil
	}
	return w.child
}

// SetWidth sets explicit mark width (<=0 clears to measured).
func (w *Watermark) SetWidth(v float64) {
	if w == nil {
		return
	}
	if v <= 0 {
		w.hasWidth = false
		w.width = 0
	} else {
		w.hasWidth = true
		w.width = v
	}
	w.dirtyMark()
}

// SetHeight sets explicit mark height (<=0 clears to measured).
func (w *Watermark) SetHeight(v float64) {
	if w == nil {
		return
	}
	if v <= 0 {
		w.hasHeight = false
		w.height = 0
	} else {
		w.hasHeight = true
		w.height = v
	}
	w.dirtyMark()
}

// SetRotate sets the tilt angle in degrees.
func (w *Watermark) SetRotate(deg float64) {
	if w == nil {
		return
	}
	w.rotate = deg
	w.dirtyMark()
}

// SetZIndex sets the overlay order (kit paint order).
func (w *Watermark) SetZIndex(z int) {
	if w == nil {
		return
	}
	w.zIndex = z
	w.dirtyMark()
}

// SetGap sets tile spacing.
func (w *Watermark) SetGap(x, y float64) {
	if w == nil {
		return
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	w.gapX, w.gapY = x, y
	w.dirtyMark()
}

// SetOffset sets the top-left offset; unset reverts to gap/2.
func (w *Watermark) SetOffset(x, y float64) {
	if w == nil {
		return
	}
	w.offX, w.offY = x, y
	w.hasOffset = true
	w.dirtyMark()
}

// ClearOffset reverts offset to gap/2.
func (w *Watermark) ClearOffset() {
	if w == nil {
		return
	}
	w.hasOffset = false
	w.dirtyMark()
}

// SetFont replaces the global text style.
func (w *Watermark) SetFont(f WatermarkFont) {
	if w == nil {
		return
	}
	w.font = f
	w.dirtyMark()
}

// SetFontColor sets the global text color (A<=0 clears to theme).
func (w *Watermark) SetFontColor(c render.RGBA) {
	if w == nil {
		return
	}
	if c.A <= 0 {
		w.font.HasColor = false
		w.font.Color = render.RGBA{}
	} else {
		w.font.HasColor = true
		w.font.Color = c
	}
	w.dirtyMark()
}

// SetFontSize sets the global text size (<=0 clears to token).
func (w *Watermark) SetFontSize(s float64) {
	if w == nil {
		return
	}
	if s <= 0 {
		w.font.FontSize = 0
	} else {
		w.font.FontSize = s
	}
	w.dirtyMark()
}

// SetFontWeight stores the global weight (normal/bold/number as text).
func (w *Watermark) SetFontWeight(s string) {
	if w == nil {
		return
	}
	w.font.FontWeight = s
	w.dirtyMark()
}

// SetFontFamily stores the global family (sans-serif default).
func (w *Watermark) SetFontFamily(s string) {
	if w == nil {
		return
	}
	w.font.FontFamily = s
	w.dirtyMark()
}

// SetFontStyle stores the global style (none/normal/italic/oblique).
func (w *Watermark) SetFontStyle(s string) {
	if w == nil {
		return
	}
	w.font.FontStyle = s
	w.dirtyMark()
}

// SetTextAlign stores the global align (left/center/right/start/end).
func (w *Watermark) SetTextAlign(s string) {
	if w == nil {
		return
	}
	w.font.TextAlign = s
	w.dirtyMark()
}

// SetImage stores the image tag; pixels arrive via SetImagePixels.
func (w *Watermark) SetImage(src string) {
	if w == nil {
		return
	}
	w.imageSrc = src
	w.imageError = false
	if !w.imageReady {
		w.loading = true
	}
	w.dirtyMark()
}

// ImageSrc returns the image tag.
func (w *Watermark) ImageSrc() string {
	if w == nil {
		return ""
	}
	return w.imageSrc
}

// SetImagePixels installs host-decoded pixels and ends loading.
func (w *Watermark) SetImagePixels(pxW, pxH int, rgba []byte) {
	if w == nil {
		return
	}
	if pxW <= 0 || pxH <= 0 || len(rgba) != pxW*pxH*4 {
		w.imageError = true
		w.imageReady = false
		w.loading = false
		w.dirtyMark()
		return
	}
	buf, err := render.NewImageBuf(pxW, pxH, render.FormatRGBA8)
	if err != nil {
		w.imageError = true
		w.imageReady = false
		w.loading = false
		w.dirtyMark()
		return
	}
	copy(buf.Data(), rgba)
	if w.imageBuf != nil {
		w.imageBuf.Dispose()
	}
	w.imageBuf = buf
	w.imageW, w.imageH = pxW, pxH
	w.imageReady = true
	w.imageError = false
	w.loading = false
	w.dirtyMark()
}

// NotifyImageError marks the image failed; content falls back to text.
func (w *Watermark) NotifyImageError() {
	if w == nil {
		return
	}
	w.imageError = true
	w.imageReady = false
	w.loading = false
	w.dirtyMark()
}

// IsImageMode reports image marks win over text.
func (w *Watermark) IsImageMode() bool {
	return w != nil && w.imageReady && !w.imageError && w.imageBuf != nil && !w.imageBuf.Disposed()
}

// SetInherit toggles Wrap propagation to popup content.
func (w *Watermark) SetInherit(b bool) {
	if w == nil {
		return
	}
	w.inherit = b
}

// Inherit reports the inherit flag.
func (w *Watermark) Inherit() bool {
	return w == nil || w.inherit
}

// Wrap copies mark config onto a new instance for popup content.
func (w *Watermark) Wrap(child rendering.RenderObject) *Watermark {
	if w == nil {
		return NewWatermark(child)
	}
	nw := NewWatermark(child)
	nw.lines = append([]WatermarkContentLine(nil), w.lines...)
	nw.width, nw.height = w.width, w.height
	nw.hasWidth, nw.hasHeight = w.hasWidth, w.hasHeight
	nw.rotate = w.rotate
	nw.gapX, nw.gapY = w.gapX, w.gapY
	nw.offX, nw.offY = w.offX, w.offY
	nw.hasOffset = w.hasOffset
	nw.font = w.font
	nw.imageSrc = w.imageSrc
	nw.inherit = w.inherit
	nw.zIndex = w.zIndex
	nw.provider = w.provider
	nw.override = w.override
	nw.face = w.face
	nw.styleHook = w.styleHook
	nw.ariaLabel = w.ariaLabel
	if w.imageReady && w.imageBuf != nil && !w.imageBuf.Disposed() && w.imageW > 0 && w.imageH > 0 {
		if cp, err := render.NewImageBuf(w.imageW, w.imageH, render.FormatRGBA8); err == nil {
			copy(cp.Data(), w.imageBuf.Data())
			nw.imageBuf = cp
			nw.imageW, nw.imageH = w.imageW, w.imageH
			nw.imageReady = true
		}
	}
	return nw
}

// SetOnRemove installs the strip callback.
func (w *Watermark) SetOnRemove(fn func()) {
	if w == nil {
		return
	}
	w.onRemove = fn
}

// NotifyRemoved fires onRemove once per call.
func (w *Watermark) NotifyRemoved() {
	if w == nil || w.onRemove == nil {
		return
	}
	w.onRemove()
}

// SetLoading toggles the image loading ticker demand.
func (w *Watermark) SetLoading(b bool) {
	if w == nil {
		return
	}
	w.loading = b
	w.dirtyMark()
}

// Loading reports the loading flag.
func (w *Watermark) Loading() bool { return w != nil && w.loading }

// Attach registers the loading ticker.
func (w *Watermark) Attach(reg *scheduler.TickerRegistry) {
	if w == nil || reg == nil {
		return
	}
	if w.attached != nil && w.attached != reg {
		w.attached.Remove(w)
	}
	w.attached = reg
	reg.Add(w)
}

// Detach unregisters the loading ticker.
func (w *Watermark) Detach() {
	if w == nil || w.attached == nil {
		return
	}
	w.attached.Remove(w)
	w.attached = nil
}

// Tick keeps registration; frames follow WantsFrame.
func (w *Watermark) Tick(_ float64) bool { return w != nil }

// WantsFrame demands frames while image loading.
func (w *Watermark) WantsFrame() bool { return w != nil && w.loading }

// SetProvider selects the theme source (nil selects process default).
func (w *Watermark) SetProvider(p *theme.Provider) {
	if w == nil {
		return
	}
	w.provider = p
	w.dirtyMark()
}

// SetTheme pins exact tokens (nil clears to provider).
func (w *Watermark) SetTheme(t *theme.Tokens) {
	if w == nil {
		return
	}
	w.override = t
	w.dirtyMark()
}

// SetFace sets the text face for measure/draw.
func (w *Watermark) SetFace(f text.Face) {
	if w == nil {
		return
	}
	w.face = f
	w.dirtyMark()
}

// SetStyle stores the semantic hook (no CSS engine).
func (w *Watermark) SetStyle(s string) {
	if w == nil {
		return
	}
	w.styleHook = s
}

// Style returns the stored hook.
func (w *Watermark) Style() string {
	if w == nil {
		return ""
	}
	return w.styleHook
}

// SetAriaLabel sets the optional accessible name.
func (w *Watermark) SetAriaLabel(s string) {
	if w == nil {
		return
	}
	w.ariaLabel = s
}

// AriaLabel returns the accessible name ("" means decorative).
func (w *Watermark) AriaLabel() string {
	if w == nil {
		return ""
	}
	return w.ariaLabel
}

// Role is presentation: watermark never takes action semantics.
func (w *Watermark) Role() string { return "presentation" }

// Focusable is always false: decoration never takes Tab.
func (w *Watermark) Focusable() bool { return false }

// Decorative reports the decoration flag (always true).
func (w *Watermark) Decorative() bool { return true }

// Node returns the tree node (layout/paint/hit through it).
func (w *Watermark) Node() rendering.RenderObject {
	if w == nil {
		return nil
	}
	return w.host
}

// Layout sizes the host (size follows child; mark never expands it).
func (w *Watermark) Layout(c rendering.Constraints) rendering.Size {
	if w == nil || w.host == nil {
		return rendering.Size{}
	}
	return w.host.Layout(c)
}

// HasMark reports visible marks (text rows or ready image).
func (w *Watermark) HasMark() bool {
	if w == nil {
		return false
	}
	if w.IsImageMode() {
		return true
	}
	for _, ln := range w.lines {
		if ln.Text != "" {
			return true
		}
	}
	return false
}

// ContentLines returns a copy of the text rows.
func (w *Watermark) ContentLines() []WatermarkContentLine {
	if w == nil {
		return nil
	}
	return append([]WatermarkContentLine(nil), w.lines...)
}

// ResolvedRotate returns the tilt angle.
func (w *Watermark) ResolvedRotate() float64 {
	if w == nil {
		return DefaultRotate
	}
	return w.rotate
}

// ResolvedGap returns tile spacing.
func (w *Watermark) ResolvedGap() (x, y float64) {
	if w == nil {
		return DefaultGapX, DefaultGapY
	}
	return w.gapX, w.gapY
}

// ResolvedOffset returns the top-left offset (gap/2 when unset).
func (w *Watermark) ResolvedOffset() (x, y float64) {
	if w == nil {
		return DefaultGapX / 2, DefaultGapY / 2
	}
	if w.hasOffset {
		return w.offX, w.offY
	}
	return w.gapX / 2, w.gapY / 2
}

// ResolvedZIndex returns the overlay order.
func (w *Watermark) ResolvedZIndex() int {
	if w == nil {
		return DefaultZIndex
	}
	return w.zIndex
}

// ResolvedInherit returns the inherit flag.
func (w *Watermark) ResolvedInherit() bool { return w == nil || w.inherit }

func (w *Watermark) themeTokens() theme.Tokens {
	if w != nil && w.override != nil {
		return *w.override
	}
	if w != nil && w.provider != nil {
		return w.provider.Current()
	}
	return theme.Default.Current()
}

// EffectiveFontSize returns the global text size.
func (w *Watermark) EffectiveFontSize() float64 {
	if w != nil && w.font.FontSize > 0 {
		return w.font.FontSize
	}
	tok := w.themeTokens()
	if tok.FontSizeLG > 0 {
		return tok.FontSizeLG
	}
	return DefaultFontSize
}

// EffectiveFontColor returns the global text color.
func (w *Watermark) EffectiveFontColor() render.RGBA {
	if w != nil && w.font.HasColor {
		return w.font.Color
	}
	tok := w.themeTokens()
	return render.RGBA{R: tok.ColorFill.R, G: tok.ColorFill.G, B: tok.ColorFill.B, A: tok.ColorFill.A}
}

// EffectiveFontWeight returns the global weight ("" means normal).
func (w *Watermark) EffectiveFontWeight() string {
	if w != nil && w.font.FontWeight != "" {
		return w.font.FontWeight
	}
	return "normal"
}

// EffectiveFontFamily returns the global family.
func (w *Watermark) EffectiveFontFamily() string {
	if w != nil && w.font.FontFamily != "" {
		return w.font.FontFamily
	}
	return DefaultFontFamily
}

// EffectiveFontStyle returns the global style.
func (w *Watermark) EffectiveFontStyle() string {
	if w != nil && w.font.FontStyle != "" {
		return w.font.FontStyle
	}
	return DefaultFontStyle
}

// EffectiveTextAlign returns the global align.
func (w *Watermark) EffectiveTextAlign() string {
	if w != nil && w.font.TextAlign != "" {
		return w.font.TextAlign
	}
	return DefaultTextAlign
}

func (w *Watermark) lineFontSize(ln WatermarkContentLine) float64 {
	if ln.HasFont && ln.Font.FontSize > 0 {
		return ln.Font.FontSize
	}
	return w.EffectiveFontSize()
}

func (w *Watermark) lineColor(ln WatermarkContentLine) render.RGBA {
	if ln.HasFont && ln.Font.HasColor {
		return ln.Font.Color
	}
	return w.EffectiveFontColor()
}

// ResolvedMarkSize returns one tile size (image or measured text).
func (w *Watermark) ResolvedMarkSize() (mw, mh float64) {
	if w == nil {
		return 0, 0
	}
	if w.IsImageMode() {
		if w.hasWidth {
			mw = w.width
		} else if w.imageW > 0 {
			mw = float64(w.imageW)
		} else {
			mw = DefaultWidth
		}
		if w.hasHeight {
			mh = w.height
		} else if w.imageH > 0 {
			mh = float64(w.imageH)
		} else {
			mh = DefaultHeight
		}
		return mw, mh
	}
	if !w.HasMark() {
		return 0, 0
	}
	if w.hasWidth {
		mw = w.width
	} else {
		for _, ln := range w.lines {
			if ln.Text == "" {
				continue
			}
			lw, _ := rendering.EstimateTextSize(ln.Text, w.lineFontSize(ln), approxCharW)
			if lw > mw {
				mw = lw
			}
		}
	}
	if w.hasHeight {
		mh = w.height
	} else {
		rows := 0
		for _, ln := range w.lines {
			if ln.Text == "" {
				continue
			}
			mh += w.lineFontSize(ln) * 1.25
			rows++
		}
		if rows > 1 {
			mh += float64(rows-1) * FontGap
		}
	}
	return mw, mh
}

// TiledCount counts tiles that fit in areaW×areaH.
func (w *Watermark) TiledCount(areaW, areaH float64) int {
	if w == nil || !w.HasMark() {
		return 0
	}
	mw, mh := w.ResolvedMarkSize()
	if mw <= 0 || mh <= 0 || areaW <= 0 || areaH <= 0 {
		return 0
	}
	gx, gy := w.ResolvedGap()
	ox, oy := w.ResolvedOffset()
	n := 0
	for y := oy; y < areaH; y += mh + gy {
		for x := ox; x < areaW; x += mw + gx {
			n++
		}
	}
	return n
}

func (w *Watermark) dirtyMark() {
	if w == nil || w.mark == nil {
		return
	}
	w.mark.MarkNeedsPaint()
}

// paintMarks tiles rotated text/image marks over the host area.
// The mark box itself stays 0x0 so the host size follows the child;
// tiling reads the host size, and the 0x0 box keeps HitTest empty.
func (w *Watermark) paintMarks(pc *rendering.PaintContext) {
	if pc == nil || w == nil || !w.HasMark() {
		return
	}
	hostW, hostH := 0.0, 0.0
	if w.host != nil {
		sz := w.host.Size()
		hostW, hostH = sz.Width, sz.Height
	}
	if hostW <= 0 || hostH <= 0 {
		return
	}
	mw, mh := w.ResolvedMarkSize()
	if mw <= 0 || mh <= 0 {
		return
	}
	gx, gy := w.ResolvedGap()
	ox, oy := w.ResolvedOffset()
	angle := w.rotate * math.Pi / 180
	if w.IsImageMode() {
		for y := oy; y < hostH; y += mh + gy {
			for x := ox; x < hostW; x += mw + gx {
				pc.Save()
				pc.RotateAbout(angle, x+mw/2, y+mh/2)
				rendering.DrawImageBuf(pc, w.imageBuf, x, y, mw, mh)
				pc.RestoreCanvas()
			}
		}
		return
	}
	for y := oy; y < hostH; y += mh + gy {
		for x := ox; x < hostW; x += mw + gx {
			pc.Save()
			pc.RotateAbout(angle, x+mw/2, y+mh/2)
			w.paintTextTile(pc, x, y, mw)
			pc.RestoreCanvas()
		}
	}
}

func (w *Watermark) paintTextTile(pc *rendering.PaintContext, x, y, mw float64) {
	if pc == nil || w == nil {
		return
	}
	curY := y
	for _, ln := range w.lines {
		if ln.Text == "" {
			continue
		}
		fs := w.lineFontSize(ln)
		lh := fs * 1.25
		col := w.lineColor(ln)
		lw, _ := rendering.EstimateTextSize(ln.Text, fs, approxCharW)
		bx := x
		align := w.font.TextAlign
		if ln.HasFont && ln.Font.TextAlign != "" {
			align = ln.Font.TextAlign
		}
		if align == "" {
			align = DefaultTextAlign
		}
		switch align {
		case "center":
			bx = x + (mw-lw)/2
		case "right", "end":
			bx = x + mw - lw
		}
		if pc.DC != nil {
			// Real glyphs only: without a face stay empty, never a bar.
			if face := w.face; face != nil {
				pc.DC.SetFont(face)
			}
			if pc.DC.Font() != nil {
				pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
				ax, ay := pc.Abs(bx, curY+fs)
				pc.DC.DrawString(ln.Text, ax, ay)
			}
		}
		curY += lh + FontGap
	}
}
