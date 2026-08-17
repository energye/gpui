// Own font parser — Pure Go implementation of ParsedFont.
//
// ownParsedFont replaces ximageParsedFont with direct binary parsing
// of TrueType/OpenType tables. Zero external dependencies for font parsing.
//
// Implements:
//   - ParsedFont              (core interface)
//   - VariableAdvanceProvider  (HVAR-based variable font advance)
//   - RawFontDataProvider      (raw bytes for auto-hinter and TT interpreter)
//
// Lazy initialization: table directory is parsed eagerly (cheap), individual
// tables are parsed on first access via sync.Once (thread-safe).
//
// This file is part of Phase 3a (ADR-048: Pure Go Font Stack).
package text

import (
	"encoding/binary"
	"fmt"
	"sync"
)

// ownParser implements FontParser using pure Go binary parsing.
type ownParser struct{}

// lazySlot lazily computes a value of type T at most once (thread-safe).
// It replaces the former per-table sync.Once + sentinel-field boilerplate:
// a zero value of T (nil pointer/slice, false flag) marks "not available",
// and an optional error is cached alongside the value (cff/cff2 slots).
type lazySlot[T any] struct {
	once sync.Once
	err  error
	val  T
}

// load runs init at most once and returns the cached value/error.
func (s *lazySlot[T]) load(init func() (T, error)) (T, error) {
	s.once.Do(func() {
		v, e := init()
		s.val, s.err = v, e
	})
	return s.val, s.err
}

// Typed results of the lazy table slots on ownParsedFont. Each groups the
// cache fields a table parse produces, replacing scattered sentinel fields.
type hmtxLazy struct {
	adv         []uint16 // advance widths from hmtx
	lsb         []int16  // left side bearings from hmtx (unused currently, kept for GlyphBounds)
	numHMetrics int      // from hhea.numberOfHMetrics
	parsed      bool     // true if hmtx was parsed successfully
}

type vmtxLazy struct {
	adv    []uint16 // advance heights from vmtx
	tsb    []int16  // top side bearings from vmtx
	parsed bool     // true if vmtx was parsed successfully
	vhea   vheaMetrics
	vheaOK bool // vhea present+valid (independent of vmtx)
}

type nameLazy struct {
	family string
	full   string
}

type metricsLazy struct {
	hhea   hheaMetrics // from hhea table
	os2    os2Metrics  // from OS/2 table
	hheaOK bool        // true if hhea was parsed successfully
	os2OK  bool        // true if OS/2 was parsed successfully
}

// Parse implements FontParser.Parse.
func (p *ownParser) Parse(data []byte) (ParsedFont, error) {
	return p.ParseIndex(data, 0)
}

// ParseIndex parses a font at the given index within a collection.
// For single fonts (.ttf/.otf), index is ignored. For collections
// (.ttc/.otc), index selects which font to use (0 = first).
func (p *ownParser) ParseIndex(data []byte, index int) (ParsedFont, error) {
	tables, err := parseFontTablesIndex(data, index)
	if err != nil {
		return nil, fmt.Errorf("text: own parser: %w", err)
	}

	// Parse head table eagerly — upem is required by many methods.
	headData, ok := tables["head"]
	if !ok {
		return nil, fmt.Errorf("text: own parser: missing head table")
	}
	upem, err := parseHeadUnitsPerEm(headData)
	if err != nil {
		return nil, fmt.Errorf("text: own parser: %w", err)
	}

	// Parse maxp numGlyphs eagerly.
	maxpData, ok := tables["maxp"]
	if !ok {
		return nil, fmt.Errorf("text: own parser: missing maxp table")
	}
	if len(maxpData) < 6 {
		return nil, fmt.Errorf("text: own parser: maxp table too short")
	}
	numGlyphs := int(binary.BigEndian.Uint16(maxpData[4:6]))

	font := &ownParsedFont{
		rawData:         data,
		collectionIndex: index,
		tables:          tables,
		upem:            upem,
		numGlyphs:       numGlyphs,
	}

	return font, nil
}

