package rendering

import (
	"strconv"
	"strings"
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
	measureCache map[string]float64
	measureHits  int64
	measureMiss  int64

	// textLayout is the single-source layout (R2). Nil = dirty.
	textLayout *TextLayout
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
		return
	}
	t.Text = s
	t.Runs = nil
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
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetOverflow sets clip vs ellipsis overflow; dirties layout+paint.
func (t *RenderText) SetOverflow(o TextOverflow) {
	if t == nil || t.Overflow == o {
		return
	}
	t.Overflow = o
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
func (t *RenderText) effectiveFace() text.Face {
	if t == nil || t.Face == nil {
		return nil
	}
	return faceForSize(t.Face, t.fontSize())
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
	if t.measureCache != nil {
		if w, ok := t.measureCache[key]; ok {
			t.measureHits++
			return w
		}
	}
	t.measureMiss++
	var w float64
	if face := t.effectiveFace(); face != nil {
		w, _ = text.Measure(s, face)
	} else {
		fs := t.fontSize()
		w = float64(utf8.RuneCountInString(s)) * fs * t.approxCharW()
	}
	if t.measureCache == nil {
		t.measureCache = make(map[string]float64)
	}
	t.measureCache[key] = w
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
	t.measureCache = nil
	t.textLayout = nil
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
		t.textLayout = BuildTextLayout(t.Text, t.effectiveFace(), t.fontSize(), t.MaxWidth, t.lineSpacing())
	}
	return t.textLayout
}

// TextLayout returns the cached single-source layout (builds if needed).
func (t *RenderText) TextLayout() *TextLayout { return t.ensureLayout() }

// MeasureCacheStats returns R9 hit/miss counters (cumulative since last reset).
func (t *RenderText) MeasureCacheStats() (hits, misses int64) {
	if t == nil {
		return 0, 0
	}
	return t.measureHits, t.measureMiss
}

// ResetMeasureCacheStats zeroes hit/miss counters (keeps cached widths).
func (t *RenderText) ResetMeasureCacheStats() {
	if t == nil {
		return
	}
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

// wrapLines produces soft-wrapped lines for the full source text (no maxLines yet).
func (t *RenderText) wrapLines() []string {
	s := ""
	if t != nil {
		s = t.Text
	}
	if s == "" {
		return nil
	}
	maxW := 0.0
	if t != nil {
		maxW = t.MaxWidth
	}
	if maxW <= 0 {
		// No wrap width: hard breaks only.
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
		return strings.Split(s, "\n")
	}
	if face := t.effectiveFace(); face != nil {
		res := text.WrapText(s, face, maxW, text.WrapWord)
		out := make([]string, len(res))
		for i, r := range res {
			out[i] = r.Text
		}
		if len(out) == 0 {
			return []string{""}
		}
		return out
	}
	return estimateWrapLines(s, maxW, t.fontSize(), t.approxCharW())
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
// content at all. Single-source: reads from TextLayout Carets.
func (t *RenderText) CaretColumn(off int) (lineIdx int, penX float64, ok bool) {
	if t == nil {
		return 0, 0, false
	}
	if lay := t.ensureLayout(); lay != nil && len(lay.Lines) > 0 {
		return lay.CaretForOffset(off)
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
	if t.hasRuns() {
		rlines := t.layoutRunLines()
		if len(rlines) == 0 {
			return nil
		}
		out := make([]string, len(rlines))
		for i, ln := range rlines {
			var b strings.Builder
			for _, sp := range ln.Spans {
				b.WriteString(sp.Text)
			}
			out[i] = b.String()
		}
		return out
	}
	lines := t.wrapLines()
	if len(lines) == 0 {
		return nil
	}
	maxL := t.MaxLines
	maxW := t.MaxWidth
	overflow := t.Overflow

	truncated := false
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

func estimateWrapLines(s string, maxW, fs, aw float64) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	paras := strings.Split(s, "\n")
	avg := aw * fs
	if avg < 1 {
		avg = 1
	}
	var out []string
	for _, para := range paras {
		if para == "" {
			out = append(out, "")
			continue
		}
		// Greedy word pack; break overlong words by rune.
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		var line string
		for _, w := range words {
			ww := float64(utf8.RuneCountInString(w)) * avg
			if ww > maxW {
				// Flush current line, then break word.
				if line != "" {
					out = append(out, line)
					line = ""
				}
				runes := []rune(w)
				for len(runes) > 0 {
					n := int(maxW / avg)
					if n < 1 {
						n = 1
					}
					if n > len(runes) {
						n = len(runes)
					}
					// Prefer leaving room if more remains.
					chunk := string(runes[:n])
					if float64(utf8.RuneCountInString(chunk))*avg > maxW && n > 1 {
						n--
						chunk = string(runes[:n])
					}
					out = append(out, chunk)
					runes = runes[n:]
				}
				continue
			}
			cand := w
			if line != "" {
				cand = line + " " + w
			}
			cw := float64(utf8.RuneCountInString(cand)) * avg
			if line != "" && cw > maxW {
				out = append(out, line)
				line = w
			} else {
				line = cand
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
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
	lo, hi := 0, len(runes)
	best := textEllipsis
	if t.measureLine(best) > maxW {
		return ""
	}
	for lo <= hi {
		mid := (lo + hi) / 2
		cand := string(runes[:mid]) + textEllipsis
		if t.measureLine(cand) <= maxW {
			best = cand
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
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
			for i, line := range lines {
				if line == "" {
					continue
				}
				y := ascent + lay.LineTop(i)
				if face != nil && pc.DC != nil && len(lay.Lines) > i {
					glyphs := lay.Lines[i].Glyphs
					if len(glyphs) > 0 && glyphs[0].GID != 0 {
						ax, ay := pc.Abs(0, y)
						pc.DC.DrawShapedGlyphs(glyphs, face, ax, ay)
					} else if face.Source() == nil {
						for byteOff, r := range line {
							var x float64
							for _, c := range lay.Lines[i].Carets {
								if c.ByteOff == byteOff+lay.Lines[i].StartByte {
									x = c.X
									break
								}
							}
							ax, ay := pc.Abs(x, y)
							pc.DC.DrawString(string(r), ax, ay)
						}
					} else {
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
// the nearest UTF-8 byte boundary — single-source via TextLayout.
func (t *RenderText) ByteOffsetAt(x float64) int {
	if t == nil || t.Text == "" {
		return 0
	}
	if lay := t.ensureLayout(); lay != nil && len(lay.Lines) > 0 {
		return lay.HitTest(x, 0, t.lineHeightLogical())
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
// the nearest UTF-8 byte boundary — single-source via TextLayout.
func (t *RenderText) ByteOffsetAtPoint(x, y float64) int {
	if t == nil {
		return 0
	}
	if lay := t.ensureLayout(); lay != nil && len(lay.Lines) > 0 {
		return lay.HitTest(x, y, t.lineHeightLogical())
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

// HitTest implements RenderObject.
func (t *RenderText) HitTest(p Point) RenderObject {
	sz := t.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return t
	}
	return nil
}
