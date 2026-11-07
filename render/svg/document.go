//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

// Document represents a parsed SVG document.
// It can be rendered multiple times at different sizes, making it suitable
// for caching parsed SVG icons.
type Document struct {
	// ViewBox defines the SVG coordinate system (minX, minY, width, height).
	// Zero if the document has no viewBox attribute (user units = px).
	ViewBox ViewBox

	// Width and Height are the explicit width/height from the SVG root element.
	// If zero, ViewBox dimensions are used.
	Width, Height float64

	// RootFill is the fill attribute from the <svg> root element.
	// In SVG, presentation attributes on <svg> are inherited by children.
	// Common in expui icons: fill="none" on root.
	RootFill string

	// Par is the root preserveAspectRatio (zero value = xMidYMid meet).
	Par PreserveAspectRatio

	// OverflowVisible is true when the root declares overflow="visible";
	// the default clips content to the viewport.
	OverflowVisible bool

	// Elements contains the top-level SVG elements.
	Elements []Element

	// Styles holds parsed <style> element blocks (minimal selector set).
	Styles []StyleBlock

	// Warnings collects parse-time and reference-validation diagnostics.
	// Parse still succeeds on warnings (fault-tolerant rendering, resvg
	// alike); strict callers can inspect this list.
	Warnings []ParseWarning

	// byID indexes every element carrying an id (defs content included).
	byID map[string]Element

	// rootAttrs carries the root <svg> presentation attributes for
	// inheritance (fill, color, font-size, ...).
	rootAttrs Attrs

	// hasViewBox is true when the root declared a viewBox.
	hasViewBox bool
}

// ElementByID returns the element with the given id (defs content included)
// or nil when absent.
func (d *Document) ElementByID(id string) Element {
	if d == nil || d.byID == nil {
		return nil
	}
	return d.byID[id]
}

// ParseWarning is one non-fatal diagnostic from Parse: an unsupported or
// dangling construct that renders as "not painted" instead of wrong pixels.
type ParseWarning struct {
	// Element is the tag name the warning originates from.
	Element string
	// Attribute is the attribute name, when applicable.
	Attribute string
	// Reason is a stable code (the warn* constants); match on it.
	Reason string
	// Detail carries the human-readable context (the offending reference,
	// the parse error text).
	Detail string
	// Offset is the byte offset of the element start in the source, when known.
	Offset int
}

// Warning reason codes carried in ParseWarning.Reason.
const (
	warnUnresolvedPaint    = "unresolved-paint"    // url(#id) target does not exist
	warnUnsupportedPaint   = "unsupported-paint"   // url(#id) target exists but is not a paint server yet
	warnUnresolvedRef      = "unresolved-ref"      // use href target does not exist
	warnUseCycle           = "use-cycle"           // use participates in a reference cycle
	warnUseDepth           = "use-depth"           // use nesting exceeds the depth limit
	warnUnsupportedElement  = "unsupported-element" // known element deliberately not rendered
	warnUnsupportedSelector = "unsupported-selector" // CSS selector outside the minimal set
	warnInvalidValue        = "invalid-value"        // attribute value failed to parse
)

// ViewBox represents the SVG viewBox attribute.
type ViewBox struct {
	MinX, MinY, Width, Height float64
}

// Attrs holds SVG attributes shared by elements. Optional values are raw
// strings: absent ("" = inherit per SVG), present values are resolved by the
// inheritance evaluator.
type Attrs struct {
	ID     string // element id (reference target)
	Class  string // class list for CSS class selectors
	Style  string // inline style declarations (highest cascade priority)
	Fill   string // color | none | currentColor | url(#ref)
	FillRule string // evenodd | nonzero
	FillOpacity string // number, "" = inherit
	Stroke string
	StrokeOpacity string
	StrokeWidth string // length, "" = inherit
	StrokeCap string   // round | square | butt
	StrokeJoin string  // round | bevel | miter
	StrokeMiterlimit string
	StrokeDasharray string  // length list | none
	StrokeDashoffset string
	Opacity string // element-level, not inherited
	Transform string
	ClipRule string
	Color string    // currentColor source
	FontSize string // length; font-relative units resolve against parent
	Visibility string
	Overflow string // svg elements: visible disables the viewport clip

	// tag is the element tag name (CSS tag selectors); set by the parser.
	tag string
	// fillInvalid/strokeInvalid mark url() paints that failed reference
	// validation: the paint is treated as "none" (fail-closed, not black).
	fillInvalid, strokeInvalid bool
}

