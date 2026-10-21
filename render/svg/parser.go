//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
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

// Parse parses SVG XML data into a reusable [Document].
// The Document can be rendered multiple times at different sizes.
//
// Only the SVG subset needed for icon rendering is supported.
// Unsupported elements are silently skipped.
func Parse(data []byte) (*Document, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	doc := &Document{
		ViewBox: ViewBox{Width: 16, Height: 16}, // sensible default
	}

	// Find the root <svg> element.
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: failed to find <svg> root element: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "svg" {
			if err := parseSVGRoot(doc, se); err != nil {
				return nil, err
			}
			// Parse children.
			children, err := parseChildren(decoder)
			if err != nil {
				return nil, err
			}
			doc.Elements = children
			return doc, nil
		}
	}
}

// parseSVGRoot extracts viewBox, width, height from the <svg> element attributes.
func parseSVGRoot(doc *Document, se xml.StartElement) error {
	// Read viewBox first: percentage width/height resolve against it.
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "viewBox":
			vb, err := parseViewBox(attr.Value)
			if err != nil {
				return err
			}
			doc.ViewBox = vb
		case "fill":
			doc.RootFill = attr.Value
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
		}
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
	vb.MinX, err = strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return ViewBox{}, fmt.Errorf("svg: invalid viewBox minX: %w", err)
	}
	vb.MinY, err = strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return ViewBox{}, fmt.Errorf("svg: invalid viewBox minY: %w", err)
	}
	vb.Width, err = strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return ViewBox{}, fmt.Errorf("svg: invalid viewBox width: %w", err)
	}
	vb.Height, err = strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return ViewBox{}, fmt.Errorf("svg: invalid viewBox height: %w", err)
	}
	return vb, nil
}

// parseDimension parses an SVG dimension value, stripping optional units.
// Percentages resolve against ref (the viewport dimension).
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
	// Strip common absolute units; the icon subset treats them as user units.
	s = strings.TrimSuffix(s, "px")
	s = strings.TrimSuffix(s, "pt")
	s = strings.TrimSuffix(s, "em")
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("svg: invalid dimension %q: %w", s, err)
	}
	return v, nil
}

// parseLength parses a child geometry length. Percentage lengths in child
// geometry are not resolved by this icon subset; reject them instead of
// silently treating "50%" as zero. Root width/height percentages are handled
// by parseDimension against the viewBox.
func parseLength(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		return 0, fmt.Errorf("percentage length %q is not supported in child geometry", s)
	}
	s = strings.TrimSuffix(s, "px")
	s = strings.TrimSuffix(s, "pt")
	s = strings.TrimSuffix(s, "em")
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid length %q: %w", s, err)
	}
	return v, nil
}

// parseChildren parses child elements until the matching end element is found.
func parseChildren(decoder *xml.Decoder) ([]Element, error) {
	var elements []Element

	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("svg: error reading XML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			elem, err := parseElement(decoder, t)
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
// Returns nil for unsupported elements (silently skipped).
func parseElement(decoder *xml.Decoder, se xml.StartElement) (Element, error) {
	switch se.Name.Local {
	case "path":
		return parsePathElement(decoder, se)
	case "circle":
		return parseCircleElement(decoder, se)
	case "rect":
		return parseRectElement(decoder, se)
	case "ellipse":
		return parseEllipseElement(decoder, se)
	case "line":
		return parseLineElement(decoder, se)
	case "polygon":
		return parsePolygonElement(decoder, se)
	case "polyline":
		return parsePolylineElement(decoder, se)
	case "g":
		return parseGroupElement(decoder, se)
	default:
		// Skip unsupported elements (defs, clipPath, use, text, etc.).
		if err := decoder.Skip(); err != nil {
			return nil, fmt.Errorf("svg: error skipping <%s>: %w", se.Name.Local, err)
		}
		return nil, nil //nolint:nilnil // nil element means "skip this element"
	}
}

// parseAttrs extracts common SVG presentation attributes from an element.
func parseAttrs(se xml.StartElement) Attrs {
	a := Attrs{
		FillOpacity:   1.0,
		StrokeOpacity: 1.0,
		StrokeWidth:   1.0,
		Opacity:       1.0,
	}
	for _, attr := range se.Attr {
		switch attr.Name.Local {
		case "fill":
			a.Fill = attr.Value
		case "fill-rule":
			a.FillRule = attr.Value
		case "fill-opacity":
			if v, err := strconv.ParseFloat(attr.Value, 64); err == nil {
				a.FillOpacity = v
			}
		case "stroke":
			a.Stroke = attr.Value
		case "stroke-width":
			if v, err := strconv.ParseFloat(attr.Value, 64); err == nil {
				a.StrokeWidth = v
			}
		case "stroke-linecap":
			a.StrokeCap = attr.Value
		case "stroke-linejoin":
			a.StrokeJoin = attr.Value
		case "stroke-opacity":
			if v, err := strconv.ParseFloat(attr.Value, 64); err == nil {
				a.StrokeOpacity = v
			}
		case "opacity":
			if v, err := strconv.ParseFloat(attr.Value, 64); err == nil {
				a.Opacity = v
			}
		case "transform":
			a.Transform = attr.Value
		case "clip-rule":
			a.ClipRule = attr.Value
		}
	}
	return a
}

// parsePathElement parses a <path> element.
func parsePathElement(decoder *xml.Decoder, se xml.StartElement) (*PathElement, error) {
	elem := &PathElement{Attrs: parseAttrs(se)}
	for _, attr := range se.Attr {
		if attr.Name.Local == "d" {
			elem.D = attr.Value
		}
	}
	if err := decoder.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <path>: %w", err)
	}
	return elem, nil
}

// parseCircleElement parses a <circle> element.
func parseCircleElement(decoder *xml.Decoder, se xml.StartElement) (*CircleElement, error) {
	elem := &CircleElement{Attrs: parseAttrs(se)}
	for _, attr := range se.Attr {
		var dst *float64
		switch attr.Name.Local {
		case "cx":
			dst = &elem.CX
		case "cy":
			dst = &elem.CY
		case "r":
			dst = &elem.R
		}
		if dst == nil {
			continue
		}
		v, err := parseLength(attr.Value)
		if err != nil {
			return nil, fmt.Errorf("svg: <circle> %s: %w", attr.Name.Local, err)
		}
		*dst = v
	}
	if err := decoder.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <circle>: %w", err)
	}
	return elem, nil
}

