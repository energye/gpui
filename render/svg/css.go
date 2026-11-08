//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

// CSS stylesheet resolution, aligned with usvg's simplecss semantics
// (crates/usvg/src/parser/svgtree/parse.rs resolve_css + parse_svg_element):
//
//   - cascade: presentation attributes < stylesheet rules < style attribute
//     (each later declaration overrides the same property);
//   - the !important flag is parsed and ignored (simplecss does not sort
//     by importance — resvg reference images confirm the behavior);
//   - `type` on <style>: "text/css" or absent parses, anything else skips
//     the whole sheet;
//   - selectors: type, universal, #id, .class, [attr] with all simplecss
//     operators (=, ~=, |=, ^=, $=, *=), :first-child, and the four
//     combinators (descendant, '>', '+', '~');
//   - matching runs at parse time and matched declarations are materialized
//     onto the element (transform replaces the transform attribute; the
//     rest merge into the style attribute ahead of its own declarations) —
//     the same insert_attribute flow usvg uses.

import (
	"sort"
	"strings"
)

// cssAttrOp is one `[attr]`/`[attr<op>value]` test.
type cssAttrOp struct {
	name  string
	op    string // "", "=", "~=", "|=", "^=", "$=", "*="
	value string
}

// cssCompound is one compound selector (no combinators inside).
type cssCompound struct {
	universal  bool
	tag        string
	id         string
	classes    []string
	attrs      []cssAttrOp
	firstChild bool
}

// cssSelPart is a compound plus the combinator to its LEFT (first part: 0).
type cssSelPart struct {
	comp cssCompound
	comb byte // 0, ' ', '>', '+', '~'
}

// cssSelector is a parsed complex selector.
type cssSelector struct {
	parts []cssSelPart
}

// parseStyleContent parses <style> text into rules. The whole sheet is
// skipped when an explicit type says it is not CSS. Selectors outside the
// supported set skip their rule with a warning (never half-applied).
func parseStyleContent(pc *parseCtx, content string, styleType string) StyleBlock {
	if styleType != "" && styleType != "text/css" {
		pc.warn("style", "type", warnUnsupportedSelector, "stylesheet type "+styleType+" skipped")
		return StyleBlock{}
	}
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
		parsed, ok := parseSelector(sel)
		if !ok {
			pc.warn("style", "selector", warnUnsupportedSelector, "unsupported selector "+sel)
			continue
		}
		var decls []CSSDecl
		for _, d := range strings.Split(chunk[open+1:], ";") {
			colon := strings.Index(d, ":")
			if colon < 0 {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(d[:colon]))
			value := strings.TrimSpace(d[colon+1:])
			value = stripImportant(value)
			if name == "" || value == "" {
				continue
			}
			decls = append(decls, CSSDecl{Name: name, Value: value})
		}
		if len(decls) > 0 {
			out.Rules = append(out.Rules, cssRuleParsed{sel: parsed, decls: decls})
		}
	}
	return out
}

// stripImportant removes a trailing !important (case-insensitive, optional
// inner whitespace) — the flag is parsed and ignored, matching usvg.
func stripImportant(value string) string {
	low := strings.ToLower(value)
	i := strings.LastIndex(low, "!")
	if i < 0 {
		return value
	}
	tail := strings.TrimSpace(low[i+1:])
	if tail == "important" {
		return strings.TrimSpace(value[:i])
	}
	return value
}

// parseSelector parses one complex selector; ok=false marks an unsupported
// construct (the rule is skipped, matching simplecss's parse failure).
func parseSelector(sel string) (cssSelector, bool) {
	var out cssSelector
	i := 0
	n := len(sel)
	for i < n {
		// Combinator before the next compound.
		var comb byte
		sawSpace := false
		for i < n {
			c := sel[i]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				sawSpace = true
				i++
				continue
			}
			if c == '>' || c == '+' || c == '~' {
				if comb != 0 && comb != ' ' {
					return cssSelector{}, false // "a > > b"
				}
				comb = c
				i++
				continue
			}
			break
		}
		if i >= n {
			return cssSelector{}, false // trailing combinator
		}
		comp, next, ok := parseCompound(sel, i)
		if !ok {
			return cssSelector{}, false
		}
		if comb == 0 && sawSpace && len(out.parts) > 0 {
			comb = ' ' // "a b" descendant
		}
		out.parts = append(out.parts, cssSelPart{comp: comp, comb: comb})
		i = next
	}
	if len(out.parts) == 0 {
		return cssSelector{}, false
	}
	return out, true
}

