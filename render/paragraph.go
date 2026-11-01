//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

// P3.3 rich text: per-run styles (color / decoration / weight), built with a
// Flutter-style ParagraphBuilder and rendered by Context.DrawParagraph one
// run at a time (SkParagraph's per-run submission model).
//
// Style spans and shaper clusters are both rune offsets into the paragraph
// text, so run→style resolution is an integer range lookup.

package render

import (
	"fmt"
	"os"
	"strings"

	"github.com/energye/gpui/render/text"
)

// TextStyle is the per-run style subset of P3.3: color, decoration and
// weight. A zero field means "inherit from the enclosing scope".
type TextStyle struct {
	// Color is the run fill color. Alpha 0 means unspecified (inherit).
	Color RGBA
	// Decoration is the underline/strikethrough/overline bit set.
	Decoration TextDecoration // keep import use
	// Weight selects a font weight for the run (0 = inherit the base face).
	// Weight resolution needs a weight-capable face stack (MultiFace): a
	// plain single Face degrades to the base face.
	Weight text.FontWeight
}

// styledSpan is one user-styled chunk of paragraph text.
type styledSpan struct {
	text     string
	style    TextStyle
	runeRuns int // cumulative rune count up to and including this span
}

// ParagraphBuilder accumulates styled text chunks (Flutter
// ParagraphBuilder-style PushStyle/AddText/Build).
type ParagraphBuilder struct {
	face   text.Face
	family *text.FontFamily // optional weight-resolution source (P3.3)
	base   TextStyle
	spans  []styledSpan
	style  []TextStyle
}

// SetFamily attaches a font family used to derive per-run weight faces
// (FontFamily.Face picks the closest real cut, falling back to synthetic
// embolden). Without it, weight runs keep the base face.
func (b *ParagraphBuilder) SetFamily(f *text.FontFamily) { b.family = f }

// NewParagraphBuilder creates a builder over a base face and base style.
func NewParagraphBuilder(face text.Face, base TextStyle) *ParagraphBuilder {
	return &ParagraphBuilder{face: face, base: base, style: []TextStyle{base}}
}

// PushStyle enters a nested style scope; zero fields inherit the enclosing
// scope (PopStyle restores it).
func (b *ParagraphBuilder) PushStyle(s TextStyle) {
	cur := b.style[len(b.style)-1]
	if s.Color.A != 0 {
		cur.Color = s.Color
	}
	if s.Decoration != 0 {
		cur.Decoration = s.Decoration
	}
	if s.Weight != 0 {
		cur.Weight = s.Weight
	}
	b.style = append(b.style, cur)
}

// PopStyle exits the innermost style scope.
func (b *ParagraphBuilder) PopStyle() {
	if len(b.style) > 1 {
		b.style = b.style[:len(b.style)-1]
	}
}

// AddText appends text with the current style.
func (b *ParagraphBuilder) AddText(text string) {
	if text == "" {
		return
	}
	prev := 0
	if len(b.spans) > 0 {
		prev = b.spans[len(b.spans)-1].runeRuns
	}
	b.spans = append(b.spans, styledSpan{
		text:     text,
		style:    b.style[len(b.style)-1],
		runeRuns: prev + len([]rune(text)),
	})
}

// Build lays out the accumulated spans into a Paragraph.
func (b *ParagraphBuilder) Build(opts text.LayoutOptions) *Paragraph {
	var full strings.Builder
	for _, sp := range b.spans {
		full.WriteString(sp.text)
	}
	paraText := full.String()

	face := b.face
	if len(b.spans) > 0 {
		face = &styledFace{Face: b.face, family: b.family, spans: b.spans}
	}

	layout := text.LayoutText(paraText, face, opts)
	// Annotate every run with its span index (first-glyph cluster → span).
	for li := range layout.Lines {
		runs := layout.Lines[li].Runs
		for ri := range runs {
			run := &runs[ri]
			if len(run.Glyphs) == 0 {
				run.Style = -1
				continue
			}
			c := run.Glyphs[0].Cluster
			idx := len(b.spans) - 1
			for i, end := range b.spans {
				if c < end.runeRuns {
					idx = i
					break
				}
			}
			run.Style = idx
		}
	}
	styles := make([]TextStyle, len(b.spans))
	boundaries := make([]int, len(b.spans))
	for i, sp := range b.spans {
		styles[i] = sp.style
		boundaries[i] = sp.runeRuns
	}
	return &Paragraph{
		Layout:     layout,
		text:       paraText,
		styles:     styles,
		spanEnds:   boundaries,
		baseWeight: b.base.Weight,
	}
}

