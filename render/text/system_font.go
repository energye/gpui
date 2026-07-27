// System / default font resolution for render/text.
//
// Policy:
//
//   - Library default = ONE common system UI font (platform sans via build tags).
//   - Apps / examples / tests set fonts with functions only (no env vars):
//     SetDefaultFontPath, SetSystemFontPaths, FontResolver.SetFontFile / SetPaths,
//     or text.NewFontSourceFromFile.
//   - Multi-script MultiFace is opt-in (LoadMultiFace / FontResolver.SetChain).
package text

import (
	"fmt"
	"os"
	"sync"
)

// FontRole groups candidate system font files (UI fallback roles, not Unicode Script).
type FontRole int

const (
	// FontRoleUI is the single default UI sans (Latin; often also Cyrillic/Greek).
	FontRoleUI FontRole = iota
	// FontRoleCJK optional Han/Hiragana/Katakana/Hangul coverage.
	FontRoleCJK
	// FontRoleThai optional Thai.
	FontRoleThai
	// FontRoleDevanagari optional Devanagari.
	FontRoleDevanagari
	// FontRoleArabic optional dedicated Arabic.
	FontRoleArabic
)

// FontRoleLatin is an alias of FontRoleUI.
const FontRoleLatin = FontRoleUI

// DefaultMultiFontRoleChain is optional MultiFace order for apps that need
// multi-script coverage. Not used by LoadDefaultFace.
var DefaultMultiFontRoleChain = []FontRole{
	FontRoleUI,
	FontRoleCJK,
	FontRoleThai,
	FontRoleDevanagari,
	FontRoleArabic,
}

var (
	sysFontMu       sync.RWMutex
	sysFontOverride = map[FontRole][]string{}
)

// SetDefaultFontPath sets the process primary UI font file (FontRoleUI).
// Empty path clears the override (platform default resumes).
// Prefer FontResolver in multi-window apps.
func SetDefaultFontPath(path string) {
	if path == "" {
		SetSystemFontPaths(FontRoleUI)
		return
	}
	SetSystemFontPaths(FontRoleUI, path)
}

// SetSystemFontPaths replaces process candidates for one role.
// Empty paths clear that role. LoadDefaultFace only uses FontRoleUI;
// other roles apply to LoadMultiFace when using process defaults.
func SetSystemFontPaths(role FontRole, paths ...string) {
	sysFontMu.Lock()
	defer sysFontMu.Unlock()
	if len(paths) == 0 {
		delete(sysFontOverride, role)
		return
	}
	cp := make([]string, len(paths))
	copy(cp, paths)
	sysFontOverride[role] = cp
}

// ClearSystemFontPaths clears all process path overrides.
func ClearSystemFontPaths() {
	sysFontMu.Lock()
	defer sysFontMu.Unlock()
	sysFontOverride = map[FontRole][]string{}
}

// SystemFontCandidates returns probe paths for a role:
// function override (SetDefaultFontPath / SetSystemFontPaths) → platform defaults.
func SystemFontCandidates(role FontRole) []string {
	var out []string
	sysFontMu.RLock()
	if ov, ok := sysFontOverride[role]; ok && len(ov) > 0 {
		out = append(out, ov...)
	}
	sysFontMu.RUnlock()
	out = append(out, platformSystemFontCandidates(role)...)
	return dedupeStrings(out)
}

// LoadDefaultFace loads exactly one common system UI font at points (default 14).
// Honors SetDefaultFontPath when set; otherwise platform candidates.
// No MultiFace — use LoadMultiFace or FontResolver in the app/example.
func LoadDefaultFace(points float64) (Face, string, error) {
	if points <= 0 {
		points = 14
	}
	return loadFaceFromCandidates(points, SystemFontCandidates(FontRoleUI))
}

// LoadMultiFace builds a MultiFace from roles (first face with glyph wins).
// Empty roles → DefaultMultiFontRoleChain.
func LoadMultiFace(points float64, roles ...FontRole) (Face, string, error) {
	if len(roles) == 0 {
		roles = DefaultMultiFontRoleChain
	}
	return assembleFaces(points, roles, SystemFontCandidates)
}

// LoadDefaultFaceFor is LoadMultiFace with an explicit role chain.
func LoadDefaultFaceFor(points float64, chain []FontRole) (Face, string, error) {
	return assembleFaces(points, chain, SystemFontCandidates)
}

// ---------------------------------------------------------------------------
// FontResolver — instance config (recommended for apps / tests)
// ---------------------------------------------------------------------------

