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
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
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
	elem := &GradientStopElement{Attrs: parseAttrs(pc, se), Opacity: 1}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "offset":
			v := strings.TrimSpace(attr.Value)
			if strings.HasSuffix(v, "%") {
				raw := strings.TrimSpace(strings.TrimSuffix(v, "%"))
				f, err := strconv.ParseFloat(raw, 64)
				if err != nil {
					pc.warn("stop", "offset", warnInvalidValue, err.Error())
				} else {
					elem.Offset = f / 100
				}
			} else if v != "" {
				f, err := strconv.ParseFloat(v, 64)
				if err != nil {
					pc.warn("stop", "offset", warnInvalidValue, err.Error())
				} else {
					elem.Offset = f
				}
			}
			elem.Offset = clamp01(elem.Offset)
		case "stop-color":
			elem.Color = attr.Value
		case "stop-opacity":
			if f, err := strconv.ParseFloat(strings.TrimSpace(attr.Value), 64); err == nil {
				elem.Opacity = clamp01(f)
			}
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
		g.X2 = Length{Value: 1}
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
	pc.doc.Styles = append(pc.doc.Styles, parseStyleContent(pc, text.String()))
	return elem, nil
}

// parseImageElement parses <image> with a data: URI href (external
// references are a named refusal, D2). Decoding runs at parse.
func parseImageElement(pc *parseCtx, se xml.StartElement) (*ImageElement, error) {
	elem := &ImageElement{Attrs: parseAttrs(pc, se)}
	set := map[string]bool{}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "x":
			elem.X = mustLength(pc, "image", "x", attr.Value)
			set["x"] = true
		case "y":
			elem.Y = mustLength(pc, "image", "y", attr.Value)
			set["y"] = true
		case "width":
			elem.W = mustLength(pc, "image", "width", attr.Value)
			set["width"] = true
		case "height":
			elem.H = mustLength(pc, "image", "height", attr.Value)
			set["height"] = true
		case "href":
			elem.HRef = attr.Value
		case "preserveAspectRatio":
			elem.Par = parsePreserveAspectRatio(attr.Value)
			elem.HasPar = true
		}
	}
	if !set["x"] {
		elem.X = Length{}
	}
	if !set["y"] {
		elem.Y = Length{}
	}
	// width/height default to the intrinsic image size at render.
	decodeImageRef(pc, elem)
	// <image> is a leaf: consume up to the matching end (self-closing emits
	// its EndElement immediately — without this the enclosing parseChildren
	// would treat it as the parent's close and drop every later sibling).
	if err := pc.dec.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <image>: %w", err)
	}
	return elem, nil
}

// decodeImageRef resolves the href: data: URIs decode at parse (has/size
// recorded); anything else is a named refusal (cross-file references are
// out of scope, D2).
func decodeImageRef(pc *parseCtx, elem *ImageElement) {
	href := strings.TrimSpace(elem.HRef)
	if href == "" {
		return
	}
	if !strings.HasPrefix(href, "data:") {
		pc.warn("image", "href", warnUnsupportedRef, "external references are not supported (data: URIs only)")
		return
	}
	comma := strings.Index(href, ",")
	if comma < 0 {
		pc.warn("image", "href", warnInvalidValue, "malformed data URI")
		return
	}
	head, payload := href[:comma], href[comma+1:]
	if !strings.Contains(head, ";base64") {
		pc.warn("image", "href", warnUnsupportedRef, "percent-encoded data URIs are not supported (use base64)")
		return
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		pc.warn("image", "href", warnInvalidValue, "base64 decode: "+err.Error())
		return
	}
	mime := strings.TrimPrefix(head, "data:")
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = mime[:i]
	}
	switch mime {
	case "image/png", "image/jpeg", "image/gif":
		cfg, _, err := image.Decode(strings.NewReader(string(raw)))
		if err != nil {
			pc.warn("image", "href", warnInvalidValue, "decode: "+err.Error())
			return
		}
		elem.HasImage = true
		elem.ImgW, elem.ImgH = cfg.Bounds().Dx(), cfg.Bounds().Dy()
		elem.raw = raw
		elem.rawMime = mime
	case "image/svg+xml", "image/webp":
		pc.warn("image", "href", warnUnsupportedRef, "mime "+mime+" is not supported in <image>")
	default:
		pc.warn("image", "href", warnUnsupportedRef, "mime "+mime+" is not supported in <image>")
	}
}

// parsePatternElement parses <pattern> (units default userSpaceOnUse for
// both patternUnits and patternContentUnits — the zero value is
// objectBoundingBox, so both are set explicitly up front).
func parsePatternElement(pc *parseCtx, se xml.StartElement) (*PatternElement, error) {
	elem := &PatternElement{
		Attrs:        parseAttrs(pc, se),
		Units:        UnitsUserSpaceOnUse,
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
			if strings.TrimSpace(attr.Value) == "objectBoundingBox" {
				elem.Units = UnitsObjectBoundingBox
			} else {
				elem.Units = UnitsUserSpaceOnUse
			}
			elem.unitsSet = true
		case "patternContentUnits":
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
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "dx":
				if v, err := parseLength(attr.Value); err == nil {
					fp.Number = v.Value
					fp.HasNumber = true
				}
			case "dy":
				if v, err := parseLength(attr.Value); err == nil {
					fp.Number = v.Value
					fp.HasNumber = true
					fp.Numbers = append(fp.Numbers, v.Value)
				}
			case "stdDeviation":
				fp.Numbers2 = parseNumberList(attr.Value)
			}
		}
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
			decodeImageRef(pc, img)
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
		depth := 1
		for depth > 0 {
			tok, err := pc.dec.Token()
			if err != nil {
				return nil, fmt.Errorf("svg: error reading <feMerge>: %w", err)
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "feMergeNode" {
					for _, attr := range t.Attr {
						if attr.Name.Local == "in" {
							fp.MergeNodes = append(fp.MergeNodes, attr.Value)
						}
					}
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
