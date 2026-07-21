// OwnShaper — Pure Go text shaper with GSUB/GPOS support.
//
// OwnShaper implements the Shaper interface with direct binary parsing of
// OpenType GSUB and GPOS tables, replacing the legacy GoTextShaper
// as part of the Pure Go Font Stack (ADR-048, Phase 5).
//
// Shaping pipeline:
//  1. Character → Glyph (cmap lookup via ownParsedFont)
//  2. GSUB substitution (ligatures, single/multiple/alternate substitution)
//  3. GPOS positioning (kerning, single adjustment)
//  4. kern table fallback (if GPOS has no 'kern' feature)
//  5. Scaling (font units → pixels)
//
// Supported features:
//   - GSUB: single (Type 1), multiple (Type 2), alternate (Type 3),
//     ligature (Type 4), contextual (Type 5), chaining (Type 6),
//     extension (Type 7), reverse chaining (Type 8)
//   - GPOS: single (Type 1), pair/kerning (Type 2), cursive (Type 3),
//     mark-to-base (Type 4), mark-to-ligature (Type 5), mark-to-mark (Type 6),
//     contextual (Type 7), chaining (Type 8), extension (Type 9)
//   - kern: format 0 fallback pairs
//   - Default features: 'liga' (ligatures), 'kern' (kerning)
//
// OwnShaper is safe for concurrent use. Parsed table caches are protected
// by sync.Once (per FontSource) and sync.RWMutex (for the cache map).
//
// This file is part of Phase 5 (ADR-048: Pure Go Font Stack).
package text

import "sync"

// OwnShaper provides text shaping using Pure Go GSUB/GPOS parsing.
// It supports ligature substitution, kerning, and other OpenType features
// without external dependencies.
//
// OwnShaper caches parsed GSUB/GPOS/kern tables per FontSource. The
// cached data is read-only and safe for concurrent use.
type OwnShaper struct {
	mu    sync.RWMutex
	cache map[*FontSource]*ownShaperCache
}

// ownShaperCache holds parsed shaping tables for a single font.
type ownShaperCache struct {
	gsub     *gsubTable // nil if font has no GSUB
	gpos     *gposTable // nil if font has no GPOS
	gdef     *gdefTable // nil if font has no GDEF (lookup flags degrade gracefully)
	kern     *kernTable // nil if font has no kern
	hasGPOS  bool       // true if GPOS was found (even if no kern feature)
	upem     int        // units per em
	cmap     *cmapLookup
	hmtxAdv  []uint16
	numHMtx  int
	numGlyph int

	// Per-script Indic font positional classes (blwf/pstf/… coverage).
	// Lazy map key = scriptTag||langTag; built once per (font, script, lang).
	fontPosMu sync.Mutex
	fontPos   map[[8]byte]*indicFontPosClasses
}

// NewOwnShaper creates a new OwnShaper.
func NewOwnShaper() *OwnShaper {
	return &OwnShaper{
		cache: make(map[*FontSource]*ownShaperCache),
	}
}

// indicFontPos returns cached per-font positional classes for script/lang.
func (sc *ownShaperCache) indicFontPos(scriptTag, langTag [4]byte) *indicFontPosClasses {
	if sc == nil || sc.gsub == nil {
		return nil
	}
	var key [8]byte
	copy(key[0:4], scriptTag[:])
	copy(key[4:8], langTag[:])
	sc.fontPosMu.Lock()
	defer sc.fontPosMu.Unlock()
	if sc.fontPos == nil {
		sc.fontPos = make(map[[8]byte]*indicFontPosClasses)
	}
	if fp, ok := sc.fontPos[key]; ok {
		return fp
	}
	fp := buildIndicFontPosClasses(sc.gsub, scriptTag, langTag)
	sc.fontPos[key] = fp // may store nil = "no classes"
	return fp
}

