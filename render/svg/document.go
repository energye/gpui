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
	"image"
	"strings"

	"github.com/energye/gpui/render"
)

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
	// hasSize records explicit root width/height attributes (usvg
	// restore_viewbox: without them AND without a viewBox the document
	// sizes itself to the content bounding box at render).
	hasSize bool

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

	// Styles holds parsed <style> element blocks; consumed (nil) after
	// parse-time CSS materialization (css.go resolveCSS).
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

	// colorVars holds the CSS custom properties var() color references
	// resolve against (SVG color-table palette contract; nil otherwise).
	colorVars map[string]string

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
	warnUnresolvedPaint     = "unresolved-paint"     // url(#id) target does not exist
	warnUnsupportedPaint    = "unsupported-paint"    // url(#id) target exists but is not a paint server yet
	warnUnresolvedRef       = "unresolved-ref"       // use href target does not exist
	warnUseCycle            = "use-cycle"            // use participates in a reference cycle
	warnUseDepth            = "use-depth"            // use nesting exceeds the depth limit
	warnUnsupportedElement  = "unsupported-element"  // known element deliberately not rendered
	warnUnsupportedSelector = "unsupported-selector" // CSS selector outside the minimal set
	warnInvalidValue        = "invalid-value"        // attribute value failed to parse
	warnUnsupportedRef      = "unsupported-ref"      // reference target exists but wrong type
)

// ViewBox represents the SVG viewBox attribute.
type ViewBox struct {
	MinX, MinY, Width, Height float64
}

// Attrs holds SVG attributes shared by elements. Optional values are raw
// strings: absent ("" = inherit per SVG), present values are resolved by the
// inheritance evaluator.
type Attrs struct {
	ID               string // element id (reference target)
	Class            string // class list for CSS class selectors
	Style            string // inline style declarations (highest cascade priority)
	Fill             string // color | none | currentColor | url(#ref)
	FillRule         string // evenodd | nonzero
	FillOpacity      string // number, "" = inherit
	Stroke           string
	StrokeOpacity    string
	StrokeWidth      string // length, "" = inherit
	StrokeCap        string // round | square | butt
	StrokeJoin       string // round | bevel | miter
	StrokeMiterlimit string
	StrokeDasharray  string // length list | none
	StrokeDashoffset string
	Opacity          string // element-level, not inherited
	Transform        string
	TransformOrigin  string // SVG2 transform-origin raw (usvg resolve_transform)
	ClipRule         string
	Color            string // currentColor source
	FontSize         string // length; font-relative units resolve against parent
	Visibility       string
	Overflow         string // svg elements: visible disables the viewport clip

	// Filter carries the raw filter="url(#id)" reference.
	Filter string

	// Text layout attributes (raw; "" = inherit).
	FontFamily    string
	FontWeight    string
	FontStyle     string
	TextAnchor    string
	LetterSpacing string

	// ClipPath carries the raw clip-path="url(#id)" reference.
	ClipPath string
	// Mask carries the raw mask="url(#id)" reference.
	Mask string
	// MarkerStart/MarkerMid/MarkerEnd carry marker-start/mid/end references
	// (url(#id) | none). The marker shorthand fills all three in attribute
	// order; later longhands override.
	MarkerStart string
	MarkerMid   string
	MarkerEnd   string

	// Raw snapshots every attribute by local name for CSS attribute
	// selectors ([x], [x=1], …) — the typed fields above are the whitelist
	// the renderer consumes; this is the full attribute surface simplecss
	// matches against.
	Raw map[string]string

	// condInvalid marks an invisible element: failed conditional
	// processing or display="none" (usvg is_visible_element). The element
	// and its subtree render nothing and stay unreachable through <use>.
	condInvalid bool
	// condFailed isolates the conditional-processing part (without
	// display) — <switch> child selection tests only this (usvg
	// is_condition_passed), then the chosen child may still be hidden by
	// display=none without falling through to the next child.
	condFailed bool

	// tag is the element tag name (CSS tag selectors); set by the parser.
	tag string
	// clipInvalid marks an unresolved clip-path reference: the element is
	// not rendered (fail-closed).
	clipInvalid bool
	// maskInvalid marks an unresolved mask reference: not rendered.
	maskInvalid bool
	// markerStartInvalid/markerMidInvalid/markerEndInvalid mark unresolved
	// or wrong-type marker references: that position renders no marker
	// (fail-closed), matching mask/clip.
	markerStartInvalid bool
	markerMidInvalid   bool
	markerEndInvalid   bool
	// fontInvalid marks a text element whose font did not resolve: not rendered.
	fontInvalid bool
	// filterInvalid marks an unresolved/wrong-type filter reference.
	filterInvalid bool
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
	Attrs  Attrs
	CX, CY Length
	R      Length
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
	// inert marks an invisible container (unknown SVG tag or foreign
	// namespace): never rendered in place, but children stay referenceable
	// by id and inheritance passes through the empty attrs.
	inert bool
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
	// OverflowVisible (overflow="visible") disables the use-viewport clip
	// (usvg symbol conversion keeps the attribute).
	OverflowVisible bool
	Children        []Element
}

