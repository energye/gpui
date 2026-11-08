//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"reflect"
	"strconv"
	"strings"
)

// parseCtx threads parse state (document under construction + warnings).
type parseCtx struct {
	doc *Document
	dec *xml.Decoder
	cfg ParseConfig
}

// ParseConfig carries the parse-time environment: the directory external
// <image> hrefs resolve against (usvg Options::resources_dir) and the
// sub-document nesting depth (internal).
type ParseConfig struct {
	// ResourcesDir is the base for plain-path <image> hrefs; empty keeps
	// the data-URI-only subset (external references warn and skip).
	ResourcesDir string

	// svgDepth is the <image>→SVG nesting depth (internal; Parse starts 0).
	svgDepth int `json:"-"`
}

// warn records a non-fatal diagnostic (parse continues). reason is a stable
// warn* code; detail is free-form context.
func (pc *parseCtx) warn(elem, attr, reason, detail string) {
	pc.doc.Warnings = append(pc.doc.Warnings, ParseWarning{
		Element:   elem,
		Attribute: attr,
		Reason:    reason,
		Detail:    detail,
		Offset:    int(pc.dec.InputOffset()),
	})
}

// Parse parses SVG XML data into a reusable [Document].
// The Document can be rendered multiple times at different sizes.
//
// Unsupported or dangling constructs are diagnosed in Document.Warnings and
// render as "not painted" (fail-closed) instead of silently wrong pixels.
func Parse(data []byte) (*Document, error) {
	return ParseWithConfig(data, ParseConfig{})
}

// ParseWithConfig parses with a parse-time environment (external image
// hrefs resolve against cfg.ResourcesDir).
func ParseWithConfig(data []byte, cfg ParseConfig) (*Document, error) {
	// DOCTYPE internal-subset entities are expanded textually before the
	// XML tokenizer (Go's decoder has no DTD support); non-UTF-8 documents
	// go through a charset reader per their encoding declaration.
	data = expandDTDEntities(data)
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charsetReader

	doc := &Document{
		ViewBox: ViewBox{Width: 16, Height: 16}, // legacy default when no viewBox
	}
	pc := &parseCtx{doc: doc, dec: decoder, cfg: cfg}

	// Find the root <svg> element.
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: failed to find <svg> root element: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "svg" && isSVGNameSpace(se.Name) {
			if err := parseSVGRoot(pc, se); err != nil {
				return nil, err
			}
			// Parse children.
			children, err := parseChildren(pc)
			if err != nil {
				return nil, err
			}
			doc.Elements = children
			// A failing condition on the root <svg> hides the whole
			// document (usvg converts nothing).
			if rootConditionFailed(se) {
				doc.Elements = nil
				doc.Styles = nil
			}
			validateReferences(pc)
			resolveCSS(pc)
			return doc, nil
		}
	}
}

// parseSVGRoot extracts viewBox, width, height, pAR, overflow and the root
// presentation attributes from the <svg> element.
func parseSVGRoot(pc *parseCtx, se xml.StartElement) error {
	doc := pc.doc
	// Read viewBox first: percentage width/height resolve against it.
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "viewBox":
			vb, err := parseViewBox(attr.Value)
			if err != nil {
				return err
			}
			doc.ViewBox = vb
			doc.hasViewBox = true
		}
	}

	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "width":
			v, err := parseDimension(attr.Value, doc.ViewBox.Width)
			if err != nil {
				return fmt.Errorf("svg: <svg> width: %w", err)
			}
			doc.Width = v
			doc.hasSize = true
		case "height":
			v, err := parseDimension(attr.Value, doc.ViewBox.Height)
			if err != nil {
				return fmt.Errorf("svg: <svg> height: %w", err)
			}
			doc.Height = v
			doc.hasSize = true
		case "preserveAspectRatio":
			doc.Par = parsePreserveAspectRatio(attr.Value)
		}
	}

	// Root presentation attributes feed the inheritance tree.
	doc.rootAttrs = parseAttrs(pc, se)
	doc.rootAttrs.tag = "svg"
	doc.RootFill = doc.rootAttrs.Fill
	if strings.TrimSpace(doc.rootAttrs.Overflow) == "visible" {
		doc.OverflowVisible = true
	}

	// If width/height not set, use viewBox.
	if doc.Width == 0 {
		doc.Width = doc.ViewBox.Width
	}
	if doc.Height == 0 {
		doc.Height = doc.ViewBox.Height
	}
	return nil
}

// parseViewBox parses a "minX minY width height" string.
func parseViewBox(s string) (ViewBox, error) {
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	if len(parts) != 4 {
		return ViewBox{}, fmt.Errorf("svg: invalid viewBox %q (expected 4 values)", s)
	}
	var vb ViewBox
	var err error
	if vb.MinX, err = strconv.ParseFloat(parts[0], 64); err != nil {
		return ViewBox{}, fmt.Errorf("svg: invalid viewBox minX: %w", err)
	}
	if vb.MinY, err = strconv.ParseFloat(parts[1], 64); err != nil {
		return ViewBox{}, fmt.Errorf("svg: invalid viewBox minY: %w", err)
	}
	if vb.Width, err = strconv.ParseFloat(parts[2], 64); err != nil {
		return ViewBox{}, fmt.Errorf("svg: invalid viewBox width: %w", err)
	}
	if vb.Height, err = strconv.ParseFloat(parts[3], 64); err != nil {
		return ViewBox{}, fmt.Errorf("svg: invalid viewBox height: %w", err)
	}
	return vb, nil
}

// parseDimension parses an SVG dimension value for the root width/height.
// Percentages resolve against ref (the viewBox axis), the legacy root-level
// convention; child geometry uses full Length units instead.
func parseDimension(s string, ref float64) (float64, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		raw := strings.TrimSpace(strings.TrimSuffix(s, "%"))
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return 0, fmt.Errorf("svg: invalid percentage dimension %q: %w", s, err)
		}
		return v * ref / 100, nil
	}
	l, err := parseLength(s)
	if err != nil {
		return 0, err
	}
	return l.Resolve(RefSpace{}, LengthAxisX), nil
}

// parseChildren parses child elements until the matching end element.
func parseChildren(pc *parseCtx) ([]Element, error) {
	var elements []Element

	for {
		tok, err := pc.dec.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: error reading XML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			elem, err := parseElement(pc, t)
			if err != nil {
				return nil, err
			}
			if elem != nil {
				elements = append(elements, elem)
			}

		case xml.EndElement:
			return elements, nil
		}
	}
}

// parseElement parses a single SVG element from a start element.
// Returns nil for constructs that produce no node (descriptive elements,
// collected-only content already registered elsewhere, elements dropped by
// tolerance rules). The nil is normalized at this boundary: a concrete
// parser's `return nil, nil` is a typed nil that would survive `!= nil`
// checks once wrapped in the Element interface.
func parseElement(pc *parseCtx, se xml.StartElement) (Element, error) {
	e, err := parseElementInner(pc, se)
	if e == nil || isNilElement(e) {
		return nil, err
	}
	checkConditions(e)
	return e, err
}

