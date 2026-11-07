//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"strings"
	"sync/atomic"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// SVG text: absolutely positioned shaped text. Font resolution goes through
// the SetFontResolver hook — svg never scans system fonts (that belongs to
// the ui/platform layer); unmatched families are a named refusal (fail-closed).
// Rendering flattens glyph outlines into a Path and runs the shape paint
// pipeline (fill/stroke/gradient/clip/mask apply to text like to shapes).

// FontResolverFunc resolves an SVG font-family (+ weight/style) to a font
// source; nil result = no match (the text is not rendered, with a warning).
type FontResolverFunc func(family string, weight text.FontWeight, italic bool) *text.FontSource

var fontResolver atomic.Pointer[FontResolverFunc]

// SetFontResolver installs the font resolution hook (nil disables text
// rendering entirely — every <text> becomes a named refusal).
func SetFontResolver(fn FontResolverFunc) {
	if fn == nil {
		fontResolver.Store(nil)
		return
	}
	fontResolver.Store(&fn)
}

func resolveFont(family string, weight text.FontWeight, italic bool) (*text.FontSource, bool) {
	h := fontResolver.Load()
	if h == nil || *h == nil {
		return nil, false
	}
	src := (*h)(family, weight, italic)
	if src == nil {
		return nil, false
	}
	return src, true
}

// textSpan is one flattened run of <text>/<tspan> content.
type textSpan struct {
	attrs  Attrs
	text   string
	x, y   []Length
	dx, dy []Length
	rotate []float64
}

// flattenTextSpans walks a <text> tree into a flat run list (document order).
// Tspan attributes inherit the unset fields from their <text> ancestor.
func flattenTextSpans(root TextElement) []textSpan {
	var out []textSpan
	if strings.TrimSpace(root.Text) != "" {
		out = append(out, textSpan{attrs: root.Attrs, text: root.Text,
			x: root.X, y: root.Y, dx: root.DX, dy: root.DY, rotate: root.Rotate})
	}
	var walk func(children []Element)
	walk = func(children []Element) {
		for _, c := range children {
			ts, ok := c.(*TspanElement)
			if !ok {
				continue
			}
			if strings.TrimSpace(ts.Text) != "" {
				out = append(out, textSpan{attrs: inheritTextAttrs(root.Attrs, ts.Attrs), text: ts.Text,
					x: ts.X, y: ts.Y, dx: ts.DX, dy: ts.DY, rotate: ts.Rotate})
			}
			walk(ts.Children)
		}
	}
	walk(root.Children)
	return out
}

// inheritTextAttrs fills the child's unset text/paint fields from the
// parent (tspan inherits from <text> per SVG 1.1).
func inheritTextAttrs(parent, child Attrs) Attrs {
	out := child
	if out.FontFamily == "" {
		out.FontFamily = parent.FontFamily
	}
	if out.FontWeight == "" {
		out.FontWeight = parent.FontWeight
	}
	if out.FontStyle == "" {
		out.FontStyle = parent.FontStyle
	}
	if out.TextAnchor == "" {
		out.TextAnchor = parent.TextAnchor
	}
	if out.LetterSpacing == "" {
		out.LetterSpacing = parent.LetterSpacing
	}
	if out.Fill == "" {
		out.Fill = parent.Fill
	}
	if out.FillOpacity == "" {
		out.FillOpacity = parent.FillOpacity
	}
	if out.FillRule == "" {
		out.FillRule = parent.FillRule
	}
	if out.Stroke == "" {
		out.Stroke = parent.Stroke
	}
	if out.StrokeWidth == "" {
		out.StrokeWidth = parent.StrokeWidth
	}
	if out.StrokeOpacity == "" {
		out.StrokeOpacity = parent.StrokeOpacity
	}
	if out.Color == "" {
		out.Color = parent.Color
	}
	return out
}

