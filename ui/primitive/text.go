package primitive

import (
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
)

// ellipsisChar is the single-character ellipsis used for C-ClipContent.
const ellipsisChar = "…"

// Text paints a string with optional width constraint and ellipsis (C-Measure / C-ClipContent).
//
// FontSize is authoritative when > 0: if Face was created at a different size,
// the face is re-derived via FontSource.
//
// Overflow (F6):
//
//	MaxWidth / tight parent Max → layout width cap
//	Ellipsis=true → truncate with "…" to fit (single-line or last line when MaxLines>1)
//	MaxLines>1 → soft-wrap; height = lineH * used lines
type Text struct {
	core.NodeBase

	Value    string
	Color    render.RGBA
	FontSize float64 // points; 0 → 14
	// Face is optional; when set, used for measure and draw (re-sized to FontSize).
	Face text.Face
	// MaxWidth when > 0 constrains preferred width (also respects parent MaxWidth).
	MaxWidth float64
	// MaxLines is the maximum number of lines after wrap. 0 or 1 = single line.
	MaxLines int
	// Ellipsis truncates overflow with "…" when true (F6). Default false for
	// back-compat; kit Typography/Tag should opt in.
	Ellipsis bool
	// Decoration is underline / line-through / overline (render.TextDecoration bitset).
	Decoration render.TextDecoration

	// paintLines is set in Layout for Paint.
	paintLines []string
}

// NewText constructs a Text node.
func NewText(value string) *Text {
	t := &Text{
		Value:    value,
		Color:    render.RGBA{R: 0, G: 0, B: 0, A: 0.88},
		FontSize: 14,
	}
	t.Init(t)
	t.Hit = core.HitDefer
	return t
}

// TypeID implements core.Node.
func (t *Text) TypeID() string { return TypeText }

// SetValue updates text and dirties layout.
func (t *Text) SetValue(v string) {
	if t.Value == v {
		return
	}
	t.Value = v
	t.MarkNeedsLayout()
}