// isNilElement reports a typed-nil Element: a concrete parser's `return
// nil, nil` wraps (*T)(nil) into the interface, where it survives both
// `== nil` and `!= nil` checks (the interface holds a non-nil type word).
func isNilElement(e Element) bool {
	v := reflect.ValueOf(e)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

func parseElementInner(pc *parseCtx, se xml.StartElement) (Element, error) {
	// Elements outside the SVG namespace render nothing (usvg drops them at
	// tag-name resolution); their content stays in the tree so `use` can
	// still reference it by id (usvg resolves hrefs against the raw XML).
	if !isSVGNameSpace(se.Name) {
		return parseInertContainer(pc, se, false)
	}
	switch se.Name.Local {
	case "path":
		return parsePathElement(pc, se)
	case "circle":
		return parseCircleElement(pc, se)
	case "rect":
		return parseRectElement(pc, se)
	case "ellipse":
		return parseEllipseElement(pc, se)
	case "line":
		return parseLineElement(pc, se)
	case "polygon":
		return parsePolygonElement(pc, se)
	case "polyline":
		return parsePolylineElement(pc, se)
	case "g":
		return parseGroupElement(pc, se)
	case "defs":
		return parseDefsElement(pc, se)
	case "symbol":
		return parseSymbolElement(pc, se)
	case "use":
		return parseUseElement(pc, se)
	case "svg":
		return parseSVGElement(pc, se)
	case "style":
		return parseStyleElement(pc, se)
	case "linearGradient":
		return parseLinearGradient(pc, se)
	case "radialGradient":
		return parseRadialGradient(pc, se)
	case "stop":
		return parseStopElement(pc, se)
	case "clipPath":
		return parseClipPathElement(pc, se)
	case "mask":
		return parseMaskElement(pc, se)
	case "image":
		return parseImageElement(pc, se)
	case "pattern":
		return parsePatternElement(pc, se)
	case "text":
		return parseTextElement(pc, se)
	case "filter":
		return parseFilterElement(pc, se)
	case "feGaussianBlur", "feOffset", "feFlood", "feColorMatrix",
		"feBlend", "feComposite", "feMerge", "feDropShadow":
		// fe* 只出现在 <filter> 内：由 parseFilterPrimitives 消费；
		// 在 filter 外出现的 fe* 是具名拒绝（结构非法）。
		if err := pc.dec.Skip(); err != nil {
			return nil, err
		}
		pc.warn(se.Name.Local, "", warnUnsupportedElement, "fe* outside <filter>")
		return nil, nil //nolint:nilnil
	case "title", "desc", "metadata":
		// Descriptive: no rendering, no warning.
		if err := pc.dec.Skip(); err != nil {
			return nil, fmt.Errorf("svg: error skipping <%s>: %w", se.Name.Local, err)
		}
		return nil, nil //nolint:nilnil // descriptive element
	case "a":
		// Links are groups (usvg parse.rs: "Treat links as groups"); href
		// and target metadata carry no rendering semantics here.
		return parseGroupElement(pc, se)
	case "switch":
		// Conditional container: the first direct child element whose
		// conditions pass renders (usvg switch.rs convert); the element
		// itself acts as a group wrapper for the chosen child.
		return parseSwitchElement(pc, se)
	default:
		// Unknown SVG-namespace element: parsed as an inert container —
		// not rendered in place (usvg skips unknown tags), but content
		// stays referenceable by id (usvg use-resolution reaches into the
		// raw XML by id). Deliberate-subset refusals that must hide their
		// whole subtree are handled by their own cases above.
		return parseInertContainer(pc, se, true)
	}
}

// isSVGNameSpace reports whether the element belongs to the SVG namespace.
// No namespace at all is treated as SVG: hand-written documents commonly
// omit xmlns, and strictness there would reject everything.
func isSVGNameSpace(name xml.Name) bool {
	return name.Space == "" || name.Space == "http://www.w3.org/2000/svg"
}

// parseInertContainer parses an element whose node is not rendered in
// place but whose children remain in the tree: `use` can reference them
// by id, inheritance passes through (empty attrs are transparent), and
// the container's own attributes — beyond the id — are dropped (usvg
// never creates the node, so its attributes never enter the cascade).
func parseInertContainer(pc *parseCtx, se xml.StartElement, warn bool) (Element, error) {
	elem := &GroupElement{inert: true}
	for _, attr := range se.Attr {
		if attr.Name.Local == "id" {
			elem.Attrs.ID = attr.Value
		}
	}
	if warn {
		pc.warn(se.Name.Local, "", warnUnsupportedElement, "element not rendered (deliberate subset boundary)")
	}
	children, err := parseChildren(pc)
	if err != nil {
		return nil, err
	}
	elem.Children = children
	return elem, nil
}

// parseAttrs extracts common SVG presentation attributes as raw strings.
// Absent attributes stay "" (= inherit); resolution happens in the
// inheritance evaluator.
func parseAttrs(pc *parseCtx, se xml.StartElement) Attrs {
	a := Attrs{tag: se.Name.Local}
	for _, attr := range se.Attr {
		if a.Raw == nil {
			a.Raw = make(map[string]string, len(se.Attr))
		}
		a.Raw[attr.Name.Local] = attr.Value
		switch attr.Name.Local {
		case "id":
			a.ID = attr.Value
		case "class":
			a.Class = attr.Value
		case "style":
			a.Style = attr.Value
		case "fill":
			a.Fill = attr.Value
		case "fill-rule":
			a.FillRule = attr.Value
		case "fill-opacity":
			a.FillOpacity = attr.Value
		case "stroke":
			a.Stroke = attr.Value
		case "stroke-opacity":
			a.StrokeOpacity = attr.Value
		case "stroke-width":
			a.StrokeWidth = attr.Value
		case "stroke-linecap":
			a.StrokeCap = attr.Value
		case "stroke-linejoin":
			a.StrokeJoin = attr.Value
		case "stroke-miterlimit":
			a.StrokeMiterlimit = attr.Value
		case "stroke-dasharray":
			a.StrokeDasharray = attr.Value
		case "stroke-dashoffset":
			a.StrokeDashoffset = attr.Value
		case "opacity":
			a.Opacity = attr.Value
		case "transform":
			a.Transform = attr.Value
		case "transform-origin":
			a.TransformOrigin = attr.Value
		case "clip-rule":
			a.ClipRule = attr.Value
		case "color":
			a.Color = attr.Value
		case "font-size":
			a.FontSize = attr.Value
		case "visibility":
			a.Visibility = attr.Value
		case "overflow":
			a.Overflow = attr.Value
		case "clip-path":
			a.ClipPath = attr.Value
		case "mask":
			a.Mask = attr.Value
		case "font-family":
			a.FontFamily = attr.Value
		case "font-weight":
			a.FontWeight = attr.Value
		case "font-style":
			a.FontStyle = attr.Value
		case "text-anchor":
			a.TextAnchor = attr.Value
		case "letter-spacing":
			a.LetterSpacing = attr.Value
		case "filter":
			a.Filter = attr.Value
		}
	}
	return a
}

// parseMaskElement parses <mask> (region + content units; children).
// maskContentUnits defaults to userSpaceOnUse (the zero value is
// objectBoundingBox, so it is set explicitly up front).
func parseMaskElement(pc *parseCtx, se xml.StartElement) (*MaskElement, error) {
	elem := &MaskElement{Attrs: parseAttrs(pc, se), ContentUnits: UnitsUserSpaceOnUse}
	set := map[string]bool{}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "x":
			elem.X = mustLength(pc, "mask", "x", attr.Value)
			set["x"] = true
		case "y":
			elem.Y = mustLength(pc, "mask", "y", attr.Value)
			set["y"] = true
		case "width":
			elem.W = mustLength(pc, "mask", "width", attr.Value)
			set["width"] = true
		case "height":
			elem.H = mustLength(pc, "mask", "height", attr.Value)
			set["height"] = true
		case "maskUnits":
			if strings.TrimSpace(attr.Value) == "userSpaceOnUse" {
				elem.Units = UnitsUserSpaceOnUse
			} else {
				elem.Units = UnitsObjectBoundingBox
			}
		case "maskContentUnits":
			if strings.TrimSpace(attr.Value) == "objectBoundingBox" {
				elem.ContentUnits = UnitsObjectBoundingBox
			} else {
				elem.ContentUnits = UnitsUserSpaceOnUse
			}
		}
	}
	// Default region: -10%..120% of the referencing bbox (oBB fractions).
	if !set["x"] {
		elem.X = Length{Value: -10, Unit: LengthUnitPercent}
	}
	if !set["y"] {
		elem.Y = Length{Value: -10, Unit: LengthUnitPercent}
	}
	if !set["width"] {
		elem.W = Length{Value: 120, Unit: LengthUnitPercent}
	}
	if !set["height"] {
		elem.H = Length{Value: 120, Unit: LengthUnitPercent}
	}
	children, err := parseChildren(pc)
	if err != nil {
		return nil, err
	}
	elem.Children = children
	return elem, nil
}

