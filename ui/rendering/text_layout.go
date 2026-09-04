package rendering

import (
	"math"
	"sort"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/emoji"
)

// GlyphCaret is one pen boundary between glyphs.
type GlyphCaret struct {
	ByteOff int
	X       float64
}

// TextLayoutLine is one visual line.
type TextLayoutLine struct {
	StartByte int
	EndByte   int
	Carets    []GlyphCaret
	Width     float64
	Height    float64
	Glyphs    []text.ShapedGlyph
	// GlyphRuns partitions Glyphs by submitting face (M2 composite batch):
	// each run covers Glyphs[Start:End] and must be submitted with Run.Face.
	// Single-face lines hold one run; empty when the line has no batchable
	// glyphs (nil-face estimate or unshaped fallback keeps the old paint path).
	// 与Carets/Glyphs一样,构建后只读共享,任何原地修改都会污染缓存快照.
	GlyphRuns []LineGlyphRun
	// clusters是行内字素簇起点(绝对字节偏移,末尾附行尾哨兵),懒算缓存.
	clusters []int
	// runRTL是整形run的RTL标记(与GlyphRuns一一对应,仅整形路径有;
	// 估算/兜底路径为nil).行内复用凭它加GlyphRuns还原旧run边界。
	runRTL []bool
}

// LineGlyphRun is one batchable glyph slice within a line (M2).
type LineGlyphRun struct {
	Face       text.Face
	Start, End int
	// IsColor marks color-glyph runs (emoji): Paint must NOT submit them
	// through the mask batch (DrawShapedGlyphs) but through the string
	// color path. TextStart/TextEnd is the run's byte range in the line.
	IsColor            bool
	TextStart, TextEnd int
}

// isColorText reports whether s holds an emoji-presentation sequence
// (M5 color-glyph detection, via render/text/emoji segmenter).
func isColorText(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range emoji.Segment(s) {
		if r.IsEmoji {
			return true
		}
	}
	return false
}

// TextLayout is the single source for paint + queries.
type TextLayout struct {
	Text string
	// lines是行表(M1-d5起不再导出).外部经LineCount/Line/LineCarets/
	// LineGlyphs/CaretAt访问(值拷贝或只读 absolutize,供懒物化演进);
	// 行内caret为行内相对偏移,绝对值=相对值+行StartByte.
	lines    []TextLayoutLine
	FontSize float64
	// LineSpacing is the multiplier used when building this layout; kept for HitTest fallback.
	LineSpacing float64
	Generation  uint64
	MaxWidth    float64
	Face        text.Face
	// MaxLines caps visible lines (0 = unlimited, mirrors RenderText.MaxLines).
	MaxLines int
	// Overflow applies when content exceeds MaxWidth/MaxLines (mirrors RenderText.Overflow).
	Overflow TextOverflow
	// Truncated is true when MaxLines dropped lines.
	Truncated bool
	// LineGen holds per-row validity marks for the row/segment cache (M1 I9).
	// TextLayout.Generation stays a global monotonic counter (2 consumers rely
	// on it); invalidation granularity is expressed here instead.
	LineGen []uint64
	// idx is the lazily validated row index + height prefix (M1 I8).
	idx *lineIndex
	// idxSeq认领idx序号(增量引擎patch时递增,失配即重建).
	idxSeq uint64
}

// finishLayout stamps per-row marks and prebuilds the row index.
func (l *TextLayout) finishLayout() *TextLayout {
	l.LineGen = make([]uint64, len(l.lines))
	for i := range l.LineGen {
		l.LineGen[i] = l.Generation
	}
	l.idx = buildLineIndex(l.lines, l.FontSize, l.LineSpacing)
	return l
}

// index returns the row index, rebuilding when Lines changed since.
func (l *TextLayout) index() *lineIndex {
	if l.idx != nil && l.idx.valid(l.lines) && l.idx.seq == l.idxSeq {
		return l.idx
	}
	l.idx = buildLineIndex(l.lines, l.FontSize, l.LineSpacing)
	l.idxSeq = l.idx.seq
	return l.idx
}

func lineHeightFor(face text.Face, fontSize, lineSpacing float64) float64 {
	if lineSpacing <= 0 {
		lineSpacing = 1.2
	}
	if fontSize <= 0 {
		fontSize = 14
	}
	if face != nil {
		m := face.Metrics()
		if lh := m.LineHeight(); lh > 0 {
			return lh * lineSpacing
		}
	}
	return fontSize * 1.25 * lineSpacing
}

// textLayoutGen是全局自增计数器(I9语义不变),原子操作保跨线程自增.
var textLayoutGen atomic.Uint64

// SnapPixel 对齐到物理像素（HiDPI 1.25/2.0 1px 采样，F0–F9）.
func SnapPixel(x, scale float64) float64 {
	if scale <= 0 {
		return x
	}
	return math.Round(x*scale) / scale
}

// TextLayout.SnappedX 返回按 scale 像素对齐的 caret X（HiDPI）。
func (l *TextLayout) SnappedX(byteOff int, scale float64) (float64, bool) {
	if l == nil || len(l.lines) == 0 {
		return 0, false
	}
	_, x, ok := l.CaretForOffset(byteOff)
	if !ok {
		return 0, false
	}
	return SnapPixel(x, scale), true
}

// BuildTextLayout produces single-source layout (no line cap).
func BuildTextLayout(textStr string, face text.Face, fontSize float64, maxWidth float64, lineSpacing float64) *TextLayout {
	return BuildTextLayoutEx(textStr, face, fontSize, maxWidth, lineSpacing, 0.55, 0, TextOverflowClip)
}