// Paragraph is a laid-out rich-text block: the standard Layout plus the
// style table that ShapedRun.Style indexes into.
type Paragraph struct {
	Layout *text.Layout

	text     string
	styles   []TextStyle
	spanEnds []int // cumulative rune counts; span i covers [end(i-1), end(i))
	// baseWeight is the builder's base style weight (diagnostics).
	baseWeight text.FontWeight
}

// StyleForRun resolves the style of a run whose first glyph carries cluster
// c (a rune offset within the paragraph text).
func (p *Paragraph) StyleForRun(c int) (TextStyle, bool) {
	if p == nil || len(p.styles) == 0 {
		return TextStyle{}, false
	}
	for i, end := range p.spanEnds {
		if c < end {
			return p.styles[i], true
		}
	}
	return p.styles[len(p.styles)-1], true
}

// StyleCount reports the number of distinct spans (diagnostics/tests).
func (p *Paragraph) StyleCount() int {
	if p == nil {
		return 0
	}
	return len(p.styles)
}

// styledFace adapts a MultiFace to a styled paragraph: it splits text at
// span boundaries and resolves each span's face through WithWeight, so the
// layout's existing face-fallback segmentation sees one face per styled
// span. It deliberately re-implements Runs (span-split first, then
// MultiFace.Runs per slice) instead of embedding, because MultiFace.Runs
// caches by (face, text) and two paragraphs sharing a base face must not
// collide.
type styledFace struct {
	text.Face // forwards Advance/Size/Metrics/... to the base face
	family    *text.FontFamily
	spans     []styledSpan
}

// Runs splits text at styled-span boundaries first, then defers each slice
// to the wrapped MultiFace (its script/bidi segmentation still applies
// within a slice). Weight-0 spans keep the base resolution.
func (f *styledFace) Runs(para string) []text.FaceRun {
	if len(f.spans) == 0 {
		return f.baseRuns(para)
	}
	var out []text.FaceRun
	pos := 0  // rune offset of the current slice start
	span := 0 // current span index
	r := []rune(para)
	for pos < len(r) {
		end := len(r)
		weight := text.FontWeight(0)
		if span < len(f.spans) {
			end = f.spans[span].runeRuns
			weight = f.spans[span].style.Weight
		}
		if end > len(r) {
			end = len(r)
		}
		if pos >= end {
			span++
			continue
		}
		slice := string(r[pos:end])
		parts, face := f.baseRuns(slice), f.Face
		if weight != 0 && f.family != nil {
			if w := f.family.Face(20.0, text.WithWeight(weight)); w != nil {
				face = w
			}
		}
		for _, pr := range parts {
			out = append(out, text.FaceRun{Face: face, Text: pr.Text, X: pr.X})
		}
		pos = end
		span++
	}
	return out
}