// parseClipPathElement parses <clipPath> (clipPathUnits; shapes via children).
func parseClipPathElement(pc *parseCtx, se xml.StartElement) (*ClipPathElement, error) {
	// Spec default: clipPathUnits = userSpaceOnUse. The Units enum's zero
	// value is objectBoundingBox, so the default must be stated explicitly
	// (same trap as pattern's patternUnits, fixed in batch 2 — clipPath was
	// missed then and silently shrank every default-units clip to nothing).
	elem := &ClipPathElement{Attrs: parseAttrs(pc, se), Units: UnitsUserSpaceOnUse}
	for _, attr := range se.Attr {
		if attr.Name.Local == "clipPathUnits" {
			if strings.TrimSpace(attr.Value) == "objectBoundingBox" {
				elem.Units = UnitsObjectBoundingBox
			}
		}
	}
	children, err := parseChildren(pc)
	if err != nil {
		return nil, err
	}
	elem.Children = children
	return elem, nil
}

// parsePathElement parses a <path> element.
func parsePathElement(pc *parseCtx, se xml.StartElement) (*PathElement, error) {
	elem := &PathElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		if attr.Name.Local == "d" {
			elem.D = attr.Value
		}
	}
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <path>: %w", err)
	}
	return elem, nil
}

// parseCircleElement parses a <circle> element.
func parseCircleElement(pc *parseCtx, se xml.StartElement) (*CircleElement, error) {
	elem := &CircleElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		var dst *Length
		var name string
		switch attr.Name.Local {
		case "cx":
			dst, name = &elem.CX, "cx"
		case "cy":
			dst, name = &elem.CY, "cy"
		case "r":
			dst, name = &elem.R, "r"
		}
		if dst == nil {
			continue
		}
		*dst = mustLength(pc, "circle", name, attr.Value)
	}
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <circle>: %w", err)
	}
	return elem, nil
}

// parseRectElement parses a <rect> element.
func parseRectElement(pc *parseCtx, se xml.StartElement) (*RectElement, error) {
	elem := &RectElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		var dst *Length
		var name string
		switch attr.Name.Local {
		case "x":
			dst, name = &elem.X, "x"
		case "y":
			dst, name = &elem.Y, "y"
		case "width":
			dst, name = &elem.W, "width"
		case "height":
			dst, name = &elem.H, "height"
		case "rx":
			dst, name = &elem.RX, "rx"
		case "ry":
			dst, name = &elem.RY, "ry"
		}
		if dst == nil {
			continue
		}
		*dst = mustLength(pc, "rect", name, attr.Value)
	}
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <rect>: %w", err)
	}
	return elem, nil
}

// parseEllipseElement parses an <ellipse> element.
func parseEllipseElement(pc *parseCtx, se xml.StartElement) (*EllipseElement, error) {
	elem := &EllipseElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		var dst *Length
		var name string
		switch attr.Name.Local {
		case "cx":
			dst, name = &elem.CX, "cx"
		case "cy":
			dst, name = &elem.CY, "cy"
		case "rx":
			dst, name = &elem.RX, "rx"
		case "ry":
			dst, name = &elem.RY, "ry"
		}
		if dst == nil {
			continue
		}
		*dst = mustLength(pc, "ellipse", name, attr.Value)
	}
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <ellipse>: %w", err)
	}
	return elem, nil
}

// parseLineElement parses a <line> element.
func parseLineElement(pc *parseCtx, se xml.StartElement) (*LineElement, error) {
	elem := &LineElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		var dst *Length
		var name string
		switch attr.Name.Local {
		case "x1":
			dst, name = &elem.X1, "x1"
		case "y1":
			dst, name = &elem.Y1, "y1"
		case "x2":
			dst, name = &elem.X2, "x2"
		case "y2":
			dst, name = &elem.Y2, "y2"
		}
		if dst == nil {
			continue
		}
		*dst = mustLength(pc, "line", name, attr.Value)
	}
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <line>: %w", err)
	}
	return elem, nil
}

// parsePolygonElement parses a <polygon> element.
func parsePolygonElement(pc *parseCtx, se xml.StartElement) (*PolygonElement, error) {
	elem := &PolygonElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		if attr.Name.Local == "points" {
			elem.Points = parsePoints(pc, "polygon", attr.Value)
		}
	}
	// usvg points_to_path: 'polygon' must contain at least 2 points — fewer
	// skips the element (warning), the document continues. The XML stream is
	// consumed first: the closing tag must be read even when dropping it.
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <polygon>: %w", err)
	}
	if len(elem.Points) < 4 {
		pc.warn("polygon", "points", warnInvalidValue, "less than 2 points, element skipped")
		return nil, nil //nolint:nilnil
	}
	return elem, nil
}

// parsePolylineElement parses a <polyline> element.
func parsePolylineElement(pc *parseCtx, se xml.StartElement) (*PolylineElement, error) {
	elem := &PolylineElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		if attr.Name.Local == "points" {
			elem.Points = parsePoints(pc, "polyline", attr.Value)
		}
	}
	// usvg points_to_path: 'polyline' must contain at least 2 points.
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <polyline>: %w", err)
	}
	if len(elem.Points) < 4 {
		pc.warn("polyline", "points", warnInvalidValue, "less than 2 points, element skipped")
		return nil, nil //nolint:nilnil
	}
	return elem, nil
}

// parseGroupElement parses a <g> element and its children recursively.
func parseGroupElement(pc *parseCtx, se xml.StartElement) (*GroupElement, error) {
	elem := &GroupElement{Attrs: parseAttrs(pc, se)}
	children, err := parseChildren(pc)
	if err != nil {
		return nil, err
	}
	elem.Children = children
	return elem, nil
}

// parseDefsElement parses <defs>: content is collected for reference lookup
// and never rendered directly.
func parseDefsElement(pc *parseCtx, se xml.StartElement) (*DefsElement, error) {
	elem := &DefsElement{Attrs: parseAttrs(pc, se)}
	children, err := parseChildren(pc)
	if err != nil {
		return nil, err
	}
	elem.Children = children
	return elem, nil
}

// parseSymbolElement parses <symbol>: a use-instantiated template with its
// own viewBox/preserveAspectRatio viewport.
func parseSymbolElement(pc *parseCtx, se xml.StartElement) (*SymbolElement, error) {
	elem := &SymbolElement{Attrs: parseAttrs(pc, se)}
	if strings.TrimSpace(elem.Attrs.Overflow) == "visible" {
		elem.OverflowVisible = true
	}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "viewBox":
			vb, err := parseViewBox(attr.Value)
			if err != nil {
				pc.warn("symbol", "viewBox", warnInvalidValue, err.Error())
			} else {
				elem.ViewBox = vb
				elem.HasViewBox = true
			}
		case "preserveAspectRatio":
			elem.Par = parsePreserveAspectRatio(attr.Value)
		}
	}
	children, err := parseChildren(pc)
	if err != nil {
		return nil, err
	}
	elem.Children = children
	return elem, nil
}

