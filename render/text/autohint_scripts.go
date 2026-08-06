package text

import (
	"fmt"
	"sync"
)

// Script detection and classification for the auto-hinter.
//
// The auto-hinter operates on a per-script basis: different scripts have
// different reference characters for measuring stem widths and defining
// blue zones. This file implements script detection and defines the
// script-specific character lists.
//
// Architecture (matching FreeType):
//
//	Font loaded → per-glyph script assignment via unicode ranges (afglobal.c)
//	             → per-script blues + widths
//
// Three script groups determine the algorithm:
//   - Default (Latin, Hebrew, Arabic, Greek, Cyrillic, ...) → computeDefaultBlues
//   - CJK (HANI) → computeCJKBlues
//   - Indic → no blues (skip)
//
// References:
//   - FreeType afscript.h — script definitions
//   - FreeType afblue.dat — blue zone reference characters
//   - skrifa style.rs — ScriptClass, ScriptGroup, SCRIPT_CLASSES
//   - skrifa generated/generated_autohint_styles.rs — script data

// scriptGroup determines which hinting algorithm to use.
type scriptGroup int

const (
	// scriptGroupDefault handles Latin, Hebrew, Arabic, Greek, Cyrillic,
	// and most other scripts. Uses computeDefaultBlues.
	scriptGroupDefault scriptGroup = iota

	// scriptGroupCJK handles CJK ideographs (HANI). Uses computeCJKBlues.
	scriptGroupCJK

	// scriptGroupIndic handles Indic scripts. No blue zones (yet).
	scriptGroupIndic
)

// blueSpec defines a blue zone by its reference characters and flags.
// Matches skrifa's (blue_str, BlueZones) tuple in ScriptClass.blues.
type blueSpec struct {
	// chars contains space-separated reference characters to measure.
	// For CJK, the '|' separator divides fill chars from flat chars.
	chars string

	// flags defines the zone properties (TOP, LONG, X_HEIGHT, etc.).
	flags blueZoneFlags
}

// scriptClass defines the hinting properties for a script.
// Matches skrifa's ScriptClass struct.
type scriptClass struct {
	// name is the human-readable script name (for diagnostics).
	name string

	// group determines the hinting algorithm.
	group scriptGroup

	// stdChars lists characters for standard stem width measurement.
	// The first character with a glyph in the font is used.
	stdChars []rune

	// hintTopToBottom marks scripts whose blues are measured top-to-bottom
	// (skrifa ScriptClass.hint_top_to_bottom: Bengali, Devanagari, Gothic,
	// Gurmukhi, Mongolian). Data is generated; the algorithm is not yet
	// wired into autohint_edges.go (known difference, see docs).
	hintTopToBottom bool

	// uniranges lists Unicode code point ranges covered by this script.
	// Used for per-glyph script detection (FreeType afglobal.c
	// af_face_globals_compute_style_coverage): each glyph is assigned
	// the FIRST script whose range contains one of its cmap code points.
	// Pairs are (first, last) inclusive. Empty means no per-glyph
	// coverage (fallback only).
	uniranges []runePair

	// blues defines blue zone specifications for this script.
	blues []blueSpec
}

// runePair is an inclusive Unicode code point range [first, last].
type runePair struct {
	first, last rune
}

// uniRange builds a runePair from two integer literals.
func uniRange(first, last rune) runePair { return runePair{first, last} }

// Script class definitions, matching skrifa generated_autohint_styles.rs.
// Only scripts we actively test and support are defined here. Additional
// scripts can be added following the same pattern.

