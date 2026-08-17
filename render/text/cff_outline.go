// CFF outline path for ownParsedFont (ENGINE_GAPS G1.b).
//
// TrueType fonts keep using the pure-Go glyf path. OpenType fonts with a
// "CFF " table (and no glyf) load outlines via golang.org/x/image/font/sfnt,
// which is already a module dependency. This restores out-of-box rendering for
// system OTF / Noto CJK CFF collections without re-implementing Type2
// charstrings in-tree.
//
// Scope:
//   - CFF (OpenType OTTO) outlines → GlyphOutline segments (incl. cubics)
//   - GlyphBounds for CFF (from loaded segments)
//   - TTC/OTC via sfnt.ParseCollection + collection index
//
// CFF2: parsed via github.com/go-text/typesetting/font/cff (default instance;
// coords empty) and with axis blend when FontVariation coords are supplied
// (normalized F2.14 via fvar/avar → go-text tables.Coord).
// CFF has no TT/auto-hint (unhinted outlines still draw).
package text

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/go-text/typesetting/font/cff"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// ErrCFF2Unsupported is returned when a CFF2 table is present but cannot be
// parsed or loaded (corrupt/truncated table). Successful CFF2 fonts use the
// go-text path and do not return this error.
var ErrCFF2Unsupported = errors.New("text: CFF2 outlines are not yet supported")

// cffOutlineSupport is attached lazily on ownParsedFont when tables have CFF
// and no glyf.
type cffOutlineSupport struct {
	font *sfnt.Font
}

// cff2OutlineSupport holds a parsed CFF2 table for glyph outline extraction.
type cff2OutlineSupport struct {
	font *cff.CFF2
}

// hasCFFTable reports CFF 1 ("CFF ") without glyf — the supported path
// hasCFFTable reports CFF 1 ("CFF ") without glyf — the supported path.
func (f *ownParsedFont) hasCFFTable() bool {
	if f == nil || f.tables == nil {
		return false
	}
	if _, ok := f.tables["glyf"]; ok {
		return false
	}
	_, ok := f.tables["CFF "]
	return ok
}

// hasCFF2Table reports a CFF2 table without TrueType glyf.
func (f *ownParsedFont) hasCFF2Table() bool {
	if f == nil || f.tables == nil {
		return false
	}
	if _, ok := f.tables["glyf"]; ok {
		return false
	}
	_, ok := f.tables["CFF2"]
	return ok
}

// hasPostScriptOutlines is true for CFF 1 or CFF2 without glyf.
func (f *ownParsedFont) hasPostScriptOutlines() bool {
	return f.hasCFFTable() || f.hasCFF2Table()
}

func (f *ownParsedFont) ensureCFF() (*cffOutlineSupport, error) {
	if f == nil {
		return nil, fmt.Errorf("text: cff: nil font")
	}
	return f.cff.load(func() (*cffOutlineSupport, error) {
		if !f.hasCFFTable() {
			return nil, fmt.Errorf("text: cff: no CFF table")
		}
		src := f.rawData
		if len(src) < 4 {
			return nil, fmt.Errorf("text: cff: empty font data")
		}
		var (
			sf  *sfnt.Font
			err error
		)
		if binary.BigEndian.Uint32(src[0:4]) == tagTTCF {
			col, cerr := sfnt.ParseCollection(src)
			if cerr != nil {
				return nil, fmt.Errorf("text: cff: collection: %w", cerr)
			}
			sf, err = col.Font(f.collectionIndex)
		} else {
			sf, err = sfnt.Parse(src)
		}
		if err != nil {
			return nil, fmt.Errorf("text: cff: parse: %w", err)
		}
		return &cffOutlineSupport{font: sf}, nil
	})
}

func (f *ownParsedFont) ensureCFF2() (*cff2OutlineSupport, error) {
	if f == nil {
		return nil, fmt.Errorf("text: cff2: nil font")
	}
	return f.cff2.load(func() (*cff2OutlineSupport, error) {
		if !f.hasCFF2Table() {
			return nil, fmt.Errorf("%w", ErrCFF2Unsupported)
		}
		raw, ok := f.tables["CFF2"]
		if !ok || len(raw) == 0 {
			return nil, fmt.Errorf("%w", ErrCFF2Unsupported)
		}
		parsed, err := cff.ParseCFF2(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCFF2Unsupported, err)
		}
		if len(parsed.Charstrings) == 0 {
			return nil, fmt.Errorf("%w: empty charstrings", ErrCFF2Unsupported)
		}
		return &cff2OutlineSupport{font: parsed}, nil
	})
}