// Shape implements the Shaper interface.
// It converts text into positioned glyphs using Pure Go GSUB/GPOS shaping.
// The font size is obtained from face.Size().
func (s *OwnShaper) Shape(text string, face Face) []ShapedGlyph {
	if text == "" || face == nil {
		return nil
	}

	source := face.Source()
	if source == nil {
		return nil
	}

	parsed := source.Parsed()
	if parsed == nil {
		return nil
	}

	sc := s.getOrCreateCache(source)
	if sc == nil {
		return nil
	}

	size := face.Size()
	runes := []rune(text)
	indic := needsIndicShaping(runes)

	// Step 1: Character → Glyph (cmap lookup).
	// Indic: initial reorder (reph after base) while preserving source clusters.
	var glyphs []shapingGlyph
	if indic {
		units := reorderIndicInitial(runes)
		glyphs = unitsToGlyphs(units, sc)
	} else {
		glyphs = runeToGlyphs(runes, sc)
	}

	// Step 2: Determine script and language tags.
	scriptTag := detectOTScriptTag(runes)
	langTag := parseLangTag(face.Language())

	// Step 3: Determine which features to apply.
	desiredGSUB, desiredGPOS := collectDesiredFeatures(face.Features())

	// Step 4: Apply GSUB substitutions.
	// Script-aware staging (ENGINE_GAPS G1.c):
	//   Arabic → isol/fina/medi/init masks
	//   Indic  → rphf/half/vatu/pres… stage order
	if sc.gsub != nil && len(desiredGSUB) > 0 {
		switch {
		case needsArabicJoining(runes):
			glyphs = sc.gsub.applyGSUBStagedArabic(glyphs, runes, scriptTag, langTag, desiredGSUB)
		case indic:
			glyphs = sc.gsub.applyGSUBStagedIndic(glyphs, scriptTag, langTag, desiredGSUB)
		default:
			glyphs = sc.gsub.applyGSUB(glyphs, scriptTag, langTag, desiredGSUB)
		}
	}

	// Step 5: Apply GPOS positioning.
	var adjustments []gposAdjustment
	if sc.gpos != nil && len(desiredGPOS) > 0 {
		metrics := gposMetrics{hmtxAdv: sc.hmtxAdv, numHMetrics: sc.numHMtx}
		adjustments = sc.gpos.applyGPOS(glyphs, scriptTag, langTag, desiredGPOS, metrics)
	}

	// Step 5b: Indic final reorder (pre-base matra / reph) before kern merge.
	// Adjustments move with glyphs by index — reorder glyphs+adj together.
	if indic {
		// Prefer per-font blwf/pstf/rkrf/vatu coverage classes (cached).
		fontPos := sc.indicFontPos(scriptTag, langTag)
		glyphs, adjustments = reorderIndicGlyphsWithAdjFont(glyphs, adjustments, runes, fontPos)
	}

	// Step 6: kern table fallback (only if GPOS has no kern feature).
	var kernFallback bool
	if sc.kern != nil && !gposHasKern(sc.gpos, scriptTag, langTag) {
		kernFallback = true
	}

	// Step 7: Build positioned glyph output.
	out := buildShapedGlyphs(glyphs, adjustments, sc, size, runes, kernFallback)

	// Step 8: RTL visual reorder (ENGINE_GAPS G1.c BiDi paint order).
	if shouldReorderRTL(face.Direction(), runes) {
		out = ReorderRTLShapedGlyphs(out)
	}
	return out
}

// ClearCache removes all cached shaping data.
func (s *OwnShaper) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[*FontSource]*ownShaperCache)
}

// RemoveSource removes cached data for a specific FontSource.
func (s *OwnShaper) RemoveSource(source *FontSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, source)
}

// getOrCreateCache returns the cached shaping tables for the given font source.
func (s *OwnShaper) getOrCreateCache(source *FontSource) *ownShaperCache {
	// Fast path: read lock.
	s.mu.RLock()
	if sc, ok := s.cache[source]; ok {
		s.mu.RUnlock()
		return sc
	}
	s.mu.RUnlock()

	// Slow path: parse and cache.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check.
	if sc, ok := s.cache[source]; ok {
		return sc
	}

	sc := buildShaperCache(source)
	s.cache[source] = sc
	return sc
}