// SetFontSize updates point size and dirties layout (re-derives Face when needed).
func (t *Text) SetFontSize(px float64) {
	if t.FontSize == px {
		return
	}
	t.FontSize = px
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetEllipsis enables/disables overflow ellipsis and dirties layout.
func (t *Text) SetEllipsis(on bool) {
	if t.Ellipsis == on {
		return
	}
	t.Ellipsis = on
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetMaxLines sets wrap line budget (≤1 = single line). Dirties layout.
func (t *Text) SetMaxLines(n int) {
	if t.MaxLines == n {
		return
	}
	t.MaxLines = n
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetDecoration sets underline / line-through / overline and dirties paint.
func (t *Text) SetDecoration(d render.TextDecoration) {
	if t.Decoration == d {
		return
	}
	t.Decoration = d
	t.MarkNeedsPaint()
}

func (t *Text) effectiveFace() text.Face {
	return faceAtSize(t.Face, t.fontSize())
}

func (t *Text) fontSize() float64 {
	if t != nil && t.FontSize > 0 {
		return t.FontSize
	}
	return 14
}

func (t *Text) lineHeight() float64 {
	face := t.effectiveFace()
	size := t.fontSize()
	if face != nil {
		m := face.Metrics()
		h := m.Ascent + m.Descent
		if h > 0 {
			return h
		}
	}
	return size * 1.2
}

func (t *Text) advance(s string) float64 {
	return measureTextWidth(t.Face, s, t.fontSize())
}

// availWidth resolves the layout width budget from MaxWidth and constraints.
// Returns 0 when unbounded (no width cap).
func (t *Text) availWidth(c core.Constraints) float64 {
	var w float64
	if c.HasBoundedWidth() {
		w = c.MaxWidth
	}
	if t.MaxWidth > 0 {
		if w <= 0 || t.MaxWidth < w {
			w = t.MaxWidth
		}
	}
	return w
}

func (t *Text) maxLines() int {
	if t.MaxLines <= 0 {
		return 1
	}
	return t.MaxLines
}

// Layout implements core.Node.
func (t *Text) Layout(c core.Constraints) core.Size {
	lineH := t.lineHeight()
	ml := t.maxLines()
	avail := t.availWidth(c)

	// Unbounded: intrinsic size (hard newlines only; no soft wrap).
	if avail <= 0 {
		lines := strings.Split(t.Value, "\n")
		if ml == 1 {
			// Single-line: first segment only for intrinsic width.
			if len(lines) > 0 {
				lines = lines[:1]
			}
		} else if len(lines) > ml {
			if t.Ellipsis {
				lines = append(append([]string{}, lines[:ml-1]...), truncateWithEllipsis(lines[ml-1], 1e9, t.advance))
				// without width, just join remainder indicator
				lines = lines[:ml]
				if !strings.HasSuffix(lines[ml-1], ellipsisChar) {
					lines[ml-1] = lines[ml-1] + ellipsisChar
				}
			} else {
				lines = lines[:ml]
			}
		}
		var maxW float64
		for _, ln := range lines {
			if w := t.advance(ln); w > maxW {
				maxW = w
			}
		}
		t.paintLines = lines
		h := lineH * float64(len(lines))
		if h < lineH {
			h = lineH
		}
		out := c.Tighten(core.Size{Width: maxW, Height: h})
		t.SetSize(out)
		return out
	}

	// Constrained width.
	if ml <= 1 {
		full := strings.ReplaceAll(t.Value, "\n", " ")
		fw := t.advance(full)
		if fw <= avail {
			t.paintLines = []string{full}
			out := c.Tighten(core.Size{Width: fw, Height: lineH})
			t.SetSize(out)
			return out
		}
		if t.Ellipsis {
			t.paintLines = []string{truncateWithEllipsis(full, avail, t.advance)}
		} else {
			// Clip paint via PushClipLocal; keep full string for paint but size to avail.
			t.paintLines = []string{full}
		}
		out := c.Tighten(core.Size{Width: avail, Height: lineH})
		t.SetSize(out)
		return out
	}

	lines := wrapTextLines(t.Value, avail, ml, t.Ellipsis, t.advance)
	t.paintLines = lines
	var maxW float64
	for _, ln := range lines {
		if w := t.advance(ln); w > maxW {
			maxW = w
		}
	}
	if maxW > avail {
		maxW = avail
	}
	if maxW < 0 {
		maxW = 0
	}
	out := c.Tighten(core.Size{Width: maxW, Height: lineH * float64(max(1, len(lines)))})
	t.SetSize(out)
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// truncateWithEllipsis returns the longest prefix of s such that prefix+"…" fits in maxW.
func truncateWithEllipsis(s string, maxW float64, advance func(string) float64) string {
	if maxW <= 0 {
		return ellipsisChar
	}
	if advance(s) <= maxW {
		return s
	}
	ellW := advance(ellipsisChar)
	if ellW >= maxW {
		return ellipsisChar
	}
	budget := maxW - ellW
	r := []rune(s)
	lo, hi := 0, len(r)
	best := 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if mid <= 0 {
			lo = mid + 1
			continue
		}
		if advance(string(r[:mid])) <= budget {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if best <= 0 {
		return ellipsisChar
	}
	return string(r[:best]) + ellipsisChar
}

// wrapTextLines soft-wraps s into at most maxLines lines of width maxW.
func wrapTextLines(s string, maxW float64, maxLines int, ellipsis bool, advance func(string) float64) []string {
	if maxLines < 1 {
		maxLines = 1
	}
	var out []string
	// Flatten hard newlines as paragraph breaks that force a line boundary.
	paras := strings.Split(s, "\n")
	for pi, para := range paras {
		if len(out) >= maxLines {
			break
		}
		remainLines := maxLines - len(out)
		isLastPara := pi == len(paras)-1
		chunk := wrapParagraph(para, maxW, remainLines, ellipsis || !isLastPara, advance)
		// If not last para and we filled budget, ensure ellipsis on last line.
		if !isLastPara && len(out)+len(chunk) >= maxLines && ellipsis && len(chunk) > 0 {
			last := chunk[len(chunk)-1]
			chunk[len(chunk)-1] = truncateWithEllipsis(strings.TrimSuffix(last, ellipsisChar), maxW, advance)
		}
		out = append(out, chunk...)
		if len(out) >= maxLines {
			out = out[:maxLines]
			break
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// wrapParagraph wraps one paragraph (no hard newlines).
func wrapParagraph(para string, maxW float64, maxLines int, ellipsis bool, advance func(string) float64) []string {
	if maxLines < 1 {
		return nil
	}
	if para == "" {
		return []string{""}
	}
	if advance(para) <= maxW {
		return []string{para}
	}
	var lines []string
	r := []rune(para)
	start := 0
	for start < len(r) && len(lines) < maxLines {
		end := start
		best := start + 1
		lastBreak := -1
		for end < len(r) {
			end++
			if advance(string(r[start:end])) <= maxW {
				best = end
				ch := r[end-1]
				if ch == ' ' || ch == '\t' {
					lastBreak = end
				}
			} else {
				break
			}
		}
		useEnd := best
		// Prefer word break when more lines remain after this one.
		if lastBreak > start+1 && len(lines)+1 < maxLines {
			useEnd = lastBreak
		}
		if useEnd <= start {
			useEnd = start + 1
		}
		// Last allowed line but content remains → ellipsis.
		if len(lines) == maxLines-1 && useEnd < len(r) && ellipsis {
			lines = append(lines, truncateWithEllipsis(string(r[start:]), maxW, advance))
			return lines
		}
		line := strings.TrimRight(string(r[start:useEnd]), " \t")
		lines = append(lines, line)
		start = useEnd
		for start < len(r) && (r[start] == ' ' || r[start] == '\t') {
			start++
		}
	}
	return lines
}

// Paint implements core.Node.
func (t *Text) Paint(pc *core.PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	lines := t.paintLines
	if lines == nil {
		if t.Value == "" {
			return
		}
		lines = []string{t.Value}
	}
	dc := pc.DC
	face := t.effectiveFace()
	if face != nil {
		dc.SetFont(face)
	}
	dc.SetRGBA(t.Color.R, t.Color.G, t.Color.B, t.Color.A)
	prevDec := dc.TextDecoration()
	if t.Decoration != render.TextDecorationNone {
		dc.SetTextDecoration(t.Decoration)
	}
	defer dc.SetTextDecoration(prevDec)
	ascent := t.fontSize() * 0.8
	if face != nil {
		ascent = face.Metrics().Ascent
	}
	lineH := t.lineHeight()
	sz := t.Size()
	pc.PushClipLocal(0, 0, sz.Width, sz.Height)
	defer pc.Pop()
	for i, ln := range lines {
		if ln == "" {
			continue
		}
		y := pc.Origin.Y + float64(i)*lineH + ascent
		dc.DrawString(ln, pc.Origin.X, y)
	}
}

// HitTest implements core.Node.
func (t *Text) HitTest(p core.Point) core.Node { return t.DefaultHitTest(p) }

// DisplayedLines returns the last laid-out lines (for tests).
func (t *Text) DisplayedLines() []string {
	if t == nil {
		return nil
	}
	return append([]string(nil), t.paintLines...)
}

// DisplayedText joins laid-out lines with newlines (for tests).
func (t *Text) DisplayedText() string {
	if t == nil {
		return ""
	}
	return strings.Join(t.paintLines, "\n")
}