// parseUseElement parses <use> with href (SVG2) / xlink:href (SVG 1.1).
func parseUseElement(pc *parseCtx, se xml.StartElement) (*UseElement, error) {
	elem := &UseElement{Attrs: parseAttrs(pc, se)}
	hasW, hasH := false, false
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "href":
			// Covers both plain href and xlink:href (same Local name).
			elem.HRef = attr.Value
		case "x":
			elem.X = mustLength(pc, "use", "x", attr.Value)
		case "y":
			elem.Y = mustLength(pc, "use", "y", attr.Value)
		case "width":
			elem.W = mustLength(pc, "use", "width", attr.Value)
			hasW = true
		case "height":
			elem.H = mustLength(pc, "use", "height", attr.Value)
			hasH = true
		}
	}
	// Symbol viewport defaults: 100% of the current viewport.
	if !hasW {
		elem.W = Length{Value: 100, Unit: LengthUnitPercent}
	}
	if !hasH {
		elem.H = Length{Value: 100, Unit: LengthUnitPercent}
	}

	elem.hasW, elem.hasH = hasW, hasH

	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <use>: %w", err)
	}
	return elem, nil
}

// parseSVGElement parses a nested <svg>: a new viewport.
func parseSVGElement(pc *parseCtx, se xml.StartElement) (*SVGElement, error) {
	elem := &SVGElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "x":
			elem.X = mustLength(pc, "svg", "x", attr.Value)
		case "y":
			elem.Y = mustLength(pc, "svg", "y", attr.Value)
		case "width":
			elem.W = mustLength(pc, "svg", "width", attr.Value)
			elem.hasW = true
		case "height":
			elem.H = mustLength(pc, "svg", "height", attr.Value)
			elem.hasH = true
		case "viewBox":
			vb, err := parseViewBox(attr.Value)
			if err != nil {
				pc.warn("svg", "viewBox", warnInvalidValue, err.Error())
			} else {
				elem.ViewBox = vb
				elem.HasViewBox = true
			}
		case "preserveAspectRatio":
			elem.Par = parsePreserveAspectRatio(attr.Value)
		}
	}
	if strings.TrimSpace(elem.Attrs.Overflow) == "visible" {
		elem.OverflowVisible = true
	}
	// Nested svg viewport defaults: 100% of the current viewport.
	if _, ok := attrValue(se, "width"); !ok {
		elem.W = Length{Value: 100, Unit: LengthUnitPercent}
	}
	if _, ok := attrValue(se, "height"); !ok {
		elem.H = Length{Value: 100, Unit: LengthUnitPercent}
	}
	children, err := parseChildren(pc)
	if err != nil {
		return nil, err
	}
	elem.Children = children
	return elem, nil
}

// parseGradientStops parses <stop> children (other content skipped).
func parseGradientStops(pc *parseCtx) ([]*GradientStopElement, error) {
	var stops []*GradientStopElement
	for {
		tok, err := pc.dec.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: error reading gradient children: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "stop" {
				st, err := parseStopElement(pc, t)
				if err != nil {
					return nil, err
				}
				if st != nil {
					stops = append(stops, st)
				}
				continue
			}
			if err := pc.dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			return stops, nil
		}
	}
}

// parseStopElement parses <stop>: offset (number or %), stop-color,
// stop-opacity (raw strings stay on Attrs for inheritance debugging).
func parseStopElement(pc *parseCtx, se xml.StartElement) (*GradientStopElement, error) {
	elem := &GradientStopElement{Attrs: parseAttrs(pc, se), Opacity: 1, OpacitySet: true}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "offset":
			// Raw storage: the prev-offset fallback and clamping happen at
			// normalization (usvg convert_stops walks the stop sequence).
			elem.OffsetSet = true
			elem.OffsetRaw = strings.TrimSpace(attr.Value)
		case "stop-color":
			elem.Color = attr.Value
		case "stop-opacity":
			elem.OpacityRaw = strings.TrimSpace(attr.Value)
			elem.OpacitySet = true
		}
	}
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <stop>: %w", err)
	}
	return elem, nil
}

// parseLinearGradient parses <linearGradient> with full geometry/units/
// transform/spread/href semantics (stops via children).
func parseLinearGradient(pc *parseCtx, se xml.StartElement) (*LinearGradientElement, error) {
	g := &LinearGradientElement{Attrs: parseAttrs(pc, se)}
	set := map[string]bool{}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "x1":
			g.X1 = mustLength(pc, "linearGradient", "x1", attr.Value)
			set["x1"] = true
			g.setX1 = true
		case "y1":
			g.Y1 = mustLength(pc, "linearGradient", "y1", attr.Value)
			set["y1"] = true
			g.setY1 = true
		case "x2":
			g.X2 = mustLength(pc, "linearGradient", "x2", attr.Value)
			set["x2"] = true
			g.setX2 = true
		case "y2":
			g.Y2 = mustLength(pc, "linearGradient", "y2", attr.Value)
			set["y2"] = true
			g.setY2 = true
		case "gradientUnits":
			if strings.TrimSpace(attr.Value) == "userSpaceOnUse" {
				g.Units = UnitsUserSpaceOnUse
			} else {
				g.Units = UnitsObjectBoundingBox
			}
			g.unitsSet = true
		case "gradientTransform":
			g.Transform = attr.Value
		case "spreadMethod":
			g.Spread = spreadToken(attr.Value)
			g.spreadSet = true
		case "href":
			g.HRef = attr.Value
		}
	}
	if !set["x1"] {
		g.X1 = Length{}
	}
	if !set["y1"] {
		g.Y1 = Length{}
	}
	if !set["x2"] {
		// Spec default: 100% — a percent, so userSpaceOnUse resolves it
		// against the viewport width (usvg Length::new(100.0, Percent)).
		g.X2 = Length{Value: 100, Unit: LengthUnitPercent}
	}
	if !set["y2"] {
		g.Y2 = Length{}
	}
	stops, err := parseGradientStops(pc)
	if err != nil {
		return nil, err
	}
	g.Stops = stops
	return g, nil
}

// parseRadialGradient parses <radialGradient> (cx/cy/r/fx/fy; fx/fy default
// to cx/cy per axis when either is present).
func parseRadialGradient(pc *parseCtx, se xml.StartElement) (*RadialGradientElement, error) {
	// Spec defaults cx=cy=r=50% stated on the Lengths (NOT on the set flags —
	// href inheritance keys off the flags, so an unset cx still inherits from
	// the referenced gradient; the 50% only survives when no link sets it).
	// The zero-value Length previously gave r=0 → the gradient painted its
	// last stop everywhere (every default radial in the resvg corpus was
	// solid white).
	g := &RadialGradientElement{
		Attrs: parseAttrs(pc, se),
		CX:    Length{Value: 50, Unit: LengthUnitPercent},
		CY:    Length{Value: 50, Unit: LengthUnitPercent},
		R:     Length{Value: 50, Unit: LengthUnitPercent},
	}
	set := map[string]bool{}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "cx":
			g.CX = mustLength(pc, "radialGradient", "cx", attr.Value)
			set["cx"] = true
			g.setCX = true
		case "cy":
			g.CY = mustLength(pc, "radialGradient", "cy", attr.Value)
			set["cy"] = true
			g.setCY = true
		case "r":
			g.R = mustLength(pc, "radialGradient", "r", attr.Value)
			set["r"] = true
			g.setR = true
		case "fx":
			g.FX = mustLength(pc, "radialGradient", "fx", attr.Value)
			set["fx"] = true
			g.setFX = true
		case "fy":
			g.FY = mustLength(pc, "radialGradient", "fy", attr.Value)
			set["fy"] = true
			g.setFY = true
		case "gradientUnits":
			if strings.TrimSpace(attr.Value) == "userSpaceOnUse" {
				g.Units = UnitsUserSpaceOnUse
			} else {
				g.Units = UnitsObjectBoundingBox
			}
			g.unitsSet = true
		case "gradientTransform":
			g.Transform = attr.Value
		case "spreadMethod":
			g.Spread = spreadToken(attr.Value)
			g.spreadSet = true
		case "href":
			g.HRef = attr.Value
		}
	}
	if !set["cx"] {
		g.CX = Length{Value: 50, Unit: LengthUnitPercent}
	}
	if !set["cy"] {
		g.CY = Length{Value: 50, Unit: LengthUnitPercent}
	}
	if !set["r"] {
		g.R = Length{Value: 50, Unit: LengthUnitPercent}
	}
	if set["fx"] || set["fy"] {
		g.HasFocus = true
		if !set["fx"] {
			g.FX = g.CX
		}
		if !set["fy"] {
			g.FY = g.CY
		}
	}
	stops, err := parseGradientStops(pc)
	if err != nil {
		return nil, err
	}
	g.Stops = stops
	return g, nil
}

