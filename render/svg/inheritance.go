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

// CSSRule is one selector with its declaration block.
type CSSRule struct {
	Selector string
	Decls    []CSSDecl
}

// StyleBlock is the parsed content of one <style> element.
type StyleBlock struct {
	Rules []CSSRule
}

// parseStyleContent parses <style> text into rules. Selectors outside the
// minimal set (single class ".name", tag "name", comma groups of those) are
// skipped with a warning rather than half-applied.
func parseStyleContent(pc *parseCtx, content string) StyleBlock {
	content = stripCSSComments(content)
	var out StyleBlock
	for _, chunk := range strings.Split(content, "}") {
		open := strings.Index(chunk, "{")
		if open < 0 {
			continue
		}
		sel := strings.TrimSpace(chunk[:open])
		if sel == "" {
			continue
		}
		if !supportedSelector(sel) {
			pc.warn("style", "selector", warnUnsupportedSelector, "unsupported selector "+sel)
			continue
		}
		rule := CSSRule{Selector: sel}
		for _, d := range strings.Split(chunk[open+1:], ";") {
			colon := strings.Index(d, ":")
			if colon < 0 {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(d[:colon]))
			value := strings.TrimSpace(d[colon+1:])
			if name == "" || value == "" {
				continue
			}
			rule.Decls = append(rule.Decls, CSSDecl{Name: name, Value: value})
		}
		if len(rule.Decls) > 0 {
			out.Rules = append(out.Rules, rule)
		}
	}
	return out
}

func supportedSelector(sel string) bool {
	for _, part := range strings.Split(sel, ",") {
		part = strings.TrimSpace(part)
		if part == "" || strings.ContainsAny(part, " \t>+~[") {
			return false // descendant/attribute/complex selectors: not supported
		}
		if !strings.HasPrefix(part, ".") && !isName(part) {
			return false
		}
	}
	return true
}

func isName(s string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return len(s) > 0
}

func stripCSSComments(s string) string {
	for {
		open := strings.Index(s, "/*")
		if open < 0 {
			return s
		}
		close_ := strings.Index(s[open+2:], "*/")
		if close_ < 0 {
			return s[:open]
		}
		s = s[:open] + s[open+2+close_+2:]
	}
}

func ruleMatches(selector string, a Attrs) bool {
	for _, part := range strings.Split(selector, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, ".") {
			for _, cls := range strings.Fields(a.Class) {
				if cls == part[1:] {
					return true
				}
			}
		} else if part == a.tag {
			return true
		}
	}
	return false
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
	strokeCap     string
	strokeJoin    string
	strokeMiter   float64
	dash          []float64
	dashOffset    float64
	color         string
	fontSize      float64 // px
	opacity       float64 // element-level, never inherited
	visibility    string
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
// values overridden by the element's own declarations in cascade order
// (presentation attrs < tag rules < class rules < inline style).
func evalProps(parent *props, a Attrs, doc *Document) *props {
	p := parent.clone()
	p.opacity = 1 // element-level: reset before applying own

	var pres, tagDecls, classDecls, inline []CSSDecl
	pres = presentationDecls(a)
	for _, blk := range doc.Styles {
		for _, r := range blk.Rules {
			if !ruleMatches(r.Selector, a) {
				continue
			}
			// One rule's strength is decided by its strongest selector part
			// (a comma group matches if any part matches). Bucket by class
			// presence so ".a, rect" still beats a plain tag rule.
			classed := false
			for _, part := range strings.Split(r.Selector, ",") {
				if strings.HasPrefix(strings.TrimSpace(part), ".") {
					classed = true
					break
				}
			}
			if classed {
				classDecls = append(classDecls, r.Decls...)
			} else {
				tagDecls = append(tagDecls, r.Decls...)
			}
		}
	}
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
	for _, d := range pres {
		applyDecl(p, d, parent)
	}
	for _, d := range tagDecls {
		applyDecl(p, d, parent)
	}
	for _, d := range classDecls {
		applyDecl(p, d, parent)
	}
	for _, d := range inline {
		applyDecl(p, d, parent)
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
	add("visibility", a.Visibility)
	return out
}

// applyDecl applies one declaration to p. Length values resolve against the
// inherited font-size only (viewport-relative % on stroke widths/dashes is
// out of the batch-1 scope and treated as user units — documented limit).
func applyDecl(p *props, d CSSDecl, parent *props) {
	fs := RefSpace{FontSize: parent.fontSize}
	switch d.Name {
	case "fill":
		p.fill = resolveColorToken(d.Value, parent)
	case "stroke":
		p.stroke = resolveColorToken(d.Value, parent)
	case "color":
		p.color = resolveColorToken(d.Value, parent)
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
	case "visibility":
		p.visibility = strings.ToLower(strings.TrimSpace(d.Value))
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
