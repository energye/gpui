package wrkit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// UIFace is a process-wide default face for ui_wr_* chrome / LiveHUD.
// DrawString / RenderText Paint **no-op without a Face** — labels placed at
// absolute coords would look like empty layout holes until this is set.
var (
	uiFaceOnce sync.Once
	uiFace     text.Face
	uiFacePath string
	uiFaceErr  error
)

// EnsureUIFace loads a system UI font once (size 14 base). Safe to call often.
// Returns (face, path, err). On err, text will still layout by rune estimate
// but will not paint — callers should log and avoid relying on invisible labels.
//
// Default chain puts Source Han Sans SC (Noto Sans CJK SC — the open-source
// Microsoft YaHei equivalent, covering Latin too) first, then the Latin UI
// sans, Thai, Devanagari and Arabic role fallbacks. The system NotoSansCJK
// TTC resolves to the JP sub-font by default collection index, so the SC
// single-language file in the user font dir is preferred when present —
// simplified-Han glyph forms with the YaHei-style design.
func EnsureUIFace() (text.Face, string, error) {
	uiFaceOnce.Do(func() {
		uiFace, uiFacePath, uiFaceErr = loadUIChain()
		if uiFaceErr != nil {
			fmt.Fprintf(os.Stderr, "wrkit: UI font load failed: %v (text will be invisible)\n", uiFaceErr)
		} else {
			fmt.Fprintf(os.Stderr, "wrkit: UI font %s\n", uiFacePath)
		}
	})
	return uiFace, uiFacePath, uiFaceErr
}

// loadUIChain builds the ui_wr_* chrome MultiFace: CJK role first so the
// YaHei-style Source Han Sans SC face drives Han, with real regular/bold
// weight families for CJK and Latin. Non-Latin script fallbacks stay on the
// platform candidate lists.
//
// Weight selection is a font-family concern (CSS/Flutter model): 600/700 on
// a family picks the real Semibold/Bold cut, never a post-raster mask
// dilation. Standalone fonts that have no bold sibling still fall back to the
// engine's standard outline embolden.
func loadUIChain() (text.Face, string, error) {
	var faces []text.Face
	var paths []string
	add := func(face text.Face, path string) {
		if face == nil || path == "" {
			return
		}
		for _, p := range paths {
			if p == path {
				return
			}
		}
		faces = append(faces, face)
		paths = append(paths, path)
	}

	// CJK: one real family (Regular + Bold) so Han text at 600/700 stays
	// crisp. The system TTC stores SC at collection index 2; the user OTF is
	// single-face.
	if cjkRefs := cjkFamilyRefs(); len(cjkRefs) > 0 {
		face, _, path, err := text.LoadFontFamily(14, cjkRefs...)
		if face != nil {
			add(face, path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "wrkit: CJK family fallback: %v\n", err)
			}
		}
	}
	if len(faces) == 0 {
		if face, path, err := loadRoleFace(text.FontRoleCJK); err == nil {
			add(face, path)
		}
	}

	// Latin: DejaVu/Noto/Liberation regular+bold sibling when installed.
	if latinRefs := latinFamilyRefs(); len(latinRefs) > 0 {
		face, _, path, err := text.LoadFontFamily(14, latinRefs...)
		if face != nil {
			add(face, path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "wrkit: Latin family fallback: %v\n", err)
			}
		}
	}
	if len(faces) == 0 {
		if face, path, err := loadRoleFace(text.FontRoleUI); err == nil {
			add(face, path)
		}
	}

	for _, role := range []text.FontRole{
		text.FontRoleThai,
		text.FontRoleDevanagari,
		text.FontRoleArabic,
	} {
		if face, path, err := loadRoleFace(role); err == nil {
			add(face, path)
		}
	}

	if len(faces) == 0 {
		return text.LoadDefaultFaceFor(14, []text.FontRole{
			text.FontRoleCJK,
			text.FontRoleUI,
			text.FontRoleThai,
			text.FontRoleDevanagari,
			text.FontRoleArabic,
		})
	}
	if len(faces) == 1 {
		return faces[0], paths[0], nil
	}
	mf, err := text.NewMultiFace(faces...)
	if err != nil {
		return faces[0], paths[0] + " (MultiFace failed: " + err.Error() + ")", nil
	}
	return mf, strings.Join(paths, " + ") + " (MultiFace)", nil
}

// loadRoleFace resolves one script role independently, so the family faces
// above can be combined with the platform fallbacks without pulling a second
// copy of CJK/Latin into the chain.
func loadRoleFace(role text.FontRole) (text.Face, string, error) {
	resolver := text.NewFontResolver()
	resolver.SetChain(role)
	return resolver.Load(14)
}