// ownParsedFont implements ParsedFont with direct binary parsing.
type ownParsedFont struct {
	rawData         []byte            // raw font file bytes (may be TTC/OTC)
	collectionIndex int               // index used when parsing a collection
	tables          map[string][]byte // raw table data keyed by tag

	// Eagerly parsed (cheap).
	upem      int
	numGlyphs int

	// Lazily parsed via lazySlot — each ensure*/load* method runs its init
	// closure at most once; a zero value marks the table absent/unparseable.

	cmap    lazySlot[*cmapLookup]         // nil if cmap missing or unparseable
	hmtx    lazySlot[hmtxLazy]            // hmtx advance widths (hhea+hmtx)
	vmtx    lazySlot[vmtxLazy]            // vmtx heights + vhea (vertical metrics)
	name    lazySlot[nameLazy]            // family/full names from name table
	metrics lazySlot[metricsLazy]         // hhea + OS/2 font-level metrics
	fvar    lazySlot[[]fvarAxis]          // fvar axes (HVAR/gvar coord normalization)
	hvar    lazySlot[*hvarTable]          // HVAR deltas; nil if absent/failed
	ttHint  lazySlot[*ttHintCache]        // TT bytecode hint cache; nil if no TT
	gvar    lazySlot[*gvarTable]          // gvar deltas; nil if absent/failed
	avar    lazySlot[*avarTable]          // avar remapping; nil if absent
	glyf    lazySlot[*cachedGlyfParser]   // glyf/loca contour parser cache
	cff     lazySlot[*cffOutlineSupport]  // CFF 1 outline backend (sfnt)
	cff2    lazySlot[*cff2OutlineSupport] // CFF2 outline backend (go-text)

	// cffBounds caches CFF/CFF2 glyph bboxes per (gid, quantized size).
	// GlyphBounds is called for every glyph of every shaping pass
	// (face.Glyphs) and previously ran a full CFF outline extraction
	// (charstring parse + FDSelect lookup) each time.
	cffBoundsMu sync.Mutex
	cffBounds   map[cffBoundsKey]Rect
}

// --- ParsedFont interface ---

// Name implements ParsedFont.Name.
func (f *ownParsedFont) Name() string {
	return f.ensureName().family
}

// FullName implements ParsedFont.FullName.
func (f *ownParsedFont) FullName() string {
	return f.ensureName().full
}

// NumGlyphs implements ParsedFont.NumGlyphs.
func (f *ownParsedFont) NumGlyphs() int {
	return f.numGlyphs
}

// UnitsPerEm implements ParsedFont.UnitsPerEm.
func (f *ownParsedFont) UnitsPerEm() int {
	return f.upem
}

// GlyphIndex implements ParsedFont.GlyphIndex.
func (f *ownParsedFont) GlyphIndex(r rune) uint16 {
	return f.ensureCmap().glyphIndex(r)
}

// GlyphAdvance implements ParsedFont.GlyphAdvance.
// Returns the advance width in pixels: advanceFU * ppem / upem.
func (f *ownParsedFont) GlyphAdvance(glyphIndex uint16, ppem float64) float64 {
	hm := f.ensureHmtx()
	if !hm.parsed || f.upem == 0 {
		return 0
	}
	advFU := hmtxAdvance(hm.adv, hm.numHMetrics, glyphIndex)
	return float64(advFU) * ppem / float64(f.upem)
}

// GlyphVerticalAdvance returns the vertical advance height in pixels for a
// glyph, from the vmtx table when present. Falls back to OS/2-derived
// values (|sTypoAscender - sTypoDescender|) when vmtx is absent, matching
// FreeType's TT_Get_VMetrics fallback (ttgload.c:110-169).
func (f *ownParsedFont) GlyphVerticalAdvance(glyphIndex uint16, ppem float64) float64 {
	vm := f.ensureVmtx()
	if f.upem == 0 {
		return 0
	}
	if vm.parsed && len(vm.adv) > 0 {
		advFU := hmtxAdvance(vm.adv, len(vm.adv), glyphIndex)
		return float64(advFU) * ppem / float64(f.upem)
	}
	// Fallback: OS/2-derived advance height (same as FreeType when the
	// vertical table is missing).
	m := f.ensureMetrics()
	asc := int32(m.os2.sTypoAscender)
	desc := int32(m.os2.sTypoDescender)
	if asc == 0 && desc == 0 {
		asc = int32(m.hhea.ascent)
		desc = int32(m.hhea.descent)
	}
	adv := asc - desc
	if adv < 0 {
		adv = -adv
	}
	return float64(adv) * ppem / float64(f.upem)
}

// ensureVmtx lazily parses the vhea and vmtx tables for vertical metrics.
func (f *ownParsedFont) ensureVmtx() vmtxLazy {
	v, _ := f.vmtx.load(func() (vmtxLazy, error) {
		out := vmtxLazy{}
		vheaData, ok := f.tables["vhea"]
		if !ok {
			return out, nil
		}
		vhea, ok := parseVheaTable(vheaData)
		if !ok || vhea.numberOfVMetrics == 0 {
			return out, nil
		}
		out.vhea = vhea
		out.vheaOK = true // vheaOK is set even if vmtx is absent/broken

		vmtxData, ok := f.tables["vmtx"]
		if !ok {
			return out, nil
		}
		adv, tsb, err := parseVmtx(vmtxData, vhea.numberOfVMetrics, f.numGlyphs)
		if err != nil {
			return out, nil
		}
		out.adv = adv
		out.tsb = tsb
		out.parsed = true
		return out, nil
	})
	return v
}