// buildShaperCache parses GSUB, GPOS, and kern tables from the font data.
func buildShaperCache(source *FontSource) *ownShaperCache {
	sc := &ownShaperCache{}

	parsed := source.Parsed()
	if parsed == nil {
		return sc
	}

	sc.upem = parsed.UnitsPerEm()
	sc.numGlyph = parsed.NumGlyphs()

	// Try to get raw table data. The own parser stores tables directly;
	// for ximage parser we need the RawFontDataProvider interface.
	tables := getRawTables(source)
	if tables == nil {
		return sc
	}

	// Parse cmap.
	if cmapData, ok := tables["cmap"]; ok {
		sc.cmap = parseCmapTable(cmapData)
	}

	// Parse hmtx (for advance widths).
	parseHmtxFromTables(tables, sc)

	// Parse GDEF first so GSUB/GPOS can honor lookup flags (IgnoreMarks…).
	if gdefData, ok := tables["GDEF"]; ok {
		sc.gdef = parseGDEF(gdefData)
	}

	// Parse GSUB.
	if gsubData, ok := tables["GSUB"]; ok {
		sc.gsub = parseGSUB(gsubData)
		if sc.gsub != nil {
			sc.gsub.gdef = sc.gdef
		}
	}

	// Parse GPOS.
	if gposData, ok := tables["GPOS"]; ok {
		sc.gpos = parseGPOS(gposData)
		sc.hasGPOS = sc.gpos != nil
		if sc.gpos != nil {
			sc.gpos.gdef = sc.gdef
		}
	}

	// Parse kern (fallback).
	if kernData, ok := tables["kern"]; ok {
		sc.kern = parseKern(kernData)
	}

	return sc
}

// parseHmtxFromTables parses the hhea and hmtx tables into the shaper cache.
// Extracted to avoid deep nesting (nestif linter).
func parseHmtxFromTables(tables map[string][]byte, sc *ownShaperCache) {
	hheaData, ok := tables["hhea"]
	if !ok {
		return
	}
	hhea, hheaOK := parseHheaTable(hheaData)
	if !hheaOK || hhea.numberOfHMetrics == 0 {
		return
	}
	hmtxData, ok := tables["hmtx"]
	if !ok {
		return
	}
	adv, _, err := parseHmtx(hmtxData, hhea.numberOfHMetrics, sc.numGlyph)
	if err != nil {
		return
	}
	sc.hmtxAdv = adv
	sc.numHMtx = hhea.numberOfHMetrics
}

// getRawTables extracts raw table data from a FontSource.
// Prefers the own parser's direct table map; falls back to re-parsing
// raw font data if available via RawFontDataProvider.
func getRawTables(source *FontSource) map[string][]byte {
	parsed := source.Parsed()

	// Own parser: tables are directly available.
	if own, ok := parsed.(*ownParsedFont); ok {
		return own.tables
	}

	// RawFontDataProvider: re-parse table directory.
	if provider, ok := parsed.(RawFontDataProvider); ok {
		rawData := provider.RawFontData()
		if rawData != nil {
			tables, err := parseFontTables(rawData)
			if err == nil {
				return tables
			}
		}
	}

	return nil
}

// runeToGlyphs converts runes to glyph entries using the cmap.
func runeToGlyphs(runes []rune, sc *ownShaperCache) []shapingGlyph {
	glyphs := make([]shapingGlyph, 0, len(runes))
	for i, r := range runes {
		// Skip non-tab control characters.
		if r < 0x20 && r != '\t' {
			continue
		}
		var gid uint16
		if r == '\t' {
			// Map tab to space glyph.
			if sc.cmap != nil {
				gid = sc.cmap.glyphIndex(' ')
			}
		} else if sc.cmap != nil {
			gid = sc.cmap.glyphIndex(r)
		}
		glyphs = append(glyphs, shapingGlyph{gid: gid, cluster: i})
	}
	return glyphs
}

