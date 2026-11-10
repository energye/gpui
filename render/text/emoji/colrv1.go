//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package emoji

import (
	"encoding/binary"
	"errors"
	"math"
)

// COLRv1 paint table formats (OT 1.9.1). Var* formats carry the same static
// layout plus a varIndexBase; static evaluation (default axis instance)
// reads them identically with deltas = 0.
const (
	PaintColrLayers                  = 1
	PaintSolid                       = 2
	PaintVarSolid                    = 3
	PaintLinearGradient              = 4
	PaintVarLinearGradient           = 5
	PaintRadialGradient              = 6
	PaintVarRadialGradient           = 7
	PaintSweepGradient               = 8
	PaintVarSweepGradient            = 9
	PaintGlyph                       = 10
	PaintColrGlyph                   = 11
	PaintTransform                   = 12
	PaintVarTransform                = 13
	PaintTranslate                   = 14
	PaintVarTranslate                = 15
	PaintScale                       = 16
	PaintVarScale                    = 17
	PaintScaleAroundCenter           = 18
	PaintVarScaleAroundCenter        = 19
	PaintScaleUniform                = 20
	PaintVarScaleUniform             = 21
	PaintScaleUniformAroundCenter    = 22
	PaintVarScaleUniformAroundCenter = 23
	PaintRotate                      = 24
	PaintVarRotate                   = 25
	PaintRotateAroundCenter          = 26
	PaintVarRotateAroundCenter       = 27
	PaintSkew                        = 28
	PaintVarSkew                     = 29
	PaintSkewAroundCenter            = 30
	PaintVarSkewAroundCenter         = 31
	PaintComposite                   = 32
)

// CompositeMode values (OT 1.9.1, from Compositing and Blending Level 1).
const (
	CompositeClear         = 0
	CompositeSrc           = 1
	CompositeDest          = 2
	CompositeSrcOver       = 3
	CompositeDestOver      = 4
	CompositeSrcIn         = 5
	CompositeDestIn        = 6
	CompositeSrcOut        = 7
	CompositeDestOut       = 8
	CompositeSrcAtop       = 9
	CompositeDestAtop      = 10
	CompositeXor           = 11
	CompositePlus          = 12
	CompositeScreen        = 13
	CompositeOverlay       = 14
	CompositeDarken        = 15
	CompositeLighten       = 16
	CompositeColorDodge    = 17
	CompositeColorBurn     = 18
	CompositeHardLight     = 19
	CompositeSoftLight     = 20
	CompositeDifference    = 21
	CompositeExclusion     = 22
	CompositeMultiply      = 23
	CompositeHSLHue        = 24
	CompositeHSLSaturation = 25
	CompositeHSLColor      = 26
	CompositeHSLLuminosity = 27
)

// Color line extend modes (same behavior as SVG spreadMethod).
const (
	ExtendPad     = 0
	ExtendRepeat  = 1
	ExtendReflect = 2
)

// ForegroundPaletteIndex is the reserved CPAL index meaning "text
// foreground color" (v0 LayerRecord and v1 color references).
const ForegroundPaletteIndex = 0xFFFF

// VarDeltas supplies integer deltas for COLRv1 variable fields. varIndexBase
// is a 32-bit delta-set index pair (outer = high 16 bits, inner = low 16) per
// HarfBuzz; a nil provider means the default axis instance (all deltas zero).
type VarDeltas interface {
	Delta(varIndex uint32) int32
}

// ColorStop is one COLRv1 color stop (static instance).
type ColorStop struct {
	Offset       float64
	PaletteIndex uint16
	Alpha        float64
}

// ColorLine is a COLRv1 color line (static instance).
type ColorLine struct {
	Extend uint8
	Stops  []ColorStop
}