// renderTextElement shapes and renders <text> via glyph outlines.
func renderTextElement(dc *render.Context, e *TextElement, a *Attrs, p *props, st *renderState) {
	if strings.TrimSpace(e.Text) == "" && len(e.Children) == 0 {
		return
	}
	spans := flattenTextSpans(*e)
	if len(spans) == 0 {
		return
	}
	extractor := text.NewOutlineExtractor()

	// Pass 1: resolve fonts + shape every span; collect glyph placements.
	type placedGlyph struct {
		path   *render.Path // outline at its final position (rotate baked)
		advSum float64
	}
	var builder render.Path
	_ = builder
	spanPaints := make([][]*render.Path, len(spans))
	penX, penY := 0.0, 0.0
	if len(e.X) > 0 {
		penX = e.X[0].Resolve(st.rs, LengthAxisX)
	}
	if len(e.Y) > 0 {
		penY = e.Y[0].Resolve(st.rs, LengthAxisY)
	}
	if penY == 0 && len(e.Y) == 0 {
		penY = 0 // SVG requires y; default baseline 0 (rare, spec allows)
	}
	totalAdv := 0.0
	type shapedSpan struct {
		glyphs []text.ShapedGlyph
		adv    float64
	}
	shaped := make([]shapedSpan, 0, len(spans))
	for _, sp := range spans {
		family := familyList(sp.attrs)[0]
		weight := mapWeight(sp.attrs)
		italic := strings.Contains(strings.ToLower(sp.attrs.FontStyle), "italic")
		src, ok := resolveFont(family, weight, italic)
		if !ok {
			return // fail-closed: no match (parse-time validation already warned)
		}
		fontSize := p.fontSize
		if fs := firstLength(sp.attrs.FontSize); fs != nil {
			fontSize = fs.Resolve(st.rs, LengthAxisY)
		}
		face := src.Face(fontSize)
		glyphs := text.Shape(sp.text, face)
		adv := 0.0
		spacing := letterSpacingOf(sp.attrs, st.rs)
		for i := range glyphs {
			adv += glyphs[i].XAdvance + spacing
		}
		shaped = append(shaped, shapedSpan{glyphs: glyphs, adv: adv})
		totalAdv += adv
	}

	// Pass 2: text-anchor shifts the whole run.
	anchorShift := 0.0
	switch textAnchorOf(spans[0].attrs) {
	case "middle":
		anchorShift = -totalAdv / 2
	case "end":
		anchorShift = -totalAdv
	}
	penX += anchorShift

	// Pass 3: place glyphs. x/y/dx/dy/rotate lists index PER SPAN (each
	// element's lists restart at its own first character, SVG 1.1); the
	// text-anchor shift applies to every absolute x as well.
	parsed := map[*text.FontSource]text.ParsedFont{}
	for si, sp := range spans {
		family := familyList(sp.attrs)[0]
		weight := mapWeight(sp.attrs)
		italic := strings.Contains(strings.ToLower(sp.attrs.FontStyle), "italic")
		src, _ := resolveFont(family, weight, italic)
		fontSize := p.fontSize
		if fs := firstLength(sp.attrs.FontSize); fs != nil {
			fontSize = fs.Resolve(st.rs, LengthAxisY)
		}
		parsedFont, cached := parsed[src]
		if !cached {
			parsedFont = src.Parsed()
			parsed[src] = parsedFont
		}
		spacing := letterSpacingOf(sp.attrs, st.rs)
		for gi, gl := range shaped[si].glyphs {
			if gi < len(sp.x) {
				penX = sp.x[gi].Resolve(st.rs, LengthAxisX) + anchorShift
			}
			if gi < len(sp.y) {
				penY = sp.y[gi].Resolve(st.rs, LengthAxisY)
			}
			if gi < len(sp.dx) {
				penX += sp.dx[gi].Resolve(st.rs, LengthAxisX)
			}
			if gi < len(sp.dy) {
				penY += sp.dy[gi].Resolve(st.rs, LengthAxisY)
			}
			rot := 0.0
			if gi < len(sp.rotate) {
				rot = sp.rotate[gi]
			}
			outline, err := extractor.ExtractOutline(parsedFont, text.GlyphID(gl.GID), fontSize)
			if err != nil || outline == nil || outline.IsEmpty() {
				penX += gl.XAdvance + spacing
				continue
			}
			gp := outlineToPath(outline, penX, penY, rot)
			if gp != nil {
				spanPaints[si] = append(spanPaints[si], gp)
			}
			penX += gl.XAdvance + spacing
		}
	}

	// Per-span paint: each span carries its own attrs (tspan overrides), so
	// spans fill/stroke independently (a whole-run union would flatten the
	// per-span styles into one paint).
	anyInk := false
	for si := range spanPaints {
		if len(spanPaints[si]) > 0 {
			anyInk = true
			break
		}
	}
	if !anyInk {
		return
	}

	dc.Push()
	applyElementTransform(dc, a)

	// Whole-run bounding box (clip/mask region resolution).
	var fullBBox render.Rect
	first := true
	for _, paths := range spanPaints {
		for _, gp := range paths {
			b := gp.BoundingBox()
			if first {
				fullBBox, first = b, false
				continue
			}
			if b.Min.X < fullBBox.Min.X {
				fullBBox.Min.X = b.Min.X
			}
			if b.Min.Y < fullBBox.Min.Y {
				fullBBox.Min.Y = b.Min.Y
			}
			if b.Max.X > fullBBox.Max.X {
				fullBBox.Max.X = b.Max.X
			}
			if b.Max.Y > fullBBox.Max.Y {
				fullBBox.Max.Y = b.Max.Y
			}
		}
	}

	// clip-path / mask resolve against the real glyph bounding box now that
	// the run is laid out (the generic pre-layout wrappers need bbox up
	// front, which only exists here for text).
	if a.ClipPath != "" && !a.clipInvalid {
		if id, isRef := paintRefID(a.ClipPath); isRef {
			if clip, rule, ok := st.doc.clipRegionFor(id, fullBBox, st.rs); ok {
				dc.SetFillRule(rule)
				dc.DrawPath(clip)
				dc.Clip()
			} else {
				dc.Pop()
				return // empty/degenerate clip region: nothing visible
			}
		}
	}
	if a.Mask != "" && !a.maskInvalid {
		if id, isRef := paintRefID(a.Mask); isRef {
			if mt, okM := st.doc.ElementByID(id).(*MaskElement); okM {
				okMask := false
				for si, sp := range spans {
					paths := spanPaints[si]
					if len(paths) == 0 {
						continue
					}
					spanProps := evalProps(p, sp.attrs, st.doc)
					spanSt := *st
					spanSt.rs.FontSize = spanProps.fontSize
					var combined *render.Path
					if len(paths) == 1 {
						combined = paths[0]
					} else {
						combined = paths[0]
						for _, gp := range paths[1:] {
							combined = combined.Op(gp, render.PathOpUnion)
						}
					}
					if renderTextMasked(dc, combined, combined.BoundingBox(), mt, &sp.attrs, spanProps, &spanSt) {
						okMask = true
					}
				}
				dc.Pop()
				if !okMask {
					return // empty mask region: everything hidden
				}
				return
			}
		}
	}

	for si, sp := range spans {
		paths := spanPaints[si]
		if len(paths) == 0 {
			continue
		}
		spanProps := evalProps(p, sp.attrs, st.doc)
		spanSt := *st
		spanSt.rs.FontSize = spanProps.fontSize
		var combined *render.Path
		if len(paths) == 1 {
			combined = paths[0]
		} else {
			combined = paths[0]
			for _, gp := range paths[1:] {
				combined = combined.Op(gp, render.PathOpUnion)
			}
		}
		bbox := combined.BoundingBox()
		fillAndStroke(dc, &sp.attrs, spanProps, &spanSt, bbox, func() {
			dc.DrawPath(combined)
		})
	}
	dc.Pop()
}

