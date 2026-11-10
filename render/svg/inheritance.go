//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"math"
	"strconv"
	"strings"
)

// CSS declaration plumbing: `style` attributes and <style> element rules.
// Priority per SVG/CSS cascade: presentation attribute < tag rule < class
// rule < inline style attribute.

// CSSDecl is a single `name:value` declaration.
type CSSDecl struct {
	Name  string
	Value string
}

// props is the resolved inherited property set at one element (SVG 1.1
// inherited properties relevant to the static subset).
type props struct {
	fill, stroke  string // resolved color token: color, "none", "url(#id)", "" = unspecified
	fillOpacity   float64
	strokeOpacity float64
	fillRule      string
	// fillRuleExplicit distinguishes an explicit fill-rule from the initial
	// value (the legacy clip-rule fallback only fires when fill-rule is
	// absent). clipRule carries clip-rule until clipPath lands (paint batch).
	fillRuleExplicit bool
	clipRule         string
	strokeWidth      float64
	strokeCap        string
	strokeJoin       string
	strokeMiter      float64
	dash             []float64
	dashOffset       float64
	color            string
	fontSize         float64 // px
	fontFamily       string  // inherited (usvg: font-family is a CSS inherited property; "" = resolver default)
	opacity          float64 // element-level, never inherited
	visibility       string
	// Marker refs are not inherited (marker properties apply to the shape
	// itself); the marker shorthand sets all three, longhands override.
	markerStart, markerMid, markerEnd string
}

// rootProps returns the SVG initial property values.
func rootProps() *props {
	return &props{
		fill:          "black",
		stroke:        "",
		fillOpacity:   1,
		strokeOpacity: 1,
		fillRule:      "nonzero",
		strokeWidth:   1,
		strokeCap:     "butt",
		strokeJoin:    "miter",
		strokeMiter:   4,
		color:         "black",
		fontSize:      16,
		opacity:       1,
		visibility:    "visible",
	}
}

func (p *props) clone() *props {
	q := *p
	return &q
}

// evalProps computes the property set for one element: inherited parent
// values overridden by the element's own declarations. Stylesheet rules
// were materialized into the style attribute at parse time (css.go), so
// the cascade here is presentation attrs < inline style (which already
// carries the rules ahead of the element's own style declarations).
//
// fill/stroke resolve in a second pass: "currentColor" refers to the
// element's OWN computed color (CSS computed-value order), so every
// color-affecting declaration — including the element's own `color` —
// must be applied first. showcase 归因（批 4d）：fill="currentColor"
// color="#e24a4a" 同元素时旧实现按 parent.color 解析成黑。
func evalProps(parent *props, a Attrs, doc *Document) *props {
	p := parent.clone()
	p.opacity = 1 // element-level: reset before applying own

	var pres, inline []CSSDecl
	pres = presentationDecls(a)
	if a.Style != "" {
		for _, d := range strings.Split(a.Style, ";") {
			colon := strings.Index(d, ":")
			if colon < 0 {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(d[:colon]))
			value := strings.TrimSpace(d[colon+1:])
			if name != "" && value != "" {
				inline = append(inline, CSSDecl{Name: name, Value: value})
			}
		}
	}
	// Cascade order pass: everything except fill/stroke (last declaration
	// of each wins, inline beats presentation attrs).
	var fillDecl, strokeDecl *CSSDecl
	apply := func(d CSSDecl) {
		switch d.Name {
		case "fill":
			fillDecl = &d
		case "stroke":
			strokeDecl = &d
		default:
			applyDecl(p, d, parent, doc)
		}
	}
	for _, d := range pres {
		apply(d)
	}
	for _, d := range inline {
		apply(d)
	}
	// Paint pass: fill/stroke resolve against the element's own computed
	// color (p starts as parent.clone, so absent color = inherited value).
	if fillDecl != nil {
		applyDecl(p, *fillDecl, parent, doc)
	}
	if strokeDecl != nil {
		applyDecl(p, *strokeDecl, parent, doc)
	}
	return p
}

// presentationDecls converts the explicitly-present presentation attributes
// into declaration form (absent = no declaration = inherit).
func presentationDecls(a Attrs) []CSSDecl {
	var out []CSSDecl
	add := func(name, value string) {
		if value != "" {
			out = append(out, CSSDecl{Name: name, Value: value})
		}
	}
	add("fill", a.Fill)
	add("fill-rule", a.FillRule)
	add("fill-opacity", a.FillOpacity)
	add("stroke", a.Stroke)
	add("stroke-opacity", a.StrokeOpacity)
	add("stroke-width", a.StrokeWidth)
	add("stroke-linecap", a.StrokeCap)
	add("stroke-linejoin", a.StrokeJoin)
	add("stroke-miterlimit", a.StrokeMiterlimit)
	add("stroke-dasharray", a.StrokeDasharray)
	add("stroke-dashoffset", a.StrokeDashoffset)
	add("opacity", a.Opacity)
	add("color", a.Color)
	add("font-size", a.FontSize)
	add("font-family", a.FontFamily)
	add("visibility", a.Visibility)
	add("marker-start", a.MarkerStart)
	add("marker-mid", a.MarkerMid)
	add("marker-end", a.MarkerEnd)
	return out
}

