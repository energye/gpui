package rendering

import (
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
	Glyphs    []text.ShapedGlyph
}

// TextLayout is the single source for paint + queries.
type TextLayout struct {
	Lines    []TextLayoutLine
	FontSize float64
}

// BuildTextLayout produces single-source layout.
// textStr is the full display text (already contains preedit).
func BuildTextLayout(textStr string, face text.Face, fontSize float64, maxWidth float64, lineSpacing float64) *TextLayout {
	if textStr == "" {
		return &TextLayout{FontSize: fontSize}
	}
	if fontSize <= 0 {
		fontSize = 14
	}
	var lines []TextLayoutLine
	// Wrap into visual lines
	wrapped := text.WrapText(textStr, face, maxWidth, text.WrapWord)
	if len(wrapped) == 0 {
		wrapped = []text.WrapResult{{Text: textStr, Start: 0, End: len(textStr)}}
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
			Glyphs:    glyphs,
		})
	}
	return &TextLayout{Lines: lines, FontSize: fontSize}
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
		for idx := range line {
			if !utf8.RuneStart(line[idx]) {
				continue
			}
			_, sz := utf8.DecodeRuneInString(line[idx:])
			next := idx + sz
			w, _ := text.Measure(line[:next], face)
			prevW, _ := text.Measure(line[:idx], face)
			adv := w - prevW
			sg = append(sg, text.ShapedGlyph{GID: 0, Cluster: 0, X: prevW, XAdvance: adv})
			carets = append(carets, GlyphCaret{ByteOff: next, X: w})
			_ = x
			if next >= len(line) {
				break
			}
		}
		w, _ := text.Measure(line, face)
		return carets, w, sg
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
func (l *TextLayout) HitTest(x, y float64, lineHeight float64) int {
	if l == nil || len(l.Lines) == 0 {
		return 0
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