// FontResolver holds per-role paths and optional MultiFace chain.
// Does not mutate process globals unless UseProcessOverride is set.
type FontResolver struct {
	mu                 sync.RWMutex
	paths              map[FontRole][]string
	chain              []FontRole
	UsePlatform        bool
	UseProcessOverride bool
}

// NewFontResolver returns a resolver for a single UI font (FontRoleUI).
// SetFontFile / SetPaths / SetChain configure it; no environment variables.
func NewFontResolver() *FontResolver {
	return &FontResolver{
		paths:       map[FontRole][]string{},
		chain:       []FontRole{FontRoleUI},
		UsePlatform: true,
	}
}

// DefaultFontResolver mirrors process LoadDefaultFace (includes SetDefaultFontPath).
func DefaultFontResolver() *FontResolver {
	r := NewFontResolver()
	r.UseProcessOverride = true
	return r
}

// SetPaths sets candidate files for one role on this resolver only.
func (r *FontResolver) SetPaths(role FontRole, paths ...string) *FontResolver {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.paths == nil {
		r.paths = map[FontRole][]string{}
	}
	if len(paths) == 0 {
		delete(r.paths, role)
		return r
	}
	cp := make([]string, len(paths))
	copy(cp, paths)
	r.paths[role] = cp
	return r
}

// SetFontFile sets the single UI font file for this resolver.
func (r *FontResolver) SetFontFile(path string) *FontResolver {
	if path == "" {
		return r.SetPaths(FontRoleUI)
	}
	return r.SetPaths(FontRoleUI, path)
}

// SetChain sets roles assembled into MultiFace (or one face if len==1).
func (r *FontResolver) SetChain(roles ...FontRole) *FontResolver {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chain = append([]FontRole{}, roles...)
	return r
}

// Candidates returns probe paths for a role under this resolver.
func (r *FontResolver) Candidates(role FontRole) []string {
	if r == nil {
		return SystemFontCandidates(role)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	if ov, ok := r.paths[role]; ok && len(ov) > 0 {
		out = append(out, ov...)
	}
	if r.UseProcessOverride {
		sysFontMu.RLock()
		if ov, ok := sysFontOverride[role]; ok && len(ov) > 0 {
			out = append(out, ov...)
		}
		sysFontMu.RUnlock()
	}
	if r.UsePlatform {
		out = append(out, platformSystemFontCandidates(role)...)
	}
	return dedupeStrings(out)
}

// Load builds a Face at points. Default = one UI font (system or SetFontFile).
func (r *FontResolver) Load(points float64) (Face, string, error) {
	if r == nil {
		return LoadDefaultFace(points)
	}
	r.mu.RLock()
	chain := append([]FontRole{}, r.chain...)
	r.mu.RUnlock()
	if len(chain) == 0 {
		chain = []FontRole{FontRoleUI}
	}
	if len(chain) == 1 {
		if points <= 0 {
			points = 14
		}
		return loadFaceFromCandidates(points, r.Candidates(chain[0]))
	}
	return assembleFaces(points, chain, r.Candidates)
}

// ---------------------------------------------------------------------------

func assembleFaces(points float64, chain []FontRole, candidates func(FontRole) []string) (Face, string, error) {
	if points <= 0 {
		points = 14
	}
	if len(chain) == 0 {
		chain = []FontRole{FontRoleUI}
	}
	if candidates == nil {
		candidates = SystemFontCandidates
	}

	var faces []Face
	var paths []string
	for _, role := range chain {
		f, path, err := loadFaceFromCandidates(points, candidates(role))
		if err != nil || f == nil {
			continue
		}
		dup := false
		for _, p := range paths {
			if p == path {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		faces = append(faces, f)
		paths = append(paths, path)
	}

	if len(faces) == 0 {
		return nil, "", fmt.Errorf("text: no system font found (SetDefaultFontPath / FontResolver.SetFontFile / platform fonts)")
	}
	if len(faces) == 1 {
		return faces[0], paths[0], nil
	}
	mf, err := NewMultiFace(faces...)
	if err != nil {
		return faces[0], paths[0] + " (MultiFace failed: " + err.Error() + ")", nil
	}
	desc := ""
	for i, p := range paths {
		if i > 0 {
			desc += " + "
		}
		desc += p
	}
	return mf, desc + " (MultiFace)", nil
}

func loadFaceFromCandidates(points float64, candidates []string) (Face, string, error) {
	var lastErr error
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		src, err := NewFontSourceFromFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		return src.Face(points), path, nil
	}
	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", fmt.Errorf("text: no font in candidate list")
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
