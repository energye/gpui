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
	Rows          int         // visual rows after wrap (0 for an empty line)
	Layout        *TextLayout // per-line layout for paint/queries
}

// TextViewport materializes only the lines touched by the viewport and keeps
// a bounded cache. Line heights live in the sparse HeightMap: measured lines
// are overrides, everything else uses one estimate, so memory stays
// O(visible) instead of O(lines). With maxWidth > 0 a logical line wraps into
// several visual rows (Layout.LineCount) and its measured height is the full
// wrapped height.
type TextViewport struct {
	src       LineSource
	face      text.Face
	fontSize  float64
	spacing   float64
	heights   *HeightMap
	cache     map[int]ViewportLine
	maxCache  int
	lineH     float64
	maxWidth  float64
	estimate  float64
	lastFirst int
	lastLast  int
}

// NewTextViewport builds a viewport view over src. maxCache bounds the
// materialized line cache (0 uses 512).
func NewTextViewport(src LineSource, face text.Face, fontSize, lineSpacing float64, maxCache int) *TextViewport {
	return NewWrappedTextViewport(src, face, fontSize, lineSpacing, 0, 0, maxCache)
}

// NewWrappedTextViewport builds a viewport whose logical lines may wrap into
// multiple visual rows (maxWidth > 0). Unmeasured logical lines use
// estimateHeight (<=0 uses one line height); visible lines are measured on
// first materialization and stored in the sparse HeightMap.
func NewWrappedTextViewport(src LineSource, face text.Face, fontSize, lineSpacing, maxWidth, estimateHeight float64, maxCache int) *TextViewport {
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
	if estimateHeight <= 0 {
		estimateHeight = lh
	}
	n := 0
	if src != nil {
		n = src.LineCount()
	}
	return &TextViewport{
		src:      src,
		face:     face,
		fontSize: fontSize,
		spacing:  lineSpacing,
		heights:  NewHeightMap(n, estimateHeight, 32768),
		cache:    make(map[int]ViewportLine),
		maxCache: maxCache,
		lineH:    lh,
		maxWidth: maxWidth,
		estimate: estimateHeight,
	}
}

func (v *TextViewport) LineCount() int {
	if v == nil {
		return 0
	}
	v.syncCount()
	if v.heights == nil {
		return 0
	}
	return v.heights.LineCount()
}

func (v *TextViewport) TotalHeight() float64 {
	if v == nil {
		return 0
	}
	v.syncCount()
	if v.heights == nil {
		return 0
	}
	return v.heights.TotalHeight()
}

func (v *TextViewport) LineTop(i int) float64 {
	if v == nil {
		return 0
	}
	v.syncCount()
	if v.heights == nil {
		return 0
	}
	return v.heights.OffsetOf(i)
}

// MeasuredCount reports how many line heights are stored as overrides
// (observability/tests: bounded by the HeightMap cap).
func (v *TextViewport) MeasuredCount() int {
	if v == nil || v.heights == nil {
		return 0
	}
	return v.heights.MeasuredCount()
}

// syncCount keeps the height map in step with a live line source (edits can
// add/remove lines). A count change invalidates geometry, so measured
// overrides and the materialized cache are dropped.
func (v *TextViewport) syncCount() {
	if v == nil || v.src == nil {
		return
	}
	n := v.src.LineCount()
	old := 0
	cap := 0
	if v.heights != nil {
		old = v.heights.LineCount()
		cap = v.heights.MeasuredCap()
	}
	if v.heights != nil && old == n {
		return
	}
	v.heights = NewHeightMap(n, v.estimate, cap)
	clear(v.cache)
}

// MaxWidth reports the wrap width (0 = no wrap).
func (v *TextViewport) MaxWidth() float64 {
	if v == nil {
		return 0
	}
	return v.maxWidth
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
	if v == nil {
		return ViewportLine{}, false
	}
	v.syncCount()
	if i < 0 || i >= v.LineCount() {
		return ViewportLine{}, false
	}
	if l, ok := v.cache[i]; ok {
		return l, true
	}
	if len(v.cache) >= v.maxCache {
		clear(v.cache)
	}
	s := v.src.LineAt(i)
	lay := BuildTextLayout(s, v.face, v.fontSize, v.maxWidth, v.spacing)
	w, h, rows := 0.0, v.estimate, 0
	if lay != nil && lay.LineCount() > 0 {
		rows = lay.LineCount()
		for r := 0; r < rows; r++ {
			if _, _, rw, _, ok := lay.Line(r); ok && rw > w {
				w = rw
			}
		}
		if lastH := lay.LineHeight(rows - 1); lastH > 0 {
			h = lay.LineTop(rows-1) + lastH
		}
		if h <= 0 {
			h = v.estimate
		}
	}
	v.heights.Measure(i, h)
	start := v.src.LineStart(i)
	l := ViewportLine{
		Start:  start,
		End:    start + len(s),
		Width:  w,
		Height: h,
		Rows:   rows,
		Layout: lay,
	}
	v.cache[i] = l
	return l, true
}