// GlyphBounds implements ParsedFont.GlyphBounds.
// Returns the glyph bounding box scaled from font units to pixels.
//
// TrueType: glyf header bounds. CFF (no glyf): segment bounds via sfnt
// (ENGINE_GAPS G1.b).
func (f *ownParsedFont) GlyphBounds(glyphIndex uint16, ppem float64) Rect {
	if f.upem == 0 {
		return Rect{}
	}
	glyfData, ok := f.tables["glyf"]
	if !ok {
		if f.hasCFFTable() || f.hasCFF2Table() {
			return f.glyphBoundsCFF(glyphIndex, ppem)
		}
		return Rect{}
	}

	// Need loca to find the glyph offset.
	locaData, ok := f.tables["loca"]
	if !ok {
		return Rect{}
	}
	headData, ok := f.tables["head"]
	if !ok || len(headData) < 54 {
		return Rect{}
	}
	isLongLoca := binary.BigEndian.Uint16(headData[50:52]) != 0

	off, length := locateGlyph(locaData, int(glyphIndex), isLongLoca)
	if length == 0 {
		return Rect{} // empty glyph (space, etc.)
	}
	end := off + length
	if end > len(glyfData) {
		return Rect{}
	}

	data := glyfData[off:end]
	if len(data) < 10 {
		return Rect{}
	}

	// Glyph header:
	//   int16 numContours
	//   int16 xMin
	//   int16 yMin
	//   int16 xMax
	//   int16 yMax
	xMin := int16(binary.BigEndian.Uint16(data[2:4]))
	yMin := int16(binary.BigEndian.Uint16(data[4:6]))
	xMax := int16(binary.BigEndian.Uint16(data[6:8]))
	yMax := int16(binary.BigEndian.Uint16(data[8:10]))

	// The glyf header stores coordinates in Y-UP (font units).
	// sfnt.GlyphBounds returns Y-DOWN (Go image convention) by negating Y.
	// To match: negate Y and swap MinY/MaxY.
	scale := ppem / float64(f.upem)
	return Rect{
		MinX: float64(xMin) * scale,
		MinY: float64(-yMax) * scale, // Y-UP → Y-DOWN: negate and swap
		MaxX: float64(xMax) * scale,
		MaxY: float64(-yMin) * scale, // Y-UP → Y-DOWN: negate and swap
	}
}

// Metrics implements ParsedFont.Metrics.
func (f *ownParsedFont) Metrics(ppem float64) FontMetrics {
	m := f.ensureMetrics()
	if !m.hheaOK && !m.os2OK {
		return FontMetrics{}
	}
	vm := f.ensureVmtx()
	return computeFontMetrics(m.hhea, m.os2, f.upem, ppem, vm.vhea, vm.vheaOK)
}

// --- RawFontDataProvider ---

// RawFontData implements RawFontDataProvider, returning the raw font file
// bytes. This enables the contour-based auto-hinter and TT bytecode
// interpreter paths.
func (f *ownParsedFont) RawFontData() []byte {
	return f.rawData
}

// FontTables returns the parsed sfnt table map (shared, read-only).
// Used by glyf contour extraction to avoid re-parsing the directory.
func (f *ownParsedFont) FontTables() map[string][]byte {
	if f == nil {
		return nil
	}
	return f.tables
}

// GlyfContours extracts TrueType contours for gid using a cached glyf/loca view.
func (f *ownParsedFont) GlyfContours(gid GlyphID) (*GlyfContours, error) {
	if f == nil {
		return nil, fmt.Errorf("text: own parser: nil font")
	}
	cache := f.ensureGlyf()
	if cache != nil {
		if int(gid) >= cache.NumGlyphs() {
			return nil, fmt.Errorf("text: glyf parser: glyph ID %d out of range (font has %d glyphs)", gid, cache.NumGlyphs())
		}
		return cache.Contours(gid)
	}
	return ParseGlyfContoursFromTables(f.tables, gid)
}

// --- VariableAdvanceProvider ---