var scriptLatin = scriptClass{
	name:     "Latin",
	group:    scriptGroupDefault,
	stdChars: []rune{'o', 'O', '0'},
	// af_latn_uniranges (FreeType afranges.c).
	uniranges: []runePair{
		uniRange(0x0020, 0x007F), uniRange(0x00A0, 0x00A9),
		uniRange(0x00AB, 0x00B1), uniRange(0x00B4, 0x00B8),
		uniRange(0x00BB, 0x00FF), uniRange(0x0100, 0x017F),
		uniRange(0x0180, 0x024F), uniRange(0x0250, 0x02AF),
		uniRange(0x02B9, 0x02DF), uniRange(0x02E5, 0x02FF),
		uniRange(0x0300, 0x036F), uniRange(0x1AB0, 0x1ABE),
		uniRange(0x1D00, 0x1D2B), uniRange(0x1D6B, 0x1D77),
		uniRange(0x1D79, 0x1D7F), uniRange(0x1D80, 0x1D9A),
		uniRange(0x1DC0, 0x1DFF), uniRange(0x1E00, 0x1EFF),
		uniRange(0x2000, 0x206F), uniRange(0x20A0, 0x20B8),
		uniRange(0x20BA, 0x20CF), uniRange(0x2150, 0x218F),
		uniRange(0x2C60, 0x2C7B), uniRange(0x2C7E, 0x2C7F),
		uniRange(0x2E00, 0x2E7F), uniRange(0xA720, 0xA76F),
		uniRange(0xA771, 0xA7F7), uniRange(0xA7FA, 0xA7FF),
		uniRange(0xAB30, 0xAB5B), uniRange(0xAB60, 0xAB6F),
		uniRange(0xFB00, 0xFB06), uniRange(0x1D400, 0x1D7FF),
	},
	blues: []blueSpec{
		{chars: "T H E Z O C Q S", flags: blueZoneTop},
		{chars: "H E Z L O C U S", flags: 0},
		{chars: "f i j k d b h", flags: blueZoneTop},
		{chars: "u v x z o e s c", flags: blueZoneTop | blueZoneXHeight},
		{chars: "n r x z o e s c", flags: 0},
		{chars: "p q g j y", flags: 0},
	},
}

var scriptHebrew = scriptClass{
	name:     "Hebrew",
	group:    scriptGroupDefault,
	stdChars: []rune{'\u05DD'}, // Final Mem (ם)
	// af_hebr_uniranges.
	uniranges: []runePair{
		uniRange(0x0590, 0x05FF), uniRange(0xFB1D, 0xFB4F),
	},
	blues: []blueSpec{
		{chars: "\u05D1 \u05D3 \u05D4 \u05D7 \u05DA \u05DB \u05DD \u05E1", flags: blueZoneTop | blueZoneLong},
		{chars: "\u05D1 \u05D8 \u05DB \u05DD \u05E1 \u05E6", flags: 0},
		{chars: "\u05E7 \u05DA \u05DF \u05E3 \u05E5", flags: 0},
	},
}

var scriptCyrillic = scriptClass{
	name:     "Cyrillic",
	group:    scriptGroupDefault,
	stdChars: []rune{'\u043E', '\u041E'}, // о О
	// af_cyrl_uniranges.
	uniranges: []runePair{
		uniRange(0x0400, 0x04FF), uniRange(0x0500, 0x052F),
		uniRange(0x2DE0, 0x2DFF), uniRange(0xA640, 0xA69F),
		uniRange(0x1C80, 0x1C8F),
	},
	blues: []blueSpec{
		{chars: "\u0411 \u0412 \u0415 \u041F \u0417 \u041E \u0421 \u042D", flags: blueZoneTop},
		{chars: "\u0411 \u0412 \u0415 \u0428 \u0417 \u041E \u0421 \u042D", flags: 0},
		{chars: "\u0445 \u043F \u043D \u0448 \u0435 \u0437 \u043E \u0441", flags: blueZoneTop | blueZoneXHeight},
		{chars: "\u0445 \u043F \u043D \u0448 \u0435 \u0437 \u043E \u0441", flags: 0},
		{chars: "\u0440 \u0443 \u0444", flags: 0},
	},
}

var scriptGreek = scriptClass{
	name:     "Greek",
	group:    scriptGroupDefault,
	stdChars: []rune{'\u03BF', '\u039F'}, // ο Ο
	// af_grek_uniranges.
	uniranges: []runePair{
		uniRange(0x0370, 0x03FF), uniRange(0x1F00, 0x1FFF),
	},
	blues: []blueSpec{
		{chars: "\u0393 \u0392 \u0395 \u0396 \u0398 \u039F \u03A9", flags: blueZoneTop},
		{chars: "\u0392 \u0394 \u0396 \u039E \u0398 \u039F", flags: 0},
		{chars: "\u03B2 \u03B8 \u03B4 \u03B6 \u03BB \u03BE", flags: blueZoneTop},
		{chars: "\u03B1 \u03B5 \u03B9 \u03BF \u03C0 \u03C3 \u03C4 \u03C9", flags: blueZoneTop | blueZoneXHeight},
		{chars: "\u03B1 \u03B5 \u03B9 \u03BF \u03C0 \u03C3 \u03C4 \u03C9", flags: 0},
		{chars: "\u03B2 \u03B3 \u03B7 \u03BC \u03C1 \u03C6 \u03C7 \u03C8", flags: 0},
	},
}