// Element is the interface implemented by all SVG element types.
type Element interface {
	// attrs returns the common presentation attributes for this element.
	attrs() *Attrs
}

// PathElement represents an SVG <path> element.
type PathElement struct {
	Attrs Attrs
	D     string // SVG path data string (user units, no unit suffixes)
}

func (e *PathElement) attrs() *Attrs { return &e.Attrs }

// CircleElement represents an SVG <circle> element.
type CircleElement struct {
	Attrs    Attrs
	CX, CY   Length
	R        Length
}

func (e *CircleElement) attrs() *Attrs { return &e.Attrs }

// RectElement represents an SVG <rect> element.
type RectElement struct {
	Attrs      Attrs
	X, Y, W, H Length
	RX, RY     Length // corner radii
}

func (e *RectElement) attrs() *Attrs { return &e.Attrs }

// EllipseElement represents an SVG <ellipse> element.
type EllipseElement struct {
	Attrs  Attrs
	CX, CY Length
	RX, RY Length
}

func (e *EllipseElement) attrs() *Attrs { return &e.Attrs }

// LineElement represents an SVG <line> element.
type LineElement struct {
	Attrs          Attrs
	X1, Y1, X2, Y2 Length
}

func (e *LineElement) attrs() *Attrs { return &e.Attrs }

// PolygonElement represents an SVG <polygon> element.
type PolygonElement struct {
	Attrs  Attrs
	Points []Length // alternating x (axis X), y (axis Y) values
}

func (e *PolygonElement) attrs() *Attrs { return &e.Attrs }

// PolylineElement represents an SVG <polyline> element.
type PolylineElement struct {
	Attrs  Attrs
	Points []Length
}

func (e *PolylineElement) attrs() *Attrs { return &e.Attrs }

// GroupElement represents an SVG <g> element with children.
type GroupElement struct {
	Attrs    Attrs
	Children []Element
}

func (e *GroupElement) attrs() *Attrs { return &e.Attrs }

// DefsElement represents <defs>: collected for reference, never rendered.
type DefsElement struct {
	Attrs    Attrs
	Children []Element
}

func (e *DefsElement) attrs() *Attrs { return &e.Attrs }

// SymbolElement represents <symbol>: a template instantiated by <use>
// with its own viewBox/preserveAspectRatio viewport.
type SymbolElement struct {
	Attrs      Attrs
	ViewBox    ViewBox
	Par        PreserveAspectRatio
	HasViewBox bool
	Children   []Element
}

func (e *SymbolElement) attrs() *Attrs { return &e.Attrs }

// RefKind classifies what a <use> resolves to.
type RefKind uint8

const (
	RefKindNone RefKind = iota
	RefKindElement
	RefKindSymbol
)

// UseElement represents <use>: an instance of a referenced element or
// symbol. HRef accepts SVG2 href and SVG 1.1 xlink:href.
type UseElement struct {
	Attrs      Attrs
	HRef       string
	X, Y       Length
	W, H       Length // width/height: symbol viewport size (default 100%)
	target     Element // resolved at parse
	kind       RefKind
	invalid    bool // unresolved/cyclic/over-deep: renders nothing
}

func (e *UseElement) attrs() *Attrs { return &e.Attrs }

// RefKind reports what this use resolves to (element or symbol); unresolved
// uses report RefKindNone.
func (e *UseElement) RefKind() RefKind { return e.kind }

// SVGElement represents a nested <svg>: a new viewport with its own
// viewBox/preserveAspectRatio (the root is carried by Document itself).
type SVGElement struct {
	Attrs           Attrs
	X, Y            Length
	W, H            Length
	ViewBox         ViewBox
	Par             PreserveAspectRatio
	HasViewBox      bool
	OverflowVisible bool
	Children        []Element
}

func (e *SVGElement) attrs() *Attrs { return &e.Attrs }

// StyleElement represents <style>; its rules are collected on the Document
// (Document.Styles) and the element itself renders nothing.
type StyleElement struct {
	Attrs Attrs
}

func (e *StyleElement) attrs() *Attrs { return &e.Attrs }