// GlyphAdvanceVar implements VariableAdvanceProvider.
// Returns the advance width in pixels adjusted by HVAR deltas for the
// given font variations.
func (f *ownParsedFont) GlyphAdvanceVar(glyphIndex uint16, ppem float64, variations []FontVariation) float64 {
	hvar := f.loadHVAR()
	axes := f.loadFvar()

	// Get base advance.
	baseAdvance := f.GlyphAdvance(glyphIndex, ppem)

	if hvar == nil || len(axes) == 0 || len(variations) == 0 {
		return baseAdvance
	}

	// Normalize variation coordinates.
	coords := normalizeCoords(axes, variations)

	// Get HVAR delta (in font units).
	delta := hvar.advanceDelta(glyphIndex, coords)
	if delta == 0 {
		return baseAdvance
	}

	// Scale delta from font units to pixels.
	if f.upem == 0 {
		return baseAdvance
	}
	scaledDelta := float64(delta) * ppem / float64(f.upem)
	return baseAdvance + scaledDelta
}

// --- Lazy initialization helpers ---
// Each ensure*/load* method runs its parse at most once via lazySlot and
// returns the typed result; a zero value marks the table absent/unparseable.

// ensureCmap lazily parses the cmap table.
func (f *ownParsedFont) ensureCmap() *cmapLookup {
	v, _ := f.cmap.load(func() (*cmapLookup, error) {
		cmapData, ok := f.tables["cmap"]
		if !ok {
			return nil, nil
		}
		return parseCmapTable(cmapData), nil
	})
	return v
}

// ensureHmtx lazily parses the hhea and hmtx tables for advance widths.
func (f *ownParsedFont) ensureHmtx() hmtxLazy {
	v, _ := f.hmtx.load(func() (hmtxLazy, error) {
		hheaData, ok := f.tables["hhea"]
		if !ok {
			return hmtxLazy{}, nil
		}
		hhea, ok := parseHheaTable(hheaData)
		if !ok || hhea.numberOfHMetrics == 0 {
			return hmtxLazy{}, nil
		}
		hmtxData, ok := f.tables["hmtx"]
		if !ok {
			return hmtxLazy{}, nil
		}
		advances, lsbs, err := parseHmtx(hmtxData, hhea.numberOfHMetrics, f.numGlyphs)
		if err != nil {
			return hmtxLazy{}, nil
		}
		return hmtxLazy{
			adv:         advances,
			lsb:         lsbs,
			numHMetrics: hhea.numberOfHMetrics,
			parsed:      true,
		}, nil
	})
	return v
}

// ensureName lazily parses the name table.
func (f *ownParsedFont) ensureName() nameLazy {
	v, _ := f.name.load(func() (nameLazy, error) {
		nameData, ok := f.tables["name"]
		if !ok {
			return nameLazy{}, nil
		}
		fam, full := parseNameTable(nameData)
		return nameLazy{family: fam, full: full}, nil
	})
	return v
}

// ensureMetrics lazily parses hhea and OS/2 tables for font-level metrics.
func (f *ownParsedFont) ensureMetrics() metricsLazy {
	v, _ := f.metrics.load(func() (metricsLazy, error) {
		var out metricsLazy
		if hheaData, ok := f.tables["hhea"]; ok {
			out.hhea, out.hheaOK = parseHheaTable(hheaData)
		}
		if os2Data, ok := f.tables["OS/2"]; ok {
			out.os2, out.os2OK = parseOS2Table(os2Data)
		}
		return out, nil
	})
	return v
}

// loadTTHintCache lazily initializes the TT bytecode hint cache.
// Thread-safe via lazySlot. Returns nil if the font has no TT instructions.
func (f *ownParsedFont) loadTTHintCache() *ttHintCache {
	v, _ := f.ttHint.load(func() (*ttHintCache, error) {
		if f.rawData == nil {
			return nil, nil
		}
		return newTTHintCache(f.rawData), nil
	})
	return v
}

// loadFvar lazily parses the fvar table to extract axis definitions.
// fvar axes are needed by both HVAR (advance deltas) and gvar (outline deltas),
// so they are parsed independently from either table.
//
// A font with gvar but no HVAR (e.g., Apple SFNS.ttf) still needs fvarAxes
// for normalizeCoords to produce the correct-length coordinate array.
func (f *ownParsedFont) loadFvar() []fvarAxis {
	v, _ := f.fvar.load(func() ([]fvarAxis, error) {
		fvarRaw, ok := f.tables["fvar"]
		if !ok {
			return nil, nil
		}
		return parseFvarAxes(fvarRaw), nil
	})
	return v
}

// loadHVAR lazily parses the HVAR table.
// Ensures fvar axes are also parsed (needed for coordinate normalization).
func (f *ownParsedFont) loadHVAR() *hvarTable {
	f.loadFvar()
	v, _ := f.hvar.load(func() (*hvarTable, error) {
		hvarRaw, ok := f.tables["HVAR"]
		if !ok {
			return nil, nil
		}
		hvar, err := parseHVAR(hvarRaw)
		if err != nil {
			return nil, nil
		}
		return hvar, nil
	})
	return v
}