// PaintNode is one node of the COLRv1 paint graph in static-evaluation
// form: Var* fields resolve to their base values (deltas = 0), transforms
// compose into the spec 2×3 matrix, and child links resolve eagerly during
// the lazy parse (cycle-guarded; a malformed sub-graph becomes a nil child
// per the spec's ignore-and-continue rule).
type PaintNode struct {
	Format uint8

	// PaintColrLayers (bottom-up z-order slice of the LayerList).
	Children []PaintRef

	// PaintGlyph / PaintColrGlyph / transforms: single child link.
	Child *PaintNode

	// PaintComposite.
	Source   *PaintNode
	Backdrop *PaintNode
	Mode     uint8

	// Solid (2/3).
	PaletteIndex uint16
	Alpha        float64

	// Gradients (4-9): shared color line, format-specific geometry in
	// font design units (y-up), angles in counter-clockwise degrees
	// (sweep angles carry the spec's +1.0 bias already resolved).
	Line                       ColorLine
	X0, Y0                     float64
	X1, Y1                     float64
	X2, Y2                     float64
	R0, R1                     float64
	CenterX, CenterY           float64
	StartAngleDeg, EndAngleDeg float64

	// Transform family (12-31): composed static 2×3 matrix
	// (x' = XX*x + XY*y + DX; y' = YX*x + YY*y + DY).
	XX, YX, XY, YY, DX, DY float64

	// PaintGlyph source glyph (also kept when Child is set).
	GlyphID uint16
}

// PaintRef identifies one child paint of a PaintColrLayers node: either an
// inline node (LayerList entry resolved during parse) or an error marker
// (malformed entry — rendered as nothing, per the spec ignore rule).
type PaintRef struct {
	Node *PaintNode
}

// ClipBox is one COLRv1 clip box in font design units (y-up).
type ClipBox struct {
	StartGlyphID uint16
	EndGlyphID   uint16
	XMin, YMin   float64
	XMax, YMax   float64
}

// Errors for COLRv1 structures.
var (
	// ErrInvalidCOLRv1Data indicates malformed v1 paint structures.
	ErrInvalidCOLRv1Data = errors.New("emoji: invalid COLRv1 paint data")

	// ErrPaintDepthExceeded indicates the paint graph exceeded the
	// traversal depth guard (treated as malformed, fail-closed).
	ErrPaintDepthExceeded = errors.New("emoji: COLRv1 paint depth exceeded")

	// ErrPaintCycle indicates a cyclic paint graph (the spec's ignore rule
	// applies at the node where the cycle is detected).
	ErrPaintCycle = errors.New("emoji: COLRv1 paint cycle")
)

// maxPaintDepth bounds paint graph recursion (same discipline as the SVG
// use-depth guard); real emoji graphs nest a handful of levels.
const maxPaintDepth = 64