func (e *SymbolElement) attrs() *Attrs { return &e.Attrs }

// MarkerElement represents <marker>: a template painted at path vertices by
// marker-start/mid/end. ViewBox maps to the marker viewport (markerWidth ×
// markerHeight); RefX/RefY is the reference point in marker user units;
// MarkerUnits is "strokeWidth" (default) or "userSpaceOnUse"; Orient is
// "auto", "auto-start-reverse" or an angle; OverflowVisible disables the
// marker viewport clip.
type MarkerElement struct {
	Attrs            Attrs
	ViewBox          ViewBox
	Par              PreserveAspectRatio
	HasViewBox       bool
	MarkerUnits      string
	RefX, RefY       Length
	MarkerW, MarkerH Length
	Orient           string
	OverflowVisible  bool
	Children         []Element
}

func (e *MarkerElement) attrs() *Attrs { return &e.Attrs }

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
	Attrs Attrs
	HRef  string
	X, Y  Length
	W, H  Length // width/height: symbol/svg viewport override (unset = target default)
	hasW  bool
	hasH  bool

	target  Element // resolved at parse
	kind    RefKind
	invalid bool // unresolved/cyclic/over-deep: renders nothing
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
	hasW, hasH      bool // explicit width/height (clip eligibility per usvg)
	ViewBox         ViewBox
	Par             PreserveAspectRatio
	HasViewBox      bool
	OverflowVisible bool
	Children        []Element
}

func (e *SVGElement) attrs() *Attrs { return &e.Attrs }

// ClipPathElement represents <clipPath>: child shapes union into the clip
// region applied through clip-path="url(#id)". A single child keeps its own
// winding (holes survive); multi-child regions union via the boolean base.
type ClipPathElement struct {
	Attrs    Attrs
	Units    Units // clipPathUnits (spec default userSpaceOnUse; stated explicitly — zero value is oBB)
	Children []Element
}

func (e *ClipPathElement) attrs() *Attrs { return &e.Attrs }

// ImageElement represents <image>: an embedded raster via data: URI
// (cross-file references are a named refusal, D2). Decoding happens at
// parse: HasImage/ImgW/ImgH carry the result for default sizing.
type ImageElement struct {
	Attrs      Attrs
	X, Y       Length
	W, H       Length // viewport (default: intrinsic image size, SVG2 auto)
	HRef       string
	Par        PreserveAspectRatio // preserveAspectRatio (default xMidYMid meet)
	HasPar     bool
	HasImage   bool
	ImgW, ImgH int

	// SubDoc carries the parsed sub-document for SVG(Z) image sources
	// (usvg ImageKind::SVG); raster sources use raw/rawMime.
	SubDoc *Document

	raw     []byte // decoded data-URI payload (parse-time)
	rawMime string
	buf     *render.ImageBuf // lazy render-time raster
}

func (e *ImageElement) attrs() *Attrs { return &e.Attrs }

// renderBuf lazily decodes the stored payload into an ImageBuf (once per
// element; the caller owns the Dispose via the document bake lifecycle).
func (e *ImageElement) renderBuf() *render.ImageBuf {
	if e.buf != nil {
		return e.buf
	}
	if !e.HasImage || len(e.raw) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(e.raw))
	if err != nil {
		return nil
	}
	e.buf = render.ImageBufFromImage(img)
	return e.buf
}