// attrValue returns the value of an attribute by local name, if present.
func attrValue(se xml.StartElement, local string) (string, bool) {
	for _, attr := range se.Attr {
		if attr.Name.Local == local {
			return attr.Value, true
		}
	}
	return "", false
}

// parseStyleElement parses <style>: rules are registered on the Document.
func parseStyleElement(pc *parseCtx, se xml.StartElement) (*StyleElement, error) {
	elem := &StyleElement{Attrs: parseAttrs(pc, se)}
	var text strings.Builder
	depth := 1 // we consumed the start element already
	for depth > 0 {
		tok, err := pc.dec.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: error reading <style>: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++ // CDATA-wrapped markup inside style: keep scanning
		case xml.EndElement:
			depth--
		case xml.CharData:
			text.Write(t)
		}
	}
	pc.doc.Styles = append(pc.doc.Styles, parseStyleContent(pc, text.String(), styleType(se)))
	return elem, nil
}

// styleType reads the <style type> attribute: an explicit non-CSS type
// makes the whole stylesheet skipped (usvg resolve_css: Some("text/css")
// parses, Some(_) skips, None parses).
func styleType(se xml.StartElement) string {
	for _, attr := range se.Attr {
		if attr.Name.Local == "type" {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

// parseImageElement parses <image>: href (data: URI or external path via
// ParseConfig.ResourcesDir — usvg parser/image.rs semantics), geometry
// with SVG2 auto/intrinsic defaults, and preserveAspectRatio. Decoding
// and sub-SVG parsing run at parse.
func parseImageElement(pc *parseCtx, se xml.StartElement) (*ImageElement, error) {
	elem := &ImageElement{Attrs: parseAttrs(pc, se)}
	hasW, hasH := false, false
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "x":
			elem.X = mustLength(pc, "image", "x", attr.Value)
		case "y":
			elem.Y = mustLength(pc, "image", "y", attr.Value)
		case "width":
			if l, ok := imageLength(pc, "width", attr.Value); ok {
				elem.W = l
				hasW = true
			}
		case "height":
			if l, ok := imageLength(pc, "height", attr.Value); ok {
				elem.H = l
				hasH = true
			}
		case "href":
			elem.HRef = attr.Value
		case "preserveAspectRatio":
			elem.Par = parsePreserveAspectRatio(attr.Value)
			elem.HasPar = true
		}
	}
	// Resolve the reference first: the intrinsic size drives the geometry
	// defaults (usvg convert: width/height default to the actual size).
	elem.raw, elem.rawMime, elem.SubDoc = func() ([]byte, string, *Document) {
		raw, mime, doc, ok := resolveImageRef(pc, elem.HRef)
		if !ok {
			return nil, "", nil
		}
		return raw, mime, doc
	}()
	if doc := elem.SubDoc; doc != nil {
		elem.HasImage = true
		sw, sh := subDocSize(doc)
		elem.ImgW, elem.ImgH = int(sw), int(sh)
	} else if elem.raw != nil {
		elem.HasImage = true
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(elem.raw)); err == nil {
			elem.ImgW, elem.ImgH = cfg.Width, cfg.Height
		}
	}
	// Geometry defaults (usvg convert_user_length + the one-sided aspect
	// rule): missing width/height default to the intrinsic size; one side
	// present scales the other by the intrinsic aspect. "auto" parsed as
	// absent above (usvg: unparseable Length → the intrinsic default).
	if elem.HasImage && elem.ImgW > 0 && elem.ImgH > 0 {
		iw, ih := float64(elem.ImgW), float64(elem.ImgH)
		switch {
		case !hasW && !hasH:
			elem.W = Length{Value: iw}
			elem.H = Length{Value: ih}
		case hasW && !hasH:
			w := resolveViewBoxLength(pc, elem.W, LengthAxisX)
			elem.H = Length{Value: ih * (w / iw)}
		case !hasW && hasH:
			h := resolveViewBoxLength(pc, elem.H, LengthAxisY)
			elem.W = Length{Value: iw * (h / ih)}
		}
	}
	// <image> is a leaf: consume up to the matching end (self-closing emits
	// its EndElement immediately — without this the enclosing parseChildren
	// would treat it as the parent's close and drop every later sibling).
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <image>: %w", err)
	}
	return elem, nil
}

// imageLength parses an image width/height: "auto" (SVG2) or "" mean
// absent (usvg: unparseable Length → the intrinsic default, no warning).
func imageLength(pc *parseCtx, attr, v string) (Length, bool) {
	v = strings.TrimSpace(v)
	if v == "" || v == "auto" {
		return Length{}, false
	}
	l, err := parseLength(v)
	if err != nil {
		pc.warn("image", attr, warnInvalidValue, err.Error())
		return Length{}, false
	}
	return l, true
}

// resolveViewBoxLength resolves one length against the document viewBox
// (usvg convert_user_length: percentages resolve against state.view_box).
func resolveViewBoxLength(pc *parseCtx, l Length, axis LengthAxis) float64 {
	return l.Resolve(RefSpace{ViewportW: pc.doc.ViewBox.Width, ViewportH: pc.doc.ViewBox.Height}, axis)
}

func parsePatternElement(pc *parseCtx, se xml.StartElement) (*PatternElement, error) {
	elem := &PatternElement{
		Attrs: parseAttrs(pc, se),
		// Spec defaults (usvg convert_units): patternUnits =
		// objectBoundingBox, patternContentUnits = userSpaceOnUse — the
		// two defaults differ; zero-value handling elsewhere relies on
		// these being explicit.
		Units:        UnitsObjectBoundingBox,
		ContentUnits: UnitsUserSpaceOnUse,
	}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "x":
			elem.X = mustLength(pc, "pattern", "x", attr.Value)
			elem.setX = true
		case "y":
			elem.Y = mustLength(pc, "pattern", "y", attr.Value)
			elem.setY = true
		case "width":
			elem.W = mustLength(pc, "pattern", "width", attr.Value)
			elem.setW = true
		case "height":
			elem.H = mustLength(pc, "pattern", "height", attr.Value)
			elem.setH = true
		case "patternUnits":
			// Invalid values fall back to the spec default oBB (usvg
			// convert_units: unparseable attribute → default), not to
			// userSpaceOnUse.
			if strings.TrimSpace(attr.Value) == "userSpaceOnUse" {
				elem.Units = UnitsUserSpaceOnUse
			} else {
				elem.Units = UnitsObjectBoundingBox
			}
			elem.unitsSet = true
		case "patternContentUnits":
			// NOTE: the content-units default is userSpaceOnUse (usvg
			// convert_units def), unlike patternUnits' oBB default; invalid
			// values fall back to that default.
			if strings.TrimSpace(attr.Value) == "objectBoundingBox" {
				elem.ContentUnits = UnitsObjectBoundingBox
			} else {
				elem.ContentUnits = UnitsUserSpaceOnUse
			}
			elem.contentUnitsSet = true
		case "patternTransform":
			elem.Transform = attr.Value
		case "viewBox":
			vb, err := parseViewBox(attr.Value)
			if err != nil {
				pc.warn("pattern", "viewBox", warnInvalidValue, err.Error())
			} else {
				elem.ViewBox = vb
				elem.HasViewBox = true
			}
		case "preserveAspectRatio":
			elem.Par = parsePreserveAspectRatio(attr.Value)
		case "href":
			elem.HRef = attr.Value
		}
	}
	children, err := parseChildren(pc)
	if err != nil {
		return nil, err
	}
	elem.Children = children
	return elem, nil
}

