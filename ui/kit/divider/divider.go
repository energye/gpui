// Package divider implements the Divider control (docs/antd/divider.md §6).
//
// Composition over new frameworks: the widget owns a rendering.RenderBox
// node (Node) and reuses ui/rendering for draw, ui/theme for color.
// No new event or frame system.
package divider

import (
	"sync/atomic"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Orientation selects horizontal or vertical layout.
type DividerOrientation int

const (
	Horizontal DividerOrientation = iota
	Vertical
)

// Size selects the horizontal marginBlock档位.
type DividerSize int

const (
	SizeUnset DividerSize = iota
	Small
	Medium
	Large
)

// Middle aliases Medium (antd compat).
const Middle = Medium

// Variant selects solid, dashed or dotted rails.
type DividerVariant int

const (
	Solid DividerVariant = iota
	Dashed
	Dotted
)

// TitlePlacement selects title position in with-text mode.
type DividerTitlePlacement int

const (
	Center DividerTitlePlacement = iota
	Start
	End
)

// Left aliases Start, Right aliases End (antd compat).
const (
	Left  = Start
	Right = End
)

// Style overrides line/title appearance for one divider.
// Border overrides the rail color, Text overrides the title color,
// FontSize overrides the title size. Zero values mean theme.
type Style struct {
	Border   render.RGBA
	Text     render.RGBA
	FontSize float64
}

// Divider is the Divider widget (docs/antd/divider.md §6.10).
//
// It owns a rendering.RenderBox node: put Node() in the tree and read
// Effective* for assertions. Rebuild replaces the subtree, Node stays stable.
type Divider struct {
	orientation    DividerOrientation
	orientationSet bool
	vertical       bool
	size           DividerSize
	variant        DividerVariant
	dashed         bool
	plain          bool
	title          string
	titleNode      rendering.RenderObject
	placement      DividerTitlePlacement
	orientMargin   float64
	style          Style
	face           text.Face
	provider       *theme.Provider
	override       *theme.Tokens
	ariaLabel      string
	root           *rendering.RenderBox

	// layoutCache is the last laid-out geometry, stored wholesale by Layout
	// (UI) and loaded once per use on either thread (paint reads on raster):
	// atomic, never nine bare floats — a torn half-layout on raster is a
	// wrong rail, not just a race report (R2-6).
	layoutCache atomic.Value // dividerLayout
}

// dividerLayout is one consistent laid-out geometry snapshot.
type dividerLayout struct {
	w, h                  float64
	titleW, titleH        float64
	railStart, railEnd    float64
	titleX, titleY, railY float64
}

// layoutSnap loads the cached geometry (zero before first layout).
func (d *Divider) layoutSnap() dividerLayout {
	if d == nil {
		return dividerLayout{}
	}
	if v, ok := d.layoutCache.Load().(dividerLayout); ok {
		return v
	}
	return dividerLayout{}
}

// NewDivider creates a horizontal solid divider.
func NewDivider() *Divider {
	d := &Divider{placement: Center}
	root := rendering.NewRenderBox()
	root.SetRepaintBoundary(true)
	root.SetRelayoutBoundary(true)
	d.root = root
	div := d
	root.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		div.paint(pc, size)
	}
	d.rebuild()
	return d
}

// NewDividerWithTitle creates a divider with a text title.
func NewDividerWithTitle(title string) *Divider {
	d := NewDivider()
	d.SetTitle(title)
	return d
}

// SetOrientation sets horizontal or vertical (wins over SetVertical).
func (d *Divider) SetOrientation(o DividerOrientation) {
	if d == nil {
		return
	}
	d.orientation = o
	d.orientationSet = true
	d.rebuild()
}

// SetVertical sets legacy vertical flag (ignored when orientation set).
func (d *Divider) SetVertical(b bool) {
	if d == nil {
		return
	}
	d.vertical = b
	d.rebuild()
}

// SetSize sets the horizontal marginBlock档位.
func (d *Divider) SetSize(s DividerSize) {
	if d == nil {
		return
	}
	d.size = s
	d.rebuild()
}

// SetVariant sets solid/dashed/dotted (paint only).
func (d *Divider) SetVariant(v DividerVariant) {
	if d == nil {
		return
	}
	d.variant = v
	d.markPaint()
}

// SetDashed toggles dashed sugar (paint only).
func (d *Divider) SetDashed(b bool) {
	if d == nil {
		return
	}
	d.dashed = b
	d.markPaint()
}

// SetPlain toggles plain title style (affects title size).
func (d *Divider) SetPlain(b bool) {
	if d == nil {
		return
	}
	d.plain = b
	d.rebuild()
}

