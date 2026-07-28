package rendering

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/energye/gpui/render/text"
)

// TryLoadDefaultFace loads one common system UI font (library default).
//
// Configure fonts with functions only (no env):
//
//	text.SetDefaultFontPath("/path/App.ttf")           // process
//	r := text.NewFontResolver().SetFontFile("a.ttf") // instance
//	face, _, _ := r.Load(16)
//	// or rendering.TryLoadDefaultFaceWith(r, 16)
//
// Multi-script is opt-in in the app/example: text.LoadMultiFace(16).
func TryLoadDefaultFace(points float64) (text.Face, string, error) {
	return text.LoadDefaultFace(points)
}

// TryLoadDefaultFaceWith uses an app-owned FontResolver.
func TryLoadDefaultFaceWith(r *text.FontResolver, points float64) (text.Face, string, error) {
	if r == nil {
		return text.LoadDefaultFace(points)
	}
	return r.Load(points)
}

// LoadFaceByFamily resolves a CSS-like family name (or file path) to a Face
// (FT-STYLE-FAMILY string API). Supported generic names:
//
//	"" / "sans" / "sans-serif" → system UI sans (LoadDefaultFace)
//	"serif"                   → common serif TTF candidates
//	"mono" / "monospace"      → common monospace TTF candidates
//
// An absolute or relative path to a .ttf/.otf/.ttc is loaded directly.
// Returns (nil, "", err) when no candidate exists (tests should Skip).
func LoadFaceByFamily(family string, points float64) (text.Face, string, error) {
	if points <= 0 {
		points = 14
	}
	fam := strings.TrimSpace(family)
	lower := strings.ToLower(fam)

	// Direct file path.
	if fam != "" && (strings.Contains(fam, "/") || strings.Contains(fam, `\`) ||
		strings.HasSuffix(lower, ".ttf") || strings.HasSuffix(lower, ".otf") ||
		strings.HasSuffix(lower, ".ttc")) {
		return loadFaceFromFile(fam, points)
	}

	switch lower {
	case "", "sans", "sans-serif", "ui", "system-ui":
		return text.LoadDefaultFace(points)
	case "serif":
		return loadFaceFromCandidates(points, serifFontCandidates())
	case "mono", "monospace", "code":
		return loadFaceFromCandidates(points, monoFontCandidates())
	default:
		// Treat bare names as basenames under common font dirs / user fonts.
		cands := append(serifFontCandidates(), monoFontCandidates()...)
		cands = append(cands, text.SystemFontCandidates(text.FontRoleUI)...)
		var matched []string
		base := lower
		if !strings.HasSuffix(base, ".ttf") && !strings.HasSuffix(base, ".otf") {
			// try as substring of path
			for _, c := range cands {
				if strings.Contains(strings.ToLower(filepath.Base(c)), base) {
					matched = append(matched, c)
				}
			}
		}
		if len(matched) == 0 {
			return nil, "", fmt.Errorf("rendering: unknown font family %q", family)
		}
		return loadFaceFromCandidates(points, matched)
	}
}

func loadFaceFromFile(path string, points float64) (text.Face, string, error) {
	src, err := text.NewFontSourceFromFile(path)
	if err != nil {
		return nil, "", err
	}
	return src.Face(points), path, nil
}

func loadFaceFromCandidates(points float64, paths []string) (text.Face, string, error) {
	var last error
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			last = err
			continue
		}
		face, desc, err := loadFaceFromFile(p, points)
		if err == nil {
			return face, desc, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("rendering: no font candidates available")
	}
	return nil, "", last
}

func serifFontCandidates() []string {
	return []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSerif.ttf",
		"/usr/share/fonts/TTF/DejaVuSerif.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSerif-Regular.ttf",
		"/usr/share/fonts/truetype/noto/NotoSerif-Regular.ttf",
		"/usr/share/fonts/truetype/freefont/FreeSerif.ttf",
	}
}

func monoFontCandidates() []string {
	return []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
		"/usr/share/fonts/TTF/DejaVuSansMono.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf",
		"/usr/share/fonts/truetype/noto/NotoSansMono-Regular.ttf",
		"/usr/share/fonts/truetype/ubuntu/UbuntuMono-R.ttf",
	}
}