// PatternElement represents <pattern>: a tile rendered offscreen and
// repeated through a CustomBrush (tiled sampling; the GPU has a native
// CustomBrush route).
type PatternElement struct {
	Attrs        Attrs
	X, Y         Length // tile origin (patternUnits; default x=y=0)
	W, H         Length // tile size
	Units        Units  // patternUnits (default userSpaceOnUse)
	ContentUnits Units  // patternContentUnits (default userSpaceOnUse)
	Transform    string // patternTransform raw
	ViewBox      ViewBox
	Par          PreserveAspectRatio
	HasViewBox   bool
	Children     []Element
	HRef         string

	setX, setY, setW, setH    bool
	unitsSet, contentUnitsSet bool
}

func (e *PatternElement) attrs() *Attrs { return &e.Attrs }
func (e *PatternElement) unitsAndTransform() (Units, string) {
	return e.Units, e.Transform
}

// TextElement represents <text>: absolutely positioned shaped text rendered
// through the shape paint pipeline (glyph outlines → Path → fill/stroke), so
// gradients/clip/mask apply to text exactly as to shapes.
type TextElement struct {
	Attrs    Attrs
	X, Y     []Length  // multi-value: per-glyph absolute positioning (global char index)
	DX, DY   []Length  // per-glyph relative shifts
	Rotate   []float64 // per-glyph rotation (degrees, around the glyph origin)
	Text     string    // direct character content before any <tspan>
	Children []Element // TspanElement sequence
}

func (e *TextElement) attrs() *Attrs { return &e.Attrs }

// TspanElement represents <tspan>: a styled/positioned sub-run of <text>.
type TspanElement struct {
	Attrs    Attrs
	X, Y     []Length
	DX, DY   []Length
	Rotate   []float64
	Text     string
	Children []Element
}

func (e *TspanElement) attrs() *Attrs { return &e.Attrs }

// TextPathElement represents <textPath> (P3.8 4b-4): text laid out along a
// referenced path's arc length (resvg usvg resolve_text_flow + layout.rs
// collect_normals semantics). The href target converts through the shape
// path builder (any shape works — rect outlines too) with the target's own
// transform baked in; the SVG2 `path` attribute is a registered no-op like
// resvg v0.42 (no href → the text falls back to the normal flow).
type TextPathElement struct {
	Attrs       Attrs
	Text        string
	Href        string    // "#id" reference; "path1" forms and missing hrefs → normal flow
	StartOffset string    // raw (length or percent of the path length)
	Children    []Element // TspanElement / nested TextPathElement
}

func (e *TextPathElement) attrs() *Attrs { return &e.Attrs }

// MaskElement represents <mask>: luminance mask content with a region
// (maskUnits on x/y/width/height; maskContentUnits for the children).
type MaskElement struct {
	Attrs        Attrs
	X, Y         Length // region (default -10%..120% of the referencing bbox)
	W, H         Length
	Units        Units // maskUnits (default objectBoundingBox)
	ContentUnits Units // maskContentUnits (default userSpaceOnUse)
	Children     []Element
}

func (e *MaskElement) attrs() *Attrs { return &e.Attrs }

// StyleElement represents <style>; its rules are collected on the Document
// (Document.Styles) and the element itself renders nothing.
type StyleElement struct {
	Attrs Attrs
}

func (e *StyleElement) attrs() *Attrs { return &e.Attrs }

// Units selects the coordinate space for paint-server geometry
// (gradientUnits / clipPathUnits / patternUnits).
type Units uint8

const (
	// UnitsObjectBoundingBox (default): coordinates are fractions of the
	// painted element's bounding box.
	UnitsObjectBoundingBox Units = iota
	// UnitsUserSpaceOnUse: coordinates are in the current user space.
	UnitsUserSpaceOnUse
)

// String renders the attribute token (diagnostics).
func (u Units) String() string {
	if u == UnitsUserSpaceOnUse {
		return "userSpaceOnUse"
	}
	return "objectBoundingBox"
}

// SpreadMode is the SVG spreadMethod (maps to render.ExtendMode).
type SpreadMode uint8