// findBaseGlyphPaint locates the BaseGlyphPaintRecord for glyphID (binary
// search; the array is sorted by glyphID per spec) and returns the paint
// offset relative to the BaseGlyphList start.
func (p *COLRParser) findBaseGlyphPaint(glyphID uint16) (uint32, bool) {
	if p.baseGlyphListOffset == 0 {
		return 0, false
	}
	data := p.colrData
	base := int(p.baseGlyphListOffset)
	if base+4 > len(data) {
		return 0, false
	}
	num := int(binary.BigEndian.Uint32(data[base : base+4]))
	lo, hi := 0, num
	for lo < hi {
		mid := (lo + hi) / 2
		rec := base + 4 + mid*6
		if rec+6 > len(data) {
			return 0, false
		}
		gid := binary.BigEndian.Uint16(data[rec : rec+2])
		if gid < glyphID {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo >= num {
		return 0, false
	}
	rec := base + 4 + lo*6
	if rec+6 > len(data) {
		return 0, false
	}
	if binary.BigEndian.Uint16(data[rec:rec+2]) != glyphID {
		return 0, false
	}
	return binary.BigEndian.Uint32(data[rec+2 : rec+6]), true
}

// PaintRoot returns the COLRv1 paint graph root for the base glyph, or
// ErrGlyphNotInCOLR when the glyph has no v1 record. Parsing is lazy and
// cycle-guarded; malformed sub-graphs yield nil children (spec: ignore and
// render the remainder).
func (p *COLRParser) PaintRoot(glyphID uint16) (*PaintNode, error) {
	return p.PaintRootVar(glyphID, nil)
}

// PaintRootVar is PaintRoot at a variation instance. The provider is injected
// through a shallow parser copy so the shared parser (and its lazy caches) stay
// immutable and concurrent calls at different instances cannot race.
func (p *COLRParser) PaintRootVar(glyphID uint16, vars VarDeltas) (*PaintNode, error) {
	off, ok := p.findBaseGlyphPaint(glyphID)
	if !ok {
		return nil, ErrGlyphNotInCOLR
	}
	q := *p
	q.vars = vars
	return q.parsePaint(q.baseGlyphListOffset+off, map[uint32]bool{}, 0)
}

// dv returns the raw integer delta at varIndexBase+i (0 at the default instance).
func (p *COLRParser) dv(base, i uint32) int32 {
	if p.vars == nil {
		return 0
	}
	return p.vars.Delta(base + i)
}

// d14 is an F2DOT14 delta; dfw is a FONT-UNIT (FWORD) delta; dfx is F16DOT16.
func (p *COLRParser) d14(base, i uint32) float64 { return float64(p.dv(base, i)) / 16384 }
func (p *COLRParser) dfw(base, i uint32) float64 { return float64(p.dv(base, i)) }
func (p *COLRParser) dfx(base, i uint32) float64 { return float64(p.dv(base, i)) / 65536 }

// LayerPaint returns the paint at a LayerList index (standalone entry
// point; graph traversal resolves layers through parsePaint's shared path).
func (p *COLRParser) LayerPaint(index uint32) (*PaintNode, error) {
	return p.resolveLayer(index, map[uint32]bool{}, 0)
}

// resolveLayer resolves one LayerList entry within an in-progress traversal.
func (p *COLRParser) resolveLayer(index uint32, path map[uint32]bool, depth int) (*PaintNode, error) {
	if p.layerListOffset == 0 {
		return nil, ErrInvalidCOLRv1Data
	}
	data := p.colrData
	base := int(p.layerListOffset)
	if base+4 > len(data) {
		return nil, ErrInvalidCOLRv1Data
	}
	num := int(binary.BigEndian.Uint32(data[base : base+4]))
	if int(index) >= num {
		return nil, ErrInvalidCOLRv1Data
	}
	pos := base + 4 + int(index)*4
	if pos+4 > len(data) {
		return nil, ErrInvalidCOLRv1Data
	}
	return p.parsePaint(p.layerListOffset+binary.BigEndian.Uint32(data[pos:pos+4]), path, depth)
}

// ClipBoxFor returns the clip box covering glyphID, if the ClipList has one.
func (p *COLRParser) ClipBoxFor(glyphID uint16) (ClipBox, bool) {
	if p.clipListOffset == 0 {
		return ClipBox{}, false
	}
	data := p.colrData
	base := int(p.clipListOffset)
	if base+5 > len(data) || data[base] != 1 {
		return ClipBox{}, false // only format 1 is defined
	}
	num := int(binary.BigEndian.Uint32(data[base+1 : base+5]))
	for i := 0; i < num; i++ {
		rec := base + 5 + i*7 // start(2) end(2) Offset24(3)
		if rec+7 > len(data) {
			return ClipBox{}, false
		}
		start := binary.BigEndian.Uint16(data[rec : rec+2])
		end := binary.BigEndian.Uint16(data[rec+2 : rec+4])
		if glyphID < start || glyphID > end {
			continue
		}
		off := int(data[rec+4])<<16 | int(data[rec+5])<<8 | int(data[rec+6])
		box := base + off
		if box+9 > len(data) || data[box] != 1 {
			return ClipBox{}, false
		}
		return ClipBox{
			StartGlyphID: start,
			EndGlyphID:   end,
			XMin:         float64(int16(binary.BigEndian.Uint16(data[box+1 : box+3]))),
			YMin:         float64(int16(binary.BigEndian.Uint16(data[box+3 : box+5]))),
			XMax:         float64(int16(binary.BigEndian.Uint16(data[box+5 : box+7]))),
			YMax:         float64(int16(binary.BigEndian.Uint16(data[box+7 : box+9]))),
		}, true
	}
	return ClipBox{}, false
}

// parsePaint decodes one paint table (static instance: Var* formats read
// the base layout, varIndexBase is ignored). path carries the table offsets
// on the current traversal for cycle detection.
func (p *COLRParser) parsePaint(abs uint32, path map[uint32]bool, depth int) (*PaintNode, error) {
	if depth > maxPaintDepth {
		return nil, ErrPaintDepthExceeded
	}
	if abs == 0 || int(abs) >= len(p.colrData) {
		return nil, ErrInvalidCOLRv1Data
	}
	if path[abs] {
		return nil, ErrPaintCycle // cycle
	}
	path[abs] = true
	defer delete(path, abs)

	data := p.colrData
	format := data[abs]
	n := &PaintNode{Format: format}
	readColorLine := func(off uint32, variable bool) error {
		cl, err := p.parseColorLine(abs+off, variable)
		if err != nil {
			return err
		}
		n.Line = cl
		return nil
	}

	// resolveChild decodes one child paint; a malformed sub-graph becomes a
	// nil child (spec: ignore that sub-graph and render the remainder).
	resolveChild := func(childAbs uint32) *PaintNode {
		np, err := p.parsePaint(childAbs, path, depth+1)
		if err != nil {
			return nil
		}
		return np
	}

	switch format {
	case PaintColrLayers:
		if int(abs)+6 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		num := int(data[abs+1])
		first := binary.BigEndian.Uint32(data[abs+2 : abs+6])
		children := make([]PaintRef, 0, num)
		for i := 0; i < num; i++ {
			lp, err := p.resolveLayer(first+uint32(i), path, depth+1)
			if err != nil {
				children = append(children, PaintRef{}) // ignore-and-continue
				continue
			}
			children = append(children, PaintRef{Node: lp})
		}
		n.Children = children

	case PaintSolid, PaintVarSolid:
		if int(abs)+5 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.PaletteIndex = binary.BigEndian.Uint16(data[abs+1 : abs+3])
		n.Alpha = f2dot14(data[abs+3 : abs+5])
		if format == PaintVarSolid {
			vb := binary.BigEndian.Uint32(data[abs+5 : abs+9])
			n.Alpha += p.d14(vb, 0)
			if n.Alpha < 0 {
				n.Alpha = 0
			} else if n.Alpha > 1 {
				n.Alpha = 1
			}
		}

	case PaintLinearGradient, PaintVarLinearGradient:
		if int(abs)+14 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		if err := readColorLine(offset24(data[abs+1:abs+4]), format == PaintVarLinearGradient); err != nil {
			return nil, err
		}
		n.X0, n.Y0 = fword(data[abs+4:abs+6]), fword(data[abs+6:abs+8])
		n.X1, n.Y1 = fword(data[abs+8:abs+10]), fword(data[abs+10:abs+12])
		n.X2, n.Y2 = fword(data[abs+12:abs+14]), fword(data[abs+14:abs+16])
		if format == PaintVarLinearGradient {
			vb := binary.BigEndian.Uint32(data[abs+16 : abs+20])
			n.X0 += p.dfw(vb, 0)
			n.Y0 += p.dfw(vb, 1)
			n.X1 += p.dfw(vb, 2)
			n.Y1 += p.dfw(vb, 3)
			n.X2 += p.dfw(vb, 4)
			n.Y2 += p.dfw(vb, 5)
		}

	case PaintRadialGradient, PaintVarRadialGradient:
		if int(abs)+16 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		if err := readColorLine(offset24(data[abs+1:abs+4]), format == PaintVarRadialGradient); err != nil {
			return nil, err
		}
		n.X0, n.Y0 = fword(data[abs+4:abs+6]), fword(data[abs+6:abs+8])
		n.R0 = float64(binary.BigEndian.Uint16(data[abs+8 : abs+10]))
		n.X1, n.Y1 = fword(data[abs+10:abs+12]), fword(data[abs+12:abs+14])
		n.R1 = float64(binary.BigEndian.Uint16(data[abs+14 : abs+16]))
		if format == PaintVarRadialGradient {
			vb := binary.BigEndian.Uint32(data[abs+16 : abs+20])
			n.X0 += p.dfw(vb, 0)
			n.Y0 += p.dfw(vb, 1)
			n.R0 += p.dfw(vb, 2)
			n.X1 += p.dfw(vb, 3)
			n.Y1 += p.dfw(vb, 4)
			n.R1 += p.dfw(vb, 5)
			if n.R0 < 0 {
				n.R0 = 0
			}
			if n.R1 < 0 {
				n.R1 = 0
			}
		}

	case PaintSweepGradient, PaintVarSweepGradient:
		if int(abs)+12 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		if err := readColorLine(offset24(data[abs+1:abs+4]), format == PaintVarSweepGradient); err != nil {
			return nil, err
		}
		n.CenterX, n.CenterY = fword(data[abs+4:abs+6]), fword(data[abs+6:abs+8])
		// Sweep angles carry a +1.0 bias: degrees = (value + 1.0) × 180.
		n.StartAngleDeg = (f2dot14(data[abs+8:abs+10]) + 1.0) * 180.0
		n.EndAngleDeg = (f2dot14(data[abs+10:abs+12]) + 1.0) * 180.0
		if format == PaintVarSweepGradient {
			vb := binary.BigEndian.Uint32(data[abs+12 : abs+16])
			n.CenterX += p.dfw(vb, 0)
			n.CenterY += p.dfw(vb, 1)
			n.StartAngleDeg += p.d14(vb, 2) * 180
			n.EndAngleDeg += p.d14(vb, 3) * 180
		}

	case PaintGlyph:
		if int(abs)+6 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.GlyphID = binary.BigEndian.Uint16(data[abs+4 : abs+6])
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))

	case PaintColrGlyph:
		if int(abs)+3 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		ref := binary.BigEndian.Uint16(data[abs+1 : abs+3])
		off, ok := p.findBaseGlyphPaint(ref)
		if !ok {
			return nil, ErrInvalidCOLRv1Data // not well-formed; ignore node
		}
		child, err := p.parsePaint(p.baseGlyphListOffset+off, path, depth+1)
		if err != nil {
			if errors.Is(err, ErrPaintCycle) {
				// The cycle is detected at the re-entered node — propagate
				// so the node owning that path position fails.
				return nil, err
			}
			// Malformed target subtree: ignore it, this node paints
			// nothing, the remainder of the graph still renders.
			n.Child = nil
			return n, nil
		}
		n.Child = child

	case PaintTransform, PaintVarTransform:
		if int(abs)+7 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		tOff := abs + offset24(data[abs+4:abs+7])
		if int(tOff)+24 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.XX = fixed(data[tOff : tOff+4])
		n.YX = fixed(data[tOff+4 : tOff+8])
		n.XY = fixed(data[tOff+8 : tOff+12])
		n.YY = fixed(data[tOff+12 : tOff+16])
		n.DX = fixed(data[tOff+16 : tOff+20])
		n.DY = fixed(data[tOff+20 : tOff+24])
		if format == PaintVarTransform {
			vb := binary.BigEndian.Uint32(data[abs+7 : abs+11])
			n.XX += p.dfx(vb, 0)
			n.YX += p.dfx(vb, 1)
			n.XY += p.dfx(vb, 2)
			n.YY += p.dfx(vb, 3)
			n.DX += p.dfx(vb, 4)
			n.DY += p.dfx(vb, 5)
		}

	case PaintTranslate, PaintVarTranslate:
		if int(abs)+8 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		n.XX, n.YY = 1, 1
		n.DX, n.DY = fword(data[abs+4:abs+6]), fword(data[abs+6:abs+8])
		if format == PaintVarTranslate {
			vb := binary.BigEndian.Uint32(data[abs+8 : abs+12])
			n.DX += p.dfw(vb, 0)
			n.DY += p.dfw(vb, 1)
		}

	case PaintScale, PaintVarScale:
		if int(abs)+8 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		n.XX = f2dot14(data[abs+4 : abs+6])
		n.YY = f2dot14(data[abs+6 : abs+8])
		if format == PaintVarScale {
			vb := binary.BigEndian.Uint32(data[abs+8 : abs+12])
			n.XX += p.d14(vb, 0)
			n.YY += p.d14(vb, 1)
		}

	case PaintScaleAroundCenter, PaintVarScaleAroundCenter:
		if int(abs)+12 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		sx, sy := f2dot14(data[abs+4:abs+6]), f2dot14(data[abs+6:abs+8])
		cx, cy := fword(data[abs+8:abs+10]), fword(data[abs+10:abs+12])
		if format == PaintVarScaleAroundCenter {
			vb := binary.BigEndian.Uint32(data[abs+12 : abs+16])
			sx += p.d14(vb, 0)
			sy += p.d14(vb, 1)
			cx += p.dfw(vb, 2)
			cy += p.dfw(vb, 3)
		}
		n.XX, n.YY, n.DX, n.DY = scaleAroundCenter(sx, sy, cx, cy)

	case PaintScaleUniform, PaintVarScaleUniform:
		if int(abs)+7 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		s := f2dot14(data[abs+4 : abs+6])
		if format == PaintVarScaleUniform {
			vb := binary.BigEndian.Uint32(data[abs+6 : abs+10])
			s += p.d14(vb, 0)
		}
		n.XX, n.YY = s, s

	case PaintScaleUniformAroundCenter, PaintVarScaleUniformAroundCenter:
		if int(abs)+11 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		s := f2dot14(data[abs+4 : abs+6])
		cx, cy := fword(data[abs+6:abs+8]), fword(data[abs+8:abs+10])
		if format == PaintVarScaleUniformAroundCenter {
			vb := binary.BigEndian.Uint32(data[abs+10 : abs+14])
			s += p.d14(vb, 0)
			cx += p.dfw(vb, 1)
			cy += p.dfw(vb, 2)
		}
		n.XX, n.YY, n.DX, n.DY = scaleAroundCenter(s, s, cx, cy)

	case PaintRotate, PaintVarRotate:
		if int(abs)+6 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		theta := f2dot14(data[abs+4 : abs+6])
		if format == PaintVarRotate {
			vb := binary.BigEndian.Uint32(data[abs+6 : abs+10])
			theta += p.d14(vb, 0)
		}
		theta *= math.Pi // 180° counter-clockwise per 1.0
		n.XX, n.YX = math.Cos(theta), math.Sin(theta)
		n.XY, n.YY = -math.Sin(theta), math.Cos(theta)

	case PaintRotateAroundCenter, PaintVarRotateAroundCenter:
		if int(abs)+10 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		theta := f2dot14(data[abs+4 : abs+6])
		cx, cy := fword(data[abs+6:abs+8]), fword(data[abs+8:abs+10])
		if format == PaintVarRotateAroundCenter {
			vb := binary.BigEndian.Uint32(data[abs+10 : abs+14])
			theta += p.d14(vb, 0)
			cx += p.dfw(vb, 1)
			cy += p.dfw(vb, 2)
		}
		theta *= math.Pi
		xx, yx, xy, yy := math.Cos(theta), math.Sin(theta), -math.Sin(theta), math.Cos(theta)
		n.XX, n.YX, n.XY, n.YY, n.DX, n.DY = composeAroundCenter(xx, yx, xy, yy, cx, cy)

	case PaintSkew, PaintVarSkew:
		if int(abs)+8 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		phi := f2dot14(data[abs+4 : abs+6])
		psi := f2dot14(data[abs+6 : abs+8])
		if format == PaintVarSkew {
			vb := binary.BigEndian.Uint32(data[abs+8 : abs+12])
			phi += p.d14(vb, 0)
			psi += p.d14(vb, 1)
		}
		phi *= math.Pi // x skew, 180° per 1.0
		psi *= math.Pi // y skew
		n.XX, n.YY = 1, 1
		n.YX = math.Tan(psi)
		n.XY = -math.Tan(phi)

	case PaintSkewAroundCenter, PaintVarSkewAroundCenter:
		if int(abs)+12 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Child = resolveChild(abs + offset24(data[abs+1:abs+4]))
		phi := f2dot14(data[abs+4 : abs+6])
		psi := f2dot14(data[abs+6 : abs+8])
		cx, cy := fword(data[abs+8:abs+10]), fword(data[abs+10:abs+12])
		if format == PaintVarSkewAroundCenter {
			vb := binary.BigEndian.Uint32(data[abs+12 : abs+16])
			phi += p.d14(vb, 0)
			psi += p.d14(vb, 1)
			cx += p.dfw(vb, 2)
			cy += p.dfw(vb, 3)
		}
		phi *= math.Pi
		psi *= math.Pi
		xx, yy := 1.0, 1.0
		yx, xy := math.Tan(psi), -math.Tan(phi)
		n.XX, n.YX, n.XY, n.YY, n.DX, n.DY = composeAroundCenter(xx, yx, xy, yy, cx, cy)

	case PaintComposite:
		if int(abs)+8 > len(data) {
			return nil, ErrInvalidCOLRv1Data
		}
		n.Mode = data[abs+4]
		n.Source = resolveChild(abs + offset24(data[abs+1:abs+4]))
		n.Backdrop = resolveChild(abs + offset24(data[abs+5:abs+8]))

	default:
		// Unrecognized format (future minor versions): ignore the node —
		// the caller treats a nil paint as "render nothing here".
		return nil, ErrInvalidCOLRv1Data
	}

	return n, nil
}

