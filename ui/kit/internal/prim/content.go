// Package prim content facades: pure text/image math plus builders over
// existing ui/rendering nodes (P3).
//
// Like layout and decor, this file creates no new RenderObjects. Label
// and RichLabel compose RenderText/ParagraphBuilder; TextStyleScope is a
// pure inherited-style merge; Picture composes RenderImage with an
// explicit placeholder→ready→error state machine; GlyphIcon resolves
// size and color from scope.Ctx. All numbers come from theme or props.
package prim

import (
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// ---- text style ----

// TextStyle is the inheritable text look: size, color, line multiplier.
// Zero values mean "inherit from parent" and resolve via Merge.
type TextStyle struct {
	FontSize float64
	Color    theme.Color
	HasColor bool
	LineMult float64
}

// Merge overlays child over parent: set child fields win, unset fields
// inherit. Mirrors DefaultTextStyle inheritance without globals.
func (c TextStyle) Merge(parent TextStyle) TextStyle {
	out := parent
	if c.FontSize > 0 {
		out.FontSize = c.FontSize
	}
	if c.HasColor {
		out.Color = c.Color
		out.HasColor = true
	}
	if c.LineMult > 0 {
		out.LineMult = c.LineMult
	}
	return out
}

// ResolveTextStyle merges seed defaults with an optional Ctx override
// chain. Empty chain returns the seed style (size 14, text color).
func ResolveTextStyle(seed theme.Tokens, chain ...TextStyle) TextStyle {
	base := TextStyle{FontSize: seed.FontSize, Color: seed.ColorText, HasColor: true, LineMult: seed.LineHeight}
	out := base
	for _, s := range chain {
		out = s.Merge(out)
	}
	if out.FontSize <= 0 {
		out.FontSize = seed.FontSize
	}
	if out.LineMult <= 0 {
		out.LineMult = seed.LineHeight
	}
	return out
}

// TextStyleScope carries one inherited style level for tests.
type TextStyleScope struct {
	Parent TextStyle
	Self   TextStyle
}

// Resolved reports the merged style.
func (s TextStyleScope) Resolved() TextStyle { return s.Self.Merge(s.Parent) }

// ---- label ----

// LabelProps configures one single-style text run.
type LabelProps struct {
	Text      string
	Style     TextStyle
	MaxWidth  float64
	MaxLines  int
	Ellipsis  bool
	LineMult  float64
	ApproxW   float64
	Face      text.Face
	Seed      theme.Tokens
	ScopeSpan TextStyle
}

// ResolveLabel merges style as props > scope span > seed.
func ResolveLabel(p LabelProps) TextStyle {
	seed := TextStyle{FontSize: p.Seed.FontSize, Color: p.Seed.ColorText, HasColor: true, LineMult: p.Seed.LineHeight}
	if p.Seed.FontSize <= 0 {
		seed = TextStyle{FontSize: 14, Color: p.Seed.ColorText, HasColor: p.Seed.ColorText.A != 0, LineMult: 1.5714285714285714}
	}
	out := p.ScopeSpan.Merge(seed)
	out = p.Style.Merge(out)
	if p.LineMult > 0 {
		out.LineMult = p.LineMult
	}
	return out
}

// NewLabel builds a single-style RenderText. Face may be nil (estimate
// path); callers attach a real face when available.
func NewLabel(p LabelProps) *rendering.RenderText {
	st := ResolveLabel(p)
	t := rendering.NewRenderText(p.Text)
	if st.FontSize > 0 {
		t.SetFontSize(st.FontSize)
	}
	if st.HasColor {
		t.SetColor(st.Color.R, st.Color.G, st.Color.B, st.Color.A)
	}
	if p.Face != nil {
		t.SetFace(p.Face)
	}
	if p.MaxWidth > 0 {
		t.SetMaxWidth(p.MaxWidth)
	}
	if p.MaxLines > 0 {
		t.SetMaxLines(p.MaxLines)
	}
	if p.Ellipsis {
		t.SetOverflow(rendering.TextOverflowEllipsis)
	}
	if st.LineMult > 0 {
		t.LineSpacing = st.LineMult
	}
	if p.ApproxW > 0 {
		t.ApproxCharW = p.ApproxW
	}
	return t
}

// ---- rich label ----

// Span is one styled run inside a RichLabel.
type Span struct {
	Text  string
	Style TextStyle
	Face  text.Face
}

// RichProps configures multi-span text over one base style.
type RichProps struct {
	Spans    []Span
	Base     TextStyle
	MaxWidth float64
	MaxLines int
	Ellipsis bool
	Seed     theme.Tokens
}

// ResolveRichSpans merges each span over the base style.
func ResolveRichSpans(p RichProps) []TextStyle {
	base := p.Base.Merge(TextStyle{FontSize: p.Seed.FontSize, Color: p.Seed.ColorText, HasColor: true, LineMult: p.Seed.LineHeight})
	if base.FontSize <= 0 {
		base.FontSize = 14
	}
	out := make([]TextStyle, len(p.Spans))
	for i, s := range p.Spans {
		out[i] = s.Style.Merge(base)
	}
	return out
}

// NewRichLabel builds a multi-run RenderText via ParagraphBuilder.
func NewRichLabel(p RichProps) *rendering.RenderText {
	styles := ResolveRichSpans(p)
	b := rendering.NewParagraphBuilder()
	for i, s := range p.Spans {
		st := styles[i]
		b.PushStyle(s.Face, st.FontSize, st.HasColor, st.Color.R, st.Color.G, st.Color.B, st.Color.A, 0)
		b.AddText(s.Text)
		b.PopStyle()
	}
	t := b.Build()
	if p.MaxWidth > 0 {
		t.SetMaxWidth(p.MaxWidth)
	}
	if p.MaxLines > 0 {
		t.SetMaxLines(p.MaxLines)
	}
	if p.Ellipsis {
		t.SetOverflow(rendering.TextOverflowEllipsis)
	}
	return t
}

// ---- measure math (unit-testable, no face needed) ----

// EstimateSize measures text with the rune heuristic used when no face
// is present: width = runes × size × approxW, height = lines × size ×
// lineMult. Newlines split lines; MaxWidth wraps by rune budget.
func EstimateSize(s string, fontSize, approxW, lineMult, maxWidth float64, maxLines int, ellipsis bool) rendering.Size {
	if fontSize <= 0 {
		fontSize = 14
	}
	if approxW <= 0 {
		approxW = 0.55
	}
	if lineMult <= 0 {
		lineMult = 1.5
	}
	lines := splitWrapLines(s, fontSize, approxW, maxWidth)
	truncated := false
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	_ = truncated
	_ = ellipsis
	maxW := 0.0
	for _, ln := range lines {
		w := float64(utf8.RuneCountInString(ln)) * fontSize * approxW
		if maxWidth > 0 && w > maxWidth {
			w = maxWidth
		}
		if w > maxW {
			maxW = w
		}
	}
	if maxWidth > 0 && (len(lines) > 1 || maxLines > 0) {
		maxW = maxWidth
	}
	if len(lines) == 0 {
		return rendering.Size{Width: 0, Height: fontSize * lineMult}
	}
	return rendering.Size{Width: maxW, Height: float64(len(lines)) * fontSize * lineMult}
}

func splitWrapLines(s string, fontSize, approxW, maxWidth float64) []string {
	paras := splitLines(s)
	if maxWidth <= 0 {
		return paras
	}
	charW := fontSize * approxW
	if charW <= 0 {
		return paras
	}
	budget := int(maxWidth / charW)
	if budget < 1 {
		budget = 1
	}
	var out []string
	for _, p := range paras {
		r := []rune(p)
		if len(r) == 0 {
			out = append(out, "")
			continue
		}
		for len(r) > budget {
			out = append(out, string(r[:budget]))
			r = r[budget:]
		}
		out = append(out, string(r))
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// Ellipsized reports whether content exceeds maxLines (ellipsis path).
func Ellipsized(lineCount, maxLines int) bool {
	return maxLines > 0 && lineCount > maxLines
}

// ---- picture (placeholder → ready → error) ----

// PictureState names the async image phase.
type PictureState int

const (
	// PicturePlaceholder shows the fill color, no pixels yet.
	PicturePlaceholder PictureState = iota
	// PictureReady shows decoded pixels.
	PictureReady
	// PictureError shows the error chrome.
	PictureError
)

// PictureSpec resolves the placeholder fill from theme.
type PictureSpec struct {
	Width, Height float64
	Fill          theme.Color
}

// ResolvePicture fills size and placeholder color from seed.
func ResolvePicture(w, h float64, seed theme.Tokens) PictureSpec {
	return PictureSpec{Width: w, Height: h, Fill: seed.ColorFillSecondary}
}

// NewPicture builds a RenderImage placeholder; the caller drives the
// state machine (SetLoading/SetImageShared/SetError wrap the node so
// tests assert transitions without IO).
func NewPicture(spec PictureSpec) *rendering.RenderImage {
	im := rendering.NewRenderImage(spec.Width, spec.Height)
	im.PR, im.PG, im.PB = spec.Fill.R, spec.Fill.G, spec.Fill.B
	return im
}

// PicturePhase maps a RenderImage state to the facade phase.
func PicturePhase(im *rendering.RenderImage) PictureState {
	if im == nil {
		return PicturePlaceholder
	}
	switch im.State {
	case rendering.ImageReady:
		return PictureReady
	case rendering.ImageError:
		return PictureError
	default:
		return PicturePlaceholder
	}
}

// ---- glyph icon ----

// IconSpec resolves icon size and color from Ctx.
type IconSpec struct {
	Size  float64
	Color theme.Color
}

// ResolveIcon reads size from the Ctx tier and color from the seed text
// hierarchy: small 12, medium 16, large 20; disabled dims to the
// disabled token.
func ResolveIcon(ctx scope.Ctx, seed theme.Tokens) IconSpec {
	size := 16.0
	switch ctx.Size {
	case scope.SizeSmall:
		size = 12
	case scope.SizeLarge:
		size = 20
	}
	color := seed.ColorTextSecondary
	if ctx.Disabled {
		color = seed.ColorTextDisabled
	}
	return IconSpec{Size: size, Color: color}
}

// IconBoxSize reports the square box for an icon spec.
func IconBoxSize(spec IconSpec) rendering.Size {
	return rendering.Size{Width: spec.Size, Height: spec.Size}
}