// DrawParagraphStyled renders a rich-text Paragraph: every run is submitted
// with its own color (SkParagraph's per-run submission), decorations draw
// per run, and the ambient paint is restored afterwards — the call never
// leaks style into subsequent draws.
//
// Blend space and blend mode still come from the ambient paint at call time
// (per-run blending follows the P3.1 semantics of the ambient paint).
func (c *Context) DrawParagraphStyled(p *Paragraph, x, y float64) {
	if c == nil || p == nil || p.Layout == nil {
		return
	}
	// Snapshot the ambient paint (color + decoration) for restore.
	prevColor := c.paint.solidColor
	prevBrush := c.paint.Brush
	prevIsSolid := c.paint.isSolid
	prevDecoration := c.TextDecoration()
	restore := func() {
		c.paint.solidColor = prevColor
		c.paint.Brush = prevBrush
		c.paint.isSolid = prevIsSolid
		c.SetTextDecoration(prevDecoration)
	}

	for li := range p.Layout.Lines {
		line := &p.Layout.Lines[li]
		for ri := range line.Runs {
			run := &line.Runs[ri]
			if len(run.Glyphs) == 0 {
				continue
			}
			if os.Getenv("GPUI_DEBUG_P33") == "1" {
				st, _ := p.styleAt(run.Style)
				fmt.Fprintf(os.Stderr, "[p33] run li=%d ri=%d style=%d cluster=%d glyphs=%d color=(%.2f,%.2f,%.2f,%.2f)\n",
					li, ri, run.Style, run.Glyphs[0].Cluster, len(run.Glyphs), st.Color.R, st.Color.G, st.Color.B, st.Color.A)
			}
			style, ok := p.styleAt(run.Style)
			if !ok {
				c.DrawShapedGlyphs(run.Glyphs, run.Face, x, y+line.Y)
				continue
			}
			// Run color: paint color drives DrawShapedGlyphs' per-call
			// capture (text_dispatch.go FromColor).
			if style.Color.A != 0 {
				c.SetRGBA(style.Color.R, style.Color.G, style.Color.B, style.Color.A)
			} else {
				c.SetRGBA(prevColor.R, prevColor.G, prevColor.B, prevColor.A)
			}
			c.SetTextDecoration(style.Decoration)
			runWidth := 0.0
			for gi := range run.Glyphs {
				runWidth += run.Glyphs[gi].XAdvance
			}
			runX := x + run.Glyphs[0].X
			runBaseY := y + line.Y
			// Decorations first (under the glyphs, matching DrawString's
			// order where the line is stroked beneath the ink).
			c.applyRunDecorations(style.Decoration, runX, runBaseY, runWidth, run.Face)
			c.DrawShapedGlyphs(run.Glyphs, run.Face, x, runBaseY)
		}
	}
	restore()
}

// styleAt resolves run.Style to a style; ok=false for the unstyled (-1) or
// out-of-range case (plain LayoutText paragraphs).
func (p *Paragraph) styleAt(idx int) (TextStyle, bool) {
	if p == nil || idx < 0 || idx >= len(p.styles) {
		return TextStyle{}, false
	}
	return p.styles[idx], true
}

// applyRunDecorations draws the decoration lines for one styled run at its
// baseline — the per-run twin of applyTextDecorations (same offsets and
// thickness from face.Metrics; the line uses the current fill color, which
// the caller set to the run color).
func (c *Context) applyRunDecorations(dec TextDecoration, runX, baselineY, runWidth float64, face text.Face) {
	if dec == TextDecorationNone || runWidth <= 0 || face == nil {
		return
	}
	m := face.Metrics()
	size := face.Size()
	thickness := size / 16
	if thickness < 1 {
		thickness = 1
	}
	if dec&TextDecorationUnderline != 0 {
		uy := baselineY + m.Descent*0.5
		if uy < baselineY+1 {
			uy = baselineY + thickness
		}
		c.drawDecorationLine(runX, uy, runWidth, thickness)
	}
	if dec&TextDecorationLineThrough != 0 {
		c.drawDecorationLine(runX, baselineY-m.Ascent*0.35, runWidth, thickness)
	}
	if dec&TextDecorationOverline != 0 {
		c.drawDecorationLine(runX, baselineY-m.Ascent, runWidth, thickness)
	}
}

// baseRuns defers a slice to the base face when it can split itself
// (MultiFace), or yields it whole when it cannot.
func (f *styledFace) baseRuns(slice string) []text.FaceRun {
	if fr, ok := f.Face.(interface{ Runs(string) []text.FaceRun }); ok {
		return fr.Runs(slice)
	}
	return []text.FaceRun{{Face: f.Face, Text: slice}}
}
