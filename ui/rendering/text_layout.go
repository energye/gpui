package rendering

import (
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
	Lines    []TextLayoutLine
	FontSize float64
	// LineSpacing is the multiplier used when building this layout; kept for HitTest fallback.
	LineSpacing float64
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

// BuildTextLayout produces single-source layout.
func BuildTextLayout(textStr string, face text.Face, fontSize float64, maxWidth float64, lineSpacing float64) *TextLayout {
	if textStr == "" {
		return &TextLayout{FontSize: fontSize, LineSpacing: lineSpacing}
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
	} else {
		wrapped = text.WrapText(textStr, face, maxWidth, text.WrapWord)
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
	return &TextLayout{Lines: lines, FontSize: fontSize, LineSpacing: lineSpacing}
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
	glyphs := text.Shape(line, face)
	if len(glyphs) == 0 {
		var carets []GlyphCaret
		var sg []text.ShapedGlyph
		carets = append(carets, GlyphCaret{ByteOff: 0, X: 0})
		x := 0.0
		for byteOff, r := range line {
			adv, _ := text.Measure(string(r), face)
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
	// Build map from cluster (rune index) to X and byte offset
	// Cluster is rune index in line
	// Need byte offsets per rune index
	runeByte := make([]int, 0)
	for i := range line {
		if utf8.RuneStart(line[i]) {
			runeByte = append(runeByte, i)
		}
	}
	runeByte = append(runeByte, len(line))
	// Glyphs are in visual order with X already
	// For caret we need ordered by X
	// Build carets: one per rune boundary, using CaretXForCluster
	n := len(runeByte) - 1 // rune count
	var carets []GlyphCaret
	for ri := 0; ri <= n; ri++ {
		x := text.CaretXForCluster(glyphs, ri)
		byteOff := 0
		if ri < len(runeByte) {
			byteOff = runeByte[ri]
		} else {
			byteOff = len(line)
		}
		carets = append(carets, GlyphCaret{ByteOff: byteOff, X: x})
	}
	var width float64
	if len(glyphs) > 0 {
		last := glyphs[len(glyphs)-1]
		width = last.X + last.XAdvance
	}
	return carets, width, glyphs
}

// CaretForOffset returns line index and X for a global byte offset.
func (l *TextLayout) CaretForOffset(off int) (lineIdx int, x float64, ok bool) {
	if l == nil || len(l.Lines) == 0 {
		return 0, 0, false
	}
	if off < 0 {
		off = 0
	}
	for i, ln := range l.Lines {
		if off >= ln.StartByte && off <= ln.EndByte {
			// find nearest caret
			for _, c := range ln.Carets {
				if c.ByteOff == off {
					return i, c.X, true
				}
			}
			// between boundaries: interpolate by nearest
			// find insertion
			for j := 0; j < len(ln.Carets)-1; j++ {
				if off > ln.Carets[j].ByteOff && off < ln.Carets[j+1].ByteOff {
					return i, ln.Carets[j].X, true
				}
			}
			if len(ln.Carets) > 0 {
				return i, ln.Carets[len(ln.Carets)-1].X, true
			}
		}
	}
	// past end
	last := l.Lines[len(l.Lines)-1]
	if len(last.Carets) > 0 {
		return len(l.Lines) - 1, last.Carets[len(last.Carets)-1].X, true
	}
	return len(l.Lines) - 1, last.Width, true
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
		return BuildTextLayout(t.Text, t.effectiveFace(), t.fontSize(), t.MaxWidth, t.lineSpacing())
	}
	// Multi-run paragraph: per-run shaping + per-line max height.
	if t.Text == "" {
		return &TextLayout{FontSize: t.fontSize(), LineSpacing: t.lineSpacing()}
	}
	maxW := t.MaxWidth
	lineSpacing := t.lineSpacing()
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
	if t.MaxLines > 0 && len(lines) > t.MaxLines {
		lines = lines[:t.MaxLines]
		// Ellipsis/clip would alter last line width but caret beyond truncation is not needed for editor.
	}
	if len(lines) == 0 {
		return &TextLayout{FontSize: t.fontSize(), LineSpacing: lineSpacing}
	}
	return &TextLayout{Lines: lines, FontSize: t.fontSize(), LineSpacing: lineSpacing}
}

func (l *TextLayout) HitTest(x, y float64, lineHeight float64) int {
	if l == nil || len(l.Lines) == 0 {
		return 0
	}
	// Use per-line heights when available (mixed-size paragraph: Flutter SkParagraph).
	if len(l.Lines) > 0 && l.Lines[0].Height > 0 {
		yTop := 0.0
		row := 0
		for i, ln := range l.Lines {
			h := ln.Height
			if h <= 0 {
				h = lineHeight
				if h <= 0 {
					h = l.FontSize * 1.2
				}
			}
			if y < yTop+h || i == len(l.Lines)-1 {
				row = i
				break
			}
			yTop += h
		}
		ln := l.Lines[row]
		if x <= 0 {
			return ln.StartByte
		}
		best := ln.Carets[0]
		if len(ln.Carets) == 1 {
			return best.ByteOff
		}
		for i := 0; i < len(ln.Carets)-1; i++ {
			a := ln.Carets[i]
			b := ln.Carets[i+1]
			mid := (a.X + b.X) * 0.5
			if x < mid {
				return a.ByteOff
			}
			if x < b.X {
				if x < mid {
					return a.ByteOff
				}
				return b.ByteOff
			}
		}
		return ln.Carets[len(ln.Carets)-1].ByteOff
	}
	if lineHeight <= 0 {
		lineHeight = l.FontSize * 1.2
	}
	row := int(y / lineHeight)
	if row < 0 {
		row = 0
	}
	if row >= len(l.Lines) {
		row = len(l.Lines) - 1
	}
	ln := l.Lines[row]
	if x <= 0 {
		return ln.StartByte
	}
	// find nearest caret by X mid-point rule
	best := ln.Carets[0]
	if len(ln.Carets) == 1 {
		return best.ByteOff
	}
	for i := 0; i < len(ln.Carets)-1; i++ {
		a := ln.Carets[i]
		b := ln.Carets[i+1]
		mid := (a.X + b.X) * 0.5
		if x < mid {
			return a.ByteOff
		}
		if x < b.X {
			// choose nearer
			if x < mid {
				return a.ByteOff
			}
			return b.ByteOff
		}
	}
	return ln.Carets[len(ln.Carets)-1].ByteOff
}