// unitsToGlyphs maps reordered Indic units to glyphs with original cluster indices.
func unitsToGlyphs(units []indicUnit, sc *ownShaperCache) []shapingGlyph {
	glyphs := make([]shapingGlyph, 0, len(units))
	for _, u := range units {
		r := u.r
		if r < 0x20 && r != '	' {
			continue
		}
		var gid uint16
		if r == '	' {
			if sc.cmap != nil {
				gid = sc.cmap.glyphIndex(' ')
			}
		} else if sc.cmap != nil {
			gid = sc.cmap.glyphIndex(r)
		}
		glyphs = append(glyphs, shapingGlyph{gid: gid, cluster: u.orig})
	}
	return glyphs
}

// reorderIndicGlyphsWithAdj final-reorders glyphs and permutes GPOS adjustments.
func reorderIndicGlyphsWithAdj(glyphs []shapingGlyph, adj []gposAdjustment, runes []rune) ([]shapingGlyph, []gposAdjustment) {
	return reorderIndicGlyphsWithAdjFont(glyphs, adj, runes, nil)
}

func reorderIndicGlyphsWithAdjFont(glyphs []shapingGlyph, adj []gposAdjustment, runes []rune, fontPos *indicFontPosClasses) ([]shapingGlyph, []gposAdjustment) {
	if len(glyphs) == 0 {
		return glyphs, adj
	}
	reordered := reorderIndicFinalGlyphsFont(append([]shapingGlyph(nil), glyphs...), runes, fontPos)
	type key struct {
		gid     uint16
		cluster int
	}
	queues := make(map[key][]int)
	for i, g := range glyphs {
		k := key{g.gid, g.cluster}
		queues[k] = append(queues[k], i)
	}
	newGlyphs := make([]shapingGlyph, len(reordered))
	newAdj := make([]gposAdjustment, len(reordered))
	for j, g := range reordered {
		k := key{g.gid, g.cluster}
		q := queues[k]
		if len(q) == 0 {
			newGlyphs[j] = g
			continue
		}
		oi := q[0]
		queues[k] = q[1:]
		newGlyphs[j] = glyphs[oi]
		if oi < len(adj) {
			newAdj[j] = adj[oi]
		}
	}
	return newGlyphs, newAdj
}

