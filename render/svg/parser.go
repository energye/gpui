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
	"strconv"
	"strings"
)

// parseCtx threads parse state (document under construction + warnings).
type parseCtx struct {
	doc *Document
	dec *xml.Decoder
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
	decoder := xml.NewDecoder(bytes.NewReader(data))

	doc := &Document{
		ViewBox: ViewBox{Width: 16, Height: 16}, // legacy default when no viewBox
	}
	pc := &parseCtx{doc: doc, dec: decoder}

	// Find the root <svg> element.
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: failed to find <svg> root element: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "svg" {
			if err := parseSVGRoot(pc, se); err != nil {
				return nil, err
			}
			// Parse children.
			children, err := parseChildren(pc)
			if err != nil {
				return nil, err
			}
			doc.Elements = children
			validateReferences(pc)
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
		case "height":
			v, err := parseDimension(attr.Value, doc.ViewBox.Height)
			if err != nil {
				return fmt.Errorf("svg: <svg> height: %w", err)
			}
			doc.Height = v
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
// collected-only content already registered elsewhere).
func parseElement(pc *parseCtx, se xml.StartElement) (Element, error) {
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
	case "title", "desc", "metadata":
		// Descriptive: no rendering, no warning.
		if err := pc.dec.Skip(); err != nil {
			return nil, fmt.Errorf("svg: error skipping <%s>: %w", se.Name.Local, err)
		}
		return nil, nil //nolint:nilnil // descriptive element
	default:
		// Deliberately unsupported (text/filter/gradient until their batches,
		// foreignObject, animation...): named refusal, rendered as nothing.
		if err := pc.dec.Skip(); err != nil {
			return nil, fmt.Errorf("svg: error skipping <%s>: %w", se.Name.Local, err)
		}
		pc.warn(se.Name.Local, "", warnUnsupportedElement, "element not rendered (deliberate subset boundary)")
		return nil, nil //nolint:nilnil // unsupported element
	}
}

// parseAttrs extracts common SVG presentation attributes as raw strings.
// Absent attributes stay "" (= inherit); resolution happens in the
// inheritance evaluator.
func parseAttrs(pc *parseCtx, se xml.StartElement) Attrs {
	a := Attrs{tag: se.Name.Local}
	for _, attr := range se.Attr {
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
		}
	}
	return a
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
			pts, err := parsePoints(pc, "polygon", attr.Value)
			if err != nil {
				return nil, fmt.Errorf("svg: <polygon> points: %w", err)
			}
			elem.Points = pts
		}
	}
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <polygon>: %w", err)
	}
	return elem, nil
}

// parsePolylineElement parses a <polyline> element.
func parsePolylineElement(pc *parseCtx, se xml.StartElement) (*PolylineElement, error) {
	elem := &PolylineElement{Attrs: parseAttrs(pc, se)}
	for _, attr := range se.Attr {
		if attr.Name.Local == "points" {
			pts, err := parsePoints(pc, "polyline", attr.Value)
			if err != nil {
				return nil, fmt.Errorf("svg: <polyline> points: %w", err)
			}
			elem.Points = pts
		}
	}
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <polyline>: %w", err)
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
		case "height":
			elem.H = mustLength(pc, "svg", "height", attr.Value)
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
	pc.doc.Styles = append(pc.doc.Styles, parseStyleContent(pc, text.String()))
	return elem, nil
}

// parsePoints parses an SVG points attribute value into alternating x/y
// lengths ("x1,y1 x2,y2" or "x1 y1 x2 y2").
func parsePoints(pc *parseCtx, elem, s string) ([]Length, error) {
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	if len(parts)%2 != 0 {
		return nil, fmt.Errorf("odd number of coordinate values (%d)", len(parts))
	}
	points := make([]Length, len(parts))
	for i, p := range parts {
		l, err := parseLength(p)
		if err != nil {
			return nil, fmt.Errorf("invalid coordinate %q: %w", p, err)
		}
		points[i] = l
	}
	return points, nil
}
