package rendering

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TextOverflow controls how RenderText handles content that exceeds MaxWidth / MaxLines.
type TextOverflow int

const (
	// TextOverflowClip truncates extra lines/width without a visual marker (default).
	TextOverflowClip TextOverflow = iota
	// TextOverflowEllipsis truncates and appends "…" so layout stays within the budget.
	TextOverflowEllipsis
)

const textEllipsis = "…"

// RenderText draws text via render.DrawString / per-line paint.
// Layout prefers Face.Measure when Face is set; otherwise EstimateTextSize (rune-based).
// When MaxWidth > 0, layout/paint wrap. MaxLines + Overflow cap the visible content.
//
// Multi-run (minimal Paragraph): when Runs is non-empty, layout/paint use per-run
// style (color/face/size). Single-string Text path remains the default when Runs is empty.
type RenderText struct {
	Base
	Text       string
	FontSize   float64 // logical; used for estimate and as draw Y baseline offset
	R, G, B, A float64
	// ApproxCharW factor for layout without a Face (latin ~0.55; CJK often ~1).
	ApproxCharW float64
	// Face optional shaped face for true Measure (from render/text).
	Face text.Face
	// MaxWidth > 0 enables word-wrap layout/paint (logical px).
	MaxWidth float64
	// LineSpacing multiplier for wrapped lines (default 1.2).
	LineSpacing float64
	// Align for wrapped text (render.Align*); multi-line paint uses left for MVP.
	Align render.Align
	// MaxLines caps visible lines; 0 = unlimited.
	MaxLines int
	// Overflow is applied when content exceeds MaxWidth and/or MaxLines.
	Overflow TextOverflow
	// Runs holds multi-span content from ParagraphBuilder / SetRuns.
	// When len(Runs) > 0, Text is treated as a cache of concatenated plain text.
	Runs []TextRun
	// Decoration is a render.TextDecoration bitset for the single-string path
	// (and default when a run has Decoration=0 and inherits — see paintRuns).
	Decoration render.TextDecoration
	// FontFamily is the last family string passed to SetFontFamily (diagnostics).
	FontFamily string

	// measureCache maps measureLine keys → width (R9). Invalidated on text/face/size change.
	// Guarded by measureMu: layout (UI thread) and paint/record (raster
	// thread) measure concurrently, and an unguarded map fatals the runtime.
	measureMu    sync.Mutex
	measureCache map[string]float64
	measureHits  int64
	measureMiss  int64

	// textLayout is the single-source layout (R2). Nil = dirty.
	textLayout *TextLayout

	// lcache是行/段增量引擎(M1-d):单串路径经update增量重排,
	// 多run路径仍走全量BuildRenderTextLayout.
	lcache *layoutCache
	// effFace是effectiveFace的记忆值:face 对象不可变,输入不变时
	// 返回同一对象,增量引擎才能按身份命中(否则每击键全量重建)。
	effFace     text.Face
	effFaceFor  text.Face
	effFaceSize float64
	// spanHint是SetTextSpan留下的变更区间,ensureLayout消费一次.
	spanHint editSpan

	// viewportHint enables Flutter-like culling for 5000 single-line viewports:
	// only glyphs within [scrollX-margin, scrollX+visW+margin] are submitted to
	// the GPU. Set by ViewportInputBox after scroll; cleared for wrapped/multi-line.
	viewportScrollX float64
	viewportWidth   float64
	hasViewportHint bool
	// recordedBand is the scroll window the retained texture currently
	// holds (written at record time when a culled band is stored).
	// Scrolling past its margin must re-record: the viewport re-blit alone
	// reuses the old band at the new offset, and the visible window then
	// maps outside the recording (empty box after ~200px of scroll).
	// Unculled/full recordings never expire.
	recordedScrollX float64
	recordedVisW    float64
	recordedCulled  bool
	// recordedEver is set by the first dirty-build note write. Before any
	// texture exists (warm-up vector paint clears the fresh dirty flags
	// before the first retained record) the band is unknown and must be
	// established, not trusted.
	recordedEver bool
	// viewportHintY bounds the visible row band for wrapped/multi-line text
	// (M2 vertical culling). Unset = unbounded, paint skips no rows.
	viewportScrollY  float64
	viewportHeight   float64
	hasViewportHintY bool
}

// NewRenderText creates a text node.
func NewRenderText(textStr string) *RenderText {
	t := &RenderText{
		Text: textStr, FontSize: 14,
		R: 0.9, G: 0.9, B: 0.9, A: 1,
		ApproxCharW: 0.55,
		LineSpacing: 1.2,
		Align:       render.AlignLeft,
		Overflow:    TextOverflowClip,
	}
	t.Init(t)
	return t
}