// collectDesiredFeatures determines which GSUB and GPOS feature tags to apply.
// User features can enable/disable individual features.
//
// Default GSUB features: ccmp, liga, clig, rlig, dlig, calt, locl,
// plus Arabic/Indic joining & forms used by complex scripts (G1.c).
//
// Why 'dlig': Some major fonts (e.g. Times New Roman) place standard Latin
// ligatures (fi, fl, ffi) under 'dlig' rather than 'liga'. Without 'dlig',
// these common ligatures would not be applied. Microsoft DirectWrite and
// most desktop applications enable these ligatures by default.
// Users who want strictly HarfBuzz-compatible behavior can disable 'dlig'
// explicitly with text.NoDLigatures.
//
// Default GPOS features: kern, mark, mkmk, curs.
func collectDesiredFeatures(userFeatures []FontFeature) (gsubTags, gposTags [][4]byte) {
	// Default features.
	ccmp := [4]byte{'c', 'c', 'm', 'p'}
	liga := [4]byte{'l', 'i', 'g', 'a'}
	kern := [4]byte{'k', 'e', 'r', 'n'}
	clig := [4]byte{'c', 'l', 'i', 'g'}
	rlig := [4]byte{'r', 'l', 'i', 'g'}
	dlig := [4]byte{'d', 'l', 'i', 'g'}
	calt := [4]byte{'c', 'a', 'l', 't'}
	locl := [4]byte{'l', 'o', 'c', 'l'}
	// Arabic / complex joining & presentation forms.
	init := [4]byte{'i', 'n', 'i', 't'}
	medi := [4]byte{'m', 'e', 'd', 'i'}
	fina := [4]byte{'f', 'i', 'n', 'a'}
	isol := [4]byte{'i', 's', 'o', 'l'}
	// Indic basic + presentation (staged in applyGSUBStagedIndic).
	nukt := [4]byte{'n', 'u', 'k', 't'}
	akhn := [4]byte{'a', 'k', 'h', 'n'}
	rphf := [4]byte{'r', 'p', 'h', 'f'}
	rkrf := [4]byte{'r', 'k', 'r', 'f'}
	pref := [4]byte{'p', 'r', 'e', 'f'}
	blwf := [4]byte{'b', 'l', 'w', 'f'}
	abvf := [4]byte{'a', 'b', 'v', 'f'}
	half := [4]byte{'h', 'a', 'l', 'f'}
	pstf := [4]byte{'p', 's', 't', 'f'}
	vatu := [4]byte{'v', 'a', 't', 'u'}
	cjct := [4]byte{'c', 'j', 'c', 't'}
	pres := [4]byte{'p', 'r', 'e', 's'}
	abvs := [4]byte{'a', 'b', 'v', 's'}
	blws := [4]byte{'b', 'l', 'w', 's'}
	psts := [4]byte{'p', 's', 't', 's'}
	haln := [4]byte{'h', 'a', 'l', 'n'}
	// Mark / cursive positioning.
	mark := [4]byte{'m', 'a', 'r', 'k'}
	mkmk := [4]byte{'m', 'k', 'm', 'k'}
	curs := [4]byte{'c', 'u', 'r', 's'}

	// GSUB defaults.
	gsubEnabled := map[[4]byte]bool{
		ccmp: true,
		liga: true,
		clig: true,
		rlig: true,
		dlig: true,
		calt: true,
		locl: true,
		init: true,
		medi: true,
		fina: true,
		isol: true,
		nukt: true, akhn: true, rphf: true, rkrf: true,
		pref: true, blwf: true, abvf: true, half: true, pstf: true,
		vatu: true, cjct: true,
		pres: true, abvs: true, blws: true, psts: true, haln: true,
	}

	// GPOS defaults.
	gposEnabled := map[[4]byte]bool{
		kern: true,
		mark: true,
		mkmk: true,
		curs: true,
	}

	// Apply user overrides.
	for _, f := range userFeatures {
		tag := f.Tag
		if f.Value == 0 {
			// Disable.
			delete(gsubEnabled, tag)
			delete(gposEnabled, tag)
		} else {
			// Enable — add to the appropriate category.
			if isGSUBFeature(tag) {
				gsubEnabled[tag] = true
			} else {
				gposEnabled[tag] = true
			}
		}
	}

	gsubTags = make([][4]byte, 0, len(gsubEnabled))
	for tag := range gsubEnabled {
		gsubTags = append(gsubTags, tag)
	}
	gposTags = make([][4]byte, 0, len(gposEnabled))
	for tag := range gposEnabled {
		gposTags = append(gposTags, tag)
	}
	return gsubTags, gposTags
}