var scriptArabic = scriptClass{
	name:     "Arabic",
	group:    scriptGroupDefault,
	stdChars: []rune{'\u0644', '\u062D', '\u0640'}, // ل ح ـ
	// af_arab_uniranges.
	uniranges: []runePair{
		uniRange(0x0600, 0x06FF), uniRange(0x0750, 0x07FF),
		uniRange(0x08A0, 0x08FF), uniRange(0xFB50, 0xFDFF),
		uniRange(0xFE70, 0xFEFF), uniRange(0x1EE00, 0x1EEFF),
	},
	blues: []blueSpec{
		{chars: "\u0627 \u0625 \u0644 \u0643 \u0637 \u0638", flags: blueZoneTop},
		{chars: "\u062A \u062B \u0637 \u0638 \u0643", flags: 0},
		{chars: "\u0640", flags: blueZoneNeutral},
	},
}

var scriptCJK = scriptClass{
	name:     "CJKV ideographs",
	group:    scriptGroupCJK,
	stdChars: []rune{'\u7530', '\u56D7'}, // 田 囗
	// af_hani_uniranges.
	uniranges: []runePair{
		uniRange(0x1100, 0x11FF), uniRange(0x2E80, 0x2EFF),
		uniRange(0x2F00, 0x2FDF), uniRange(0x2FF0, 0x2FFF),
		uniRange(0x3000, 0x303F), uniRange(0x3040, 0x309F),
		uniRange(0x30A0, 0x30FF), uniRange(0x3100, 0x312F),
		uniRange(0x3130, 0x318F), uniRange(0x3190, 0x319F),
		uniRange(0x31A0, 0x31BF), uniRange(0x31C0, 0x31EF),
		uniRange(0x31F0, 0x31FF), uniRange(0x3300, 0x33FF),
		uniRange(0x3400, 0x4DBF), uniRange(0x4DC0, 0x4DFF),
		uniRange(0x4E00, 0x9FFF), uniRange(0xA960, 0xA97F),
		uniRange(0xAC00, 0xD7AF), uniRange(0xD7B0, 0xD7FF),
		uniRange(0xF900, 0xFAFF), uniRange(0xFE10, 0xFE1F),
		uniRange(0xFE30, 0xFE4F), uniRange(0xFF00, 0xFFEF),
		uniRange(0x1B000, 0x1B0FF), uniRange(0x1B100, 0x1B12F),
		uniRange(0x1D300, 0x1D35F),
	},
	blues: []blueSpec{
		// CJK vertical blues (the only ones active — horizontal is disabled
		// per FreeType since 2004). The '|' separator divides fill chars from
		// flat chars within the string.
		{
			chars: "\u4ED6 \u4EEC \u4F60 \u4F86 \u5011 \u5230 \u548C \u5730 " +
				"\u5BF9 \u5C0D \u5C31 \u5E2D \u6211 \u65F6 \u6642 \u6703 " +
				"\u6765 \u70BA \u80FD \u8230 \u8AAA \u8BF4 \u8FD9 \u9019 \u9F4A" +
				" | " +
				"\u519B \u540C \u5DF2 \u613F \u65E2 \u661F \u662F \u666F " +
				"\u6C11 \u7167 \u73B0 \u73FE \u7406 \u7528 \u7F6E \u8981 " +
				"\u8ECD \u90A3 \u914D \u91CC \u958B \u96F7 \u9732 \u9762 \u987E",
			flags: blueZoneTop,
		},
		{
			chars: "\u4E2A \u4E3A \u4EBA \u4ED6 \u4EE5 \u4EEC \u4F60 \u4F86 " +
				"\u500B \u5011 \u5230 \u548C \u5927 \u5BF9 \u5C0D \u5C31 " +
				"\u6211 \u65F6 \u6642 \u6709 \u6765 \u70BA \u8981 \u8AAA \u8BF4" +
				" | " +
				"\u4E3B \u4E9B \u56E0 \u5B83 \u60F3 \u610F \u7406 \u751F " +
				"\u7576 \u770B \u7740 \u7F6E \u8005 \u81EA \u8457 \u88E1 " +
				"\u8FC7 \u8FD8 \u8FDB \u9032 \u904E \u9053 \u9084 \u91CC \u9762",
			flags: 0,
		},
	},
}