// renderTextMasked composes an already-laid-out text path through a
// luminance mask (same user-space three-offscreen scheme as renderMasked,
// but the content leg is the shaped glyph path). Returns false when the
// mask region is empty (everything hidden).
func renderTextMasked(dc *render.Context, combined *render.Path, bbox render.Rect, m *MaskElement, a *Attrs, p *props, st *renderState) bool {
	if m.ContentUnits == UnitsObjectBoundingBox {
		return false // named refusal (registered limitation)
	}
	bw := bbox.Max.X - bbox.Min.X
	bh := bbox.Max.Y - bbox.Min.Y
	regionCoord := func(l Length, axis LengthAxis, min float64, dim float64) float64 {
		if m.Units == UnitsUserSpaceOnUse {
			return l.Resolve(st.rs, axis)
		}
		frac := l.Value
		if l.Unit == LengthUnitPercent {
			frac = l.Value / 100
		}
		return min + frac*dim
	}
	rx := regionCoord(m.X, LengthAxisX, bbox.Min.X, bw)
	ry := regionCoord(m.Y, LengthAxisY, bbox.Min.Y, bh)
	rw := regionCoord(m.W, LengthAxisX, bbox.Min.X, bw) - rx
	rh := regionCoord(m.H, LengthAxisY, bbox.Min.Y, bh) - ry
	if rw <= 0 || rh <= 0 {
		return false
	}
	ca := render.NewContext(int(rw), int(rh))
	ca.Translate(-rx, -ry)
	maskProps := evalProps(p, m.Attrs, st.doc)
	maskSt := *st
	maskSt.rs.FontSize = maskProps.fontSize
	renderElements(ca, m.Children, maskProps, &maskSt)
	lm := render.NewLuminanceMask(ca.Image())
	if lm == nil {
		return false
	}
	cb := render.NewContext(int(rw), int(rh))
	cb.Translate(-rx, -ry)
	// Content leg: run the span's own paint (fill color/gradient) so the
	// masked text keeps its fill — a bare black Fill would drop it.
	fillAndStroke(cb, a, p, st, bbox, func() {
		cb.DrawPath(combined)
	})
	cb.ApplyMask(lm)
	img := cb.Image()
	if img == nil {
		return false
	}
	dc.DrawImage(render.ImageBufFromImage(img), rx, ry)
	return true
}