// BuildTextLayoutEx is BuildTextLayout with an estimate width factor
// (approxCharW, used only when face == nil) and a MaxLines/Overflow cap
// applied to the built lines (layout-side truncation, I11).
func BuildTextLayoutEx(textStr string, face text.Face, fontSize float64, maxWidth float64, lineSpacing float64, approxCharW float64, maxLines int, overflow TextOverflow) *TextLayout {
	if textStr == "" {
		gen := textLayoutGen.Add(1)
		return (&TextLayout{Text: textStr, FontSize: fontSize, LineSpacing: lineSpacing, Generation: gen, MaxWidth: maxWidth, Face: face, MaxLines: maxLines, Overflow: overflow}).finishLayout()
	}
	if fontSize <= 0 {
		fontSize = 14
	}
	if lineSpacing <= 0 {
		lineSpacing = 1.2
	}
	lh := lineHeightFor(face, fontSize, lineSpacing)
	var lines []TextLayoutLine
	var wrapped []text.WrapResult
	if maxWidth <= 0 {
		parts := strings.Split(textStr, "\n")
		off := 0
		for _, p := range parts {
			wrapped = append(wrapped, text.WrapResult{Text: p, Start: off, End: off + len(p)})
			off += len(p) + 1
		}
		if len(wrapped) == 0 {
			wrapped = []text.WrapResult{{Text: textStr, Start: 0, End: len(textStr)}}
		}
	} else if face == nil {
		// No-face estimate wrap (mirrors the old display-side estimate so
		// layout and display break identically, I7). Kept in layout so the
		// estimate path also yields caret geometry.
		wrapped = estimateWrapResults(textStr, maxWidth, fontSize, approxCharW)
		if len(wrapped) == 0 {
			wrapped = []text.WrapResult{{Text: textStr, Start: 0, End: len(textStr)}}
		}
	} else {
		wrapped = text.WrapText(textStr, face, maxWidth, text.WrapWordChar)
		if len(wrapped) == 0 {
			wrapped = []text.WrapResult{{Text: textStr, Start: 0, End: len(textStr)}}
		}
	}
	for _, w := range wrapped {
		lines = append(lines, materializeWrappedRow(w, face, lh))
	}
	truncated := false
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	gen := textLayoutGen.Add(1)
	return (&TextLayout{Text: textStr, lines: lines, FontSize: fontSize, LineSpacing: lineSpacing, Generation: gen, MaxWidth: maxWidth, Face: face, MaxLines: maxLines, Overflow: overflow, Truncated: truncated}).finishLayout()
}

// LineCount返回行数(面1迁移新API,值拷贝语义,供懒物化演进).
func (l *TextLayout) LineCount() int {
	if l == nil {
		return 0
	}
	return len(l.lines)
}

// Line返回第i行的几何(值拷贝,不暴露内部切片).
func (l *TextLayout) Line(i int) (start, end int, width, height float64, ok bool) {
	if l == nil || i < 0 || i >= len(l.lines) {
		return 0, 0, 0, 0, false
	}
	ln := l.lines[i]
	return ln.StartByte, ln.EndByte, ln.Width, ln.Height, true
}

// CaretAt返回指定行内绝对字节偏移的X(行内二分,I8).
// byteOff是文档全局偏移(含行基址),不是行内相对值;
// 行内相对表由LineCarets转绝对,此处做逆换算.传相对值会错一个行基址.
func (l *TextLayout) CaretAt(lineIdx int, byteOff int) (x float64, ok bool) {
	if l == nil || lineIdx < 0 || lineIdx >= len(l.lines) {
		return 0, false
	}
	ln := &l.lines[lineIdx]
	return caretXForOffset(ln.Carets, byteOff-ln.StartByte)
}

// LineCarets返回第i行的caret表(绝对偏移拷贝,调用方可安全持有).
func (l *TextLayout) LineCarets(i int) []GlyphCaret {
	if l == nil || i < 0 || i >= len(l.lines) {
		return nil
	}
	ln := &l.lines[i]
	out := make([]GlyphCaret, len(ln.Carets))
	for k, c := range ln.Carets {
		c.ByteOff += ln.StartByte
		out[k] = c
	}
	return out
}

// LineGlyphs返回第i行的整形字形(内部切片只读共享,调用方不得修改).
// 绘制批量提交的唯一字形源(I1).
func (l *TextLayout) LineGlyphs(i int) []text.ShapedGlyph {
	if l == nil || i < 0 || i >= len(l.lines) {
		return nil
	}
	return l.lines[i].Glyphs
}

// LineGlyphRuns返回第i行的批量分区(值拷贝,调用方可安全持有).
// 每个分区覆盖 Glyphs[Start:End],用 Face 独立批量提交 (M2).
// 无可批量字形时返回 nil,调用方走旧绘制路径.
func (l *TextLayout) LineGlyphRuns(i int) []LineGlyphRun {
	if l == nil || i < 0 || i >= len(l.lines) {
		return nil
	}
	runs := l.lines[i].GlyphRuns
	if len(runs) == 0 {
		return nil
	}
	return append([]LineGlyphRun(nil), runs...)
}

