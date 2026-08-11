package text

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"github.com/go-text/typesetting/language"
)

// fontScanFallback resolves missing glyphs against the system font index
// (M3: C-class fontscan integration).
//
// Policy:
//   - Lazy: the system font scan happens on the first rune miss, not at startup.
//   - Index: typesetting/fontscan builds a persistent index on first use
//     (cacheDir), so subsequent runs do not rescan the file system.
//   - Selection: uses FontMap (SetScript + ResolveFace) — the go-text
//     full matching semantics (family substitution copied from fontconfig,
//     per-script fallback candidates), NOT a hand-rolled score. Results
//     cached per rune and per (rune, size) face on top of FontMap's own
//     LRU. (M7: switched from bestLocation to FontMap to match fc-match
//     script-aware selection, e.g. Thai → Loma, Devanagari → Lohit.)
//   - Failure: scan errors degrade to no fallback (missing glyph stays
//     a .notdef box as before).
type fontScanFallback struct {
	mu         sync.Mutex
	initOnce   sync.Once
	fontMap    *fontscan.FontMap // nil if system scan failed
	footprints []fontscan.Footprint // family lookup data (SystemFontForFamily)
	pathCache  map[rune]string // rune → font file path ("" = not found)
	faceCache  map[fallbackKey]Face
}

type fallbackKey struct {
	r    rune
	size float64
}

// globalFallback is the process-wide missing-glyph fallback resolver.
var globalFallback = &fontScanFallback{}

// fallbackLogger quiets fontscan's internal scan logging.
type fallbackLogger struct{}

func (fallbackLogger) Printf(format string, args ...interface{}) {}

func (f *fontScanFallback) ensure() {
	f.initOnce.Do(func() {
		cacheDir := ""
		if dir, err := os.UserCacheDir(); err == nil {
			cacheDir = filepath.Join(dir, "gpui", "fontindex")
			_ = os.MkdirAll(cacheDir, 0o755)
		}
		fps, err := fontscan.SystemFonts(fallbackLogger{}, cacheDir)
		if err != nil {
			log.Printf("text: system font scan failed, missing-glyph fallback disabled: %v", err)
			return
		}
		// FontMap provides the full script-aware matching (family
		// substitution copied from fontconfig, per-script fallbacks).
		// It reuses the same persistent index / cache dir, so the scan
		// above and UseSystemFonts below share one index (no double work).
		fm := fontscan.NewFontMap(fallbackLogger{})
		if err := fm.UseSystemFonts(cacheDir); err != nil {
			log.Printf("text: fontmap init failed, rune fallback disabled: %v", err)
			return
		}
		f.mu.Lock()
		f.fontMap = fm
		f.footprints = fps
		f.pathCache = make(map[rune]string, 512)
		f.faceCache = make(map[fallbackKey]Face, 64)
		f.mu.Unlock()
	})
}

// resolvePath returns the system font file covering r, or "" if none.
func (f *fontScanFallback) resolvePath(r rune) string {
	f.ensure()
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pathCache[r]; ok {
		return p
	}
	best := f.bestLocation(f.fontMap, r)
	if best == "" {
		f.pathCache[r] = ""
		return ""
	}
	f.pathCache[r] = best
	return best
}

// resolveFace returns a Face for r at size, built from the covering system
// font; nil if none covers r.
func (f *fontScanFallback) resolveFace(r rune, size float64) Face {
	key := fallbackKey{r: r, size: size}
	f.ensure()
	f.mu.Lock()
	defer f.mu.Unlock()
	if fc, ok := f.faceCache[key]; ok {
		return fc
	}
	if fc, ok := f.faceCache[fallbackKey{r: r, size: -1}]; ok && fc != nil {
		// a size-agnostic face is cached
		return fc
	}
	path := f.pathCache[r]
	if path == "" && f.fontMap != nil {
		path = f.bestLocation(f.fontMap, r)
		f.pathCache[r] = path
	}
	if path == "" {
		return nil
	}
	src, err := NewFontSourceFromFile(path)
	if err != nil {
		return nil
	}
	face := src.Face(size)
	f.faceCache[key] = face
	return face
}

// bestLocation resolves the most suitable system font file covering r,
// using FontMap's script-aware matching (SetScript + ResolveFace). The
// script is derived from the rune via Unicode Script property, matching
// how fontconfig picks a language-appropriate face for the glyph.
func (f *fontScanFallback) bestLocation(fm *fontscan.FontMap, r rune) string {
	if fm == nil {
		return ""
	}
	if s := language.LookupScript(r); s != language.Unknown {
		fm.SetScript(s)
	}
	face := fm.ResolveFace(r)
	if face == nil {
		return ""
	}
	loc := fm.FontLocation(face.Font)
	return loc.File
}

func aspectScore(a font.Aspect) int {
	s := 0
	switch {
	case a.Weight >= 300 && a.Weight <= 500:
		s += 4
	case a.Weight < 300:
		s += 2
	default:
		s += 1
	}
	if a.Style == font.StyleNormal {
		s += 4
	}
	if a.Stretch == font.StretchNormal {
		s += 2
	}
	return s
}

// ClearFontScanFallbackCache drops cached rune→font resolutions (tests).
func ClearFontScanFallbackCache() {
	globalFallback.mu.Lock()
	defer globalFallback.mu.Unlock()
	globalFallback.pathCache = make(map[rune]string, 512)
	globalFallback.faceCache = make(map[fallbackKey]Face, 64)
}

// ---------------------------------------------------------------------------
// Family matching (M7: FontMgr-equivalent lookup).
//
// Reuses the same lazily-built system font index (fontscan.SystemFonts) as
// the missing-glyph fallback, so family queries do not trigger a second
// scan. Matching is case/space normalized via font.NormalizeFamily, the
// same normalizer go-text uses internally — this keeps our lookup
// consistent with fontscan's Footprint.Family values.
// ---------------------------------------------------------------------------

// SystemFontForFamily returns the file path of the best system font whose
// family name matches the given family (e.g. "Noto Sans CJK JP").
// "Best" = highest aspect score among regular-weight candidates, matching
// the rune-fallback bias toward normal faces. ok=false when the family is
// not present in the system index.
func SystemFontForFamily(family string) (path string, ok bool) {
	if family == "" {
		return "", false
	}
	norm := font.NormalizeFamily(family)
	globalFallback.ensure()
	globalFallback.mu.Lock()
	defer globalFallback.mu.Unlock()
	best := ""
	bestScore := -1
	for i := range globalFallback.footprints {
		fp := &globalFallback.footprints[i]
		if fp.Family != norm {
			continue
		}
		s := aspectScore(fp.Aspect)
		if s > bestScore {
			bestScore = s
			best = fp.Location.File
		}
	}
	return best, best != ""
}

// SystemFontsByFamily returns all system font file paths whose family name
// matches the given family (case/space normalized). Useful for building a
// per-family candidate list; order is the scan order (not ranked).
func SystemFontsByFamily(family string) []string {
	if family == "" {
		return nil
	}
	norm := font.NormalizeFamily(family)
	globalFallback.ensure()
	globalFallback.mu.Lock()
	defer globalFallback.mu.Unlock()
	var out []string
	for i := range globalFallback.footprints {
		fp := &globalFallback.footprints[i]
		if fp.Family == norm {
			out = append(out, fp.Location.File)
		}
	}
	return out
}