// familyList splits the font-family list; the first family is the primary.
func familyList(a Attrs) []string {
	f := strings.TrimSpace(a.FontFamily)
	if f == "" {
		return []string{""}
	}
	parts := strings.Split(f, ",")
	for i := range parts {
		parts[i] = strings.Trim(strings.TrimSpace(parts[i]), "'\"")
	}
	return parts
}

func mapWeight(a Attrs) text.FontWeight {
	w := strings.ToLower(strings.TrimSpace(a.FontWeight))
	switch w {
	case "bold", "bolder", "700", "800", "900":
		return text.WeightBold
	}
	return text.WeightRegular
}

func textAnchorOf(a Attrs) string {
	return strings.ToLower(strings.TrimSpace(a.TextAnchor))
}

func letterSpacingOf(a Attrs, rs RefSpace) float64 {
	if a.LetterSpacing == "" {
		return 0
	}
	if l, err := parseLength(a.LetterSpacing); err == nil {
		return l.Resolve(rs, LengthAxisX)
	}
	return 0
}

func firstLength(raw string) *Length {
	if raw == "" {
		return nil
	}
	list := parseLengthList(raw)
	if len(list) == 0 {
		return nil
	}
	return &list[0]
}

// outlineToPath flattens a glyph outline into a Path at (x, y) with an
// optional per-glyph rotation (degrees, around the glyph origin). Font Y is
// up, screen Y is down → Y is flipped.
func outlineToPath(outline *text.GlyphOutline, x, y, rotateDeg float64) *render.Path {
	if outline == nil || outline.SegmentCount() == 0 {
		return nil
	}
	var m render.Matrix
	if rotateDeg != 0 {
		m = render.Rotate(rotateDeg * 3.141592653589793 / 180.0)
	}
	fl := func(px, py float32) (float64, float64) {
		fx := float64(px)
		fy := -float64(py) // font Y up → screen Y down
		if rotateDeg != 0 {
			pt := m.TransformPoint(render.Point{X: fx, Y: fy})
			return pt.X, pt.Y
		}
		return fx, fy
	}
	b := render.BuildPath()
	segs := outline.Segments
	closeSub := func() { b.Close() } // close at subpath boundaries only
	for i, seg := range segs {
		switch seg.Op {
		case text.OutlineOpMoveTo:
			if i > 0 {
				closeSub() // the previous subpath ends where a new one starts
			}
			if i+1 < len(segs) && segs[i+1].Op == text.OutlineOpMoveTo {
				continue // degenerate contour (dot artifact), scene/text 同款
			}
			if i+1 >= len(segs) {
				closeSub() // trailing MoveTo: close its (empty) subpath
				continue
			}
			fx, fy := fl(seg.Points[0].X, seg.Points[0].Y)
			b.MoveTo(x+fx, y+fy)
		case text.OutlineOpLineTo:
			fx, fy := fl(seg.Points[0].X, seg.Points[0].Y)
			b.LineTo(x+fx, y+fy)
		case text.OutlineOpQuadTo:
			cx, cy := fl(seg.Points[0].X, seg.Points[0].Y)
			ex, ey := fl(seg.Points[1].X, seg.Points[1].Y)
			b.QuadTo(x+cx, y+cy, x+ex, y+ey)
		case text.OutlineOpCubicTo:
			c1x, c1y := fl(seg.Points[0].X, seg.Points[0].Y)
			c2x, c2y := fl(seg.Points[1].X, seg.Points[1].Y)
			ex, ey := fl(seg.Points[2].X, seg.Points[2].Y)
			b.CubicTo(x+c1x, y+c1y, x+c2x, y+c2y, x+ex, y+ey)
		}
	}
	// Close the final subpath: every contour is a closed shape, and an open
	// MoveTo..LineTo chain fills nothing on the scanline rasterizer.
	closeSub()
	return b.Path()
}