// LineBulkRoutable reports whether row i can be submitted in batches:
// single-face bulk (real glyphs + sourced face, Paint's first branch) or
// per-face composite partitions (Paint's second branch, see below).
// Engine and example probes share this single source instead of
// reimplementing the routing condition.
//
// NOTE: paintCompositeRuns calls this only after the single-face branch
// failed, so inside that call the first clause never fires — the comment
// stays to keep the predicate total, not to change that call path.
func (l *TextLayout) LineBulkRoutable(i int) bool {
	if l == nil || i < 0 || i >= len(l.lines) {
		return false
	}
	glyphs := l.lines[i].Glyphs
	if len(glyphs) > 0 && glyphs[0].GID != 0 && l.Face != nil && l.Face.Source() != nil {
		return true
	}
	runs := l.lines[i].GlyphRuns
	if len(runs) == 0 {
		return false
	}
	for _, r := range runs {
		if r.Start < 0 || r.End > len(glyphs) || r.Start >= r.End {
			return false
		}
		part := glyphs[r.Start:r.End]
		if len(part) == 0 || part[0].GID == 0 || r.Face == nil || r.Face.Source() == nil {
			return false
		}
	}
	return true
}

// VisibleLineRange返回与纵向区间 [y0,y1) 相交的行区间 [lo,hi).
// 无交集时 lo==hi.行高前缀和二分定位,O(log n),不遍历全表 (M2).
func (l *TextLayout) VisibleLineRange(y0, y1 float64) (lo, hi int) {
	if l == nil || len(l.lines) == 0 || y1 <= y0 {
		return 0, 0
	}
	idx := l.index()
	n := len(l.lines)
	lo = sort.Search(n, func(i int) bool {
		return idx.tops[i]+l.LineHeight(i) > y0
	})
	hi = sort.Search(n, func(i int) bool {
		return idx.tops[i] >= y1
	})
	if lo > hi {
		lo = hi
	}
	return lo, hi
}

// DamageRectForRows返回行区间 [startRow,endRow) 的脏矩形 (M2 damage).
// X 取区间内最大行宽,Y 取首行顶到底行底.调用方只把被改行/段传进来,
// damage 面积即 O(变动行/段).行号越界钳制,空区间返回零矩形.
func (l *TextLayout) DamageRectForRows(startRow, endRow int) Rect {
	if l == nil || len(l.lines) == 0 {
		return Rect{}
	}
	if startRow < 0 {
		startRow = 0
	}
	if endRow > len(l.lines) {
		endRow = len(l.lines)
	}
	if startRow >= endRow {
		return Rect{}
	}
	maxW := 0.0
	for i := startRow; i < endRow; i++ {
		if w := l.lines[i].Width; w > maxW {
			maxW = w
		}
	}
	y0 := l.LineTop(startRow)
	y1 := l.LineTop(endRow-1) + l.LineHeight(endRow-1)
	return Rect{Min: Point{X: 0, Y: y0}, Max: Point{X: maxW, Y: y1}}
}

// materializeWrappedRow把一段回绕结果物化为行(主构建与段缓存共用,同源).
// 行内caret为行内相对偏移(绝对值=相对值+w.Start);行起止为父坐标.
func materializeWrappedRow(w text.WrapResult, face text.Face, lh float64) TextLayoutLine {
	lineText := w.Text
	start := w.Start
	end := w.End
	carets, width, glyphs, gruns, runRTL := buildCaretsForLineWithRuns(lineText, face)
	// Adjust glyph X to be relative to line start (already 0) and keep
	if len(carets) == 0 {
		carets = []GlyphCaret{{ByteOff: 0, X: 0}, {ByteOff: len(lineText), X: 0}}
	} else {
		if carets[len(carets)-1].ByteOff != len(lineText) {
			carets = append(carets, GlyphCaret{ByteOff: len(lineText), X: width})
		}
	}
	return TextLayoutLine{
		StartByte: start,
		EndByte:   end,
		Carets:    carets,
		Width:     width,
		Height:    lh,
		Glyphs:    glyphs,
		GlyphRuns: gruns,
		runRTL:    runRTL,
	}
}