// applyDecl applies one declaration to p. Length values resolve against the
// inherited font-size only (viewport-relative % on stroke widths/dashes is
// out of the batch-1 scope and treated as user units — documented limit).
// Color values resolve var() references first (SVG color-table palette).
func applyDecl(p *props, d CSSDecl, parent *props, doc *Document) {
	fs := RefSpace{FontSize: parent.fontSize}
	switch d.Name {
	case "fill":
		p.fill = resolveColorToken(resolveVarTokens(doc, d.Value), p)
	case "stroke":
		p.stroke = resolveColorToken(resolveVarTokens(doc, d.Value), p)
	case "color":
		p.color = resolveColorToken(resolveVarTokens(doc, d.Value), parent)
	case "fill-opacity":
		if v, ok := parseUnitFloat(d.Value); ok {
			p.fillOpacity = clamp01(v)
		}
	case "stroke-opacity":
		if v, ok := parseUnitFloat(d.Value); ok {
			p.strokeOpacity = clamp01(v)
		}
	case "opacity":
		if v, ok := parseUnitFloat(d.Value); ok {
			p.opacity = clamp01(v)
		}
	case "fill-rule":
		p.fillRule = strings.ToLower(strings.TrimSpace(d.Value))
		p.fillRuleExplicit = true
	case "clip-rule":
		p.clipRule = strings.ToLower(strings.TrimSpace(d.Value))
	case "stroke-width":
		if l, err := parseLength(d.Value); err == nil {
			p.strokeWidth = l.Resolve(fs, LengthAxisX)
		}
	case "stroke-linecap":
		p.strokeCap = strings.ToLower(strings.TrimSpace(d.Value))
	case "stroke-linejoin":
		p.strokeJoin = strings.ToLower(strings.TrimSpace(d.Value))
	case "stroke-miterlimit":
		if v, ok := parseUnitFloat(d.Value); ok && v >= 1 {
			p.strokeMiter = v
		}
	case "stroke-dasharray":
		p.dash = parseDashArray(d.Value, fs)
	case "stroke-dashoffset":
		if l, err := parseLength(d.Value); err == nil {
			p.dashOffset = l.Resolve(fs, LengthAxisX)
		}
	case "font-size":
		p.fontSize = resolveFontSize(d.Value, parent.fontSize)
	case "font-family":
		p.fontFamily = strings.TrimSpace(d.Value)
	case "visibility":
		p.visibility = strings.ToLower(strings.TrimSpace(d.Value))
	case "marker-start":
		p.markerStart = strings.TrimSpace(d.Value)
	case "marker-mid":
		p.markerMid = strings.TrimSpace(d.Value)
	case "marker-end":
		p.markerEnd = strings.TrimSpace(d.Value)
	case "marker":
		v := strings.TrimSpace(d.Value)
		p.markerStart, p.markerMid, p.markerEnd = v, v, v
	}
	// Unknown property names are ignored (CSS forward compatibility).
}

// resolveColorToken resolves "currentColor" against the inherited color;
// everything else passes through (including "none" and url(...) refs).
func resolveColorToken(v string, parent *props) string {
	v = strings.TrimSpace(v)
	if strings.EqualFold(v, "currentColor") {
		if parent.color == "" {
			return "black"
		}
		return parent.color
	}
	return v
}

// resolveFontSize resolves font-size against the parent size: %/em/ex are
// relative to it, absolute units convert at 96dpi.
func resolveFontSize(v string, parentFS float64) float64 {
	v = strings.TrimSpace(v)
	if l, err := parseLength(v); err == nil {
		switch l.Unit {
		case LengthUnitPercent:
			return parentFS * l.Value / 100
		case LengthUnitEm:
			return parentFS * l.Value
		case LengthUnitEx:
			return parentFS * l.Value * 0.5
		default:
			return l.Resolve(RefSpace{FontSize: parentFS}, LengthAxisX)
		}
	}
	return parentFS
}

// parseDashArray parses stroke-dasharray. "none" clears; an odd number of
// values is repeated to even per SVG 1.1.
func parseDashArray(v string, fs RefSpace) []float64 {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "none") {
		return nil
	}
	fields := strings.FieldsFunc(v, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' })
	var out []float64
	for _, f := range fields {
		l, err := parseLength(f)
		if err != nil {
			return nil
		}
		out = append(out, l.Resolve(fs, LengthAxisX))
	}
	if len(out)%2 == 1 {
		out = append(out, out...)
	}
	return out
}

func parseUnitFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