// parseCompound parses one compound selector starting at i; returns the
// index just past it.
func parseCompound(s string, i int) (cssCompound, int, bool) {
	var c cssCompound
	n := len(s)
	start := i
	for i < n {
		switch s[i] {
		case '*':
			c.universal = true
			i++
		case '#':
			name, next, ok := parseIdent(s, i+1)
			if !ok {
				return c, i, false
			}
			c.id = name
			i = next
		case '.':
			name, next, ok := parseIdent(s, i+1)
			if !ok {
				return c, i, false
			}
			c.classes = append(c.classes, name)
			i = next
		case '[':
			op, next, ok := parseAttrOp(s, i)
			if !ok {
				return c, i, false
			}
			c.attrs = append(c.attrs, op)
			i = next
		case ':':
			name, next, ok := parseIdent(s, i+1)
			if !ok {
				return c, i, false
			}
			if name != "first-child" {
				return c, i, false // other pseudo-classes: not supported
			}
			c.firstChild = true
			i = next
		case '>', '+', '~', ' ', '\t', '\n', '\r', '{', ',':
			goto done
		default:
			name, next, ok := parseIdent(s, i)
			if !ok {
				return c, i, false
			}
			if c.tag != "" {
				return c, i, false // two tags in one compound
			}
			c.tag = name
			i = next
		}
	}
done:
	if i == start {
		return c, i, false // empty compound
	}
	return c, i, true
}

// parseIdent reads a CSS identifier (letters, digits, -, _).
func parseIdent(s string, i int) (string, int, bool) {
	start := i
	for i < len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			i++
			continue
		}
		break
	}
	if i == start {
		return "", i, false
	}
	return s[start:i], i, true
}

// parseAttrOp parses `[name]`, `[name=op value]` with the simplecss
// operator set; the value may be quoted.
func parseAttrOp(s string, i int) (cssAttrOp, int, bool) {
	var op cssAttrOp
	n := len(s)
	i++ // past '['
	i += countSpace(s[i:])
	name, next, ok := parseIdent(s, i)
	if !ok {
		return op, i, false
	}
	op.name = name
	i = next
	i += countSpace(s[i:])
	if i < n && s[i] != ']' {
		switch {
		case s[i] == '=':
			op.op = "="
			i++
		case i+1 < n && (s[i:i+2] == "~=" || s[i:i+2] == "|=" || s[i:i+2] == "^=" || s[i:i+2] == "$=" || s[i:i+2] == "*="):
			op.op = s[i : i+2]
			i += 2
		default:
			return op, i, false
		}
		i += countSpace(s[i:])
		if i >= n {
			return op, i, false
		}
		if s[i] == '"' || s[i] == '\'' {
			q := s[i]
			end := strings.IndexByte(s[i+1:], q)
			if end < 0 {
				return op, i, false
			}
			op.value = s[i+1 : i+1+end]
			i = i + 1 + end + 1
		} else {
			v, next, ok := parseIdent(s, i)
			if !ok {
				return op, i, false
			}
			op.value = v
			i = next
		}
		i += countSpace(s[i:])
	}
	if i >= n || s[i] != ']' {
		return op, i, false
	}
	return op, i + 1, true
}