// isGSUBFeature returns true if the feature tag is typically a GSUB feature.
func isGSUBFeature(tag [4]byte) bool {
	gsubFeatures := map[[4]byte]bool{
		{'c', 'c', 'm', 'p'}: true,
		{'l', 'i', 'g', 'a'}: true,
		{'c', 'l', 'i', 'g'}: true,
		{'r', 'l', 'i', 'g'}: true,
		{'d', 'l', 'i', 'g'}: true,
		{'s', 'm', 'c', 'p'}: true,
		{'c', '2', 's', 'c'}: true,
		{'s', 'w', 's', 'h'}: true,
		{'s', 'a', 'l', 't'}: true,
		{'c', 'a', 'l', 't'}: true,
		{'l', 'o', 'c', 'l'}: true,
		{'i', 'n', 'i', 't'}: true,
		{'m', 'e', 'd', 'i'}: true,
		{'f', 'i', 'n', 'a'}: true,
		{'i', 's', 'o', 'l'}: true,
		// Indic
		{'n', 'u', 'k', 't'}: true, {'a', 'k', 'h', 'n'}: true,
		{'r', 'p', 'h', 'f'}: true, {'r', 'k', 'r', 'f'}: true,
		{'p', 'r', 'e', 'f'}: true, {'b', 'l', 'w', 'f'}: true,
		{'a', 'b', 'v', 'f'}: true, {'h', 'a', 'l', 'f'}: true,
		{'p', 's', 't', 'f'}: true, {'v', 'a', 't', 'u'}: true,
		{'c', 'j', 'c', 't'}: true, {'c', 'f', 'a', 'r'}: true,
		{'p', 'r', 'e', 's'}: true, {'a', 'b', 'v', 's'}: true,
		{'b', 'l', 'w', 's'}: true, {'p', 's', 't', 's'}: true,
		{'h', 'a', 'l', 'n'}: true,
	}
	return gsubFeatures[tag]
}

// gposHasKern checks whether the GPOS table has a 'kern' feature
// available for the given script and language.
func gposHasKern(gpos *gposTable, scriptTag, langTag [4]byte) bool {
	if gpos == nil {
		return false
	}
	kernTag := [4]byte{'k', 'e', 'r', 'n'}
	indices := collectLookupIndices(gpos.scripts, gpos.features, scriptTag, langTag, [][4]byte{kernTag})
	return len(indices) > 0
}

// buildShapedGlyphs converts internal glyph entries + adjustments to ShapedGlyph output.
func buildShapedGlyphs(
	glyphs []shapingGlyph,
	adjustments []gposAdjustment,
	sc *ownShaperCache,
	size float64,
	runes []rune,
	kernFallback bool,
) []ShapedGlyph {
	if len(glyphs) == 0 {
		return nil
	}

	scale := size / float64(sc.upem)
	result := make([]ShapedGlyph, len(glyphs))
	var x, y float64

	for i := range glyphs {
		ge := &glyphs[i]

		// Get base advance (font units).
		var advFU uint16
		if sc.hmtxAdv != nil {
			advFU = hmtxAdvance(sc.hmtxAdv, sc.numHMtx, ge.gid)
		}

		// Tab handling: use tab-stop advance.
		if ge.cluster < len(runes) && runes[ge.cluster] == '\t' {
			tabGID, tabAdv := ownTabAdvance(sc, size)
			ge.gid = tabGID
			result[i] = ShapedGlyph{
				GID:      GlyphID(ge.gid),
				Cluster:  ge.cluster,
				X:        x,
				Y:        y,
				XAdvance: tabAdv,
			}
			x += tabAdv
			continue
		}

		// GPOS adjustments.
		var adj gposAdjustment
		if i < len(adjustments) {
			adj = adjustments[i]
		}

		// kern table fallback.
		if kernFallback && sc.kern != nil && i+1 < len(glyphs) {
			kv := sc.kern.kernValue(ge.gid, glyphs[i+1].gid)
			if kv != 0 {
				adj.xAdvance += kv
			}
		}

		// Scale to pixels.
		xAdv := float64(advFU)*scale + float64(adj.xAdvance)*scale
		yAdv := float64(adj.yAdvance) * scale
		xOff := float64(adj.xPlacement) * scale
		yOff := float64(adj.yPlacement) * scale

		var cjk bool
		if ge.cluster < len(runes) {
			cjk = IsCJKRune(runes[ge.cluster])
		}

		result[i] = ShapedGlyph{
			GID:      GlyphID(ge.gid),
			Cluster:  ge.cluster,
			X:        x + xOff,
			Y:        y + yOff,
			XAdvance: xAdv,
			YAdvance: yAdv,
			IsCJK:    cjk,
		}

		x += xAdv
		y += yAdv
	}

	return result
}