// parseTextElement parses <text> with mixed content: direct character data
// plus <tspan> sub-runs (other elements skipped).
func parseTextElement(pc *parseCtx, se xml.StartElement) (Element, error) {
	return parseTextLike(pc, se, "text")
}

func parseTspanElement(pc *parseCtx, se xml.StartElement) (Element, error) {
	return parseTextLike(pc, se, "tspan")
}

func parseTextLike(pc *parseCtx, se xml.StartElement, tag string) (Element, error) {
	var textElem, spanElem Element
	if tag == "text" {
		textElem = &TextElement{Attrs: parseAttrs(pc, se)}
	} else {
		spanElem = &TspanElement{Attrs: parseAttrs(pc, se)}
	}
	// Pointer views for uniform handling.
	var xOut, yOut, dxOut, dyOut *[]Length
	var rotOut *[]float64
	var textOut *string
	var childrenOut *[]Element
	if te, ok := textElem.(*TextElement); ok {
		xOut, yOut, dxOut, dyOut, rotOut, textOut, childrenOut =
			&te.X, &te.Y, &te.DX, &te.DY, &te.Rotate, &te.Text, &te.Children
	} else {
		tse := spanElem.(*TspanElement)
		xOut, yOut, dxOut, dyOut, rotOut, textOut, childrenOut =
			&tse.X, &tse.Y, &tse.DX, &tse.DY, &tse.Rotate, &tse.Text, &tse.Children
	}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "x":
			*xOut = parseLengthList(attr.Value)
		case "y":
			*yOut = parseLengthList(attr.Value)
		case "dx":
			*dxOut = parseLengthList(attr.Value)
		case "dy":
			*dyOut = parseLengthList(attr.Value)
		case "rotate":
			*rotOut = parseNumberList(attr.Value)
		}
	}
	// Mixed content: CharData → text runs; <tspan> → children; else skip.
	depth := 1
	for depth > 0 {
		tok, err := pc.dec.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: error reading <%s>: %w", tag, err)
		}
		switch t := tok.(type) {
		case xml.CharData:
			*textOut += string(t)
		case xml.StartElement:
			if t.Name.Local == "tspan" {
				sub, err := parseTspanElement(pc, t)
				if err != nil {
					return nil, err
				}
				if sub != nil {
					*childrenOut = append(*childrenOut, sub)
				}
				continue
			}
			if t.Name.Local == "textPath" {
				sub, err := parseTextPathElement(pc, t)
				if err != nil {
					return nil, err
				}
				if sub != nil {
					*childrenOut = append(*childrenOut, sub)
				}
				continue
			}
			if err := pc.dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			depth--
		}
	}
	if tag == "text" {
		return textElem, nil
	}
	return spanElem, nil
}

// parseTextPathElement parses <textPath> (P3.8 4b-4): href + startOffset
// attributes, mixed text/tspan/nested-textPath content. The SVG2 `path`
// attribute and side/method/spacing are parsed-and-ignored like resvg
// v0.42 (a missing/invalid href falls back to the normal text flow).
func parseTextPathElement(pc *parseCtx, se xml.StartElement) (*TextPathElement, error) {
	elem := &TextPathElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "href", "xlink:href":
			// First one wins (resvg); an href without the "#id" form stays
			// unresolved → normal flow at render.
			if elem.Href == "" {
				elem.Href = attr.Value
			}
		case "startOffset":
			elem.StartOffset = attr.Value
		}
	}
	depth := 1
	for depth > 0 {
		tok, err := pc.dec.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: error reading <textPath>: %w", err)
		}
		switch t := tok.(type) {
		case xml.CharData:
			elem.Text += string(t)
		case xml.StartElement:
			if t.Name.Local == "tspan" {
				sub, err := parseTspanElement(pc, t)
				if err != nil {
					return nil, err
				}
				if sub != nil {
					elem.Children = append(elem.Children, sub)
				}
				continue
			}
			if t.Name.Local == "textPath" {
				sub, err := parseTextPathElement(pc, t)
				if err != nil {
					return nil, err
				}
				if sub != nil {
					elem.Children = append(elem.Children, sub)
				}
				continue
			}
			if err := pc.dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			depth--
		}
	}
	return elem, nil
}

// parseLengthList parses a whitespace/comma separated x/y/dx/dy list.
func parseLengthList(s string) []Length {
	s = strings.ReplaceAll(s, ",", " ")
	fields := strings.Fields(s)
	out := make([]Length, 0, len(fields))
	for _, f := range fields {
		l, err := parseLength(f)
		if err != nil {
			continue
		}
		out = append(out, l)
	}
	return out
}

// parseNumberList parses a whitespace/comma separated number list.
func parseNumberList(s string) []float64 {
	s = strings.ReplaceAll(s, ",", " ")
	fields := strings.Fields(s)
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

// xmlStartTag is a minimal alias used by parseFilterElement's signature.
type xmlStartTag = xml.StartElement

// tName extracts the local name of a start element.
func tName(se xml.StartElement) string { return se.Name.Local }

// parseFilterElement parses <filter> (region + units; primitives via children).
func parseFilterElement(pc *parseCtx, se xml.StartElement) (Element, error) {
	// Defaults stated explicitly: the enum zero value is objectBoundingBox
	// but the spec defaults are filterUnits=oBB / primitiveUnits=userSpaceOnUse;
	// the region defaults to -10%..120% of the bbox (resvg usvg filter.rs
	// resolve_number fallbacks — a zero-value Length here silently produced
	// a zero-area region and skipped every filter without explicit x/y/w/h).
	elem := &FilterElement{
		Attrs:        parseAttrs(pc, se),
		Units:        UnitsObjectBoundingBox,
		ContentUnits: UnitsUserSpaceOnUse,
		// Spec default: linearRGB (resvg usvg ColorInterpolation::default).
		LinearRGB: true,
		X:         Length{Value: -10, Unit: LengthUnitPercent},
		Y:         Length{Value: -10, Unit: LengthUnitPercent},
		W:         Length{Value: 120, Unit: LengthUnitPercent},
		H:         Length{Value: 120, Unit: LengthUnitPercent},
	}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "x":
			elem.X = mustLength(pc, "filter", "x", attr.Value)
		case "y":
			elem.Y = mustLength(pc, "filter", "y", attr.Value)
		case "width":
			elem.W = mustLength(pc, "filter", "width", attr.Value)
		case "height":
			elem.H = mustLength(pc, "filter", "height", attr.Value)
		case "filterUnits":
			if strings.TrimSpace(attr.Value) == "userSpaceOnUse" {
				elem.Units = UnitsUserSpaceOnUse
			} else {
				elem.Units = UnitsObjectBoundingBox
			}
		case "color-interpolation-filters":
			// Only the explicit "sRGB" leaves the linearRGB default.
			elem.LinearRGB = strings.TrimSpace(attr.Value) != "sRGB"
		case "flood-color", "flood-opacity":
			// Kept raw: a feFlood child's flood-color="inherit" reads the
			// <filter> element's own value (non-inheritable one-level
			// inheritance, usvg resolve_inherit).
			if attr.Name.Local == "flood-color" {
				elem.FloodColor = attr.Value
			} else {
				elem.FloodOpacity = attr.Value
			}
		case "primitiveUnits":
			// Spec default userSpaceOnUse; objectBoundingBox scales the
			// primitives' numeric params and subregions by the element bbox
			// (4b-1 resolves subregions with it; numeric params stay a
			// registered gap).
			if strings.TrimSpace(attr.Value) == "objectBoundingBox" {
				elem.ContentUnits = UnitsObjectBoundingBox
			} else {
				elem.ContentUnits = UnitsUserSpaceOnUse
			}
		}
	}
	prims, err := parseFilterPrimitives(pc)
	if err != nil {
		return nil, err
	}
	elem.Primitives = prims
	return elem, nil
}