// estimateWrapResults is the layout-side no-face estimate wrap. It produces
// exactly the line strings the old display-side estimateWrapLines produced
// (greedy word pack, rune-break for overlong words), plus byte offsets into
// the ORIGINAL text so carets stay addressable. Whitespace collapsing matches
// the old path; on multi-space text caret offsets may drift within the gap
// (same lossiness the old display path had — display collapsed too).
func estimateWrapResults(s string, maxW, fs, aw float64) []text.WrapResult {
	avg := aw * fs
	if avg < 1 {
		avg = 1
	}
	var out []text.WrapResult
	emit := func(txt string, start, end int) {
		out = append(out, text.WrapResult{Text: txt, Start: start, End: end})
	}
	// Split hard breaks on '\n' in original coordinates; a trailing '\r'
	// pairs with the '\n' (original "\r\n") and is not a second break.
	segs := strings.Split(s, "\n")
	base := 0
	flushSeg := func(seg string, segBase int) {
		// Sub-split lone '\r' (each is a break, like the old normalization).
		paras := strings.Split(seg, "\r")
		for pi, para := range paras {
			pStart := segBase
			for i := 0; i < pi; i++ {
				pStart += len(paras[i]) + 1
			}
			pEnd := pStart + len(para)
			if para == "" {
				// Trailing '\r' pairing with '\n' is not an extra line.
				if pi == len(paras)-1 && strings.HasSuffix(seg, "\r") {
					continue
				}
				emit("", pStart, pStart)
				continue
			}
			words := strings.Fields(para)
			if len(words) == 0 {
				emit("", pStart, pStart)
				continue
			}
			cursor := pStart
			nextWord := func(w string) (int, int) {
				rel := strings.Index(s[cursor:pEnd], w)
				if rel < 0 {
					rel = 0
				}
				ws := cursor + rel
				return ws, ws + len(w)
			}
			var line string
			lineStart, lineEnd := 0, 0
			for _, w := range words {
				ws, we := nextWord(w)
				cursor = we
				ww := float64(utf8.RuneCountInString(w)) * avg
				if ww > maxW {
					if line != "" {
						emit(line, lineStart, lineEnd)
						line = ""
					}
					runes := []rune(w)
					cs := ws
					for len(runes) > 0 {
						n := int(maxW / avg)
						if n < 1 {
							n = 1
						}
						if n > len(runes) {
							n = len(runes)
						}
						chunk := string(runes[:n])
						if float64(utf8.RuneCountInString(chunk))*avg > maxW && n > 1 {
							n--
							chunk = string(runes[:n])
						}
						emit(chunk, cs, cs+len(chunk))
						cs += len(chunk)
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
					emit(line, lineStart, lineEnd)
					line = w
					lineStart = ws
					lineEnd = we
				} else {
					if line == "" {
						lineStart = ws
					}
					line = cand
					lineEnd = we
				}
			}
			if line != "" {
				emit(line, lineStart, lineEnd)
			}
		}
	}
	for _, seg := range segs {
		flushSeg(seg, base)
		base += len(seg) + 1
	}
	if len(out) == 0 {
		return []text.WrapResult{{Text: s, Start: 0, End: len(s)}}
	}
	return out
}

// LineTop returns the Y offset of line idx from the text origin (sum of previous line heights).
func (l *TextLayout) LineTop(idx int) float64 {
	if l == nil || idx <= 0 {
		return 0
	}
	x := l.index()
	if idx < x.n {
		return x.tops[idx]
	}
	return x.totalH
}

// LineHeight returns the height of line idx, falling back to uniform calculation.
func (l *TextLayout) LineHeight(idx int) float64 {
	if l == nil || len(l.lines) == 0 {
		return lineHeightFor(nil, l.FontSize, l.LineSpacing)
	}
	if idx >= 0 && idx < len(l.lines) {
		if h := l.lines[idx].Height; h > 0 {
			return h
		}
	}
	return lineHeightFor(nil, l.FontSize, l.LineSpacing)
}

func buildCaretsForLine(line string, face text.Face) ([]GlyphCaret, float64, []text.ShapedGlyph) {
	carets, width, glyphs, _, _ := buildCaretsForLineWithRuns(line, face)
	return carets, width, glyphs
}

// buildCaretsForLineWithRuns shapes like buildCaretsForLine and additionally
// reports per-face glyph partitions for M2 composite batch submission.
// Runs is nil when the line has no batchable glyphs (nil-face estimate or
// unshaped fallback); callers keep the legacy paint path then.
// runRTL mirrors the shaping runs (nil unless the shaped path succeeded).
func buildCaretsForLineWithRuns(line string, face text.Face) ([]GlyphCaret, float64, []text.ShapedGlyph, []LineGlyphRun, []bool) {
	if line == "" {
		return []GlyphCaret{{ByteOff: 0, X: 0}}, 0, nil, nil, nil
	}
	if face == nil {
		var carets []GlyphCaret
		var glyphs []text.ShapedGlyph
		x := 0.0
		adv := 10.0
		carets = append(carets, GlyphCaret{ByteOff: 0, X: 0})
		for idx, r := range line {
			glyphs = append(glyphs, text.ShapedGlyph{GID: 0, Cluster: 0, X: x, XAdvance: adv})
			x += adv
			_, sz := utf8.DecodeRuneInString(line[idx:])
			next := idx + sz
			carets = append(carets, GlyphCaret{ByteOff: next, X: x})
			if next >= len(line) {
				break
			}
			_ = r
		}
		return carets, x, glyphs, nil, nil
	}
	// Shaped path (M0 item 8): split into (face, script, direction) runs and
	// shape each run, then stitch carets in a single pass — O(n), no
	// CaretXForCluster table scan. Any unshapable run falls back below.
	if runs := itemizeRuns(line, face); len(runs) > 0 {
		if carets, width, glyphs, gruns, ok := buildShapedCarets(line, runs, 0, 0); ok {
			rtl := make([]bool, len(runs))
			for i, r := range runs {
				rtl[i] = r.rtl
			}
			return carets, width, glyphs, gruns, rtl
		}
	}
	var carets []GlyphCaret
	var sg []text.ShapedGlyph
	carets = append(carets, GlyphCaret{ByteOff: 0, X: 0})
	x := 0.0
	for byteOff, r := range line {
		adv := text.RuneAdvance(face, r)
		sg = append(sg, text.ShapedGlyph{GID: 0, Cluster: 0, X: x, XAdvance: adv})
		x += adv
		next := byteOff + utf8.RuneLen(r)
		carets = append(carets, GlyphCaret{ByteOff: next, X: x})
		if next >= len(line) {
			break
		}
	}
	if len(sg) == 0 {
		return carets, x, sg, nil, nil
	}
	return carets, x, sg, []LineGlyphRun{{Face: face, Start: 0, End: len(sg)}}, nil
}

// itemizedRun is one independently shapable slice of a line: byte range,
// the face covering it, and whether it needs RTL visual reordering.
type itemizedRun struct {
	face       text.Face
	start, end int
	rtl        bool
}

// itemizeRuns intersects font-fallback runs (MultiFace.Runs) with
// script/bidi segments (segment.go): same face but different script or
// direction still splits, or complex scripts would never shape.
func itemizeRuns(line string, face text.Face) []itemizedRun {
	if line == "" || face == nil {
		return nil
	}
	segs := text.SegmentText(line)
	if len(segs) == 0 {
		return nil
	}
	isRTL := func(d text.Direction) bool { return d == text.DirectionRTL }
	if mf, ok := face.(*text.MultiFace); ok {
		fruns := mf.Runs(line)
		type frange struct {
			face       text.Face
			start, end int
		}
		var ranges []frange
		cur := 0
		for _, fr := range fruns {
			if fr.Text == "" {
				continue
			}
			rel := strings.Index(line[cur:], fr.Text)
			if rel < 0 {
				return nil
			}
			s := cur + rel
			ranges = append(ranges, frange{fr.Face, s, s + len(fr.Text)})
			cur = s + len(fr.Text)
		}
		if cur != len(line) {
			return nil
		}
		var out []itemizedRun
		j := 0
		for _, r := range ranges {
			for j < len(segs) && segs[j].End <= r.start {
				j++
			}
			for k := j; k < len(segs) && segs[k].Start < r.end; k++ {
				s := max(r.start, segs[k].Start)
				e := min(r.end, segs[k].End)
				if s < e {
					out = append(out, itemizedRun{face: r.face, start: s, end: e, rtl: isRTL(segs[k].Direction)})
				}
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	out := make([]itemizedRun, 0, len(segs))
	for _, s := range segs {
		out = append(out, itemizedRun{face: face, start: s.Start, end: s.End, rtl: isRTL(s.Direction)})
	}
	return out
}

// buildShapedCarets shapes each run and stitches one caret per rune boundary
// in a single pass. ok=false when any run shapes empty (caller falls back).
// gruns partitions the returned glyphs by submitting face (M2 composite
// batch): gruns[k] covers glyphs[gruns[k].Start:gruns[k].End] with Face.
// cursor0/prevX0 seed the pen for window reuse (full-line builds pass 0,0);
// Cluster/Caret ByteOff stay relative to line, glyph X absorbs cursor0.
func buildShapedCarets(line string, runs []itemizedRun, cursor0, prevX0 float64) ([]GlyphCaret, float64, []text.ShapedGlyph, []LineGlyphRun, bool) {
	runeByte := make([]int, 0)
	for i := range line {
		if utf8.RuneStart(line[i]) {
			runeByte = append(runeByte, i)
		}
	}
	runeByte = append(runeByte, len(line))
	n := len(runeByte) - 1
	if n <= 0 {
		return nil, 0, nil, nil, false
	}
	xs := make([]float64, n)
	filled := make([]bool, n)
	var glyphs []text.ShapedGlyph
	var gruns []LineGlyphRun
	cursor := cursor0
	runeBase := 0
	for _, r := range runs {
		seg := line[r.start:r.end]
		// Shape results may be cache-owned: copy before remapping in place.
		sg := text.Shape(seg, r.face)
		if len(sg) == 0 {
			return nil, 0, nil, nil, false
		}
		g := append([]text.ShapedGlyph(nil), sg...)
		if r.rtl {
			g = text.ReorderRTLShapedGlyphs(g)
		}
		end := cursor
		for i := range g {
			g[i].Cluster += runeBase
			g[i].X += cursor
			if e := g[i].X + g[i].XAdvance; e > end {
				end = e
			}
			c := g[i].Cluster
			if c >= 0 && c < n {
				if !filled[c] || g[i].X < xs[c] {
					xs[c] = g[i].X
					filled[c] = true
				}
			}
		}
		gstart := len(glyphs)
		glyphs = append(glyphs, g...)
		gruns = append(gruns, LineGlyphRun{Face: r.face, Start: gstart, End: len(glyphs),
			IsColor: isColorText(seg), TextStart: r.start, TextEnd: r.end})
		cursor = end
		runeBase += utf8.RuneCountInString(seg)
	}
	carets := make([]GlyphCaret, 0, n+1)
	prevX := prevX0
	for ri := 0; ri < n; ri++ {
		x := prevX
		if filled[ri] {
			x = xs[ri]
		}
		carets = append(carets, GlyphCaret{ByteOff: runeByte[ri], X: x})
		prevX = x
	}
	carets = append(carets, GlyphCaret{ByteOff: len(line), X: cursor})
	return carets, cursor, glyphs, gruns, true
}

// RowForY returns the row whose band contains y, mirroring the legacy
// linear scan predicate (y in [top-0.01, top+h-0.01), first hit wins).
// Out-of-range y falls through to row 0, same as the old loop. O(log n).
func (l *TextLayout) RowForY(y float64) int {
	if l == nil || len(l.lines) == 0 {
		return 0
	}
	idx := l.index()
	n := len(l.lines)
	j := sort.Search(n, func(i int) bool {
		return y < idx.tops[i]+l.LineHeight(i)-0.01
	})
	if j < n && y >= idx.tops[j]-0.01 {
		return j
	}
	return 0
}

// CaretForOffset returns line index and X for a global byte offset (downstream affinity).
func (l *TextLayout) CaretForOffset(off int) (lineIdx int, x float64, ok bool) {
	if l == nil || len(l.lines) == 0 {
		return 0, 0, false
	}
	x, y, _, ok := l.GetOffsetForCaret(off, AffinityDownstream, 1.5)
	if !ok {
		return 0, 0, false
	}
	// Map y back to line index via the height prefix (O(log n)).
	idx := l.index()
	n := len(l.lines)
	j := sort.Search(n, func(i int) bool {
		return y < idx.tops[i]+l.LineHeight(i)-0.01
	})
	if j < n && y >= idx.tops[j]-0.01 {
		return j, x, true
	}
	// Past end.
	return n - 1, x, true
}

// HitTest returns byte offset for a point (x,y) in text-local coords.
// BuildRenderTextLayout builds a TextLayout for a RenderText, handling both
// single-string and multi-run (Paragraph) cases. Single-string delegates to
// BuildTextLayout; multi-run shapes per-run with its own Face/FontSize so
// mixed-size CJK/latin caret X is precise (Flutter TextPainter/SkParagraph).
func BuildRenderTextLayout(t *RenderText) *TextLayout {
	if t == nil {
		return nil
	}
	if !t.hasRuns() {
		return BuildTextLayoutEx(t.Text, t.effectiveFace(), t.fontSize(), t.MaxWidth, t.lineSpacing(), t.approxCharW(), t.MaxLines, t.Overflow)
	}
	// Multi-run paragraph: per-run shaping + per-line max height.
	maxW := t.MaxWidth
	lineSpacing := t.lineSpacing()
	if t.Text == "" {
		gen := textLayoutGen.Add(1)
		return (&TextLayout{Text: t.Text, FontSize: t.fontSize(), LineSpacing: lineSpacing, Generation: gen, MaxWidth: maxW, Face: t.effectiveFace(), MaxLines: t.MaxLines, Overflow: t.Overflow}).finishLayout()
	}
	// Helper to flush current line.
	var lines []TextLayoutLine
	curStart := 0
	globalOff := 0
	curX := 0.0
	curHeight := 0.0
	curCarets := []GlyphCaret{{ByteOff: 0, X: 0}}
	flush := func() {
		if len(curCarets) <= 1 && curX == 0 && curHeight == 0 {
			return
		}
		// Ensure final caret at line end is present (curCarets already ends at globalOff).
		// Width is curX, Height is max of runs in this line.
		h := curHeight
		if h <= 0 {
			h = lineHeightFor(nil, t.fontSize(), lineSpacing)
		}
		lines = append(lines, TextLayoutLine{
			StartByte: curStart,
			EndByte:   globalOff,
			Carets:    append([]GlyphCaret(nil), curCarets...),
			Width:     curX,
			Height:    h,
		})
	}
	for _, run := range t.Runs {
		if run.Text == "" {
			continue
		}
		parts := strings.Split(strings.ReplaceAll(strings.ReplaceAll(run.Text, "\r\n", "\n"), "\r", "\n"), "\n")
		for pi, part := range parts {
			if pi > 0 {
				// Hard break: close current line and account for '\n' byte.
				flush()
				// '\n' itself is 1 byte in t.Text (runs are concatenated with original \n).
				globalOff++
				curStart = globalOff
				curX = 0
				curHeight = 0
				curCarets = []GlyphCaret{{ByteOff: 0, X: 0}}
			}
			remain := part
			for remain != "" {
				rh := t.runLineHeight(run)
				if rh > curHeight {
					curHeight = rh
				}
				budget := 1e12
				if maxW > 0 {
					budget = maxW - curX
					if budget < 1 && curX > 0 {
						flush()
						curStart = globalOff
						curX = 0
						curHeight = rh
						curCarets = []GlyphCaret{{ByteOff: 0, X: 0}}
						budget = maxW
					}
				}
				chunk, rest := fitRunPrefix(t, run, remain, budget)
				if chunk == "" {
					rs := []rune(remain)
					if len(rs) == 0 {
						break
					}
					chunk = string(rs[0])
					rest = string(rs[1:])
					if curX > 0 && maxW > 0 {
						flush()
						curStart = globalOff
						curX = 0
						curHeight = rh
						curCarets = []GlyphCaret{{ByteOff: 0, X: 0}}
					}
				}
				// Emit per-rune carets for chunk to match paint's per-rune advances.
				for _, r := range chunk {
					adv := t.measureRunString(run, string(r))
					curX += adv
					bLen := utf8.RuneLen(r)
					globalOff += bLen
					curCarets = append(curCarets, GlyphCaret{ByteOff: globalOff - curStart, X: curX})
				}
				remain = rest
				if maxW > 0 && curX >= maxW-0.5 && remain != "" {
					flush()
					curStart = globalOff
					curX = 0
					curHeight = 0
					curCarets = []GlyphCaret{{ByteOff: 0, X: 0}}
				}
			}
			// If part was empty (consecutive \n or trailing), remain=="" loop does nothing;
			// hard-break handling above already flushed.
		}
		// Note: run.Text's \n bytes between parts were accounted via globalOff++ above.
		// No extra increment needed here beyond the per-rune loop.
	}
	// Close last line.
	flush()
	// MaxLines / overflow truncation (mirror layoutRunLines).
	truncated := false
	if t.MaxLines > 0 && len(lines) > t.MaxLines {
		lines = lines[:t.MaxLines]
		truncated = true
		// Ellipsis/clip would alter last line width but caret beyond truncation is not needed for editor.
	}
	if len(lines) == 0 {
		gen := textLayoutGen.Add(1)
		return (&TextLayout{Text: t.Text, FontSize: t.fontSize(), LineSpacing: lineSpacing, Generation: gen, MaxWidth: maxW, Face: t.effectiveFace(), MaxLines: t.MaxLines, Overflow: t.Overflow}).finishLayout()
	}
	gen := textLayoutGen.Add(1)
	return (&TextLayout{Text: t.Text, lines: lines, FontSize: t.fontSize(), LineSpacing: lineSpacing, Generation: gen, MaxWidth: maxW, Face: t.effectiveFace(), MaxLines: t.MaxLines, Overflow: t.Overflow, Truncated: truncated}).finishLayout()
}

// BoxesForRange mirrors Flutter getBoxesForRange — line-box union for a byte range.
// Returned boxes are in text-local coords (X from Carets, Y from LineTop, H from LineHeight).
// Empty or out-of-range input returns nil. Boxes are clipped to the line's carets.
func (l *TextLayout) BoxesForRange(startByte, endByte int) []Rect {
	if l == nil || len(l.lines) == 0 || startByte >= endByte {
		return nil
	}
	if startByte < 0 {
		startByte = 0
	}
	if endByte > len(l.Text) {
		endByte = len(l.Text)
	}
	var out []Rect
	idx := l.index()
	lo, hi := idx.rowRangeForSpan(startByte, endByte)
	if hi >= lo {
		if n := hi - lo + 1; n > 0 && n <= len(l.lines) {
			out = make([]Rect, 0, n)
		}
	}
	for i := lo; i <= hi && i < len(l.lines); i++ {
		if i < 0 {
			continue
		}
		ln := l.lines[i]
		if endByte <= ln.StartByte || startByte >= ln.EndByte {
			continue
		}
		s := startByte
		if s < ln.StartByte {
			s = ln.StartByte
		}
		e := endByte
		if e > ln.EndByte {
			e = ln.EndByte
		}
		x0, found0 := caretXForOffset(ln.Carets, s-ln.StartByte)
		if !found0 {
			x0 = 0
		}
		x1, found1 := caretXForOffset(ln.Carets, e-ln.StartByte)
		if !found1 {
			x1 = ln.Width
		}
		y := l.LineTop(i)
		h := ln.Height
		if x1 < x0 {
			x1 = x0
		}
		out = append(out, NewRect(x0, y, x1-x0, h))
	}
	return out
}

// Flutter-aligned caret APIs — TextPainter.getOffsetForCaret / getPositionForOffset / getFullHeightForCaret
const (
	AffinityDownstream = 0
	AffinityUpstream   = 1
)

// isNewlineAt checks if text[byteOff-1] is a hard line break (mirrors WordBoundary._isNewline).
func isNewlineAt(text string, byteOff int) bool {
	if byteOff <= 0 || byteOff > len(text) {
		return false
	}
	// Only need to check the single byte before, as our text uses '\n' for breaks.
	// For multi-byte newline variants (0x2028 etc.) we check rune.
	r, _ := utf8.DecodeLastRuneInString(text[:byteOff])
	switch r {
	case '\n', 0x0085, 0x000B, 0x000C, 0x2028, 0x2029:
		return true
	}
	return false
}

// lineClusters返回行内簇起点(绝对偏移,末尾附行尾哨兵),首算O(L)后缓存.
// 簇不跨\n(GB4/GB5控制字符恒切分),故按行切分与整串切分一致.
func (l *TextLayout) lineClusters(li int) []int {
	if l == nil || li < 0 || li >= len(l.lines) {
		return nil
	}
	ln := &l.lines[li]
	if ln.clusters == nil {
		// 撕裂快照的行界可能越过 Text,钳制后再切分.
		base, end := ln.StartByte, ln.EndByte
		if base < 0 {
			base = 0
		}
		if end > len(l.Text) {
			end = len(l.Text)
		}
		if base > end {
			base = end
		}
		rel := text.ClusterStarts(l.Text[base:end])
		abs := make([]int, len(rel))
		for i, o := range rel {
			abs[i] = base + o
		}
		ln.clusters = abs
	}
	return ln.clusters
}

// snapToCluster把off吸附到所在簇边界(行内,affinity语义同SnapCluster).
// 已在边界或行区间外保持不动.起点表行内缓存,吸附走共享单源.
func (l *TextLayout) snapToCluster(li int, off int, affinity int) int {
	if l == nil || li < 0 || li >= len(l.lines) {
		return off
	}
	ln := &l.lines[li]
	if off <= ln.StartByte || off >= ln.EndByte {
		return off
	}
	return text.SnapInStarts(l.lineClusters(li), off, affinity != AffinityUpstream)
}

// clampOffset把字节偏移钳制进Text区间(撕裂快照的行界可能越界).
func (l *TextLayout) clampOffset(off int) int {
	if off < 0 {
		return 0
	}
	if l != nil && off > len(l.Text) {
		return len(l.Text)
	}
	return off
}

// snapToGraphemeBoundary snaps byteOff to a UAX#29 cluster boundary per affinity.
func (l *TextLayout) snapToGraphemeBoundary(byteOff int, affinity int) int {
	if l == nil || l.Text == "" {
		return byteOff
	}
	if byteOff < 0 {
		return 0
	}
	if byteOff > len(l.Text) {
		return len(l.Text)
	}
	row := l.index().rowForOffset(byteOff)
	ln := l.lines[row]
	if byteOff < ln.StartByte || byteOff > ln.EndByte {
		return byteOff
	}
	return l.snapToCluster(row, byteOff, affinity)
}

// GetOffsetForCaret mirrors Flutter TextPainter.getOffsetForCaret.
// byteOff is a UTF-8 byte offset on a grapheme boundary, affinity selects leading vs trailing edge.
// caretWidth is the prototype width (1.5) for RTL adjustment.
func (l *TextLayout) GetOffsetForCaret(byteOff int, affinity int, caretWidth float64) (x, y, h float64, ok bool) {
	if l == nil || len(l.lines) == 0 {
		return 0, 0, 0, false
	}
	if l.Text == "" {
		// Empty paragraph: top-left.
		return 0, 0, l.LineHeight(0), true
	}
	byteOff = l.snapToGraphemeBoundary(byteOff, affinity)
	if byteOff < 0 {
		byteOff = 0
	}
	if byteOff > len(l.Text) {
		byteOff = len(l.Text)
	}
	// Flutter affinity rules:
	// 0 -> leading (downstream). Upstream with newline-1 -> still leading at new line.
	// Otherwise upstream -> trailing of previous grapheme.
	effectiveOff := byteOff
	effectiveAffinity := affinity
	if affinity == AffinityUpstream {
		if byteOff == 0 {
			effectiveOff = 0
			effectiveAffinity = AffinityDownstream
		} else if isNewlineAt(l.Text, byteOff) {
			effectiveOff = byteOff
			effectiveAffinity = AffinityDownstream
		} else {
			// Trailing edge of previous grapheme: keep byteOff but mark as trailing.
			// For X we will use the caret at byteOff (which is start of next grapheme)
			// but if that caret is at line start, upstream should be previous line end.
			// So detect line-start case below.
		}
		_ = effectiveAffinity
	}
	// Find line for effectiveOff via the row index (O(log n),首个命中语义).
	idx := l.index()
	row := idx.rowForOffset(effectiveOff)
	ln := l.lines[row]
	if effectiveOff >= ln.StartByte && effectiveOff <= ln.EndByte {
		i := row
		// If upstream and effectiveOff == StartByte of this line and not first line,
		// Flutter's upstream at line start should be trailing of prev line.
		if affinity == AffinityUpstream && effectiveOff == ln.StartByte && i > 0 && !isNewlineAt(l.Text, effectiveOff) {
			prev := l.lines[i-1]
			// Return trailing of previous line.
			if len(prev.Carets) > 0 {
				last := prev.Carets[len(prev.Carets)-1]
				return last.X, l.LineTop(i - 1), prev.Height, true
			}
		}
		// Normal: find X within this line via caret binary search.
		if x, exact := caretXForOffset(ln.Carets, effectiveOff-ln.StartByte); exact {
			return x, l.LineTop(i), ln.Height, true
		}
		// Fallback mid-grapheme (should not happen after snap).
		if x, ok := caretXInSpan(ln.Carets, effectiveOff-ln.StartByte); ok {
			return x, l.LineTop(i), ln.Height, true
		}
		if len(ln.Carets) > 0 {
			return ln.Carets[len(ln.Carets)-1].X, l.LineTop(i), ln.Height, true
		}
		return ln.Width, l.LineTop(i), ln.Height, true
	}
	// Past end → end-of-text caret (Flutter _endOfTextCaretMetrics).
	last := l.lines[len(l.lines)-1]
	if len(last.Carets) > 0 {
		x = last.Carets[len(last.Carets)-1].X
	} else {
		x = last.Width
	}
	// RTL shift would be x - caretWidth, but we are LTR.
	if caretWidth != 0 {
		_ = caretWidth
	}
	return x, l.LineTop(len(l.lines) - 1), last.Height, true
}

// GetFullHeightForCaret mirrors TextPainter.getFullHeightForCaret.
func (l *TextLayout) GetFullHeightForCaret(byteOff int, affinity int) float64 {
	if _, _, h, ok := l.GetOffsetForCaret(byteOff, affinity, 1.5); ok {
		return h
	}
	return l.LineHeight(0)
}

// GetPositionForOffset mirrors TextPainter.getPositionForOffset.
// Returns byte offset + affinity for a visual point (x,y) in text-local coords.
func (l *TextLayout) GetPositionForOffset(x, y float64) (byteOff int, affinity int) {
	if l == nil || len(l.lines) == 0 {
		return 0, AffinityDownstream
	}
	// Find line by y via the height prefix (O(log n),首个y<top+h语义).
	idx := l.index()
	row := idx.rowForY(y, l.FontSize, l.LineSpacing, l.LineHeight)
	ln := l.lines[row]
	if x <= 0 {
		return l.clampOffset(ln.StartByte), AffinityDownstream
	}
	if len(ln.Carets) == 0 {
		return l.clampOffset(ln.StartByte), AffinityDownstream
	}
	if len(ln.Carets) == 1 {
		return l.clampOffset(ln.Carets[0].ByteOff), AffinityDownstream
	}
	// Mid-point rule for nearest caret, but also set affinity:
	// If x is in left half of a grapheme, affinity downstream (leading), else upstream (trailing).
	// For line-start/end edge, follow Flutter's line-break affinity.
	// M1-c:结果再按簇吸附(downstream),不得落进组合音标/ZWJ序列内.
	// caret为行内相对,先加行基址转绝对再吸附.
	off, aff := offsetForX(ln.Carets, x)
	off += ln.StartByte
	return l.clampOffset(l.snapToCluster(row, off, aff)), aff
}

func (l *TextLayout) HitTest(x, y float64, lineHeight float64) int {
	off, _ := l.GetPositionForOffset(x, y)
	return off
}