// scriptClasses lists all supported scripts in priority order for per-glyph
// detection. Higher priority scripts are checked first: a glyph is assigned
// the first script whose unirange contains one of its cmap code points.
// This matches FreeType afglobal.c af_face_globals_compute_style_coverage:
// Unicode ranges are mutually exclusive between scripts, so priority only
// matters for glyphs addressable by multiple code points in different
// scripts (rare; first wins in FreeType too).
//
// The list is generated in autohint_scripts_gen.go (skrifa SCRIPT_CLASSES
// order; Latin/Hebrew/Cyrillic/Greek/Arabic and CJK are defined here and
// referenced there).

// glyphScriptCache caches the per-glyph script assignment for each font.
// Matches FreeType's AF_FaceGlobals.glyph_styles array.
var glyphScriptCache struct {
	mu    sync.RWMutex
	cache map[string][]*scriptClass
}

func init() {
	glyphScriptCache.cache = make(map[string][]*scriptClass)
}

// glyphScriptKey identifies a font for the per-glyph script cache.
// FullName+UPM matches autoHintMetricsKey conventions.
func glyphScriptKey(font ParsedFont) string {
	return fmt.Sprintf("%s@%d", font.FullName(), font.UnitsPerEm())
}

// perGlyphScripts returns the per-glyph script assignment for a font,
// computing it once and caching. The returned slice has one entry per
// glyph index. Glyphs covered by no unirange fall back to scriptLatin
// (FreeType: default style = the module's default script, Latin).
//
// See FreeType afglobal.c:126 af_face_globals_compute_style_coverage.
func perGlyphScripts(font ParsedFont) []*scriptClass {
	key := glyphScriptKey(font)

	glyphScriptCache.mu.RLock()
	s, ok := glyphScriptCache.cache[key]
	glyphScriptCache.mu.RUnlock()
	if ok {
		return s
	}

	glyphScriptCache.mu.Lock()
	defer glyphScriptCache.mu.Unlock()
	if s, ok := glyphScriptCache.cache[key]; ok {
		return s
	}

	numGlyphs := font.NumGlyphs()
	s = make([]*scriptClass, numGlyphs)
	// Default assignment: Latin (fallback style for uncovered glyphs).
	for i := range s {
		s[i] = &scriptLatin
	}

	// Scan each script's unicode ranges, assigning the FIRST script that
	// covers a glyph. Matches FreeType's loop order in afglobal.c.
	for _, sc := range scriptClasses {
		for _, r := range sc.uniranges {
			for cp := r.first; cp <= r.last; cp++ {
				gid := int(font.GlyphIndex(cp))
				if gid > 0 && gid < numGlyphs && s[gid] == &scriptLatin {
					s[gid] = sc
				}
			}
		}
	}

	// NOTE: a glyph already assigned to a non-Latin script is never
	// overwritten, even if a later range of a higher-priority script
	// covers it — this matches FreeType's "first style wins" (styles
	// are scanned in order, glyphs assigned once).
	glyphScriptCache.cache[key] = s
	return s
}

// scriptForGlyph returns the auto-hint script for a glyph, using
// per-glyph detection with Latin fallback.
func scriptForGlyph(font ParsedFont, gid GlyphID) *scriptClass {
	s := perGlyphScripts(font)
	if int(gid) < len(s) {
		return s[gid]
	}
	return &scriptLatin
}

// detectFontScript returns the primary script for a font for legacy
// font-level callers (metric computation that is not per-glyph).
// It returns the script that covers the most glyphs; ties break in
// scriptClasses order (Latin preferred last, matching fallback logic).
// This is a font-level summary of perGlyphScripts and is only used where
// a single script must be chosen (e.g. computing standard widths).
func detectFontScript(font ParsedFont) *scriptClass {
	s := perGlyphScripts(font)
	counts := map[*scriptClass]int{}
	for _, sc := range s {
		counts[sc]++
	}
	best := &scriptLatin
	bestN := -1
	for _, sc := range scriptClasses {
		if n := counts[sc]; n > bestN {
			best, bestN = sc, n
		}
	}
	if counts[&scriptLatin] > bestN {
		best, bestN = &scriptLatin, counts[&scriptLatin]
	}
	return best
}