// parseFilterPrimitives parses fe* children until the enclosing end.
func parseFilterPrimitives(pc *parseCtx) ([]*FilterPrimitive, error) {
	var out []*FilterPrimitive
	depth := 1 // the <filter> start element is already consumed
	for depth > 0 {
		tok, err := pc.dec.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: error reading <filter>: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			kind := t.Name.Local
			if strings.HasPrefix(kind, "fe") {
				fp, err := parseFilterPrimitive(pc, t, kind)
				if err != nil {
					return nil, err
				}
				if fp != nil {
					out = append(out, fp)
				}
				continue
			}
			if err := pc.dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			depth--
		}
	}
	return out, nil
}

// parseFilterPrimitive parses one fe* primitive (generic attribute set).
func parseFilterPrimitive(pc *parseCtx, se xml.StartElement, kind string) (*FilterPrimitive, error) {
	fp := &FilterPrimitive{Kind: kind, FloodOpacity: 1}
	switch kind {
	case "feGaussianBlur":
		for _, attr := range se.Attr {
			if attr.Name.Local == "stdDeviation" {
				fp.Numbers = parseNumberList(attr.Value)
			}
		}
	case "feOffset", "feDropShadow":
		// Name-keyed read + early return: this primitive must NOT fall
		// through to the generic numeric tail, which would append dx/dy a
		// second time and shift the slots (批 4d: dx="14" dy="0" rendered
		// as (0,14) — the offsets swapped). Canonical layout shared with
		// offsetDeltas/dropShadowParams: Number = dx, Numbers[0] = dy.
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "in":
				fp.In = attr.Value
			case "result":
				fp.Result = attr.Value
			case "x":
				if v, err := parseLength(attr.Value); err == nil {
					fp.SX, fp.SXSet = v, true
				}
			case "y":
				if v, err := parseLength(attr.Value); err == nil {
					fp.SY, fp.SYSet = v, true
				}
			case "width":
				if v, err := parseLength(attr.Value); err == nil {
					fp.SW, fp.SWSet = v, true
				}
			case "height":
				if v, err := parseLength(attr.Value); err == nil {
					fp.SH, fp.SHSet = v, true
				}
			case "dx":
				if v, err := parseLength(attr.Value); err == nil {
					fp.Number = v.Value
					fp.HasNumber = true
				}
			case "dy":
				if v, err := parseLength(attr.Value); err == nil {
					fp.Numbers = append(fp.Numbers, v.Value)
				}
			case "stdDeviation":
				fp.Numbers2 = parseNumberList(attr.Value)
			}
		}
		if err := pc.dec.Skip(); err != nil {
			return nil, fmt.Errorf("svg: error reading <%s>: %w", kind, err)
		}
		return fp, nil
	case "feColorMatrix":
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "type":
				fp.Str1 = attr.Value
			case "values":
				fp.Numbers2 = parseNumberList(attr.Value)
			}
		}
	case "feFlood":
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "flood-color":
				fp.FloodColor = attr.Value
			case "flood-opacity":
				if v, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
					fp.FloodOpacity = clamp01(v)
				}
			}
		}
	case "feBlend", "feComposite":
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "mode":
				fp.Str1 = attr.Value
			case "operator":
				fp.Str1 = attr.Value
			case "k1":
				fp.K1 = numF(attr.Value)
			case "k2":
				fp.K2 = numF(attr.Value)
			case "k3":
				fp.K3 = numF(attr.Value)
			case "k4":
				fp.K4 = numF(attr.Value)
			}
		}
	case "feDiffuseLighting", "feSpecularLighting":
		fp.SurfaceScale, fp.DiffuseConstant, fp.SpecularConstant, fp.SpecularExponent = 1, 1, 1, 1
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "surfaceScale":
				fp.SurfaceScale = numF(attr.Value)
			case "diffuseConstant":
				fp.DiffuseConstant = numF(attr.Value)
			case "specularConstant":
				fp.SpecularConstant = numF(attr.Value)
			case "specularExponent":
				fp.SpecularExponent = numF(attr.Value)
			case "lighting-color":
				fp.LightingColor = attr.Value
			}
		}
		// The light source is a child element (first one wins — resvg usvg).
		depth := 1
		for depth > 0 {
			tok, err := pc.dec.Token()
			if err != nil {
				return nil, fmt.Errorf("svg: error reading <%s>: %w", kind, err)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if ls := parseLightSource(t); ls != nil && fp.Light == nil {
					fp.Light = ls
				}
				if err := pc.dec.Skip(); err != nil {
					return nil, fmt.Errorf("svg: error reading <%s>: %w", kind, err)
				}
			case xml.EndElement:
				depth--
			}
		}
		return fp, nil
	case "feMorphology":
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "operator":
				fp.Operator = strings.ToLower(strings.TrimSpace(attr.Value))
			case "radius":
				fp.Radius = parseNumberList(attr.Value)
			}
		}
	case "feTurbulence":
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "type":
				fp.Str1 = attr.Value
			case "baseFrequency":
				fp.BaseFrequency = parseNumberList(attr.Value)
			case "numOctaves":
				if v, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
					fp.NumOctaves, fp.HasNumOctaves = v, true
				}
			case "seed":
				if v, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
					fp.Seed, fp.HasSeed = v, true
				}
			case "stitchTiles":
				fp.StitchTiles = strings.TrimSpace(attr.Value) == "stitch"
			}
		}
	case "feConvolveMatrix":
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "order":
				fp.Order = parseNumberList(attr.Value)
			case "kernelMatrix":
				fp.KernelMatrix = parseNumberList(attr.Value)
			case "divisor":
				if v, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
					fp.Divisor, fp.HasDivisor = v, true
				}
			case "bias":
				if v, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
					fp.Bias, fp.HasBias = v, true
				}
			case "targetX":
				if v, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
					fp.TargetX, fp.HasTargetX = int(v), true
				}
			case "targetY":
				if v, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
					fp.TargetY, fp.HasTargetY = int(v), true
				}
			case "edgeMode":
				fp.EdgeMode = attr.Value
			case "preserveAlpha":
				fp.PreserveAlpha = strings.TrimSpace(attr.Value) == "true"
			}
		}
	case "feDisplacementMap":
		fp.XSel, fp.YSel = 3, 3 // spec defaults: A/A
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "xChannelSelector":
				fp.XSel = colorChannelIndex(attr.Value)
			case "yChannelSelector":
				fp.YSel = colorChannelIndex(attr.Value)
			case "scale":
				if v, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
					fp.Scale, fp.HasScale = v, true
				}
			}
		}
	case "feImage":
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "href":
				fp.Href = attr.Value
			case "preserveAspectRatio":
				fp.Aspect = attr.Value
			}
		}
		// External/data-URI hrefs decode now (element refs "#id" resolve at
		// render); missing/invalid href → empty result at render (resvg
		// dummy primitive stance).
		if ref := strings.TrimSpace(fp.Href); ref != "" && !strings.HasPrefix(ref, "#") {
			img := &ImageElement{HRef: ref}
			if fp.Aspect != "" {
				img.Par = parsePreserveAspectRatio(fp.Aspect)
				img.HasPar = true
			}
			decodeImageInto(pc, img)
			fp.Img = img
		}
	case "feComponentTransfer":
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "in":
				fp.In = attr.Value
			case "result":
				fp.Result = attr.Value
			}
		}
		depth := 1
		for depth > 0 {
			tok, err := pc.dec.Token()
			if err != nil {
				return nil, fmt.Errorf("svg: error reading <feComponentTransfer>: %w", err)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if ch := transferChannelIndex(t.Name.Local); ch >= 0 {
					fp.Funcs[ch] = parseTransferFunc(t)
				}
				if err := pc.dec.Skip(); err != nil {
					return nil, fmt.Errorf("svg: error reading <feComponentTransfer>: %w", err)
				}
			case xml.EndElement:
				depth--
			}
		}
		return fp, nil
	case "feMerge":
		for _, attr := range se.Attr {
			if attr.Name.Local == "result" {
				fp.Result = attr.Value
			}
		}
		depth := 1
		for depth > 0 {
			tok, err := pc.dec.Token()
			if err != nil {
				return nil, fmt.Errorf("svg: error reading <feMerge>: %w", err)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "feMergeNode" {
					// A feMergeNode without `in` means the previous
					// primitive's result (SVG spec / resvg resolve_input);
					// keep the "" sentinel — the renderer resolves it with
					// the same default rule as a primitive's missing in.
					// 批 4d: the implicit first node used to be dropped, so
					// <feMergeNode/> contributed nothing (glowChain 泛光分支
					// 整体消失).
					in := ""
					for _, attr := range t.Attr {
						if attr.Name.Local == "in" {
							in = attr.Value
						}
					}
					fp.MergeNodes = append(fp.MergeNodes, in)
				}
				if err := pc.dec.Skip(); err != nil {
					return nil, err
				}
			case xml.EndElement:
				depth--
			}
		}
		return fp, nil
	}
	// Common in/in2/result + subregion + generic numeric catch-alls.
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "in":
			fp.In = attr.Value
		case "in2":
			fp.In2 = attr.Value
		case "result":
			fp.Result = attr.Value
		case "x":
			if v, err := parseLength(attr.Value); err == nil {
				fp.SX, fp.SXSet = v, true
			}
		case "y":
			if v, err := parseLength(attr.Value); err == nil {
				fp.SY, fp.SYSet = v, true
			}
		case "width":
			if v, err := parseLength(attr.Value); err == nil {
				fp.SW, fp.SWSet = v, true
			}
		case "height":
			if v, err := parseLength(attr.Value); err == nil {
				fp.SH, fp.SHSet = v, true
			}
		case "dx", "dy":
			if v, err := parseLength(attr.Value); err == nil {
				fp.Numbers = append(fp.Numbers, v.Value)
			}
		}
	}
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <%s>: %w", kind, err)
	}
	return fp, nil
}

