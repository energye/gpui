package rendering

import (
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TextRun is one styled span inside a multi-run paragraph (minimal ParagraphBuilder model).
// Empty Face falls back to the parent RenderText.Face / estimate metrics.
type TextRun struct {
	Text        string
	Face        text.Face
	FontSize    float64 // 0 → inherit parent FontSize
	R, G, B, A  float64
	ApproxCharW float64 // 0 → inherit parent
	// IsColor marks color-glyph runs (emoji): they paint via the string
	// color path and never enter the mask batch (M5). Auto-detected in
	// AddRun/SetRuns; an explicit true is never cleared.
	IsColor bool
	// Decoration is a render.TextDecoration bitset (underline etc.); 0 = none.
	Decoration render.TextDecoration
}

// paragraphStyle is one stack frame for PushStyle / PopStyle.
type paragraphStyle struct {
	Face        text.Face
	FontSize    float64
	R, G, B, A  float64
	ApproxCharW float64
	Decoration  render.TextDecoration
}

// ParagraphBuilder assembles ordered TextRuns for RenderText.SetRuns.
// Supports a small style stack (PushStyle/PopStyle) for nested span styling.
// Not full Flutter ParagraphBuilder parity (no locale/strut/placeholder).
type ParagraphBuilder struct {
	runs []TextRun
	// Defaults applied when AddText is used without an explicit run style.
	Face        text.Face
	FontSize    float64
	R, G, B, A  float64
	ApproxCharW float64
	Decoration  render.TextDecoration
	styleStack  []paragraphStyle
}

// NewParagraphBuilder creates an empty builder with neutral defaults.
func NewParagraphBuilder() *ParagraphBuilder {
	return &ParagraphBuilder{
		FontSize: 14,
		R:        0.9, G: 0.9, B: 0.9, A: 1,
		ApproxCharW: 0.55,
	}
}

// SetDefaultStyle sets style used by AddText.
func (b *ParagraphBuilder) SetDefaultStyle(face text.Face, fontSize, r, g, bl, a, approxCharW float64) *ParagraphBuilder {
	if b == nil {
		return nil
	}
	b.Face = face
	b.FontSize = fontSize
	b.R, b.G, b.B, b.A = r, g, bl, a
	b.ApproxCharW = approxCharW
	return b
}

// SetDecoration sets the current text decoration bitset for subsequent AddText.
func (b *ParagraphBuilder) SetDecoration(d render.TextDecoration) *ParagraphBuilder {
	if b == nil {
		return nil
	}
	b.Decoration = d
	return b
}

// PushStyle saves the current default style and optionally overrides fields.
// Zero fontSize / nil face / negative alpha keep the previous value for that field.
// Colors: pass useColor=true to replace RGBA.
func (b *ParagraphBuilder) PushStyle(face text.Face, fontSize float64, useColor bool, r, g, bl, a float64, dec render.TextDecoration) *ParagraphBuilder {
	if b == nil {
		return nil
	}
	b.styleStack = append(b.styleStack, paragraphStyle{
		Face: b.Face, FontSize: b.FontSize,
		R: b.R, G: b.G, B: b.B, A: b.A,
		ApproxCharW: b.ApproxCharW, Decoration: b.Decoration,
	})
	if face != nil {
		b.Face = face
	}
	if fontSize > 0 {
		b.FontSize = fontSize
	}
	if useColor {
		b.R, b.G, b.B, b.A = r, g, bl, a
	}
	// dec is always applied (caller can re-SetDecoration after pop).
	b.Decoration = dec
	return b
}

// PopStyle restores the style saved by the last PushStyle. No-op if stack empty.
func (b *ParagraphBuilder) PopStyle() *ParagraphBuilder {
	if b == nil || len(b.styleStack) == 0 {
		return b
	}
	top := b.styleStack[len(b.styleStack)-1]
	b.styleStack = b.styleStack[:len(b.styleStack)-1]
	b.Face = top.Face
	b.FontSize = top.FontSize
	b.R, b.G, b.B, b.A = top.R, top.G, top.B, top.A
	b.ApproxCharW = top.ApproxCharW
	b.Decoration = top.Decoration
	return b
}

// AddRun appends a fully specified run.
func (b *ParagraphBuilder) AddRun(run TextRun) *ParagraphBuilder {
	if b == nil {
		return nil
	}
	if run.Text == "" {
		return b
	}
	if !run.IsColor && isColorText(run.Text) {
		run.IsColor = true
	}
	b.runs = append(b.runs, run)
	return b
}

// AddText appends text using the builder default style (can change between calls).
func (b *ParagraphBuilder) AddText(s string) *ParagraphBuilder {
	if b == nil || s == "" {
		return b
	}
	return b.AddRun(TextRun{
		Text: s, Face: b.Face, FontSize: b.FontSize,
		R: b.R, G: b.G, B: b.B, A: b.A, ApproxCharW: b.ApproxCharW,
		Decoration: b.Decoration,
	})
}

// Runs returns a copy of accumulated runs.
func (b *ParagraphBuilder) Runs() []TextRun {
	if b == nil || len(b.runs) == 0 {
		return nil
	}
	out := make([]TextRun, len(b.runs))
	copy(out, b.runs)
	return out
}

// Build applies runs onto a new RenderText (MaxWidth/MaxLines still set by caller).
func (b *ParagraphBuilder) Build() *RenderText {
	t := NewRenderText("")
	if b != nil {
		t.SetRuns(b.Runs())
		if b.FontSize > 0 {
			t.FontSize = b.FontSize
		}
		t.R, t.G, t.B, t.A = b.R, b.G, b.B, b.A
		if b.Face != nil {
			t.Face = b.Face
		}
		t.Decoration = b.Decoration
	}
	return t
}

// Apply sets runs on an existing RenderText.
func (b *ParagraphBuilder) Apply(t *RenderText) {
	if b == nil || t == nil {
		return
	}
	t.SetRuns(b.Runs())
	if b.Face != nil {
		t.SetFace(b.Face)
	}
	if b.FontSize > 0 {
		t.SetFontSize(b.FontSize)
	}
}

// displaySpan is one painted fragment on a line after multi-run layout.
type displaySpan struct {
	Text        string
	Face        text.Face
	FontSize    float64
	R, G, B, A  float64
	ApproxCharW float64
	X           float64
	Width       float64
	IsColor     bool // carried from TextRun (M5 color-glyph marking)
	Decoration  render.TextDecoration
}

// displayLine is one visual line of spans.
type displayLine struct {
	Spans  []displaySpan
	Height float64
}

func (t *RenderText) hasRuns() bool {
	return t != nil && len(t.Runs) > 0
}

func (t *RenderText) runFontSize(r TextRun) float64 {
	if r.FontSize > 0 {
		return r.FontSize
	}
	return t.fontSize()
}

func (t *RenderText) runApprox(r TextRun) float64 {
	if r.ApproxCharW > 0 {
		return r.ApproxCharW
	}
	return t.approxCharW()
}

func (t *RenderText) runFace(r TextRun) text.Face {
	base := r.Face
	if base == nil {
		base = t.Face
	}
	if base == nil {
		return nil
	}
	return faceForSize(base, t.runFontSize(r))
}

func (t *RenderText) measureRunString(r TextRun, s string) float64 {
	if s == "" {
		return 0
	}
	if face := t.runFace(r); face != nil {
		w, _ := text.Measure(s, face)
		return w
	}
	fs := t.runFontSize(r)
	return float64(utf8.RuneCountInString(s)) * fs * t.runApprox(r)
}

func (t *RenderText) runLineHeight(r TextRun) float64 {
	fs := t.runFontSize(r)
	ls := t.lineSpacing()
	if face := t.runFace(r); face != nil {
		m := face.Metrics()
		if lh := m.LineHeight(); lh > 0 {
			return lh * ls
		}
	}
	return fs * 1.25 * ls
}

// layoutRunLines places runs left-to-right with soft wrap at MaxWidth.
// MaxLines / Overflow ellipsis are applied like the single-string path.
func (t *RenderText) layoutRunLines() []displayLine {
	if t == nil || len(t.Runs) == 0 {
		return nil
	}
	maxW := t.MaxWidth
	var lines []displayLine
	cur := displayLine{Height: t.lineHeightLogical()}
	x := 0.0

	flush := func() {
		if len(cur.Spans) == 0 && cur.Height <= 0 {
			return
		}
		if cur.Height <= 0 {
			cur.Height = t.lineHeightLogical()
		}
		lines = append(lines, cur)
		cur = displayLine{Height: t.lineHeightLogical()}
		x = 0
	}

	for _, run := range t.Runs {
		if run.Text == "" {
			continue
		}
		// Split hard breaks first.
		parts := strings.Split(strings.ReplaceAll(strings.ReplaceAll(run.Text, "\r\n", "\n"), "\r", "\n"), "\n")
		for pi, part := range parts {
			if pi > 0 {
				flush()
			}
			remain := part
			for remain != "" {
				rh := t.runLineHeight(run)
				if rh > cur.Height {
					cur.Height = rh
				}
				budget := 1e12
				if maxW > 0 {
					budget = maxW - x
					if budget < 1 && x > 0 {
						flush()
						budget = maxW
						if rh > cur.Height {
							cur.Height = rh
						}
					}
				}
				// Fit as many runes as possible.
				chunk, rest := fitRunPrefix(t, run, remain, budget)
				if chunk == "" {
					// Single glyph wider than budget: force one rune to avoid infinite loop.
					r := []rune(remain)
					chunk = string(r[0])
					rest = string(r[1:])
					if x > 0 && maxW > 0 {
						flush()
						if rh > cur.Height {
							cur.Height = rh
						}
					}
				}
				w := t.measureRunString(run, chunk)
				a := run.A
				if a == 0 && (run.R != 0 || run.G != 0 || run.B != 0) {
					a = 1
				}
				if a == 0 {
					a = t.A
					if a == 0 {
						a = 1
					}
				}
				rr, gg, bb := run.R, run.G, run.B
				if rr == 0 && gg == 0 && bb == 0 && run.A == 0 && chunk != "" {
					// Unspecified color → parent
					rr, gg, bb, a = t.R, t.G, t.B, t.A
					if a == 0 {
						a = 1
					}
				}
				cur.Spans = append(cur.Spans, displaySpan{
					Text: chunk, Face: t.runFace(run), FontSize: t.runFontSize(run),
					R: rr, G: gg, B: bb, A: a, ApproxCharW: t.runApprox(run),
					X: x, Width: w, IsColor: run.IsColor, Decoration: run.Decoration,
				})
				x += w
				remain = rest
				if maxW > 0 && x >= maxW-0.5 && remain != "" {
					flush()
				}
			}
		}
	}
	if len(cur.Spans) > 0 {
		flush()
	}
	if len(lines) == 0 {
		return nil
	}

	// MaxLines + overflow
	maxL := t.MaxLines
	truncated := false
	if maxL > 0 && len(lines) > maxL {
		lines = lines[:maxL]
		truncated = true
	}
	if truncated && t.Overflow == TextOverflowEllipsis && len(lines) > 0 {
		lines[len(lines)-1] = ellipsizeDisplayLine(t, lines[len(lines)-1], maxW)
	} else if maxW > 0 && len(lines) > 0 {
		last := &lines[len(lines)-1]
		total := 0.0
		for _, sp := range last.Spans {
			total += sp.Width
		}
		if total > maxW+0.5 {
			if t.Overflow == TextOverflowEllipsis {
				*last = ellipsizeDisplayLine(t, *last, maxW)
			} else {
				*last = clipDisplayLine(t, *last, maxW)
			}
		}
	}
	return lines
}

func fitRunPrefix(t *RenderText, run TextRun, s string, budget float64) (chunk, rest string) {
	if s == "" {
		return "", ""
	}
	if budget > 1e11 {
		return s, ""
	}
	if t.measureRunString(run, s) <= budget {
		return s, ""
	}
	// UAX#14 word-priority wrap (align Flutter/SkParagraph line breaking):
	// WrapText breaks at word boundaries with per-character fallback for
	// over-long words, using the run's face at the run's font size.
	if face := t.runFace(run); face != nil && budget > 0 {
		lines := text.WrapText(s, face, budget, text.WrapWordChar)
		if len(lines) > 0 && lines[0].End > 0 && lines[0].End < len(s) {
			origChunk := s[:lines[0].End]
			chunk = strings.TrimRight(origChunk, " \t")
			// Keep the split lossless: trailing spaces of the wrapped line are
			// excluded from the chunk width but stay at the front of rest
			// (SkParagraph trims line-trailing whitespace, preserves text).
			rest = origChunk[len(chunk):] + s[lines[0].End:]
			return chunk, rest
		}
	}
	runes := []rune(s)
	// Binary search max prefix that fits.
	lo, hi := 0, len(runes)
	best := 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if mid == 0 {
			lo = mid + 1
			continue
		}
		cand := string(runes[:mid])
		if t.measureRunString(run, cand) <= budget {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if best == 0 {
		return "", s
	}
	return string(runes[:best]), string(runes[best:])
}

func ellipsizeDisplayLine(t *RenderText, line displayLine, maxW float64) displayLine {
	ell := textEllipsis
	if maxW <= 0 {
		if len(line.Spans) == 0 {
			line.Spans = []displaySpan{{Text: ell, FontSize: t.fontSize(), R: t.R, G: t.G, B: t.B, A: t.A}}
			return line
		}
		last := &line.Spans[len(line.Spans)-1]
		last.Text += ell
		return line
	}
	// Rebuild by keeping spans while room for ellipsis remains.
	var out displayLine
	out.Height = line.Height
	x := 0.0
	// Reserve ellipsis width using parent metrics.
	ellRun := TextRun{Text: ell, FontSize: t.fontSize(), Face: t.Face, ApproxCharW: t.approxCharW()}
	ellW := t.measureRunString(ellRun, ell)
	for _, sp := range line.Spans {
		room := maxW - x - ellW
		if room <= 0 {
			break
		}
		w := sp.Width
		if w <= room+0.5 {
			sp.X = x
			out.Spans = append(out.Spans, sp)
			x += w
			continue
		}
		// Truncate this span.
		run := TextRun{Text: sp.Text, Face: sp.Face, FontSize: sp.FontSize, ApproxCharW: sp.ApproxCharW}
		chunk, _ := fitRunPrefix(t, run, sp.Text, room)
		if chunk != "" {
			nw := t.measureRunString(run, chunk)
			sp.Text = chunk
			sp.Width = nw
			sp.X = x
			out.Spans = append(out.Spans, sp)
			x += nw
		}
		break
	}
	out.Spans = append(out.Spans, displaySpan{
		Text: ell, Face: t.Face, FontSize: t.fontSize(),
		R: t.R, G: t.G, B: t.B, A: func() float64 {
			if t.A == 0 {
				return 1
			}
			return t.A
		}(),
		X: x, Width: ellW,
	})
	return out
}

func clipDisplayLine(t *RenderText, line displayLine, maxW float64) displayLine {
	var out displayLine
	out.Height = line.Height
	x := 0.0
	for _, sp := range line.Spans {
		room := maxW - x
		if room <= 0 {
			break
		}
		if sp.Width <= room+0.5 {
			sp.X = x
			out.Spans = append(out.Spans, sp)
			x += sp.Width
			continue
		}
		run := TextRun{Text: sp.Text, Face: sp.Face, FontSize: sp.FontSize, ApproxCharW: sp.ApproxCharW}
		chunk, _ := fitRunPrefix(t, run, sp.Text, room)
		if chunk != "" {
			nw := t.measureRunString(run, chunk)
			sp.Text = chunk
			sp.Width = nw
			sp.X = x
			out.Spans = append(out.Spans, sp)
		}
		break
	}
	return out
}

func (t *RenderText) measureRunsSize() (w, h float64) {
	lines := t.layoutRunLines()
	if len(lines) == 0 {
		return 0, t.fontSize() * 1.25
	}
	var longest float64
	for _, ln := range lines {
		var lw float64
		for _, sp := range ln.Spans {
			lw += sp.Width
		}
		if lw > longest {
			longest = lw
		}
		h += ln.Height
	}
	maxW := t.MaxWidth
	if maxW > 0 {
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