// ownTabAdvance returns the space glyph ID and tab-stop advance for a font
// using the cached shaper data (without going through ParsedFont interface).
func ownTabAdvance(sc *ownShaperCache, size float64) (uint16, float64) {
	var spaceGID uint16
	if sc.cmap != nil {
		spaceGID = sc.cmap.glyphIndex(' ')
	}

	var spaceAdvFU uint16
	if sc.hmtxAdv != nil {
		spaceAdvFU = hmtxAdvance(sc.hmtxAdv, sc.numHMtx, spaceGID)
	}

	if sc.upem == 0 {
		return spaceGID, 0
	}
	spaceAdv := float64(spaceAdvFU) * size / float64(sc.upem)
	return spaceGID, spaceAdv * float64(globalTabWidth)
}

// --- Script detection ---

// detectOTScriptTag returns the OpenType script tag for the dominant
// script in the given runes. Falls back to 'latn' (Latin).
func detectOTScriptTag(runes []rune) [4]byte {
	for _, r := range runes {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		return runeToOTScript(r)
	}
	return [4]byte{'l', 'a', 't', 'n'} // Latin
}

// scriptRange maps a Unicode code point range to an OpenType script tag.
type scriptRange struct {
	lo, hi rune
	tag    [4]byte
}

// otScriptRanges maps Unicode ranges to OpenType script tags.
// Ranges must be non-overlapping and sorted by lo. The first match wins.
// This covers the most common scripts; complex scripts will need
// more detailed mapping in the future.
var otScriptRanges = []scriptRange{
	{0x0370, 0x03FF, [4]byte{'g', 'r', 'e', 'k'}}, // Greek
	{0x0400, 0x04FF, [4]byte{'c', 'y', 'r', 'l'}}, // Cyrillic
	{0x0590, 0x05FF, [4]byte{'h', 'e', 'b', 'r'}}, // Hebrew
	{0x0600, 0x06FF, [4]byte{'a', 'r', 'a', 'b'}}, // Arabic
	{0x0900, 0x097F, [4]byte{'d', 'e', 'v', '2'}}, // Devanagari
	{0x3000, 0x30FF, [4]byte{'k', 'a', 'n', 'a'}}, // Hiragana + Katakana
	{0x3100, 0x9FFF, [4]byte{'h', 'a', 'n', 'i'}}, // CJK
	{0xAC00, 0xD7AF, [4]byte{'h', 'a', 'n', 'g'}}, // Hangul
}

// runeToOTScript maps a rune to its OpenType script tag.
func runeToOTScript(r rune) [4]byte {
	latn := [4]byte{'l', 'a', 't', 'n'}
	if r <= 0x024F {
		return latn // Latin (Basic + Extended)
	}
	for i := range otScriptRanges {
		sr := &otScriptRanges[i]
		if r >= sr.lo && r <= sr.hi {
			return sr.tag
		}
	}
	return latn
}

// parseLangTag converts a BCP 47 language tag (e.g. "en") to an OpenType
// language system tag. The OpenType spec uses uppercase 4-byte tags with
// trailing space padding. For simplicity, we return a zero tag (which
// will match the default LangSys) for most languages.
func parseLangTag(lang string) [4]byte {
	// Map common languages to OpenType language tags.
	// Most fonts only define a default LangSys, so an empty tag is fine.
	switch lang {
	case "tr":
		return [4]byte{'T', 'R', 'K', ' '}
	case "az":
		return [4]byte{'A', 'Z', 'E', ' '}
	case "ro":
		return [4]byte{'R', 'O', 'M', ' '}
	case "nl":
		return [4]byte{'N', 'L', 'D', ' '}
	default:
		// For most languages, the default LangSys is used.
		// Return a zero tag — collectLookupIndices will fall back to defaultLan.
		return [4]byte{}
	}
}

// init is intentionally empty — OwnShaper is now the default shaper,
// initialized via defaultOwnShaper in shaper.go (ADR-048 Phase 6).
