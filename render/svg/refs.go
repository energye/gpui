//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import "strings"

// useDepthLimit bounds <use> instantiation chains. Cycles are rejected at
// parse (static reference graph); legit chains deeper than the limit are
// refused with a warning. Render recursion is therefore bounded too.
const useDepthLimit = 64

// validateReferences runs after the document tree is built:
//  1. index every id-carrying element (defs content included);
//  2. resolve <use> href targets and classify them;
//  3. detect use reference cycles / over-deep chains (static graph walk);
//  4. validate fill/stroke url(#id) paints on the rendered tree —
//     unresolved or not-yet-supported targets render as "not painted"
//     (fail-closed) with a warning, never as black.
func validateReferences(pc *parseCtx) {
	doc := pc.doc
	doc.byID = make(map[string]Element)
	walkElements(doc.Elements, func(e Element) bool {
		if a := e.attrs(); a != nil && a.ID != "" {
			if _, dup := doc.byID[a.ID]; !dup {
				doc.byID[a.ID] = e
			}
		}
		return true
	})

	uses := collectUses(doc.Elements)
	for _, u := range uses {
		resolveUse(pc, u)
	}
	detectUseCycles(pc, uses)
	validatePaints(pc)
	validateClips(pc)
}

// validateTextFonts resolves every text span's font at parse time: an
// unmatched family is a named warning and marks the element not-rendered
// (fail-closed, without mutating document state during render).
func validateTextFonts(pc *parseCtx, te *TextElement) {
	spans := flattenTextSpans(*te, nil)
	for _, sp := range spans {
		family := familyList(sp.attrs)[0]
		if family == "" {
			continue
		}
		if _, ok := resolveFont(family, mapWeight(sp.attrs),
			strings.Contains(strings.ToLower(sp.attrs.FontStyle), "italic")); !ok {
			te.attrs().fontInvalid = true
			pc.warn("text", "font-family", warnUnresolvedRef, "no resolver match for "+family)
			return
		}
	}
}

// validateClips checks clip-path="url(#id)" references on the rendered
// tree: unresolved targets mark the element not-rendered (fail-closed).
func validateClips(pc *parseCtx) {
	var walk func(elems []Element)
	walk = func(elems []Element) {
		for _, e := range elems {
			if e == nil {
				continue
			}
			if _, isDefs := e.(*DefsElement); isDefs {
				continue
			}
			a := e.attrs()
			if a != nil && a.ClipPath != "" {
				id := extractURLRefID(a.ClipPath)
				tgt, ok := pc.doc.byID[id]
				if !ok || tgt == nil {
					a.clipInvalid = true
					pc.warn(a.tag, "clip-path", warnUnresolvedRef, "#"+id)
				} else if _, isClip := tgt.(*ClipPathElement); !isClip {
					a.clipInvalid = true
					pc.warn(a.tag, "clip-path", warnUnsupportedRef, "#"+id)
				}
			}
			if te, isText := e.(*TextElement); isText {
				validateTextFonts(pc, te)
			}
			if a != nil && a.Filter != "" {
				id := extractURLRefID(a.Filter)
				tgt, ok := pc.doc.byID[id]
				if !ok || tgt == nil {
					a.filterInvalid = true
					pc.warn(a.tag, "filter", warnUnresolvedRef, "#"+id)
				} else if _, isFilter := tgt.(*FilterElement); !isFilter {
					a.filterInvalid = true
					pc.warn(a.tag, "filter", warnUnsupportedRef, "#"+id)
				} else {
					for _, fp := range tgt.(*FilterElement).Primitives {
						if !supportedFilterPrimitive(fp.Kind) {
							a.filterInvalid = true
							pc.warn(fp.Kind, "filter", warnUnsupportedElement, "#"+id)
							break
						}
					}
				}
			}
			if a != nil && a.Mask != "" {
				id := extractURLRefID(a.Mask)
				tgt, ok := pc.doc.byID[id]
				if !ok || tgt == nil {
					a.maskInvalid = true
					pc.warn(a.tag, "mask", warnUnresolvedRef, "#"+id)
				} else if _, isMask := tgt.(*MaskElement); !isMask {
					a.maskInvalid = true
					pc.warn(a.tag, "mask", warnUnsupportedRef, "#"+id)
				}
			}
			switch t := e.(type) {
			case *GroupElement:
				walk(t.Children)
			case *SymbolElement:
				walk(t.Children)
			case *SVGElement:
				walk(t.Children)
			}
		}
	}
	walk(pc.doc.Elements)
}

// walkElements visits every element in the tree (including defs/symbol
// content). Return false from visit to skip the subtree.
func walkElements(elems []Element, visit func(Element) bool) {
	for _, e := range elems {
		if e == nil {
			continue
		}
		if !visit(e) {
			continue
		}
		switch t := e.(type) {
		case *GroupElement:
			walkElements(t.Children, visit)
		case *DefsElement:
			walkElements(t.Children, visit)
		case *SymbolElement:
			walkElements(t.Children, visit)
		case *SVGElement:
			walkElements(t.Children, visit)
		case *ClipPathElement:
			walkElements(t.Children, visit)
		case *MaskElement:
			walkElements(t.Children, visit)
		case *PatternElement:
			walkElements(t.Children, visit)
		}
	}
}

// collectUses gathers every <use> in the document, defs content included
// (symbol bodies may reference further elements).
func collectUses(elems []Element) []*UseElement {
	var out []*UseElement
	walkElements(elems, func(e Element) bool {
		if u, ok := e.(*UseElement); ok {
			out = append(out, u)
		}
		return true
	})
	return out
}