// parseRectElement parses a <rect> element.
func parseRectElement(decoder *xml.Decoder, se xml.StartElement) (*RectElement, error) {
	elem := &RectElement{Attrs: parseAttrs(se)}
	for _, attr := range se.Attr {
		var dst *float64
		switch attr.Name.Local {
		case "x":
			dst = &elem.X
		case "y":
			dst = &elem.Y
		case "width":
			dst = &elem.W
		case "height":
			dst = &elem.H
		case "rx":
			dst = &elem.RX
		case "ry":
			dst = &elem.RY
		}
		if dst == nil {
			continue
		}
		v, err := parseLength(attr.Value)
		if err != nil {
			return nil, fmt.Errorf("svg: <rect> %s: %w", attr.Name.Local, err)
		}
		*dst = v
	}
	if err := decoder.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <rect>: %w", err)
	}
	return elem, nil
}

// parseEllipseElement parses an <ellipse> element.
func parseEllipseElement(decoder *xml.Decoder, se xml.StartElement) (*EllipseElement, error) {
	elem := &EllipseElement{Attrs: parseAttrs(se)}
	for _, attr := range se.Attr {
		var dst *float64
		switch attr.Name.Local {
		case "cx":
			dst = &elem.CX
		case "cy":
			dst = &elem.CY
		case "rx":
			dst = &elem.RX
		case "ry":
			dst = &elem.RY
		}
		if dst == nil {
			continue
		}
		v, err := parseLength(attr.Value)
		if err != nil {
			return nil, fmt.Errorf("svg: <ellipse> %s: %w", attr.Name.Local, err)
		}
		*dst = v
	}
	if err := decoder.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <ellipse>: %w", err)
	}
	return elem, nil
}

// parseLineElement parses a <line> element.
func parseLineElement(decoder *xml.Decoder, se xml.StartElement) (*LineElement, error) {
	elem := &LineElement{Attrs: parseAttrs(se)}
	for _, attr := range se.Attr {
		var dst *float64
		switch attr.Name.Local {
		case "x1":
			dst = &elem.X1
		case "y1":
			dst = &elem.Y1
		case "x2":
			dst = &elem.X2
		case "y2":
			dst = &elem.Y2
		}
		if dst == nil {
			continue
		}
		v, err := parseLength(attr.Value)
		if err != nil {
			return nil, fmt.Errorf("svg: <line> %s: %w", attr.Name.Local, err)
		}
		*dst = v
	}
	if err := decoder.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <line>: %w", err)
	}
	return elem, nil
}

// parsePolygonElement parses a <polygon> element.
func parsePolygonElement(decoder *xml.Decoder, se xml.StartElement) (*PolygonElement, error) {
	elem := &PolygonElement{Attrs: parseAttrs(se)}
	for _, attr := range se.Attr {
		if attr.Name.Local == "points" {
			pts, err := parsePoints(attr.Value)
			if err != nil {
				return nil, fmt.Errorf("svg: <polygon> points: %w", err)
			}
			elem.Points = pts
		}
	}
	if err := decoder.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <polygon>: %w", err)
	}
	return elem, nil
}

// parsePolylineElement parses a <polyline> element.
func parsePolylineElement(decoder *xml.Decoder, se xml.StartElement) (*PolylineElement, error) {
	elem := &PolylineElement{Attrs: parseAttrs(se)}
	for _, attr := range se.Attr {
		if attr.Name.Local == "points" {
			pts, err := parsePoints(attr.Value)
			if err != nil {
				return nil, fmt.Errorf("svg: <polyline> points: %w", err)
			}
			elem.Points = pts
		}
	}
	if err := decoder.Skip(); err != nil {
		return nil, fmt.Errorf("svg: error reading <polyline>: %w", err)
	}
	return elem, nil
}

// parseGroupElement parses a <g> element and its children recursively.
func parseGroupElement(decoder *xml.Decoder, se xml.StartElement) (*GroupElement, error) {
	elem := &GroupElement{Attrs: parseAttrs(se)}
	children, err := parseChildren(decoder)
	if err != nil {
		return nil, err
	}
	elem.Children = children
	return elem, nil
}

// parsePoints parses an SVG points attribute value.
// Supports both "x1,y1 x2,y2" and "x1 y1 x2 y2" formats.
func parsePoints(s string) ([]float64, error) {
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	if len(parts)%2 != 0 {
		return nil, fmt.Errorf("odd number of coordinate values (%d)", len(parts))
	}
	points := make([]float64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid coordinate %q: %w", p, err)
		}
		points[i] = v
	}
	return points, nil
}
