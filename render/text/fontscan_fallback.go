package text

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
)

// fontScanFallback resolves missing glyphs against the system font index
// (M3: C-class fontscan integration).
//
// Policy:
//   - Lazy: the system font scan happens on the first rune miss, not at startup.
//   - Index: typesetting/fontscan builds a persistent index on first use
//     (cacheDir), so subsequent runs do not rescan the file system.
//   - Selection: first font covering the rune, biased toward regular
//     (normal weight/style/stretch) faces; results cached per rune and
//     per (rune, size) face.
//   - Failure: scan errors degrade to no fallback (missing glyph stays
//     a .notdef box as before).
type fontScanFallback struct {
	mu         sync.Mutex
	initOnce   sync.Once
	footprints []fontscan.Footprint
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
		f.mu.Lock()
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
	best := f.bestLocation(f.footprints, r)
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
	if path == "" && f.footprints != nil {
		path = f.bestLocation(f.footprints, r)
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

// bestLocation returns the most suitable font file covering r.
// Prefers regular faces (weight 300-500, normal style/stretch) over
// bold/italic/condensed ones.
func (f *fontScanFallback) bestLocation(fps []fontscan.Footprint, r rune) string {
	best := ""
	bestScore := -1
	for i := range fps {
		fp := &fps[i]
		if !fp.Runes.Contains(r) {
			continue
		}
		s := aspectScore(fp.Aspect)
		if s > bestScore {
			bestScore = s
			best = fp.Location.File
		}
	}
	return best
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