// Prepare materializes the visible window and returns the line range covering
// [scrollY, scrollY+viewportH]. Measuring wrapped lines changes their heights
// (and total height), so after each pass the band is re-derived; when the map
// still under-covers the band (estimates disagree with measured heights) the
// window grows one line at a time. Every line is at least one row tall, so at
// most viewportH/lineH extra steps are needed. Lines outside the window plus
// one window of overscan are dropped, keeping memory O(visible).
func (v *TextViewport) Prepare(scrollY, viewportH float64) (first, last int) {
	n := v.LineCount()
	if n == 0 || viewportH <= 0 {
		return 0, -1
	}
	first, last = v.VisibleRange(scrollY, viewportH)
	if last < first {
		return 0, -1
	}
	bottom := scrollY + viewportH
	maxSteps := 8 + int(viewportH/v.lineH) + 8
	if maxSteps < 16 {
		maxSteps = 16
	}
	for step := 0; step < maxSteps; step++ {
		for i := first; i <= last; i++ {
			_, _ = v.Line(i)
		}
		nf, nl := v.VisibleRange(scrollY, viewportH)
		if nl < nf {
			break
		}
		if nf != first || nl != last {
			// Measured heights moved the band: retarget the window instead of
			// keeping the stale estimate-based one (an estimate that is too
			// small would otherwise paint many screens of off-band rows).
			first, last = nf, nl
			continue
		}
		endLine, ok := v.Line(last)
		if ok && last < n-1 && v.LineTop(last)+endLine.Height < bottom {
			last++
			continue
		}
		if first > 0 && v.LineTop(first) > scrollY {
			first--
			continue
		}
		break
	}
	margin := last - first + 1
	lo, hi := first-margin, last+margin
	if lo < 0 {
		lo = 0
	}
	if hi >= n {
		hi = n - 1
	}
	for k := range v.cache {
		if k < lo || k > hi {
			delete(v.cache, k)
		}
	}
	v.lastFirst, v.lastLast = first, last
	return first, last
}

// LineRowText returns the source text of visual row r inside a materialized
// line (empty when the row is out of range). Wrapped paint uses one row at a
// time: shaped rows index Layout.LineGlyphs(r), fallback rows draw this text.
func LineRowText(lay *TextLayout, r int) string {
	if lay == nil || r < 0 || r >= lay.LineCount() {
		return ""
	}
	s, e, _, _, ok := lay.Line(r)
	if !ok {
		return ""
	}
	if s < 0 {
		s = 0
	}
	if e > len(lay.Text) {
		e = len(lay.Text)
	}
	if s >= e {
		return ""
	}
	return lay.Text[s:e]
}

// CacheLen reports the number of materialized lines (tests/observability).
func (v *TextViewport) CacheLen() int {
	if v == nil {
		return 0
	}
	return len(v.cache)
}

// Invalidate drops all materialized lines and measured heights (e.g. after an
// edit: wrapped line heights depend on content).
func (v *TextViewport) Invalidate() {
	if v == nil {
		return
	}
	clear(v.cache)
	n := 0
	cap := 0
	if v.src != nil {
		n = v.src.LineCount()
	}
	if v.heights != nil {
		cap = v.heights.MeasuredCap()
	}
	v.heights = NewHeightMap(n, v.estimate, cap)
}

// InvalidateLine drops one materialized line after a localized edit,
// returning its height to the estimate until it is measured again.
func (v *TextViewport) InvalidateLine(i int) {
	if v == nil {
		return
	}
	delete(v.cache, i)
	if v.heights != nil {
		v.heights.Reset(i)
	}
}

// LineAtY maps a content Y to a line index (clamped).
func (v *TextViewport) LineAtY(y float64) int {
	if v == nil {
		return 0
	}
	v.syncCount()
	if v.heights == nil {
		return 0
	}
	return v.heights.IndexAtOffset(y)
}