// SetText updates the string and dirties layout+paint when changed.
// Clears multi-run content so the single-string path is used.
func (t *RenderText) SetText(s string) {
	if t == nil {
		return
	}
	if t.Text == s && len(t.Runs) == 0 {
		t.spanHint.ok = false
		return
	}
	t.Text = s
	t.Runs = nil
	t.spanHint.ok = false
	t.invalidateMeasureCache()
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// editSpan是InputBox.sync传来的击键变更区间(旧串→新串),免去引擎逐字节diff.
type editSpan struct {
	oldA, oldB, newA, newB int
	ok                     bool
}

// SetTextSpan同SetText,另带Editor记录的变更区间.区间经长度方程校验,
// 不一致回退普通SetText(保正确).调用方须保证区间描述的是本次变更
// (密码掩码/占位替换等变换后的串不得用此通道).
func (t *RenderText) SetTextSpan(s string, oldA, oldB, newA, newB int) {
	if t == nil {
		return
	}
	// 布局前连续两次SetTextSpan:第二区间相对中间串,与引擎live对不上,
	// 直接丢区间走普通通道(引擎抽查是第二道防线).
	if t.spanHint.ok {
		t.SetText(s)
		return
	}
	if oldA < 0 || oldB < oldA || oldB > len(t.Text) ||
		newA < 0 || newB < newA || newB > len(s) ||
		(newB-newA)-(oldB-oldA) != len(s)-len(t.Text) {
		t.SetText(s)
		return
	}
	if len(t.Runs) == 0 && oldA == oldB && newA == newB && t.Text == s {
		// 空区间=无变更才需整串确认;非空区间下串必然已变(撒谎区间由
		// 引擎checkSpan校验,结果仍正确),跳过O(n)整串比较(击键主瓶颈).
		t.spanHint.ok = false
		return
	}
	t.Text = s
	t.Runs = nil
	t.spanHint = editSpan{oldA: oldA, oldB: oldB, newA: newA, newB: newB, ok: true}
	t.invalidateMeasureCache()
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetRuns installs multi-span content (minimal paragraph). Copies runs.
// Updates Text to the concatenation of run texts for debugging / DisplayText.
func (t *RenderText) SetRuns(runs []TextRun) {
	if t == nil {
		return
	}
	if len(runs) == 0 {
		t.Runs = nil
		t.Text = ""
		t.MarkNeedsLayout()
		t.MarkNeedsPaint()
		return
	}
	cp := make([]TextRun, len(runs))
	copy(cp, runs)
	for i := range cp {
		if !cp[i].IsColor && isColorText(cp[i].Text) {
			cp[i].IsColor = true
		}
	}
	t.Runs = cp
	var b strings.Builder
	for i, r := range cp {
		if i > 0 {
			// no separator — runs are adjacent spans
		}
		b.WriteString(r.Text)
	}
	t.Text = b.String()
	t.invalidateMeasureCache()
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// RunCount returns the number of styled runs (0 = single-string mode).
func (t *RenderText) RunCount() int {
	if t == nil {
		return 0
	}
	return len(t.Runs)
}

// SetColor updates RGBA and dirties paint only (no layout).
func (t *RenderText) SetColor(r, g, b, a float64) {
	if t == nil {
		return
	}
	t.R, t.G, t.B, t.A = r, g, b, a
	t.MarkNeedsPaint()
}

// SetFace sets an optional font face for measure/draw alignment with render text.
// Layout and paint use Face scaled to FontSize when the face size differs
// (via Source().Face or MultiFace.AtSize).
func (t *RenderText) SetFace(face text.Face) {
	if t == nil {
		return
	}
	t.Face = face
	t.invalidateMeasureCache()
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetFontSize updates logical font size and dirties layout+paint.
// When Face is set, measure/paint re-derive a face at this size so layout
// tracks FontSize (not only the size baked into the Face at SetFace time).
func (t *RenderText) SetFontSize(points float64) {
	if t == nil {
		return
	}
	if points <= 0 {
		points = 14
	}
	if t.FontSize == points {
		return
	}
	t.FontSize = points
	t.invalidateMeasureCache()
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetFontFamily resolves family via LoadFaceByFamily and SetFace (FT-STYLE-FAMILY).
// Returns the load error (caller may ignore and keep heuristic layout).
func (t *RenderText) SetFontFamily(family string) error {
	if t == nil {
		return nil
	}
	face, _, err := LoadFaceByFamily(family, t.fontSize())
	if err != nil {
		return err
	}
	t.FontFamily = family
	t.SetFace(face)
	return nil
}

// SetDecoration sets text decorations for the single-string paint path (FT-DECORATION).
func (t *RenderText) SetDecoration(d render.TextDecoration) {
	if t == nil {
		return
	}
	t.Decoration = d
	t.MarkNeedsPaint()
}

// SetMaxWidth enables (>0) or disables (≤0) wrap; dirties layout+paint.
func (t *RenderText) SetMaxWidth(w float64) {
	if t == nil || t.MaxWidth == w {
		return
	}
	t.MaxWidth = w
	// Wrap budget changed; line splits change — drop width cache for safety.
	t.invalidateMeasureCache()
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetMaxLines caps visible lines (0 = unlimited); dirties layout+paint.
func (t *RenderText) SetMaxLines(n int) {
	if t == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	if t.MaxLines == n {
		return
	}
	t.MaxLines = n
	t.textLayout = nil
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetOverflow sets clip vs ellipsis overflow; dirties layout+paint.
func (t *RenderText) SetOverflow(o TextOverflow) {
	if t == nil || t.Overflow == o {
		return
	}
	t.Overflow = o
	t.textLayout = nil
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

func (t *RenderText) fontSize() float64 {
	if t == nil || t.FontSize <= 0 {
		return 14
	}
	return t.FontSize
}

func (t *RenderText) approxCharW() float64 {
	if t == nil || t.ApproxCharW <= 0 {
		return 0.55
	}
	return t.ApproxCharW
}

func (t *RenderText) lineSpacing() float64 {
	if t == nil || t.LineSpacing <= 0 {
		return 1.2
	}
	return t.LineSpacing
}

// faceForSize returns a Face usable at points for measure/paint.
// Single faces are re-derived from Source(); MultiFace uses AtSize.
// If size already matches (within 0.25pt) the input face is returned as-is.
func faceForSize(face text.Face, points float64) text.Face {
	if face == nil {
		return nil
	}
	if points <= 0 {
		points = 14
	}
	cur := face.Size()
	if cur > 0 {
		d := cur - points
		if d < 0 {
			d = -d
		}
		if d < 0.25 {
			return face
		}
	}
	if mf, ok := face.(*text.MultiFace); ok {
		return mf.AtSize(points)
	}
	src := face.Source()
	if src == nil {
		return face
	}
	var opts []text.FaceOption
	if feats := face.Features(); len(feats) > 0 {
		opts = append(opts, text.WithFeatures(feats...))
	}
	if vars := face.Variations(); len(vars) > 0 {
		opts = append(opts, text.WithVariations(vars...))
	}
	if lang := face.Language(); lang != "" {
		opts = append(opts, text.WithLanguage(lang))
	}
	return src.Face(points, opts...)
}

// effectiveFace is Face scaled to FontSize for single-string measure/paint.
// The derived face is memoized on stable inputs: face objects are immutable,
// and identity stability is what lets the incremental engine match the live
// cache across keystrokes instead of fully rebuilding every time.
func (t *RenderText) effectiveFace() text.Face {
	if t == nil || t.Face == nil {
		return nil
	}
	pts := t.fontSize()
	if t.effFace != nil && t.effFaceFor == t.Face && t.effFaceSize == pts {
		return t.effFace
	}
	ef := faceForSize(t.Face, pts)
	t.effFaceFor, t.effFaceSize, t.effFace = t.Face, pts, ef
	return ef
}

// lineHeightLogical is the per-line advance used for layout height and multi-line paint.
func (t *RenderText) lineHeightLogical() float64 {
	fs := t.fontSize()
	ls := t.lineSpacing()
	if face := t.effectiveFace(); face != nil {
		m := face.Metrics()
		lh := m.LineHeight()
		if lh > 0 {
			return lh * ls
		}
	}
	return fs * 1.25 * ls
}

func (t *RenderText) measureLine(s string) float64 {
	if s == "" {
		return 0
	}
	key := t.measureCacheKey(s)
	t.measureMu.Lock()
	if t.measureCache != nil {
		if w, ok := t.measureCache[key]; ok {
			t.measureHits++
			t.measureMu.Unlock()
			return w
		}
	}
	t.measureMu.Unlock()
	var w float64
	if face := t.effectiveFace(); face != nil {
		w, _ = text.Measure(s, face)
	} else {
		fs := t.fontSize()
		w = float64(utf8.RuneCountInString(s)) * fs * t.approxCharW()
	}
	t.measureMu.Lock()
	t.measureMiss++
	if t.measureCache == nil {
		t.measureCache = make(map[string]float64)
	}
	t.measureCache[key] = w
	t.measureMu.Unlock()
	return w
}

func (t *RenderText) measureCacheKey(s string) string {
	// Include size + approx factor so cache does not cross style changes.
	fs := t.fontSize()
	aw := t.approxCharW()
	// Face identity is not stable as a pointer string; invalidateMeasureCache
	// on SetFace covers face changes. Key is content+size+approx.
	return s + "\x00" + formatMeasureKey(fs, aw)
}

func formatMeasureKey(fs, aw float64) string {
	return strconv.FormatInt(int64(fs*100+0.5), 10) + "/" + strconv.FormatInt(int64(aw*1000+0.5), 10)
}

// invalidateMeasureCache drops width cache (text/face/size change).
func (t *RenderText) invalidateMeasureCache() {
	if t == nil {
		return
	}
	t.measureMu.Lock()
	t.measureCache = nil
	t.textLayout = nil
	t.measureMu.Unlock()
}

// ensureLayout returns the single-source TextLayout, building if dirty.
// For multi-run paragraphs (mixed sizes) it builds a per-run layout so
// CaretForOffset / HitTest use per-run advances and per-line max heights
// (Flutter Paragraph / SkParagraph semantics).
func (t *RenderText) ensureLayout() *TextLayout {
	if t == nil {
		return nil
	}
	if t.textLayout != nil {
		return t.textLayout
	}
	if t.hasRuns() {
		t.textLayout = BuildRenderTextLayout(t)
	} else {
		if t.lcache == nil {
			t.lcache = newLayoutCache()
		}
		if t.spanHint.ok {
			sp := t.spanHint
			t.spanHint.ok = false
			t.textLayout = t.lcache.updateSpan(t.Text, t.effectiveFace(), t.fontSize(), t.MaxWidth, t.lineSpacing(), t.approxCharW(), t.MaxLines, t.Overflow, sp)
		} else {
			t.textLayout = t.lcache.update(t.Text, t.effectiveFace(), t.fontSize(), t.MaxWidth, t.lineSpacing(), t.approxCharW(), t.MaxLines, t.Overflow)
		}
	}
	return t.textLayout
}

// SetViewportHint enables culling for long single-line viewports (Flutter RenderEditable).
// scrollX is the viewport ScrollOffset.X, visW is the viewport width (>0). Call after SetText.
// Pass visW<=0 to clear the hint (no culling). Does not mark dirty.
func (t *RenderText) SetViewportHint(scrollX, visW float64) {
	if t == nil {
		return
	}
	if visW <= 0 {
		t.hasViewportHint = false
		return
	}
	t.viewportScrollX = scrollX
	t.viewportWidth = visW
	t.hasViewportHint = true
}

// SetViewportRect extends SetViewportHint with a vertical band for
// wrapped/multi-line text (M2 vertical culling). scrollY/visH bound the
// visible rows; visH<=0 clears only the vertical band (horizontal hint kept).
// Does not mark dirty.
func (t *RenderText) SetViewportRect(scrollX, visW, scrollY, visH float64) {
	t.SetViewportHint(scrollX, visW)
	if t == nil {
		return
	}
	if visH <= 0 {
		t.hasViewportHintY = false
		return
	}
	t.viewportScrollY = scrollY
	t.viewportHeight = visH
	t.hasViewportHintY = true
}

// TextLayout returns the cached single-source layout (builds if needed).
func (t *RenderText) TextLayout() *TextLayout { return t.ensureLayout() }

// MeasureCacheStats returns R9 hit/miss counters (cumulative since last reset).
func (t *RenderText) MeasureCacheStats() (hits, misses int64) {
	if t == nil {
		return 0, 0
	}
	t.measureMu.Lock()
	defer t.measureMu.Unlock()
	return t.measureHits, t.measureMiss
}

// ResetMeasureCacheStats zeroes hit/miss counters (keeps cached widths).
func (t *RenderText) ResetMeasureCacheStats() {
	if t == nil {
		return
	}
	t.measureMu.Lock()
	defer t.measureMu.Unlock()
	t.measureHits, t.measureMiss = 0, 0
}

// TreeMeasureCacheStats sums cumulative measure hit/miss over every RenderText
// in the render tree (R9 measure_cache_hit sampling).
func TreeMeasureCacheStats(root RenderObject) (hits, misses int64) {
	if root == nil {
		return 0, 0
	}
	var walk func(n RenderObject)
	walk = func(n RenderObject) {
		if n == nil {
			return
		}
		if t, ok := n.(*RenderText); ok {
			h, m := t.MeasureCacheStats()
			hits += h
			misses += m
		}
		for _, ch := range n.Children() {
			walk(ch)
		}
	}
	walk(root)
	return hits, misses
}

// FontSizePt returns the effective font size in points (default 14 when
// unset). Exposed for caret geometry consumers.
func (t *RenderText) FontSizePt() float64 {
	if t == nil {
		return 0
	}
	return t.fontSize()
}

// Metrics returns the effective face's font metrics at the effective size
// (ascent/descent/lineGap — the typographic truth for caret/candidate
// geometry). Returns ok=false when no face is set; callers fall back to the
// heuristic (fontSize × 1.25).
func (t *RenderText) Metrics() (m text.Metrics, ok bool) {
	if t == nil {
		return m, false
	}
	face := t.effectiveFace()
	if face == nil {
		return m, false
	}
	return face.Metrics(), true
}

// LineHeight returns the logical line height used for multi-line painting
// (face metrics when available, else fontSize × lineSpacing). Exposed for
// caret/candidate geometry (IME anchoring).
func (t *RenderText) LineHeight() float64 {
	if t == nil {
		return 0
	}
	return t.lineHeightLogical()
}

// DisplayLines returns the visible lines after wrap + maxLines + overflow.
// Layout and paint both use this so measured size matches what is drawn.
// For multi-run content, each line is the concatenation of that line's span texts.
// MeasureWidth returns the rendered width of s using this text's face and
// font size (single line, no wrap). Heuristic estimate when no Face is set.
// Used for caret/candidate anchoring (IME cursor rectangles).
func (t *RenderText) MeasureWidth(s string) float64 {
	if t == nil {
		return 0
	}
	return t.measureLine(s)
}

// GlyphInkBounds returns the ink bounding box (glyph outline extents, not the
// advance box) of r relative to the pen origin at x=0, using this text's face
// and font size. ok=false when no Face is set or the rune has no glyph
// (control chars); whitespace has an empty outline → MinX==MaxX==0.
// General glyph-metric accessor for text-anchored overlays.
func (t *RenderText) GlyphInkBounds(r rune) (bounds text.Rect, ok bool) {
	if t == nil || t.Face == nil {
		return text.Rect{}, false
	}
	face := t.effectiveFace()
	if face == nil {
		return text.Rect{}, false
	}
	for g := range face.Glyphs(string(r)) {
		return g.Bounds, true
	}
	return text.Rect{}, false
}

// CaretColumn returns the caret geometry for a display byte offset, following
// the standard text-caret model (Flutter TextPainter.getOffsetForCaret /
// Skia): line index + the PEN BOUNDARY x between the glyphs left and right of
// the offset. The bar's CENTER belongs on that boundary — advance boxes touch,
// so this is exactly "between any two characters", for latin, CJK and mixed
// text alike. No ink scanning involved.
//
// The offset must lie on a rune boundary within t.Text; out-of-range clamps
// to 0 / len(Text). Returns (lineIdx, penX, true); ok=false when there is no
// content at all. Single-source: Flutter-aligned via GetOffsetForCaret.
func (t *RenderText) CaretColumn(off int) (lineIdx int, penX float64, ok bool) {
	if t == nil {
		return 0, 0, false
	}
	if lay := t.ensureLayout(); lay != nil && lay.LineCount() > 0 {
		// 行号经行索引二分(I8),与CaretForOffset同源,不再逐行扫LineTop.
		if li, x, ok := lay.CaretForOffset(off); ok {
			return li, x, true
		}
		return 0, 0, false
	}
	lines := t.DisplayLines()
	if len(lines) == 0 {
		return 0, 0, false
	}
	if off < 0 {
		off = 0
	}
	if off > len(t.Text) {
		off = len(t.Text)
	}
	start := 0
	for i, ln := range lines {
		end := start + len(ln)
		if off <= end || i == len(lines)-1 {
			inLine := t.Text[start:min(off, end)]
			return i, t.measureLine(inLine), true
		}
		start = end + 1
	}
	return 0, 0, false
}

func (t *RenderText) DisplayLines() []string {
	if t == nil {
		return nil
	}
	// Single-source (I7): line breaks come from TextLayout; only the
	// truncation-marker fitting below stays string-level.
	lay := t.ensureLayout()
	if lay == nil || lay.LineCount() == 0 {
		return nil
	}
	lines := make([]string, lay.LineCount())
	for i := 0; i < lay.LineCount(); i++ {
		s, e, _, _, ok := lay.Line(i)
		if !ok {
			continue
		}
		if s < 0 {
			s = 0
		}
		if e > len(lay.Text) {
			e = len(lay.Text)
		}
		if s > e {
			s = e
		}
		lines[i] = lay.Text[s:e]
	}
	maxL := t.MaxLines
	maxW := t.MaxWidth
	overflow := t.Overflow

	truncated := lay.Truncated
	if maxL > 0 && len(lines) > maxL {
		lines = append([]string(nil), lines[:maxL]...)
		truncated = true
	}

	// Single-line (or last visible line) may still exceed MaxWidth — ellipsize/clip width.
	if maxW > 0 && len(lines) > 0 {
		last := len(lines) - 1
		if t.measureLine(lines[last]) > maxW+0.5 {
			truncated = true
			if overflow == TextOverflowEllipsis {
				lines[last] = ellipsizeToWidth(lines[last], maxW, t)
			} else {
				lines[last] = clipToWidth(lines[last], maxW, t)
			}
		} else if truncated && overflow == TextOverflowEllipsis {
			// Dropped lines below: mark last visible line with ellipsis.
			lines[last] = ellipsizeToWidth(lines[last], maxW, t)
		}
	} else if truncated && overflow == TextOverflowEllipsis && len(lines) > 0 {
		// No MaxWidth: still append ellipsis to last line (may grow width slightly).
		last := len(lines) - 1
		if !strings.HasSuffix(lines[last], textEllipsis) {
			lines[last] = lines[last] + textEllipsis
		}
	}

	// Also: MaxLines==1 with MaxWidth, wrap produced 1 long line that fits measure
	// only after ellipsize — handled above via measureLine > maxW.

	return lines
}

// DisplayText joins DisplayLines with newlines (for debugging / tests).
func (t *RenderText) DisplayText() string {
	return strings.Join(t.DisplayLines(), "\n")
}

func ellipsizeToWidth(s string, maxW float64, t *RenderText) string {
	if maxW <= 0 {
		if strings.HasSuffix(s, textEllipsis) {
			return s
		}
		return s + textEllipsis
	}
	if t.measureLine(s) <= maxW {
		if !strings.HasSuffix(s, textEllipsis) {
			// Caller wants a marker because lower lines were dropped.
			cand := s + textEllipsis
			if t.measureLine(cand) <= maxW {
				return cand
			}
			// Need to shrink to fit ellipsis.
			// fall through to binary search on s+ellipsis below.
		} else {
			return s
		}
	}
	runes := []rune(s)
	// Remove existing trailing ellipsis runes before re-fitting.
	for strings.HasSuffix(string(runes), textEllipsis) {
		runes = runes[:len(runes)-utf8.RuneCountInString(textEllipsis)]
	}
	return ellipsizeWithPrefix(runes, maxW, t)
}

// runePrefixWidths builds cumulative advance in one O(n) pass so the ellipsis
// locator probes in O(log n) instead of re-measuring whole prefixes
// (O(n log n) shaped work). Estimate path is exact; shaped path sums cached
// per-rune advances and the caller verifies the fixed point with real
// measures, so results match the old search byte-for-byte.
func (t *RenderText) runePrefixWidths(runes []rune) []float64 {
	prefix := make([]float64, len(runes)+1)
	if face := t.effectiveFace(); face != nil {
		for i, r := range runes {
			prefix[i+1] = prefix[i] + text.RuneAdvance(face, r)
		}
		return prefix
	}
	w := t.fontSize() * t.approxCharW()
	for i := range runes {
		prefix[i+1] = prefix[i] + w
	}
	return prefix
}

// ellipsizeWithPrefix returns the longest runes[:k]+ellipsis fitting maxW.
// Probes are O(log n) prefix lookups; at most a constant number of real
// measures verify/adjust the fixed point (shaping is not purely additive).
func ellipsizeWithPrefix(runes []rune, maxW float64, t *RenderText) string {
	ellW := t.measureLine(textEllipsis)
	if ellW > maxW {
		return ""
	}
	prefix := t.runePrefixWidths(runes)
	lo, hi := 0, len(runes)
	best := 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if prefix[mid]+ellW <= maxW {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	widthOf := func(k int) float64 {
		return t.measureLine(string(runes[:k]) + textEllipsis)
	}
	for best < len(runes) && widthOf(best+1) <= maxW {
		best++
	}
	for best > 0 && widthOf(best) > maxW {
		best--
	}
	return string(runes[:best]) + textEllipsis
}

func clipToWidth(s string, maxW float64, t *RenderText) string {
	if t.measureLine(s) <= maxW {
		return s
	}
	runes := []rune(s)
	lo, hi := 0, len(runes)
	best := ""
	for lo <= hi {
		mid := (lo + hi) / 2
		cand := string(runes[:mid])
		if t.measureLine(cand) <= maxW {
			best = cand
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

func (t *RenderText) measureSize() (w, h float64) {
	if t.hasRuns() {
		return t.measureRunsSize()
	}
	lines := t.DisplayLines()
	if len(lines) == 0 {
		fs := t.fontSize()
		return 0, fs * 1.25
	}
	lh := t.lineHeightLogical()
	h = float64(len(lines)) * lh

	var longest float64
	for _, line := range lines {
		lw := t.measureLine(line)
		if lw > longest {
			longest = lw
		}
	}

	maxW := t.MaxWidth
	if maxW > 0 {
		// Budgeted width: never exceed MaxWidth; use MaxWidth when content fills the box
		// (wrap / ellipsis / multi-line), else shrink to longest line for short labels.
		if longest >= maxW-0.5 || len(lines) > 1 || t.MaxLines > 0 || t.Overflow == TextOverflowEllipsis {
			w = maxW
		} else {
			w = longest
		}
		if w > maxW {
			w = maxW
		}
		return w, h
	}

	return longest, h
}

// Layout implements RenderObject.
func (t *RenderText) Layout(c Constraints) Size {
	if sz, ok := t.LayoutSkipIfClean(c); ok {
		return sz
	}
	w, h := t.measureSize()
	out := c.Tighten(Size{Width: w, Height: h})
	t.setSize(out)
	t.RememberConstraints(c)
	t.clearLayoutDirty()
	return out
}

// Paint implements RenderObject — single-source via TextLayout + DrawShapedGlyphs.
func (t *RenderText) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !t.NeedsPaint() {
		return
	}
	pc.NotePaintVisit()
	if t.hasRuns() {
		t.paintRuns(pc)
	} else {
		lay := t.ensureLayout()
		lines := t.DisplayLines()
		if len(lines) > 0 && lay != nil {
			a := t.A
			if a == 0 && (t.R != 0 || t.G != 0 || t.B != 0) {
				a = 1
			}
			face := t.effectiveFace()
			if face != nil && pc.DC != nil {
				pc.DC.SetFont(face)
			}
			if pc.DC != nil {
				pc.DC.SetTextDecoration(t.Decoration)
			}
			if pc.DC != nil {
				pc.DC.SetRGBA(t.R, t.G, t.B, a)
			}
			// Flutter SkParagraph 行盒语义：行高按该行最大 run 撑开，基线 = 行顶 + 最大 ascent
			// 单字号时退化为 lh = ascent+descent(+gap)，与 caret 的 lineTop/lineHeight 同源。
			fs := t.fontSize()
			ascent := fs * 0.8
			if m, ok := t.Metrics(); ok && m.Ascent > 0 {
				ascent = m.Ascent
			}
			// M2 纵向裁剪:只提交可见行带(+/-1 行护栏保半行),无 Y hint 时全画.
			rowLo, rowHi := 0, lay.LineCount()
			if t.hasViewportHintY {
				rowLo, rowHi = visibleRowBandOf(lay, t.viewportScrollY, t.viewportHeight)
			}
			for i, line := range lines {
				if line == "" {
					continue
				}
				if i < rowLo || i >= rowHi {
					continue
				}
				y := ascent + lay.LineTop(i)
				if face != nil && pc.DC != nil && i < lay.LineCount() {
					glyphs := lay.LineGlyphs(i)
					// Bulk submission needs a sourced face (outlines/atlas
					// page); composite faces submit per-face partitions (M2).
					// Lines holding color runs bypass bulk: the mask atlas
					// is outline-only, bulk submit would only hit the
					// explicit refusal and fall back every frame.
					if len(glyphs) > 0 && glyphs[0].GID != 0 && face.Source() != nil && !lay.LineHasColorRun(i) {
						// Flutter viewport culling for 5000 single-line:
						// only submit glyphs within the viewport window
						// (+200px margin) to keep GPU vertex count O(visible).
						if lo, hi, ok := t.hCullWindow(lay.LineCount(), len(glyphs)); ok {
							s, e := cullRangeForWindow(glyphs, lo, hi)
							glyphs = glyphs[s:e]
						}
						ax, ay := pc.Abs(0, y)
						pc.DC.DrawShapedGlyphs(glyphs, face, ax, ay)
					} else if t.paintCompositeRuns(pc, lay, i, y, line) {
						// M2 composite batch submitted per-face partitions.
					} else if face.Source() == nil {
						// M1-13: byteOff→X一次建表后O(1)查,不再每字线性扫Carets.
						// caret为行内相对,键即相对偏移;range line的byteOff同为行内相对.
						// 同包直读内部表(零拷贝),d5后外部统一走LineCarets.
						xByOff := caretXByOffset(lay.lines[i].Carets)
						for byteOff, r := range line {
							x := xByOff[byteOff]
							// 逐字路无二分开销,不设200门:屏外字照常跳过,漏跳的由GPU裁掉,画面一致.
							if t.hasViewportHint && t.MaxWidth == 0 && lay.LineCount() == 1 {
								if x < t.viewportScrollX-viewportMargin || x > t.viewportScrollX+t.viewportWidth+viewportMargin {
									continue
								}
							}
							ax, ay := pc.Abs(x, y)
							pc.DC.DrawString(string(r), ax, ay)
						}
					} else {
						// Fallback colored path (also cull for large single line)
						if t.hasViewportHint && t.MaxWidth == 0 && lay.LineCount() == 1 && len(line) > 500 {
							// drawTextColored draws whole line; skip if we have hint and line is huge
							// fallback to per-glyph culling via DrawString above is already handled,
							// but this path is for GID==0 with source != nil (rare). Skip whole draw
							// and let culling happen via clipping (GPU will discard off-screen).
						}
						drawTextColored(pc, line, 0, y, t.R, t.G, t.B, a)
					}
				} else {
					drawTextColored(pc, line, 0, y, t.R, t.G, t.B, a)
				}
			}
			if pc.DC != nil {
				pc.DC.SetTextDecoration(render.TextDecorationNone)
			}
		}
	}
	t.clearPaintDirty()
}

// paintCompositeRuns submits a composite-face line in per-face batches (M2).
// Each run covers glyphs[Start:End] with its own sourced face; coordinates
// stay glyph.X (I1 single source, same as the single-face path above).
// The single-line viewport cull applies per partition. Reports false when the
// row is not routable (see TextLayout.LineBulkRoutable) — the caller then
// falls back to the legacy per-rune path, so the picture never changes,
// only the submit cost does.
//
// GPU batch layout is pen-origin based (it positions from the first glyph /
// pen origin, not from absolute glyph.X), so each partition is rebased to
// its own origin and submitted at ax+offset — final positions are identical,
// only the representation changes. Skipping the rebase stacks every run at
// the line start (Latin over CJK).
func (t *RenderText) paintCompositeRuns(pc *PaintContext, lay *TextLayout, row int, y float64, line string) bool {
	if !lay.LineBulkRoutable(row) {
		return false
	}
	glyphs := lay.LineGlyphs(row)
	runs := lay.LineGlyphRuns(row)
	cullLo, cullHi, cull := t.hCullWindow(lay.LineCount(), len(glyphs))
	for _, r := range runs {
		part := glyphs[r.Start:r.End]
		if r.IsColor {
			// M5: color runs bypass the mask batch entirely (never stuffed
			// into the mask atlas). Shaped color glyphs keep absolute
			// glyph.X, so no rebase is needed (unlike pen-origin mask
			// batches); off-screen parts are GPU-clipped, so no
			// glyph-window culling here (positions unchanged, I1/I3 hold).
			if pc.DC != nil && len(part) > 0 {
				ax, ay := pc.Abs(0, y)
				a := t.A
				if a == 0 && (t.R != 0 || t.G != 0 || t.B != 0) {
					a = 1
				}
				pc.DC.SetRGBA(t.R, t.G, t.B, a)
				pc.DC.DrawShapedColorGlyphs(part, r.Face, ax, ay)
			}
			continue
		}
		if cull {
			s, e := cullRangeForWindow(part, cullLo, cullHi)
			part = part[s:e]
		}
		if len(part) == 0 {
			continue
		}
		shifted, off := rebaseGlyphs(part)
		ax, ay := pc.Abs(off, y)
		pc.DC.DrawShapedGlyphs(shifted, r.Face, ax, ay)
	}
	return true
}

// rebaseGlyphs shifts a glyph partition so it starts at X=0, returning the
// shifted copy and the removed origin offset. shifted[i].X+off == part[i].X
// for every glyph: positions are preserved exactly, satisfying pen-origin
// based batch consumers (GPU glyph-mask layout) without touching I1.
func rebaseGlyphs(part []text.ShapedGlyph) (shifted []text.ShapedGlyph, off float64) {
	if len(part) == 0 {
		return nil, 0
	}
	off = part[0].X
	shifted = append([]text.ShapedGlyph(nil), part...)
	if off != 0 {
		for i := range shifted {
			shifted[i].X -= off
		}
	}
	return shifted, off
}

// SubmittedGlyphEstimate returns the glyphs Paint would submit under the
// current viewport hints (M2 observability, paints nothing). Unbounded (no
// hints) = every line's glyphs; with hints = the culled visible subset —
// the same gates Paint uses, so the estimate tracks real submissions.
// Coordinates are untouched (I3: culling only narrows submit range).
func (t *RenderText) SubmittedGlyphEstimate() int {
	if t == nil {
		return 0
	}
	if t.hasRuns() {
		// Multi-run rich text still paints per-rune (M5 domain); vertical
		// banding does not apply here, count all spans' runes.
		total := 0
		for _, ln := range t.layoutRunLines() {
			for _, sp := range ln.Spans {
				total += utf8.RuneCountInString(sp.Text)
			}
		}
		return total
	}
	lay := t.ensureLayout()
	if lay == nil {
		return 0
	}
	lines := t.DisplayLines()
	if len(lines) == 0 {
		return 0
	}
	rowLo, rowHi := 0, lay.LineCount()
	if t.hasViewportHintY && lay.LineCount() > 0 {
		rowLo, rowHi = visibleRowBandOf(lay, t.viewportScrollY, t.viewportHeight)
	}
	total := 0
	for i := rowLo; i < rowHi && i < len(lines); i++ {
		if lines[i] == "" {
			continue
		}
		glyphs := lay.LineGlyphs(i)
		if cullLo, cullHi, ok := t.hCullWindow(lay.LineCount(), len(glyphs)); ok {
			s, e := cullRangeForWindow(glyphs, cullLo, cullHi)
			total += e - s
			continue
		}
		total += len(glyphs)
	}
	return total
}

// SubmittedMaskGlyphEstimate mirrors SubmittedGlyphEstimate but excludes
// IsColor runs: the glyphs that would enter the mask atlas (M5 color
// observability). Color runs paint via the string color path and must not
// be counted as mask load.
func (t *RenderText) SubmittedMaskGlyphEstimate() int {
	if t == nil {
		return 0
	}
	if t.hasRuns() {
		// Rich runs paint per-span via the string path, never the mask
		// batch: mask load is zero by construction.
		return 0
	}
	lay := t.ensureLayout()
	if lay == nil {
		return 0
	}
	lines := t.DisplayLines()
	if len(lines) == 0 {
		return 0
	}
	rowLo, rowHi := 0, lay.LineCount()
	if t.hasViewportHintY && lay.LineCount() > 0 {
		rowLo, rowHi = visibleRowBandOf(lay, t.viewportScrollY, t.viewportHeight)
	}
	total := 0
	for i := rowLo; i < rowHi && i < len(lines); i++ {
		if lines[i] == "" {
			continue
		}
		glyphs := lay.LineGlyphs(i)
		mask := func(s, e int) int {
			n := 0
			for _, r := range lay.LineGlyphRuns(i) {
				if r.IsColor {
					continue
				}
				lo, hi := r.Start, r.End
				if lo < s {
					lo = s
				}
				if hi > e {
					hi = e
				}
				if hi > lo {
					n += hi - lo
				}
			}
			return n
		}
		if cullLo, cullHi, ok := t.hCullWindow(lay.LineCount(), len(glyphs)); ok {
			s, e := cullRangeForWindow(glyphs, cullLo, cullHi)
			total += mask(s, e)
			continue
		}
		total += mask(0, len(glyphs))
	}
	return total
}

// visibleRowBandOf maps a Y viewport band to a row range with ±1 row guard
// (shared by Paint and SubmittedGlyphEstimate so both skip the same rows).
func visibleRowBandOf(lay *TextLayout, scrollY, visH float64) (lo, hi int) {
	if lay == nil || lay.LineCount() == 0 || visH <= 0 {
		return 0, 0
	}
	lo, hi = lay.VisibleLineRange(scrollY, scrollY+visH)
	if lo > 0 {
		lo--
	}
	if hi < lay.LineCount() {
		hi++
	}
	return lo, hi
}

// TreeSubmittedGlyphEstimate sums SubmittedGlyphEstimate over every
// RenderText in the tree (M2 window-level O(V) observability).
func TreeSubmittedGlyphEstimate(root RenderObject) int {
	if root == nil {
		return 0
	}
	total := 0
	var walk func(n RenderObject)
	walk = func(n RenderObject) {
		if n == nil {
			return
		}
		if t, ok := n.(*RenderText); ok {
			total += t.SubmittedGlyphEstimate()
		}
		for _, ch := range n.Children() {
			walk(ch)
		}
	}
	walk(root)
	return total
}

// caretXByOffset把行内caret表建成byteOff→X索引(M1-13):建表O(n)一次,
// 之后每字O(1)查.调用方复用,勿每字重建.首个caret胜出(与旧线性扫首命中一致).
func caretXByOffset(carets []GlyphCaret) map[int]float64 {
	xByOff := make(map[int]float64, len(carets))
	for _, c := range carets {
		if _, dup := xByOff[c.ByteOff]; !dup {
			xByOff[c.ByteOff] = c.X
		}
	}
	return xByOff
}

// viewportMargin是视口裁剪向可见区外扩的护栏:滚动半字/字形出头不闪.
// 横向裁剪四处(批量单脸/复合分区/提交预估/逐字兜底)共用此值.
const viewportMargin = 200.0

// hCullWindow returns the visible X window when single-line viewport
// culling applies, ok=false means submit everything. The gate (hint set,
// unwrapped single line, enough glyphs to matter) and the margin live here
// once — Paint, paintCompositeRuns and SubmittedGlyphEstimate share it,
// so the three can never disagree on what "visible" means.
func (t *RenderText) hCullWindow(lineCount, totalGlyphs int) (lo, hi float64, ok bool) {
	if !t.hasViewportHint || t.MaxWidth != 0 || lineCount != 1 || totalGlyphs <= 200 {
		return 0, 0, false
	}
	return t.viewportScrollX - viewportMargin, t.viewportScrollX + t.viewportWidth + viewportMargin, true
}

// cullRangeForWindow returns the glyph index range intersecting [loX,hiX).
// Glyphs must be sorted by X (true for all layout-built lines: shaped runs
// stitch with a monotonically advancing cursor, estimate/fallback paths
// accumulate in rune order). Two binary searches, O(log n), no full scan.
// Empty intersection returns start==end; callers slice glyphs[start:end].
func cullRangeForWindow(glyphs []text.ShapedGlyph, loX, hiX float64) (start, end int) {
	n := len(glyphs)
	if n == 0 || hiX <= loX {
		return 0, 0
	}
	start = sort.Search(n, func(i int) bool {
		return glyphs[i].X+glyphs[i].XAdvance >= loX
	})
	end = sort.Search(n, func(i int) bool {
		return glyphs[i].X > hiX
	})
	if start > n {
		start = n
	}
	if end > n {
		end = n
	}
	if start > end {
		start = end
	}
	return start, end
}

func (t *RenderText) paintRuns(pc *PaintContext) {
	lines := t.layoutRunLines()
	if len(lines) == 0 {
		return
	}
	lineTop := 0.0
	for _, ln := range lines {
		// 行内最大 ascent 决定基线（Flutter SkParagraph：行盒按最大 run 撑开，基线 = 行顶 + 最大 ascent）
		maxAscent := 0.0
		for _, sp := range ln.Spans {
			a := sp.FontSize * 0.8
			if sp.Face != nil {
				if m := sp.Face.Metrics(); m.Ascent > 0 {
					a = m.Ascent
				}
			} else if m, ok := t.Metrics(); ok && m.Ascent > 0 {
				// Fallback to parent metrics when span has no face.
				_ = m
			}
			if a > maxAscent {
				maxAscent = a
			}
		}
		if maxAscent == 0 {
			if m, ok := t.Metrics(); ok && m.Ascent > 0 {
				maxAscent = m.Ascent
			} else {
				maxAscent = t.fontSize() * 0.8
			}
		}
		baseline := lineTop + maxAscent
		for _, sp := range ln.Spans {
			if sp.Text == "" {
				continue
			}
			// Prefer span face at span size; fall back to parent effective face.
			var face text.Face
			if sp.Face != nil {
				fs := sp.FontSize
				if fs <= 0 {
					fs = t.fontSize()
				}
				face = faceForSize(sp.Face, fs)
			} else {
				face = t.effectiveFace()
			}
			if face != nil && pc.DC != nil {
				pc.DC.SetFont(face)
			}
			dec := sp.Decoration
			if dec == 0 {
				dec = t.Decoration
			}
			if pc.DC != nil {
				pc.DC.SetTextDecoration(dec)
			}
			drawTextColored(pc, sp.Text, sp.X, baseline, sp.R, sp.G, sp.B, sp.A)
			if pc.DC != nil {
				pc.DC.SetTextDecoration(render.TextDecorationNone)
			}
		}
		lineTop += ln.Height
	}
}

// ByteOffsetAt converts an x offset (logical px from the text origin) into
// the nearest UTF-8 byte boundary — single-source via TextLayout (Flutter GetPositionForOffset).
func (t *RenderText) ByteOffsetAt(x float64) int {
	if t == nil || t.Text == "" {
		return 0
	}
	if lay := t.ensureLayout(); lay != nil && lay.LineCount() > 0 {
		off, _ := lay.GetPositionForOffset(x, 0)
		return off
	}
	if x <= 0 {
		return 0
	}
	prev := 0.0
	for idx, r := range t.Text {
		right := t.measureLine(t.Text[:idx+utf8.RuneLen(r)])
		if x < (prev+right)/2 {
			return idx
		}
		prev = right
	}
	return len(t.Text)
}

// ByteOffsetAtPoint converts a point (logical px, text-origin relative) into
// the nearest UTF-8 byte boundary — single-source via TextLayout (Flutter GetPositionForOffset).
func (t *RenderText) ByteOffsetAtPoint(x, y float64) int {
	if t == nil {
		return 0
	}
	if lay := t.ensureLayout(); lay != nil && lay.LineCount() > 0 {
		off, _ := lay.GetPositionForOffset(x, y)
		return off
	}
	lines := t.DisplayLines()
	if len(lines) <= 1 || y <= 0 {
		return t.ByteOffsetAt(x)
	}
	fs := t.fontSize()
	if fs <= 0 {
		fs = 14
	}
	lh := fs * t.lineSpacing()
	row := int(y / lh)
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}
	start := 0
	for i := 0; i < row; i++ {
		start += len(lines[i]) + 1
	}
	if x <= 0 {
		return start
	}
	line := lines[row]
	prev := 0.0
	for idx, r := range line {
		right := t.measureLine(line[:idx+utf8.RuneLen(r)])
		if x < (prev+right)/2 {
			return start + idx
		}
		prev = right
	}
	return start + len(line)
}

// HitTest implements RenderObject — single-source via TextLayout (Flutter Paragraph).
func (t *RenderText) HitTest(p Point) RenderObject {
	if lay := t.TextLayout(); lay != nil && lay.LineCount() > 0 {
		maxW := 0.0
		for i := 0; i < lay.LineCount(); i++ {
			if _, _, w, _, ok := lay.Line(i); ok && w > maxW {
				maxW = w
			}
		}
		totalH := lay.LineTop(lay.LineCount()-1) + lay.LineHeight(lay.LineCount()-1)
		if p.X >= 0 && p.Y >= 0 && p.X < maxW+0.5 && p.Y < totalH+0.5 {
			return t
		}
		return nil
	}
	sz := t.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return t
	}
	return nil
}