// parseColorLine decodes a ColorLine (or VarColorLine) subtable.
func (p *COLRParser) parseColorLine(abs uint32, variable bool) (ColorLine, error) {
	var out ColorLine
	data := p.colrData
	step := 6 // ColorStop: F2DOT14 offset + uint16 paletteIndex + F2DOT14 alpha
	if variable {
		step = 10 // VarColorStop adds a uint32 varIndexBase
	}
	if int(abs)+3 > len(data) {
		return out, ErrInvalidCOLRv1Data
	}
	out.Extend = data[abs]
	num := int(binary.BigEndian.Uint16(data[abs+1 : abs+3]))
	end := int(abs) + 3 + num*step
	if end > len(data) {
		return out, ErrInvalidCOLRv1Data
	}
	for i := 0; i < num; i++ {
		pos := int(abs) + 3 + i*step
		stop := ColorStop{
			Offset:       f2dot14(data[pos : pos+2]),
			PaletteIndex: binary.BigEndian.Uint16(data[pos+2 : pos+4]),
			Alpha:        f2dot14(data[pos+4 : pos+6]),
		}
		if variable {
			vb := binary.BigEndian.Uint32(data[pos+6 : pos+10])
			stop.Offset += p.d14(vb, 0)
			stop.Alpha += p.d14(vb, 1)
			if stop.Alpha < 0 {
				stop.Alpha = 0
			} else if stop.Alpha > 1 {
				stop.Alpha = 1
			}
		}
		out.Stops = append(out.Stops, stop)
	}
	return out, nil
}