// resolveUse classifies one use's target; unresolved targets mark the use
// invalid (renders nothing) with a warning.
func resolveUse(pc *parseCtx, u *UseElement) {
	if u.HRef == "" {
		u.invalid = true
		pc.warn("use", "href", warnUnresolvedRef, "empty href")
		return
	}
	id := strings.TrimPrefix(u.HRef, "#")
	tgt, ok := pc.doc.byID[id]
	if !ok || tgt == nil {
		u.invalid = true
		pc.warn("use", "href", warnUnresolvedRef, u.HRef)
		return
	}
	u.target = tgt
	if _, isSym := tgt.(*SymbolElement); isSym {
		u.kind = RefKindSymbol
	} else {
		u.kind = RefKindElement
	}
}

// detectUseCycles finds reference cycles and over-deep chains in the static
// use graph: an edge u→v exists when v is a use inside (or equal to) u's
// target subtree. Uses on a cycle are marked invalid; the chain root gets a
// warning (fail-closed, no hang, no stack overflow at render).
func detectUseCycles(pc *parseCtx, uses []*UseElement) {
	// Adjacency: for each use, the uses reachable through its target subtree.
	adj := make(map[*UseElement][]*UseElement, len(uses))
	byElem := make(map[Element]*UseElement, len(uses))
	for _, u := range uses {
		byElem[u] = u
	}
	for _, u := range uses {
		if u.target == nil {
			continue
		}
		var inner []*UseElement
		if t, ok := u.target.(*UseElement); ok {
			inner = append(inner, t)
		} else {
			walkElements([]Element{u.target}, func(e Element) bool {
				if v, ok := e.(*UseElement); ok {
					inner = append(inner, v)
				}
				return true
			})
		}
		adj[u] = inner
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[*UseElement]int, len(uses))
	cycleWarned := make(map[*UseElement]bool, len(uses))

	var dfs func(u *UseElement, path []*UseElement)
	dfs = func(u *UseElement, path []*UseElement) {
		color[u] = gray
		if len(path) >= useDepthLimit {
			if !cycleWarned[u] {
				cycleWarned[u] = true
				pc.warn("use", "href", warnUseDepth, "use chain exceeds depth limit")
			}
			u.invalid = true
		}
		for _, v := range adj[u] {
			if v.invalid || color[v] == black {
				continue
			}
			if color[v] == gray {
				// Back edge: mark every use on the cycle invalid, warn once.
				for _, pu := range path {
					pu.invalid = true
					if !cycleWarned[pu] {
						cycleWarned[pu] = true
						pc.warn("use", "href", warnUseCycle, pu.HRef)
					}
				}
				u.invalid = true
				if !cycleWarned[u] {
					cycleWarned[u] = true
					pc.warn("use", "href", warnUseCycle, u.HRef)
				}
				continue
			}
			dfs(v, append(path, u))
		}
		color[u] = black
	}
	for _, u := range uses {
		if color[u] == white {
			dfs(u, nil)
		}
	}
}

// validatePaints checks fill/stroke url(#id) references on the rendered
// tree (defs subtrees excluded — they only render through a use, whose own
// paint refs are validated when instantiated).
func validatePaints(pc *parseCtx) {
	var walk func(elems []Element)
	walk = func(elems []Element) {
		for _, e := range elems {
			if e == nil {
				continue
			}
			if _, isDefs := e.(*DefsElement); isDefs {
				continue
			}
			a := e.attrs()
			if a != nil {
				checkPaint(pc, e, a, &a.fillInvalid, a.Fill)
				checkPaint(pc, e, a, &a.strokeInvalid, a.Stroke)
			}
			switch t := e.(type) {
			case *GroupElement:
				walk(t.Children)
			case *SymbolElement:
				walk(t.Children)
			case *SVGElement:
				walk(t.Children)
			}
		}
	}
	walk(pc.doc.Elements)
}

func checkPaint(pc *parseCtx, e Element, a *Attrs, invalid *bool, value string) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "url(") {
		return
	}
	id := extractURLRefID(value)
	tgt, ok := pc.doc.byID[id]
	if !ok || tgt == nil {
		*invalid = true
		pc.warn(a.tag, paintAttrName(a, value), warnUnresolvedPaint, "#"+id)
		return
	}
	if !isPaintServer(tgt) {
		*invalid = true
		pc.warn(a.tag, paintAttrName(a, value), warnUnsupportedPaint, "#"+id)
		return
	}
	// Paint server present: zero-stop gradients stay invalid (SVG treats a
	// stopless gradient as a paint error); valid servers resolve at paint.
	if gp, isGrad := tgt.(stopsProvider); isGrad {
		if len(EffectiveStops(pc.doc, gp)) == 0 {
			*invalid = true
			pc.warn(a.tag, paintAttrName(a, value), warnUnresolvedPaint, "#"+id)
			return
		}
	}
}

// extractURLRefID pulls the fragment id out of `url(#id)` (optionally quoted,
// optionally with whitespace, per CSS url() syntax).
func extractURLRefID(v string) string {
	open := strings.Index(v, "(")
	close_ := strings.LastIndex(v, ")")
	if open < 0 || close_ <= open {
		return ""
	}
	inner := strings.TrimSpace(v[open+1 : close_])
	inner = strings.Trim(inner, `'" `)
	return strings.TrimPrefix(inner, "#")
}

func paintAttrName(a *Attrs, value string) string {
	if strings.TrimSpace(a.Fill) == strings.TrimSpace(value) {
		return "fill"
	}
	return "stroke"
}