// SetTitle sets the text title (empty means plain line).
func (d *Divider) SetTitle(s string) {
	if d == nil {
		return
	}
	d.title = s
	d.rebuild()
}

// SetTitleNode sets a custom title node (wins over Title string).
func (d *Divider) SetTitleNode(n rendering.RenderObject) {
	if d == nil {
		return
	}
	d.titleNode = n
	d.rebuild()
}

// SetTitlePlacement sets start/center/end placement.
func (d *Divider) SetTitlePlacement(p DividerTitlePlacement) {
	if d == nil {
		return
	}
	d.placement = p
	d.rebuild()
}

// SetOrientationMargin sets start/end near-rail ratio (<=0 selects 0.05).
func (d *Divider) SetOrientationMargin(ratio float64) {
	if d == nil {
		return
	}
	d.orientMargin = ratio
	d.rebuild()
}

// SetTheme pins exact tokens (nil clears to provider).
func (d *Divider) SetTheme(t *theme.Tokens) {
	if d == nil {
		return
	}
	if t == nil {
		d.override = nil
	} else {
		cp := *t
		d.override = &cp
	}
	d.markPaint()
}

// SetProvider selects the theme source (nil selects process default).
func (d *Divider) SetProvider(p *theme.Provider) {
	if d == nil {
		return
	}
	d.provider = p
	d.markPaint()
}

// SetStyle overrides line/title appearance.
func (d *Divider) SetStyle(s Style) {
	if d == nil {
		return
	}
	needLayout := s.FontSize != d.style.FontSize
	d.style = s
	if needLayout {
		d.rebuild()
		return
	}
	d.markPaint()
}

// SetFace sets the title font face.
func (d *Divider) SetFace(f text.Face) {
	if d == nil {
		return
	}
	d.face = f
	d.rebuild()
}

// SetAriaLabel sets the accessible name (empty keeps decorative).
func (d *Divider) SetAriaLabel(s string) {
	if d == nil {
		return
	}
	d.ariaLabel = s
}

// Title returns the text title.
func (d *Divider) Title() string {
	if d == nil {
		return ""
	}
	return d.title
}

// Plain reports the plain flag.
func (d *Divider) Plain() bool { return d != nil && d.plain }

// Size returns the size档位.
func (d *Divider) Size() DividerSize {
	if d == nil {
		return SizeUnset
	}
	return d.size
}

// Variant returns the raw variant.
func (d *Divider) Variant() DividerVariant {
	if d == nil {
		return Solid
	}
	return d.variant
}

// Dashed reports the dashed sugar flag.
func (d *Divider) Dashed() bool { return d != nil && d.dashed }

// TitlePlacement returns the title placement.
func (d *Divider) TitlePlacement() DividerTitlePlacement {
	if d == nil {
		return Center
	}
	return d.placement
}

// OrientationMargin returns the raw ratio (<=0 means default 0.05).
func (d *Divider) OrientationMargin() float64 {
	if d == nil {
		return 0
	}
	return d.orientMargin
}

// AriaLabel returns the accessible name.
func (d *Divider) AriaLabel() string {
	if d == nil {
		return ""
	}
	return d.ariaLabel
}

// Role returns separator for the root (decorative divider).
func (d *Divider) Role() string { return "separator" }

// Focusable is always false: divider never takes Tab.
func (d *Divider) Focusable() bool { return false }

// HasTitle reports whether a title is present (custom node wins).
// Vertical dividers ignore titles per antd.
func (d *Divider) HasTitle() bool {
	if d == nil || d.IsVertical() {
		return false
	}
	return d.titleNode != nil || d.title != ""
}

// EffectiveOrientation resolves orientation (orientation wins over vertical).
func (d *Divider) EffectiveOrientation() DividerOrientation {
	if d != nil && d.orientationSet {
		return d.orientation
	}
	if d != nil && d.vertical {
		return Vertical
	}
	return Horizontal
}

// EffectiveVariant resolves dotted > dashed|Dashed > solid.
func (d *Divider) EffectiveVariant() DividerVariant {
	if d != nil && d.variant == Dotted {
		return Dotted
	}
	if d != nil && (d.variant == Dashed || d.dashed) {
		return Dashed
	}
	return Solid
}

// IsVertical reports vertical layout.
func (d *Divider) IsVertical() bool { return d.EffectiveOrientation() == Vertical }

