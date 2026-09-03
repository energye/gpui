package rendering

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
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
}

// TextLayout is the single source for paint + queries.
type TextLayout struct {
	Text     string
	Lines    []TextLayoutLine
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

var textLayoutGen uint64

// SnapPixel 对齐到物理像素（HiDPI 1.25/2.0 1px 采样，F0–F9）.
func SnapPixel(x, scale float64) float64 {
	if scale <= 0 {
		return x
	}
	return math.Round(x*scale) / scale
}

// TextLayout.SnappedX 返回按 scale 像素对齐的 caret X（HiDPI）。
func (l *TextLayout) SnappedX(byteOff int, scale float64) (float64, bool) {
	if l == nil || len(l.Lines) == 0 {
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
		textLayoutGen++
		return &TextLayout{Text: textStr, FontSize: fontSize, LineSpacing: lineSpacing, Generation: textLayoutGen, MaxWidth: maxWidth, Face: face, MaxLines: maxLines, Overflow: overflow}
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
		lineText := w.Text
		start := w.Start
		end := w.End
		carets, width, glyphs := buildCaretsForLine(lineText, face)
		for i := range carets {
			carets[i].ByteOff += start
		}
		// Adjust glyph X to be relative to line start (already 0) and keep
		if len(carets) == 0 {
			carets = []GlyphCaret{{ByteOff: start, X: 0}, {ByteOff: end, X: 0}}
		} else {
			if carets[len(carets)-1].ByteOff != end {
				carets = append(carets, GlyphCaret{ByteOff: end, X: width})
			}
		}
		lines = append(lines, TextLayoutLine{
			StartByte: start,
			EndByte:   end,
			Carets:    carets,
			Width:     width,
			Height:    lh,
			Glyphs:    glyphs,
		})
	}
	truncated := false
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	textLayoutGen++
	return &TextLayout{Text: textStr, Lines: lines, FontSize: fontSize, LineSpacing: lineSpacing, Generation: textLayoutGen, MaxWidth: maxWidth, Face: face, MaxLines: maxLines, Overflow: overflow, Truncated: truncated}
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
	top := 0.0
	for i := 0; i < idx && i < len(l.Lines); i++ {
		h := l.Lines[i].Height
		if h <= 0 {
			h = lineHeightFor(nil, l.FontSize, l.LineSpacing)
		}
		top += h
	}
	return top
}

// LineHeight returns the height of line idx, falling back to uniform calculation.
func (l *TextLayout) LineHeight(idx int) float64 {
	if l == nil || len(l.Lines) == 0 {
		return lineHeightFor(nil, l.FontSize, l.LineSpacing)
	}
	if idx >= 0 && idx < len(l.Lines) {
		if h := l.Lines[idx].Height; h > 0 {
			return h
		}
	}
	return lineHeightFor(nil, l.FontSize, l.LineSpacing)
}

func buildCaretsForLine(line string, face text.Face) ([]GlyphCaret, float64, []text.ShapedGlyph) {
	if line == "" {
		return []GlyphCaret{{ByteOff: 0, X: 0}}, 0, nil
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
		return carets, x, glyphs
	}
	// Shaped path (M0 item 8): split into (face, script, direction) runs and
	// shape each run, then stitch carets in a single pass — O(n), no
	// CaretXForCluster table scan. Any unshapable run falls back below.
	if runs := itemizeRuns(line, face); len(runs) > 0 {
		if carets, width, glyphs, ok := buildShapedCarets(line, runs); ok {
			return carets, width, glyphs
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
	return carets, x, sg
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
func buildShapedCarets(line string, runs []itemizedRun) ([]GlyphCaret, float64, []text.ShapedGlyph, bool) {
	runeByte := make([]int, 0)
	for i := range line {
		if utf8.RuneStart(line[i]) {
			runeByte = append(runeByte, i)
		}
	}
	runeByte = append(runeByte, len(line))
	n := len(runeByte) - 1
	if n <= 0 {
		return nil, 0, nil, false
	}
	xs := make([]float64, n)
	filled := make([]bool, n)
	var glyphs []text.ShapedGlyph
	cursor := 0.0
	runeBase := 0
	for _, r := range runs {
		seg := line[r.start:r.end]
		// Shape results may be cache-owned: copy before remapping in place.
		sg := text.Shape(seg, r.face)
		if len(sg) == 0 {
			return nil, 0, nil, false
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
		glyphs = append(glyphs, g...)
		cursor = end
		runeBase += utf8.RuneCountInString(seg)
	}
	carets := make([]GlyphCaret, 0, n+1)
	prevX := 0.0
	for ri := 0; ri < n; ri++ {
		x := prevX
		if filled[ri] {
			x = xs[ri]
		}
		carets = append(carets, GlyphCaret{ByteOff: runeByte[ri], X: x})
		prevX = x
	}
	carets = append(carets, GlyphCaret{ByteOff: len(line), X: cursor})
	return carets, cursor, glyphs, true
}

// CaretForOffset returns line index and X for a global byte offset (downstream affinity).
func (l *TextLayout) CaretForOffset(off int) (lineIdx int, x float64, ok bool) {
	if l == nil || len(l.Lines) == 0 {
		return 0, 0, false
	}
	x, y, _, ok := l.GetOffsetForCaret(off, AffinityDownstream, 1.5)
	if !ok {
		return 0, 0, false
	}
	// Map y back to line index via LineTop.
	for i := range l.Lines {
		top := l.LineTop(i)
		h := l.LineHeight(i)
		if y >= top-0.01 && y < top+h-0.01 {
			return i, x, true
		}
	}
	// Past end.
	return len(l.Lines) - 1, x, true
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
		textLayoutGen++
		return &TextLayout{Text: t.Text, FontSize: t.fontSize(), LineSpacing: lineSpacing, Generation: textLayoutGen, MaxWidth: maxW, Face: t.effectiveFace(), MaxLines: t.MaxLines, Overflow: t.Overflow}
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
				curCarets = []GlyphCaret{{ByteOff: curStart, X: 0}}
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
						curCarets = []GlyphCaret{{ByteOff: curStart, X: 0}}
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
						curCarets = []GlyphCaret{{ByteOff: curStart, X: 0}}
					}
				}
				// Emit per-rune carets for chunk to match paint's per-rune advances.
				for _, r := range chunk {
					adv := t.measureRunString(run, string(r))
					curX += adv
					bLen := utf8.RuneLen(r)
					globalOff += bLen
					curCarets = append(curCarets, GlyphCaret{ByteOff: globalOff, X: curX})
				}
				remain = rest
				if maxW > 0 && curX >= maxW-0.5 && remain != "" {
					flush()
					curStart = globalOff
					curX = 0
					curHeight = 0
					curCarets = []GlyphCaret{{ByteOff: curStart, X: 0}}
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
		textLayoutGen++
		return &TextLayout{Text: t.Text, FontSize: t.fontSize(), LineSpacing: lineSpacing, Generation: textLayoutGen, MaxWidth: maxW, Face: t.effectiveFace(), MaxLines: t.MaxLines, Overflow: t.Overflow}
	}
	textLayoutGen++
	return &TextLayout{Text: t.Text, Lines: lines, FontSize: t.fontSize(), LineSpacing: lineSpacing, Generation: textLayoutGen, MaxWidth: maxW, Face: t.effectiveFace(), MaxLines: t.MaxLines, Overflow: t.Overflow, Truncated: truncated}
}

// BoxesForRange mirrors Flutter getBoxesForRange — line-box union for a byte range.
// Returned boxes are in text-local coords (X from Carets, Y from LineTop, H from LineHeight).
// Empty or out-of-range input returns nil. Boxes are clipped to the line's carets.
func (l *TextLayout) BoxesForRange(startByte, endByte int) []Rect {
	if l == nil || len(l.Lines) == 0 || startByte >= endByte {
		return nil
	}
	if startByte < 0 {
		startByte = 0
	}
	if endByte > len(l.Text) {
		endByte = len(l.Text)
	}
	var out []Rect
	for i, ln := range l.Lines {
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
		var x0, x1 float64
		found0, found1 := false, false
		for _, c := range ln.Carets {
			if !found0 && c.ByteOff == s {
				x0 = c.X
				found0 = true
			}
			if !found1 && c.ByteOff == e {
				x1 = c.X
				found1 = true
			}
		}
		if !found0 {
			x0 = 0
		}
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

// snapToGraphemeBoundary snaps byteOff to a grapheme start per affinity.
// Our carets are per-rune (each rune = one grapheme for now; surrogate+ZWJ clusters would need text/segment).
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
	// If already on a rune start, keep.
	if byteOff == 0 || byteOff == len(l.Text) || utf8.RuneStart(l.Text[byteOff]) {
		return byteOff
	}
	// Inside a multi-byte rune: snap per affinity.
	if affinity == AffinityUpstream {
		for byteOff > 0 && byteOff < len(l.Text) && (l.Text[byteOff]&0xC0) == 0x80 {
			byteOff--
		}
	} else {
		for byteOff < len(l.Text) && (l.Text[byteOff]&0xC0) == 0x80 {
			byteOff++
		}
	}
	return byteOff
}

// GetOffsetForCaret mirrors Flutter TextPainter.getOffsetForCaret.
// byteOff is a UTF-8 byte offset on a grapheme boundary, affinity selects leading vs trailing edge.
// caretWidth is the prototype width (1.5) for RTL adjustment.
func (l *TextLayout) GetOffsetForCaret(byteOff int, affinity int, caretWidth float64) (x, y, h float64, ok bool) {
	if l == nil || len(l.Lines) == 0 {
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
	// Find line for effectiveOff.
	lineIdx := -1
	var lineX float64
	for i, ln := range l.Lines {
		if effectiveOff >= ln.StartByte && effectiveOff <= ln.EndByte {
			// If upstream and effectiveOff == StartByte of this line and not first line,
			// Flutter's upstream at line start should be trailing of prev line.
			if affinity == AffinityUpstream && effectiveOff == ln.StartByte && i > 0 && !isNewlineAt(l.Text, effectiveOff) {
				prev := l.Lines[i-1]
				// Return trailing of previous line.
				if len(prev.Carets) > 0 {
					last := prev.Carets[len(prev.Carets)-1]
					return last.X, l.LineTop(i - 1), prev.Height, true
				}
			}
			// Normal: find X within this line.
			for _, c := range ln.Carets {
				if c.ByteOff == effectiveOff {
					return c.X, l.LineTop(i), ln.Height, true
				}
			}
			// Fallback mid-grapheme (should not happen after snap).
			for j := 0; j < len(ln.Carets)-1; j++ {
				if effectiveOff > ln.Carets[j].ByteOff && effectiveOff < ln.Carets[j+1].ByteOff {
					return ln.Carets[j].X, l.LineTop(i), ln.Height, true
				}
			}
			if len(ln.Carets) > 0 {
				return ln.Carets[len(ln.Carets)-1].X, l.LineTop(i), ln.Height, true
			}
			lineIdx = i
			lineX = ln.Width
			break
		}
	}
	if lineIdx >= 0 {
		return lineX, l.LineTop(lineIdx), l.Lines[lineIdx].Height, true
	}
	// Past end → end-of-text caret (Flutter _endOfTextCaretMetrics).
	last := l.Lines[len(l.Lines)-1]
	if len(last.Carets) > 0 {
		x = last.Carets[len(last.Carets)-1].X
	} else {
		x = last.Width
	}
	// RTL shift would be x - caretWidth, but we are LTR.
	if caretWidth != 0 {
		_ = caretWidth
	}
	return x, l.LineTop(len(l.Lines) - 1), last.Height, true
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
	if l == nil || len(l.Lines) == 0 {
		return 0, AffinityDownstream
	}
	// Find line by y (per-line heights).
	yTop := 0.0
	row := 0
	for i, ln := range l.Lines {
		h := ln.Height
		if h <= 0 {
			h = l.FontSize * 1.2
		}
		if y < yTop+h || i == len(l.Lines)-1 {
			row = i
			break
		}
		yTop += h
	}
	ln := l.Lines[row]
	if x <= 0 {
		return ln.StartByte, AffinityDownstream
	}
	if len(ln.Carets) == 0 {
		return ln.StartByte, AffinityDownstream
	}
	if len(ln.Carets) == 1 {
		return ln.Carets[0].ByteOff, AffinityDownstream
	}
	// Mid-point rule for nearest caret, but also set affinity:
	// If x is in left half of a grapheme, affinity downstream (leading), else upstream (trailing).
	// For line-start/end edge, follow Flutter's line-break affinity.
	for i := 0; i < len(ln.Carets)-1; i++ {
		a := ln.Carets[i]
		b := ln.Carets[i+1]
		mid := (a.X + b.X) * 0.5
		if x < mid {
			return a.ByteOff, AffinityDownstream
		}
		if x < b.X {
			// Between mid and b.X -> nearer to b.
			return b.ByteOff, AffinityDownstream
		}
	}
	return ln.Carets[len(ln.Carets)-1].ByteOff, AffinityDownstream
}

func (l *TextLayout) HitTest(x, y float64, lineHeight float64) int {
	off, _ := l.GetPositionForOffset(x, y)
	return off
}