// cjkFamilyRefs prefers the single-face SC OTF in the user font dir (the
// crisp YaHei-style design), with the SC face of the system Noto CJK TTC as
// regular fallback. Bold comes from Noto CJK Bold TTC index 2 (SC).
func cjkFamilyRefs() []text.FontRef {
	var regulars []text.FontRef
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		regulars = append(regulars, text.FontRef{
			Path: filepath.Join(home, ".local", "share", "fonts", "NotoSansCJKsc-Regular.otf"),
		})
	}
	regulars = append(regulars,
		text.FontRef{Path: "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", CollectionIndex: 2},
		text.FontRef{Path: "/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc", CollectionIndex: 2},
		text.FontRef{Path: "/usr/share/fonts/opentype/source-han-sans/SourceHanSansSC-Regular.otf"},
	)
	regular := firstExistingRef(regulars)
	if regular.Path == "" {
		return nil
	}
	bold := firstExistingRef([]text.FontRef{
		{Path: "/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc", CollectionIndex: 2},
		{Path: "/usr/share/fonts/noto-cjk/NotoSansCJK-Bold.ttc", CollectionIndex: 2},
		{Path: "/usr/share/fonts/opentype/source-han-sans/SourceHanSansSC-Bold.otf"},
	})
	if bold.Path == "" {
		return []text.FontRef{regular}
	}
	return []text.FontRef{regular, bold}
}

// latinFamilyRefs finds the platform UI regular and its -Bold sibling. A
// missing sibling yields a one-cut family, which still renders through the
// standard outline fallback.
func latinFamilyRefs() []text.FontRef {
	candidates := text.SystemFontCandidates(text.FontRoleUI)
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		refs := []text.FontRef{{Path: path}}
		if bold := boldSibling(path); bold != "" {
			refs = append(refs, text.FontRef{Path: bold})
		}
		return refs
	}
	return nil
}

// boldSibling returns the conventional -Bold file next to a regular static
// face. The family constructor still validates that both files report the
// same typographic family and distinct weights.
func boldSibling(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for _, suffix := range []string{"-Bold", "Bold"} {
		candidate := base + suffix + ext
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// DejaVu/Liberation use "FamilySans.ttf" for regular and
	// "FamilySans-Bold.ttf" for bold; Noto uses "Family-Regular.ttf" and
	// "Family-Bold.ttf". Both are covered above. Try replacing "Regular"
	// with "Bold" for the remaining naming schemes.
	if strings.HasSuffix(base, "Regular") {
		candidate := strings.TrimSuffix(base, "Regular") + "Bold" + ext
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func firstExistingRef(refs []text.FontRef) text.FontRef {
	for _, ref := range refs {
		if ref.Path == "" {
			continue
		}
		if _, err := os.Stat(ref.Path); err == nil {
			return ref
		}
	}
	return text.FontRef{}
}

// FaceAt returns a face scaled to points (falls back to EnsureUIFace base).
// The MultiFace chain is re-pinned to the production hinting mode
// (HintingVertical = FT light, matching the GPU glyph-mask pipeline, engine
// default is HintingFull) — MultiFace.Source() is nil, so the re-pin goes
// through the engine's MultiFace.WithHinting + AtSize (options preserved).
func FaceAt(points float64) text.Face {
	base, _, err := EnsureUIFace()
	if err != nil || base == nil {
		return nil
	}
	if points <= 0 {
		points = 14
	}
	if mf, ok := base.(*text.MultiFace); ok {
		if f := mf.WithHinting(text.HintingVertical); f != nil {
			if f2 := f.AtSize(points); f2 != nil {
				return f2
			}
		}
	}
	if src := base.Source(); src != nil {
		if f := src.Face(points, text.WithHinting(text.HintingVertical)); f != nil {
			return f
		}
	}
	return base
}

// Label builds a RenderText chrome label **with Face set** so Paint draws.
// Without Face, DrawString no-ops → Place coords look empty.
func Label(s string, size float64, r, g, b float64) *rendering.RenderText {
	t := rendering.NewRenderText(s)
	t.FontSize = size
	t.R, t.G, t.B, t.A = r, g, b, 1
	t.ApproxCharW = 0.55
	if face := FaceAt(size); face != nil {
		t.SetFace(face)
	}
	return t
}

// ApplyFace sets Face on an existing RenderText (no-op if face unavailable).
func ApplyFace(t *rendering.RenderText, points float64) {
	if t == nil {
		return
	}
	if face := FaceAt(points); face != nil {
		t.SetFace(face)
	}
}