// scaleAroundCenter composes T(c)·S·T(−c) for a diagonal scale.
func scaleAroundCenter(sx, sy, cx, cy float64) (xx, yy, dx, dy float64) {
	xx, yy = sx, sy
	dx = cx - sx*cx
	dy = cy - sy*cy
	return
}

// composeAroundCenter wraps a linear 2×2 in T(c)·M·T(−c).
func composeAroundCenter(xx, yx, xy, yy, cx, cy float64) (oxx, oyx, oxy, oyy, odx, ody float64) {
	oxx, oyx, oxy, oyy = xx, yx, xy, yy
	odx = cx - xx*cx - xy*cy
	ody = cy - yx*cx - yy*cy
	return
}

// f2dot14 decodes a signed 2.14 fixed-point value.
func f2dot14(b []byte) float64 {
	return float64(int16(binary.BigEndian.Uint16(b))) / 16384.0
}

// fixed decodes a signed 16.16 fixed-point value.
func fixed(b []byte) float64 {
	return float64(int32(binary.BigEndian.Uint32(b))) / 65536.0
}

// fword decodes a signed FWORD (int16).
func fword(b []byte) float64 {
	return float64(int16(binary.BigEndian.Uint16(b)))
}

// offset24 decodes a 3-byte unsigned offset.
func offset24(b []byte) uint32 {
	return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
}
