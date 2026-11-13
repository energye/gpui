package rendering

import "github.com/energye/gpui/render/text"

// LineSource supplies logical lines without requiring the caller to
// materialize the whole document (buffer-backed in production).
type LineSource interface {
	LineCount() int
	LineAt(k int) string
	LineStart(k int) int // byte offset of line k in the document
}

// ViewportLine is one lazily materialized logical line.
type ViewportLine struct {
	Start, End    int
	Width, Height float64
	Layout        *TextLayout // single-line layout for paint/queries
}

// TextViewport materializes only the lines touched by the viewport and keeps
// a bounded cache. V2a supports the no-wrap model (one source line == one
// visual line); wrapped/variable-height lines are V3.
type TextViewport struct {
	src       LineSource
	face      text.Face
	fontSize  float64
	spacing   float64
	heights   *VirtualTextLines
	cache     map[int]ViewportLine
	maxCache  int
	lineH     float64
	lastFirst int
	lastLast  int
}

// NewTextViewport builds a viewport view over src. maxCache bounds the
// materialized line cache (0 uses 512).
func NewTextViewport(src LineSource, face text.Face, fontSize, lineSpacing float64, maxCache int) *TextViewport {
	if fontSize <= 0 {
		fontSize = 14
	}
	if lineSpacing <= 0 {
		lineSpacing = 1.2
	}
	if maxCache <= 0 {
		maxCache = 512
	}
	lh := lineHeightFor(face, fontSize, lineSpacing)
	n := 0
	if src != nil {
		n = src.LineCount()
	}
	return &TextViewport{
		src:      src,
		face:     face,
		fontSize: fontSize,
		spacing:  lineSpacing,
		heights:  NewVirtualTextLines(n, lh),
		cache:    make(map[int]ViewportLine),
		maxCache: maxCache,
		lineH:    lh,
	}
}

func (v *TextViewport) LineCount() int {
	if v == nil || v.heights == nil {
		return 0
	}
	return v.heights.LineCount()
}

func (v *TextViewport) TotalHeight() float64 {
	if v == nil || v.heights == nil {
		return 0
	}
	return v.heights.TotalHeight()
}

func (v *TextViewport) LineTop(i int) float64 {
	if v == nil || v.heights == nil {
		return 0
	}
	return v.heights.OffsetOf(i)
}

// VisibleRange maps a Y band to logical line indices (inclusive).
func (v *TextViewport) VisibleRange(scrollY, viewportH float64) (first, last int) {
	n := v.LineCount()
	if n == 0 || viewportH <= 0 {
		return 0, -1
	}
	first = v.heights.IndexAtOffset(scrollY)
	last = v.heights.IndexAtOffset(scrollY + viewportH)
	if last < first {
		last = first
	}
	if last >= n {
		last = n - 1
	}
	return first, last
}

// Line returns the lazily materialized line i (false when out of range).
func (v *TextViewport) Line(i int) (ViewportLine, bool) {
	if v == nil || i < 0 || i >= v.LineCount() {
		return ViewportLine{}, false
	}
	if l, ok := v.cache[i]; ok {
		return l, true
	}
	if len(v.cache) >= v.maxCache {
		clear(v.cache)
	}
	s := v.src.LineAt(i)
	lay := BuildTextLayout(s, v.face, v.fontSize, 0, v.spacing)
	w, h := 0.0, v.lineH
	if lay != nil && lay.LineCount() > 0 {
		_, _, w, h, _ = lay.Line(0)
		if h <= 0 {
			h = v.lineH
		}
	}
	l := ViewportLine{
		Start:  v.src.LineStart(i),
		End:    v.src.LineStart(i) + len(s),
		Width:  w,
		Height: h,
		Layout: lay,
	}
	v.cache[i] = l
	return l, true
}

// Prepare materializes the visible window (plus one window of overscan on
// each side) and drops lines outside it, so memory stays O(visible).
func (v *TextViewport) Prepare(scrollY, viewportH float64) (first, last int) {
	first, last = v.VisibleRange(scrollY, viewportH)
	if last < first {
		return 0, -1
	}
	margin := last - first + 1
	lo, hi := first-margin, last+margin
	if lo < 0 {
		lo = 0
	}
	if hi >= v.LineCount() {
		hi = v.LineCount() - 1
	}
	for i := first; i <= last; i++ {
		_, _ = v.Line(i)
	}
	for k := range v.cache {
		if k < lo || k > hi {
			delete(v.cache, k)
		}
	}
	v.lastFirst, v.lastLast = first, last
	return first, last
}

// CacheLen reports the number of materialized lines (tests/observability).
func (v *TextViewport) CacheLen() int {
	if v == nil {
		return 0
	}
	return len(v.cache)
}

// Invalidate drops all materialized lines (e.g. after an edit).
func (v *TextViewport) Invalidate() {
	if v != nil {
		clear(v.cache)
	}
}

// InvalidateLine drops one materialized line after a localized edit.
func (v *TextViewport) InvalidateLine(i int) {
	if v != nil {
		delete(v.cache, i)
	}
}

// LineAtY maps a content Y to a line index (clamped).
func (v *TextViewport) LineAtY(y float64) int {
	if v == nil || v.heights == nil {
		return 0
	}
	return v.heights.IndexAtOffset(y)
}