// loadGvar lazily parses the gvar table.
func (f *ownParsedFont) loadGvar() *gvarTable {
	v, _ := f.gvar.load(func() (*gvarTable, error) {
		gvarRaw, ok := f.tables["gvar"]
		if !ok {
			return nil, nil
		}
		gvar, err := parseGvar(gvarRaw)
		if err != nil {
			return nil, nil
		}
		return gvar, nil
	})
	return v
}

// loadAvar lazily parses the avar table.
func (f *ownParsedFont) loadAvar() *avarTable {
	v, _ := f.avar.load(func() (*avarTable, error) {
		avarRaw, ok := f.tables["avar"]
		if !ok {
			return nil, nil
		}
		return parseAvar(avarRaw), nil
	})
	return v
}

// ensureGlyf lazily builds the cached glyf/loca contour parser.
// Returns nil when the cache cannot be built; callers fall back to
// ParseGlyfContoursFromTables (same behavior as the former inline Once).
func (f *ownParsedFont) ensureGlyf() *cachedGlyfParser {
	v, _ := f.glyf.load(func() (*cachedGlyfParser, error) {
		cache, err := newCachedGlyfParserFromTables(f.tables)
		if err != nil {
			return nil, err // fall back to ParseGlyfContoursFromTables
		}
		return cache, nil
	})
	return v
}

// applyVariations computes gvar deltas and applies them to the given
// outline points. Points are modified in-place.
//
// Parameters:
//   - glyphID: the glyph to look up in gvar
//   - points: outline points as [x, y] pairs (modified in-place)
//   - contourEnds: end-of-contour point indices
//   - variations: user-space variation settings (e.g., wght=700)
//
// The function normalizes coordinates, applies avar remapping, then
// computes gvar deltas and adds them to the points.
func (f *ownParsedFont) applyVariations(
	glyphID uint16,
	points [][2]int32,
	contourEnds []uint16,
	variations []FontVariation,
) {
	if len(variations) == 0 {
		return
	}

	axes := f.loadFvar()
	if len(axes) == 0 {
		return
	}

	gvar := f.loadGvar()
	if gvar == nil {
		return
	}

	// Normalize variation coordinates.
	coords := normalizeCoords(axes, variations)

	// Apply avar remapping.
	f.loadAvar().apply(coords)

	// Optimization: skip gvar delta computation when all normalized coords
	// are zero (default instance). This is the common case when the user
	// specifies e.g. wght=400 on a font where 400 is the default weight.
	// Avoids allocation + IUP computation for a zero-delta result.
	allZero := true
	for _, c := range coords {
		if c != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return
	}

	// Total outline points (without phantom points).
	numPoints := len(points) - 4
	if numPoints < 0 {
		return
	}

	// Compute gvar deltas.
	dx, dy := gvar.glyphVariationDeltas(glyphID, coords, numPoints, contourEnds, points)
	if dx == nil || dy == nil {
		return
	}

	// Apply deltas to points.
	for i := range points {
		if i < len(dx) {
			points[i][0] += dx[i]
		}
		if i < len(dy) {
			points[i][1] += dy[i]
		}
	}
}

// locateGlyph returns the byte offset and length of a glyph within the glyf
// table, using the loca table for lookup.
func locateGlyph(locaData []byte, glyphIndex int, isLong bool) (offset, length int) {
	if isLong {
		// Long format: uint32 offsets.
		pos := glyphIndex * 4
		nextPos := pos + 4
		if nextPos+4 > len(locaData) {
			return 0, 0
		}
		start := int(binary.BigEndian.Uint32(locaData[pos : pos+4]))
		end := int(binary.BigEndian.Uint32(locaData[nextPos : nextPos+4]))
		if end > start {
			return start, end - start
		}
		return 0, 0
	}

	// Short format: uint16 offsets * 2.
	pos := glyphIndex * 2
	nextPos := pos + 2
	if nextPos+2 > len(locaData) {
		return 0, 0
	}
	start := int(binary.BigEndian.Uint16(locaData[pos:pos+2])) * 2
	end := int(binary.BigEndian.Uint16(locaData[nextPos:nextPos+2])) * 2
	if end > start {
		return start, end - start
	}
	return 0, 0
}

func init() {
	// Register the own parser alongside ximage.
	// Users can select it with WithParser("own").
	RegisterParser("own", &ownParser{})
}
