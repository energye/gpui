package rendering

import (
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
func (t *RenderText) SetFace(face text.Face) {
	if t == nil {
		return
	}
	t.Face = face
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetMaxWidth enables (>0) or disables (≤0) wrap; dirties layout+paint.
func (t *RenderText) SetMaxWidth(w float64) {
	if t == nil || t.MaxWidth == w {
		return
	}
	t.MaxWidth = w
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

// lineHeightLogical is the per-line advance used for layout height and multi-line paint.
func (t *RenderText) lineHeightLogical() float64 {
	fs := t.fontSize()
	ls := t.lineSpacing()
	if t.Face != nil {
		m := t.Face.Metrics()
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
	if t.Face != nil {
		w, _ := text.Measure(s, t.Face)
		return w
	}
	fs := t.fontSize()
	return float64(utf8.RuneCountInString(s)) * fs * t.approxCharW()
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
	if t.Face != nil {
		res := text.WrapText(s, t.Face, maxW, text.WrapWord)
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

// DisplayLines returns the visible lines after wrap + maxLines + overflow.
// Layout and paint both use this so measured size matches what is drawn.
// For multi-run content, each line is the concatenation of that line's span texts.
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
			s = s // fall through to binary search on s+ellipsis
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

// Paint implements RenderObject.
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
		lines := t.DisplayLines()
		if len(lines) > 0 {
			a := t.A
			if a == 0 && (t.R != 0 || t.G != 0 || t.B != 0) {
				a = 1
			}
			if t.Face != nil && pc.DC != nil {
				pc.DC.SetFont(t.Face)
			}
			fs := t.fontSize()
			lh := t.lineHeightLogical()
			// Y uses FontSize as first baseline (single-line MVP convention); subsequent lines step by lh.
			for i, line := range lines {
				if line == "" {
					continue
				}
				y := fs + float64(i)*lh
				drawTextColored(pc, line, 0, y, t.R, t.G, t.B, a)
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
	// First baseline uses parent FontSize (same convention as single-string path).
	baseline := t.fontSize()
	for i, ln := range lines {
		if i > 0 {
			baseline += lines[i-1].Height
		}
		for _, sp := range ln.Spans {
			if sp.Text == "" {
				continue
			}
			if sp.Face != nil && pc.DC != nil {
				pc.DC.SetFont(sp.Face)
			} else if t.Face != nil && pc.DC != nil {
				pc.DC.SetFont(t.Face)
			}
			drawTextColored(pc, sp.Text, sp.X, baseline, sp.R, sp.G, sp.B, sp.A)
		}
	}
}

// HitTest implements RenderObject.
func (t *RenderText) HitTest(p Point) RenderObject {
	sz := t.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return t
	}
	return nil
}