func (d *Divider) themeTokens() theme.Tokens {
	if d != nil && d.override != nil {
		return *d.override
	}
	if d != nil && d.provider != nil {
		return d.provider.Current()
	}
	return theme.Default.Current()
}

// MarginBlock returns horizontal上下外边距.
func (d *Divider) MarginBlock() float64 {
	if d != nil && d.IsVertical() {
		return 0
	}
	tok := d.themeTokens()
	marginXS, margin, marginLG := 8.0, 16.0, 24.0
	if tok.MarginXS > 0 {
		marginXS = tok.MarginXS
	}
	if tok.Margin > 0 {
		margin = tok.Margin
	}
	if tok.MarginLG > 0 {
		marginLG = tok.MarginLG
	}
	if d == nil {
		return marginLG
	}
	switch d.size {
	case Small:
		return marginXS
	case Medium:
		return margin
	case Large:
		return marginLG
	default:
		if d.HasTitle() {
			return margin
		}
		return marginLG
	}
}

// MarginInline returns vertical左右外边距.
func (d *Divider) MarginInline() float64 {
	tok := d.themeTokens()
	if tok.MarginXS > 0 {
		return tok.MarginXS
	}
	return 8
}

// LineWidth returns the rail thickness (theme lineWidth).
func (d *Divider) LineWidth() float64 {
	tok := d.themeTokens()
	if tok.LineWidth > 0 {
		return tok.LineWidth
	}
	return 1
}

// LineColor returns the resolved rail color (theme colorSplit fallback).
func (d *Divider) LineColor() render.RGBA {
	if d != nil && d.style.Border.A > 0 {
		return d.style.Border
	}
	tok := d.themeTokens()
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	if tok.ColorBorderSecondary.A > 0 {
		return toRGBA(tok.ColorBorderSecondary)
	}
	if tok.Border.A > 0 {
		return toRGBA(tok.Border)
	}
	return toRGBA(tok.ColorBorder)
}

// TitleFontSize returns the title size (plain 14, default 16).
func (d *Divider) TitleFontSize() float64 {
	if d != nil && d.style.FontSize > 0 {
		return d.style.FontSize
	}
	tok := d.themeTokens()
	if d != nil && d.plain {
		if tok.FontSize > 0 {
			return tok.FontSize
		}
		return 14
	}
	if tok.FontSizeLG > 0 {
		return tok.FontSizeLG
	}
	return 16
}

// TitleColor returns the resolved title color.
func (d *Divider) TitleColor() render.RGBA {
	if d != nil && d.style.Text.A > 0 {
		return d.style.Text
	}
	tok := d.themeTokens()
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	if d != nil && d.plain {
		return toRGBA(tok.ColorText)
	}
	if tok.ColorTextHeading.A > 0 {
		return toRGBA(tok.ColorTextHeading)
	}
	return toRGBA(tok.ColorText)
}

// OrientationMarginRatio returns start/end near-rail ratio (default 0.05).
func (d *Divider) OrientationMarginRatio() float64 {
	if d == nil || d.orientMargin <= 0 {
		return 0.05
	}
	r := d.orientMargin
	if r < 0 {
		return 0.05
	}
	if r > 1 {
		return 1
	}
	return r
}

// RailGrows returns start/end grow factors for with-text rails.
func (d *Divider) RailGrows() (start, end float64) {
	if d == nil {
		return 1, 1
	}
	switch d.placement {
	case Start:
		r := d.OrientationMarginRatio()
		return r, 1 - r
	case End:
		r := d.OrientationMarginRatio()
		return 1 - r, r
	default:
		return 1, 1
	}
}

// Node returns the tree node (always the same root).
func (d *Divider) Node() rendering.RenderObject {
	if d == nil {
		return nil
	}
	return d.root
}

// ChromeNode returns the rail or with-text root for tests.
func (d *Divider) ChromeNode() rendering.RenderObject {
	if d == nil {
		return nil
	}
	return d.root
}

// Layout sizes the node under constraints.
func (d *Divider) Layout(c rendering.Constraints) rendering.Size {
	if d == nil || d.root == nil {
		return rendering.Size{}
	}
	d.compute(c.MaxWidth)
	L := d.layoutSnap()
	d.root.FixedWidth = L.w
	d.root.FixedHeight = L.h
	sz := d.root.Layout(c)
	// Keep cached split consistent with the tightened width.
	if d.HasTitle() && sz.Width != L.w {
		d.resplit(sz.Width)
	}
	// RenderBox resets custom child offsets to Pad; restore title offset.
	if d.titleNode != nil {
		L = d.layoutSnap()
		d.titleNode.SetOffset(rendering.Point{X: L.titleX, Y: L.titleY})
	}
	return sz
}