func countSpace(s string) int {
	i := 0
	for i < len(s) {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// cssMatchCtx carries the structural context matching needs: the ancestor
// attribute chain (outermost first) and the sibling sequence position.
type cssMatchCtx struct {
	ancestors []*Attrs // last = direct parent
	siblings  []Element
	index     int
}

// matchSelector matches a parsed selector right-to-left (simplecss order).
func matchSelector(sel cssSelector, a *Attrs, ctx *cssMatchCtx) bool {
	last := len(sel.parts) - 1
	if !compoundMatches(sel.parts[last].comp, a, ctx) {
		return false
	}
	elem := a
	cur := ctx
	for i := last - 1; i >= 0; i-- {
		switch sel.parts[i+1].comb {
		case '>':
			if len(cur.ancestors) == 0 {
				return false
			}
			elem = cur.ancestors[len(cur.ancestors)-1]
			cur = parentCtx(cur)
			if !compoundMatches(sel.parts[i].comp, elem, cur) {
				return false
			}
		case ' ':
			found := false
			for len(cur.ancestors) > 0 {
				elem = cur.ancestors[len(cur.ancestors)-1]
				cur = parentCtx(cur)
				if compoundMatches(sel.parts[i].comp, elem, cur) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case '+':
			prev := prevSibling(cur)
			if prev == nil || !compoundMatches(sel.parts[i].comp, prev, siblingCtx(cur, cur.index-1)) {
				return false
			}
		case '~':
			found := false
			for j := cur.index - 1; j >= 0; j-- {
				ps := siblingAttrs(cur.siblings, j)
				if ps != nil && compoundMatches(sel.parts[i].comp, ps, siblingCtx(cur, j)) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// parentCtx derives the context seen from the direct parent.
func parentCtx(ctx *cssMatchCtx) *cssMatchCtx {
	if len(ctx.ancestors) == 0 {
		return ctx
	}
	// Ancestor contexts beyond the walked chain are unknown: matching
	// against them only needs their own attrs + their ancestor chain,
	// both carried in ancestors.
	clone := &cssMatchCtx{ancestors: ctx.ancestors[:len(ctx.ancestors)-1]}
	// Grandparent siblings are not tracked (siblings matter only at the
	// matched element's level for '+'/'~'); descendant climbing does not
	// consult them.
	return clone
}

func prevSibling(ctx *cssMatchCtx) *Attrs {
	if ctx.index <= 0 {
		return nil
	}
	return siblingAttrs(ctx.siblings, ctx.index-1)
}

func siblingAttrs(sibs []Element, i int) *Attrs {
	if i < 0 || i >= len(sibs) {
		return nil
	}
	e := sibs[i]
	if e == nil {
		return nil
	}
	if a := e.attrs(); a != nil {
		return a
	}
	return nil
}

func siblingCtx(ctx *cssMatchCtx, index int) *cssMatchCtx {
	return &cssMatchCtx{ancestors: ctx.ancestors, siblings: ctx.siblings, index: index}
}

// compoundMatches tests one compound against the element.
func compoundMatches(c cssCompound, a *Attrs, ctx *cssMatchCtx) bool {
	if a == nil {
		return false
	}
	if !c.universal && c.tag != "" && c.tag != a.tag {
		return false
	}
	if c.id != "" && c.id != a.ID {
		return false
	}
	for _, cls := range c.classes {
		if !hasClass(a.Class, cls) {
			return false
		}
	}
	for _, op := range c.attrs {
		if !attrOpMatches(op, a) {
			return false
		}
	}
	if c.firstChild && ctx != nil && ctx.index != firstElementIndex(ctx.siblings) {
		return false
	}
	return true
}

// firstElementIndex returns the index of the first sibling with attrs
// (usvg prev_sibling_element: previous XML element sibling).
func firstElementIndex(sibs []Element) int {
	for i, e := range sibs {
		if e == nil {
			continue
		}
		if e.attrs() != nil {
			return i
		}
	}
	return -1
}

func hasClass(classList, cls string) bool {
	for _, f := range strings.Fields(classList) {
		if f == cls {
			return true
		}
	}
	return false
}

// attrOpMatches tests one [attr] op against the raw attribute surface.
func attrOpMatches(op cssAttrOp, a *Attrs) bool {
	v, ok := a.Raw[op.name]
	if !ok {
		return false
	}
	switch op.op {
	case "":
		return true
	case "=":
		return v == op.value
	case "~=":
		return hasClass(v, op.value)
	case "|=":
		return v == op.value || strings.HasPrefix(v, op.value+"-")
	case "^=":
		return op.value != "" && strings.HasPrefix(v, op.value)
	case "$=":
		return op.value != "" && strings.HasSuffix(v, op.value)
	case "*=":
		return op.value != "" && strings.Contains(v, op.value)
	}
	return false
}

// StyleBlock is the parsed content of one <style> element; rules carry
// their selector pre-parsed (unsupported selectors were skipped at parse).
type StyleBlock struct {
	Rules []cssRuleParsed
}

// cssRuleParsed is a stylesheet rule with its selector pre-parsed.
type cssRuleParsed struct {
	sel   cssSelector
	decls []CSSDecl
}

// specificity returns the simplecss specificity triple [ids, classes+attrs,
// types] — subselectors count classes/attributes/pseudo-classes as b and
// the compound's type as c; the universal selector counts nothing.
func (sel cssSelector) specificity() [3]int {
	var spec [3]int
	for _, part := range sel.parts {
		c := part.comp
		if c.tag != "" {
			spec[2]++
		}
		if c.id != "" {
			spec[0]++
		}
		spec[1] += len(c.classes) + len(c.attrs)
		if c.firstChild {
			spec[1]++
		}
	}
	return spec
}

// resolveCSS matches the stylesheet against the parsed tree and
// materializes declarations onto elements (usvg parse_svg_element flow):
// transform replaces the transform attribute, everything else merges into
// the style attribute ahead of its own declarations, so the cascade order
// presentation attr < CSS < style attribute falls out of evalProps.
func resolveCSS(pc *parseCtx) {
	doc := pc.doc
	var sheet []cssRuleParsed
	for _, blk := range doc.Styles {
		sheet = append(sheet, blk.Rules...)
	}
	// simplecss sorts the stylesheet by specificity (stable, ascending):
	// higher-specificity rules apply later and win the cascade, equal
	// specificity keeps declaration order.
	sort.SliceStable(sheet, func(i, j int) bool {
		a, b := sheet[i].sel.specificity(), sheet[j].sel.specificity()
		return a[0] < b[0] || (a[0] == b[0] && (a[1] < b[1] || (a[1] == b[1] && a[2] < b[2])))
	})
	var walk func(elems []Element, anc []*Attrs, sibs []Element)
	walk = func(elems []Element, anc []*Attrs, sibs []Element) {
		for idx, e := range elems {
			if e == nil {
				continue
			}
			a := e.attrs()
			if a != nil {
				// Matched stylesheet declarations first, then the element's
				// own style attribute (usvg's write_declaration order);
				// transform declarations land in the transform attribute.
				var decls []CSSDecl
				for _, r := range sheet {
					ctx := &cssMatchCtx{ancestors: anc, siblings: elems, index: idx}
					if matchSelector(r.sel, a, ctx) {
						decls = append(decls, r.decls...)
					}
				}
				decls = append(decls, splitStyleDecls(a.Style)...)
				if len(decls) > 0 {
					materializeCSS(a, decls)
				}
			}
			nextAnc := anc
			if a != nil {
				nextAnc = append(append([]*Attrs{}, anc...), a)
			}
			switch t := e.(type) {
			case *GroupElement:
				walk(t.Children, nextAnc, nil)
			case *DefsElement:
				walk(t.Children, nextAnc, nil)
			case *SymbolElement:
				walk(t.Children, nextAnc, nil)
			case *SVGElement:
				walk(t.Children, nextAnc, nil)
			case *ClipPathElement:
				walk(t.Children, nextAnc, nil)
			case *MaskElement:
				walk(t.Children, nextAnc, nil)
			case *PatternElement:
				walk(t.Children, nextAnc, nil)
			}
		}
	}
	// The root <svg> participates as an ancestor only (its tag matches
	// `svg …` selectors); rules targeting the root element itself are out
	// of the batch scope.
	rootAttrs := &Attrs{tag: "svg"}
	walk(doc.Elements, []*Attrs{rootAttrs}, nil)
	// Rules are consumed: evalProps sees them through the merged style
	// attributes only.
	doc.Styles = nil
}

// splitStyleDecls tokenizes a style attribute into declarations.
func splitStyleDecls(style string) []CSSDecl {
	if style == "" {
		return nil
	}
	var out []CSSDecl
	for _, d := range strings.Split(style, ";") {
		colon := strings.Index(d, ":")
		if colon < 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(d[:colon]))
		value := strings.TrimSpace(d[colon+1:])
		if name != "" && value != "" {
			out = append(out, CSSDecl{Name: name, Value: value})
		}
	}
	return out
}

// materializeCSS folds matched declarations into the element: the
// transform property replaces Attrs.Transform (a presentation attribute in
// usvg's flow), remaining declarations merge ahead of the existing style
// attribute so the style attribute still wins the cascade.
func materializeCSS(a *Attrs, decls []CSSDecl) {
	var style strings.Builder
	for _, d := range decls {
		if d.Name == "transform" {
			a.Transform = d.Value
			continue
		}
		if style.Len() > 0 {
			style.WriteByte(';')
		}
		style.WriteString(d.Name)
		style.WriteByte(':')
		style.WriteString(d.Value)
	}
	merged := style.String()
	if a.Style != "" {
		if merged != "" {
			merged += ";"
		}
		merged += a.Style
	}
	a.Style = merged
}

// stripCSSComments removes /* ... */ comments (unterminated: cut).
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