const (
	SpreadPad SpreadMode = iota
	SpreadRepeat
	SpreadReflect
)

// spreadToken parses the spreadMethod attribute value (default pad).
func spreadToken(s string) SpreadMode {
	switch strings.TrimSpace(s) {
	case "repeat":
		return SpreadRepeat
	case "reflect":
		return SpreadReflect
	}
	return SpreadPad
}

// extendOf maps a spread mode onto the render extend enum.
func (m SpreadMode) extendOf() render.ExtendMode {
	switch m {
	case SpreadRepeat:
		return render.ExtendRepeat
	case SpreadReflect:
		return render.ExtendReflect
	}
	return render.ExtendPad
}

// GradientStopElement represents <stop> inside a gradient.
type GradientStopElement struct {
	Attrs Attrs
	// Offset: raw per usvg convert_stops — missing/invalid/other-unit
	// offsets fall back to the previous stop's resolved number at
	// normalization, so only the raw string is kept here.
	OffsetSet bool
	OffsetRaw string
	// Color raw: number/percent/currentColor/inherit resolved at
	// normalization (currentColor against the inherited color, inherit
	// one level into the owning gradient).
	Color string
	// Opacity: raw string; missing/invalid → 1, inherit → the owning
	// gradient's stop-opacity.
	Opacity    float64
	OpacitySet bool
	OpacityRaw string
}

func (e *GradientStopElement) attrs() *Attrs { return &e.Attrs }

// stopsProvider is implemented by gradient elements for href stop chaining.
type stopsProvider interface {
	Element
	stopsList() []*GradientStopElement
	hrefTarget() string
}

// LinearGradientElement represents <linearGradient>.
type LinearGradientElement struct {
	Attrs     Attrs
	X1, Y1    Length // defaults (0,0)→(1,0) in objectBoundingBox units
	X2, Y2    Length
	Units     Units
	Transform string // gradientTransform raw
	Spread    SpreadMode
	Stops     []*GradientStopElement
	HRef      string // href/xlink:href: unset attributes inherit from the target

	// set* mark explicitly authored attributes (href inheritance is
	// per-attribute first-set-wins).
	setX1, setY1, setX2, setY2 bool
	unitsSet, spreadSet        bool
}

func (e *LinearGradientElement) attrs() *Attrs { return &e.Attrs }
func (e *LinearGradientElement) stopsList() []*GradientStopElement {
	return e.Stops
}
func (e *LinearGradientElement) hrefTarget() string { return e.HRef }
func (e *LinearGradientElement) unitsAndTransform() (Units, string) {
	return e.Units, e.Transform
}

// RadialGradientElement represents <radialGradient>.
type RadialGradientElement struct {
	Attrs     Attrs
	CX, CY, R Length // defaults cx=cy=50%, r=50% in objectBoundingBox units
	FX, FY    Length
	HasFocus  bool // fx/fy present (defaults to cx/cy per-axis)
	Units     Units
	Transform string // gradientTransform raw
	Spread    SpreadMode
	Stops     []*GradientStopElement
	HRef      string

	setCX, setCY, setR, setFX, setFY bool
	unitsSet, spreadSet              bool
}

func (e *RadialGradientElement) attrs() *Attrs { return &e.Attrs }
func (e *RadialGradientElement) stopsList() []*GradientStopElement {
	return e.Stops
}
func (e *RadialGradientElement) hrefTarget() string { return e.HRef }
func (e *RadialGradientElement) unitsAndTransform() (Units, string) {
	return e.Units, e.Transform
}

// EffectiveStops resolves the href stop chain (same-type targets only,
// cycle-safe). Empty result = invalid paint per SVG.
func EffectiveStops(d *Document, g stopsProvider) []*GradientStopElement {
	cur := g
	seen := map[Element]bool{cur.(Element): true}
	for len(cur.stopsList()) == 0 {
		if cur.hrefTarget() == "" {
			return nil
		}
		tgt := d.ElementByID(strings.TrimPrefix(cur.hrefTarget(), "#"))
		if tgt == nil || seen[tgt] {
			return nil
		}
		np, ok := tgt.(stopsProvider)
		if !ok {
			return nil
		}
		seen[tgt] = true
		cur = np
	}
	return cur.stopsList()
}