// fixed26_6ToFloat converts a 26.6 fixed-point value to float64 pixels.
func fixed26_6ToFloat(v fixed.Int26_6) float64 {
	return float64(v) / 64
}

// extractCFFOutline loads a CFF glyph outline at ppem (pixels per em).
// Coordinates are already Y-down from sfnt.LoadGlyph.
func (f *ownParsedFont) extractCFFOutline(gid GlyphID, size float64) (*GlyphOutline, error) {
	// Prefer CFF 1 when both exist (rare); CFF2 alone uses go-text path.
	if f.hasCFF2Table() && !f.hasCFFTable() {
		return f.extractCFF2Outline(gid, size, nil)
	}
	sup, err := f.ensureCFF()
	if err != nil {
		return nil, err
	}
	if size <= 0 {
		return nil, &FontError{Reason: "cff: non-positive size"}
	}
	sf := sup.font
	if int(gid) >= sf.NumGlyphs() {
		return nil, &FontError{Reason: fmt.Sprintf("cff: glyph ID %d out of range", gid)}
	}

	ppem := fixed.Int26_6(size * 64)
	if ppem <= 0 {
		ppem = 1
	}

	var buf sfnt.Buffer
	segs, err := sf.LoadGlyph(&buf, sfnt.GlyphIndex(gid), ppem, nil)
	if err != nil {
		return nil, fmt.Errorf("text: cff LoadGlyph: %w", err)
	}

	advance := float32(f.GlyphAdvance(uint16(gid), size))
	if len(segs) == 0 {
		return &GlyphOutline{
			GID:     gid,
			Type:    GlyphTypeOutline,
			Advance: advance,
		}, nil
	}

	segments := make([]OutlineSegment, 0, len(segs))
	for _, s := range segs {
		seg, ok := sfntSegmentToOutline(s)
		if !ok {
			continue
		}
		segments = append(segments, seg)
	}

	outline := &GlyphOutline{
		Segments: segments,
		GID:      gid,
		Type:     GlyphTypeOutline,
		Advance:  advance,
	}
	if len(segments) > 0 {
		minX, minY := float64(1e10), float64(1e10)
		maxX, maxY := float64(-1e10), float64(-1e10)
		for _, seg := range segments {
			for j := range segPointCount(seg.Op) {
				updateBounds(seg.Points[j], &minX, &minY, &maxX, &maxY)
			}
		}
		outline.Bounds = Rect{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
	}
	return outline, nil
}

// extractCFF2Outline loads a CFF2 glyph, optionally blending variation axes.
// variations nil/empty → default master. Non-empty → fvar normalize + avar + blend.
// go-text segments are font-unit Y-up; we scale to ppem and flip Y to match
// the rest of the pipeline (Y-down pixels, same as sfnt CFF1 path).
func (f *ownParsedFont) extractCFF2Outline(gid GlyphID, size float64, variations []FontVariation) (*GlyphOutline, error) {
	sup, err := f.ensureCFF2()
	if err != nil {
		return nil, err
	}
	if size <= 0 {
		return nil, &FontError{Reason: "cff2: non-positive size"}
	}
	upem := f.UnitsPerEm()
	if upem == 0 {
		return nil, &FontError{Reason: "cff2: zero unitsPerEm"}
	}
	cs := sup.font.Charstrings
	if int(gid) >= len(cs) {
		return nil, &FontError{Reason: fmt.Sprintf("cff2: glyph ID %d out of range", gid)}
	}

	coords := f.cff2VariationCoords(variations)
	segs, _, err := sup.font.LoadGlyph(tables.GlyphID(gid), coords)
	if err != nil {
		return nil, fmt.Errorf("text: cff2 LoadGlyph: %w", err)
	}

	var advance float32
	if len(variations) > 0 {
		advance = float32(f.GlyphAdvanceVar(uint16(gid), size, variations))
	} else {
		advance = float32(f.GlyphAdvance(uint16(gid), size))
	}
	if len(segs) == 0 {
		return &GlyphOutline{
			GID:     gid,
			Type:    GlyphTypeOutline,
			Advance: advance,
		}, nil
	}

	scale := size / float64(upem)
	segments := make([]OutlineSegment, 0, len(segs))
	for _, s := range segs {
		seg, ok := goTextSegmentToOutline(s, scale)
		if !ok {
			continue
		}
		segments = append(segments, seg)
	}

	outline := &GlyphOutline{
		Segments: segments,
		GID:      gid,
		Type:     GlyphTypeOutline,
		Advance:  advance,
	}
	if len(segments) > 0 {
		minX, minY := float64(1e10), float64(1e10)
		maxX, maxY := float64(-1e10), float64(-1e10)
		for _, seg := range segments {
			for j := range segPointCount(seg.Op) {
				updateBounds(seg.Points[j], &minX, &minY, &maxX, &maxY)
			}
		}
		outline.Bounds = Rect{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
	}
	return outline, nil
}

// cff2VariationCoords converts user FontVariation values to go-text
// normalized coordinates (F2.14). Returns nil for default instance so
// LoadGlyph skips blend application (still handles blend ops for stack).
func (f *ownParsedFont) cff2VariationCoords(variations []FontVariation) []tables.Coord {
	if f == nil || len(variations) == 0 {
		return nil
	}
	axes := f.loadFvar()
	if len(axes) == 0 {
		return nil
	}
	coords := normalizeCoords(axes, variations)
	f.loadAvar().apply(coords)
	allZero := true
	for _, c := range coords {
		if c != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return nil
	}
	out := make([]tables.Coord, len(coords))
	for i, c := range coords {
		// Our normalizeCoords and go-text tables.Coord are both F2.14 int16.
		out[i] = tables.Coord(c)
	}
	return out
}

// goTextSegmentToOutline converts go-text font-unit Y-up segments to our
// Y-down pixel OutlineSegment at the given scale (ppem/upem).
func goTextSegmentToOutline(s ot.Segment, scale float64) (OutlineSegment, bool) {
	var op OutlineOp
	n := 1
	switch s.Op {
	case ot.SegmentOpMoveTo:
		op = OutlineOpMoveTo
	case ot.SegmentOpLineTo:
		op = OutlineOpLineTo
	case ot.SegmentOpQuadTo:
		op = OutlineOpQuadTo
		n = 2
	case ot.SegmentOpCubeTo:
		op = OutlineOpCubicTo
		n = 3
	default:
		return OutlineSegment{}, false
	}
	var pts [3]OutlinePoint
	for i := 0; i < n; i++ {
		// Y-up font units → Y-down pixels
		pts[i] = OutlinePoint{
			X: float32(float64(s.Args[i].X) * scale),
			Y: float32(-float64(s.Args[i].Y) * scale),
		}
	}
	return OutlineSegment{Op: op, Points: pts}, true
}

func sfntSegmentToOutline(s sfnt.Segment) (OutlineSegment, bool) {
	var op OutlineOp
	n := 1
	switch s.Op {
	case sfnt.SegmentOpMoveTo:
		op = OutlineOpMoveTo
	case sfnt.SegmentOpLineTo:
		op = OutlineOpLineTo
	case sfnt.SegmentOpQuadTo:
		op = OutlineOpQuadTo
		n = 2
	case sfnt.SegmentOpCubeTo:
		op = OutlineOpCubicTo
		n = 3
	default:
		return OutlineSegment{}, false
	}
	var pts [3]OutlinePoint
	for i := 0; i < n; i++ {
		pts[i] = OutlinePoint{
			X: float32(fixed26_6ToFloat(s.Args[i].X)),
			Y: float32(fixed26_6ToFloat(s.Args[i].Y)),
		}
	}
	return OutlineSegment{Op: op, Points: pts}, true
}

// glyphBoundsCFF returns glyph bounds from CFF segments (Y-down pixels).
// cffBoundsKey keys the CFF glyph-bbox cache: bounds scale with the pixel
// size, so the key carries the quantized size (Q4).
type cffBoundsKey struct {
	gid    uint16
	sizeQ4 int16
}

// cffBoundsCacheCap bounds the per-font CFF bbox cache (glyphs × size
// quanta); on overflow the cache is rebuilt (the active glyph set is small).
const cffBoundsCacheCap = 4096

func (f *ownParsedFont) glyphBoundsCFF(glyphIndex uint16, ppem float64) Rect {
	if f == nil || ppem <= 0 {
		return Rect{}
	}
	k := cffBoundsKey{gid: glyphIndex, sizeQ4: int16(ppem * 16)}
	f.cffBoundsMu.Lock()
	if b, ok := f.cffBounds[k]; ok {
		f.cffBoundsMu.Unlock()
		return b
	}
	f.cffBoundsMu.Unlock()

	outline, err := f.extractCFFOutline(GlyphID(glyphIndex), ppem)
	if err != nil || outline == nil {
		return Rect{}
	}
	b := outline.Bounds

	f.cffBoundsMu.Lock()
	if f.cffBounds == nil {
		f.cffBounds = make(map[cffBoundsKey]Rect, 64)
	}
	if len(f.cffBounds) >= cffBoundsCacheCap {
		f.cffBounds = make(map[cffBoundsKey]Rect, 64)
	}
	f.cffBounds[k] = b
	f.cffBoundsMu.Unlock()
	return b
}