// colorChannelIndex maps an SVG color-channel selector word onto 0=R..3=A
// (unknown → A, the spec default).
func colorChannelIndex(s string) int {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "R":
		return 0
	case "G":
		return 1
	case "B":
		return 2
	default:
		return 3
	}
}

// transferChannelIndex maps feFuncR/G/B/A onto the channel slot (-1: other).
func transferChannelIndex(local string) int {
	switch local {
	case "feFuncR":
		return 0
	case "feFuncG":
		return 1
	case "feFuncB":
		return 2
	case "feFuncA":
		return 3
	default:
		return -1
	}
}

// parseTransferFunc parses one feFunc* element (SVG Filter Effects §10).
// Defaults follow the spec: slope/amplitude/exponent 1, the rest 0; an
// unknown type degrades to identity.
func parseTransferFunc(se xml.StartElement) *TransferFunc {
	fn := &TransferFunc{Type: "identity", Slope: 1, Amplitude: 1, Exponent: 1}
	num := func(v string) float64 {
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0
		}
		return f
	}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "type":
			fn.Type = strings.ToLower(strings.TrimSpace(attr.Value))
		case "tableValues":
			fn.TableValues = parseNumberList(attr.Value)
		case "slope":
			fn.Slope = num(attr.Value)
		case "intercept":
			fn.Intercept = num(attr.Value)
		case "amplitude":
			fn.Amplitude = num(attr.Value)
		case "exponent":
			fn.Exponent = num(attr.Value)
		case "offset":
			fn.Offset = num(attr.Value)
		}
	}
	switch fn.Type {
	case "identity", "table", "discrete", "linear", "gamma":
	default:
		fn.Type = "identity"
	}
	return fn
}

// numF parses one float number ("0"/"-1"/"2.5"); unparsable → 0.
func numF(v string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0
	}
	return f
}

// parseLightSource parses one feDistantLight/fePointLight/feSpotLight child
// (nil for other elements). Defaults per SVG Filter Effects §10.
func parseLightSource(se xml.StartElement) *LightSource {
	ls := &LightSource{}
	num := numF
	switch se.Name.Local {
	case "feDistantLight":
		ls.Type = 0
	case "fePointLight":
		ls.Type = 1
	case "feSpotLight":
		ls.Type = 2
		ls.SpotSpecularExponent = 1
	default:
		return nil
	}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "azimuth":
			ls.Azimuth = num(attr.Value)
		case "elevation":
			ls.Elevation = num(attr.Value)
		case "x":
			ls.X = num(attr.Value)
		case "y":
			ls.Y = num(attr.Value)
		case "z":
			ls.Z = num(attr.Value)
		case "pointsAtX":
			ls.PointsAtX = num(attr.Value)
		case "pointsAtY":
			ls.PointsAtY = num(attr.Value)
		case "pointsAtZ":
			ls.PointsAtZ = num(attr.Value)
		case "specularExponent":
			ls.SpotSpecularExponent = num(attr.Value)
		case "limitingConeAngle":
			ls.LimitingConeAngle = num(attr.Value)
			ls.HasLimitingCone = true
		}
	}
	return ls
}

// parsePoints parses an SVG points attribute value into alternating x/y
// lengths ("x1,y1 x2,y2" or "x1 y1 x2 y2") with the usvg PointsParser
// tolerance (P3.8 4b-4): pairs are collected until the first token pair that
// fails to parse — that token and everything after it is ignored
// (stop-processing-on-invalid-data); a trailing odd coordinate without a
// partner is dropped (ignore-odd-points). A malformed attribute therefore
// degrades to a point prefix instead of failing the whole document; the
// element-level "<2 points → skip" rule lives at the call sites.
func parsePoints(pc *parseCtx, elem, s string) []Length {
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	var points []Length
	for i := 0; i+1 < len(parts); i += 2 {
		x, errx := parseLength(parts[i])
		y, erry := parseLength(parts[i+1])
		if errx != nil || erry != nil {
			break
		}
		points = append(points, x, y)
	}
	return points
}