// RailStartWidth returns the last laid-out start rail width.
func (d *Divider) RailStartWidth() float64 {
	if d == nil {
		return 0
	}
	return d.layoutSnap().railStart
}

// RailEndWidth returns the last laid-out end rail width.
func (d *Divider) RailEndWidth() float64 {
	if d == nil {
		return 0
	}
	return d.layoutSnap().railEnd
}

// TitleBlockWidth returns the last laid-out title block width.
func (d *Divider) TitleBlockWidth() float64 {
	if d == nil {
		return 0
	}
	return d.layoutSnap().titleW
}

func (d *Divider) rebuild() {
	if d == nil || d.root == nil {
		return
	}
	// Manage custom title child: string titles draw in OnPaint, no child.
	cur := d.root.Children()
	for _, c := range cur {
		if c != d.titleNode {
			d.root.RemoveChild(c)
		}
	}
	if d.titleNode != nil && d.HasTitle() {
		found := false
		for _, c := range d.root.Children() {
			if c == d.titleNode {
				found = true
				break
			}
		}
		if !found {
			d.root.AddChild(d.titleNode)
		}
		// Size the custom node loosely so measureTitle sees a size.
		d.titleNode.Layout(rendering.Constraints{
			MaxWidth:  rendering.Unbounded,
			MaxHeight: rendering.Unbounded,
		})
	}
	d.root.MarkNeedsLayout()
}

func (d *Divider) markPaint() {
	if d == nil || d.root == nil {
		return
	}
	d.root.MarkNeedsPaint()
}

func (d *Divider) compute(maxW float64) {
	if d == nil {
		return
	}
	if d.IsVertical() {
		tok := d.themeTokens()
		fs := tok.FontSize
		if fs <= 0 {
			fs = 14
		}
		lw := d.LineWidth()
		mi := d.MarginInline()
		d.layoutCache.Store(dividerLayout{
			w: 2*mi + lw, h: 0.9 * fs,
		})
		return
	}
	mb := d.MarginBlock()
	lw := d.LineWidth()
	if !d.HasTitle() {
		w := maxW
		if w <= 0 || w >= rendering.Unbounded/2 {
			w = 200
		}
		d.layoutCache.Store(dividerLayout{
			w: w, h: 2*mb + lw,
			railStart: w, railEnd: 0,
			railY: mb,
		})
		return
	}
	fs := d.TitleFontSize()
	tw, th := d.measureTitle(fs)
	pad := fs
	blockW := tw + 2*pad
	blockH := th
	if blockH < lw {
		blockH = lw
	}
	w := maxW
	if w <= 0 || w >= rendering.Unbounded/2 {
		w = blockW + 80
	}
	if w < blockW {
		w = blockW
	}
	d.layoutCache.Store(dividerLayout{
		w: w, h: 2*mb + blockH,
		titleW: blockW, titleH: blockH,
		titleY: mb + (blockH-th)/2,
		railY:  mb + (blockH-lw)/2,
	})
	d.resplit(w)
}

func (d *Divider) resplit(w float64) {
	// Rails derive from the stored title width (same-UI-thread as compute,
	// which stored just above); the merged snapshot keeps one consistent set.
	L := d.layoutSnap()
	avail := w - L.titleW
	if avail < 0 {
		avail = 0
	}
	gs, ge := d.RailGrows()
	sum := gs + ge
	if sum <= 0 {
		gs, ge, sum = 1, 1, 2
	}
	L.railStart = avail * gs / sum
	L.railEnd = avail * ge / sum
	L.titleX = L.railStart
	L.w = w
	d.layoutCache.Store(L)
}

func (d *Divider) measureTitle(fs float64) (w, h float64) {
	if d == nil || !d.HasTitle() {
		return 0, 0
	}
	if d.titleNode != nil {
		sz := d.titleNode.Layout(rendering.Constraints{
			MaxWidth:  rendering.Unbounded,
			MaxHeight: rendering.Unbounded,
		})
		return sz.Width, sz.Height
	}
	// True width when a face is set; estimate keeps headless layout stable.
	if d.face != nil {
		tw, th := text.Measure(d.title, d.face)
		if tw > 0 && th > 0 {
			if sz := d.face.Size(); sz > 0 && sz != fs {
				k := fs / sz
				tw *= k
				th *= k
			}
			return tw, th
		}
	}
	w, h = rendering.EstimateTextSize(d.title, fs, 0.55)
	if h <= 0 {
		h = fs * 1.25
	}
	return w, h
}
